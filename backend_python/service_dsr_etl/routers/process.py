"""POST /process/{file_type} -- generic manual ETL trigger (drains the whole
FTP_DATA/DSR input folder, DSR's own batch fallback), plus two DSR-specific
single-file endpoints for the vendor upload dry-run/confirm flow.

/process/{file_type} is DSR's batch safety net -- it drains a whole folder, no
target file. The two /process/dsr/* endpoints below instead target ONE named
file, because the vendor upload flow is: upload -> dry-run parse (preview, no
DB write) -> vendor confirms -> commit (re-parse the same file, write to DB).
See scheduler/dsr/dsr_etl.py's --mode dry_run/--mode commit CLI surface and
its module docstring for the full contract.
"""
from __future__ import annotations

import json

from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel

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


class DsrFileRequest(BaseModel):
    filename: str


def _last_json_line(stdout: str) -> dict | None:
    """dsr_etl.py prints exactly one JSON line as its final stdout output in
    --mode dry_run/--mode commit; logging.info() lines (if any leak through) go
    to stderr per setup_logging()'s handler, but split on the last line anyway
    as a defensive measure."""
    lines = [line for line in stdout.strip().splitlines() if line.strip()]
    if not lines:
        return None
    try:
        return json.loads(lines[-1])
    except ValueError:
        return None


@router.post("/process/dsr/dry-run")
async def dsr_dry_run(payload: DsrFileRequest, request: Request):
    scheduler_service = request.app.state.scheduler_service
    result = await scheduler_service.run_manual_process(
        "dsr", ["--mode", "dry_run", "--file", payload.filename],
    )
    if not result.success:
        return {
            "status": "error", "data": None,
            "error": result.stderr[-_STDOUT_TAIL_CHARS:] or "dry run failed",
        }

    parsed = _last_json_line(result.stdout)
    if parsed is None:
        return {"status": "error", "data": None, "error": "dry run produced no parseable output"}
    return {"status": "success", "data": parsed, "error": None}


@router.post("/process/dsr/commit")
async def dsr_commit(payload: DsrFileRequest, request: Request):
    scheduler_service = request.app.state.scheduler_service
    result = await scheduler_service.run_manual_process(
        "dsr", ["--mode", "commit", "--file", payload.filename],
    )
    if not result.success:
        return {
            "status": "error", "data": None,
            "error": result.stderr[-_STDOUT_TAIL_CHARS:] or "commit failed",
        }

    parsed = _last_json_line(result.stdout)
    return {"status": "success", "data": parsed, "error": None}
