# Design Document: EOD Retry Scheduler

## Overview

The EOD Retry Scheduler is a standalone Python FastAPI microservice that monitors the CMS ETL file processing pipeline, detects failures and SLA breaches, and orchestrates automatic and manual retries by re-invoking existing ETL scripts. It exposes a REST API consumed by the Go backend and the frontend EOD Monitoring page.

The service runs alongside the existing `scheduler/` ETL scripts but is a separate long-running process rather than a cron job.

---

## Architecture

### System Context

```
┌──────────────────┐     REST API      ┌──────────────────────────┐
│   Go Backend     │◄──────────────────►│   EOD Retry Scheduler    │
│   (cmd/api)      │                    │   (Python FastAPI)        │
└──────────────────┘                    └────────────┬─────────────┘
                                                     │
┌──────────────────┐     REST API                    │  subprocess
│ Frontend EOD     │◄────────────────────────────────┤
│ Monitoring Page  │ (via Go backend proxy)          │
└──────────────────┘                                 ▼
                                        ┌──────────────────────────┐
                                        │  ETL Scripts (existing)   │
                                        │  - dmaa_etl.py            │
                                        │  - itm_cashpos_etl.py     │
                                        │  - itm_replenish_etl.py   │
                                        └────────────┬─────────────┘
                                                     │
                                                     ▼
                              ┌──────────────────────────────────────┐
                              │           PostgreSQL (CMS)            │
                              │  dmaa_files, itm_cashpos_files,       │
                              │  itm_replenish_files,                 │
                              │  retry_audit_logs, late_detections,   │
                              │  scan_runs                            │
                              └──────────────────────────────────────┘
                                                     ▲
                                                     │ filesystem scan
                              ┌──────────────────────────────────────┐
                              │        FTP_DATA/                      │
                              │  DMAA/ (input + not_processed/)       │
                              │  ITM/atm_caspos/ (input + backups/)   │
                              │  ITM/atm_replenish/ (input + backups/)│
                              └──────────────────────────────────────┘
```

### Key Design Decisions

1. **Subprocess invocation for retries.** The retry scheduler does not duplicate ETL logic. It calls existing scripts (`dmaa_etl.py`, `itm_cashpos_etl.py`, `itm_replenish_etl.py`) via `subprocess.run()`. This keeps the retry mechanism decoupled from parsing/transformation logic.

2. **Filesystem-based detection.** Detection relies on scanning actual directories (`not_processed/`, input directories post-window) rather than polling DB state alone. This catches cases where ETL scripts crash before writing any DB record.

3. **Shared PostgreSQL, separate tables.** The retry scheduler connects to the same CMS database but owns its operational tables (`retry_audit_logs`, `late_detections`, `scan_runs`). It reads from `dmaa_files`, `itm_cashpos_files`, `itm_replenish_files` for status but does not write to them — the ETL scripts handle that.

4. **APScheduler for background tasks.** Uses APScheduler (AsyncIO scheduler) inside the FastAPI process for cron-like scan scheduling and retry intervals. No external cron dependency.

5. **Single-process with async lock for scan mutual exclusion.** An `asyncio.Lock` prevents concurrent scan executions within the single-process deployment.

---

## Components

### Directory Structure

```
scheduler/
  retry_scheduler/
    __init__.py
    main.py                 # FastAPI app factory, lifespan
    config.py               # Settings via pydantic-settings (env + file)
    models.py               # SQLAlchemy/asyncpg models (retry tables)
    schemas.py              # Pydantic request/response schemas
    database.py             # Async DB connection pool (asyncpg)
    dependencies.py         # FastAPI deps (auth, db session)
    routers/
      __init__.py
      health.py             # GET /health
      status.py             # GET /status, GET /status/{file_id}/history
      retry.py              # POST /retry/{file_id}
      audit.py              # GET /audit
      late.py               # GET /late
      summary.py            # GET /summary
    services/
      __init__.py
      detector.py           # File failure + late detection logic
      retry_executor.py     # ETL subprocess invocation + result handling
      scheduler_service.py  # APScheduler job registration
      audit_service.py      # Audit log writer
    utils/
      __init__.py
      checksum.py           # SHA-256 file hashing
      timezone.py           # WIB timezone helpers
```

### Component Responsibilities

