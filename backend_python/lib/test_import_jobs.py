"""Integration tests for lib/import_jobs.py against a real Postgres with migration 019
(.claude/sdlc/import-export-jobs spec A1-A9, A4a, A5a, A16).

Skipped when DATABASE_URL is empty. Most tests run inside a transaction that is rolled
back; the concurrency and staleness tests must commit (two connections / real clock) and
delete their rows afterwards. All rows use a `test_ij_*` source and dates in 2099.

Run from backend_python/:  python -m unittest lib.test_import_jobs -v
"""
from __future__ import annotations

import asyncio
import os
import unittest
import uuid
from datetime import date, timedelta

import asyncpg

from lib import import_jobs as ij

DATABASE_URL = os.environ.get("DATABASE_URL", "")
DAY = date(2099, 1, 1)


def h(c: str) -> str:
    return c * 64


@unittest.skipUnless(DATABASE_URL, "DATABASE_URL not set")
class ImportJobsTest(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self) -> None:
        self.conn = await asyncpg.connect(DATABASE_URL)
        self.tx = self.conn.transaction()
        await self.tx.start()
        self.source = f"test_ij_{uuid.uuid4().hex[:12]}"

    async def asyncTearDown(self) -> None:
        await self.tx.rollback()
        await self.conn.close()

    async def reg(self, c: str, day: date | None = DAY, **kw):
        return await ij.register(
            self.conn, source=self.source, processing_date=day, file_hash=h(c),
            original_filename=f"{c}.csv", **kw,
        )

    async def done(self, c: str, day: date | None = DAY):
        job, _ = await self.reg(c, day)
        await ij.start(self.conn, job["id"])
        return await ij.complete(self.conn, job["id"], row_count=3, error_count=0)

    async def status(self, job_id: int) -> str:
        return await self.conn.fetchval("SELECT status FROM import_jobs WHERE id = $1", job_id)

    # A1
    async def test_register_new_file_is_pending_v1(self):
        job, dup = await self.reg("a")
        self.assertFalse(dup)
        self.assertEqual((job["status"], job["version"], job["runtime"]), ("pending", 1, "python"))
        self.assertIsNone(job["supersedes_job_id"])

    # A2
    async def test_same_hash_when_completed_is_duplicate_without_new_row(self):
        a = await self.done("a")
        again, dup = await self.reg("a")
        self.assertTrue(dup)
        self.assertEqual(again["id"], a["id"])
        n = await self.conn.fetchval("SELECT count(*) FROM import_jobs WHERE source = $1", self.source)
        self.assertEqual(n, 1)

    async def test_same_hash_while_pending_is_duplicate(self):
        a, _ = await self.reg("a")
        again, dup = await self.reg("a")
        self.assertTrue(dup)
        self.assertEqual(again["id"], a["id"])

    # A3
    async def test_failed_hash_reuses_row_back_to_pending(self):
        a, _ = await self.reg("a")
        await ij.start(self.conn, a["id"])
        await ij.fail(self.conn, a["id"], "boom")
        again, dup = await self.reg("a")
        self.assertFalse(dup)
        self.assertEqual(again["id"], a["id"])
        self.assertEqual(again["status"], "pending")
        self.assertIsNone(again["error_message"])
        self.assertIsNotNone(again["last_retry_at"])

    async def test_exhausted_hash_also_reuses_row(self):
        a, _ = await self.reg("a")
        await self.conn.execute("UPDATE import_jobs SET max_retries = 1 WHERE id = $1", a["id"])
        await ij.start(self.conn, a["id"])
        exhausted = await ij.fail(self.conn, a["id"], "boom", auto_retry=True)
        self.assertEqual(exhausted["status"], "max_retries_exhausted")
        again, dup = await self.reg("a")
        self.assertEqual((again["id"], again["status"], dup), (a["id"], "pending", False))

    async def test_failed_hash_reset_blocked_by_other_inflight_run(self):
        a, _ = await self.reg("a")
        await ij.start(self.conn, a["id"])
        await ij.fail(self.conn, a["id"], "boom")
        await self.reg("b")  # now in flight for the same source+date
        with self.assertRaises(ij.JobInProgressError):
            await self.reg("a")

    # A4 (same connection, sequential)
    async def test_second_file_same_date_while_inflight_raises(self):
        await self.reg("a")
        with self.assertRaises(ij.JobInProgressError):
            await self.reg("b")

    # A4a
    async def test_null_date_rows_are_independent(self):
        a, _ = await self.reg("a", day=None)
        b, _ = await self.reg("b", day=None)
        self.assertEqual((a["version"], b["version"]), (1, 1))
        self.assertEqual((a["status"], b["status"]), ("pending", "pending"))

    # A5
    async def test_new_hash_for_completed_date_is_next_version(self):
        a = await self.done("a")
        b, dup = await self.reg("b")
        self.assertFalse(dup)
        self.assertEqual((b["version"], b["supersedes_job_id"]), (2, a["id"]))
        self.assertEqual(await self.status(a["id"]), "completed")

    # A5a
    async def test_revert_to_superseded_hash_is_version_three(self):
        a = await self.done("a")
        b = await self.done("b")
        self.assertEqual(await self.status(a["id"]), "superseded")
        a2, dup = await self.reg("a")
        self.assertFalse(dup)
        self.assertNotEqual(a2["id"], a["id"])
        self.assertEqual((a2["version"], a2["supersedes_job_id"]), (3, b["id"]))

    # A6
    async def test_complete_supersedes_previous_version(self):
        a = await self.done("a")
        b = await self.done("b")
        self.assertEqual(b["status"], "completed")
        self.assertEqual(await self.status(a["id"]), "superseded")
        self.assertEqual((b["row_count"], b["error_count"]), (3, 0))
        cur = await ij.current(self.conn, source=self.source, processing_date=DAY)
        self.assertEqual(cur["id"], b["id"])

    async def test_complete_rolls_back_with_callers_transaction(self):
        a = await self.done("a")
        b, _ = await self.reg("b")
        await ij.start(self.conn, b["id"])
        with self.assertRaises(RuntimeError):
            async with self.conn.transaction():  # caller's business transaction
                await ij.complete(self.conn, b["id"], row_count=1)
                raise RuntimeError("business write failed")
        self.assertEqual(await self.status(a["id"]), "completed")
        self.assertEqual(await self.status(b["id"]), "processing")

    # A7
    async def test_fail_keeps_previous_version_current(self):
        a = await self.done("a")
        b, _ = await self.reg("b")
        await ij.start(self.conn, b["id"])
        failed = await ij.fail(self.conn, b["id"], "x" * 5000)
        self.assertEqual(failed["status"], "failed")
        self.assertEqual(len(failed["error_message"]), ij.MAX_ERROR_MESSAGE)
        self.assertEqual(await self.status(a["id"]), "completed")

    async def test_auto_retry_failures_exhaust_at_max_retries(self):
        job = await ij.detect(
            self.conn, source=self.source, processing_date=DAY, file_hash=h("a"),
            original_filename="a", file_path=None, detection_source="not_processed",
        )
        for attempt in range(1, 4):
            retried = await ij.start(self.conn, job["id"])
            self.assertIsNotNone(retried["last_retry_at"])
            job = await ij.fail(self.conn, job["id"], "boom", auto_retry=True)
            self.assertEqual(job["auto_retry_count"], attempt)
        self.assertEqual(job["status"], "max_retries_exhausted")
        # manual retry is still allowed past the limit and never counts
        await ij.start(self.conn, job["id"])
        job = await ij.fail(self.conn, job["id"], "boom again")
        self.assertEqual((job["status"], job["auto_retry_count"]), ("failed", 3))

    # A9
    async def test_illegal_transitions_are_rejected(self):
        a = await self.done("a")
        with self.assertRaises(ij.IllegalTransitionError):
            await ij.start(self.conn, a["id"])
        with self.assertRaises(ij.IllegalTransitionError):
            await ij.fail(self.conn, a["id"], "x")
        b, _ = await self.reg("b")
        with self.assertRaises(ij.IllegalTransitionError):
            await ij.complete(self.conn, b["id"])

    async def test_unknown_job_id(self):
        with self.assertRaises(ij.JobNotFoundError):
            await ij.start(self.conn, -1)

    # FR12 / P6
    async def test_detect_is_insert_if_absent_and_never_resets(self):
        job = await ij.detect(
            self.conn, source=self.source, processing_date=DAY, file_hash=h("a"),
            original_filename="a", file_path="/ftp/a", detection_source="input_remaining",
            error_message="left in input",
        )
        self.assertEqual((job["status"], job["detection_source"]), ("failed", "input_remaining"))
        await self.conn.execute(
            "UPDATE import_jobs SET status = 'max_retries_exhausted', auto_retry_count = 3 WHERE id = $1",
            job["id"],
        )
        again = await ij.detect(
            self.conn, source=self.source, processing_date=DAY, file_hash=h("a"),
            original_filename="a", file_path="/ftp/a", detection_source="input_remaining",
        )
        self.assertIsNone(again)
        row = await self.conn.fetchrow("SELECT status, auto_retry_count FROM import_jobs WHERE id = $1", job["id"])
        self.assertEqual((row["status"], row["auto_retry_count"]), ("max_retries_exhausted", 3))

    # A16
    async def test_user_transitions_write_audit_logs_in_same_tx(self):
        uid = await self.conn.fetchval("SELECT id FROM users ORDER BY id LIMIT 1")
        if uid is None:
            self.skipTest("no users row to act as auditor")
        a, _ = await self.reg("a", created_by=uid, ip="10.0.0.1")
        await ij.start(self.conn, a["id"], actor_id=uid)
        await ij.fail(self.conn, a["id"], "boom", actor_id=uid)
        rows = await self.conn.fetch(
            """SELECT actor_id, action, before, after, ip FROM audit_logs
               WHERE entity_type = 'import_job' AND entity_id = $1 ORDER BY id""",
            a["id"],
        )
        self.assertEqual(
            [r["action"] for r in rows],
            ["import_job_register", "import_job_start", "import_job_fail"],
        )
        self.assertIsNone(rows[0]["before"])
        self.assertIn('"processing"', rows[1]["after"])
        self.assertEqual({r["actor_id"] for r in rows}, {uid})
        self.assertEqual(rows[0]["ip"], "10.0.0.1")

    async def test_system_transitions_write_no_audit_logs(self):
        a, _ = await self.reg("a")
        await ij.start(self.conn, a["id"])
        n = await self.conn.fetchval(
            "SELECT count(*) FROM audit_logs WHERE entity_type = 'import_job' AND entity_id = $1", a["id"],
        )
        self.assertEqual(n, 0)


