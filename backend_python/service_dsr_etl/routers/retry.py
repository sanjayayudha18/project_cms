"""POST /retry/{file_id} -- manual retry (task 14.3)."""
from __future__ import annotations

from uuid import UUID, uuid4

from fastapi import APIRouter, Depends, Request, Response

from lib.dependencies import extract_user_id, require_auth
from lib.services.scheduler_service import FileNotFoundInTrackingError, RetryConflictError

router = APIRouter(dependencies=[Depends(require_auth)])


@router.post("/retry/{file_id}")
async def manual_retry(
    file_id: UUID, request: Request, response: Response, auth=Depends(require_auth),
):
    user_id = extract_user_id(auth)
    scheduler_service = request.app.state.scheduler_service

    try:
        result = await scheduler_service.process_manual_retry(file_id, user_id)
    except FileNotFoundInTrackingError as e:
        response.status_code = 404
        return {"status": "error", "data": None, "error": str(e)}
    except RetryConflictError as e:
        response.status_code = 409
        return {"status": "error", "data": None, "error": str(e)}

    return {
        "status": "success",
        "data": {
            "job_id": uuid4(),
            "file_id": result["file_id"],
            "processing_status": result["processing_status"],
            "triggered_by": result["triggered_by"],
        },
    }
