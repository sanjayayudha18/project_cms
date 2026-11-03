"""09:00 WIB notification for vendor branches without a DSR (.claude/sdlc/dsr-late-report spec FR5).

One transaction per run under an advisory lock: unsent branches (queries.UNSENT_BRANCHES, the
Go report's SQL) -> one notification per vendor recipient listing their branches, email-only
for PICs, one summary per ATM-USER -> retry_audit_logs success row. Idempotent per report date
(a success row for the date means nothing is sent again, FR5.6/5.7). K3: writes the same
notifications + notification_emails rows Go notification.Send writes; the Go worker sends the
email. System action: retry_audit_logs + log, never audit_logs (FR5.8).
"""
from __future__ import annotations

import logging
import time as time_module
from dataclasses import dataclass
from datetime import date, datetime, time

import asyncpg

from dsr_late_notify import queries
from dsr_late_notify.message import (
    EMAIL_FOOTER,
    INTERNAL_LINK,
    internal_message,
    normalize_email,
    vendor_message,
)
from lib.utils.timezone import WIB

logger = logging.getLogger(__name__)

JOB = "dsr_late_notify"  # retry_audit_logs.event_type + file_type, advisory lock key
NOTIFICATION_TYPE = "dsr_late"
DEADLINE = time(9, 0)  # WIB, spec K1
VENDOR_USER_ROLES = ["VENDOR-USER", "VENDOR-ATM-USER", "VENDOR-VAULT-USER"]
PIC_ROLES = ["replenish", "vault"]
INTERNAL_ROLES = ["ATM-USER"]


@dataclass(frozen=True)
class Summary:
    status: str  # sent | already_sent | before_start | failed
    branches: int = 0
    vendor_notifications: int = 0
    pic_emails: int = 0
    internal_notifications: int = 0
    error: str | None = None


class _Outbox:
    """Inserts notifications + outbox rows; one email per (address, body) like Send's per-call dedupe."""

    def __init__(self, conn: asyncpg.Connection, email_enabled: bool):
        self.conn = conn
        self.status = "pending" if email_enabled else "skipped"
        self.seen: set[tuple[str, str]] = set()

    async def notify(self, user_id: int, email: str | None, title: str, body: str, link: str | None) -> None:
        nid = await self.conn.fetchval("""
            INSERT INTO notifications (recipient_user_id, type, title, body, link)
            VALUES ($1, $2, $3, $4, $5) RETURNING id""", user_id, NOTIFICATION_TYPE, title, body, link)
        await self.email(nid, email, title, body)

    async def email(self, notification_id: int | None, raw: str | None, title: str, body: str) -> bool:
        addr = normalize_email(raw)
        if addr is None or (addr, body) in self.seen:
            return False
        self.seen.add((addr, body))
        await self.conn.execute("""
            INSERT INTO notification_emails (notification_id, to_address, subject, body, status)
            VALUES ($1, $2, $3, $4, $5)""", notification_id, addr, title, body + EMAIL_FOOTER, self.status)
        return True


async def _audit(conn: asyncpg.Connection, report_date: date, trigger: str, initiated_by: str,
                 outcome: str, started: float, error: str | None = None) -> None:
    await conn.execute("""
        INSERT INTO retry_audit_logs (event_type, trigger_type, file_type, processing_date, initiated_by,
                                      outcome, duration_ms, error_detail)
        VALUES ($1, $2, $1, $3, $4, $5, $6, $7)""",
        JOB, trigger, report_date, initiated_by, outcome, int((time_module.monotonic() - started) * 1000), error)


