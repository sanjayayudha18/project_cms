"""Daily realisasi classification (.claude/sdlc/replenishment-realisasi/spec.md FR3).

One transaction per run, serialized by an advisory lock. Recomputes every non-final ticket row,
voids rows of inactive tickets / cancelled requests, writes `no_order` rows for fills nobody
ordered once their window closed. Final rows are never recomputed (FR3.5). System action:
retry_audit_logs + log, no audit_logs (S4 import_jobs pattern, FR3.10).
"""
from __future__ import annotations

import logging
import time as time_module
from dataclasses import dataclass, field
from datetime import date, timedelta
from typing import Any

import asyncpg

from replenish_classify.rules import (
    Fill,
    Ticket,
    add_workdays,
    amount_rows,
    classify_offset,
    is_vendor_mismatch,
    match_fills,
    window_closed,
    workday_offset,
)

logger = logging.getLogger(__name__)

LOCK_KEY = "replenish_classify"
FILE_TYPE = "replenish_classify"
# Instruction statuses (Q2b): vendor_accepted onward, plus legacy approved/ready.
INSTRUCTION_STATUSES = ["vendor_accepted", "completion_pending", "completed", "approved", "ready"]
# ponytail: fills older than this are not looked at again (a window closes within ~4 days);
# raise it if the job can be down for a month.
LOOKBACK_DAYS = 31
ITM_DENOMS = {100000: "replenish_denom_100k", 50000: "replenish_denom_50k",
              20000: "replenish_denom_20k", 10000: "replenish_denom_10k"}
LATE_ROLES = ["ATM-SPV", "BRANCH-ATM-SPV"]
EMAIL_FOOTER = "\n\n--\nPesan otomatis CROWN Cash Management System. Mohon tidak membalas email ini."


@dataclass
class Summary:
    skipped: bool = False
    voided: int = 0
    tickets: int = 0
    final: int = 0
    no_order: int = 0
    late_notified: int = 0
    vendor_refreshed: int = 0
    counts: dict[str, int] = field(default_factory=dict)


