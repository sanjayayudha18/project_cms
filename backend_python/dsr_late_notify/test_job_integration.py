"""Integration test for dsr_late_notify against real Postgres with migration 033
(dsr-late-report spec FR5, K3, plan P6/R2/R3). Seeds the shared fixture
backend/internal/repository/testdata/dsr_late_cases.json the same way the Go repository test
does; everything except the run_once failure test runs in one transaction rolled back at the end.

Run from backend_python/:  python -m unittest discover -s dsr_late_notify -t . -v
Skipped when DATABASE_URL is empty (use localhost, not host.docker.internal). Use a throwaway DB.
"""
from __future__ import annotations

import hashlib
import json
import os
import time as time_module
import unittest
import uuid
from datetime import date, datetime
from pathlib import Path
from unittest import mock

import asyncpg

from dsr_late_notify import job
from dsr_late_notify.queries import UNSENT_BRANCHES

DATABASE_URL = os.environ.get("DATABASE_URL", "")
FIXTURE = Path(__file__).resolve().parents[2] / "backend/internal/repository/testdata/dsr_late_cases.json"
CASES = json.loads(FIXTURE.read_text(encoding="utf-8"))
X = date(2026, 10, 5)
AT_0900_01 = datetime.fromisoformat("2026-10-05T09:00:01+07:00")
# Go backend/queries/notifications.sql ListNotificationsForUser (R3: Go reads what Python writes).
GO_LIST_NOTIFICATIONS = """
SELECT id, type, title, body, link, is_read, created_at
FROM notifications
WHERE recipient_user_id = $1 AND (NOT $2::boolean OR is_read = false)
ORDER BY created_at DESC, id DESC
LIMIT $3 OFFSET $4"""


async def seed(c: asyncpg.Connection, tag: str) -> dict[str, dict[str, int]]:
    """Mirror of seedDsrLateCases (Go): tag appended to codes/names/labels, checksum = sha256(tag + key)."""
    ids: dict[str, dict[str, int]] = {"vendor": {}, "branch": {}, "vault": {}}
    ts = datetime.fromisoformat
    for v in CASES["vendors"]:
        ids["vendor"][v["key"]] = await c.fetchval(
            "INSERT INTO vendors (code, name, kind) VALUES ($1, $1, $2) RETURNING id", f"DLR_{v['key']}_{tag}", v["kind"])
    for b in CASES["branches"]:
        ids["branch"][b["key"]] = await c.fetchval("""
            INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, is_active, deleted_at, created_at, region_code)
            VALUES ($1, $2, $2, $3, $4, $5, $6) RETURNING id""",
            ids["vendor"][b["vendor"]], f"{b['key']}_{tag}", b["is_active"],
            ts(b["deleted_at"]) if b.get("deleted_at") else None, ts(b["created_at"]), b.get("region_code"))
    for v in CASES["vaults"]:
        ids["vault"][v["key"]] = await c.fetchval("""
            INSERT INTO vendor_vaults (vendor_branch_id, vault_code, type, category, currency_code, is_active, created_at)
            VALUES ($1, $2, 'CASH', 'CASH', 'IDR', $3, $4) RETURNING id""",
            ids["branch"][v["branch"]], f"{v['key']}_{tag}", v["is_active"], ts(v["created_at"]))
    for m in CASES["maps"]:
        await c.execute("INSERT INTO dsr_location_vault_maps (vendor_id, dsr_location, vendor_vault_id) VALUES ($1, $2, $3)",
                        ids["vendor"][m["vendor"]], f"{m['label']} {tag}", ids["vault"][m["vault"]])
    for r in CASES["receipts"]:
        vendor_id = ids["vendor"][r["vendor"]] if r["vendor"] else None
        label = f"DLR_{r['vendor']}_{tag}" if r["vendor"] else f"{r['vendor_label']} {tag}"
        await c.execute("""
            INSERT INTO dsr_receipts (vendor_label, vendor_id, report_date, checksum, locations, received_at)
            VALUES ($1, $2, $3, $4, $5, $6)""",
            label, vendor_id, date.fromisoformat(r["report_date"]), hashlib.sha256((tag + r["key"]).encode()).hexdigest(),
            [f"{loc} {tag}" for loc in r["locations"]], ts(r["received_at"]))
    return ids


