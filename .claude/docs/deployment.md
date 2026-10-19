# GCP Deployment (0 -> Deployed)

> Moved verbatim from `.claude/CLAUDE.md` Sec 10 on 2026-09-29. "Sec N" references point to CLAUDE.md.


| # | Product | Role |
| ---| ---| --- |
| 1 | Compute Engine | Frontend (internal + vendor) |
| 2 | Compute Engine | Backend, currently ONE VM running `backend/` (port 8080) only; `backend-cit/` (8081) is not deployed until it is renamed to `backend-exq` and has content (decision 2026-10-08) — see split-VM plan below |
| 3 | CloudSQL Postgres | Primary (write) |
| 4 | CloudSQL Postgres | Read replica (reporting/dashboard) |
| 5 | Cloud Storage | Uploads/exports/escrow batch files |
| 6 | Memorystore Redis | Cache/sessions |
| 7 | Artifact Registry | Docker images |
| 8 | Cloud Build | CD: build images -> push Artifact Registry (git SHA tag) -> deploy. CI (lint/test/scan/E2E) runs in GitHub Actions `.github/workflows/ci.yml` |
| 9 | Secret Manager | Secrets (JWT, LDAP, SMTP, DB) |
| 10 | Cloud DNS | Domains (internal + vendor) |
| 11 | Cloud Armor | WAF (esp. vendor portal) |
| 12 | Cloud Logging | Structured logs |
| 13 | Cloud Monitoring | Metrics/alerts (incl. replica lag) |

**Current state (now): ONE Compute Engine VM runs `backend` only** (Docker container on 8080) — see `backend/docker-compose.yaml` (Sec 9). `backend-cit` (8081) is **not deployed**: it holds only empty `doc.go` files (CIT functions are built in `backend/`, decision 2026-10-08) and is planned to be renamed to `backend-exq` (export + dashboard monitoring queries). It enters the deploy only once `backend-exq` has real content. The module stays in `go.work` and CI keeps building/testing it.

