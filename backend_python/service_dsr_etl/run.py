"""Uvicorn entrypoint. Run: python -m service_dsr_etl.run"""
from __future__ import annotations

import uvicorn

from .config import Settings


def main() -> None:
    settings = Settings()
    uvicorn.run(
        "service_dsr_etl.main:app",
        host="0.0.0.0",
        port=8090,
        log_level=settings.log_level.lower(),
    )


if __name__ == "__main__":
    main()