| Component | Responsibility |
|-----------|---------------|
| `config.py` | Load and validate SLA deadlines, retry intervals, max retry count, directory paths, DB URL, auth secret from environment/config file. Reject invalid config at startup. |
| `detector.py` | Scan filesystem directories, compute checksums, compare against DB state, produce list of failed/late files. Pure detection logic separated from persistence. |
| `retry_executor.py` | Invoke ETL scripts via subprocess, capture stdout/stderr, determine success/failure, update file status, write audit logs. |
| `scheduler_service.py` | Register APScheduler jobs for periodic detection scans and automatic retry cycles. Handle scan mutual exclusion. |
| `audit_service.py` | Append-only audit log writer. Never updates or deletes existing entries. |
| `routers/*` | Thin HTTP layer: validate request, call service, return response envelope. |
| `dependencies.py` | Auth middleware (API key or JWT validation), DB session injection. |

---

## Interfaces

### REST API Endpoints

All responses use a consistent JSON envelope:

```python
class APIResponse(BaseModel):
    status: Literal["success", "error"]
    data: Any | None = None
    error: str | None = None
```

#### Health (no auth required)

```
GET /health
Response 200:
{
  "status": "success",
  "data": {
    "service": "eod-retry-scheduler",
    "version": "1.0.0",
    "uptime_seconds": 3600,
    "last_successful_scan_at": "2025-01-15T06:15:00+07:00",
    "database": "connected",
    "filesystem": "accessible"
  }
}

Response 503:
{
  "status": "error",
  "data": null,
  "error": "Database connectivity failure: connection refused"
}
```

#### Status (auth required)

```
GET /status?processing_date=2025-01-15
Response 200:
{
  "status": "success",
  "data": {
    "processing_date": "2025-01-15",
    "by_file_type": {
      "dmaa": [
        {
          "file_id": "uuid",
          "filename": "Order_All_20250115.xlsx",
          "checksum": "sha256...",
          "processing_status": "failed",
          "retry_count": 2,
          "max_retries_exhausted": false,
          "detected_at": "2025-01-15T06:15:00+07:00",
          "last_retry_at": "2025-01-15T07:00:00+07:00",
          "failure_reason": "Missing required columns"
        }
      ],
      "itm_cashpos": [...],
      "itm_replenish": [...]
    }
  }
}
```

```
GET /status/{file_id}/history
Response 200:
{
  "status": "success",
  "data": {
    "file_id": "uuid",
    "filename": "Order_All_20250115.xlsx",
    "file_type": "dmaa",
    "attempts": [
      {
        "attempt_number": 1,
        "trigger": "auto",
        "started_at": "2025-01-15T06:30:00+07:00",
        "completed_at": "2025-01-15T06:30:05+07:00",
        "outcome": "failed",
        "duration_ms": 5000,
        "error_detail": "Exit code 1: ValueError: Missing columns"
      }
    ]
  }
}
```

#### Manual Retry (auth required)

```
POST /retry/{file_id}
Headers: Authorization: Bearer <token>
Response 200:
{
  "status": "success",
  "data": {
    "job_id": "uuid",
    "file_id": "uuid",
    "processing_status": "processing",
    "triggered_by": "user@company.co.id"
  }
}

Response 409 (file already completed):
{
  "status": "error",
  "data": null,
  "error": "File is already in 'completed' status. Retry not allowed."
}
```

#### Late Detection (auth required)

```
GET /late?processing_date=2025-01-15
Response 200:
{
  "status": "success",
  "data": [
    {
      "id": "uuid",
      "file_type": "dmaa",
      "processing_date": "2025-01-15",
      "sla_deadline": "06:00:00",
      "detected_at": "2025-01-15T06:00:01+07:00",
      "resolved_at": null,
      "is_resolved": false
    }
  ]
}
```

#### Summary (auth required)

```
GET /summary?processing_date=2025-01-15
Response 200:
{
  "status": "success",
  "data": {
    "processing_date": "2025-01-15",
    "counts": {
      "pending": 0,
      "processing": 1,
      "completed": 1,
      "failed": 1,
      "max_retries_exhausted": 0,
      "late": 1
    },
    "by_file_type": {
      "dmaa": {"pending": 0, "processing": 0, "completed": 1, "failed": 0},
      "itm_cashpos": {"pending": 0, "processing": 1, "completed": 0, "failed": 0},
      "itm_replenish": {"pending": 0, "processing": 0, "completed": 0, "failed": 1}
    }
  }
}
```

#### Audit (auth required)

```
GET /audit?processing_date=2025-01-15&file_type=dmaa&trigger=manual
Response 200:
{
  "status": "success",
  "data": [
    {
      "id": "uuid",
      "event_type": "retry_initiated",
      "trigger": "manual",
      "file_id": "uuid",
      "file_type": "dmaa",
      "file_checksum": "sha256...",
      "processing_date": "2025-01-15",
      "initiated_by": "user@company.co.id",
      "outcome": null,
      "duration_ms": null,
      "error_detail": null,
      "created_at": "2025-01-15T08:00:00+07:00"
    }
  ]
}
```

