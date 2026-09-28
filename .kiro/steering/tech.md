# CMS Tech Stack (Kiro Steering Doc)

# Tech Stack

## Decision Record (SOURCE OF TRUTH)

| Layer | Choice |
|-------|--------|
| Backend (transactional API) | Go + Chi (v5) |
| Backend (EOD / ETL runtime) | Python + FastAPI (`backend_python/`) — valid third runtime, user decision 2026-09-25 |
| DB pool | pgx / pgxpool (`pkg/database`, Go) · asyncpg (`backend_python/lib`, Python) |
| Frontend | React + Vite, served via Nginx |
| Database | PostgreSQL (external, NON-dockerized) — primary + read replica |
| Cache | Redis (Memorystore in prod) |
| Auth internal | LDAP → JWT |
| Auth vendor | Local credentials (CMS DB) → JWT |
| Email | Company SMTP relay |
| Container | Docker + docker-compose (local) |
| Cloud | GCP |

> Frontend: Dockerized. Backend: Dockerized or local. Database: NOT dockerized.

> **Three runtimes, not two.** The Go+Chi API (`backend/`, `backend-cit/`) is the transactional/office-hours runtime. React+Vite is the frontend. **Python+FastAPI (`backend_python/`) is the EOD / ETL runtime** — DMAA/ITM/DSR ingest + the retry scheduler. It is NOT a Go `cmd/batch` (that entrypoint was deliberately dropped). "Backend is Go + Chi" below describes the transactional API only; EOD is Python. Keep this file, `structure.md`, `project-context.md`, and `.claude/CLAUDE.md` in sync — all four are canonical.

---

## Frontend

| Layer | Technology | Version | Purpose |
|-------|-----------|---------|---------|
| Framework | React | 19 | UI components |
| Language | TypeScript | 5.x | Type safety |
| Build | Vite | 6 | Dev server + production build |
| Router | TanStack Router | latest | Type-safe file-based routing |
| Server state | TanStack Query | v5 | Caching, optimistic updates, background refetch |
| Tables | TanStack Table | v8 | Headless table for DSR, orders, invoices, recaps |
| Forms | React Hook Form + Zod | latest | Complex multi-field forms (berita acara, checklists) |
| Styling | Tailwind CSS | 4 | OKLCH-native, maps to design tokens |
| PDF preview | @react-pdf/renderer | latest | Client-side PDF preview before server generation |
| Icons | Lucide React | latest | Consistent icon set |

## Backend

| Layer | Technology | Version | Purpose |
|-------|-----------|---------|---------|
| Language | Go | 1.23+ | Backend runtime |
| HTTP framework | Chi | v5 | Lightweight router, middleware composition |
| DB pool | pgx / pgxpool | v5 | Connection pooling, primary + replica pools |
| DB access | sqlc | latest | SQL-first, type-safe generated Go code |
| Migrations | golang-migrate | latest | Versioned schema migrations (owned by `backend/migrations/`) |
| Validation | go-playground/validator | v10 | Struct tag validation |
| Auth | Custom JWT + RBAC middleware (`pkg/auth`, `pkg/middleware`) | — | Multi-role auth, D-3 approval hierarchy |
| PDF generation | Typst CLI | latest | Template-based PDF (berita acara, surat konfirmasi, memo) |
| Excel | excelize | latest | DSR import, CIT/CPC recap parsing, schedule export |
| Queue | asynq (candidate) | latest | Redis-backed async jobs. NOT yet adopted — a standalone async worker was dropped (plan 2026-09-25); async is decided per-feature (goroutine + DB job table vs Redis queue) when a feature needs it |
| Logging | slog (stdlib) | — | Structured logging |
| Config | envconfig or viper | latest | Environment-based configuration (`pkg/config`) |
| HTTP client | net/http (stdlib) | — | Future integration calls |

## Backend Topology

Two separate Go modules in a workspace (`go.work`), sharing `pkg/`:

| Service | Module | Port | Role |
|---------|--------|------|------|
| ATM backend | `backend/` | 8080 | ATM operations, auth issuer, master data, invoice |
| CIT backend | `backend-cit/` | 8081 | CIT orders, journals, DSR, reconciliation, integration |

- `pkg/` is shared infra (auth, middleware, config, response, database). No dependency on either backend.
- ATM backend keeps flat JSON responses for wire compatibility. CIT backend uses `pkg/response` envelope.
- Both reach the same PostgreSQL primary/replica and the same Redis instance.
- **Deployment (current):** ONE Compute Engine VM runs both backends as separate containers. Planned split to two VMs in 2028 — the module separation already makes that a zero-code-change deployment change.

## Python EOD / ETL Runtime (`backend_python/`)

The End-of-Day and ingest pipeline runs as Python, not Go. It shares the same PostgreSQL and validates the same JWTs the Go API issues (never issues its own).

| Layer | Technology | Purpose |
|-------|-----------|---------|
| Language | Python 3.11+ | ETL + scheduler runtime |
| API framework | FastAPI + uvicorn | HTTP services (`service_dsr_etl`, `eod_retry_scheduler`) |
| DB driver | asyncpg | Async pool (`lib/database.py`) — Python side does NOT use pgx |
| Models / config | pydantic v2 + pydantic-settings | Request/response models, env config |
| Scheduling | APScheduler | Retry scheduler, timed EOD triggers |
| Auth | python-jose[cryptography] | Validates the Go-issued JWT (shared `JWT_SECRET`) |

