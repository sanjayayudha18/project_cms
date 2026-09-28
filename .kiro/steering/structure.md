# Project Structure

## Layout — Go Workspace

```
CMS2/
├── go.work                       # Go workspace: links backend/, backend-cit/, pkg/
│
├── pkg/                          # Shared infra (own go.mod, no dependency on either backend)
│   ├── auth/                     # JWT TokenService, blacklist, Provider/UserRepository interfaces
│   ├── middleware/               # RequireAuth, RequireRoles, rate limiter
│   ├── config/                   # Env config loader, Load(defaultPort)
│   ├── response/                 # {success,data} JSON envelope (used by backend-cit only)
│   └── database/                 # pgxpool: primary (write) + replica (read)
│
├── backend/                      # ATM backend — own go.mod, port 8080
│   ├── cmd/
│   │   ├── api/main.go           # HTTP server entrypoint (bootstrap + route registration)
│   │   └── hashpw/               # Password-hash CLI helper (dev)
│   ├── internal/                 # Organized by LAYER, not by feature (as built):
│   │   ├── handler/              # HTTP handlers (one file per resource) + *_test.go
│   │   ├── service/              # Business logic + Applier registry (maker-checker apply)
│   │   ├── repository/           # DB access (hand-written + sqlc-style)
│   │   ├── db/                   # sqlc-generated / query layer
│   │   ├── auth/                 # Login (LDAP + local), JWT issue, password lifecycle
│   │   ├── approval/             # Maker-checker orchestrator, chain resolver, delegation/leave
│   │   ├── audit/                # Append-only audit writer
│   │   └── rolemgmt/             # Role/permission catalog (immediate-apply + audit-in-tx)
│   ├── migrations/               # ALL DB migrations (ATM + CIT tables); baseline 001/002 + 003–018
│   ├── go.mod
│   └── go.sum
│
├── backend_python/               # Python EOD / ETL runtime (FastAPI + uvicorn, asyncpg)
│   ├── lib/                      # Shared: database.py (asyncpg pool), dependencies, schemas, services/, utils/
│   ├── service_dsr_etl/          # FastAPI: DSR upload dry-run/confirm, late/summary/audit/retry routers
│   ├── eod_retry_scheduler/      # FastAPI: EOD retry scheduling (APScheduler)
│   ├── dmaa/dmaa_etl.py          # DMAA forecast ETL
│   ├── dsr/dsr_etl.py            # DSR ETL (+ test_dsr_etl.py)
│   └── itm/{cashpos,replenish}/  # ITM cash-position + replenishment ETL
│
├── backend-cit/                  # CIT backend — own go.mod, port 8081
│   ├── cmd/
│   │   └── api/main.go           # Health check + RequireAuth-protected routes; validates tokens, issues none
│   ├── internal/
│   │   ├── cit/                  # CIT orders, handover
│   │   ├── journal/              # CIT journals
│   │   ├── dsr/                  # CIT DSR uploads
│   │   ├── reconciliation/       # CIT reconciliation (order vs DSR vs journal)
│   │   └── integration/          # Corebanking escrow batch ingest
│   ├── go.mod
│   └── go.sum
│
├── frontend/
│   ├── CompanyPortal-Vite/       # Internal app (LDAP login)
│   │   ├── src/
│   │   │   ├── components/       # Shared UI components
│   │   │   ├── features/         # Feature modules
│   │   │   ├── lib/              # Utilities, API client, auth
│   │   │   ├── routes/           # TanStack Router file-based routes
│   │   │   └── styles/           # Design tokens, Tailwind config
│   │   ├── package.json
│   │   └── vite.config.ts
│   │
│   └── VendorPortal-Vite/        # Vendor portal (local login)
│       ├── src/
│       │   ├── components/
│       │   ├── features/
│       │   ├── lib/
│       │   ├── routes/
│       │   └── styles/
│       ├── package.json
│       └── vite.config.ts
│
├── docker-compose.yml            # Local dev: backend + backend-cit + redis (Postgres external)
├── backend/Dockerfile            # Multi-stage, builds from repo root (context .) for go.work
├── backend-cit/Dockerfile        # Multi-stage, builds from repo root (context .) for go.work
├── .env.example                  # Environment variables template
│
├── documents/                    # Business requirements (existing)
├── archives/                     # Archived docs (existing)
└── .kiro/                        # Steering, agents, hooks
```

## Key Architecture Rules

- `pkg/` depends on neither backend. `backend/` and `backend-cit/` depend on `pkg/` only — never on each other. Compiler-enforced acyclic.
- `backend/migrations/` is the sole owner of ALL DB migrations (including CIT tables). `backend_python/` reads/writes the same schema but never owns migrations.
- ATM backend keeps existing flat JSON response shape for wire compatibility. CIT backend uses `pkg/response` envelope. Python services use their own `{status, data, error}` envelope.
- Each backend builds and deploys as an independent artifact (separate Compute Engine in prod).
- **`backend/internal/` is organized by layer** (handler/service/repository/db) with cross-cutting domains (auth/approval/audit/rolemgmt) as sibling packages — NOT the by-feature layout an older version of this doc showed. Match the layer layout when adding code.
- **`backend-cit/` is still a skeleton:** `/health` + an auth-protected empty route group only; every `internal/*` package (`cit`, `journal`, `dsr`, `reconciliation`, `integration`) is a stub file. Build it out only in its own late phase (see `development-plan.md`).
- **Python runtime owns EOD/ETL, not Go.** There is no Go `cmd/batch` — that entrypoint was dropped (decision 2026-09-25). Batch/ingest work lives in `backend_python/`.

## Guidelines

- Go backend: organize `internal/` by layer (handler/service/repository/db) with cross-cutting domains as sibling packages, matching the current build.
- Frontend: group by feature under `src/features/`.
- Python: shared code in `backend_python/lib/`; each FastAPI service is its own folder with `main.py`/`config.py`/`run.py`/`requirements.txt`.
- Co-locate tests with source: `*_test.go` next to the file under test (Go); `test_*.py` beside the module (Python); `__tests__/` beside the feature (frontend).
- Keep the folder structure flat until complexity demands nesting.
- Update this file whenever new top-level directories are introduced, and keep it in sync with `tech.md`, `project-context.md`, and `.claude/CLAUDE.md` (all canonical).
