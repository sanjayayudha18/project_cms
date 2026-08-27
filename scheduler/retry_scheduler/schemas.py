"""Pydantic request/response schemas for the EOD Retry Scheduler API."""
from __future__ import annotations

from datetime import date, datetime, time
from enum import Enum
from typing import Any
from uuid import UUID

from pydantic import BaseModel


class FileType(str, Enum):
    DMAA = "dmaa"
    ITM_CASHPOS = "itm_cashpos"
    ITM_REPLENISH = "itm_replenish"


class ProcessingStatus(str, Enum):
    PENDING = "pending"
    PROCESSING = "processing"
    COMPLETED = "completed"
    FAILED = "failed"
    MAX_RETRIES_EXHAUSTED = "max_retries_exhausted"


class TriggerType(str, Enum):
    AUTO = "auto"
    MANUAL = "manual"


class APIResponse(BaseModel):
    status: str  # "success" or "error"
    data: Any | None = None
    error: str | None = None


class FileStatusItem(BaseModel):
    file_id: UUID
    filename: str
    checksum: str
    processing_status: ProcessingStatus
    retry_count: int
    max_retries_exhausted: bool
    detected_at: datetime
    last_retry_at: datetime | None
    failure_reason: str | None


class RetryAttempt(BaseModel):
    attempt_number: int
    trigger: TriggerType
    started_at: datetime
    completed_at: datetime | None
    outcome: str | None
    duration_ms: int | None
    error_detail: str | None


class ManualRetryResponse(BaseModel):
    job_id: UUID
    file_id: UUID
    processing_status: ProcessingStatus
    triggered_by: str


class LateDetectionItem(BaseModel):
    id: UUID
    file_type: FileType
    processing_date: date
    sla_deadline: time
    detected_at: datetime
    resolved_at: datetime | None
    is_resolved: bool


class SummaryCount(BaseModel):
    pending: int = 0
    processing: int = 0
    completed: int = 0
    failed: int = 0
    max_retries_exhausted: int = 0
    late: int = 0


class AuditLogItem(BaseModel):
    id: UUID
    event_type: str
    trigger: TriggerType
    file_id: UUID | None
    file_type: FileType
    file_checksum: str | None
    processing_date: date
    initiated_by: str
    outcome: str | None
    duration_ms: int | None
    error_detail: str | None
    created_at: datetime
