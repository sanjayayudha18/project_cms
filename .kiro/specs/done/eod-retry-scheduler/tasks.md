# Implementation Plan: EOD Retry Scheduler

## Overview

A standalone Python FastAPI microservice that detects failed and late ETL file processing, provides automated and manual retry capabilities, and exposes a REST API for the Go backend and EOD Monitoring frontend. The service lives under `scheduler/retry_scheduler/` and uses APScheduler for background tasks, asyncpg for database access, and subprocess invocation for ETL retries.

## Tasks

- [x] 1. Project setup and configuration
  - [x] 1.1 Create project structure and dependencies
    - Create `scheduler/retry_scheduler/` directory structure per design (routers/, services/, utils/)
    - Create `__init__.py` files for all packages
    - Create `requirements.txt` with pinned dependencies: fastapi, uvicorn, pydantic, pydantic-settings, asyncpg, apscheduler, python-jose[cryptography], python-dotenv (sqlalchemy dropped -- see task 2.3 note)
    - Create a `.env.example` with all RETRY_* environment variables
    - _Requirements: 3.1, 3.2_

  - [x] 1.2 Implement configuration module (`config.py`)
    - Implement `Settings` class using pydantic-settings with all configurable values (SLA deadlines, retry intervals, max retries, filesystem paths, ETL script paths, DB URL, auth secret)
    - Implement `field_validator` for SLA time format validation that rejects invalid formats at startup
    - Implement `get_sla_time()` helper to map file_type to its SLA deadline as `datetime.time`
    - Implement computed defaults for directory paths (dmaa_input_dir defaults from ftp_data_root, etc.)
    - _Requirements: 3.1, 3.2, 3.3, 3.4_

  - [ ]* 1.3 Write unit tests for configuration validation
    - Test valid time formats are accepted
    - Test invalid time formats raise ValidationError at startup
    - Test default values are correctly applied
    - Test `get_sla_time()` returns correct time per file_type
    - **Property 5: SLA configuration validation**
    - **Validates: Requirements 3.1, 3.3, 3.4**

- [x] 2. Database layer
  - [x] 2.1 Create SQL migration files for retry scheduler tables
    - Create migration file with `retry_file_tracking`, `late_detections`, `retry_audit_logs`, and `scan_runs` tables
    - Include all indexes defined in the design (on processing_date, processing_status, file_type, started_at)
    - Include UNIQUE constraints (uq_retry_file_checksum_date, uq_late_detection)
    - _Requirements: 1.2, 1.5, 2.2, 7.3_

  - [x] 2.2 Implement database connection module (`database.py`)
    - Create async connection pool using asyncpg
    - Implement `get_db_pool()` lifespan context for FastAPI
    - Implement connection health check function for the `/health` endpoint
    - _Requirements: 9.1, 9.2_

  - [x] 2.3 Implement SQLAlchemy models (`models.py`) -- SKIPPED, simplified per ponytail
    - Decision: no ORM layer. The service is thin (4 tables, straightforward CRUD, no
      relationships to traverse) so raw asyncpg Records are used directly in
      services/routers instead of a SQLAlchemy model layer. sqlalchemy dropped from
      requirements.txt. Migration 012 remains the single source of truth for field names
      (e.g. `trigger_type`, not `trigger`) and every query against it spells those names
      out literally.
    - _Requirements: 1.5, 2.2, 7.3, 7.5_

- [x] 3. Pydantic schemas and response envelope
  - [x] 3.1 Implement request/response schemas (`schemas.py`)
    - Define enums: `FileType`, `ProcessingStatus`, `TriggerType`
    - Define `APIResponse` envelope model with `status`, `data`, `error` fields
    - Define `FileStatusItem`, `RetryAttempt`, `ManualRetryResponse`, `LateDetectionItem`, `SummaryCount` models
    - Define request query parameter models for filtering (processing_date, file_type, trigger)
    - _Requirements: 6.5, 6.1, 6.2, 6.3, 6.4_

  - [ ]* 3.2 Write unit tests for schema validation and serialization
    - Test APIResponse always contains status, data, error fields
    - Test FileType enum only allows valid values
    - Test ProcessingStatus transitions match state machine
    - **Property 12: API response envelope consistency**
    - **Validates: Requirements 6.5**

