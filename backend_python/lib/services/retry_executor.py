"""ETL subprocess invocation for retry attempts (task 9.1). Generic over
whichever settings object is passed in -- only needs settings.etl_script(file_type)."""
from __future__ import annotations

import asyncio
import time as time_module
from pathlib import Path
from typing import Any


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

    def __init__(self, settings: Any):
        self.settings = settings

    def get_script_path(self, file_type: str) -> Path:
        """Return the ETL script path for a given file type."""
        return self.settings.etl_script(file_type)

    async def execute_retry(self, file_type: str, extra_args: list[str] | None = None) -> RetryResult:
        """Run the ETL script for the given file type and capture the result.

        extra_args is only used by the DSR single-file dry-run/commit flow
        (service_dsr_etl's routers/process.py) -- every other caller (auto-retry,
        manual retry, the generic /process/{file_type} trigger) omits it,
        preserving the original no-argument, drain-whole-folder invocation.
        """
        # .resolve() BEFORE building cwd: a relative script_path combined with
        # cwd=script_path.parent makes the *same* relative argument get
        # re-resolved against that new cwd by the python interpreter, producing
        # a nonexistent double-nested path. Resolving first makes both the arg
        # and cwd absolute, so no double-join can happen regardless of this
        # service's own working directory.
        script_path = self.get_script_path(file_type).resolve()
        start = time_module.monotonic()

        proc = await asyncio.create_subprocess_exec(
            "python", str(script_path), *(extra_args or []),
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
            cwd=str(script_path.parent),
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
    # Smoke check: script routing + execute_retry end-to-end, using a tiny
    # duck-typed settings stand-in (no concrete service's Settings needed).
    import tempfile

    class _FakeSettings:
        def __init__(self, script: Path):
            self.script = script

        def etl_script(self, file_type: str) -> Path:
            return self.script

    async def _demo() -> None:
        with tempfile.NamedTemporaryFile(
            mode="w", suffix=".py", delete=False,
        ) as f:
            f.write("import sys; sys.exit(0)\n")
            script = Path(f.name)
        try:
            settings = _FakeSettings(script)
            executor = RetryExecutor(settings)
            assert executor.get_script_path("dmaa") == script
            result = await executor.execute_retry("dmaa")
            assert isinstance(result, RetryResult)
            assert result.success is True
            assert result.duration_ms >= 0
        finally:
            script.unlink()

    asyncio.run(_demo())
    print("retry_executor.py demo OK")
