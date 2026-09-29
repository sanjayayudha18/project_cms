"""Integration test for FileDetector.persist_detected_files -> import_jobs
(.claude/sdlc/import-export-jobs spec A11 / FR12). Skipped when DATABASE_URL is empty.

Run from backend_python/:  python -m unittest lib.services.test_detector -v
"""
from __future__ import annotations

import os
import tempfile
import unittest
import uuid
from datetime import date
from pathlib import Path

import asyncpg

from lib.services.detector import FileDetector

DATABASE_URL = os.environ.get("DATABASE_URL", "")


class _Settings:
    def __init__(self, not_processed: Path):
        self._dir = not_processed

    def not_processed_dir(self, file_type: str) -> Path:
        return self._dir


@unittest.skipUnless(DATABASE_URL, "DATABASE_URL not set")
class PersistDetectedFilesTest(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self) -> None:
        self.pool = await asyncpg.create_pool(DATABASE_URL, min_size=1, max_size=2)
        self.file_type = f"test_det_{uuid.uuid4().hex[:12]}"
        self.tmp = tempfile.TemporaryDirectory()
        (Path(self.tmp.name) / "Order_All_bad.xlsx").write_bytes(uuid.uuid4().bytes)
        self.detector = FileDetector(_Settings(Path(self.tmp.name)))

    async def asyncTearDown(self) -> None:
        await self.pool.execute("DELETE FROM import_jobs WHERE source = $1", self.file_type)
        await self.pool.close()
        self.tmp.cleanup()

    async def test_scan_twice_one_row_and_existing_row_untouched(self):
        files = self.detector.scan_not_processed(self.file_type)
        day = date(2099, 1, 1)

        self.assertEqual(await self.detector.persist_detected_files(self.pool, files, day), 1)
        self.assertEqual(await self.detector.persist_detected_files(self.pool, files, day), 0)

        row = await self.pool.fetchrow("SELECT * FROM import_jobs WHERE source = $1", self.file_type)
        self.assertEqual(
            (row["status"], row["detection_source"], row["original_filename"], row["runtime"]),
            ("failed", "not_processed", "Order_All_bad.xlsx", "python"),
        )
        self.assertTrue(row["error_message"].startswith("File found in not_processed"))

        await self.pool.execute(
            "UPDATE import_jobs SET status = 'max_retries_exhausted', auto_retry_count = 3 WHERE id = $1",
            row["id"],
        )
        # next day's scan finds the same file again: still one row, never revived (P6)
        self.assertEqual(await self.detector.persist_detected_files(self.pool, files, date(2099, 1, 2)), 0)
        rows = await self.pool.fetch(
            "SELECT status, auto_retry_count FROM import_jobs WHERE source = $1", self.file_type,
        )
        self.assertEqual([(r["status"], r["auto_retry_count"]) for r in rows], [("max_retries_exhausted", 3)])


if __name__ == "__main__":
    unittest.main()
