"""GET /summary (task 14.5)."""
from __future__ import annotations

from datetime import date

from fastapi import APIRouter, Depends, Request

from lib.dependencies import require_auth

from ..config import FILE_TYPES

router = APIRouter(dependencies=[Depends(require_auth)])

_STATUSES = ("pending", "processing", "completed", "failed", "max_retries_exhausted")


@router.get("/summary")
async def get_summary(request: Request, processing_date: date):
    pool = request.app.state.db_pool
    async with pool.acquire() as conn:
        rows = await conn.fetch(
            """
            SELECT file_type, processing_status, COUNT(*) AS cnt
            FROM retry_file_tracking
            WHERE processing_date = $1
            GROUP BY file_type, processing_status
            """,
            processing_date,
        )
        late_count = await conn.fetchval(
            """
            SELECT COUNT(*) FROM late_detections
            WHERE processing_date = $1 AND is_resolved = false
            """,
            processing_date,
        )

    counts = {status: 0 for status in _STATUSES}
    counts["late"] = late_count
    by_file_type = {ft: {status: 0 for status in _STATUSES} for ft in FILE_TYPES}

    for row in rows:
        status = row["processing_status"]
        if status in counts:
            counts[status] += row["cnt"]
        if row["file_type"] in by_file_type and status in by_file_type[row["file_type"]]:
            by_file_type[row["file_type"]][status] += row["cnt"]

    return {
        "status": "success",
        "data": {
            "processing_date": processing_date,
            "counts": counts,
            "by_file_type": by_file_type,
        },
    }
