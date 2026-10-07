"""GET /summary (task 14.5)."""
from __future__ import annotations

from datetime import date

from fastapi import APIRouter, Depends, Request

from lib.dependencies import require_eod_role

from ..config import FILE_TYPES

router = APIRouter(dependencies=[Depends(require_eod_role)])

# superseded is its own bucket: an older version replaced by a newer file, not a failure.
_STATUSES = ("pending", "processing", "completed", "failed", "max_retries_exhausted", "superseded")


@router.get("/summary")
async def get_summary(request: Request, processing_date: date):
    pool = request.app.state.db_read_pool
    async with pool.acquire() as conn:
        rows = await conn.fetch(
            """
            SELECT source, status, COUNT(*) AS cnt
            FROM import_jobs
            WHERE processing_date = $1 AND source = ANY($2::text[])
            GROUP BY source, status
            """,
            processing_date, list(FILE_TYPES),
        )
        late_count = await conn.fetchval(
            """
            SELECT COUNT(*) FROM late_detections
            WHERE processing_date = $1 AND is_resolved = false AND file_type = ANY($2::text[])
            """,
            processing_date, list(FILE_TYPES),
        )

    counts = {status: 0 for status in _STATUSES}
    counts["late"] = late_count
    by_file_type = {ft: {status: 0 for status in _STATUSES} for ft in FILE_TYPES}

    for row in rows:
        counts[row["status"]] += row["cnt"]
        by_file_type[row["source"]][row["status"]] += row["cnt"]

    return {
        "status": "success",
        "data": {
            "processing_date": processing_date,
            "counts": counts,
            "by_file_type": by_file_type,
        },
    }