@unittest.skipUnless(DATABASE_URL, "DATABASE_URL not set")
class ImportJobsCommittedTest(unittest.IsolatedAsyncioTestCase):
    """Needs committed rows: parallel connections (A4) and a real clock (A8)."""

    async def asyncSetUp(self) -> None:
        self.source = f"test_ij_{uuid.uuid4().hex[:12]}"
        self.conns = [await asyncpg.connect(DATABASE_URL) for _ in range(2)]

    async def asyncTearDown(self) -> None:
        await self.conns[0].execute("DELETE FROM import_jobs WHERE source = $1", self.source)
        for c in self.conns:
            await c.close()

    # A4 (parallel)
    async def test_parallel_register_same_date_one_wins_one_in_progress(self):
        async def attempt(conn, c):
            try:
                await ij.register(conn, source=self.source, processing_date=DAY,
                                  file_hash=h(c), original_filename=c)
                return "ok"
            except ij.JobInProgressError:
                return "in_progress"

        results = await asyncio.gather(attempt(self.conns[0], "a"), attempt(self.conns[1], "b"))
        self.assertEqual(sorted(results), ["in_progress", "ok"])

    async def test_parallel_register_same_hash_one_is_duplicate(self):
        async def attempt(conn):
            _, dup = await ij.register(conn, source=self.source, processing_date=None,
                                       file_hash=h("a"), original_filename="a")
            return dup

        results = await asyncio.gather(attempt(self.conns[0]), attempt(self.conns[1]))
        self.assertEqual(sorted(results), [False, True])

    # A8
    async def test_stale_inflight_row_is_failed_only_past_its_limit(self):
        conn = self.conns[0]
        a, _ = await ij.register(conn, source=self.source, processing_date=DAY,
                                 file_hash=h("a"), original_filename="a")
        await ij.start(conn, a["id"])
        await asyncio.sleep(0.3)

        # a batch-ETL limit (e.g. 60m) does not fire yet ...
        self.assertEqual(await ij.mark_stale(conn, source=self.source, stale_after=timedelta(hours=1)), [])
        # ... and a new file for the same date is still blocked
        with self.assertRaises(ij.JobInProgressError):
            await ij.register(conn, source=self.source, processing_date=DAY, file_hash=h("b"),
                              original_filename="b", stale_after=timedelta(hours=1))

        # past the limit, register marks it failed and proceeds (FR8)
        b, dup = await ij.register(conn, source=self.source, processing_date=DAY, file_hash=h("b"),
                                   original_filename="b", stale_after=timedelta(milliseconds=100))
        self.assertFalse(dup)
        row = await conn.fetchrow("SELECT status, error_message FROM import_jobs WHERE id = $1", a["id"])
        self.assertEqual(row["status"], "failed")
        self.assertTrue(row["error_message"].startswith("stale:"))

    async def test_mark_stale_without_date_sweeps_all_dates(self):
        conn = self.conns[0]
        ids = []
        for i, c in enumerate("ab"):
            job, _ = await ij.register(conn, source=self.source, processing_date=DAY + timedelta(days=i),
                                       file_hash=h(c), original_filename=c)
            ids.append(job["id"])
        await asyncio.sleep(0.3)
        swept = await ij.mark_stale(conn, source=self.source, stale_after=timedelta(milliseconds=100))
        self.assertEqual(sorted(swept), sorted(ids))


if __name__ == "__main__":
    unittest.main()
