"""Integration tests for SchedulerService retry orchestration on import_jobs
(.claude/sdlc/import-export-jobs spec A12 / FR8 / FR13). The ETL subprocess is faked;
DB is real. Skipped when DATABASE_URL is empty.

Run from backend_python/:  python -m unittest lib.services.test_scheduler_service -v
"""
from __future__ import annotations

import asyncio
import os
import unittest
import uuid
from datetime import date, time
from unittest import mock

import asyncpg

from lib import import_jobs as ij
from lib.services.detector import LateDetector
from lib.services.retry_executor import RetryResult
from lib.services.scheduler_service import (
    LEGACY_DONE_SQL,
    FileNotFoundInTrackingError,
    RetryConflictError,
    SchedulerService,
    is_source_done,
    resolve_actor_id,
)

DATABASE_URL = os.environ.get("DATABASE_URL", "")
DAY = date(2099, 1, 1)


class _Settings:
    stale_after_minutes = 60

    def get_sla_time(self, file_type: str) -> time:
        return time(6, 0)


class _FakeExecutor:
    """Returns queued outcomes (True = ETL exit 0) and records each call."""

    def __init__(self, *outcomes: bool):
        self.outcomes = list(outcomes)
        self.calls: list[str] = []

    async def execute_retry(self, file_type: str, extra_args=None) -> RetryResult:
        self.calls.append(file_type)
        ok = self.outcomes.pop(0)
        return RetryResult(success=ok, duration_ms=5, stdout="", stderr="" if ok else "etl boom",
                           return_code=0 if ok else 1)


