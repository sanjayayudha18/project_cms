"""Integration test for replenish_classify.job against real Postgres with migration 031
(replenishment-realisasi FR3.2-3.8, K2, K3). Everything runs in one transaction rolled back at the
end; all dates are in 2099 so no dev ticket or ITM row is in scope.

Run from backend_python/:  python -m unittest replenish_classify.test_job_integration -v
Skipped when DATABASE_URL is empty (use localhost, not host.docker.internal).
"""
from __future__ import annotations

import os
import unittest
import uuid
from datetime import date, time

import asyncpg

from replenish_classify.job import classify

DATABASE_URL = os.environ.get("DATABASE_URL", "")
START = date(2099, 1, 1)
WED, THU, FRI, MON12, TUE6, THU15 = (date(2099, 1, d) for d in (7, 8, 9, 12, 6, 15))


@unittest.skipUnless(DATABASE_URL, "DATABASE_URL not set")
class ClassifyJobTest(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self) -> None:
        self.conn = await asyncpg.connect(DATABASE_URL)
        self.tx = self.conn.transaction()
        await self.tx.start()
        tag = uuid.uuid4().hex[:8].upper()
        self.t = {i: f"ITRC{tag}{i}" for i in range(1, 7)}
        c = self.conn
        user = await c.fetchval("SELECT min(id) FROM users")
        self.number = f"REP-ITRC-{tag}-20990107-001"
        self.req = await c.fetchval("""INSERT INTO vendor_requests (request_number, forecast_date, created_by, status)
            VALUES ($1, $2, $3, 'approved') RETURNING id""", self.number, WED, user)
        orders = {1: 200_000_000, 2: 100_000_000, 3: 50_000_000, 4: 100_000_000, 6: 100_000_000}
        self.ticket = {}
        for i, amount in orders.items():
            await c.execute("""INSERT INTO vendor_request_items (vendor_request_id, terminal_id, periode_pred, denom, amount_replenish)
                VALUES ($1, $2, $3, 100000, $4)""", self.req, self.t[i], WED, amount)
            self.ticket[i] = await c.fetchval("""INSERT INTO vendor_request_tickets (request_number, terminal_id, denom_code, replenish_date, seq, ticket_number)
                VALUES ($1, $2, '100K', $3, 1, $2 || '_100K_20990107_001') RETURNING id""", self.number, self.t[i], WED)
        await c.execute("INSERT INTO holidays (holiday_date, name, created_by) VALUES ($1, 'Libur uji', $2)", THU, user)
        file_id = await c.fetchval("INSERT INTO itm_replenish_files (filename, file_date) VALUES ($1, $2) RETURNING id",
                                   f"it-rc-{tag}.csv", WED)

        async def fill(i: int, d: date, amount: int) -> None:
            await c.execute("""INSERT INTO itm_replenish (file_id, replenish_date, replenish_time, terminal_id, machine_type,
                teller_id, branch_code, replenish_denom_100k) VALUES ($1, $2, $3, $4, 'ATM', 'T', 'B', $5)""",
                file_id, d, time(9), self.t[i], amount)

        await fill(1, WED, 200_000_000)   # on schedule
        await fill(2, FRI, 100_000_000)   # Thu is a holiday -> 1 workday late
        await fill(2, MON12, 100_000_000)  # 2 workdays after Fri: outside T2's window -> no_order
        await fill(3, TUE6, 40_000_000)   # 1 day early, short
        await fill(5, WED, 70_000_000)    # nobody ordered T5 -> no_order
        # T1: vendor reported success with a different amount -> vendor_mismatch, not late.
        await c.execute("UPDATE vendor_request_tickets SET result = 'success' WHERE id = $1", self.ticket[1])
        rid = await c.fetchval("""INSERT INTO vendor_request_realizations (ticket_id, vendor_request_id, realized_date, submitted_by)
            VALUES ($1, $2, $3, $4) RETURNING id""", self.ticket[1], self.req, WED, user)
        await c.execute("INSERT INTO vendor_request_realization_amounts (realization_id, denom, amount) VALUES ($1, 100000, 190000000)", rid)

    async def asyncTearDown(self) -> None:
        await self.tx.rollback()
        await self.conn.close()

    async def rows(self) -> dict[str, asyncpg.Record]:
        recs = await self.conn.fetch("""SELECT c.*, t.terminal_id AS tt FROM replenish_classifications c
            LEFT JOIN vendor_request_tickets t ON t.id = c.ticket_id WHERE c.terminal_id = ANY($1::text[])""",
            list(self.t.values()))
        return {f"{r['terminal_id']}|{r['classification']}" if r["ticket_id"] is None else r["terminal_id"]: r for r in recs}

    async def test_classify_pending_then_final_idempotent(self) -> None:
        c = self.conn
        s = await classify(c, THU, START, email_enabled=False)
        rows = await self.rows()
        self.assertEqual(s.tickets, 5)
        self.assertEqual({k: r["classification"] for k, r in rows.items()},
                         {self.t[i]: "pending" for i in (1, 2, 3, 4, 6)})  # windows still open, no no_order yet
        self.assertEqual(rows[self.t[1]]["realized_date"], WED)

        # K2: a ticket deactivated before its window closed becomes void.
        await c.execute("UPDATE vendor_request_tickets SET is_active = false, deactivated_at = now() WHERE id = $1", self.ticket[6])
        s = await classify(c, THU15, START, email_enabled=False)
        rows = await self.rows()
        got = {k: (r["classification"], r["workday_offset"], r["has_amount_diff"], r["vendor_mismatch"], r["vendor_report_late"])
               for k, r in rows.items()}
        self.assertEqual(got, {
            self.t[1]: ("on_schedule", 0, False, True, False),
            self.t[2]: ("late", 1, False, False, True),
            self.t[3]: ("early", -1, True, False, True),
            self.t[4]: ("not_done", None, False, False, True),
            self.t[6]: ("void", None, False, False, False),
            f"{self.t[2]}|no_order": ("no_order", None, False, False, False),
            f"{self.t[5]}|no_order": ("no_order", None, False, False, False),
        })
        self.assertTrue(all(r["is_final"] for r in rows.values()))
        amounts = await c.fetch("SELECT denom, order_amount, itm_amount, vendor_amount, diff FROM replenish_classification_amounts WHERE classification_id = $1",
                                rows[self.t[3]]["id"])
        self.assertEqual([tuple(a) for a in amounts], [(100000, 50_000_000, 40_000_000, None, -10_000_000)])
        self.assertEqual(s.late_notified, 3)
        late_notes = await c.fetchval("SELECT count(*) FROM notifications WHERE type = 'replenishment.vendor_report_late' AND body LIKE '%' || $1 || '%'", self.t[4])
        spvs = await c.fetchval("""SELECT count(*) FROM users u JOIN roles r ON r.id = u.role_id
            WHERE r.role IN ('ATM-SPV', 'BRANCH-ATM-SPV') AND u.is_active AND u.deleted_at IS NULL""")
        self.assertEqual(late_notes, spvs)

        # Final rows are never recomputed; a second pass adds nothing and notifies nobody again.
        before = {k: r["classified_at"] for k, r in rows.items()}
        s = await classify(c, THU15, START, email_enabled=False)
        self.assertEqual((s.tickets, s.no_order, s.late_notified), (0, 0, 0))
        self.assertEqual({k: r["classified_at"] for k, r in (await self.rows()).items()}, before)

        # Review R4: a vendor report after the row went final updates only the vendor columns.
        await c.execute("UPDATE vendor_request_tickets SET result = 'success' WHERE id = $1", self.ticket[4])
        rid = await c.fetchval("""INSERT INTO vendor_request_realizations (ticket_id, vendor_request_id, realized_date, submitted_by, submitted_at)
            VALUES ($1, $2, $3, (SELECT min(id) FROM users), now() + interval '1 minute') RETURNING id""", self.ticket[4], self.req, WED)
        await c.execute("INSERT INTO vendor_request_realization_amounts (realization_id, denom, amount) VALUES ($1, 100000, 100000000)", rid)
        s = await classify(c, THU15, START, email_enabled=False)
        self.assertEqual(s.vendor_refreshed, 1)  # T1's report predates its final row
        r4 = (await self.rows())[self.t[4]]
        self.assertEqual((r4["classification"], r4["vendor_mismatch"], r4["is_final"]), ("not_done", True, True))
        self.assertEqual(await c.fetchval("SELECT vendor_amount FROM replenish_classification_amounts WHERE classification_id = $1",
                                          r4["id"]), 100_000_000)

    async def test_fill_moves_and_rejected_request_voids_after_window(self) -> None:
        c = self.conn
        await classify(c, THU, START, email_enabled=False)  # T3 pending, holding the Tue fill
        # Review R2: T3 deactivated while a second request schedules the same ATM on Tue -> the fill
        # leaves the voided row and moves to the new ticket.
        await c.execute("UPDATE vendor_request_tickets SET is_active = false, deactivated_at = now() WHERE id = $1", self.ticket[3])
        number2 = self.number[:-3] + "002"
        req2 = await c.fetchval("""INSERT INTO vendor_requests (request_number, forecast_date, created_by, status)
            VALUES ($1, $2, (SELECT min(id) FROM users), 'approved') RETURNING id""", number2, TUE6)
        await c.execute("""INSERT INTO vendor_request_items (vendor_request_id, terminal_id, periode_pred, denom, amount_replenish)
            VALUES ($1, $2, $3, 100000, 40000000)""", req2, self.t[3], TUE6)
        moved = await c.fetchval("""INSERT INTO vendor_request_tickets (request_number, terminal_id, denom_code, replenish_date, seq, ticket_number)
            VALUES ($1, $2, '100K', $3, 1, $2 || '_100K_20990106_001') RETURNING id""", number2, self.t[3], TUE6)
        # Review R3: the vendor rejects the first request -> its rows wait while the window is open...
        await c.execute("UPDATE vendor_requests SET status = 'vendor_rejected' WHERE id = $1", self.req)
        await classify(c, THU, START, email_enabled=False)
        by_ticket = {r["ticket_id"]: r for r in await c.fetch("SELECT * FROM replenish_classifications WHERE ticket_id = ANY($1::bigint[])",
                                                               [*self.ticket.values(), moved])}
        self.assertEqual(by_ticket[self.ticket[3]]["classification"], "void")
        self.assertIsNone(by_ticket[self.ticket[3]]["itm_replenish_id"])
        self.assertEqual((by_ticket[moved]["classification"], by_ticket[moved]["realized_date"]), ("pending", TUE6))
        self.assertEqual({by_ticket[self.ticket[i]]["classification"] for i in (1, 2, 4, 6)}, {"pending"})
        # ...and become void once it closed; the fill they held is then nobody's order.
        s = await classify(c, THU15, START, email_enabled=False)
        rows = await self.rows()
        self.assertEqual({rows[self.t[i]]["classification"] for i in (1, 2, 4, 6)}, {"void"})
        self.assertEqual(rows[f"{self.t[1]}|no_order"]["realized_date"], WED)
        moved_row = await c.fetchrow("SELECT classification, workday_offset FROM replenish_classifications WHERE ticket_id = $1", moved)
        self.assertEqual(tuple(moved_row), ("on_schedule", 0))
        self.assertEqual(s.late_notified, 1)  # only the moved ticket: void rows are never "late"


if __name__ == "__main__":
    unittest.main()
