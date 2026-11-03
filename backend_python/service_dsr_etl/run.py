"""Uvicorn entrypoint. Run: python -m service_dsr_etl.run"""
from __future__ import annotations

import uvicorn

from lib.utils.proxy import uvicorn_proxy_kwargs

from .config import Settings


def main() -> None:
    settings = Settings()
    uvicorn.run(
        "service_dsr_etl.main:app",
        host="0.0.0.0",
        port=8090,
        log_level=settings.log_level.lower(),
        **uvicorn_proxy_kwargs(settings.trusted_proxies),
    )


if __name__ == "__main__":
    main()
