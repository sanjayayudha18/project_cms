# CMS Tech Stack (Kiro Steering Doc)

# Tech Stack

> Synced with `.claude/CLAUDE.md` (Sec 2, 2a, 9, 12, 14) on 2026-10-20. If this file and CLAUDE.md disagree, CLAUDE.md wins; fix this file.

## Decision Record (SOURCE OF TRUTH)

| Layer | Choice |
|-------|--------|
| Backend (transactional API) | Go + Chi (v5) |
| Backend (EOD / ETL runtime) | Python + FastAPI (`backend_python/`) — valid third runtime, user decision 2026-09-25 |
| DB pool | pgx v5 / pgxpool (Go; primary + replica pools built in `backend/cmd/api/main.go`, **no** shared `pkg/database`) · asyncpg (`backend_python/lib`, FastAPI services) · psycopg 3 (Python ETL scripts) |
| SQL codegen | sqlc **v1.31.1 (pinned)** — `backend/sqlc.yaml`, queries in `backend/queries/` |
| API contract | OpenAPI 3.0.3, hand-written — `backend/api/openapi.yaml` (pilot: `/vendor-requests*`, `/vendor/replenish-orders*`); kin-openapi v0.151.0 (Go tests only), openapi-typescript 7.13.0 (CompanyPortal devDependency) |
| Frontend | React + Vite, served via Nginx |
| Database | PostgreSQL (external, NON-dockerized) — primary + read replica |
| Cache | Redis (Memorystore in prod) |
| Auth internal | LDAP → JWT |
| Auth vendor | Local credentials (CMS DB) → JWT |
| Email | Company SMTP relay |
| Container | Docker + docker-compose (local) |
| CI | GitHub Actions `.github/workflows/ci.yml` is the live CI. Move to Jenkins is specced but **on hold** (`.claude/sdlc/ci-jenkins/`) |
| Cloud | GCP |

> Frontend: Dockerized. Backend: Dockerized or local. Database: NOT dockerized.

> **Three runtimes, not two.** The Go+Chi API (`backend/`, plus `backend-exq/`) is the transactional/office-hours runtime. React+Vite is the frontend. **Python+FastAPI (`backend_python/`) is the EOD / ETL runtime** — DMAA/ITM/DSR ingest, the retry scheduler, and the replenish classification job. It is NOT a Go `cmd/batch` (that entrypoint was deliberately dropped). "Backend is Go + Chi" below describes the transactional API only; EOD is Python. Keep this file, `structure.md`, `project-context.md`, and `.claude/CLAUDE.md` in sync — all four are canonical.

---

## Frontend

| Layer | Technology | Version | Purpose |
|-------|-----------|---------|---------|
| Framework | React | 19 | UI components |
| Language | TypeScript | 5.x | Type safety |
| Build | Vite | 6 | Dev server + production build |
| Router | TanStack Router | ^1.170.44 (seroval 1.6.9) | Type-safe file-based routing; route-level lazy loading in CompanyPortal |
| Server state | TanStack Query | v5 | Caching, optimistic updates, background refetch |
| Tables | TanStack Table | v8 | Headless table for DSR, orders, invoices, recaps |
| Forms | React Hook Form + Zod | RHF 7 / Zod 3 | Complex multi-field forms |
| Client state | Zustand | 5 | Auth stores and small UI state |
| Charts | Recharts | 3 | Dashboard charts |
| Styling | Tailwind CSS | 4 | OKLCH-native, maps to design tokens |
| Icons | Lucide React | latest | Consistent icon set |
| API types | openapi-typescript | 7.13.0 (exact) | `pnpm run gen:api` → `src/lib/api/schema.d.ts`; commit it, a vitest fails when stale |

