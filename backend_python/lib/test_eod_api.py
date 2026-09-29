"""Router tests for both EOD APIs on import_jobs (.claude/sdlc/import-export-jobs spec A15,
FR16, plus R5: Go-issued JWT identity reaches audit_logs).

httpx (FastAPI's TestClient) is not installed, so requests go straight into the ASGI app
with a tiny stdlib driver. The lifespan is not run (it would start APScheduler); the pool
and a SchedulerService with a fake ETL executor are put on app.state instead.

Rows use the services' real sources with dates in 2099 and are deleted by id afterwards.
Skipped when DATABASE_URL is empty.

Run from backend_python/:  python -m unittest lib.test_eod_api -v
"""
from __future__ import annotations

import json
import os
import time
import unittest
from datetime import date

import asyncpg
from jose import jwt

from eod_retry_scheduler import config as retry_config
from eod_retry_scheduler.main import create_app as create_retry_app
from lib import import_jobs as ij
from lib.services.retry_executor import RetryResult
from lib.services.scheduler_service import SchedulerService
from service_dsr_etl import config as dsr_config
from service_dsr_etl.main import create_app as create_dsr_app

DATABASE_URL = os.environ.get("DATABASE_URL", "")
DAY = date(2099, 1, 1)
SECRET = "test-secret"
CLIENT_IP = "10.9.8.7"


def access_token(role: str | None = "ADMIN", uid: int = 1, exp: bool = True, secret: str = SECRET) -> str:
    """Shape of pkg/auth AccessTokenClaims (numeric `id`, no `sub`); role=None mimics a refresh token."""
    claims: dict = {"id": uid, "username": "t"}
    if role is not None:
        claims["role"] = role
    if exp:
        claims["exp"] = int(time.time()) + 300
    return jwt.encode(claims, secret, algorithm="HS256")

# (name, app factory, config module, a source owned by that service, hash char)
SERVICES = (
    ("eod_retry_scheduler", create_retry_app, retry_config, "dmaa", "a"),
    ("service_dsr_etl", create_dsr_app, dsr_config, "dsr", "b"),
)


class _AlwaysOkExecutor:
    async def execute_retry(self, file_type: str, extra_args=None, timeout=None) -> RetryResult:
        return RetryResult(success=True, duration_ms=1, stdout="", stderr="", return_code=0)


async def call(app, method: str, path: str, query: str = "", token: str = SECRET) -> tuple[int, dict]:
    """Minimal ASGI request -> (status, json body)."""
    scope = {
        "type": "http", "asgi": {"version": "3.0"}, "http_version": "1.1",
        "method": method, "scheme": "http", "path": path, "raw_path": path.encode(),
        "root_path": "", "query_string": query.encode(),
        "headers": [(b"authorization", f"Bearer {token}".encode())],
        "client": (CLIENT_IP, 5555), "server": ("test", 80),
    }
    sent: list[dict] = []

    async def receive():
        return {"type": "http.request", "body": b"", "more_body": False}

    async def send(message):
        sent.append(message)

    await app(scope, receive, send)
    status = next(m["status"] for m in sent if m["type"] == "http.response.start")
    body = b"".join(m.get("body", b"") for m in sent if m["type"] == "http.response.body")
    return status, json.loads(body)