async def classify(conn: asyncpg.Connection, today: date, start_date: date, *, email_enabled: bool) -> Summary:
    """Runs one classification pass inside the caller's transaction."""
    if not await conn.fetchval("SELECT pg_try_advisory_xact_lock(hashtextextended($1, 0))", LOCK_KEY):
        return Summary(skipped=True)
    s = Summary()
    holidays = {r["holiday_date"] for r in await conn.fetch("SELECT holiday_date FROM holidays WHERE is_active")}
    s.voided = int((await conn.execute("""
        UPDATE replenish_classifications c
        SET classification = 'void', is_final = true, finalized_at = now(), classified_at = now(), itm_replenish_id = NULL
        FROM vendor_request_tickets t JOIN vendor_requests vr ON vr.request_number = t.request_number
        WHERE c.ticket_id = t.id AND NOT c.is_final
          AND (NOT t.is_active OR vr.is_canceled OR vr.status = 'cancelled')""")).split()[-1])
    # Review R3 (PO 2026-10-14): a request outside the instruction statuses (rejected, vendor_rejected)
    # stays pending while it may still come back, and becomes void once its window closed.
    parked = [r["id"] for r in await conn.fetch("""
        SELECT c.id, c.scheduled_date FROM replenish_classifications c
        JOIN vendor_request_tickets t ON t.id = c.ticket_id
        JOIN vendor_requests vr ON vr.request_number = t.request_number
        WHERE NOT c.is_final AND vr.status <> ALL($1::text[])""", INSTRUCTION_STATUSES)
        if window_closed(r["scheduled_date"], today, holidays)]
    if parked:
        await conn.execute("""
            UPDATE replenish_classifications SET classification = 'void', is_final = true, finalized_at = now(),
                classified_at = now(), itm_replenish_id = NULL
            WHERE id = ANY($1::bigint[])""", parked)
        s.voided += len(parked)
    # Review R2: pending rows hand their fill back before matching, so a fill can move to another
    # ticket without tripping UNIQUE (itm_replenish_id); the loop below claims it again.
    await conn.execute("UPDATE replenish_classifications SET itm_replenish_id = NULL WHERE NOT is_final AND itm_replenish_id IS NOT NULL")

    tickets = await conn.fetch("""
        SELECT t.id, t.terminal_id, t.ticket_number, t.replenish_date, t.result, vr.id AS request_id, vr.vendor_id
        FROM vendor_request_tickets t
        JOIN vendor_requests vr ON vr.request_number = t.request_number
        LEFT JOIN replenish_classifications c ON c.ticket_id = t.id
        WHERE t.is_active AND NOT vr.is_canceled AND vr.status = ANY($1::text[])
          AND t.replenish_date >= $2 AND (c.id IS NULL OR NOT c.is_final)""", INSTRUCTION_STATUSES, start_date)
    fills = await conn.fetch(f"""
        SELECT i.id, i.terminal_id, i.replenish_date, i.replenish_time, {", ".join(ITM_DENOMS.values())}
        FROM itm_replenish i
        WHERE i.replenish_date >= GREATEST($1::date, $2::date) AND i.replenish_date <= $3
          AND NOT EXISTS (SELECT 1 FROM replenish_classifications c WHERE c.itm_replenish_id = i.id AND c.is_final)""",
        start_date, today - timedelta(days=LOOKBACK_DAYS), today)
    fill_by_id = {f["id"]: f for f in fills}
    matched = match_fills(
        [Ticket(t["id"], t["terminal_id"], t["ticket_number"], t["replenish_date"]) for t in tickets],
        [Fill(f["id"], f["terminal_id"], f["replenish_date"], f["replenish_time"]) for f in fills],
        holidays)

    ticket_ids = [t["id"] for t in tickets]
    orders = await _per_ticket(conn, """
        SELECT t.id AS key, i.denom, SUM(i.amount_replenish)::bigint AS amount
        FROM vendor_request_tickets t
        JOIN vendor_requests vr ON vr.request_number = t.request_number
        JOIN vendor_request_items i ON i.vendor_request_id = vr.id AND i.terminal_id = t.terminal_id
        WHERE t.id = ANY($1::bigint[]) GROUP BY t.id, i.denom""", ticket_ids)
    reports = {r["ticket_id"]: r["realized_date"] for r in await conn.fetch(
        "SELECT ticket_id, realized_date FROM vendor_request_realizations WHERE ticket_id = ANY($1::bigint[])", ticket_ids)}
    report_amounts = await _per_ticket(conn, """
        SELECT r.ticket_id AS key, a.denom, a.amount FROM vendor_request_realizations r
        JOIN vendor_request_realization_amounts a ON a.realization_id = r.id
        WHERE r.ticket_id = ANY($1::bigint[])""", ticket_ids)
    branches = await _managing_branches(conn, [(t["terminal_id"], t["replenish_date"]) for t in tickets])

    late: list[asyncpg.Record] = []
    for t in tickets:
        fill = fill_by_id[matched[t["id"]].id] if t["id"] in matched else None
        row_id, notified = await _upsert_ticket_row(conn, t, fill, holidays, today, orders.get(t["id"], {}),
                                                    t["id"] in reports, reports.get(t["id"]), report_amounts.get(t["id"]),
                                                    branches.get((t["terminal_id"], t["replenish_date"])), s)
        if t["result"] is None and window_closed(t["replenish_date"], today, holidays) and notified is None:
            late.append(t)
    s.tickets = len(tickets)

    used = {f.id for f in matched.values()}
    loose = [f for f in fills if f["id"] not in used and window_closed(f["replenish_date"], today, holidays)]
    loose_branches = await _managing_branches(conn, [(f["terminal_id"], f["replenish_date"]) for f in loose])
    for f in loose:
        s.no_order += await _insert_no_order(conn, f, loose_branches.get((f["terminal_id"], f["replenish_date"])))
    s.counts["no_order"] = s.no_order

    if late:
        s.late_notified = await _notify_late(conn, late, email_enabled)
    s.vendor_refreshed = await _refresh_vendor_columns(conn)
    return s


async def _per_ticket(conn: asyncpg.Connection, sql: str, ids: list[int]) -> dict[int, dict[int, int]]:
    out: dict[int, dict[int, int]] = {}
    for r in await conn.fetch(sql, ids):
        out.setdefault(r["key"], {})[r["denom"]] = int(r["amount"])
    return out


async def _managing_branches(conn: asyncpg.Connection, keys: list[tuple[str, date]]) -> dict[tuple[str, date], tuple[int, int]]:
    """(terminal, date) -> (vendor_id, vendor_branch_id) of the ATM's active kelolaan on that date."""
    if not keys:
        return {}
    rows = await conn.fetch("""
        SELECT x.terminal_id, x.d, cur.vendor_id, cur.branch_id
        FROM unnest($1::text[], $2::date[]) AS x(terminal_id, d)
        JOIN LATERAL (
            SELECT vb.vendor_id, vb.id AS branch_id
            FROM atms a
            JOIN atm_vendor_packages avp ON avp.atm_id = a.id AND avp.is_active
            LEFT JOIN vendor_packages_branch p ON p.id = avp.vendor_package_id
            JOIN vendor_branches vb ON vb.id = COALESCE(p.vendor_branch_id, avp.vendor_branch_id)
            WHERE a.terminal_id = x.terminal_id
              AND avp.effective_start_date::date <= x.d
              AND (avp.effective_end_date IS NULL OR avp.effective_end_date::date >= x.d)
            ORDER BY avp.effective_start_date DESC, avp.id DESC LIMIT 1
        ) cur ON true""", [k[0] for k in keys], [k[1] for k in keys])
    return {(r["terminal_id"], r["d"]): (r["vendor_id"], r["branch_id"]) for r in rows}