- [x] 4. Authentication and security middleware
  - [x] 4.1 Implement authentication dependency (`dependencies.py`)
    - Implement `require_auth()` FastAPI dependency supporting both API key and JWT modes
    - Implement JWT validation using python-jose: extract `sub` claim for user identity
    - Return HTTP 401 for missing or invalid credentials
    - Implement `extract_user_id()` helper that never logs the token itself
    - _Requirements: 11.1, 11.2, 11.3, 11.4_

  - [ ]* 4.2 Write unit tests for authentication
    - Test missing auth header returns 401
    - Test invalid API key returns 401
    - Test valid API key returns user identity
    - Test valid JWT returns extracted sub claim
    - Test tokens are never included in error responses
    - **Property 20: Authentication enforcement**
    - **Property 21: Sensitive data exclusion from logs and responses**
    - **Validates: Requirements 11.1, 11.2, 11.4**

- [x] 5. Utility modules
  - [x] 5.1 Implement checksum utility (`utils/checksum.py`)
    - Implement `compute_checksum(path: Path) -> str` using SHA-256 with chunked reading (1MB chunks)
    - _Requirements: 1.2, 4.6_

  - [x] 5.2 Implement timezone utility (`utils/timezone.py`)
    - Implement `WIB` timezone constant using `zoneinfo.ZoneInfo("Asia/Jakarta")`
    - Implement `current_processing_date() -> date` returning today in WIB
    - Implement `is_past_sla(sla_time, processing_date) -> bool` checking if current WIB time exceeds deadline
    - _Requirements: 3.4, 2.1_

- [x] 6. Checkpoint - Ensure foundational modules work
  - Ensure all tests pass, ask the user if questions arise. (py_compile clean; no venv to run pytest -- see final report)

- [x] 7. File failure detection service
  - [x] 7.1 Implement FileDetector class (`services/detector.py`)
    - Implement `scan_not_processed(file_type)` — scans the not_processed directory for each file type, computes checksums, returns `DetectedFile` named tuples
    - Implement `scan_input_remaining(file_type)` — scans input directory for files matching type-specific glob patterns that remain after ETL window
    - Implement directory path mapping per file_type (DMAA → `FTP_DATA/DMAA/not_processed`, ITM cashpos → `FTP_DATA/ITM/atm_caspos/not_processed`, ITM replenish → `FTP_DATA/ITM/atm_replenish/not_processed`)
    - Implement file pattern mapping (dmaa → `Order_All_*.xlsx`, itm_cashpos → `ATM_Cashpos_*.csv`, itm_replenish → `ATM_Replenish_*.csv`)
    - _Requirements: 1.1, 1.3, 1.5_

  - [x] 7.2 Implement detection persistence logic
    - Implement `persist_detected_files()` — inserts new detected files into `retry_file_tracking` using file_checksum + processing_date as idempotency key (ON CONFLICT DO NOTHING)
    - Implement `record_scan_run()` — creates/updates `scan_runs` entry with start time, end time, files detected count, and status
    - _Requirements: 1.2, 1.4, 1.5, 8.4_

  - [ ]* 7.3 Write property tests for detection
    - Test: scanning directory with files returns all files (Property 1: Detection scan completeness)
    - Test: same file detected twice does not create duplicate record (Property 2: Detection idempotency via checksum)
    - Test: detected file record contains all required fields (Property 3: Detection record field completeness)
    - **Property 1: Detection scan completeness**
    - **Property 2: Detection idempotency via checksum**
    - **Property 3: Detection record field completeness**
    - **Validates: Requirements 1.1, 1.2, 1.3, 1.4, 1.5**