async def notify(conn: asyncpg.Connection, report_date: date, *, as_of: datetime, email_enabled: bool,
                 trigger: str, initiated_by: str, started: float) -> Summary:
    """Runs one notification pass inside the caller's transaction (rolls back with it)."""
    await conn.execute("SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", JOB)
    if await conn.fetchval("""SELECT EXISTS(SELECT 1 FROM retry_audit_logs
            WHERE file_type = $1 AND processing_date = $2 AND outcome = 'success')""", JOB, report_date):
        await _audit(conn, report_date, trigger, initiated_by, "skipped", started, "already sent")
        return Summary(status="already_sent")

    unsent = await conn.fetch(queries.UNSENT_BRANCHES, report_date, as_of)
    out = _Outbox(conn, email_enabled)
    vendor_n = pic_n = internal_n = 0
    if unsent:
        order = {r["vendor_branch_id"]: i for i, r in enumerate(unsent)}
        label = {r["vendor_branch_id"]: (r["branch_code"], r["branch_name"]) for r in unsent}
        ids = list(order)

        users: dict[int, tuple[str, set[int]]] = {}
        for r in await conn.fetch(queries.VENDOR_USERS, ids, VENDOR_USER_ROLES):
            users.setdefault(r["user_id"], (r["email"], set()))[1].add(r["vendor_branch_id"])
        for uid in sorted(users):
            email, branch_ids = users[uid]
            title, body = vendor_message(report_date, [label[b] for b in sorted(branch_ids, key=order.get)])
            await out.notify(uid, email, title, body, None)
            vendor_n += 1

        pics: dict[str, set[int]] = {}
        for r in await conn.fetch(queries.VENDOR_PIC_EMAILS, ids, PIC_ROLES):
            addr = normalize_email(r["email"])
            if addr:
                pics.setdefault(addr, set()).add(r["vendor_branch_id"])
        for addr in sorted(pics):
            title, body = vendor_message(report_date, [label[b] for b in sorted(pics[addr], key=order.get)])
            pic_n += await out.email(None, addr, title, body)

        per_vendor: dict[str, int] = {}
        for r in unsent:
            per_vendor[r["vendor_name"]] = per_vendor.get(r["vendor_name"], 0) + 1
        title, body = internal_message(report_date, list(per_vendor.items()))
        for r in await conn.fetch(queries.ROLE_USERS, INTERNAL_ROLES):
            await out.notify(r["user_id"], r["email"], title, body, INTERNAL_LINK)
            internal_n += 1

    await _audit(conn, report_date, trigger, initiated_by, "success", started)
    return Summary(status="sent", branches=len(unsent), vendor_notifications=vendor_n,
                   pic_emails=pic_n, internal_notifications=internal_n)


async def run_once(pool: asyncpg.Pool, report_date: date, start_date: date | None, *, trigger: str,
                   initiated_by: str = "system", email_enabled: bool = False,
                   as_of: datetime | None = None) -> Summary | None:
    """One run. None when no start date is configured (FR5.1); nothing before it (no backfill, #16)."""
    if start_date is None:
        logger.info("dsr_late_notify skipped: no start date configured")
        return None
    as_of = as_of or datetime.now(WIB)
    if report_date > as_of.astimezone(WIB).date():
        raise ValueError(f"report date {report_date} is in the future")
    # Before the deadline every branch without a receipt yet would be told "missing", and the
    # success row would then suppress the real 09:00 run (review R2, spec FR5.7).
    if as_of < datetime.combine(report_date, DEADLINE, WIB):
        raise ValueError(f"as_of {as_of.isoformat()} is before the {DEADLINE:%H:%M} WIB deadline of {report_date}")
    if report_date < start_date:
        logger.info("dsr_late_notify skipped: %s is before start date %s", report_date, start_date)
        return Summary(status="before_start")
    started = time_module.monotonic()
    try:
        async with pool.acquire() as conn, conn.transaction():
            s = await notify(conn, report_date, as_of=as_of, email_enabled=email_enabled,
                             trigger=trigger, initiated_by=initiated_by, started=started)
    except Exception as e:  # rolled back; logged + audited, never crashes the scheduler
        logger.exception("dsr_late_notify failed for %s", report_date)
        s = Summary(status="failed", error=f"{type(e).__name__}: {e}")
        async with pool.acquire() as conn:
            await _audit(conn, report_date, trigger, initiated_by, "failed", started, s.error)
    logger.info("dsr_late_notify %s for %s (trigger=%s): %s", s.status, report_date, trigger, s)
    return s
