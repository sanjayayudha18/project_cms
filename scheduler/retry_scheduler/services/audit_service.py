"""Append-only audit log writer (task 10.1).

No update/delete methods are exposed on purpose -- audit records must
never be mutated after being written (Requirement 7.5).
"""
from __future__ import annotations

from datetime import date
from uuid import UUID

import asyncpg


class AuditService:
    """Writes retry_audit_logs entries. Append-only: no update/delete methods."""

    def __init__(self, pool: asyncpg.Pool):
        self.pool = pool

    async def log_retry_initiated(
        self, trigger: str, file_id: UUID | None, file_type: str, file_checksum: str | None,
        processing_date: date, initiated_by: str,
    ) -> UUID:
        async with self.pool.acquire() as conn:
            row = await conn.fetchrow(
                """
                INSERT INTO retry_audit_logs
                    (event_type, trigger_type, file_id, file_type, file_checksum,
                     processing_date, initiated_by)
                VALUES ('retry_initiated', $1, $2, $3, $4, $5, $6)
                RETURNING id
                """,
                trigger, file_id, file_type, file_checksum, processing_date, initiated_by,
            )
        return row["id"]

    async def log_retry_completed(
        self, trigger: str, file_id: UUID | None, file_type: str, file_checksum: str | None,
        processing_date: date, initiated_by: str, outcome: str,
        duration_ms: int, error_detail: str | None,
    ) -> UUID:
        async with self.pool.acquire() as conn:
            row = await conn.fetchrow(
                """
                INSERT INTO retry_audit_logs
                    (event_type, trigger_type, file_id, file_type, file_checksum,
                     processing_date, initiated_by, outcome, duration_ms, error_detail)
                VALUES ('retry_completed', $1, $2, $3, $4, $5, $6, $7, $8, $9)
                RETURNING id
                """,
                trigger, file_id, file_type, file_checksum, processing_date, initiated_by,
                outcome, duration_ms, error_detail,
            )
        return row["id"]


if __name__ == "__main__":
    # Smoke check: confirm no update/delete methods are exposed (append-only invariant).
    public_methods = {m for m in dir(AuditService) if not m.startswith("_")}
    assert public_methods == {"log_retry_initiated", "log_retry_completed"}
    for forbidden in ("update", "delete", "remove"):
        assert not any(forbidden in m.lower() for m in public_methods)
    print("audit_service.py demo OK")