- [x] 8. Late detection service
  - [x] 8.1 Implement LateDetector class (`services/detector.py`)
    - Implement `check_late(file_type, processing_date, has_completed) -> bool` — checks if current WIB time exceeds SLA deadline and no completed file exists
    - Implement `persist_late_detection()` — creates `late_detections` record with UNIQUE constraint on (file_type, processing_date)
    - Implement `resolve_late_detection()` — updates late record with resolved_at timestamp and is_resolved=True when file achieves completed status
    - _Requirements: 2.1, 2.2, 2.3, 2.4_

  - [ ]* 8.2 Write property tests for late detection lifecycle
    - Test: file type reports late when past SLA and no completed file exists
    - Test: late status resolves when file achieves completed status
    - Test: late detection record contains all required fields
    - **Property 4: Late status lifecycle**
    - **Validates: Requirements 2.1, 2.3, 2.4**

- [x] 9. Retry executor service
  - [x] 9.1 Implement RetryExecutor class (`services/retry_executor.py`)
    - Implement ETL script path mapping: dmaa → `dmaa_etl.py`, itm_cashpos → `itm_cashpos_etl.py`, itm_replenish → `itm_replenish_etl.py`
    - Implement `execute_retry(file_type) -> RetryResult` using `asyncio.create_subprocess_exec` with stdout/stderr capture
    - Return `RetryResult` containing success flag, duration_ms, stdout, stderr, return_code, error_detail
    - Set working directory to the script's parent directory
    - _Requirements: 4.2, 4.6_

  - [x] 9.2 Implement retry orchestration logic
    - Implement `process_auto_retry(file)` — check auto_retry_count < max_retries, transition status to `processing`, invoke executor, update status to `completed` or `failed`, increment retry count, write audit logs
    - Implement `process_manual_retry(file_id, user_id)` — bypass max retry count check, validate file not in `completed` status, invoke executor, write audit logs
    - Implement state machine transitions as per design (failed → processing → completed/failed, max_retries_exhausted → processing via manual only)
    - _Requirements: 4.1, 4.3, 4.4, 4.5, 5.1, 5.2, 5.3_

  - [ ]* 9.3 Write property tests for retry logic
    - Test: retry invokes correct ETL script per file_type (Property 6)
    - Test: auto_retry_count never exceeds max (Property 7)
    - Test: successful retry sets status to completed and stops further retries (Property 8)
    - Test: manual retry bypasses max count (Property 10)
    - Test: manual retry on completed file is rejected (Property 11)
    - **Property 6: ETL script routing correctness**
    - **Property 7: Max auto-retry invariant**
    - **Property 8: Successful retry terminates loop**
    - **Property 10: Manual retry bypasses max count**
    - **Property 11: Manual retry rejects completed files**
    - **Validates: Requirements 4.2, 4.3, 4.4, 4.5, 5.2, 5.3**

- [x] 10. Audit logging service
  - [x] 10.1 Implement AuditService class (`services/audit_service.py`)
    - Implement `log_retry_initiated(trigger, file_id, file_type, file_checksum, processing_date, initiated_by)` — inserts initiation record
    - Implement `log_retry_completed(file_id, outcome, duration_ms, error_detail)` — inserts completion record
    - Enforce append-only: no update/delete methods on audit records
    - _Requirements: 7.1, 7.2, 7.3, 7.5_

  - [ ]* 10.2 Write property tests for audit logging
    - Test: every retry initiation creates an audit entry with all required fields (Property 14)
    - Test: audit entries are never modified or deleted (Property 15)
    - **Property 14: Audit trail completeness**
    - **Property 15: Audit immutability**
    - **Validates: Requirements 7.1, 7.2, 7.5**