Layout inside `backend_python/`:

| Path | Role |
|------|------|
| `lib/` | Shared: `database.py` (asyncpg pool), `dependencies.py`, `schemas.py`, `services/` (e.g. `SchedulerService`), `utils/` (e.g. `timezone.WIB`) |
| `service_dsr_etl/` | FastAPI service — vendor DSR upload dry-run/confirm, late/summary/audit/retry routers |
| `eod_retry_scheduler/` | FastAPI service — EOD retry scheduling |
| `dmaa/dmaa_etl.py` | DMAA forecast ETL script |
| `dsr/dsr_etl.py` | DSR ETL script (+ `test_dsr_etl.py`) |
| `itm/{cashpos,replenish}/` | ITM cash-position + replenishment ETL |

- **Response envelope (Python):** `{status, data, error}` — similar in spirit to Go's `pkg/response` but a separate shape; do not assume they are identical.
- Each FastAPI service has its own `requirements.txt`, `config.py`, `main.py` (app factory + lifespan), `run.py`, `.env.example`.

## Infrastructure (Dockerized)

| Service | Image | Purpose |
|---------|-------|---------|
| ATM Backend API | Custom (multi-stage Go build, `backend/Dockerfile`) | ~15-25MB final image, port 8080 |
| CIT Backend API | Custom (multi-stage Go build, `backend-cit/Dockerfile`) | ~15-25MB final image, port 8081 |
| Python ETL / EOD | uvicorn + FastAPI (`backend_python/*`) | DSR ETL + EOD retry scheduler services |
| Frontend (internal) | nginx:alpine + static build | Serves CompanyPortal-Vite output |
| Frontend (vendor) | nginx:alpine + static build | Serves VendorPortal-Vite output |
| Redis | redis:7-alpine | Queue backend (asynq) + JWT blacklist + rate-limit counters |

## Database (NOT Dockerized)

| Component | Technology | Notes |
|-----------|-----------|-------|
| Primary DB | PostgreSQL 16 | External instance, managed separately (CloudSQL in prod) |
| Read Replica | PostgreSQL 16 | Reporting, dashboards, heavy reads |
| Extensions | pgcrypto, pg_trgm | UUID generation, text search |

## Dev Tooling

| Tool | Purpose |
|------|---------|
| pnpm | Frontend package manager |
| Biome | Frontend lint + format |
| golangci-lint | Go linting (staticcheck, govet, errcheck, etc.) |
| air | Go hot-reload in development |
| Vitest | Frontend unit/integration tests |
| Playwright | E2E tests |
| Docker Compose | Local dev orchestration |
| Taskfile (go-task) | Cross-platform task runner (replaces Makefile) |

## Build Commands

```bash
# Frontend (run from frontend/CompanyPortal-Vite/ or frontend/VendorPortal-Vite/)
pnpm install          # Install dependencies
pnpm dev              # Vite dev server
pnpm build            # Production build
pnpm test             # Vitest
pnpm lint             # Biome check
pnpm format           # Biome format

# Backend (run from backend/ or backend-cit/)
go mod tidy           # Sync dependencies
air                   # Dev server with hot reload
go build -o bin/api cmd/api/main.go       # Build API
go test ./...         # Run all tests
golangci-lint run     # Lint

# Database (from backend/)
migrate -path migrations -database $DATABASE_URL up    # Run migrations
migrate -path migrations -database $DATABASE_URL down  # Rollback
sqlc generate         # Regenerate DB query code

# Docker (from repo root)
docker compose up -d           # Start all services
docker compose up -d --build   # Rebuild and start
docker compose down            # Stop all services

# Full quality gate
task check            # Runs lint + test + build for both frontend and backend
```

## Architecture Principles

1. **Clean separation:** Frontend knows nothing about DB. Backend exposes REST API only.
2. **SQL-first:** Use sqlc — write real SQL, get type-safe Go. No ORM magic.
3. **Async when needed, decided per-feature:** long work (projection, reconciliation, PDF gen) should not block a request, but there is no blanket asynq worker. Choose the mechanism (goroutine + DB job table, Redis queue, or the Python ETL pipeline) when the feature lands. EOD/ingest runs in the Python runtime.
4. **Feature-based organization:** Both frontend and backend group code by business domain (forecasting, invoice, cit), not by technical layer.
5. **Single binary deploy:** Go compiles to one binary per entrypoint. No runtime dependencies in container.
6. **Shared nothing between frontend/backend:** No code sharing. API contract defined via OpenAPI spec, frontend generates typed client from it.
7. **Excel → DB, not file storage:** Excel uploads (DSR, CIT/CPC, orders) are parsed into structured DB rows on upload. Each upload type has its own parser implementing a shared `ExcelParser` interface.
8. **Sync parse for small files:** CMS uploads are < 1000 rows — parse synchronously in the API handler. No queue needed for Excel processing.
9. **Go workspace isolation:** `backend/` and `backend-cit/` never import each other. Shared code lives in `pkg/` only.