---

## Data Models

### New Tables (owned by Retry Scheduler)

```sql
-- Tracks detected failed/late files for the retry scheduler's own state.
-- The ETL tables (dmaa_files, itm_cashpos_files, itm_replenish_files) remain
-- the source of truth for ETL processing status.
CREATE TABLE retry_file_tracking (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_type       VARCHAR(20) NOT NULL,  -- 'dmaa', 'itm_cashpos', 'itm_replenish'
    filename        VARCHAR(500) NOT NULL,
    file_path       VARCHAR(1000) NOT NULL,
    file_checksum   VARCHAR(64) NOT NULL,
    processing_date DATE NOT NULL,
    detection_source VARCHAR(20) NOT NULL,  -- 'not_processed', 'input_remaining'
    failure_reason  TEXT,
    processing_status VARCHAR(30) NOT NULL DEFAULT 'failed',
        -- 'pending', 'processing', 'completed', 'failed', 'max_retries_exhausted'
    auto_retry_count INT NOT NULL DEFAULT 0,
    max_retries     INT NOT NULL DEFAULT 3,
    detected_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_retry_at   TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT uq_retry_file_checksum_date UNIQUE (file_checksum, processing_date)
);

CREATE INDEX idx_retry_file_tracking_date ON retry_file_tracking(processing_date);
CREATE INDEX idx_retry_file_tracking_status ON retry_file_tracking(processing_status);
CREATE INDEX idx_retry_file_tracking_type ON retry_file_tracking(file_type);

-- Late detection records
CREATE TABLE late_detections (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_type       VARCHAR(20) NOT NULL,
    processing_date DATE NOT NULL,
    sla_deadline    TIME NOT NULL,
    detected_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at     TIMESTAMPTZ,
    is_resolved     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT uq_late_detection UNIQUE (file_type, processing_date)
);

CREATE INDEX idx_late_detections_date ON late_detections(processing_date);

-- Append-only audit log
CREATE TABLE retry_audit_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type      VARCHAR(30) NOT NULL,  -- 'retry_initiated', 'retry_completed'
    trigger         VARCHAR(10) NOT NULL,  -- 'auto', 'manual'
    file_id         UUID,
    file_type       VARCHAR(20) NOT NULL,
    file_checksum   VARCHAR(64),
    processing_date DATE NOT NULL,
    initiated_by    VARCHAR(200) NOT NULL,  -- 'system' or user ID from token
    outcome         VARCHAR(20),           -- 'completed', 'failed' (null for initiated)
    duration_ms     INT,
    error_detail    TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- No UPDATE or DELETE allowed on this table (enforced at application layer)
CREATE INDEX idx_retry_audit_date ON retry_audit_logs(processing_date);
CREATE INDEX idx_retry_audit_type ON retry_audit_logs(file_type);
CREATE INDEX idx_retry_audit_trigger ON retry_audit_logs(trigger);

-- Scan execution history
CREATE TABLE scan_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scan_type       VARCHAR(20) NOT NULL,  -- 'failure_detection', 'late_detection'
    started_at      TIMESTAMPTZ NOT NULL,
    finished_at     TIMESTAMPTZ,
    status          VARCHAR(20) NOT NULL DEFAULT 'running',  -- 'running', 'success', 'failed'
    files_detected  INT DEFAULT 0,
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_scan_runs_started ON scan_runs(started_at DESC);
```

### Pydantic Models (schemas.py)

