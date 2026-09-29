"""GET /status, GET /status/{file_id}/history (task 14.2).

Reads import_jobs (spec FR16). JSON field names are unchanged for
features/eod-monitoring; file_id is the bigint id serialized as a string.
"""
from __future__ import annotations

from datetime import date

from fastapi import APIRouter, Depends, Path, Request

from lib.dependencies import require_eod_role

from ..config import FILE_TYPES

router = APIRouter(dependencies=[Depends(require_eod_role)])


@router.get("/status")
async def get_status(request: Request, processing_date: date):
    pool = request.app.state.db_pool
    async with pool.acquire() as conn:
        rows = await conn.fetch(
            """
            SELECT * FROM import_jobs
            WHERE processing_date = $1 AND source = ANY($2::text[])
            ORDER BY id
            """,
            processing_date, list(FILE_TYPES),
        )

    by_file_type: dict[str, list[dict]] = {ft: [] for ft in FILE_TYPES}
    for row in rows:
        by_file_type[row["source"]].append({
            "file_id": str(row["id"]),
            "filename": row["original_filename"],
            "checksum": row["file_hash"],
            "processing_status": row["status"],
            "retry_count": row["auto_retry_count"],
            "max_retries_exhausted": row["status"] == "max_retries_exhausted",
            "detected_at": row["created_at"],
            "last_retry_at": row["last_retry_at"],
            "failure_reason": row["error_message"],
        })

    return {
        "status": "success",
        "data": {"processing_date": processing_date, "by_file_type": by_file_type},
    }


@router.get("/status/{file_id}/history")
async def get_status_history(request: Request, file_id: int = Path(ge=1, le=9223372036854775807)):
    pool = request.app.state.db_pool
    async with pool.acquire() as conn:
        # only this service's own sources: a job of the other service looks unknown
        file_row = await conn.fetchrow(
            "SELECT * FROM import_jobs WHERE id = $1 AND source = ANY($2::text[])", file_id, list(FILE_TYPES),
        )
        audit_rows = await conn.fetch(
            """
            SELECT * FROM retry_audit_logs WHERE file_id = $1
            ORDER BY created_at ASC
            """,
            file_id,
        )

    if file_row is None:
        return {
            "status": "success",
            "data": {"file_id": str(file_id), "filename": None, "file_type": None, "attempts": []},
        }

    # Pair up retry_initiated -> retry_completed rows into attempts, in order.
    attempts = []
    pending_start = None
    attempt_number = 0
    for row in audit_rows:
        if row["event_type"] == "retry_initiated":
            attempt_number += 1
            pending_start = row
        elif row["event_type"] == "retry_completed":
            attempts.append({
                "attempt_number": attempt_number,
                "trigger": row["trigger_type"],
                "started_at": pending_start["created_at"] if pending_start else row["created_at"],
                "completed_at": row["created_at"],
                "outcome": row["outcome"],
                "duration_ms": row["duration_ms"],
                "error_detail": row["error_detail"],
            })
            pending_start = None

    return {
        "status": "success",
        "data": {
            "file_id": str(file_id),
            "filename": file_row["original_filename"],
            "file_type": file_row["source"],
            "attempts": attempts,
        },
    }
