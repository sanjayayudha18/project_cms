"""FastAPI app factory + lifespan for the DSR ETL service (vendor upload
dry-run/confirm flow -- see routers/process.py)."""
from __future__ import annotations

import logging
from contextlib import asynccontextmanager
from datetime import datetime

from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from starlette.exceptions import HTTPException as StarletteHTTPException

from lib.database import create_db_pool
from lib.services.scheduler_service import SchedulerService
from lib.utils.timezone import WIB

from .config import FILE_TYPES, Settings
from .routers import audit, health, late, process, retry, status, summary

logging.basicConfig(level=logging.INFO)


@asynccontextmanager
async def lifespan(app: FastAPI):
    settings: Settings = app.state.settings
    app.state.start_time = datetime.now(WIB)

    pool = await create_db_pool(settings)
    app.state.db_pool = pool

    scheduler_service = SchedulerService(settings, pool, FILE_TYPES)
    scheduler_service.start()
    app.state.scheduler_service = scheduler_service

    try:
        yield
    finally:
        scheduler_service.stop()
        await pool.close()


def create_app(settings: Settings | None = None) -> FastAPI:
    app = FastAPI(title="service-dsr-etl", lifespan=lifespan)
    app.state.settings = settings or Settings()

    app.include_router(health.router)
    app.include_router(status.router)
    app.include_router(retry.router)
    app.include_router(process.router)
    app.include_router(late.router)
    app.include_router(summary.router)
    app.include_router(audit.router)

    @app.exception_handler(StarletteHTTPException)
    async def http_exception_handler(request: Request, exc: StarletteHTTPException):
        return JSONResponse(
            status_code=exc.status_code,
            content={"status": "error", "data": None, "error": exc.detail},
        )

    @app.exception_handler(RequestValidationError)
    async def validation_exception_handler(request: Request, exc: RequestValidationError):
        return JSONResponse(
            status_code=422,
            content={"status": "error", "data": None, "error": str(exc)},
        )

    return app


app = create_app()
