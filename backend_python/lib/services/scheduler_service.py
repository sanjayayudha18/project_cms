"""APScheduler job registration + retry orchestration state machine (tasks 9.2, 13.1).

Generic over the file_types tuple passed at construction time -- each service
(eod_retry_scheduler, service_dsr_etl) supplies its own subset instead of this
module importing a fixed FILE_TYPES constant, so the same class can drive both.
"""
from __future__ import annotations

import asyncio
import logging
from datetime import datetime, timedelta
from typing import Any

import asyncpg
from apscheduler.schedulers.asyncio import AsyncIOScheduler
from apscheduler.triggers.cron import CronTrigger
from apscheduler.triggers.interval import IntervalTrigger

from .. import import_jobs
from ..database import with_db_retry
from ..utils.timezone import WIB, current_processing_date
from .audit_service import AuditService
from .detector import FileDetector, LateDetector
from .retry_executor import RetryExecutor, RetryResult

logger = logging.getLogger(__name__)


# FR14/C3: the legacy ETLs record success in their own tables and never write
# import_jobs, so a source+date also counts as done when its table has a
# successful file for that date. Fixed SQL per source, never built from input.
# file_date/report_date equal the WIB processing_date: ITM from the filename,
# DSR = saldo 00:00 of that day, DMAA = file mtime on a host in Asia/Jakarta (R2a).
LEGACY_DONE_SQL: dict[str, str] = {
    "dmaa": "SELECT EXISTS(SELECT 1 FROM dmaa_files WHERE status = 'success' AND file_date = $1)",
    "itm_cashpos": "SELECT EXISTS(SELECT 1 FROM itm_cashpos_files WHERE status = 'completed' AND file_date = $1)",
    "itm_replenish": "SELECT EXISTS(SELECT 1 FROM itm_replenish_files WHERE status = 'completed' AND file_date = $1)",
    "dsr": "SELECT EXISTS(SELECT 1 FROM dsr_uploads WHERE daily_status = 'completed' AND report_date = $1)",
}


async def is_source_done(conn: asyncpg.Connection, file_type: str, processing_date) -> bool:
    """True when import_jobs has a completed row for source+date, or the legacy table has a success."""
    if await import_jobs.current(conn, source=file_type, processing_date=processing_date) is not None:
        return True
    legacy_sql = LEGACY_DONE_SQL.get(file_type)
    return legacy_sql is not None and bool(await conn.fetchval(legacy_sql, processing_date))


async def resolve_actor_id(conn: asyncpg.Connection, user_id: str) -> int | None:
    """users.id for an auth identity, or None ("api_key_user", unknown/non-numeric ids).
    audit_logs.actor_id is NOT NULL + FK, so only a real user can be recorded there (P4)."""
    if not user_id.isdecimal():
        return None
    return await conn.fetchval("SELECT id FROM users WHERE id = $1", int(user_id))


class RetryConflictError(Exception):
    """Raised when a manual retry is requested on a completed file (HTTP 409)."""


class FileNotFoundInTrackingError(Exception):
    """Raised when a manual retry targets an unknown file_id (HTTP 404)."""