- [x] 11. Error handling patterns
  - [x] 11.1 Implement DB retry utility with exponential backoff
    - Implement `with_db_retry(operation, max_attempts=3, base_delay=1.0)` async function
    - Exponential backoff: 1s, 2s, 4s delays between attempts
    - Log each failure attempt with context, raise after all attempts exhausted
    - Landed in `database.py` (not a separate `utils/resilience.py`) -- one home for DB concerns.
    - _Requirements: 10.3_

  - [x] 11.2 Implement failure isolation in batch processing
    - Implemented inline in `services/scheduler_service.py` (`_run_auto_retries`
      try/except-per-file loop, and `_run_failure_scan`'s try/except around each
      file_type's scan+persist) rather than as a standalone `process_file_batch()` helper
      -- the pattern is tightly coupled to retry orchestration state, so a generic helper
      would be an unused abstraction for a single caller.
    - Ensure unrecoverable errors include stack trace in log (`logger.exception(...)`)
    - _Requirements: 10.1, 10.2, 10.4, 10.5_

  - [ ]* 11.3 Write unit tests for error handling
    - Test: DB retry retries up to max attempts with increasing delays (Property 19)
    - Test: single file failure does not prevent processing of other files (Property 18)
    - **Property 18: Failure isolation per file**
    - **Property 19: DB retry with exponential backoff**
    - **Validates: Requirements 10.1, 10.3, 10.4**

- [x] 12. Checkpoint - Ensure all service modules work
  - Ensure all tests pass, ask the user if questions arise. (py_compile clean; see final report)

- [x] 13. Scheduler service (APScheduler integration)
  - [x] 13.1 Implement SchedulerService class (`services/scheduler_service.py`)
    - Register failure detection scan job with CronTrigger (every N minutes within configured hour window, default 05:00–09:00 WIB every 15 min)
    - Register automatic retry cycle job with IntervalTrigger (default every 30 minutes)
    - Register late detection check jobs at each SLA deadline time (one job per file_type)
    - Implement `asyncio.Lock` for scan mutual exclusion — skip with warning if lock is held
    - Implement `start()` and `stop()` lifecycle methods
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 4.1_

  - [ ]* 13.2 Write unit tests for scheduler mutual exclusion
    - Test: concurrent scan trigger is skipped when lock is held (Property 17)
    - Test: scheduler registers correct number of jobs for all file types
    - **Property 17: Scan mutual exclusion**
    - **Validates: Requirements 8.3**

- [x] 14. REST API routers
  - [x] 14.1 Implement health endpoint (`routers/health.py`)
    - `GET /health` — no auth required, check DB connectivity and filesystem accessibility
    - Return 200 with service version, uptime, last successful scan timestamp, db status, filesystem status
    - Return 503 with diagnostic message if DB or filesystem is inaccessible
    - _Requirements: 9.1, 9.2, 9.3, 9.4_

  - [x] 14.2 Implement status endpoints (`routers/status.py`)
    - `GET /status?processing_date=` — return all file statuses grouped by file_type for a given date
    - `GET /status/{file_id}/history` — return retry attempt history for a specific file
    - Return empty data set (HTTP 200) when no records exist for the date
    - Auth required on both endpoints
    - _Requirements: 6.1, 6.2, 6.6_

  - [x] 14.3 Implement manual retry endpoint (`routers/retry.py`)
    - `POST /retry/{file_id}` — validate file exists and is not completed, queue retry, return job_id + status
    - Return HTTP 409 if file is already completed
    - Extract user identity from auth token for audit
    - Return job_id and current processing_status
    - Auth required
    - _Requirements: 5.1, 5.2, 5.3, 5.4_

  - [x] 14.4 Implement late detection endpoint (`routers/late.py`)
    - `GET /late?processing_date=` — return all late detection records for the given date
    - Return empty data set (HTTP 200) when no records exist
    - Auth required
    - _Requirements: 6.3, 6.6_

  - [x] 14.5 Implement summary endpoint (`routers/summary.py`)
    - `GET /summary?processing_date=` — return counts of pending, processing, completed, failed, max_retries_exhausted, and late files for the date
    - Include per-file_type breakdown
    - Auth required
    - _Requirements: 6.4, 6.6_

  - [x] 14.6 Implement audit endpoint (`routers/audit.py`)
    - `GET /audit?processing_date=&file_type=&trigger=` — return audit log entries with optional filters
    - All filter parameters are optional; combine with AND logic
    - Auth required
    - _Requirements: 7.4_

  - [ ]* 14.7 Write integration tests for API endpoints
    - Test: all protected endpoints return 401 without auth (Property 20)
    - Test: status endpoint returns correct grouped data
    - Test: summary counts match actual DB record counts (Property 13)
    - Test: audit filters return only matching records (Property 16)
    - Test: empty processing_date returns 200 with empty data, not error
    - **Property 13: Summary count accuracy**
    - **Property 16: Audit filtering correctness**
    - **Property 20: Authentication enforcement**
    - **Validates: Requirements 6.1, 6.4, 6.6, 7.4, 11.1**

- [x] 15. FastAPI application factory and lifespan
  - [x] 15.1 Implement main application (`main.py`)
    - Create FastAPI app factory with lifespan handler
    - Initialize DB pool on startup, close on shutdown
    - Initialize SchedulerService on startup, stop on shutdown
    - Store settings and services in `app.state`
    - Include all routers with appropriate prefixes
    - Add exception handlers for consistent error response envelope
    - Track service start time for uptime calculation
    - _Requirements: 9.1, 9.4, 8.1_

- [x] 16. Checkpoint - Full integration validation
  - Ensure all tests pass, ask the user if questions arise. (py_compile clean across all modules; no venv available to boot the app or hit a live DB per task brief -- flagged as an open item, see final report)

- [x] 17. Wire everything together and validate
  - [x] 17.1 Create run script and Docker configuration
    - Create `scheduler/retry_scheduler/run.py` with uvicorn startup
    - All routers mounted and services initialized on startup in `main.py`'s lifespan (reviewed by
      inspection; not executed, no venv/live DB per task brief)
    - Detection scan -> persist -> auto-retry -> audit flow traced by inspection through
      `SchedulerService._run_failure_scan` / `_run_auto_retries` / `process_auto_retry`
    - Manual retry -> audit -> status update flow traced by inspection through
      `routers/retry.py` / `SchedulerService.process_manual_retry`
    - Docker configuration NOT created -- not requested in scope (task brief says scheduler/
      + one migration file only; no Dockerfile was part of "already done" or asked for)
    - _Requirements: 4.1, 5.1, 7.1, 8.1_

  - [ ]* 17.2 Write end-to-end integration test for full retry flow
    - Test: file detected → auto retry → status update → audit logged
    - Test: manual retry on exhausted file → processes → audit logged
    - Test: late detection → resolved on completion → audit logged
    - **Property 22: User identity propagation**
    - **Validates: Requirements 4.1, 5.1, 5.3, 7.1, 11.3**