@unittest.skipUnless(DATABASE_URL, "DATABASE_URL not set")
class SchedulerRetryTest(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self) -> None:
        self.pool = await asyncpg.create_pool(DATABASE_URL, min_size=1, max_size=3)
        self.source = f"test_sch_{uuid.uuid4().hex[:8]}"  # late_detections.file_type is varchar(20)

    async def asyncTearDown(self) -> None:
        # test-only cleanup of the audit rows these synthetic jobs produced
        await self.pool.execute(
            """DELETE FROM audit_logs WHERE entity_type = 'import_job'
               AND entity_id IN (SELECT id FROM import_jobs WHERE source = $1)""",
            self.source,
        )
        await self.pool.execute(
            "DELETE FROM retry_audit_logs WHERE file_id IN (SELECT id FROM import_jobs WHERE source = $1)",
            self.source,
        )
        await self.pool.execute("DELETE FROM import_jobs WHERE source = $1", self.source)
        await self.pool.execute("DELETE FROM late_detections WHERE file_type = $1", self.source)
        await self.pool.close()

    def service(self, *outcomes: bool, stale_minutes: float = 60) -> tuple[SchedulerService, _FakeExecutor]:
        settings = _Settings()
        settings.stale_after_minutes = stale_minutes
        executor = _FakeExecutor(*outcomes)
        return SchedulerService(settings, self.pool, (self.source,), retry_executor=executor), executor

    async def detected(self, c: str = "a") -> int:
        async with self.pool.acquire() as conn:
            job = await ij.detect(conn, source=self.source, processing_date=DAY, file_hash=c * 64,
                                  original_filename=f"{c}.xlsx", file_path=None,
                                  detection_source="not_processed")
        return job["id"]

    async def job(self, job_id: int):
        return await self.pool.fetchrow("SELECT * FROM import_jobs WHERE id = $1", job_id)

    # A12: fails x3 -> max_retries_exhausted, then no more auto retries
    async def test_auto_retry_three_failures_exhausts_and_stops(self):
        job_id = await self.detected()
        svc, executor = self.service(False, False, False)
        for _ in range(4):
            await svc._run_auto_retries()
        job = await self.job(job_id)
        self.assertEqual((job["status"], job["auto_retry_count"]), ("max_retries_exhausted", 3))
        self.assertEqual(job["error_message"], "etl boom")
        self.assertEqual(len(executor.calls), 3)
        events = await self.pool.fetch(
            "SELECT event_type, trigger_type, outcome FROM retry_audit_logs WHERE file_id = $1 ORDER BY created_at",
            job_id,
        )
        self.assertEqual(len(events), 6)
        self.assertEqual({e["trigger_type"] for e in events}, {"auto"})

    async def test_auto_retry_success_completes_and_resolves_late_detection(self):
        job_id = await self.detected()
        await self.pool.execute(
            "INSERT INTO late_detections (file_type, processing_date, sla_deadline) VALUES ($1, $2, '06:00')",
            self.source, DAY,
        )
        svc, _ = self.service(True)
        await svc._run_auto_retries()
        self.assertEqual((await self.job(job_id))["status"], "completed")
        resolved = await self.pool.fetchval(
            "SELECT is_resolved FROM late_detections WHERE file_type = $1 AND processing_date = $2",
            self.source, DAY,
        )
        self.assertTrue(resolved)

    # A12: manual retry bypasses max_retries and never counts
    async def test_manual_retry_past_limit_then_completed_conflict(self):
        job_id = await self.detected()
        await self.pool.execute(
            "UPDATE import_jobs SET status = 'max_retries_exhausted', auto_retry_count = 3 WHERE id = $1", job_id,
        )
        svc, _ = self.service(False, True)

        out = await svc.process_manual_retry(job_id, "42")
        self.assertEqual(out["processing_status"], "failed")
        self.assertEqual((await self.job(job_id))["auto_retry_count"], 3)

        out = await svc.process_manual_retry(job_id, "42")
        self.assertEqual(out["processing_status"], "completed")

        with self.assertRaises(RetryConflictError):  # A12: completed -> 409
            await svc.process_manual_retry(job_id, "42")
        initiated_by = await self.pool.fetchval(
            "SELECT DISTINCT initiated_by FROM retry_audit_logs WHERE file_id = $1", job_id,
        )
        self.assertEqual(initiated_by, "42")

    # T5.4 / FR18 / P4
    async def test_manual_retry_by_real_user_writes_audit_logs(self):
        uid = await self.pool.fetchval("SELECT id FROM users ORDER BY id LIMIT 1")
        if uid is None:
            self.skipTest("no users row to act as auditor")
        job_id = await self.detected()
        svc, _ = self.service(True)
        await svc.process_manual_retry(job_id, str(uid), ip="10.1.2.3")
        rows = await self.pool.fetch(
            """SELECT actor_id, action, ip FROM audit_logs
               WHERE entity_type = 'import_job' AND entity_id = $1 ORDER BY id""",
            job_id,
        )
        self.assertEqual([r["action"] for r in rows], ["import_job_retry", "import_job_complete"])
        self.assertEqual({(r["actor_id"], r["ip"]) for r in rows}, {(uid, "10.1.2.3")})

    async def test_manual_retry_without_real_user_writes_no_audit_logs(self):
        for hash_char, identity in zip("abc", ("api_key_user", "unknown", "999999999999")):
            with self.subTest(identity=identity):
                job_id = await self.detected(hash_char)
                svc, _ = self.service(False)
                await svc.process_manual_retry(job_id, identity)
                n = await self.pool.fetchval(
                    "SELECT count(*) FROM audit_logs WHERE entity_type = 'import_job' AND entity_id = $1", job_id,
                )
                self.assertEqual(n, 0)
                initiated_by = await self.pool.fetchval(
                    "SELECT DISTINCT initiated_by FROM retry_audit_logs WHERE file_id = $1", job_id,
                )
                self.assertEqual(initiated_by, identity)  # still traceable in retry_audit_logs

    async def test_resolve_actor_id_rejects_non_numeric(self):
        async with self.pool.acquire() as conn:
            for identity in ("", "api_key_user", "12abc", "-1", "1.5", " 1"):
                with self.subTest(identity=identity):
                    self.assertIsNone(await resolve_actor_id(conn, identity))

    async def test_manual_retry_unknown_id(self):
        svc, _ = self.service()
        with self.assertRaises(FileNotFoundInTrackingError):
            await svc.process_manual_retry(-1, "42")

    async def test_manual_retry_while_other_run_inflight_is_conflict(self):
        job_id = await self.detected("a")
        async with self.pool.acquire() as conn:
            await ij.register(conn, source=self.source, processing_date=DAY, file_hash="b" * 64,
                              original_filename="b.xlsx")
        svc, executor = self.service(True)
        with self.assertRaises(RetryConflictError):
            await svc.process_manual_retry(job_id, "42")
        self.assertEqual(executor.calls, [])

    # FR13: another run in flight for source+date -> skip this cycle, retry later
    async def test_auto_retry_skips_row_blocked_by_inflight_run(self):
        job_id = await self.detected("a")
        async with self.pool.acquire() as conn:
            await ij.register(conn, source=self.source, processing_date=DAY, file_hash="b" * 64,
                              original_filename="b.xlsx")
        svc, executor = self.service(True)
        await svc._run_auto_retries()
        self.assertEqual(executor.calls, [])
        job = await self.job(job_id)
        self.assertEqual((job["status"], job["auto_retry_count"]), ("failed", 0))

    # FR8 / T5.2: a stuck processing row is failed at the start of the cycle, then retried
    async def test_cycle_marks_stuck_processing_row_stale_then_retries_it(self):
        job_id = await self.detected()
        async with self.pool.acquire() as conn:
            await ij.start(conn, job_id)  # ETL "crashed" mid-run
        await asyncio.sleep(0.3)
        svc, executor = self.service(True, stale_minutes=0.001)  # ~60ms
        await svc._run_auto_retries()
        self.assertEqual(executor.calls, [self.source])
        self.assertEqual((await self.job(job_id))["status"], "completed")


class _NoClockLateDetector(LateDetector):
    """Deadline always passed, so the test only exercises the has_completed input."""

    def check_late(self, file_type: str, processing_date: date, has_completed: bool) -> bool:
        return not has_completed


