"""POST /process/{file_type} -- generic manual ETL trigger for dmaa, itm_cashpos,
itm_replenish. Drains a whole input folder, no target file. (DSR's single-file
dry-run/commit endpoints live in the sibling service_dsr_etl service.)"""
from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException, Request

from lib.dependencies import require_auth

from ..config import FILE_TYPES

router = APIRouter(dependencies=[Depends(require_auth)])

_STDOUT_TAIL_CHARS = 2000


@router.post("/process/{file_type}")
async def trigger_process(file_type: str, request: Request):
    if file_type not in FILE_TYPES:
        raise HTTPException(
            status_code=422,
            detail=f"Unknown file_type '{file_type}'. Expected one of {FILE_TYPES}.",
        )

    scheduler_service = request.app.state.scheduler_service
    result = await scheduler_service.run_manual_process(file_type)

    return {
        "status": "success",
        "data": {
            "file_type": file_type,
            "status": "success" if result.success else "failed",
            "duration_ms": result.duration_ms,
            "stdout_tail": result.stdout[-_STDOUT_TAIL_CHARS:],
            "stderr_tail": result.stderr[-_STDOUT_TAIL_CHARS:] if not result.success else None,
        },
        "error": None,
    }
