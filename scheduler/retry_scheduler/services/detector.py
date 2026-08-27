"""File failure detection + late (SLA) detection services (tasks 7.1, 7.2, 8.1)."""
from __future__ import annotations

import logging
from datetime import date, datetime, time
from pathlib import Path
from typing import NamedTuple
from uuid import UUID

import asyncpg

from ..config import FILE_PATTERNS, Settings
from ..utils.timezone import WIB

logger = logging.getLogger(__name__)


class DetectedFile(NamedTuple):
    file_path: Path
    filename: str
    file_type: str
    checksum: str
    detection_source: str  # 'not_processed' or 'input_remaining'
    failure_reason: str


class FileDetector:
    """Scans filesystem directories to find failed/unprocessed files."""

    def __init__(self, settings: Settings):
        self.settings = settings

    def scan_not_processed(self, file_type: str) -> list[DetectedFile]:
        """Find files in the not_processed directory for a given file type."""
        from ..utils.checksum import compute_checksum

        dir_path = self.settings.not_processed_dir(file_type)
        if not dir_path.exists():
            return []

        results = []
        for path in dir_path.iterdir():
            if path.is_file():
                results.append(DetectedFile(
                    file_path=path,
                    filename=path.name,
                    file_type=file_type,
                    checksum=compute_checksum(path),
                    detection_source="not_processed",
                    failure_reason=f"File found in not_processed directory: {dir_path}",
                ))
        return results

    def scan_input_remaining(self, file_type: str) -> list[DetectedFile]:
        """Find files remaining in the input directory after the ETL window closed."""
        from ..utils.checksum import compute_checksum

        dir_path = self.settings.input_dir(file_type)
        pattern = FILE_PATTERNS[file_type]
        if not dir_path.exists():
            return []

        results = []
        for path in dir_path.glob(pattern):
            if path.is_file():
                results.append(DetectedFile(
                    file_path=path,
                    filename=path.name,
                    file_type=file_type,
                    checksum=compute_checksum(path),
                    detection_source="input_remaining",
                    failure_reason=f"File remaining in input after ETL window: {dir_path}",
                ))
        return results

    async def persist_detected_files(
        self, pool: asyncpg.Pool, files: list[DetectedFile], processing_date: date,
    ) -> int:
        """Insert newly detected files into retry_file_tracking. Idempotent per (checksum, date)."""
        inserted = 0
        async with pool.acquire() as conn:
            for f in files:
                row = await conn.fetchrow(
                    """
                    INSERT INTO retry_file_tracking
                        (file_type, filename, file_path, file_checksum, processing_date,
                         detection_source, failure_reason, processing_status)
                    VALUES ($1, $2, $3, $4, $5, $6, $7, 'failed')
                    ON CONFLICT (file_checksum, processing_date) DO NOTHING
                    RETURNING id
                    """,
                    f.file_type, f.filename, str(f.file_path), f.checksum,
                    processing_date, f.detection_source, f.failure_reason,
                )
                if row is not None:
                    inserted += 1
        return inserted

    async def record_scan_run(
        self, pool: asyncpg.Pool, scan_type: str, started_at: datetime,
        finished_at: datetime | None, status: str, files_detected: int,
        error_message: str | None = None,
    ) -> UUID:
        """Create a scan_runs entry recording this scan's outcome."""
        async with pool.acquire() as conn:
            row = await conn.fetchrow(
                """
                INSERT INTO scan_runs
                    (scan_type, started_at, finished_at, status, files_detected, error_message)
                VALUES ($1, $2, $3, $4, $5, $6)
                RETURNING id
                """,
                scan_type, started_at, finished_at, status, files_detected, error_message,
            )
        return row["id"]


class LateDetector:
    """Checks whether SLA deadlines have passed without completed processing."""

    def __init__(self, settings: Settings):
        self.settings = settings

    def check_late(self, file_type: str, processing_date: date, has_completed: bool) -> bool:
        """True if file_type is late for the given processing_date."""
        now_wib = datetime.now(WIB)
        sla_time = self.settings.get_sla_time(file_type)
        deadline = datetime.combine(processing_date, sla_time, tzinfo=WIB)
        return now_wib > deadline and not has_completed

    async def persist_late_detection(
        self, pool: asyncpg.Pool, file_type: str, processing_date: date, sla_deadline: time,
    ) -> UUID | None:
        """Create a late_detections record. Idempotent per (file_type, processing_date)."""
        async with pool.acquire() as conn:
            row = await conn.fetchrow(
                """
                INSERT INTO late_detections (file_type, processing_date, sla_deadline)
                VALUES ($1, $2, $3)
                ON CONFLICT (file_type, processing_date) DO NOTHING
                RETURNING id
                """,
                file_type, processing_date, sla_deadline,
            )
        return row["id"] if row is not None else None

    async def resolve_late_detection(
        self, pool: asyncpg.Pool, file_type: str, processing_date: date,
    ) -> bool:
        """Mark a late_detections record resolved when the file achieves completed status."""
        async with pool.acquire() as conn:
            result = await conn.execute(
                """
                UPDATE late_detections
                SET resolved_at = now(), is_resolved = true, updated_at = now()
                WHERE file_type = $1 AND processing_date = $2 AND is_resolved = false
                """,
                file_type, processing_date,
            )
        return result.endswith("1")


if __name__ == "__main__":
    # Smoke check: pure logic only (no DB, no real filesystem beyond tmp dir).
    import tempfile
    from datetime import timedelta

    settings = Settings(auth_secret="x")
    detector = FileDetector(settings)

    with tempfile.TemporaryDirectory() as tmp:
        settings.dmaa_not_processed_dir = Path(tmp)
        (Path(tmp) / "Order_All_bad.xlsx").write_bytes(b"data")
        found = detector.scan_not_processed("dmaa")
        assert len(found) == 1
        assert found[0].detection_source == "not_processed"
        assert len(found[0].checksum) == 64

    late = LateDetector(settings)
    today = datetime.now(WIB).date()
    past = (datetime.now(WIB) - timedelta(hours=1)).time()
    future = (datetime.now(WIB) + timedelta(hours=1)).time()
    assert late.check_late("dmaa", today, has_completed=False) == (
        datetime.now(WIB) > datetime.combine(today, settings.get_sla_time("dmaa"), tzinfo=WIB)
    )
    assert late.check_late("dmaa", today, has_completed=True) is False
    print("detector.py demo OK")
