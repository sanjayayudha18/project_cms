"""Uvicorn entrypoint (task 17.1). Run: python -m retry_scheduler.run"""
from __future__ import annotations

import uvicorn

from .config import Settings


def main() -> None:
    settings = Settings()
    uvicorn.run(
        "retry_scheduler.main:app",
        host="0.0.0.0",
        port=8082,
        log_level=settings.log_level.lower(),
    )


if __name__ == "__main__":
    main()