@unittest.skipUnless(DATABASE_URL, "DATABASE_URL not set")
class EodApiTest(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self) -> None:
        self.pool = await asyncpg.create_pool(DATABASE_URL, min_size=1, max_size=3)
        self.job_ids: list[int] = []

    async def asyncTearDown(self) -> None:
        await self.pool.execute(
            "DELETE FROM audit_logs WHERE entity_type = 'import_job' AND entity_id = ANY($1::bigint[])", self.job_ids,
        )
        await self.pool.execute("DELETE FROM retry_audit_logs WHERE file_id = ANY($1::bigint[])", self.job_ids)
        # newer versions reference older ones (supersedes_job_id): delete newest first
        for job_id in sorted(self.job_ids, reverse=True):
            await self.pool.execute("DELETE FROM import_jobs WHERE id = $1", job_id)
        await self.pool.close()

    def app(self, create_app, config, auth_mode: str = "api_key"):
        settings = config.Settings(_env_file=None, database_url=DATABASE_URL,
                                   auth_secret=SECRET, auth_mode=auth_mode)
        app = create_app(settings)
        app.state.db_pool = self.pool
        app.state.scheduler_service = SchedulerService(
            settings, self.pool, config.FILE_TYPES, retry_executor=_AlwaysOkExecutor(),
        )
        return app

    async def detected(self, source: str, hash_char: str) -> int:
        async with self.pool.acquire() as conn:
            job = await ij.detect(conn, source=source, processing_date=DAY, file_hash=hash_char * 64,
                                  original_filename=f"{hash_char}.xlsx", file_path=None,
                                  detection_source="not_processed", error_message="boom")
        self.job_ids.append(job["id"])
        return job["id"]

    async def superseded_pair(self, source: str) -> None:
        """v1 superseded by v2, both through the helper."""
        async with self.pool.acquire() as conn:
            for c in "de":
                job, _ = await ij.register(conn, source=source, processing_date=DAY,
                                           file_hash=c * 64, original_filename=f"{c}.xlsx")
                self.job_ids.append(job["id"])
                await ij.start(conn, job["id"])
                await ij.complete(conn, job["id"])

    async def test_status_serializes_file_id_as_string_and_keeps_field_names(self):
        for name, create_app, config, source, c in SERVICES:
            with self.subTest(service=name):
                job_id = await self.detected(source, c)
                status, body = await call(self.app(create_app, config), "GET", "/status",
                                          f"processing_date={DAY}")
                self.assertEqual(status, 200)
                self.assertEqual(set(body["data"]["by_file_type"]), set(config.FILE_TYPES))
                item = next(i for i in body["data"]["by_file_type"][source] if i["file_id"] == str(job_id))
                self.assertEqual(set(item), {
                    "file_id", "filename", "checksum", "processing_status", "retry_count",
                    "max_retries_exhausted", "detected_at", "last_retry_at", "failure_reason",
                })
                self.assertEqual(
                    (item["processing_status"], item["filename"], item["checksum"], item["failure_reason"]),
                    ("failed", f"{c}.xlsx", c * 64, "boom"),
                )

    async def test_status_only_lists_the_services_own_sources(self):
        dsr_job = await self.detected("dsr", "c")
        _, body = await call(self.app(create_retry_app, retry_config), "GET", "/status", f"processing_date={DAY}")
        listed = {i["file_id"] for items in body["data"]["by_file_type"].values() for i in items}
        self.assertNotIn(str(dsr_job), listed)

    async def test_history_and_retry_take_numeric_ids_only(self):
        for name, create_app, config, _, _ in SERVICES:
            app = self.app(create_app, config)
            for bad in ("abc", "3f2b8c1e-8a55-4b8e-9d7e-2d8f1c2a9b10"):
                with self.subTest(service=name, id=bad):
                    self.assertEqual((await call(app, "GET", f"/status/{bad}/history"))[0], 422)
                    self.assertEqual((await call(app, "POST", f"/retry/{bad}"))[0], 422)

    async def test_manual_retry_then_history_then_conflict(self):
        for name, create_app, config, source, c in SERVICES:
            with self.subTest(service=name):
                app = self.app(create_app, config)
                job_id = await self.detected(source, c)

                status, body = await call(app, "POST", f"/retry/{job_id}")
                self.assertEqual(status, 200)
                self.assertEqual(body["data"]["file_id"], str(job_id))
                self.assertEqual(body["data"]["processing_status"], "completed")

                status, body = await call(app, "GET", f"/status/{job_id}/history")
                self.assertEqual(status, 200)
                self.assertEqual((body["data"]["file_id"], body["data"]["file_type"]), (str(job_id), source))
                self.assertEqual([a["outcome"] for a in body["data"]["attempts"]], ["completed"])

                self.assertEqual((await call(app, "POST", f"/retry/{job_id}"))[0], 409)
                self.assertEqual((await call(app, "POST", "/retry/999999999999"))[0], 404)

                status, body = await call(app, "GET", "/audit", f"processing_date={DAY}")
                self.assertEqual(status, 200)
                mine = [e for e in body["data"] if e["file_id"] == str(job_id)]
                self.assertEqual({e["event_type"] for e in mine}, {"retry_initiated", "retry_completed"})

    async def test_summary_counts_superseded_separately_from_failed(self):
        for name, create_app, config, source, _ in SERVICES:
            with self.subTest(service=name):
                await self.superseded_pair(source)
                status, body = await call(self.app(create_app, config), "GET", "/summary", f"processing_date={DAY}")
                self.assertEqual(status, 200)
                per_source = body["data"]["by_file_type"][source]
                self.assertEqual((per_source["superseded"], per_source["completed"], per_source["failed"]), (1, 1, 0))
                self.assertEqual(body["data"]["counts"]["superseded"], 1)

    async def test_go_issued_jwt_identity_reaches_audit_logs(self):
        uid = await self.pool.fetchval("SELECT id FROM users ORDER BY id LIMIT 1")
        if uid is None:
            self.skipTest("no users row")
        app = self.app(create_retry_app, retry_config, auth_mode="jwt")
        job_id = await self.detected("dmaa", "9")
        token = access_token("ADMIN", uid)
        status, body = await call(app, "POST", f"/retry/{job_id}", token=token)
        self.assertEqual(status, 200)
        self.assertEqual(body["data"]["triggered_by"], str(uid))
        rows = await self.pool.fetch(
            "SELECT actor_id, action, ip FROM audit_logs WHERE entity_type = 'import_job' AND entity_id = $1 ORDER BY id",
            job_id,
        )
        self.assertEqual([r["action"] for r in rows], ["import_job_retry", "import_job_complete"])
        self.assertEqual({(r["actor_id"], r["ip"]) for r in rows}, {(uid, CLIENT_IP)})

    # review: RBAC on monitoring + retry (Sec 5), JWT hardening
    async def test_jwt_role_gate_and_token_hardening(self):
        for name, create_app, config, source, c in SERVICES:
            with self.subTest(service=name):
                app = self.app(create_app, config, auth_mode="jwt")
                job_id = await self.detected(source, c)
                q = f"processing_date={DAY}"
                for path, query, method in (("/status", q, "GET"), ("/summary", q, "GET"), ("/late", q, "GET"),
                                            ("/audit", q, "GET"), (f"/retry/{job_id}", "", "POST")):
                    self.assertEqual((await call(app, method, path, query, token=access_token("VENDOR")))[0], 403, path)
                    self.assertEqual((await call(app, method, path, query, token=access_token(None)))[0], 401, path)
                    self.assertEqual((await call(app, method, path, query, token=access_token(exp=False)))[0], 401, path)
                    self.assertEqual((await call(app, method, path, query, token=access_token(secret="wrong")))[0], 401, path)
                self.assertEqual((await call(app, "GET", "/summary", q, token=access_token("ADMIN")))[0], 200)
                self.assertEqual((await call(app, "GET", "/summary", q, token=access_token("APPACCESS")))[0], 200)
                # the VENDOR attempts above must not have touched the job
                self.assertEqual(
                    await self.pool.fetchval("SELECT status FROM import_jobs WHERE id = $1", job_id), "failed")

    async def test_other_services_job_is_unknown_and_id_bounds(self):
        dsr_job = await self.detected("dsr", "c")
        app = self.app(create_retry_app, retry_config)
        self.assertEqual((await call(app, "POST", f"/retry/{dsr_job}"))[0], 404)
        _, body = await call(app, "GET", f"/status/{dsr_job}/history")
        self.assertEqual((body["data"]["filename"], body["data"]["attempts"]), (None, []))
        self.assertEqual(
            await self.pool.fetchval("SELECT status FROM import_jobs WHERE id = $1", dsr_job), "failed")
        for bad in ("0", "-1", "9223372036854775808"):
            self.assertEqual((await call(app, "POST", f"/retry/{bad}"))[0], 422, bad)


if __name__ == "__main__":
    unittest.main()
