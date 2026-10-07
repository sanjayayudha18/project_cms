"""RetryExecutor: hung ETL is killed at the timeout; error detail is bounded (review T8.2).

Run from backend_python/:  python -m unittest lib.services.test_retry_executor -v
"""
from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from lib.services.retry_executor import MAX_ERROR_DETAIL, RetryExecutor, RetryResult


class _Settings:
    def __init__(self, script: Path):
        self.script = script

    def etl_script(self, file_type: str) -> Path:
        return self.script


class RetryExecutorTest(unittest.IsolatedAsyncioTestCase):
    async def run_script(self, code: str, timeout: float | None) -> RetryResult:
        with tempfile.TemporaryDirectory() as tmp:
            script = Path(tmp) / "etl.py"
            script.write_text(code, encoding="utf-8")
            return await RetryExecutor(_Settings(script)).execute_retry("x", timeout=timeout)

    async def test_hung_etl_is_killed_at_timeout(self):
        result = await self.run_script("import time; time.sleep(30)", timeout=0.5)
        self.assertFalse(result.success)
        self.assertEqual(result.return_code, -1)
        self.assertIn("timed out", result.error_detail)

    async def test_fast_etl_within_timeout_succeeds(self):
        self.assertTrue((await self.run_script("pass", timeout=30)).success)

    def test_error_detail_keeps_the_bounded_tail(self):
        result = RetryResult(False, 1, "", "head" + "x" * 5000 + "TAIL", 1)
        self.assertEqual(len(result.error_detail), MAX_ERROR_DETAIL)
        self.assertTrue(result.error_detail.endswith("TAIL"))


if __name__ == "__main__":
    unittest.main()