```python
from datetime import date, datetime, time
from enum import Enum
from typing import Any
from uuid import UUID

from pydantic import BaseModel


class FileType(str, Enum):
    DMAA = "dmaa"
    ITM_CASHPOS = "itm_cashpos"
    ITM_REPLENISH = "itm_replenish"


class ProcessingStatus(str, Enum):
    PENDING = "pending"
    PROCESSING = "processing"
    COMPLETED = "completed"
    FAILED = "failed"
    MAX_RETRIES_EXHAUSTED = "max_retries_exhausted"


class TriggerType(str, Enum):
    AUTO = "auto"
    MANUAL = "manual"


class APIResponse(BaseModel):
    status: str  # "success" or "error"
    data: Any | None = None
    error: str | None = None


class FileStatusItem(BaseModel):
    file_id: UUID
    filename: str
    checksum: str
    processing_status: ProcessingStatus
    retry_count: int
    max_retries_exhausted: bool
    detected_at: datetime
    last_retry_at: datetime | None
    failure_reason: str | None


class RetryAttempt(BaseModel):
    attempt_number: int
    trigger: TriggerType
    started_at: datetime
    completed_at: datetime | None
    outcome: str | None
    duration_ms: int | None
    error_detail: str | None


class ManualRetryResponse(BaseModel):
    job_id: UUID
    file_id: UUID
    processing_status: ProcessingStatus
    triggered_by: str


class LateDetectionItem(BaseModel):
    id: UUID
    file_type: FileType
    processing_date: date
    sla_deadline: time
    detected_at: datetime
    resolved_at: datetime | None
    is_resolved: bool


class SummaryCount(BaseModel):
    pending: int = 0
    processing: int = 0
    completed: int = 0
    failed: int = 0
    max_retries_exhausted: int = 0
    late: int = 0
```

---

## Configuration (config.py)

```python
from datetime import time
from pathlib import Path

from pydantic import field_validator
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    # Database
    database_url: str = "postgresql+asyncpg://postgres:postgres@localhost:5432/cms"

    # Auth
    auth_secret: str  # JWT secret or API key
    auth_mode: str = "api_key"  # "api_key" or "jwt"

    # SLA Deadlines (WIB = Asia/Jakarta)
    sla_dmaa: str = "06:00"
    sla_itm_cashpos: str = "07:00"
    sla_itm_replenish: str = "07:00"

    # Retry settings
    retry_interval_minutes: int = 30
    max_auto_retries: int = 3
    db_retry_max_attempts: int = 3
    db_retry_base_delay_seconds: float = 1.0

    # Detection schedule (cron)
    scan_cron_start_hour: int = 5
    scan_cron_end_hour: int = 9
    scan_interval_minutes: int = 15

    # Filesystem paths
    ftp_data_root: Path = Path("FTP_DATA")
    dmaa_input_dir: Path | None = None  # defaults to ftp_data_root / "DMAA"
    dmaa_not_processed_dir: Path | None = None  # defaults to dmaa_input_dir / "not_processed"
    itm_cashpos_input_dir: Path | None = None
    itm_replenish_input_dir: Path | None = None

    # ETL script paths
    etl_dmaa_script: Path = Path("scheduler/dmaa/dmaa_etl.py")
    etl_itm_cashpos_script: Path = Path("scheduler/itm/cashpos/itm_cashpos_etl.py")
    etl_itm_replenish_script: Path = Path("scheduler/itm/replenish/itm_replenish_etl.py")

    # Service
    service_version: str = "1.0.0"
    log_level: str = "INFO"

    @field_validator("sla_dmaa", "sla_itm_cashpos", "sla_itm_replenish")
    @classmethod
    def validate_time_format(cls, v: str) -> str:
        """Reject invalid time formats at startup."""
        parts = v.split(":")
        if len(parts) != 2:
            raise ValueError(f"Invalid time format: {v}. Expected HH:MM")
        hour, minute = int(parts[0]), int(parts[1])
        if not (0 <= hour <= 23 and 0 <= minute <= 59):
            raise ValueError(f"Invalid time value: {v}")
        return v

    def get_sla_time(self, file_type: str) -> time:
        mapping = {
            "dmaa": self.sla_dmaa,
            "itm_cashpos": self.sla_itm_cashpos,
            "itm_replenish": self.sla_itm_replenish,
        }
        raw = mapping[file_type]
        h, m = map(int, raw.split(":"))
        return time(h, m)

    class Config:
        env_prefix = "RETRY_"
        env_file = ".env"
```

---

## Core Service Logic

### Detection Service (detector.py)