def _itm_amounts(fill: asyncpg.Record | None) -> dict[int, int]:
    return {d: int(fill[col]) for d, col in ITM_DENOMS.items()} if fill else {}


async def _upsert_ticket_row(conn: asyncpg.Connection, t: asyncpg.Record, fill: asyncpg.Record | None,
                             holidays: set[date], today: date, order: dict[int, int], reported: bool,
                             vendor_date: date | None, vendor_amounts: dict[int, int] | None,
                             branch: tuple[int, int] | None, s: Summary) -> tuple[int, Any]:
    closed = window_closed(t["replenish_date"], today, holidays)
    offset = workday_offset(t["replenish_date"], fill["replenish_date"], holidays) if fill else None
    if not closed:
        cls = "pending"
    else:
        cls = classify_offset(offset) if fill else "not_done"
        s.final += 1
    s.counts[cls] = s.counts.get(cls, 0) + 1
    itm = _itm_amounts(fill)
    rows, diff = amount_rows(order, itm, vendor_amounts if reported else None)
    vendor_mismatch = reported and is_vendor_mismatch(fill["replenish_date"] if fill else None, itm, vendor_date,
                                                      vendor_amounts or {})
    row = await conn.fetchrow("""
        INSERT INTO replenish_classifications
            (ticket_id, itm_replenish_id, terminal_id, vendor_id, vendor_branch_id, scheduled_date,
             realized_date, realized_time, workday_offset, classification, is_final, has_amount_diff,
             vendor_mismatch, vendor_report_late, classified_at, finalized_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, now(), CASE WHEN $11 THEN now() END)
        ON CONFLICT (ticket_id) DO UPDATE SET
            itm_replenish_id = EXCLUDED.itm_replenish_id, vendor_id = EXCLUDED.vendor_id,
            vendor_branch_id = EXCLUDED.vendor_branch_id, scheduled_date = EXCLUDED.scheduled_date,
            realized_date = EXCLUDED.realized_date, realized_time = EXCLUDED.realized_time,
            workday_offset = EXCLUDED.workday_offset, classification = EXCLUDED.classification,
            is_final = EXCLUDED.is_final, has_amount_diff = EXCLUDED.has_amount_diff,
            vendor_mismatch = EXCLUDED.vendor_mismatch, vendor_report_late = EXCLUDED.vendor_report_late,
            classified_at = now(), finalized_at = EXCLUDED.finalized_at
        RETURNING id, late_notified_at""",
        t["id"], fill["id"] if fill else None, t["terminal_id"], t["vendor_id"], branch[1] if branch else None,
        t["replenish_date"], fill["replenish_date"] if fill else None, fill["replenish_time"] if fill else None,
        offset, cls, closed, bool(fill) and diff, vendor_mismatch, closed and t["result"] is None)
    await _replace_amounts(conn, row["id"], rows)
    return row["id"], row["late_notified_at"]


async def _refresh_vendor_columns(conn: asyncpg.Connection) -> int:
    """Review R4: a vendor report may arrive after its row went final (no deadline, FR2.7). Only the
    vendor columns follow it; class, offset and ITM amounts stay final (FR3.5)."""
    rows = await conn.fetch("""
        SELECT c.id, c.realized_date, r.id AS realization_id, r.realized_date AS vendor_date
        FROM replenish_classifications c
        JOIN vendor_request_realizations r ON r.ticket_id = c.ticket_id
        WHERE c.is_final AND c.classification <> 'void' AND r.submitted_at > c.classified_at""")
    # ponytail: two small reads per late report; batch them if this ever runs over thousands of rows.
    for r in rows:
        vendor = {a["denom"]: int(a["amount"]) for a in await conn.fetch(
            "SELECT denom, amount FROM vendor_request_realization_amounts WHERE realization_id = $1", r["realization_id"])}
        itm = {a["denom"]: int(a["itm_amount"]) for a in await conn.fetch(
            "SELECT denom, itm_amount FROM replenish_classification_amounts WHERE classification_id = $1", r["id"])}
        await conn.executemany(
            "UPDATE replenish_classification_amounts SET vendor_amount = $3 WHERE classification_id = $1 AND denom = $2",
            [(r["id"], d, vendor.get(d)) for d in itm])
        await conn.execute("UPDATE replenish_classifications SET vendor_mismatch = $2, classified_at = now() WHERE id = $1",
                           r["id"], is_vendor_mismatch(r["realized_date"], itm, r["vendor_date"], vendor))
    return len(rows)


