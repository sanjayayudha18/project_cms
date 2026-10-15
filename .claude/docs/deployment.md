# GCP Deployment (0 -> Deployed)

> Moved verbatim from `.claude/CLAUDE.md` Sec 10 on 2026-09-29. "Sec N" references point to CLAUDE.md.


| # | Product | Role |
| ---| ---| --- |
| 1 | Compute Engine | Frontend (internal + vendor) |
| 2 | Compute Engine | Backend, currently ONE VM running both `backend/` (ATM, port 8080) and `backend-cit/` (CIT, port 8081) as separate containers — see split-VM plan below |
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

**Current state (now): ONE Compute Engine VM runs both backends** as separate Docker containers (`backend` on 8080, `backend-cit` on 8081) — see root `docker-compose.yml` (Sec 9). Fine for now: CIT is still a skeleton with no real endpoints, low load.

**Planned (2028): split into two separate Compute Engine VMs**, one per backend. This is *why* the codebase was split into a Go workspace with a shared `pkg/` module (Sec 3) well ahead of the actual VM split: each backend already has its own `go.mod`/Dockerfile/image and zero import-time dependency on the other, so moving CIT to its own VM later requires **no code change** — just deploy the existing `cms-backend-cit` image to a new VM and repoint its `.env`. When that split happens:
*   Both VMs must reach the **same** CloudSQL primary/replica (#3/#4) and the **same** Memorystore Redis (#6) — Redis is shared for JWT blacklist + rate-limit counters (`pkg/auth`, `pkg/middleware`), so a second Redis instance would desync those.
*   `JWT_SECRET` must be identical on both VMs via Secret Manager (#9) — CIT only *validates* tokens (`pkg/auth.TokenService`), it never issues them; ATM is the sole issuer.
*   Firewall/VPC rules must allow the CIT VM the same egress to CloudSQL + Memorystore as the ATM VM.

**Pipeline**: CI = GitHub Actions `.github/workflows/ci.yml` on push/PR to `main`/`dev1` (Go vet + golangci-lint + govulncheck + `go test -race` per module, integration tests on Postgres 18 with all migrations and no skips, sqlc drift, Python unittest; frontend + E2E jobs added by fe-tech-debt E1 / be-tech-debt B5). CD = Cloud Build -> build images (`backend/Dockerfile`, `backend-cit/Dockerfile`, separately) -> push to Artifact Registry with immutable tag (git SHA, not latest) -> deploy to Compute Engine (one VM today, one VM per backend from 2028).
**Rollback**: keep previous image SHA deployable, per service.

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
| Staging | 2 (`backend` + `backend-cit`) | 100 | (100 − 20) / 2 = **40** (start at 20) | 20 |
| Prod (1 VM today) | 2 (`backend` + `backend-cit`) | read from the chosen tier | (max − 20) / 2, cap 50 | (max_replica − 10) / 1, cap 50 |
| Prod (2028, VM per backend) | unchanged count (one pool each) | same | same formula | same |

*   Start at 20 per pool and raise only when the dashboard p95 (≤ 3 s) shows pool waits; a pool larger than the DB can serve concurrently only moves the queue into Postgres. Re-compute whenever a new instance (another API replica, a new Python service) starts connecting.