# One successful legacy row per source (FR14 mapping), written inside a rolled-back tx.
_LEGACY_SUCCESS_INSERT = {
    "dmaa": "INSERT INTO dmaa_files (name, status, file_date) VALUES ('test.xlsx', 'success', $1)",
    "itm_cashpos": "INSERT INTO itm_cashpos_files (filename, file_date, status) VALUES ('test.csv', $1, 'completed')",
    "itm_replenish": "INSERT INTO itm_replenish_files (filename, file_date, status) VALUES ('test.csv', $1, 'completed')",
    "dsr": "INSERT INTO dsr_uploads (filename, vendor, report_date, daily_status) VALUES ('test.xlsx', 'TEST_OK', $1, 'completed')",
}
_LEGACY_FAILED_INSERT = {
    "dmaa": "INSERT INTO dmaa_files (name, status, file_date) VALUES ('test.xlsx', 'failed', $1)",
    "itm_cashpos": "INSERT INTO itm_cashpos_files (filename, file_date, status) VALUES ('test.csv', $1, 'failed')",
    "itm_replenish": "INSERT INTO itm_replenish_files (filename, file_date, status) VALUES ('test.csv', $1, 'failed')",
    "dsr": "INSERT INTO dsr_uploads (filename, vendor, report_date, daily_status) VALUES ('test.xlsx', 'TEST', $1, 'failed')",
}


@unittest.skipUnless(DATABASE_URL, "DATABASE_URL not set")
class IsSourceDoneTest(unittest.IsolatedAsyncioTestCase):
    """A13 per source, against the real legacy tables; every write is rolled back."""

    async def asyncSetUp(self) -> None:
        self.conn = await asyncpg.connect(DATABASE_URL)
        self.tx = self.conn.transaction()
        await self.tx.start()

    async def asyncTearDown(self) -> None:
        await self.tx.rollback()
        await self.conn.close()

    async def test_mapping_covers_every_retry_source(self):
        from eod_retry_scheduler.config import FILE_TYPES as RETRY_TYPES
        from service_dsr_etl.config import FILE_TYPES as DSR_TYPES
        self.assertEqual(set(LEGACY_DONE_SQL), set(RETRY_TYPES) | set(DSR_TYPES))

    async def test_each_source(self):
        # Each source reads its own table, so rows of one source never affect another.
        for source in LEGACY_DONE_SQL:
            with self.subTest(source=source):
                self.assertFalse(await is_source_done(self.conn, source, DAY), "empty -> not done")
                await self.conn.execute(_LEGACY_FAILED_INSERT[source], DAY)
                self.assertFalse(await is_source_done(self.conn, source, DAY), "legacy failed -> not done")
                await self.conn.execute(_LEGACY_SUCCESS_INSERT[source], DAY)
                self.assertTrue(await is_source_done(self.conn, source, DAY), "legacy success -> done")
                self.assertFalse(await is_source_done(self.conn, source, date(2099, 1, 2)), "other date")

    async def test_import_jobs_completed_counts_as_done_without_legacy_row(self):
        source = f"test_sch_{uuid.uuid4().hex[:8]}"
        self.assertFalse(await is_source_done(self.conn, source, DAY))
        job, _ = await ij.register(self.conn, source=source, processing_date=DAY,
                                   file_hash="a" * 64, original_filename="a")
        await ij.start(self.conn, job["id"])
        self.assertFalse(await is_source_done(self.conn, source, DAY), "processing is not done")
        await ij.complete(self.conn, job["id"])
        self.assertTrue(await is_source_done(self.conn, source, DAY))


@unittest.skipUnless(DATABASE_URL, "DATABASE_URL not set")
class LateCheckTest(unittest.IsolatedAsyncioTestCase):
    """A13 end to end: _run_late_check persists late_detections only when not done."""

    async def asyncSetUp(self) -> None:
        self.pool = await asyncpg.create_pool(DATABASE_URL, min_size=1, max_size=2)
        self.source = f"test_sch_{uuid.uuid4().hex[:8]}"
        self.svc = SchedulerService(
            _Settings(), self.pool, (self.source,),
            late_detector=_NoClockLateDetector(_Settings()), retry_executor=_FakeExecutor(),
        )
        self.patch = mock.patch("lib.services.scheduler_service.current_processing_date", return_value=DAY)
        self.patch.start()

    async def asyncTearDown(self) -> None:
        self.patch.stop()
        await self.pool.execute("DELETE FROM import_jobs WHERE source = $1", self.source)
        await self.pool.execute("DELETE FROM late_detections WHERE file_type = $1", self.source)
        await self.pool.close()

    async def late_rows(self) -> int:
        return await self.pool.fetchval("SELECT count(*) FROM late_detections WHERE file_type = $1", self.source)

    async def test_nothing_done_is_late(self):
        await self.svc._run_late_check(self.source)
        await self.svc._run_late_check(self.source)  # idempotent per source+date
        self.assertEqual(await self.late_rows(), 1)

    async def test_completed_import_job_is_not_late(self):
        async with self.pool.acquire() as conn:
            job, _ = await ij.register(conn, source=self.source, processing_date=DAY,
                                       file_hash="a" * 64, original_filename="a")
            await ij.start(conn, job["id"])
            await ij.complete(conn, job["id"])
        await self.svc._run_late_check(self.source)
        self.assertEqual(await self.late_rows(), 0)


if __name__ == "__main__":
    unittest.main()
