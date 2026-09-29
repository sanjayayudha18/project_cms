"""import_jobs registry helper (.claude/sdlc/import-export-jobs, spec FR1-FR10, FR18).

Single owner of every import_jobs status transition in Python. The Go
counterpart (backend/internal/importjob) is deferred to forecast 2.1 (plan R4)
and must match the behaviour here.

Every function takes an asyncpg connection and opens its own
`conn.transaction()`; inside a caller's transaction that becomes a savepoint, so
`complete()` can commit atomically with the caller's business writes (FR6) and a
caught 23505 never aborts the caller's outer transaction.

Concurrency lives in the partial unique indexes of migration 019 (NFR1):
  import_jobs_hash_uq      -> duplicate file (FR2)
  import_jobs_inflight_uq  -> JobInProgressError (FR4)
  import_jobs_current_uq   -> one completed version per source+date (FR6)

Audit (FR18): a transition with an actor_id also writes audit_logs in the same
transaction. System transitions (detect, stale, auto retry) only log here;
retry events keep going to retry_audit_logs via services.audit_service.
"""
from __future__ import annotations

import json
import logging
from datetime import date, timedelta

import asyncpg

logger = logging.getLogger(__name__)

DEFAULT_STALE_AFTER = timedelta(minutes=15)  # FR8 / S5; batch ETL sources override via service config (C1)
MAX_ERROR_MESSAGE = 1000  # FR7

# FR10. failed/max_retries_exhausted -> pending only via register() (FR3).
ALLOWED_TRANSITIONS: dict[str, frozenset[str]] = {
    "pending": frozenset({"processing", "failed"}),
    "processing": frozenset({"completed", "failed"}),
    "completed": frozenset({"superseded"}),
    "failed": frozenset({"pending", "processing", "max_retries_exhausted"}),
    "max_retries_exhausted": frozenset({"pending", "processing"}),
    "superseded": frozenset(),
}

_HASH_UQ = "import_jobs_hash_uq"
_INFLIGHT_UQ = "import_jobs_inflight_uq"
_LIVE_DUPLICATE = ("pending", "processing", "completed")


class JobInProgressError(Exception):
    """Another pending/processing job exists for the same source + processing_date (FR4, HTTP 409)."""


class IllegalTransitionError(Exception):
    """Status change not allowed by FR10."""


class JobNotFoundError(Exception):
    """No import_jobs row with that id."""


def check_transition(from_status: str, to_status: str) -> None:
    if to_status not in ALLOWED_TRANSITIONS.get(from_status, frozenset()):
        raise IllegalTransitionError(f"import_job transition {from_status} -> {to_status} not allowed")


def truncate_error(message: str | None) -> str | None:
    if message is None:
        return None
    return message[:MAX_ERROR_MESSAGE]


# -- public API ---------------------------------------------------------------


async def register(
    conn: asyncpg.Connection,
    *,
    source: str,
    processing_date: date | None,
    file_hash: str,
    original_filename: str,
    runtime: str = "python",
    file_path: str | None = None,
    created_by: int | None = None,
    stale_after: timedelta = DEFAULT_STALE_AFTER,
    ip: str | None = None,
) -> tuple[asyncpg.Record, bool]:
    """Register a file before processing it (FR1). Returns (job, duplicate).

    duplicate=True -> same hash already pending/processing/completed; caller stops (FR2).
    A previously failed hash reuses its row, back to pending (FR3).
    Raises JobInProgressError when another run for source+date is in flight (FR4).
    """
    async with conn.transaction():
        if processing_date is not None:
            await mark_stale(conn, source=source, stale_after=stale_after, processing_date=processing_date)

        live = await conn.fetchrow(
            "SELECT * FROM import_jobs WHERE source = $1 AND file_hash = $2 AND status <> 'superseded' FOR UPDATE",
            source, file_hash,
        )
        if live is not None:
            if live["status"] in _LIVE_DUPLICATE:
                return live, True
            job = await _to_in_flight(
                conn, live, "pending", action="register",
                sets="last_retry_at = now(), error_message = NULL, started_at = NULL, finished_at = NULL",
                actor_id=created_by, ip=ip,
            )
            return job, False

        version, supersedes = 1, None
        if processing_date is not None:
            current_row = await current(conn, source=source, processing_date=processing_date)
            if current_row is not None:
                version, supersedes = current_row["version"] + 1, current_row["id"]

        try:
            async with conn.transaction():
                job = await conn.fetchrow(
                    """
                    INSERT INTO import_jobs
                        (source, processing_date, file_hash, original_filename, file_path, runtime,
                         version, supersedes_job_id, created_by)
                    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
                    RETURNING *
                    """,
                    source, processing_date, file_hash, original_filename, file_path, runtime,
                    version, supersedes, created_by,
                )
        except asyncpg.UniqueViolationError as e:
            if e.constraint_name == _INFLIGHT_UQ:
                raise JobInProgressError(f"import already in progress for {source} {processing_date}") from e
            if e.constraint_name == _HASH_UQ:  # lost a race with an identical upload
                raced = await conn.fetchrow(
                    "SELECT * FROM import_jobs WHERE source = $1 AND file_hash = $2 AND status <> 'superseded'",
                    source, file_hash,
                )
                return raced, True
            raise

        await _audit(conn, job["id"], "register", None, "pending", actor_id=created_by, ip=ip)
        return job, False


