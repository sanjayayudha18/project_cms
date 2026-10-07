"""GET /late (task 14.4)."""
from __future__ import annotations

from datetime import date

from fastapi import APIRouter, Depends, Request

from lib.dependencies import require_eod_role

from ..config import FILE_TYPES

router = APIRouter(dependencies=[Depends(require_eod_role)])


@router.get("/late")
async def get_late(request: Request, processing_date: date):
    pool = request.app.state.db_pool
    async with pool.acquire() as conn:
        rows = await conn.fetch(
            "SELECT * FROM late_detections WHERE processing_date = $1 AND file_type = ANY($2::text[])",
            processing_date, list(FILE_TYPES),
        )

    data = [
        {
            "id": row["id"],
            "file_type": row["file_type"],
            "processing_date": row["processing_date"],
            "sla_deadline": row["sla_deadline"],
            "detected_at": row["detected_at"],
            "resolved_at": row["resolved_at"],
            "is_resolved": row["is_resolved"],
        }
        for row in rows
    ]
    return {"status": "success", "data": data}