Not adopted yet: `@react-pdf/renderer` (no PDF consumer; `internal/export` XLSX/PDF deferred). Adding it needs approval (Golden Rule #1).

Security: CSP + security headers are set in both portals' Nginx config (FE-01). `xlsx` was removed from CompanyPortal (FE-02); `pnpm audit --prod` must stay at 0.

## Backend

| Layer | Technology | Version | Purpose |
|-------|-----------|---------|---------|
| Language | Go | 1.26 (`go.work`, toolchain go1.26.9) | Backend runtime |
| HTTP framework | Chi | v5 | Lightweight router, middleware composition |
| DB pool | pgx / pgxpool | v5 | Primary + replica pools (in `cmd/api/main.go`) |
| DB access | sqlc | v1.31.1 (pinned) | SQL-first, type-safe generated Go code |
| Migrations | Custom runner `backend/cmd/migrate` | — | `status \| up \| baseline --through NNN`; numbered SQL in `backend/migrations/`, tracked in `schema_migrations` (SHA-256, advisory lock, applied through `psql`). **golang-migrate is NOT used.** |
| Validation | go-playground/validator | v10 | Struct tag validation |
| Auth | Custom JWT + RBAC middleware (`pkg/auth`, `pkg/middleware`) | golang-jwt v5 | Multi-role auth, maker-checker approval hierarchy |
| Redis client | go-redis | v9 | JWT blacklist, rate-limit counters |
| Email | stdlib `net/smtp` | — | Outbox worker goroutine in `cmd/api` (`notification_emails`) |
| Property tests | pgregory.net/rapid | v1.3.0 | Property-based tests |
| PDF generation | Typst CLI | — | **Not adopted.** Candidate for berita acara / surat; needs approval + `internal/export` first |
| Excel | excelize | — | **Not adopted.** Master-data bulk import/export is CSV (stdlib `encoding/csv`) only |
| Queue | asynq | — | **Not adopted.** Async is decided per-feature (goroutine + DB job table vs Redis queue vs Python pipeline) |
| Logging | slog (stdlib) | — | Structured JSON; `RequestLogger` emits `http_request` |
| Config | `pkg/config` | — | Environment-based configuration |
| HTTP client | net/http (stdlib) | — | Future integration calls |

Middleware baseline (both backends): `TrustedRealIP(TRUSTED_PROXIES)` (client IP only from `r.RemoteAddr`; never read IP headers in handlers), `RequestLogger`, `BodyLimit` 1 MiB (DSR upload + master-data import exempt), chi `Timeout(30s)`, public `GET /health` and `GET /ready` (db / db_replica / redis, 200/503).

## Backend Topology

Two Go modules in a workspace (`go.work`: `backend`, `backend-exq`, `pkg`), sharing `pkg/`:

| Service | Module | Port | Role |
|---------|--------|------|------|
| ATM backend | `backend/` | 8080 | ATM operations, **CIT functions** (decision 2026-10-08), auth issuer, master data, approval, notification, vendor request flow |
| EXQ backend | `backend-exq/` | 8081 | Export + dashboard monitoring queries (replica reads). Renamed from `backend-cit` on 2026-10-20. Skeleton only; not deployed until it has content |

- `pkg/` is shared infra (`auth`, `middleware`, `config`, `response`, `health`). No dependency on either backend. There is no `pkg/database`.
- ATM backend keeps flat JSON responses for wire compatibility, including the CIT endpoints built there. `backend-exq` uses the `pkg/response` envelope.
- Both reach the same PostgreSQL primary/replica and the same Redis instance, and share one `JWT_SECRET`. Only ATM issues tokens.
- **Deployment (current):** ONE Compute Engine VM runs `backend` only. `backend-exq` is deployed once it has content. Planned split to one VM per backend in 2028 — the module separation already makes that a zero-code-change deployment change.

## Python EOD / ETL Runtime (`backend_python/`)

The End-of-Day and ingest pipeline runs as Python, not Go. It shares the same PostgreSQL and validates the same JWTs the Go API issues (never issues its own).

| Layer | Technology | Purpose |
|-------|-----------|---------|
| Language | Python 3.14 (Docker `3.14-slim`, CI 3.14) | ETL + scheduler runtime |
| API framework | FastAPI 0.141 + uvicorn | HTTP services (`service_dsr_etl`, `eod_retry_scheduler`) |
| DB driver (services) | asyncpg | Async pool (`lib/database.py`); `db_read_pool` for replica reads |
| DB driver (ETL scripts) | psycopg 3 (`psycopg[binary]`) | Standalone ETL CLIs |
| Data parsing | pandas, openpyxl, xlrd | DMAA / DSR / ITM files. Money parsed as `Decimal`, never float |
| Models / config | pydantic v2 + pydantic-settings | Request/response models, env config |
| Scheduling | APScheduler | Retry scheduler, timed EOD triggers |
| Auth | **PyJWT 2.15.1** (HS256 only, `exp` required) | Validates the Go-issued JWT (shared `JWT_SECRET`). `python-jose` was replaced (BE-05) |

All `requirements.txt` files pin exact versions (`==`). Tests use stdlib `unittest` (pytest is not installed).

Layout inside `backend_python/`:

| Path | Role |
|------|------|
| `lib/` | Shared: `database.py`, `dependencies.py`, `schemas.py`, `import_jobs.py`, `services/` (scheduler, detector, retry executor, audit), `utils/` (`timezone.WIB`, `checksum`, `proxy`) |
| `service_dsr_etl/` | FastAPI :8090 — vendor DSR upload dry-run/commit, late/summary/audit/retry routers |
| `eod_retry_scheduler/` | FastAPI :8091 — EOD scan + retry scheduling (dmaa, itm_cashpos, itm_replenish) |
| `dmaa/dmaa_etl.py` | DMAA order ETL → `dmaa_files`, `dmaa_atm_forecast` (CROWN does no forecasting) |
| `dsr/dsr_etl.py` | DSR ETL (+ `test_dsr_etl.py`) |
| `itm/{cashpos,replenish}/` | ITM cash-position + replenishment ETL |
| `replenish_classify/` | Replenish realisasi vs order classification job (`job.py`, `rules.py`, `run.py`); runs after a successful `itm_replenish` file, cron 09:15 WIB, and on manual run. Writes `notifications` + `notification_emails` directly (keep in sync with Go `notification.Send`) |
| `Dockerfile` | `python:3.14-slim`, non-root, `TZ=Asia/Jakarta`; both services healthy |

- **Response envelope (Python):** `{status, data, error}` — similar in spirit to Go's `pkg/response` but a separate shape; do not assume they are identical.
- Each FastAPI service has its own `requirements.txt`, `config.py`, `main.py` (app factory + lifespan), `run.py`, `.env.example`.
- The EOD monitoring page calls `/api/eod/*`; Vite dev proxy and CompanyPortal `nginx.conf` strip the prefix and forward to `:8091`.
- Host running the ETL/scheduler must use `TZ=Asia/Jakarta`.

## Infrastructure (Dockerized)

| Service | Image | Purpose |
|---------|-------|---------|
| ATM Backend API | Custom (multi-stage Go build, `backend/Dockerfile`, Alpine 3.24, non-root) | Port 8080, HEALTHCHECK on `/health` |
| EXQ Backend API | Custom (multi-stage Go build, `backend-exq/Dockerfile`) | Port 8081. Still root + no HEALTHCHECK (open item in BE-06) |
| Python ETL / EOD | `backend_python/Dockerfile` (python:3.14-slim) | DSR ETL + EOD retry scheduler services |
| Frontend (internal) | nginx:alpine + static build | Serves CompanyPortal-Vite output |
| Frontend (vendor) | nginx:alpine + static build | Serves VendorPortal-Vite output |
| Redis | redis:7-alpine | JWT blacklist + rate-limit counters |

Compose files: `backend/docker-compose.yaml` (backend + redis), `backend-exq/docker-compose.yaml`, `frontend/docker-compose.yml`. Both backend Dockerfiles build from the repo root (`context: ..`) so `pkg/` resolves through `go.work`. Root `.dockerignore` is an allowlist (`go.work*`, `pkg/`, `backend/`, `backend-exq/`; never `.env`). There is no root `docker-compose.yml`.

## Database (NOT Dockerized)

| Component | Technology | Notes |
|-----------|-----------|-------|
| Primary DB | PostgreSQL (dev and CI on 18) | External instance, managed separately (CloudSQL in prod) |
| Read Replica | PostgreSQL | Reporting, dashboards, heavy reads |
| Extensions | pgcrypto, pg_trgm, btree_gist | UUID generation, trigram search, effective-date exclusion constraints |

Dev DB host: `backend/.env` uses `host.docker.internal`, which only resolves inside a container. From a host shell use `localhost:5432`.

## Dev Tooling

| Tool | Purpose |
|------|---------|
| pnpm | Frontend package manager |
| Biome | Frontend lint + format |
| golangci-lint | Go linting, root `.golangci.yml`, run per module with `--build-tags integration` |
| govulncheck | Go vulnerability scan |
| Vitest (+ coverage-v8, fast-check) | Frontend unit/integration tests; thresholds CP 84/80/80/85, VP 93/84/90/93 (`pnpm run test:coverage`) |
| Playwright | E2E tests |
| Docker Compose | Local dev orchestration |
| graphify | Codebase knowledge graph (`graphify-out/`) |

Not in use: Taskfile / `task check`, `air`, golang-migrate. Do not reference them in new docs.

## Build Commands

```bash
# Frontend (pnpm; same scripts in both apps)
pnpm --dir frontend/CompanyPortal-Vite run dev|build|test|lint     # dev :5173
pnpm --dir frontend/VendorPortal-Vite  run dev|build|test|lint     # dev :5174
pnpm --dir frontend/CompanyPortal-Vite run gen:api                 # regenerate src/lib/api/schema.d.ts after editing openapi.yaml

# Go (run per module; go.work links them)
cd backend && go test ./...          # integration tests skip when DATABASE_URL is empty
cd backend-exq && go test ./...
cd pkg && go test ./...
cd backend && go run ./cmd/api       # ATM API :8080 (backend-exq: go run ./cmd/api → :8081)
cd backend && golangci-lint run --build-tags integration ./...
cd backend && govulncheck ./...

# sqlc (after editing backend/queries/*.sql) — MUST be v1.31.1
cd backend && sqlc generate          # then hand-fix UserLeafe → UserLeave in backend/internal/db/

# OpenAPI contract (after editing backend/api/openapi.yaml or a handler it covers)
cd backend && go test ./internal/handler -run OpenAPI

# Migrations (never `psql -f` by hand; it would not be recorded)
cd backend && go run ./cmd/migrate status   # exit != 0 on checksum MISMATCH
cd backend && go run ./cmd/migrate up       # needs psql on PATH, DATABASE_URL via localhost

# Python EOD (run from backend_python/ so `lib` imports resolve)
cd backend_python && python -m eod_retry_scheduler.run     # :8091
cd backend_python && python -m service_dsr_etl.run         # :8090
cd backend_python && python -m unittest discover -s dsr
cd backend_python && python -m unittest lib.test_import_jobs -v   # skips when DATABASE_URL is empty

# Docker
docker compose -f backend/docker-compose.yaml up -d --build
```

## Architecture Principles

1. **Clean separation:** Frontend knows nothing about DB. Backend exposes REST API only.
2. **SQL-first:** Use sqlc — write real SQL, get type-safe Go. No ORM magic.
3. **Async when needed, decided per-feature:** long work (reconciliation, PDF gen, classification) should not block a request, but there is no blanket asynq worker. Choose the mechanism (goroutine + DB job table, Redis queue, or the Python ETL pipeline) when the feature lands. EOD/ingest runs in the Python runtime.
4. **Layered backend, feature-based frontend:** ATM `backend/internal/` is organized by layer (`handler/` → `service/` → `repository/` → `db/`) with cross-cutting packages (`approval/`, `audit/`, `auth/`, `notification/`, `rolemgmt/`). Frontend groups code by business domain under `src/features/`.
5. **Single binary deploy:** Go compiles to one binary per entrypoint. No runtime dependencies in container.
6. **API contract:** Hand-written OpenAPI is the contract for modules already in `backend/api/openapi.yaml`; the frontend types are generated from it. A new module starts with its spec. Modules not yet covered stay hand-typed (BE-10 deferred the rest).
7. **File ingest → DB, not file storage:** Uploads (DSR, DMAA, ITM, master-data CSV) are parsed into structured DB rows, idempotent per SHA-256 file hash through `import_jobs`.
8. **Sync parse for small files:** Uploads are small (< 1000 rows). Parse synchronously in the handler. No queue needed.
9. **Go workspace isolation:** `backend/` and `backend-exq/` never import each other. Shared code lives in `pkg/` only.
10. **Maker-checker for master data:** new entities follow queries → read-only repo → service `Submit` → `Applier` in `main.go` → handler 202. Use the `/cms2-master-data` skill.
