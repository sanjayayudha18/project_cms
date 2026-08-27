"""ETL subprocess invocation for retry attempts (task 9.1)."""
from __future__ import annotations

import asyncio
import time as time_module
from pathlib import Path

from ..config import Settings


class RetryResult:
    def __init__(
        self, success: bool, duration_ms: int, stdout: str, stderr: str, return_code: int,
    ):
        self.success = success
        self.duration_ms = duration_ms
        self.stdout = stdout
        self.stderr = stderr
        self.return_code = return_code
        self.error_detail = stderr if not success else None


class RetryExecutor:
    """Invokes ETL scripts via subprocess for retry attempts."""

    def __init__(self, settings: Settings):
        self.settings = settings

    def get_script_path(self, file_type: str) -> Path:
        """Return the ETL script path for a given file type."""
        return self.settings.etl_script(file_type)

    async def execute_retry(self, file_type: str) -> RetryResult:
        """Run the ETL script for the given file type and capture the result."""
        script_path = self.get_script_path(file_type)
        start = time_module.monotonic()

        proc = await asyncio.create_subprocess_exec(
            "python", str(script_path),
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
            cwd=str(script_path.parent) if script_path.parent != Path("") else None,
        )
        stdout_bytes, stderr_bytes = await proc.communicate()

        duration_ms = int((time_module.monotonic() - start) * 1000)
        stdout = stdout_bytes.decode("utf-8", errors="replace")
        stderr = stderr_bytes.decode("utf-8", errors="replace")

        return RetryResult(
            success=(proc.returncode == 0),
            duration_ms=duration_ms,
            stdout=stdout,
            stderr=stderr,
            return_code=proc.returncode or 0,
        )


if __name__ == "__main__":
    # Smoke check: script routing (Property 6) without actually running a real ETL job.
    settings = Settings(auth_secret="x")
    executor = RetryExecutor(settings)
    assert executor.get_script_path("dmaa") == settings.etl_dmaa_script
    assert executor.get_script_path("itm_cashpos") == settings.etl_itm_cashpos_script
    assert executor.get_script_path("itm_replenish") == settings.etl_itm_replenish_script

    async def _demo() -> None:
        # Exercise execute_retry end-to-end against a throwaway script that exits 0.
        import tempfile

        with tempfile.NamedTemporaryFile(
            mode="w", suffix=".py", delete=False,
        ) as f:
            f.write("import sys; sys.exit(0)\n")
            script = Path(f.name)
        try:
            settings.etl_dmaa_script = script
            result = await executor.execute_retry("dmaa")
            assert isinstance(result, RetryResult)
            assert result.success is True
            assert result.duration_ms >= 0
        finally:
            script.unlink()

    asyncio.run(_demo())
    print("retry_executor.py demo OK")
