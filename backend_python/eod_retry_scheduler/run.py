"""Uvicorn entrypoint. Run: python -m eod_retry_scheduler.run"""
from __future__ import annotations

import uvicorn

from .config import Settings


def main() -> None:
    settings = Settings()
    uvicorn.run(
        "eod_retry_scheduler.main:app",
        host="0.0.0.0",
        port=8091,
        log_level=settings.log_level.lower(),
    )


if __name__ == "__main__":
    main()