```python
import asyncio
import hashlib
from datetime import date, datetime
from pathlib import Path
from typing import NamedTuple

from zoneinfo import ZoneInfo

WIB = ZoneInfo("Asia/Jakarta")


class DetectedFile(NamedTuple):
    file_path: Path
    filename: str
    file_type: str
    checksum: str
    detection_source: str  # 'not_processed' or 'input_remaining'
    failure_reason: str


def compute_checksum(path: Path) -> str:
    """SHA-256 checksum of file contents."""
    digest = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


class FileDetector:
    """Scans filesystem directories to find failed/unprocessed files."""

    def __init__(self, settings):
        self.settings = settings

    def scan_not_processed(self, file_type: str) -> list[DetectedFile]:
        """Find files in not_processed directory for a given file type."""
        dir_path = self._get_not_processed_dir(file_type)
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
        """Find files remaining in input directory after ETL window closed."""
        dir_path = self._get_input_dir(file_type)
        pattern = self._get_file_pattern(file_type)
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

    def _get_not_processed_dir(self, file_type: str) -> Path:
        mapping = {
            "dmaa": self.settings.ftp_data_root / "DMAA" / "not_processed",
            "itm_cashpos": self.settings.ftp_data_root / "ITM" / "atm_caspos" / "not_processed",
            "itm_replenish": self.settings.ftp_data_root / "ITM" / "atm_replenish" / "not_processed",
        }
        return mapping[file_type]

    def _get_input_dir(self, file_type: str) -> Path:
        mapping = {
            "dmaa": self.settings.ftp_data_root / "DMAA",
            "itm_cashpos": self.settings.ftp_data_root / "ITM" / "atm_caspos",
            "itm_replenish": self.settings.ftp_data_root / "ITM" / "atm_replenish",
        }
        return mapping[file_type]

    def _get_file_pattern(self, file_type: str) -> str:
        mapping = {
            "dmaa": "Order_All_*.xlsx",
            "itm_cashpos": "ATM_Cashpos_*.csv",
            "itm_replenish": "ATM_Replenish_*.csv",
        }
        return mapping[file_type]


class LateDetector:
    """Checks whether SLA deadlines have passed without completed processing."""

    def __init__(self, settings):
        self.settings = settings

    def check_late(self, file_type: str, processing_date: date, 
                   has_completed: bool) -> bool:
        """Return True if file_type is late for the given processing_date."""
        now_wib = datetime.now(WIB)
        sla_time = self.settings.get_sla_time(file_type)
        deadline = datetime.combine(processing_date, sla_time, tzinfo=WIB)
        return now_wib > deadline and not has_completed
```

### Retry Executor (retry_executor.py)

```python
import asyncio
import subprocess
import time
from pathlib import Path
from uuid import UUID

from .config import Settings


class RetryResult:
    def __init__(self, success: bool, duration_ms: int, 
                 stdout: str, stderr: str, return_code: int):
        self.success = success
        self.duration_ms = duration_ms
        self.stdout = stdout
        self.stderr = stderr
        self.return_code = return_code
        self.error_detail = stderr if not success else None


class RetryExecutor:
    """Invokes ETL scripts via subprocess for retry attempts."""

    def __init__(self, settings: Settings):
        self.settings = settings
        self._script_map = {
            "dmaa": settings.etl_dmaa_script,
            "itm_cashpos": settings.etl_itm_cashpos_script,
            "itm_replenish": settings.etl_itm_replenish_script,
        }

    def get_script_path(self, file_type: str) -> Path:
        """Return the ETL script path for a given file type."""
        return self._script_map[file_type]

    async def execute_retry(self, file_type: str) -> RetryResult:
        """Run the ETL script for the given file type and capture result."""
        script_path = self.get_script_path(file_type)
        start = time.monotonic()

        proc = await asyncio.create_subprocess_exec(
            "python", str(script_path),
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
            cwd=str(script_path.parent),
        )
        stdout_bytes, stderr_bytes = await proc.communicate()
        
        duration_ms = int((time.monotonic() - start) * 1000)
        stdout = stdout_bytes.decode("utf-8", errors="replace")
        stderr = stderr_bytes.decode("utf-8", errors="replace")

        return RetryResult(
            success=(proc.returncode == 0),
            duration_ms=duration_ms,
            stdout=stdout,
            stderr=stderr,
            return_code=proc.returncode or 0,
        )
```

### Scheduler Service (scheduler_service.py)