- [x] 18. Final checkpoint - Ensure all tests pass
  - `python -m py_compile` clean across every new .py file. Per-module smoke checks
    (`if __name__ == "__main__"` demos) added to checksum.py, timezone.py, database.py,
    dependencies.py, detector.py, retry_executor.py, audit_service.py, scheduler_service.py
    per the ponytail "one runnable check per non-trivial module" convention -- not run in
    this session (no venv / deps installed, per task brief instructions not to). See final
    report for how to run them once deps are installed.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document
- The service uses the same PostgreSQL instance as the main CMS backend but owns its own tables
- ETL scripts are invoked via subprocess — no ETL logic duplication in the retry scheduler
- All SLA evaluations use Asia/Jakarta (WIB) timezone
- Audit logs are append-only; no UPDATE/DELETE operations are exposed

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "1.2"] },
    { "id": 1, "tasks": ["1.3", "2.1", "5.1", "5.2"] },
    { "id": 2, "tasks": ["2.2", "2.3", "3.1"] },
    { "id": 3, "tasks": ["3.2", "4.1"] },
    { "id": 4, "tasks": ["4.2", "7.1", "8.1", "9.1", "10.1", "11.1", "11.2"] },
    { "id": 5, "tasks": ["7.2", "7.3", "8.2", "9.2", "10.2", "11.3"] },
    { "id": 6, "tasks": ["9.3", "13.1"] },
    { "id": 7, "tasks": ["13.2", "14.1", "14.2", "14.3", "14.4", "14.5", "14.6"] },
    { "id": 8, "tasks": ["14.7", "15.1"] },
    { "id": 9, "tasks": ["17.1"] },
    { "id": 10, "tasks": ["17.2"] }
  ]
}
```
