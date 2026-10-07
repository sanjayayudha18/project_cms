"""Async asyncpg connection pool + health check + DB retry helper.

Shared between eod_retry_scheduler and service_dsr_etl -- takes any settings
object exposing `.database_url: str` (each service's own config.Settings),
no ownership of a specific Settings class here.

No ORM: the service is thin enough that raw asyncpg Records are used
throughout instead of a SQLAlchemy model layer (see task 2.3 note in
tasks.md for rationale).
"""
from __future__ import annotations

import asyncio
import logging
from contextlib import asynccontextmanager
from typing import Any, AsyncIterator, Awaitable, Callable, TypeVar

import asyncpg

logger = logging.getLogger(__name__)

T = TypeVar("T")


async def create_db_pool(settings: Any) -> asyncpg.Pool:
    """Create the asyncpg connection pool."""
    return await asyncpg.create_pool(dsn=settings.database_url, min_size=1, max_size=10)


@asynccontextmanager
async def get_db_pool(settings: Any) -> AsyncIterator[asyncpg.Pool]:
    """Lifespan helper: yields a pool, closes it on exit."""
    pool = await create_db_pool(settings)
    try:
        yield pool
    finally:
        await pool.close()


async def check_db_health(pool: asyncpg.Pool) -> bool:
    """True if a trivial query succeeds against the pool."""
    try:
        async with pool.acquire() as conn:
            await conn.fetchval("SELECT 1")
        return True
    except Exception:
        logger.exception("Database health check failed")
        return False


async def with_db_retry(
    operation: Callable[[], Awaitable[T]],
    max_attempts: int = 3,
    base_delay: float = 1.0,
) -> T:
    """Execute a DB operation with exponential backoff retry (1s, 2s, 4s...)."""
    for attempt in range(1, max_attempts + 1):
        try:
            return await operation()
        except Exception as e:
            if attempt == max_attempts:
                logger.error("DB operation failed after %d attempts: %s", max_attempts, e)
                raise
            delay = base_delay * (2 ** (attempt - 1))
            logger.warning(
                "DB operation failed (attempt %d/%d), retrying in %.1fs: %s",
                attempt, max_attempts, delay, e,
            )
            await asyncio.sleep(delay)
    raise RuntimeError("unreachable")  # pragma: no cover


if __name__ == "__main__":
    # Smoke check: with_db_retry actually retries then raises after exhausting attempts.
    async def _demo() -> None:
        calls = 0

        async def _always_fails():
            nonlocal calls
            calls += 1
            raise ValueError("boom")

        try:
            await with_db_retry(_always_fails, max_attempts=3, base_delay=0.01)
        except ValueError:
            pass
        assert calls == 3, f"expected 3 attempts, got {calls}"

        succeeded = {"n": 0}

        async def _succeeds_on_second():
            succeeded["n"] += 1
            if succeeded["n"] < 2:
                raise ValueError("transient")
            return "ok"

        result = await with_db_retry(_succeeds_on_second, max_attempts=3, base_delay=0.01)
        assert result == "ok"
        print("database.py demo OK")

    asyncio.run(_demo())
