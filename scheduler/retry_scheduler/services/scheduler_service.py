"""APScheduler job registration + retry orchestration state machine (tasks 9.2, 13.1)."""
from __future__ import annotations

import asyncio
import logging
from datetime import datetime
from uuid import UUID

import asyncpg
from apscheduler.schedulers.asyncio import AsyncIOScheduler
from apscheduler.triggers.cron import CronTrigger
from apscheduler.triggers.interval import IntervalTrigger

from ..config import FILE_TYPES, Settings
from ..database import with_db_retry
from ..utils.timezone import WIB, current_processing_date
from .audit_service import AuditService
from .detector import FileDetector, LateDetector
from .retry_executor import RetryExecutor

logger = logging.getLogger(__name__)


class RetryConflictError(Exception):
    """Raised when a manual retry is requested on a completed file (HTTP 409)."""


class FileNotFoundInTrackingError(Exception):
    """Raised when a manual retry targets an unknown file_id (HTTP 404)."""


class SchedulerService:
    """Manages periodic detection scans, retry cycles, and retry orchestration."""

    def __init__(
        self, settings: Settings, pool: asyncpg.Pool,
        detector: FileDetector | None = None,
        late_detector: LateDetector | None = None,
        retry_executor: RetryExecutor | None = None,
        audit_service: AuditService | None = None,
    ):
        self.settings = settings
        self.pool = pool
        self.detector = detector or FileDetector(settings)
        self.late_detector = late_detector or LateDetector(settings)
        self.retry_executor = retry_executor or RetryExecutor(settings)
        self.audit = audit_service or AuditService(pool)
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

        for file_type in FILE_TYPES:
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
                for file_type in FILE_TYPES:
                    files = (
                        self.detector.scan_not_processed(file_type)
                        + self.detector.scan_input_remaining(file_type)
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
            rows = await conn.fetch(
                """
                SELECT * FROM retry_file_tracking
                WHERE processing_status = 'failed' AND auto_retry_count < max_retries
                """
            )
        for row in rows:
            try:
                await self.process_auto_retry(dict(row))
            except Exception as e:
                logger.exception(
                    "Failed to auto-retry file %s (type=%s, date=%s): %s",
                    row["id"], row["file_type"], row["processing_date"], e,
                )
                # Continue to next file -- do not break the loop (Property 18).

    async def _run_late_check(self, file_type: str) -> None:
        """Check if the SLA deadline passed without completion (Requirement 2)."""
        processing_date = current_processing_date()
        # ponytail: retry_scheduler only tracks files it has seen fail/retry via
        # retry_file_tracking (per migration 012 -- the only schema this service owns).
        # A "completed" ETL run that never touched retry_file_tracking is invisible here;
        # true completion status lives in dmaa_files/itm_cashpos_files/itm_replenish_files,
        # which are out of scope for this migration. Upgrade: read those tables once their
        # schema is approved (see open question in the final report).
        async with self.pool.acquire() as conn:
            completed = await conn.fetchval(
                """
                SELECT EXISTS(
                    SELECT 1 FROM retry_file_tracking
                    WHERE file_type = $1 AND processing_date = $2
                      AND processing_status = 'completed'
                )
                """,
                file_type, processing_date,
            )
        if self.late_detector.check_late(file_type, processing_date, has_completed=completed):
            sla = self.settings.get_sla_time(file_type)
            await self.late_detector.persist_late_detection(
                self.pool, file_type, processing_date, sla,
            )

    # -- retry orchestration (task 9.2) -----------------------------------

    async def process_auto_retry(self, file: dict) -> None:
        """Run one automatic retry attempt for a failed file (state machine transitions)."""
        file_id: UUID = file["id"]
        file_type: str = file["file_type"]
        processing_date = file["processing_date"]
        checksum: str = file["file_checksum"]

        async with self.pool.acquire() as conn:
            await conn.execute(
                """
                UPDATE retry_file_tracking
                SET processing_status = 'processing', last_retry_at = now(), updated_at = now()
                WHERE id = $1
                """,
                file_id,
            )

        await self.audit.log_retry_initiated(
            "auto", file_id, file_type, checksum, processing_date, "system",
        )

        result = await self.retry_executor.execute_retry(file_type)

        if result.success:
            new_status = "completed"
        else:
            new_auto_count = file["auto_retry_count"] + 1
            new_status = (
                "max_retries_exhausted" if new_auto_count >= file["max_retries"] else "failed"
            )

        async with self.pool.acquire() as conn:
            if result.success:
                await conn.execute(
                    """
                    UPDATE retry_file_tracking
                    SET processing_status = 'completed', completed_at = now(), updated_at = now()
                    WHERE id = $1
                    """,
                    file_id,
                )
                await self.late_detector.resolve_late_detection(
                    self.pool, file_type, processing_date,
                )
            else:
                await conn.execute(
                    """
                    UPDATE retry_file_tracking
                    SET processing_status = $2, auto_retry_count = auto_retry_count + 1,
                        updated_at = now()
                    WHERE id = $1
                    """,
                    file_id, new_status,
                )

        await self.audit.log_retry_completed(
            "auto", file_id, file_type, checksum, processing_date, "system",
            "completed" if result.success else "failed",
            result.duration_ms, result.error_detail,
        )

    async def process_manual_retry(self, file_id: UUID, user_id: str) -> dict:
        """Manual retry: bypasses max_retries, rejects completed files (Properties 10, 11)."""
        async with self.pool.acquire() as conn:
            file = await conn.fetchrow(
                "SELECT * FROM retry_file_tracking WHERE id = $1", file_id,
            )
        if file is None:
            raise FileNotFoundInTrackingError(f"No tracked file with id={file_id}")
        if file["processing_status"] == "completed":
            raise RetryConflictError(
                "File is already in 'completed' status. Retry not allowed."
            )

        file_type = file["file_type"]
        processing_date = file["processing_date"]
        checksum = file["file_checksum"]

        async with self.pool.acquire() as conn:
            await conn.execute(
                """
                UPDATE retry_file_tracking
                SET processing_status = 'processing', last_retry_at = now(), updated_at = now()
                WHERE id = $1
                """,
                file_id,
            )

        await self.audit.log_retry_initiated(
            "manual", file_id, file_type, checksum, processing_date, user_id,
        )

        result = await self.retry_executor.execute_retry(file_type)
        new_status = "completed" if result.success else "failed"

        async with self.pool.acquire() as conn:
            if result.success:
                await conn.execute(
                    """
                    UPDATE retry_file_tracking
                    SET processing_status = 'completed', completed_at = now(), updated_at = now()
                    WHERE id = $1
                    """,
                    file_id,
                )
                await self.late_detector.resolve_late_detection(
                    self.pool, file_type, processing_date,
                )
            else:
                await conn.execute(
                    """
                    UPDATE retry_file_tracking
                    SET processing_status = 'failed', updated_at = now()
                    WHERE id = $1
                    """,
                    file_id,
                )

        await self.audit.log_retry_completed(
            "manual", file_id, file_type, checksum, processing_date, user_id,
            new_status, result.duration_ms, result.error_detail,
        )

        return {"file_id": file_id, "processing_status": new_status, "triggered_by": user_id}


if __name__ == "__main__":
    # Smoke check: job registration count + mutual-exclusion lock behavior (no real DB/pool).
    import asyncio as _asyncio

    class _FakePool:
        def acquire(self):
            raise AssertionError("should not touch DB in this smoke check")

    settings = Settings(auth_secret="x")
    svc = SchedulerService(settings, _FakePool())  # type: ignore[arg-type]
    svc.start()
    job_ids = {job.id for job in svc._scheduler.get_jobs()}
    assert "failure_detection_scan" in job_ids
    assert "auto_retry_cycle" in job_ids
    for ft in FILE_TYPES:
        assert f"late_check_{ft}" in job_ids
    svc.stop()

    async def _lock_demo():
        async with svc._scan_lock:
            assert svc._scan_lock.locked()
        assert not svc._scan_lock.locked()

    _asyncio.run(_lock_demo())
    print("scheduler_service.py demo OK")
