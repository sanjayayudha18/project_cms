# CMS Tech Stack (Kiro Steering Doc)

# Tech Stack

## Decision Record
- **Backend language:** Go (chosen over Node.js for compile-time safety, single-binary deploy, banking-sector fit)
- **Frontend framework:** React + TypeScript + Vite (committed via design system)
- **Database:** PostgreSQL 16 (NOT dockerized — external instance)
- **App containerization:** Docker (all services except DB)

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
| DB access | sqlc | latest | SQL-first, type-safe generated Go code |
| Migrations | golang-migrate | latest | Versioned schema migrations |
| Validation | go-playground/validator | v10 | Struct tag validation |
| Auth | Custom JWT + RBAC middleware | — | Multi-role auth (8 roles), D-3 approval hierarchy |
| PDF generation | Typst CLI | latest | Template-based PDF (berita acara, surat konfirmasi, memo) |
| Excel | excelize | latest | DSR import, CIT/CPC recap parsing, schedule export |
| Queue | asynq | latest | Redis-backed async jobs (projections, PDF gen, reconciliation) |
| File storage client | MinIO Go SDK | latest | S3-compatible uploads/downloads |
| Logging | slog (stdlib) | — | Structured logging |
| Config | envconfig or viper | latest | Environment-based configuration |
| HTTP client | net/http (stdlib) | — | Future integration calls |

## Infrastructure (Dockerized)

| Service | Image | Purpose |
|---------|-------|---------|
| Backend API | Custom (multi-stage Go build) | ~15-25MB final image |
| Worker | Same Go binary, different entrypoint | asynq worker for background jobs |
| Frontend | nginx:alpine + static build | Serves Vite production output |
| Redis | redis:7-alpine | Queue backend (asynq) + session cache |
| MinIO | minio/minio | S3-compatible file storage |
| Reverse proxy | caddy:alpine | TLS termination, route /api→backend, /→frontend |
| Typst | Custom (typst binary in worker image) | PDF rendering engine |

## Database (NOT Dockerized)

| Component | Technology | Notes |
|-----------|-----------|-------|
| Primary DB | PostgreSQL 16 | External instance, managed separately |
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
# Frontend
pnpm install          # Install dependencies
pnpm dev              # Vite dev server
pnpm build            # Production build
pnpm test             # Vitest
pnpm lint             # Biome check
pnpm format           # Biome format

# Backend
go mod tidy           # Sync dependencies
air                   # Dev server with hot reload
go build -o bin/api cmd/api/main.go       # Build API
go build -o bin/worker cmd/worker/main.go # Build worker
go test ./...         # Run all tests
golangci-lint run     # Lint

# Database
migrate -path migrations -database $DATABASE_URL up    # Run migrations
migrate -path migrations -database $DATABASE_URL down  # Rollback
sqlc generate         # Regenerate DB query code

# Docker
docker compose up -d           # Start all services
docker compose up -d --build   # Rebuild and start
docker compose down            # Stop all services

# Full quality gate
task check            # Runs lint + test + build for both frontend and backend
```

## Architecture Principles

1. **Clean separation:** Frontend knows nothing about DB. Backend exposes REST API only.
2. **SQL-first:** Use sqlc — write real SQL, get type-safe Go. No ORM magic.
3. **Async by default:** PDF generation, projection computation, and reconciliation batches go through asynq. API returns immediately with a job ID.
4. **Feature-based organization:** Both frontend and backend group code by business domain (forecasting, invoice, cash-count), not by technical layer.
5. **Single binary deploy:** Go compiles to one binary per entrypoint. No runtime dependencies in container.
6. **Shared nothing between frontend/backend:** No code sharing. API contract defined via OpenAPI spec, frontend generates typed client from it.
7. **Excel → DB, not file storage:** Excel uploads (DSR, CIT/CPC, orders) are parsed into structured DB rows on upload. No original files stored — audit trail comes from DB records (upload metadata, parsed rows, timestamps, uploader). Each upload type has its own parser implementing a shared `ExcelParser` interface.
8. **Sync parse for small files:** CMS uploads are < 1000 rows — parse synchronously in the API handler. No queue needed for Excel processing.