```python
import asyncio
import logging
from datetime import date, datetime

from apscheduler.schedulers.asyncio import AsyncIOScheduler
from apscheduler.triggers.cron import CronTrigger
from apscheduler.triggers.interval import IntervalTrigger
from zoneinfo import ZoneInfo

from .config import Settings

WIB = ZoneInfo("Asia/Jakarta")
logger = logging.getLogger(__name__)


class SchedulerService:
    """Manages periodic detection scans and retry cycles."""

    def __init__(self, settings: Settings, detector, retry_executor, db):
        self.settings = settings
        self.detector = detector
        self.retry_executor = retry_executor
        self.db = db
        self._scan_lock = asyncio.Lock()
        self._scheduler = AsyncIOScheduler(timezone=WIB)

    def start(self):
        """Register jobs and start the APScheduler."""
        # Failure detection scan: every N minutes within window
        self._scheduler.add_job(
            self._run_failure_scan,
            CronTrigger(
                minute=f"*/{self.settings.scan_interval_minutes}",
                hour=f"{self.settings.scan_cron_start_hour}-{self.settings.scan_cron_end_hour}",
                timezone=WIB,
            ),
            id="failure_detection_scan",
        )

        # Automatic retry cycle: every N minutes
        self._scheduler.add_job(
            self._run_auto_retries,
            IntervalTrigger(minutes=self.settings.retry_interval_minutes),
            id="auto_retry_cycle",
        )

        # Late detection: run at each SLA deadline
        for file_type in ("dmaa", "itm_cashpos", "itm_replenish"):
            sla = self.settings.get_sla_time(file_type)
            self._scheduler.add_job(
                self._run_late_check,
                CronTrigger(hour=sla.hour, minute=sla.minute, timezone=WIB),
                id=f"late_check_{file_type}",
                kwargs={"file_type": file_type},
            )

        self._scheduler.start()

    async def _run_failure_scan(self):
        """Execute failure detection with mutual exclusion."""
        if self._scan_lock.locked():
            logger.warning("Scan already in progress, skipping triggered scan")
            return

        async with self._scan_lock:
            logger.info("Starting failure detection scan")
            # ... scan logic with DB persistence and audit ...

    async def _run_auto_retries(self):
        """Process automatic retries for eligible failed files."""
        # Query files: status='failed', auto_retry_count < max_retries
        # For each file: execute retry, update status, write audit
        # Isolate failures per file
        pass

    async def _run_late_check(self, file_type: str):
        """Check if SLA deadline passed without completion."""
        pass

    def stop(self):
        self._scheduler.shutdown()
```

---

## Error Handling

### Strategy

| Error Type | Behavior |
|-----------|----------|
| ETL script subprocess failure | Capture stderr, mark file `failed`, increment retry count, continue to next file |
| Filesystem inaccessible during scan | Log error, mark scan_run as `failed`, next scheduled scan proceeds normally |
| Database write failure | Retry up to 3 times with exponential backoff (1s, 2s, 4s). If all fail, log and mark operation failed |
| File already processed (checksum match) | Skip silently — idempotency guarantee |
| Concurrent scan trigger | Skip with warning log — mutex via asyncio.Lock |
| Invalid auth token | Return HTTP 401 immediately |
| Manual retry on completed file | Return HTTP 409 with explanation |

### DB Retry Pattern

```python
import asyncio
import logging

logger = logging.getLogger(__name__)


async def with_db_retry(operation, max_attempts: int = 3, base_delay: float = 1.0):
    """Execute a DB operation with exponential backoff retry."""
    for attempt in range(1, max_attempts + 1):
        try:
            return await operation()
        except Exception as e:
            if attempt == max_attempts:
                logger.error("DB operation failed after %d attempts: %s", max_attempts, e)
                raise
            delay = base_delay * (2 ** (attempt - 1))
            logger.warning(
                "DB operation failed (attempt %d/%d), retrying in %.1fs: %s",
                attempt, max_attempts, delay, e,
            )
            await asyncio.sleep(delay)
```

### Failure Isolation Pattern

```python
async def process_file_batch(files: list, executor, db):
    """Process multiple files, isolating failures per file."""
    results = []
    for file in files:
        try:
            result = await executor.execute_retry(file.file_type)
            # ... update status ...
            results.append((file, result))
        except Exception as e:
            logger.exception(
                "Failed to process file %s (type=%s, date=%s): %s",
                file.file_path, file.file_type, file.processing_date, e,
            )
            # Continue to next file — do not break the loop
            results.append((file, None))
    return results
```

---

## Authentication

```python
from fastapi import Depends, HTTPException, Request
from fastapi.security import HTTPBearer

security = HTTPBearer(auto_error=False)


async def require_auth(request: Request):
    """Validate API key or JWT on protected endpoints."""
    auth_header = request.headers.get("Authorization")
    if not auth_header:
        raise HTTPException(status_code=401, detail="Missing authentication credentials")
    
    # Support both "Bearer <token>" and "ApiKey <key>" schemes
    scheme, _, token = auth_header.partition(" ")
    if not token:
        raise HTTPException(status_code=401, detail="Invalid authentication format")

    settings = request.app.state.settings
    if settings.auth_mode == "api_key":
        if token != settings.auth_secret:
            raise HTTPException(status_code=401, detail="Invalid API key")
        return "system"
    else:
        # JWT validation — extract user ID from payload
        payload = validate_jwt(token, settings.auth_secret)
        return payload.get("sub", "unknown")


def extract_user_id(auth_result: str) -> str:
    """Get user identity for audit log. Never logs the token itself."""
    return auth_result
```