@unittest.skipUnless(DATABASE_URL, "DATABASE_URL not set")
class DsrLateNotifyTest(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self) -> None:
        self.conn = await asyncpg.connect(DATABASE_URL)
        self.tx = self.conn.transaction()
        await self.tx.start()
        self.tag = uuid.uuid4().hex[:8].upper()
        self.ids = await seed(self.conn, self.tag)
        self.branch_key = {v: k for k, v in self.ids["branch"].items()}

    async def asyncTearDown(self) -> None:
        await self.tx.rollback()
        await self.conn.close()

    async def unsent(self, report_date: date, as_of: datetime) -> set[str]:
        fixture_vendors = set(self.ids["vendor"].values())
        return {self.branch_key[r["vendor_branch_id"]] for r in await self.conn.fetch(UNSENT_BRANCHES, report_date, as_of)
                if r["vendor_id"] in fixture_vendors}

    async def user(self, role: str, vendor: str | None = None, branch: str | None = None, active: bool = True) -> int:
        n = uuid.uuid4().hex[:8]
        return await self.conn.fetchval("""
            INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source, vendor_id, vendor_branch_id, is_active)
            VALUES ((SELECT id FROM roles WHERE role = $1), $2, $2, $3, false, 'local', $4, $5, $6) RETURNING id""",
            role, f"dlr_{n}", f"Dlr.{n}@Vendor.Test", self.ids["vendor"].get(vendor), self.ids["branch"].get(branch), active)

    async def pic(self, email: str, roles: list[str], branch: str | None = None, recipient: bool = True) -> None:
        await self.conn.execute("""
            INSERT INTO vendor_pics (vendor_id, vendor_branch_id, name, email, is_notification_recipient, roles)
            VALUES ($1, $2, 'PIC', $3, $4, $5)""",
            self.ids["vendor"]["FLM"], self.ids["branch"].get(branch), email, recipient, roles)

    async def notify(self, as_of: datetime = AT_0900_01, email_enabled: bool = False) -> job.Summary:
        return await job.notify(self.conn, X, as_of=as_of, email_enabled=email_enabled, trigger="manual",
                                initiated_by="test", started=time_module.monotonic())

    # R2: the Python copy of the SQL gives the Go report's "no receipt" rows for the shared cases.
    async def test_unsent_matches_shared_fixture(self):
        for sc in CASES["scenarios"]:
            as_of = datetime.fromisoformat(sc["as_of"])
            dates = sorted({e["report_date"] for e in sc["expected"]})
            for d in dates:
                with self.subTest(scenario=sc["name"], date=d):
                    want = {e["branch"] for e in sc["expected"]
                            if e["report_date"] == d and e["status"] in ("pending", "not_sent")}
                    self.assertEqual(await self.unsent(date.fromisoformat(d), as_of), want)

    async def test_recipients_idempotent_and_readable_by_go(self):
        c = self.conn
        branch_ok = await self.user("VENDOR-ATM-USER", "FLM", "OK")
        wide = await self.user("VENDOR-USER", "FLM")
        vault_wide = await self.user("VENDOR-VAULT-USER", "FLM")
        sent_branch = await self.user("VENDOR-USER", "FLM", "NEW")       # NEW already sent: nothing
        inactive = await self.user("VENDOR-USER", "FLM", active=False)
        roh = await self.user("VENDOR-USER", "ROH")
        spv = await self.user("ATM-SPV")
        atm_user = await self.user("ATM-USER")
        await self.pic("pic.ok@vendor.test", ["vault"], "OK")
        await self.pic("PIC.Wide@Vendor.Test", ["replenish"])
        await self.pic("pic.cash@vendor.test", ["cashcount"])
        await self.pic("pic.off@vendor.test", ["replenish", "vault"], recipient=False)
        await self.pic("pic.new@vendor.test", ["replenish"], "NEW")

        s = await self.notify()
        self.assertEqual((s.status, s.branches), ("sent", 3))  # OK, PARTIAL, VAULTNEW (fixture)
        rows = {r["recipient_user_id"]: r for r in await c.fetch(
            "SELECT * FROM notifications WHERE type = 'dsr_late' AND created_at = now()")}
        for uid in (sent_branch, inactive, roh, spv):
            self.assertNotIn(uid, rows)
        t = self.tag
        self.assertIn(f"OK_{t} - OK_{t}.", rows[branch_ok]["body"])
        self.assertNotIn(f"PARTIAL_{t}", rows[branch_ok]["body"])
        for uid in (wide, vault_wide):
            self.assertIn(f"(3 cabang)", rows[uid]["title"])
            for key in ("OK", "PARTIAL", "VAULTNEW"):
                self.assertIn(f"{key}_{t} - {key}_{t}", rows[uid]["body"])
        self.assertIsNone(rows[wide]["link"])
        internal = await c.fetch("""SELECT u.id FROM users u JOIN roles r ON r.id = u.role_id
            WHERE r.role = 'ATM-USER' AND u.is_active AND u.deleted_at IS NULL""")
        for r in internal:
            self.assertEqual(rows[r["id"]]["link"], "/dsr-late-report")
        self.assertIn(f"DLR_FLM_{t} (3 cabang)", rows[atm_user]["body"])
        self.assertEqual(s.internal_notifications, len(internal))
        self.assertEqual(s.vendor_notifications, 3)

        emails = {r["to_address"]: r for r in await c.fetch("SELECT * FROM notification_emails WHERE created_at = now()")}
        self.assertEqual({r["status"] for r in emails.values()}, {"skipped"})  # SMTP off
        self.assertIsNone(emails["pic.ok@vendor.test"]["notification_id"])
        self.assertIn(f"OK_{t} - OK_{t}.", emails["pic.ok@vendor.test"]["body"])
        self.assertIn("(3 cabang)", emails["pic.wide@vendor.test"]["subject"])
        for gone in ("pic.cash@vendor.test", "pic.off@vendor.test", "pic.new@vendor.test"):
            self.assertNotIn(gone, emails)
        self.assertEqual(s.pic_emails, 2)
        self.assertTrue(emails["pic.wide@vendor.test"]["body"].endswith("Mohon tidak membalas email ini."))

        go = await c.fetch(GO_LIST_NOTIFICATIONS, wide, True, 20, 0)
        self.assertEqual((go[0]["type"], go[0]["title"], go[0]["is_read"]), ("dsr_late", rows[wide]["title"], False))

        # FR5.6: a second run for the date sends nothing and records a skipped run.
        before = await c.fetchval("SELECT count(*) FROM notifications WHERE type = 'dsr_late'")
        self.assertEqual((await self.notify()).status, "already_sent")
        self.assertEqual(await c.fetchval("SELECT count(*) FROM notifications WHERE type = 'dsr_late'"), before)
        audit = await c.fetch("""SELECT event_type, outcome FROM retry_audit_logs
            WHERE file_type = 'dsr_late_notify' AND processing_date = $1 AND created_at = now() ORDER BY outcome""", X)
        self.assertEqual([tuple(r) for r in audit], [("dsr_late_notify", "skipped"), ("dsr_late_notify", "success")])
        self.assertEqual(await c.fetchval("SELECT count(*) FROM audit_logs WHERE created_at = now()"), 0)

    async def test_email_pending_when_smtp_on(self):
        await self.pic("pic.wide@vendor.test", ["vault"])
        self.assertEqual((await self.notify(email_enabled=True)).status, "sent")
        self.assertEqual(await self.conn.fetchval(
            "SELECT status FROM notification_emails WHERE to_address = 'pic.wide@vendor.test' AND created_at = now()"), "pending")

    async def test_nothing_unsent_still_writes_success_row(self):
        # before any fixture vault exists: no fixture branch is obliged, the run is still recorded once.
        d = date(2025, 6, 1)
        self.assertEqual(await self.unsent(d, AT_0900_01), set())
        s = await job.notify(self.conn, d, as_of=AT_0900_01, email_enabled=False, trigger="auto",
                             initiated_by="system", started=time_module.monotonic())
        self.assertEqual(s.status, "sent")
        self.assertEqual(await self.conn.fetchval("""SELECT count(*) FROM retry_audit_logs
            WHERE file_type = 'dsr_late_notify' AND processing_date = $1 AND outcome = 'success'""", d), 1)

    async def test_failure_rolls_back_every_row(self):
        c = self.conn
        uid = await self.user("VENDOR-USER", "FLM")
        await self.pic("pic.wide@vendor.test", ["vault"])
        sp = c.transaction()
        await sp.start()
        with mock.patch("dsr_late_notify.job.internal_message", side_effect=RuntimeError("boom")):
            with self.assertRaises(RuntimeError):
                await self.notify()  # fails after the vendor rows were written
        await sp.rollback()
        self.assertEqual(await c.fetchval("SELECT count(*) FROM notifications WHERE recipient_user_id = $1", uid), 0)
        self.assertEqual(await c.fetchval(
            "SELECT count(*) FROM notification_emails WHERE to_address = 'pic.wide@vendor.test'"), 0)
        self.assertEqual(await c.fetchval("""SELECT count(*) FROM retry_audit_logs
            WHERE file_type = 'dsr_late_notify' AND processing_date = $1 AND created_at = now()""", X), 0)
        self.assertEqual((await self.notify()).status, "sent")  # a later run may try again


