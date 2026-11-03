"""Manual run of the realisasi classification (replenishment-realisasi FR3.1).

Run from backend_python/:  python -m replenish_classify.run [--date YYYY-MM-DD] [--start-date YYYY-MM-DD]
Uses the eod_retry_scheduler settings (RETRY_DATABASE_URL, RETRY_REPLENISH_CLASSIFY_START_DATE).
Exit 1 when the run failed (details in the log + retry_audit_logs).
"""
from __future__ import annotations

import argparse
import asyncio
import logging
from datetime import date

import asyncpg

from eod_retry_scheduler.config import Settings
from lib.utils.timezone import current_processing_date
from replenish_classify.job import run_once


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--date", type=date.fromisoformat, help="WIB date treated as today (default: today)")
    p.add_argument("--start-date", type=date.fromisoformat, help="override RETRY_REPLENISH_CLASSIFY_START_DATE")
    args = p.parse_args(argv)
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
    settings = Settings()
    start = args.start_date or settings.replenish_classify_start_date
    if start is None:
        logging.info("no start date (RETRY_REPLENISH_CLASSIFY_START_DATE / --start-date): nothing to do")
        return 0

    async def go() -> object:
        pool = await asyncpg.create_pool(dsn=settings.database_url, min_size=1, max_size=2)
        try:
            return await run_once(pool, args.date or current_processing_date(), start, trigger="manual",
                                  initiated_by="cli", email_enabled=settings.notify_email_enabled)
        finally:
            await pool.close()

    return 0 if asyncio.run(go()) is not None else 1


if __name__ == "__main__":
    raise SystemExit(main())