async def _replace_amounts(conn: asyncpg.Connection, classification_id: int, rows: list[tuple[int, int, int, int | None]]) -> None:
    # Replace-all child rows (documented hard delete, like 030/031 *_amounts).
    await conn.execute("DELETE FROM replenish_classification_amounts WHERE classification_id = $1", classification_id)
    if rows:
        await conn.executemany("""
            INSERT INTO replenish_classification_amounts (classification_id, denom, order_amount, itm_amount, vendor_amount)
            VALUES ($1, $2, $3, $4, $5)""", [(classification_id, *r) for r in rows])


async def _insert_no_order(conn: asyncpg.Connection, f: asyncpg.Record, branch: tuple[int, int] | None) -> int:
    """FR3.8: one final no_order row per unclaimed fill; 0 when the fill already has a row."""
    row_id = await conn.fetchval("""
        INSERT INTO replenish_classifications
            (itm_replenish_id, terminal_id, vendor_id, vendor_branch_id, realized_date, realized_time,
             classification, is_final, finalized_at)
        VALUES ($1, $2, $3, $4, $5, $6, 'no_order', true, now())
        ON CONFLICT (itm_replenish_id) DO NOTHING RETURNING id""",
        f["id"], f["terminal_id"], branch[0] if branch else None, branch[1] if branch else None,
        f["replenish_date"], f["replenish_time"])
    if row_id is None:
        return 0
    rows, _ = amount_rows({}, _itm_amounts(f), None)
    await _replace_amounts(conn, row_id, rows)
    return 1


async def _notify_late(conn: asyncpg.Connection, late: list[asyncpg.Record], email_enabled: bool) -> int:
    """K3: one summary notification per active ATM-SPV/BRANCH-ATM-SPV for the tickets that just became
    late, then mark them so they are never announced twice. Same rows notification.Send writes
    (backend/internal/notification): in-app + email outbox (pending only when SMTP is configured)."""
    shown = ", ".join(f"{t['ticket_number']}" for t in late[:20])
    more = f" dan {len(late) - 20} lainnya" if len(late) > 20 else ""
    title = "Laporan realisasi vendor terlambat"
    body = (f"{len(late)} tiket replenish lewat H+2 hari kerja tanpa laporan realisasi: {shown}{more}.")[:1000]
    users = await conn.fetch("""
        SELECT u.id, u.email FROM users u JOIN roles r ON r.id = u.role_id
        WHERE r.role = ANY($1::text[]) AND u.is_active AND u.deleted_at IS NULL ORDER BY u.id""", LATE_ROLES)
    seen: set[str] = set()
    for u in users:
        nid = await conn.fetchval("""
            INSERT INTO notifications (recipient_user_id, type, title, body, link)
            VALUES ($1, 'replenishment.vendor_report_late', $2, $3, '/replenishment') RETURNING id""", u["id"], title, body)
        email = (u["email"] or "").strip().lower()
        if email and "@" in email and email not in seen:
            seen.add(email)
            await conn.execute("""
                INSERT INTO notification_emails (notification_id, to_address, subject, body, status)
                VALUES ($1, $2, $3, $4, $5)""", nid, email, title, body + EMAIL_FOOTER,
                "pending" if email_enabled else "skipped")
    await conn.execute("UPDATE replenish_classifications SET late_notified_at = now() WHERE ticket_id = ANY($1::bigint[])",
                       [t["id"] for t in late])
    return len(late)


async def run_once(pool: asyncpg.Pool, today: date, start_date: date | None, *, trigger: str,
                   initiated_by: str = "system", email_enabled: bool = False) -> Summary | None:
    """One run with its retry_audit_logs row. None when no start date is configured (no backfill, K4)."""
    if start_date is None:
        logger.info("replenish_classify skipped: no start date configured")
        return None
    started = time_module.monotonic()
    try:
        async with pool.acquire() as conn, conn.transaction():
            s = await classify(conn, today, start_date, email_enabled=email_enabled)
        outcome, error = ("skipped" if s.skipped else "completed"), None
    except Exception as e:  # logged + audited, never crashes the scheduler
        logger.exception("replenish_classify failed")
        s, outcome, error = None, "failed", f"{type(e).__name__}: {e}"
    duration = int((time_module.monotonic() - started) * 1000)
    async with pool.acquire() as conn:
        await conn.execute("""
            INSERT INTO retry_audit_logs (event_type, trigger_type, file_type, processing_date, initiated_by,
                                          outcome, duration_ms, error_detail)
            VALUES ('classification_run', $1, $2, $3, $4, $5, $6, $7)""",
            trigger, FILE_TYPE, today, initiated_by, outcome, duration, error)
    logger.info("replenish_classify %s in %dms: %s", outcome, duration, s)
    return s