---

## State Machine: File Processing Lifecycle

```
                    ┌──────────┐
                    │  (new)   │  file detected in not_processed or input
                    └────┬─────┘
                         │ detection scan
                         ▼
                    ┌──────────┐
            ┌──────│  failed   │◄────────────┐
            │      └────┬─────┘              │
            │           │                     │
            │           │ auto/manual retry    │ retry fails
            │           ▼                     │
            │      ┌──────────┐              │
            │      │processing │──────────────┘
            │      └────┬─────┘
            │           │ retry succeeds
            │           ▼
            │      ┌──────────┐
            │      │completed  │  (terminal — no more retries)
            │      └──────────┘
            │
            │  auto_retry_count >= max_retries
            ▼
    ┌───────────────────────┐
    │ max_retries_exhausted  │  (no more AUTO retries, manual still allowed)
    └───────────────────────┘
            │ manual retry
            ▼
       ┌──────────┐
       │processing │───► completed or back to max_retries_exhausted
       └──────────┘
```

**Transition Rules:**
- `failed` → `processing`: auto retry (if count < max) or manual retry
- `processing` → `completed`: ETL script exits 0
- `processing` → `failed`: ETL script exits non-zero, auto_retry_count incremented
- `failed` → `max_retries_exhausted`: auto_retry_count reaches max_retries
- `max_retries_exhausted` → `processing`: manual retry only (bypasses max count)
- `completed` → *: no transitions allowed (terminal state)

---

## Timezone Handling

All SLA evaluations use Asia/Jakarta (WIB, UTC+7). The `processing_date` is always a calendar date in WIB.

```python
from datetime import date, datetime
from zoneinfo import ZoneInfo

WIB = ZoneInfo("Asia/Jakarta")


def current_processing_date() -> date:
    """Get the current processing date in WIB."""
    return datetime.now(WIB).date()


def is_past_sla(sla_time, processing_date: date) -> bool:
    """Check if current WIB time is past the SLA deadline for a date."""
    now_wib = datetime.now(WIB)
    deadline = datetime.combine(processing_date, sla_time, tzinfo=WIB)
    return now_wib > deadline
```

---

## Security Constraints

1. **No secrets in logs or responses.** Auth tokens, DB credentials, and file contents are never included in log output or API responses. Logs reference files by ID/path only.
2. **Auth on all endpoints except `/health`.** The health endpoint is public for infrastructure probes.
3. **User identity from token to audit.** Manual retry actions extract `sub` claim from JWT (or mark as "api_key_user" for API key auth) and persist in audit log.
4. **Append-only audit.** Application layer enforces no UPDATE/DELETE on `retry_audit_logs`. No ORM method exposes mutation on this table.

---

## Dependencies (requirements.txt)

```
fastapi>=0.115
uvicorn[standard]>=0.30
pydantic>=2.0
pydantic-settings>=2.0
asyncpg>=0.29
sqlalchemy[asyncio]>=2.0
apscheduler>=3.10
python-jose[cryptography]>=3.3  # JWT validation
python-dotenv>=1.0
```

---

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Detection scan completeness

*For any* file present in a monitored directory (not_processed directory or input directory after the ETL window has closed), the detection scan SHALL identify and return that file in its results.

**Validates: Requirements 1.1, 1.3**

### Property 2: Detection idempotency via checksum

*For any* file detected during a scan, if the same file (same checksum) is detected again on a subsequent scan for the same processing_date, the system SHALL NOT create a duplicate record — the existing record is preserved unchanged.

**Validates: Requirements 1.2, 1.4**

### Property 3: Detection record field completeness

*For any* detected file (failed or late), the persisted record SHALL contain all required fields: file path, file checksum, file type, detection timestamp, and failure reason (for failed files) or SLA deadline and processing_date (for late detections).

**Validates: Requirements 1.5, 2.2**

### Property 4: Late status lifecycle

*For any* file type and processing_date where the current WIB time exceeds the SLA deadline AND no completed file exists, the system SHALL report the file type as late. Furthermore, for any late file type that subsequently achieves `completed` status, the late detection record SHALL be updated with a resolution timestamp and `is_resolved = true`.

**Validates: Requirements 2.1, 2.3, 2.4**

### Property 5: SLA configuration validation

*For any* SLA deadline configuration value that does not match a valid `HH:MM` format with hour in [0, 23] and minute in [0, 59], the system SHALL reject the configuration at startup. For any valid configuration, all SLA evaluations SHALL use Asia/Jakarta timezone.

