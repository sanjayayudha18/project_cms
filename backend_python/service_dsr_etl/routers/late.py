"""GET /late (task 14.4)."""
from __future__ import annotations

from datetime import date

from fastapi import APIRouter, Depends, Request

from lib.dependencies import require_auth

router = APIRouter(dependencies=[Depends(require_auth)])


@router.get("/late")
async def get_late(request: Request, processing_date: date):
    pool = request.app.state.db_pool
    async with pool.acquire() as conn:
        rows = await conn.fetch(
            "SELECT * FROM late_detections WHERE processing_date = $1", processing_date,
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
