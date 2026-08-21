"""
Preservation property tests for itm_cashpos_etl.py's idempotent ingest.

Feature: rename_db_table (itm_cashpos -> itm_replenish)
Property 2: Preservation - Query Results and ETL Behavior Unchanged

Written BEFORE the rename fix (task 2), against the pre-rename table
naming (task 3.4 renamed the SQL string constants, not the column/logic
structure, and not this module's filename), so these tests pass unchanged
both before and after the fix. Uses a lightweight in-memory fake for the
psycopg connection/cursor instead of a real Postgres connection, so it
needs no DATABASE_URL and runs everywhere `python -m unittest` runs.

The fake matches queries structurally (by clause shape, not by literal
table name), so it kept working once itm_cashpos_etl.py's SQL strings
were updated to itm_replenish naming.
"""
from __future__ import annotations

import unittest
from datetime import date

from itm_cashpos_etl import (
    create_file_record,
    file_already_processed,
    update_file_status,
)


class _FakeCursor:
    """Minimal cursor stub that interprets the small set of SQL shapes
    itm_cashpos_etl.py's DB helper functions issue, keyed on clause
    structure rather than the literal (renameable) table name."""

    def __init__(self, conn: "_FakeConn") -> None:
        self._conn = conn
        self._result = None

    def __enter__(self) -> "_FakeCursor":
        return self

    def __exit__(self, *exc_info: object) -> None:
        return None

    def execute(self, sql: str, params: tuple = ()) -> None:
        normalized = " ".join(sql.split())
        self._result = None

        if normalized.startswith("SELECT id, status FROM") and "WHERE checksum" in normalized:
            (checksum,) = params
            row = self._conn.files_by_checksum.get(checksum)
            self._result = (row["id"], row["status"]) if row else None

        elif normalized.startswith("SELECT id FROM") and "WHERE checksum" in normalized:
            (checksum,) = params
            row = self._conn.files_by_checksum.get(checksum)
            self._result = (row["id"],) if row else None

        elif normalized.startswith("UPDATE") and "status = 'processing'" in normalized:
            (file_id,) = params
            self._conn.files_by_id[file_id]["status"] = "processing"

        elif normalized.startswith("DELETE FROM") and "WHERE file_id" in normalized:
            (file_id,) = params
            self._conn.rows = [r for r in self._conn.rows if r["file_id"] != file_id]

        elif normalized.startswith("INSERT INTO") and "RETURNING id" in normalized:
            # status is a literal ('processing') in the INSERT, not a
            # bound param — only filename/file_date/checksum come through.
            filename, file_date, checksum = params
            new_id = self._conn.next_id
            self._conn.next_id += 1
            row = {
                "id": new_id,
                "filename": filename,
                "file_date": file_date,
                "checksum": checksum,
                "status": "processing",
            }
            self._conn.files_by_id[new_id] = row
            self._conn.files_by_checksum[checksum] = row
            self._result = (new_id,)

        elif normalized.startswith("UPDATE") and "status = %s, row_count" in normalized:
            status, row_count, success_count, error_count, error_message, file_id = params
            row = self._conn.files_by_id[file_id]
            row.update(
                status=status,
                row_count=row_count,
                success_count=success_count,
                error_count=error_count,
                error_message=error_message,
            )

        else:
            raise AssertionError(f"_FakeCursor: unhandled query shape: {normalized!r}")

    def fetchone(self) -> tuple | None:
        return self._result


class _FakeConn:
    """In-memory stand-in for the psycopg connection, tracking
    itm_replenish_files rows (by id and by checksum) and itm_replenish rows
    (by file_id), mirroring the two-table shape originally created by
    009_itm_cashpos.sql and renamed by 010_rename_itm_cashpos_to_itm_replenish.sql."""

    def __init__(self) -> None:
        self.files_by_id: dict[int, dict] = {}
        self.files_by_checksum: dict[str, dict] = {}
        self.rows: list[dict] = []
        self.next_id = 1
        self.commit_count = 0

    def cursor(self) -> _FakeCursor:
        return _FakeCursor(self)

    def commit(self) -> None:
        self.commit_count += 1

    def add_replenish_row(self, file_id: int) -> None:
        self.rows.append({"file_id": file_id})