**Validates: Requirements 3.1, 3.3, 3.4**

### Property 6: ETL script routing correctness

*For any* retry attempt, the system SHALL invoke the ETL script that corresponds to the file's file_type: `dmaa` maps to `dmaa_etl.py`, `itm_cashpos` maps to `itm_cashpos_etl.py`, `itm_replenish` maps to `itm_replenish_etl.py`.

**Validates: Requirements 4.2**

### Property 7: Max auto-retry invariant

*For any* file tracked by the retry scheduler, the automatic retry count SHALL never exceed the configured maximum. Once `auto_retry_count >= max_retries` AND status is still `failed`, the system SHALL transition status to `max_retries_exhausted` and cease automatic retries.

**Validates: Requirements 4.3, 4.4**

### Property 8: Successful retry terminates loop

*For any* file where a retry attempt results in the ETL script exiting with code 0, the system SHALL update the file status to `completed` and SHALL NOT execute any further automatic or scheduled retries for that file.

**Validates: Requirements 4.5**

### Property 9: Retry idempotency via checksum

*For any* file, executing the ETL script multiple times against the same file content (same SHA-256 checksum) SHALL NOT produce duplicate data rows in the database — the ETL script's own checksum-based idempotency guarantees this.

**Validates: Requirements 4.6**

### Property 10: Manual retry bypasses max count

*For any* manual retry request on a file with status `failed` or `max_retries_exhausted`, regardless of the current `auto_retry_count`, the system SHALL execute the retry.

**Validates: Requirements 5.3**

### Property 11: Manual retry rejects completed files

*For any* manual retry request targeting a file with `processing_status = 'completed'`, the system SHALL reject the request with an error response and SHALL NOT initiate a retry.

**Validates: Requirements 5.2**

### Property 12: API response envelope consistency

*For any* API endpoint response (success or error), the response body SHALL contain exactly the fields `status`, `data`, and `error` in the JSON envelope.

**Validates: Requirements 6.5**

### Property 13: Summary count accuracy

*For any* processing_date, the summary endpoint's counts (pending, processing, completed, failed, max_retries_exhausted, late) SHALL exactly equal the actual count of records in those respective states for that date.

**Validates: Requirements 6.4**

### Property 14: Audit trail completeness

*For any* retry attempt initiation, the audit log SHALL contain: trigger type, file identifier, file type, file checksum, processing_date, initiated_by, and timestamp. *For any* retry attempt completion, the audit log SHALL contain: file identifier, outcome, duration, and error detail (if failed).

**Validates: Requirements 7.1, 7.2**

### Property 15: Audit immutability

*For any* sequence of operations performed by the system, previously written audit log entries SHALL never be modified or deleted. The count of audit log entries SHALL be monotonically non-decreasing.

**Validates: Requirements 7.5**

### Property 16: Audit filtering correctness

*For any* filter combination (processing_date, file_type, trigger_type) applied to the audit endpoint, all returned records SHALL match the provided filter criteria and no matching records SHALL be omitted.

**Validates: Requirements 7.4**

### Property 17: Scan mutual exclusion

*For any* overlapping scan triggers, at most one detection scan SHALL execute concurrently. Additional triggers while a scan is in progress SHALL be skipped with a warning log.

**Validates: Requirements 8.3**

### Property 18: Failure isolation per file

*For any* batch of files being processed (detection or retry), a failure in processing one file SHALL NOT prevent the processing of other files in the same batch. All non-failing files SHALL complete their processing.

**Validates: Requirements 10.1, 10.4**

### Property 19: DB retry with exponential backoff

*For any* database write failure, the system SHALL retry the operation up to the configured maximum attempts (default 3) with exponential backoff delays. If all attempts fail, the operation SHALL be marked as failed.

**Validates: Requirements 10.3**

### Property 20: Authentication enforcement

*For any* request to a protected endpoint (all endpoints except `/health`) that lacks valid authentication credentials, the system SHALL return HTTP 401 and SHALL NOT execute the requested operation.

**Validates: Requirements 11.1, 11.2**

### Property 21: Sensitive data exclusion from logs and responses

*For any* log entry or API response produced by the system, authentication tokens, database credentials, and raw file contents SHALL NOT appear in the output.

**Validates: Requirements 11.4**

### Property 22: User identity propagation

*For any* manual retry triggered with a valid authentication token, the user identity extracted from the token SHALL be recorded in the corresponding audit log entry's `initiated_by` field.

**Validates: Requirements 11.3**