class SchedulerService:
    """Manages periodic detection scans, retry cycles, and retry orchestration."""

    def __init__(
        self, settings: Any, pool: asyncpg.Pool, file_types: tuple[str, ...],
        detector: FileDetector | None = None,
        late_detector: LateDetector | None = None,
        retry_executor: RetryExecutor | None = None,
        audit_service: AuditService | None = None,
    ):
        self.settings = settings
        self.pool = pool
        self.file_types = file_types
        self.detector = detector or FileDetector(settings)
        self.late_detector = late_detector or LateDetector(settings)
        self.retry_executor = retry_executor or RetryExecutor(settings)
        self.audit = audit_service or AuditService(pool)
        # C1: every source these services own is a batch ETL, so one limit per service.
        self.stale_after = timedelta(minutes=settings.stale_after_minutes)
        # kill a hung ETL before its row would be swept as stale while it is still running
        self.etl_timeout = self.stale_after.total_seconds() * 0.9
        self._scan_lock = asyncio.Lock()
        self._scheduler = AsyncIOScheduler(timezone=WIB)
        self.last_successful_scan_at: datetime | None = None

    # -- lifecycle -----------------------------------------------------

    def start(self) -> None:
        """Register jobs and start the APScheduler."""
        self._scheduler.add_job(
            self._run_failure_scan,
            CronTrigger(
                minute=f"*/{self.settings.scan_interval_minutes}",
                hour=f"{self.settings.scan_cron_start_hour}-{self.settings.scan_cron_end_hour}",
                timezone=WIB,
            ),
            id="failure_detection_scan",
        )

        self._scheduler.add_job(
            self._run_auto_retries,
            IntervalTrigger(minutes=self.settings.retry_interval_minutes),
            id="auto_retry_cycle",
        )

        for file_type in self.file_types:
            sla = self.settings.get_sla_time(file_type)
            self._scheduler.add_job(
                self._run_late_check,
                CronTrigger(hour=sla.hour, minute=sla.minute, timezone=WIB),
                id=f"late_check_{file_type}",
                kwargs={"file_type": file_type},
            )

        self._scheduler.start()

    def stop(self) -> None:
        self._scheduler.shutdown()

    # -- scheduled jobs --------------------------------------------------

    async def _run_failure_scan(self) -> None:
        """Execute failure detection with mutual exclusion (Property 17)."""
        if self._scan_lock.locked():
            logger.warning("Scan already in progress, skipping triggered scan")
            return

        async with self._scan_lock:
            started_at = datetime.now(WIB)
            processing_date = current_processing_date()
            total_detected = 0
            error_message = None
            status = "success"
            logger.info("Starting failure detection scan")
            try:
                for file_type in self.file_types:
                    # sync filesystem walk + SHA-256: keep it off the event loop
                    files = await asyncio.to_thread(
                        lambda ft=file_type: (
                            self.detector.scan_not_processed(ft) + self.detector.scan_input_remaining(ft)
                        ),
                    )
                    if not files:
                        continue
                    inserted = await with_db_retry(
                        lambda f=files: self.detector.persist_detected_files(
                            self.pool, f, processing_date,
                        ),
                        max_attempts=self.settings.db_retry_max_attempts,
                        base_delay=self.settings.db_retry_base_delay_seconds,
                    )
                    total_detected += inserted
                self.last_successful_scan_at = datetime.now(WIB)
            except Exception as e:  # filesystem/db failure: isolate, do not crash scheduler
                logger.exception("Failure detection scan failed: %s", e)
                status = "failed"
                error_message = str(e)

            try:
                await self.detector.record_scan_run(
                    self.pool, "failure_detection", started_at, datetime.now(WIB),
                    status, total_detected, error_message,
                )
            except Exception:
                logger.exception("Failed to record scan_run entry")

    async def _run_auto_retries(self) -> None:
        """Process automatic retries for eligible failed files, isolating failures per file."""
        async with self.pool.acquire() as conn:
            # FR8: a run stuck in processing (crashed ETL) is failed first, so it
            # neither blocks its source+date forever nor waits for a new upload.
            for file_type in self.file_types:
                await import_jobs.mark_stale(conn, source=file_type, stale_after=self.stale_after)
            rows = await conn.fetch(
                """
                SELECT * FROM import_jobs
                WHERE status = 'failed' AND auto_retry_count < max_retries
                  AND source = ANY($1::text[])
                ORDER BY id
                """,
                list(self.file_types),
            )
        for row in rows:
            try:
                await self.process_auto_retry(dict(row))
            except Exception as e:
                logger.exception(
                    "Failed to auto-retry import_job %s (source=%s, date=%s): %s",
                    row["id"], row["source"], row["processing_date"], e,
                )
                # Continue to next file -- do not break the loop (Property 18).

    async def _run_late_check(self, file_type: str) -> None:
        """Check if the SLA deadline passed without completion (Requirement 2)."""
        processing_date = current_processing_date()
        async with self.pool.acquire() as conn:
            completed = await is_source_done(conn, file_type, processing_date)
        if self.late_detector.check_late(file_type, processing_date, has_completed=completed):
            sla = self.settings.get_sla_time(file_type)
            await self.late_detector.persist_late_detection(
                self.pool, file_type, processing_date, sla,
            )

    # -- retry orchestration (task 9.2) -----------------------------------

    async def process_auto_retry(self, file: dict) -> None:
        """Run one automatic retry attempt for a failed import_job (FR13)."""
        file_id: int = file["id"]
        file_type: str = file["source"]
        processing_date = file["processing_date"]
        checksum: str = file["file_hash"]

        async with self.pool.acquire() as conn:
            try:
                await import_jobs.start(conn, file_id)
            except import_jobs.JobInProgressError:
                logger.info(
                    "Skipping auto-retry of import_job %s: another %s run for %s is in progress",
                    file_id, file_type, processing_date,
                )
                return  # retried on the next cycle (FR13)

        result = await self._run_attempt(
            "auto", file_id, file_type, checksum, processing_date, "system", auto_retry=True,
        )
        await self._finish_retry(file_id, file_type, processing_date, result, auto_retry=True)
        await self.audit.log_retry_completed(
            "auto", file_id, file_type, checksum, processing_date, "system",
            "completed" if result.success else "failed",
            result.duration_ms, result.error_detail,
        )

    async def process_manual_retry(self, file_id: int, user_id: str, ip: str | None = None) -> dict:
        """Manual retry: bypasses max_retries, rejects completed files (Properties 10, 11).
        When user_id is a real users.id, the transitions also go to audit_logs (FR18/P4)."""
        async with self.pool.acquire() as conn:
            file = await conn.fetchrow("SELECT * FROM import_jobs WHERE id = $1", file_id)
            # a job of the other service (or a source this one cannot run) is unknown here
            if file is None or file["source"] not in self.file_types:
                raise FileNotFoundInTrackingError(f"No tracked file with id={file_id}")
            if file["status"] in ("completed", "superseded", "pending"):
                raise RetryConflictError(
                    f"File is already in '{file['status']}' status. Retry not allowed."
                )
            actor_id = await resolve_actor_id(conn, user_id)
            try:
                await import_jobs.start(conn, file_id, actor_id=actor_id, ip=ip)
            except (import_jobs.JobInProgressError, import_jobs.IllegalTransitionError) as e:
                raise RetryConflictError(str(e)) from e

        file_type = file["source"]
        processing_date = file["processing_date"]
        checksum = file["file_hash"]

        result = await self._run_attempt(
            "manual", file_id, file_type, checksum, processing_date, user_id,
            auto_retry=False, actor_id=actor_id, ip=ip,
        )
        new_status = await self._finish_retry(
            file_id, file_type, processing_date, result, auto_retry=False, actor_id=actor_id, ip=ip,
        )
        await self.audit.log_retry_completed(
            "manual", file_id, file_type, checksum, processing_date, user_id,
            new_status, result.duration_ms, result.error_detail,
        )

        return {"file_id": file_id, "processing_status": new_status, "triggered_by": user_id}

    async def _run_attempt(
        self, trigger: str, file_id: int, file_type: str, checksum: str, processing_date, initiator: str,
        *, auto_retry: bool, actor_id: int | None = None, ip: str | None = None,
    ) -> RetryResult:
        """Audit + run the ETL for a job already in `processing`. If anything raises (missing
        script, DB blip, cancellation), the job is failed before re-raising so it never
        stays stranded in `processing` until the stale sweep."""
        try:
            await self.audit.log_retry_initiated(trigger, file_id, file_type, checksum, processing_date, initiator)
            return await self.retry_executor.execute_retry(file_type, timeout=self.etl_timeout)
        except BaseException as e:
            await asyncio.shield(self._fail_after_error(file_id, e, auto_retry, actor_id, ip))
            raise

    async def _fail_after_error(
        self, file_id: int, error: BaseException, auto_retry: bool, actor_id: int | None, ip: str | None,
    ) -> None:
        try:
            async with self.pool.acquire() as conn:
                await import_jobs.fail(
                    conn, file_id, f"ETL did not run: {type(error).__name__}: {error}",
                    auto_retry=auto_retry, actor_id=actor_id, ip=ip,
                )
        except Exception:
            logger.exception("Could not fail import_job %s after an ETL error", file_id)

    async def _finish_retry(
        self, file_id: int, file_type: str, processing_date, result: RetryResult, *, auto_retry: bool,
        actor_id: int | None = None, ip: str | None = None,
    ) -> str:
        """completed (+ resolve late detection) or failed/max_retries_exhausted; returns the new status."""
        async with self.pool.acquire() as conn:
            try:
                if result.success:
                    job = await import_jobs.complete(conn, file_id, actor_id=actor_id, ip=ip)
                else:
                    job = await import_jobs.fail(
                        conn, file_id,
                        result.error_detail or f"ETL exited with code {result.return_code}",
                        auto_retry=auto_retry, actor_id=actor_id, ip=ip,
                    )
            except import_jobs.IllegalTransitionError:
                # the stale sweep failed this row while its ETL was still running (FR8)
                job = await conn.fetchrow("SELECT * FROM import_jobs WHERE id = $1", file_id)
                logger.warning("import_job %s left processing before its ETL finished (now %s)", file_id, job["status"])
                if result.success and job["status"] in ("failed", "max_retries_exhausted"):
                    try:  # the data did load: put the row back through processing to completed
                        await import_jobs.start(conn, file_id, actor_id=actor_id, ip=ip)
                        job = await import_jobs.complete(conn, file_id, actor_id=actor_id, ip=ip)
                    except import_jobs.JobInProgressError:
                        logger.warning("import_job %s stays %s: another run is in flight", file_id, job["status"])
        if job["status"] == "completed":
            await self.late_detector.resolve_late_detection(self.pool, file_type, processing_date)
        return job["status"]

    # -- generic manual trigger (POST /process/{file_type}) --------------

    async def run_manual_process(self, file_type: str, extra_args: list[str] | None = None) -> RetryResult:
        """Runs the ETL for `file_type` once, serialized behind the same lock the
        cron failure-scan uses, so a manual trigger and the scan can never race
        (Go's DSR upload handler calls this via routers/process.py in service_dsr_etl).
        extra_args targets a single file for DSR's dry-run/commit endpoints; every
        other caller omits it."""
        async with self._scan_lock:
            return await self.retry_executor.execute_retry(file_type, extra_args)