**Planned (2028): split into two separate Compute Engine VMs**, one per backend (once `backend-exq` is deployed). This is *why* the codebase was split into a Go workspace with a shared `pkg/` module (Sec 3) well ahead of the actual VM split: each backend already has its own `go.mod`/Dockerfile/image and zero import-time dependency on the other, so deploying the second backend to its own VM later requires **no code change** — just deploy its image to a new VM and repoint its `.env`. When that split happens:
*   Both VMs must reach the **same** CloudSQL primary/replica (#3/#4) and the **same** Memorystore Redis (#6) — Redis is shared for JWT blacklist + rate-limit counters (`pkg/auth`, `pkg/middleware`), so a second Redis instance would desync those.
*   `JWT_SECRET` must be identical on both VMs via Secret Manager (#9) — CIT only *validates* tokens (`pkg/auth.TokenService`), it never issues them; ATM is the sole issuer.
*   Firewall/VPC rules must allow the CIT VM the same egress to CloudSQL + Memorystore as the ATM VM.

**Pipeline**: CI = GitHub Actions `.github/workflows/ci.yml` on push/PR to `main`/`dev1` (Go vet + golangci-lint + govulncheck + `go test -race` per module, integration tests on Postgres 18 with all migrations and no skips, sqlc drift, Python unittest; frontend + E2E jobs added by fe-tech-debt E1 / be-tech-debt B5). CD = Cloud Build -> build image (`backend/Dockerfile`; `backend-cit/Dockerfile` is added only when `backend-exq` is deployed) -> push to Artifact Registry with immutable tag (git SHA, not latest) -> deploy to Compute Engine (one VM, `backend` only, today; one VM per backend from 2028).
**Rollback**: keep previous image SHA deployable, per service.

**Database migrations at deploy (BE-03, be-tech-debt D1–D3)**: migrations are a separate deploy step, never run by the API at start. Runner = `backend/cmd/migrate`, tracked in table `schema_migrations` (created by the runner).
*   **Where**: the operator host (not the API image): needs Go (or a prebuilt `migrate` binary), `psql` on `PATH`, network access to the Cloud SQL **primary**, and the release's `backend/migrations/` checked out at the same git SHA as the image being deployed. `DATABASE_URL` = primary, from Secret Manager (export it in the shell; never put it on the command line). For `sslmode=verify-ca|verify-full` also export `PGSSLROOTCERT` (psql gets the connection via `PG*` env; the password never appears in argv or logs).
*   **When**: outside office hours **07:00–20:00 WIB** (Sec 14), and before the new API image starts. Migrations are written to stay compatible with the previous image (expand first), so rollback = previous image, no down migrations.
*   **Order** (from `backend/`):
    1. `go run ./cmd/migrate status`: every applied row `ok`, pending = exactly the files this release adds. `MISMATCH` (an applied file was edited) or an unexpected pending file → **stop**, do not run `up`.
    2. `go run ./cmd/migrate up`: applies pending files in order, one `psql -v ON_ERROR_STOP=1 -X -q -f` per file, under an advisory lock (a second run waits). On failure it stops at that file (exit ≠ 0, psql error printed) and records nothing for it: fix forward, then run `up` again.
    3. `go run ./cmd/migrate status`: `0 pending`; keep the output in the deploy log.
*   **New environment** (staging/prod first time): empty database → `up` builds everything from `001`. An environment that was migrated by hand before the runner existed → `baseline --through NNN` once (NNN = last file verified present), then `status`; dev was baselined this way (CLAUDE.md Sec 12).
*   Long data migrations (backfills) still follow Sec 14: never inside 07:00–20:00.

**EOD monitoring routing (S6)**: the internal frontend's nginx sends `/api/eod/*` to `eod_retry_scheduler` on the VM (`host.docker.internal:8091`, prefix stripped), everything else under `/api/` to the Go API `:8080`. That service must run with `RETRY_AUTH_MODE=jwt` and `RETRY_AUTH_SECRET` equal to the Go `JWT_SECRET` (Secret Manager) and refuses to start otherwise.

**Python EOD host prerequisite (import_jobs, R2a)**: the VM/container running `backend_python` (ETL scripts + `eod_retry_scheduler` + `service_dsr_etl`) MUST run with `TZ=Asia/Jakarta`. DMAA `file_date` is derived from file mtime in the host timezone; a different TZ makes the DMAA late check misfire. Verify at VM/container setup.

**DSR upload file permissions (QC F5, accepted 2026-10-09)**: the Go API container (`appuser`) writes vendor DSR uploads to the `FTP_DATA` bind mount (`DSR_UPLOAD_DIR`, default `FTP_DATA/DSR`) as `0644` (dir `0755`), because `backend_python` runs as a different host user and must read the file and move it to `backups/`. Accepted because the VM is application-only (no other interactive users). The `FTP_DATA` directory itself must be writable by the Python user. When Python is containerised, tighten to `0640`/`0750` with a shared group (compose `group_add`) and update `backend/internal/service/dsr_upload.go`.

**IP client & proxy tepercaya (QC F13, `.claude/sdlc/client-ip-trust/`)**: the Go API takes the client IP (login rate limit per IP, `audit_logs.ip`) from `X-Forwarded-For` **only** when the connection comes from an address in `TRUSTED_PROXIES` (`pkg/middleware.TrustedRealIP`, read right to left, trusted hops skipped); otherwise from the connection itself.
*   Production value = the address the backend sees Nginx connect from (Docker network gateway on the VM; check `docker network inspect` or the `RemoteAddr` in the request log), as narrow as possible (`/32`). The start log prints `trusted proxies configured count=N` — `0` behind Nginx is wrong.
*   **If left empty in prod**, every user is recorded with Nginx's IP and the per-IP login limit (`RATE_LIMIT_IP`, default 20 / 15 min) is shared by everyone → mass login lockout. An invalid entry stops the service at start.
*   Close port 8080 from outside the VM (firewall / don't publish it) so every request goes through Nginx (defense in depth; the narrow CIDR already stops spoofing from direct callers).
*   When a GCP Load Balancer / Cloud Armor (#11) is added in front of Nginx: append the LB address ranges to `TRUSTED_PROXIES`; no code change.
*   `backend-cit` reads the variable but does not use it yet. Python EOD services: set `RETRY_TRUSTED_PROXIES` / `DSR_ETL_TRUSTED_PROXIES` to the same value (uvicorn `forwarded_allow_ips` via `lib/utils/proxy.py`; empty = proxy headers off, invalid = fail start). Only `eod_retry_scheduler` sits behind Nginx today; `service_dsr_etl` is called by the Go API, so leave it empty unless that changes.

**Ukuran pool DB (BE-09, be-tech-debt E4) — usulan, belum diterapkan di GCP**: the pool size is set per URL with `?pool_max_conns=N` on `DATABASE_URL` / `DATABASE_REPLICA_URL` (pgx; without it pgx uses `max(4, NumCPU)`, i.e. 4 on a small VM — too low for 300 concurrent users). Each API prints `db pool pool=primary|replica max_conns=N` at start.
*   Formula per Cloud SQL instance: `pool_max_conns = (max_connections − cadangan) / jumlah instance` that connect with a Go pool. `max_connections` = `SHOW max_connections;` on that instance (Cloud SQL derives it from the tier's memory unless the flag is set). `cadangan` = `superuser_reserved_connections` (3) + Python EOD (`eod_retry_scheduler` + `service_dsr_etl` + ETL scripts, ~10) + ops/psql/migrations (~5) ≈ **20**.
*   Primary and replica are separate instances: compute each with its own `max_connections`. Only `backend` uses the replica pool; `backend-cit` uses the primary only.

| Env | Instances on primary | Example `max_connections` | `pool_max_conns` primary | `pool_max_conns` replica |
| --- | --- | --- | --- | --- |
| Local dev | 1 (`backend`) | 100 (Postgres default) | 20 (`.env.example`) | — (replica unset = primary) |
| Staging | 1 (`backend`) | 100 | (100 − 20) / 1 = **80** (start at 20) | 20 |
| Prod (1 VM today) | 1 (`backend`) | read from the chosen tier | (max − 20) / 1, cap 50 | (max_replica − 10) / 1, cap 50 |
| Prod (after `backend-exq` is deployed, e.g. 2028 VM per backend) | 2 (`backend` + `backend-exq`) | same | (max − 20) / 2, cap 50 | (max_replica − 10) / 2, cap 50 |

*   Start at 20 per pool and raise only when the dashboard p95 (≤ 3 s) shows pool waits; a pool larger than the DB can serve concurrently only moves the queue into Postgres. Re-compute whenever a new instance (another API replica, a new Python service) starts connecting.

**Request logs, `/ready` and log-based metrics (BE-04, be-tech-debt E1/E2/E5) — metrics and alert are not yet created in GCP (user, outstanding)**
*   Both Go APIs write one JSON line per request via `pkg/middleware.RequestLogger`: `{"level":"INFO|WARN|ERROR","msg":"http_request","method","route","path","status","duration_ms","bytes","request_id","remote_ip"}`. `route` = chi pattern (`/api/v1/vendor-requests/{id}`, low cardinality, use it as the label); `path` = raw path, only for debugging. Level: 5xx `ERROR`, 4xx `WARN`, else `INFO`. Headers, query strings and bodies are never logged. 2xx `/health` + `/ready` are not logged.
*   `GET /ready` (public, like `/health`): pings `db`, `db_replica` (only when `DATABASE_REPLICA_URL` is a separate pool) and `redis` (`backend`) / `db` + `redis` (`backend-cit`), 2 s timeout, answers 200 `{"status":"ok","checks":{…:"ok"}}` or 503 with the failing check marked `"fail"`; the error itself only goes to the log (`msg="readiness check failed"`). Docker `HEALTHCHECK` still uses `/health` (liveness) — moving it to `/ready` would restart containers on every DB/Redis blip, so that is a separate decision. Use `/ready` for an LB health check / uptime check.
*   Precondition: the container's stdout must reach Cloud Logging **parsed as JSON** (Ops Agent / COS logging agent), so fields land in `jsonPayload.*`. Check one entry in Logs Explorer before creating metrics. slog writes `level`, not `severity`, so entries show severity `DEFAULT`; filter on `jsonPayload.level` (or map it in the agent).
*   Logs Explorer filter: `jsonPayload.msg="http_request"` (add `resource.type="gce_instance"` and the container name as needed).
*   **Latency metric** (distribution, label `route`), `gcloud logging metrics create http_request_latency --config-from-file=http_request_latency.yaml`:

```yaml
description: CMS API request latency (ms) per chi route
filter: jsonPayload.msg="http_request"
valueExtractor: EXTRACT(jsonPayload.duration_ms)
labelExtractors:
  route: EXTRACT(jsonPayload.route)
  method: EXTRACT(jsonPayload.method)
metricDescriptor:
  metricKind: DELTA
  valueType: DISTRIBUTION
  unit: ms
  labels:
    - key: route
    - key: method
bucketOptions:
  explicitBuckets:
    bounds: [50, 100, 250, 500, 1000, 2000, 3000, 5000, 10000, 30000]
```

*   **5xx counter** (label `route`), `gcloud logging metrics create http_5xx --config-from-file=http_5xx.yaml`:

```yaml
description: CMS API responses with status >= 500
filter: jsonPayload.msg="http_request" AND jsonPayload.status>=500
labelExtractors:
  route: EXTRACT(jsonPayload.route)
metricDescriptor:
  metricKind: DELTA
  valueType: INT64
  unit: "1"
  labels:
    - key: route
```

*   **Example alert — p95 > 3 s (NFR dashboard ≤ 3 s p95)**: Cloud Monitoring → Alerting → condition on `logging.googleapis.com/user/http_request_latency`, aligner `ALIGN_PERCENTILE_95` over 5 min, group by `metric.label.route`, threshold **> 3000** (ms) for 10 min, notify the app-support channel (email). Narrow `metric.label.route` to the routes the dashboard calls once they are confirmed; start with all routes. The bucket bound at 3000 keeps the p95 estimate exact at the threshold. Companion alert: `http_5xx` rate > 0 for 5 min (or > N/min once a baseline exists).
*   Outstanding (user): create both metrics + the alert + notification channel in the GCP project; confirm JSON parsing in Cloud Logging; decide whether an LB/uptime check uses `/ready`.