@unittest.skipUnless(DATABASE_URL, "DATABASE_URL not set")
class RunOnceTest(unittest.IsolatedAsyncioTestCase):
    """run_once guards + the failed row in its own tx. Dates in 2099: no real branch is in scope."""

    DAY = date(2099, 1, 5)
    AS_OF = datetime.fromisoformat("2099-01-05T09:00:00+07:00")

    async def asyncSetUp(self) -> None:
        self.pool = await asyncpg.create_pool(DATABASE_URL, min_size=1, max_size=2)

    async def asyncTearDown(self) -> None:
        # test-only cleanup of the audit rows these runs committed
        await self.pool.execute("DELETE FROM retry_audit_logs WHERE file_type = 'dsr_late_notify' AND processing_date >= '2099-01-01'")
        await self.pool.close()

    async def audit(self) -> list[str]:
        return [r["outcome"] for r in await self.pool.fetch(
            "SELECT outcome FROM retry_audit_logs WHERE file_type = 'dsr_late_notify' AND processing_date = $1", self.DAY)]

    async def test_no_start_date_and_before_start_do_nothing(self):
        self.assertIsNone(await job.run_once(self.pool, self.DAY, None, trigger="auto", as_of=self.AS_OF))
        s = await job.run_once(self.pool, self.DAY, date(2099, 1, 6), trigger="auto", as_of=self.AS_OF)
        self.assertEqual(s.status, "before_start")
        self.assertEqual(await self.audit(), [])

    async def test_future_date_is_refused(self):
        with self.assertRaises(ValueError):
            await job.run_once(self.pool, date(2099, 1, 6), date(2099, 1, 1), trigger="manual", as_of=self.AS_OF)

    async def test_failure_writes_failed_row_and_nothing_else(self):
        with mock.patch("dsr_late_notify.job.notify", side_effect=RuntimeError("boom")):
            s = await job.run_once(self.pool, self.DAY, date(2099, 1, 1), trigger="auto", as_of=self.AS_OF)
        self.assertEqual((s.status, s.error), ("failed", "RuntimeError: boom"))
        self.assertEqual(await self.audit(), ["failed"])


if __name__ == "__main__":
    unittest.main()