async def detect(
    conn: asyncpg.Connection,
    *,
    source: str,
    processing_date: date,
    file_hash: str,
    original_filename: str,
    file_path: str | None,
    detection_source: str,
    error_message: str | None = None,
) -> asyncpg.Record | None:
    """Record a file the retry scheduler found unprocessed (FR12). Returns the new row, or
    None when the hash is already tracked. Never resets an existing row: register()'s FR3
    reset here would revive failed files on every scan and bypass max_retries (plan P6)."""
    try:
        async with conn.transaction():
            job = await conn.fetchrow(
                """
                INSERT INTO import_jobs
                    (source, processing_date, file_hash, original_filename, file_path, runtime,
                     detection_source, status, error_message)
                VALUES ($1, $2, $3, $4, $5, 'python', $6, 'failed', $7)
                RETURNING *
                """,
                source, processing_date, file_hash, original_filename, file_path,
                detection_source, truncate_error(error_message),
            )
    except asyncpg.UniqueViolationError as e:
        if e.constraint_name == _HASH_UQ:
            return None
        raise
    logger.info("import_job %s detected: source=%s date=%s via %s", job["id"], source, processing_date, detection_source)
    return job


async def start(
    conn: asyncpg.Connection, job_id: int, *, actor_id: int | None = None, ip: str | None = None,
) -> asyncpg.Record:
    """pending -> processing, or a retry of a failed/max_retries_exhausted job.
    Raises JobInProgressError if another run for the same source+date is in flight (FR4)."""
    async with conn.transaction():
        job = await _lock(conn, job_id)
        is_retry = job["status"] in ("failed", "max_retries_exhausted")
        sets = "started_at = now(), finished_at = NULL"
        if is_retry:
            sets += ", last_retry_at = now()"
        return await _to_in_flight(
            conn, job, "processing", action="retry" if is_retry else "start",
            sets=sets, actor_id=actor_id, ip=ip,
        )


async def complete(
    conn: asyncpg.Connection,
    job_id: int,
    *,
    row_count: int | None = None,
    error_count: int | None = None,
    actor_id: int | None = None,
    ip: str | None = None,
) -> asyncpg.Record:
    """processing -> completed; the previous completed version of source+date becomes
    superseded in the same transaction (FR6). Pass the caller's connection while it is
    inside its own transaction to make business writes + completion atomic."""
    async with conn.transaction():
        job = await _lock(conn, job_id)
        check_transition(job["status"], "completed")
        if job["processing_date"] is not None:
            previous = await conn.fetchrow(
                """
                SELECT * FROM import_jobs
                WHERE source = $1 AND processing_date = $2 AND status = 'completed' AND id <> $3
                FOR UPDATE
                """,
                job["source"], job["processing_date"], job_id,
            )
            if previous is not None:
                await _transition(conn, previous, "superseded", action="supersede", actor_id=actor_id, ip=ip)
        return await _transition(
            conn, job, "completed", action="complete",
            sets="finished_at = now(), error_message = NULL, row_count = $2, error_count = $3",
            params=(row_count, error_count), actor_id=actor_id, ip=ip,
        )


async def fail(
    conn: asyncpg.Connection,
    job_id: int,
    error_message: str,
    *,
    auto_retry: bool = False,
    actor_id: int | None = None,
    ip: str | None = None,
) -> asyncpg.Record:
    """pending/processing -> failed (FR7); the previous completed version stays current.
    auto_retry=True counts the attempt and moves to max_retries_exhausted once
    auto_retry_count reaches max_retries (FR13). Manual retries never count."""
    async with conn.transaction():
        job = await _lock(conn, job_id)
        sets = "finished_at = now(), error_message = $2"
        if auto_retry:
            sets += ", auto_retry_count = auto_retry_count + 1"
        job = await _transition(
            conn, job, "failed", action="fail", sets=sets,
            params=(truncate_error(error_message),), actor_id=actor_id, ip=ip,
        )
        if auto_retry and job["auto_retry_count"] >= job["max_retries"]:
            job = await _transition(conn, job, "max_retries_exhausted", action="exhaust")
        return job


