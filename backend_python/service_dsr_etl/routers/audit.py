"""GET /audit (task 14.6)."""
from __future__ import annotations

from datetime import date

from fastapi import APIRouter, Depends, Request

from lib.dependencies import require_auth

router = APIRouter(dependencies=[Depends(require_auth)])


@router.get("/audit")
async def get_audit(
    request: Request,
    processing_date: date | None = None,
    file_type: str | None = None,
    trigger: str | None = None,
):
    pool = request.app.state.db_pool

    clauses = []
    params: list = []
    if processing_date is not None:
        params.append(processing_date)
        clauses.append(f"processing_date = ${len(params)}")
    if file_type is not None:
        params.append(file_type)
        clauses.append(f"file_type = ${len(params)}")
    if trigger is not None:
        params.append(trigger)
        clauses.append(f"trigger_type = ${len(params)}")

    where = f"WHERE {' AND '.join(clauses)}" if clauses else ""
    query = f"SELECT * FROM retry_audit_logs {where} ORDER BY created_at DESC"

    async with pool.acquire() as conn:
        rows = await conn.fetch(query, *params)

    data = [
        {
            "id": row["id"],
            "event_type": row["event_type"],
            "trigger": row["trigger_type"],
            "file_id": row["file_id"],
            "file_type": row["file_type"],
            "file_checksum": row["file_checksum"],
            "processing_date": row["processing_date"],
            "initiated_by": row["initiated_by"],
            "outcome": row["outcome"],
            "duration_ms": row["duration_ms"],
            "error_detail": row["error_detail"],
            "created_at": row["created_at"],
        }
        for row in rows
    ]
    return {"status": "success", "data": data}