class FileAlreadyProcessedTests(unittest.TestCase):
    def test_new_checksum_is_not_already_processed(self) -> None:
        conn = _FakeConn()
        self.assertFalse(file_already_processed(conn, "brand-new-checksum"))

    def test_completed_checksum_is_already_processed(self) -> None:
        conn = _FakeConn()
        file_id = create_file_record(conn, "ATM_Replenish_01_Jan_2026.csv", date(2026, 1, 1), "checksum-a")
        update_file_status(conn, file_id, "completed", row_count=1, success_count=1, error_count=0)

        self.assertTrue(file_already_processed(conn, "checksum-a"))

    def test_failed_checksum_is_allowed_to_reprocess(self) -> None:
        conn = _FakeConn()
        file_id = create_file_record(conn, "ATM_Replenish_01_Jan_2026.csv", date(2026, 1, 1), "checksum-b")
        update_file_status(conn, file_id, "failed", row_count=1, success_count=0, error_count=1, error_message="boom")

        self.assertFalse(file_already_processed(conn, "checksum-b"))


class IdempotentIngestPropertyTest(unittest.TestCase):
    """Property: for any file checksum already recorded with status
    'completed', re-ingesting the same checksum must be rejected (skipped)
    rather than creating a duplicate file/row set — the core guarantee
    process_file's `if file_already_processed(...): return skip` branch
    depends on."""

    def test_duplicate_checksum_is_idempotently_rejected_after_completion(self) -> None:
        checksums = ["dup-checksum-1", "dup-checksum-2", "dup-checksum-3"]
        for checksum in checksums:
            with self.subTest(checksum=checksum):
                conn = _FakeConn()

                # First ingest: not yet processed, completes successfully.
                self.assertFalse(file_already_processed(conn, checksum))
                file_id = create_file_record(conn, "ATM_Replenish_21_Agu_2026.csv", date(2026, 8, 21), checksum)
                conn.add_replenish_row(file_id)
                update_file_status(conn, file_id, "completed", row_count=1, success_count=1, error_count=0)

                # Second ingest attempt with the identical checksum: must be
                # recognized as already processed (the ETL's skip branch),
                # and must not create a second file record or duplicate rows.
                self.assertTrue(file_already_processed(conn, checksum))
                self.assertEqual(1, len(conn.files_by_id), "duplicate checksum must not create a second file record")
                self.assertEqual(1, len([r for r in conn.rows if r["file_id"] == file_id]), "duplicate checksum must not duplicate replenish rows")

    def test_retry_of_failed_file_resets_and_clears_old_rows(self) -> None:
        """Not idempotent-skip, but the companion retry path: a checksum
        that previously failed gets its file record reset to 'processing'
        and any partial rows from the failed attempt deleted, so retries
        don't accumulate duplicate/partial data."""
        conn = _FakeConn()
        checksum = "retry-checksum"

        file_id = create_file_record(conn, "ATM_Replenish_21_Agu_2026.csv", date(2026, 8, 21), checksum)
        conn.add_replenish_row(file_id)  # partial row from the failed attempt
        update_file_status(conn, file_id, "failed", row_count=1, success_count=0, error_count=1, error_message="parse error")

        self.assertFalse(file_already_processed(conn, checksum))
        retried_file_id = create_file_record(conn, "ATM_Replenish_21_Agu_2026.csv", date(2026, 8, 21), checksum)

        self.assertEqual(file_id, retried_file_id, "retry must reuse the existing file record, not create a new one")
        self.assertEqual("processing", conn.files_by_id[file_id]["status"])
        self.assertEqual([], [r for r in conn.rows if r["file_id"] == file_id], "old partial rows must be cleared before retry")


if __name__ == "__main__":
    unittest.main()