async def current(
    conn: asyncpg.Connection, *, source: str, processing_date: date,
) -> asyncpg.Record | None:
    """Current completed version for source + processing_date (FR9)."""
    return await conn.fetchrow(
        "SELECT * FROM import_jobs WHERE source = $1 AND processing_date = $2 AND status = 'completed'",
        source, processing_date,
    )


async def mark_stale(
    conn: asyncpg.Connection,
    *,
    source: str,
    stale_after: timedelta = DEFAULT_STALE_AFTER,
    processing_date: date | None = None,
) -> list[int]:
    """pending/processing rows untouched for longer than stale_after -> failed (FR8).
    processing_date=None sweeps every date of the source (auto-retry cycle, T5.2)."""
    rows = await conn.fetch(
        """
        UPDATE import_jobs
        SET status = 'failed', finished_at = now(),
            error_message = 'stale: no progress for ' || $2::interval::text
        WHERE source = $1 AND status IN ('pending', 'processing')
          AND updated_at < now() - $2::interval
          AND ($3::date IS NULL OR processing_date = $3)
        RETURNING id
        """,
        source, stale_after, processing_date,
    )
    ids = [r["id"] for r in rows]
    if ids:
        logger.warning("import_jobs marked stale (source=%s date=%s): %s", source, processing_date, ids)
    return ids


# -- internals ----------------------------------------------------------------


async def _lock(conn: asyncpg.Connection, job_id: int) -> asyncpg.Record:
    job = await conn.fetchrow("SELECT * FROM import_jobs WHERE id = $1 FOR UPDATE", job_id)
    if job is None:
        raise JobNotFoundError(f"import_job {job_id} not found")
    return job


async def _to_in_flight(conn: asyncpg.Connection, job: asyncpg.Record, to_status: str, **kwargs) -> asyncpg.Record:
    """Transition into pending/processing inside a savepoint; inflight_uq -> JobInProgressError (FR4)."""
    try:
        async with conn.transaction():
            return await _transition(conn, job, to_status, **kwargs)
    except asyncpg.UniqueViolationError as e:
        if e.constraint_name == _INFLIGHT_UQ:
            raise JobInProgressError(
                f"import already in progress for {job['source']} {job['processing_date']}"
            ) from e
        raise


async def _transition(
    conn: asyncpg.Connection,
    job: asyncpg.Record,
    to_status: str,
    *,
    action: str,
    sets: str = "",
    params: tuple = (),
    actor_id: int | None = None,
    ip: str | None = None,
) -> asyncpg.Record:
    """Validate against FR10, update, audit. `sets` is a fixed SQL fragment from this
    module (never caller input); its placeholders start at $2 and bind `params`."""
    check_transition(job["status"], to_status)
    set_clause = f"status = ${len(params) + 2}" + (f", {sets}" if sets else "")
    updated = await conn.fetchrow(
        f"UPDATE import_jobs SET {set_clause} WHERE id = $1 RETURNING *",
        job["id"], *params, to_status,
    )
    await _audit(conn, job["id"], action, job["status"], to_status, actor_id=actor_id, ip=ip)
    return updated


async def _audit(
    conn: asyncpg.Connection,
    job_id: int,
    action: str,
    before: str | None,
    after: str,
    *,
    actor_id: int | None,
    ip: str | None,
) -> None:
    if actor_id is None:
        logger.info("import_job %s %s: %s -> %s (system)", job_id, action, before, after)
        return
    await conn.execute(
        """
        INSERT INTO audit_logs (actor_id, action, entity_type, entity_id, before, after, ip)
        VALUES ($1, $2, 'import_job', $3, $4::jsonb, $5::jsonb, $6)
        """,
        actor_id, f"import_job_{action}", job_id,
        None if before is None else json.dumps({"status": before}),
        json.dumps({"status": after}), ip,
    )


if __name__ == "__main__":
    # Smoke check of the pure parts; DB behaviour is covered by test_import_jobs.py (T3.2).
    check_transition("pending", "processing")
    check_transition("failed", "max_retries_exhausted")
    check_transition("max_retries_exhausted", "processing")
    for bad in (("completed", "processing"), ("superseded", "pending"), ("pending", "completed")):
        try:
            check_transition(*bad)
        except IllegalTransitionError:
            pass
        else:
            raise AssertionError(f"{bad} should be illegal")
    assert len(truncate_error("x" * 5000)) == MAX_ERROR_MESSAGE
    assert truncate_error(None) is None
    print("import_jobs.py demo OK")
