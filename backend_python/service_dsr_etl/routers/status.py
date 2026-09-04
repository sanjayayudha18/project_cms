"""GET /status, GET /status/{file_id}/history (task 14.2)."""
from __future__ import annotations

from datetime import date
from uuid import UUID

from fastapi import APIRouter, Depends, Request

from lib.dependencies import require_auth

from ..config import FILE_TYPES

router = APIRouter(dependencies=[Depends(require_auth)])


@router.get("/status")
async def get_status(request: Request, processing_date: date):
    pool = request.app.state.db_pool
    async with pool.acquire() as conn:
        rows = await conn.fetch(
            "SELECT * FROM retry_file_tracking WHERE processing_date = $1", processing_date,
        )

    by_file_type: dict[str, list[dict]] = {ft: [] for ft in FILE_TYPES}
    for row in rows:
        by_file_type.setdefault(row["file_type"], []).append({
            "file_id": row["id"],
            "filename": row["filename"],
            "checksum": row["file_checksum"],
            "processing_status": row["processing_status"],
            "retry_count": row["auto_retry_count"],
            "max_retries_exhausted": row["processing_status"] == "max_retries_exhausted",
            "detected_at": row["detected_at"],
            "last_retry_at": row["last_retry_at"],
            "failure_reason": row["failure_reason"],
        })

    return {
        "status": "success",
        "data": {"processing_date": processing_date, "by_file_type": by_file_type},
    }


@router.get("/status/{file_id}/history")
async def get_status_history(request: Request, file_id: UUID):
    pool = request.app.state.db_pool
    async with pool.acquire() as conn:
        file_row = await conn.fetchrow(
            "SELECT * FROM retry_file_tracking WHERE id = $1", file_id,
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
            "data": {"file_id": file_id, "filename": None, "file_type": None, "attempts": []},
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
            "file_id": file_id,
            "filename": file_row["filename"],
            "file_type": file_row["file_type"],
            "attempts": attempts,
        },
    }
