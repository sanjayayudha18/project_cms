"""GET /health -- no auth required (task 14.1)."""
from __future__ import annotations

from datetime import datetime

from fastapi import APIRouter, Request, Response

from ..config import FILE_TYPES
from ..database import check_db_health
from ..utils.timezone import WIB

router = APIRouter()


@router.get("/health")
async def health(request: Request, response: Response):
    settings = request.app.state.settings
    pool = request.app.state.db_pool

    db_ok = await check_db_health(pool)
    if not db_ok:
        response.status_code = 503
        return {
            "status": "error", "data": None,
            "error": "Database connectivity failure: connection refused",
        }

    fs_ok = True
    for file_type in FILE_TYPES:
        input_dir = settings.input_dir(file_type)
        if not input_dir.parent.exists():
            fs_ok = False
            break
    if not fs_ok:
        response.status_code = 503
        return {
            "status": "error", "data": None,
            "error": "Filesystem access failure: configured directory is not accessible",
        }

    uptime_seconds = int((datetime.now(WIB) - request.app.state.start_time).total_seconds())
    scheduler_service = getattr(request.app.state, "scheduler_service", None)
    last_scan = scheduler_service.last_successful_scan_at if scheduler_service else None

    return {
        "status": "success",
        "data": {
            "service": "eod-retry-scheduler",
            "version": settings.service_version,
            "uptime_seconds": uptime_seconds,
            "last_successful_scan_at": last_scan,
            "database": "connected",
            "filesystem": "accessible",
        },
    }