if __name__ == "__main__":
    # Smoke check: job registration count + mutual-exclusion lock behavior (no real DB/pool).
    import asyncio as _asyncio
    from datetime import time as _time

    class _FakePool:
        def acquire(self):
            raise AssertionError("should not touch DB in this smoke check")

    class _FakeSettings:
        scan_interval_minutes = 15
        scan_cron_start_hour = 5
        scan_cron_end_hour = 9
        retry_interval_minutes = 30
        db_retry_max_attempts = 3
        db_retry_base_delay_seconds = 1.0
        stale_after_minutes = 60

        def get_sla_time(self, file_type: str) -> _time:
            return _time(6, 0)

    file_types = ("dmaa", "itm_cashpos", "itm_replenish")

    async def _demo():  # APScheduler's asyncio scheduler needs a running loop
        svc = SchedulerService(_FakeSettings(), _FakePool(), file_types)  # type: ignore[arg-type]
        svc.start()
        job_ids = {job.id for job in svc._scheduler.get_jobs()}
        assert "failure_detection_scan" in job_ids
        assert "auto_retry_cycle" in job_ids
        for ft in file_types:
            assert f"late_check_{ft}" in job_ids
        svc.stop()
        async with svc._scan_lock:
            assert svc._scan_lock.locked()
        assert not svc._scan_lock.locked()

    _asyncio.run(_demo())
    print("scheduler_service.py demo OK")
