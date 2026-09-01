# PROJECT_CONTEXT.md — CMS ATM & CIT (OpenCode)

> **AI: READ THIS FIRST, EVERY SESSION.** Single source of truth. If your work conflicts with this, STOP and ask. Never invent endpoints, tables, columns, env vars, or modules.
> **For simple explanations of CMS concepts**, see `.claude/eli5.md` — use when explaining to teammates or stakeholders.
> This file consolidates `.claude/CLAUDE.md` and all `.kiro/steering/*.md` into one reference for OpenCode.

* * *

## 0\. Golden Rules
1. Stack is FIXED (Sec 2). No new libs/frameworks without approval.
2. Follow module & table map (Sec 3). No modules/tables not listed unless asked.
3. Every state-changing action respects maker-checker + writes audit\_log (Sec 5).
4. Money = numeric / integer minor units, never float (Sec 6).
5. Plan before non-trivial work. Small diffs. One concern per change.
6. DB topology: write/update -> primary; read/report/dashboard -> read replica (Sec 6).
7. Two frontends: internal (LDAP) + vendor portal (local auth). Keep separate (Sec 5).
8. Two backend entrypoints, one codebase: `cmd/api` (transactional, office hours) + `cmd/batch` (EOD, midnight). Shared `internal/*` (Sec 14).
9. EOD batch output is the source of truth for the transactional module. Write to DB, signal readiness before transactional reads (Sec 14).
10. OpenCode must not run any Git command. The user handles all Git operations.

* * *

## 1\. Overview
*   **Name**: CROWN THE Cash Management System
*   **Goal**: E2E ATM cash management: vendor replenishment, daily DSR reporting, forecasting & scheduling, cash count (vault + selective machine), reconciliation vs Corebanking escrow, vendor invoice validation & approval.
*   **Stage**: Greenfield, vibe-coded with AI.
*   **Roles**: Admin, Operator, Manager (approver), Vendor, Branch/Internal User.

* * *

## graphify

This project has a knowledge graph at `graphify-out/` with god nodes, community structure, and cross-file relationships.

Rules:
- **Before any coding task**: orient via `graphify query` / `graphify explain` instead of reading the full codebase; fall back to targeted file reads only for what the graph doesn't answer.
- For codebase questions, first run `graphify query "<question>"` when `graphify-out/graph.json` exists. Use `graphify path "<A>" "<B>"` for relationships and `graphify explain "<concept>"` for focused concepts. These return a scoped subgraph, usually much smaller than `GRAPH_REPORT.md` or raw grep output.
- If `graphify-out/wiki/index.md` exists, use it for broad navigation instead of raw source browsing.
- Read `graphify-out/GRAPH_REPORT.md` only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).

* * *

## 2\. Tech Stack (SOURCE OF TRUTH)

| Layer | Choice |
| ---| --- |
| Backend | Go + Chi (v5) |
| DB pool | pgx / pgxpool (`pkg/database`) |
| Frontend | React + TypeScript + Vite, served via Nginx |
| Database | PostgreSQL (external, NON-dockerized) — primary + read replica |
| Cache | Redis (Memorystore in prod) |
| Auth internal | LDAP -> JWT |
| Auth vendor | Local credentials (CMS DB) -> JWT |
| Email | Company SMTP relay |
| Container | Docker + docker-compose (local) |
| Cloud | GCP (Sec 10) |

> Frontend: Dockerized. Backend: Dockerized or local. Database: NOT dockerized.

### Detailed libraries & versions (from steering `tech.md`)

**Frontend**
| Layer | Technology | Version | Purpose |
|-------|-----------|---------|---------|
| Framework | React | 19 | UI components |
| Language | TypeScript | 5.x | Type safety |
| Build | Vite | 6 | Dev server + production build |
| Router | TanStack Router | latest | Type-safe file-based routing |
| Server state | TanStack Query | v5 | Caching, optimistic updates, background refetch |
| Tables | TanStack Table | v8 | Headless table for DSR, orders, invoices, recaps |
| Forms | React Hook Form + Zod | latest | Complex multi-field forms |
| Styling | Tailwind CSS | 4 | OKLCH-native, maps to design tokens |
| PDF preview | @react-pdf/renderer | latest | Client-side PDF preview |
| Icons | Lucide React | latest | Consistent icon set |

**Backend**
| Layer | Technology | Version | Purpose |
|-------|-----------|---------|---------|
| Language | Go | 1.23+ | Backend runtime |
| HTTP framework | Chi | v5 | Lightweight router, middleware |
| DB access | sqlc | latest | SQL-first, type-safe generated Go |
| Migrations | golang-migrate | latest | Versioned schema migrations |
| Validation | go-playground/validator | v10 | Struct tag validation |
| Auth | Custom JWT + RBAC middleware | — | Multi-role auth, D-3 approval hierarchy |
| PDF generation | Typst CLI | latest | Template-based PDF |
| Excel | excelize | latest | DSR import, CIT/CPC recap, schedule export |
| Queue | asynq | latest | Redis-backed async jobs |
| File storage | MinIO Go SDK | latest | S3-compatible uploads/downloads |
| Logging | slog (stdlib) | — | Structured logging |
| Config | envconfig or viper | latest | Env-based config |
| HTTP client | net/http (stdlib) | — | Future integration calls |

**Infrastructure (Dockerized)**
| Service | Image | Purpose |
|---------|-------|---------|
| Backend API | Custom (multi-stage Go build) | ~15-25MB final image |
| Worker | Same Go binary, different entrypoint | asynq worker |
| Frontend | nginx:alpine + static build | Serves Vite output |
| Redis | redis:7-alpine | Queue + session cache |
| MinIO | minio/minio | S3-compatible file storage |
| Reverse proxy | caddy:alpine | TLS, route /api->backend, /->frontend |
| Typst | Custom (typst binary in worker image) | PDF rendering |

**Dev Tooling**
| Tool | Purpose |
|------|---------|
| pnpm | Frontend package manager |
| Biome | Frontend lint + format |
| golangci-lint | Go linting (staticcheck, govet, errcheck) |
| air | Go hot-reload in dev |
| Vitest | Frontend unit/integration tests |
| Playwright | E2E tests |
| Docker Compose | Local dev orchestration |
| Taskfile (go-task) | Cross-platform task runner (replaces Makefile) |

**Build commands**
```bash
# Frontend
pnpm install && pnpm dev && pnpm build && pnpm test && pnpm lint && pnpm format
# Backend
go mod tidy && air && go build -o bin/api cmd/api/main.go && go build -o bin/worker cmd/worker/main.go
go test ./... && golangci-lint run
# Database
migrate -path migrations -database $DATABASE_URL up
sqlc generate
# Docker
docker compose up -d --build
# Full quality gate
task check
```

**Architecture principles (from `tech.md`)**
1. Clean separation: frontend knows nothing about DB; backend exposes REST only.
2. SQL-first: use sqlc — real SQL, type-safe Go. No ORM magic.
3. Async by default: PDF gen, projection, reconciliation batches go through asynq; API returns job ID.
4. Feature-based organization (forecasting, invoice, cash-count), not by technical layer.
5. Single binary deploy per entrypoint.
6. Shared nothing between frontend/backend; API contract via OpenAPI spec, typed client generated.
7. Excel -> DB, not file storage: uploads parsed into structured DB rows on upload; no original files stored.
8. Sync parse for small files (<1000 rows) in API handler; no queue for Excel processing.

* * *

## 3\. Architecture, Modules & Data Map
### Backend layout — Go workspace, three modules

Repo root is a Go workspace (`go.work`) linking three sibling modules. `pkg/` is shared infra with **no** dependency on either backend; `backend/` and `backend-cit/` each depend on `pkg/` only — never on each other (acyclic, compiler-enforced).

```
CMS2/
  go.work                    # links backend/, backend-cit/, pkg/
  pkg/                       # shared: auth, middleware, config, response
    auth/                    # JWT TokenService, blacklist, Provider/UserRepository interfaces
    middleware/              # RequireAuth, RequireRoles, rate limiter
    config/                  # env config loader, Load(defaultPort)
    response/                # {success,data} envelope (adopted by backend-cit; NOT by backend — see below)
  backend/                   # ATM backend — own go.mod, port 8080
    cmd/api/main.go          # transactional server entrypoint
    cmd/batch/main.go        # EOD runner entrypoint
    internal/<module>/       # ATM-specific: auth (login service), handler, service, repository
    migrations/              # sole owner of ALL DB migrations (CIT tables included)
  backend-cit/               # CIT backend — own go.mod, port 8081
    cmd/api/main.go          # health check + RequireAuth-protected route group; validates tokens, issues none
    internal/<module>/       # CIT-specific: cit, journal, dsr, reconciliation, integration, handler, service, repository
```

**ATM's `internal/handler` keeps its existing flat JSON response shape** (e.g. `{"access_token":...}`) for wire compatibility with the existing frontends (`CompanyPortal-Vite`/`VendorPortal-Vite`) — it does NOT use `pkg/response`'s envelope. New CIT endpoints in `backend-cit` use `pkg/response`.
### Frontends (two separate SPAs)
```
frontend/CompanyPortal-Vite/   # internal app, LDAP login
frontend/VendorPortal-Vite/  # vendor portal, local login
```
### Modules (create ONLY these unless told)
**Platform Core**: `internal/auth` (LDAP + local, JWT, /me) · `internal/user` · `internal/audit` · `internal/approval` (maker-checker) · `internal/document` · `internal/notification` (in-app + SMTP) · `internal/export` (CSV/XLSX/PDF)
**Master Data**: `internal/vendor` · `internal/vendorpic` · `internal/vault` · `internal/location` · `internal/atm` · `internal/assignment`
**ATM Operations**: `internal/dsr` · `internal/replenishment` · `internal/forecast` (H+2) · `internal/cashcount` (vault + selective machine cash count — BA/checklist/photo/e-sign/3-way reconciliation; tables not yet approved, see Sec 3 DB map note)
**Finance**: `internal/invoice` (upload + validate + approve — NO payment execution) · `internal/reconciliation`
**CIT**: `internal/cit` · `internal/journal` · CIT reconciliation (order vs DSR vs journal)
**Integration**: `internal/corebanking` (escrow batch file ingest + parse -> feeds reconciliation)

### DB logical groups (canonical table names)
*   **Auth**: `roles`, `users` (add `auth_source` = ldap|local; password hash only for local)
*   **Core**: `audit_logs`, `approval_requests`, `documents`, `notifications`, `import_jobs`, `export_jobs`
*   **Master**: `vendors`, `vendor_pics`, `vendor_vaults`, `locations`, `atms`, `vendor_assignments`
*   **ATM**: `atm_dsr_uploads`, `atm_dsr_rows`, `replenishment_instructions`, `forecast_runs`, `forecast_results` — *(proposed, NOT yet approved — see Sec 3a)* `cash_count_schedules`, `cash_count_evidences`
*   **Finance**: `invoice_uploads`, `invoice_items`, `invoice_reconciliation_results`
*   **CIT**: `cit_orders`, `cit_handover_evidences`, `cit_journals`, `cit_dsr_uploads`, `cit_reconciliation_results`
*   **Integration**: `escrow_batch_files`, `escrow_batch_rows`, `escrow_reconciliation_results`
> Need a new table/column? Propose here FIRST, get approval, then migrate.

* * *

## 3a. Business Rules & Requirements (from URS v0.3 Rev1, `archives/UR New Template v0.3...docx`)
> Source-of-truth requirements doc. Anything below not yet reflected in code/schema is a **spec**, not an implemented behavior — check code before assuming it's live.

*   **Order ATM formula** (daily forecasting/replenishment, `cmd/api` — distinct from the EOD `Final Realisasi` formula in Sec 14, which is a different calc for a different job):
    `Order ATM = (Saldo DSR + Proyeksi Refund) − (Rekomendasi DMAA + Rencana Isi Hari-H)`
*   DSR daily upload deadline: **09:00**. Monthly report of late/missing DSR per vendor feeds FLM penalty basis.
*   Duplicate-order prevention: an ATM with an active order is not reissued a new one for the same period.
*   ATM in "problem" status (pending part, vandalism, etc.) is excluded from replenishment recommendations.
*   Replenishment result is classified into 4 categories (holiday-adjusted): on-schedule · early (1–2 days) · late (1–2 days) · not done (>2 days off or skipped).
*   **Cash count (vault, monthly)**: risk category from escrow (SIBS/MIS) balance analysis drives a random/non-patterned visit schedule. Assigned PIC gets email notification, can accept/reject (reject → reschedule or reassign). On accept, a surat tugas is issued. Berita Acara (BA) is filled digitally on-site, DSR column auto-fills from vendor's uploaded DSR, photo evidence attached, dual e-sign (vendor + bank PIC). Monthly recap = 3-way reconciliation: cash count vs. escrow (H-1, auto from MIS/SIBS) vs. proofing (manual input).
*   **Cash count selektif (machine-level)**: same flow as vault cash count, scoped to specific ATMs per supervision instruction.
*   **Invoice reconciliation**: vendor uploads invoice + supporting docs; CIMB Niaga internal team uploads ATM master data (active/terminated ATM, price, trip package, category VIP/Industri/Regular); system auto-reconciles; internal team can manually adjust against vendor disputes (sanggahan).
*   **NFR targets**: 24×7 availability outside planned maintenance · dashboard load ≤3s (p95) · DSR upload ≤30s/doc · journal-post initial response ≤5s with async status confirm ≤2min · 300 concurrent active users · horizontal scalability for vendor portal · Data Centers: Bintaro & NTT.
*   **DGCC / data privacy**: this system is internal + vendor-operational, not customer-facing, so UU PDP/POJK 22/2023 items on customer personal-data collection are likely N/A — but vendor (FLM) data exchange involves a third party, so a Third-Party Risk Assessment (TPRA) and Data Processing Agreement should be tracked as a compliance item, not assumed done.

* * *

## 4\. AI Collaboration Rules (the leash)
1. Plan first for non-trivial work: list files + steps, wait for OK.
2. Small diffs, one concern. No drive-by refactors.
3. Ask, don't guess on names/contracts/rules. No invented APIs/columns/env.
4. Respect stack & module map. No new deps without approval.
5. Read before edit. Match existing patterns in `pkg/` and `internal/`.
6. No hallucinated files/functions.
7. STOP & flag on: auth, money/journal, reconciliation, data migrations, deletes.
8. State tradeoffs in 1-2 lines.
9. Every feature ships with tests (Sec 8).

* * *

## 5\. Cross-Cutting Domain Rules (NON-NEGOTIABLE)
*   **Auth split**: Internal -> LDAP bind (no local password). Vendor -> local creds (bcrypt/argon2), separate frontend + login route. Both issue same JWT; role + auth\_source in claims. Vendors scoped to own assignments only.
*   **Maker-checker**: any create/update/delete on financial or master data -> `approval_requests` (pending -> approved/rejected). Effect applies only after approval. Maker != checker.
*   **Invoice**: CMS validates & approves only. Does NOT trigger/execute payment. Terminal = approved (handed off downstream).
*   **Escrow reconciliation**: source = batch file from Corebanking. Ingest -> parse into `escrow_batch_rows` -> reconcile vs CMS cash position -> store deltas. Idempotent per file hash.
*   **Audit**: every state change writes `audit_logs` (who, what, before/after, when, ip).
*   **RBAC**: enforce at middleware AND service layer.
*   **Idempotency**: all file ingests (DSR/invoice/escrow) idempotent per file hash -> `import_jobs`.
*   **Consistent response**: CIT endpoints (`backend-cit`) return the `pkg/response` envelope. ATM's `internal/handler` keeps its existing flat JSON shape (e.g. `{"access_token":...}`) for frontend wire compatibility — see Sec 3.

* * *

## 6\. Money, Data Integrity & DB Topology
*   Monetary/cash amounts as **numeric** (or integer minor units). NEVER float/double.
*   Currency: default IDR, always store code explicitly.
*   Timestamps: timestamptz, store UTC, display Asia/Jakarta.
*   **Read/Write split**: Primary pool -> all writes + transactional reads. Replica pool -> reporting, dashboards, cash monitoring, exports, heavy reads. Repos expose `db` (primary) vs `dbRead` (replica). Never write on replica. Read-after-write in same flow uses primary (beware replica lag).
*   Reconciliation results reproducible & explainable (store inputs + deltas).
*   Keep raw rows (`atm_dsr_rows`, `escrow_batch_rows`) separate from summaries.

* * *

## 7\. Glossary
| Term | Meaning |
| ---| --- |
| DSR | Daily Status Report: cash position per ATM/CRM per day. |
| CIT | Cash in Transit: physical pickup/delivery by vendor. |
| Replenishment | Instruction + execution to refill ATM cash. |
| Vault | Vendor cash vault holding bank cash. |
| Escrow reconciliation | Match CMS cash position vs Corebanking escrow batch file. |
| Forecast (H+2) | 2-day-ahead cash need -> shortage -> vendor cash need. |
| Cash count | Physical count: vault (vendor) + selective machine. |
| Maker-checker | Two-person approval control. |
| Invoice reconciliation | Validate invoice vs executed CIT/replenishment; approve only, no payment. |

* * *

## 8\. Testing (from `claude.md` Sec 8 + steering `testing.md`)
*   Go: `go test ./...`. Table-driven services. Mock repos for unit tests.
*   Integration: real Postgres for repo/API tests.
*   Frontend: component tests for critical flows on BOTH frontends (Vitest + Playwright E2E).
*   Coverage >= 80% on `internal/*`. No merge with failing/skipped tests.
*   Always test: LDAP vs local auth, maker-checker gates, RBAC denials, money math, reconciliation deltas, replica-vs-primary routing.
*   **TDD mandatory** for new features: write test (RED) -> implement (GREEN) -> refactor (IMPROVE) -> verify 80%+ coverage.
*   Test types required: Unit (functions/utils/components), Integration (API/DB), E2E (critical user flows).
*   Troubleshoot failures: check test isolation, verify mocks, fix implementation not tests (unless tests are wrong).

* * *

## 9\. Env, Config & Docker
```
APP_ENV=development
PORT=8080
DATABASE_URL=postgres://user:pass@primary-host:5432/cms
DATABASE_REPLICA_URL=postgres://user:pass@replica-host:5432/cms
REDIS_URL=redis://localhost:6379
JWT_SECRET=change_me
LDAP_URL=ldap://ldap.company.local:389
LDAP_BASE_DN=dc=company,dc=local
LDAP_BIND_DN=
LDAP_BIND_PASSWORD=
SMTP_HOST=smtp.company.local
SMTP_PORT=587
SMTP_USER=
SMTP_PASSWORD=
SMTP_FROM=cms-noreply@company.co.id
GCS_BUCKET=
LOG_LEVEL=info
```
*   Never commit `.env`. Never log secrets/JWTs/LDAP/SMTP creds.
*   docker-compose (local, root `docker-compose.yml`): backend + backend-cit + redis. Postgres external. Frontends have their own Dockerfiles/compose, not yet wired into this file.
*   Backend Dockerfile: multi-stage, distroless/alpine, non-root, HEALTHCHECK on /health. `backend/Dockerfile` and `backend-cit/Dockerfile` both build from repo root (context `.`) to resolve `pkg/` via `go.work`.
*   Frontend Dockerfiles: Vite build -> Nginx serve dist (one image per frontend).
*   .dockerignore: node\_modules, dist, .env, .git.

* * *

## 10\. GCP Deployment (0 -> Deployed)
| # | Product | Role |
| ---| ---| --- |
| 1 | Compute Engine | Frontend (internal + vendor) |
| 2 | Compute Engine | Backend, currently ONE VM running both `backend/` (ATM, port 8080) and `backend-cit/` (CIT, port 8081) as separate containers — see split-VM plan below |
| 3 | CloudSQL Postgres | Primary (write) |
| 4 | CloudSQL Postgres | Read replica (reporting/dashboard) |
| 5 | Cloud Storage | Uploads/exports/escrow batch files |
| 6 | Memorystore Redis | Cache/sessions |
| 7 | Artifact Registry | Docker images |
| 8 | Cloud Build | CI: build -> test -> scan -> push |
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

**Pipeline**: Cloud Build -> lint + go test + vite build (x2) -> build images (`backend/Dockerfile`, `backend-cit/Dockerfile`, separately) -> push to Artifact Registry with immutable tag (git SHA, not latest) -> deploy to Compute Engine (one VM today, one VM per backend from 2028).
**Rollback**: keep previous image SHA deployable, per service.

* * *

## 11\. Definition of Done
- [ ] Matches requirement + module/table map (Sec 3)
- [ ] Correct auth path (LDAP internal / local vendor), scoped RBAC
- [ ] Maker-checker + audit\_log wired (Sec 5)
- [ ] Reads on replica, writes on primary (Sec 6)
- [ ] Money as numeric, timestamps timestamptz
- [ ] Tests passing incl. auth/RBAC/money cases
- [ ] No secrets/config hardcoded; .env.example updated
- [ ] Uses pkg/response envelope (backend-cit); ATM keeps flat JSON for frontend wire compat (Sec 3)
- [ ] Builds cleanly in Docker

* * *

## 12\. Resolved Decisions
*   Escrow integration: batch file -> `internal/corebanking` ingest -> reconciliation.
*   Invoice payment: validate & approve only. No payment execution.
*   Frontends: two separate SPAs. Internal = LDAP; Vendor = local creds in CMS.
*   Email: company SMTP relay.
*   DB topology: primary for write/update, read replica for reporting/dashboard/cash monitoring.
*   Backend topology: `backend/` (ATM) and `backend-cit/` (CIT) are separate Go modules sharing `pkg/` (Sec 3). Deployed together on ONE Compute Engine VM today (two containers); planned to split into two separate VMs in 2028 (Sec 10) — the module split already makes that a zero-code-change deployment change when it happens.
*   Cash count (vault + selective machine) is confirmed in-scope per URS v0.3 Rev1, but its module/tables are a **proposal pending approval** (Sec 3a) — not yet part of the approved module/table map in Sec 3 until confirmed.

* * *

## 13\. UI/UX & Brand (from `claude.md` Sec 13 + steering `ui_design.md`)

**Brand anchor**: CIMB Niaga Red — `#E4142A` → `oklch(56% 0.223 27)`. Use OKLCH for all colors; build shade scales by holding chroma+hue constant and varying lightness. Never `#000`/`#fff`; tint neutrals slightly toward the brand hue. **Light mode only** (no dark mode, no toggle).

**Two themes, one per frontend:**
*   **Internal app** (`frontend/CompanyPortal-Vite`) -> "Merah Sirih". Warm off-white neutrals, red as a ≤10% accent. Optimized for data-dense screens.
*   **Vendor portal** (`frontend/VendorPortal-Vite`) -> "Merah Menyala". Bold: maroon-red top bar, full-red active sidebar.

**Design tokens (canonical, from `ui_design.md`):**
*   Brand Red scale (hue 29): `--red-50` `oklch(0.965 0.018 29)` … `--red-500` `oklch(0.552 0.205 29)` (Primary/CIMB Red/CTA) … `--red-900` `oklch(0.250 0.082 29)`.
*   Neutrals (tinted hue 29): `--n-0` `oklch(0.992 0.003 29)` (cards/inputs) … `--n-900` `oklch(0.178 0.005 29)` (headings).
*   Semantic (far from brand hue): Success (155), Warning (78), Danger (12, rose — NOT brand red), Info (245).
*   **Hard rule**: Brand red = hue 29 (action/identity). Error red = hue 12 rose (destructive only). Never same red for both. Error NEVER signalled by color alone — always icon + text.
*   Typography: system font stack, one family, hierarchy via scale+weight. `tabular-nums` on EVERY amount/metric/table; money monospace, right-aligned.
*   Spacing: 4pt scale (`--space-1` 4px … `--space-18` 72px). NO `--space-5/7/9` (resolves to 0). Radius sm 4 / md 6 / lg 10. Subtle brand-tinted shadows.
*   Components: one primary per view; tables are primary interface (uppercase headers, amounts right/tabular/mono, status via badges not row color); focus ring = soft red halo.
*   **Bans**: no side-stripe accent borders >1px, no gradient text, no glassmorphism default, no hero-metric template, no identical repeated card grids, no em dashes in UI copy, no pure black/white, no color-only status.

* * *

## 14\. Backend Split: Transactional vs Batch (EOD)
**One codebase, two entrypoints.** Domain logic stays shared in `internal/*`.
*   `cmd/api` — transactional server. Always on, serves the frontends. Office hours active.
*   `cmd/batch` — End-of-Day runner. Cron at midnight, processes, then exits/idles.
Both run on the **same VM**; active windows don't overlap (no contention).

**What EOD does**: ingest from DSR (saldo akhir 00:00 per vault), Opti Cash forecast (Order H-1 & H), horizon H-2 (refund validation); compute Final Realisasi = rekomendasi DMAA − (saldo DSR + refund horizon H-2) per vendor; produce daily summaries read next working day.

> This EOD `Final Realisasi` formula is separate from the daily `Order ATM` formula (Sec 3a) — don't conflate the two, they answer different questions (backdated realisasi summary vs. same-day/H-2 order calc).

**Handoff rules (NON-NEGOTIABLE):**
*   EOD output written to **DB = source of truth**. Redis only cache, never store of record.
*   Every EOD run tracked in a run table (`forecast_runs` / an `eod_runs` table): `processing_date`, status (running/success/failed), started\_at, finished\_at, records\_processed, error.
*   Transactional reads a `processing_date` only after run marked **success**. Never partial/in-progress.
*   On completion emit domain event (`EODCompleted` / `SummaryReady`).
*   Batch ingests idempotent per file/`processing_date` (re-run safe).
*   **EOD Monitoring** (admin/app-support only): dashboard per-run status + email alert on failure OR not-complete before office hours.

* * *

## 15\. Project Structure (from steering `structure.md`)
```
CMS2/
├── frontend/                 # React + TypeScript + Vite (monorepo root)
│   ├── CompanyPortal-Vite/  # internal app, LDAP login
│   └── VendorPortal-Vite/    # vendor portal, local login
├── go.work                   # links backend/, backend-cit/, pkg/
├── pkg/                      # shared Go infra (auth, middleware, config, response)
│   ├── auth/
│   ├── middleware/
│   ├── config/
│   └── response/
├── backend/                  # ATM backend — own go.mod, port 8080
│   ├── cmd/
│   │   ├── api/              # HTTP server entrypoint (transactional)
│   │   ├── batch/           # EOD runner entrypoint
│   │   └── worker/          # asynq worker entrypoint
│   ├── internal/
│   │   ├── domain/           # Business entities, value objects
│   │   ├── service/          # Business logic
│   │   ├── handler/          # HTTP handlers (Chi routes) — flat JSON for frontend wire compat
│   │   ├── repository/       # DB access (sqlc + interfaces)
│   │   ├── middleware/       # Auth, logging, RBAC, rate-limit
│   │   ├── queue/            # asynq task definitions + handlers
│   │   └── pdf/              # Typst template rendering
│   ├── migrations/           # SQL migration files (sole owner of ALL DB migrations)
│   ├── queries/              # sqlc SQL query files
│   ├── templates/            # Typst PDF templates
│   └── go.mod / go.sum
├── backend-cit/              # CIT backend — own go.mod, port 8081
│   ├── cmd/api/              # health check + RequireAuth-protected routes (validates tokens)
│   ├── internal/             # cit, journal, dsr, reconciliation, integration, handler, service, repository
│   └── go.mod / go.sum
├── docker/                   # Dockerfiles (api, frontend, worker)
├── docker-compose.yml / docker-compose.prod.yml
├── Taskfile.yml
├── .env.example
├── documents/  archives/  .kiro/
```
*   Group by feature, not file type. Co-locate tests: `*_test.go` next to source; frontend tests in `__tests__/`.
*   Frontend monorepo: `frontend/` is orchestration root. Each sub-app self-contained (own Dockerfile, nginx.conf, package.json). Image naming: `{appname}-vite-fe`.

* * *

## 16\. Coding Style (from steering `coding-style.md`)
*   **Immutability (CRITICAL)**: ALWAYS create new objects, NEVER mutate existing ones. Return new copy with change.
*   **File organization**: many small files > few large. 200-400 lines typical, 800 max. Extract utilities. Organize by feature/domain.
*   **Error handling**: handle explicitly at every level; user-friendly messages in UI; detailed context in server logs; never silently swallow.
*   **Input validation**: validate at boundaries; schema-based (Zod / go-playground/validator); fail fast; never trust external data.
*   **Quality checklist**: readable & well-named; functions <50 lines; files <800 lines; nesting ≤4 levels; proper errors; no hardcoded values; no mutation.

* * *

## 17\. Go Patterns (from steering `golang-patterns.md`)
*   **Functional Options**: `type Option func(*T)` + `NewX(opts ...Option) *T`.
*   **Small Interfaces**: define where used, not where implemented.
*   **Dependency Injection**: constructor functions inject deps (e.g. `NewUserService(repo, logger)`).
*   **RateLimiter interface** in `internal/auth/` (consumer) not imported from middleware — for unit-testable stubs (Go structural typing).
*   DB constraint evolution: add NEW enum value (e.g. `local_dev`), don't relax existing prod constraints.

* * *

## 18\. Common Patterns (from steering `patterns.md`)
*   **Repository Pattern**: encapsulate data access behind interface (findAll/findById/create/update/delete). Business logic depends on abstraction; enables mock swapping.
*   **API Response Envelope**: consistent `{ success/status, data (nullable on error), error (nullable on success), meta (pagination) }` via `pkg/response`.
*   **Skeleton projects**: evaluate battle-tested skeletons via parallel agents before cloning as foundation.

* * *

## 19\. Security (from steering `security.md`)
**Mandatory checks before ANY commit:**
- [ ] No hardcoded secrets (keys/passwords/tokens)
- [ ] All user inputs validated
- [ ] SQL injection prevention (parameterized queries)
- [ ] XSS prevention (sanitized HTML)
- [ ] CSRF protection enabled
- [ ] Auth/authorization verified
- [ ] Rate limiting on all endpoints
- [ ] Error messages don't leak sensitive data

**Secret management**: never hardcode; use env vars or secret manager; validate required secrets at startup; rotate exposed secrets.
**Response protocol**: STOP -> use security review -> fix CRITICAL -> rotate secrets -> review codebase for similar issues.

* * *

## 20\. Git & Development Workflow (from steering `git-workflow.md` + `development-workflow.md`)
**Commit format**: `<type>: <description>` (types: feat, fix, refactor, docs, test, chore, perf, ci). Conventional commits.
**Feature workflow**: (1) Plan with implementation plan + phases; (2) TDD (RED->GREEN->IMPROVE, 80%+ cover); (3) Code review, fix CRITICAL/HIGH; (4) Commit + PR.
**PR**: analyze full commit history (not just latest); `git diff [base]...HEAD`; comprehensive summary + test plan; push `-u` for new branch.
> Note: Kiro-specific agent names (planner, tdd-guide, code-reviewer) map to OpenCode's general plan/test/review workflow — use OpenCode's plan mode and verification loops instead.

* * *

## 21\. Long Flow Process Guidance (from steering `how_to_handle_flowprocess.md`)
Long multi-step flows must NOT be one big synchronous request. Use **state machine + DB-backed orchestration + async workers/batch**:
*   Model flow as explicit states (draft -> uploaded -> validated -> pending_approval -> approved -> processing -> completed/failed/rejected); valid transitions only.
*   Separate command from processing: API validates input, saves record + initial state + audit\_log, creates approval/import/run, triggers background work, returns tracking status. Don't hold HTTP request until done.
*   **DB = source of truth** for progress; Redis only for signal/short-lived marker/cache/lock.
*   Three flow types: (A) Human workflow (maker-checker gate, effect after approval); (B) File-processing (one `import_jobs` per file/hash, idempotent, raw rows separated from summary); (C) Batch/EOD (one run per `processing_date`, status running/success/failed, read only success).
*   Orchestrator service (not controller) orders steps; domain modules hold business rules; repo accesses DB.
*   **Every step idempotent**: unique key / file hash / processing date; guard "already published".
*   **Failure handling**: classify retryable (network/SMTP/DB) vs business (invalid/rejected/mismatch) vs terminal (corrupt/ schema/state). Prefer marking failed + partial safe results + manual rerun over complex cross-step rollback (esp. for money/reconciliation).
*   Separate **business status** (draft/approved) from **technical status** (queued/processing/failed) — never one column.
*   **Observability**: monitoring pages for import jobs, reconciliation runs, forecast runs, EOD runs (current step, timings, who triggered, totals, error, links to audit/import/approval).
*   **UX**: submit -> status badge -> poll status endpoint -> show last step + error; show approval waiters clearly. No 2-min spinner timeouts.
*   Read-after-write uses primary; replica only for reporting/summary.

* * *

## 22\. Lessons Learned — Key Pitfalls (from steering `lessons-learned.md`)
**Frontend**
*   Tailwind CSS 4: declare tokens in BOTH `:root` AND `@theme { }` block (`--color-*`/`--spacing-*`); needs `@tailwindcss/vite` plugin in `vite.config.ts` or app renders unstyled with no error.
*   Only use defined spacing tokens (`--space-1/2/3/4/6/8/12/18`); `--space-5/7/9` resolve to 0px.
*   TanStack Router `beforeLoad` is synchronous — pair with component-level auth check; restart dev server + hard refresh after core auth changes.
*   Auth `initialize()` must NOT call `logout()` on failure (infinite redirect loop); set state instead.
*   `ColumnDef<T, any>[]` for `createColumnHelper` output compatibility.
*   Vitest: exclude `e2e/**`; mock `ResponsiveContainer` + polyfill `ResizeObserver` for Recharts; enable `resolveJsonModule`.
*   FilterSelect tests: find `<select>` via DOM traversal from label, not `getByRole`.
*   Page-enter animation: use `key={location.pathname}` to re-trigger on route change.
*   Cross-portal cookie leakage (localhost shared): guard `role` in BOTH `initialize()` and `refreshToken()`; type API response fields as `string` (not literal) so bundler doesn't dead-code-eliminate runtime guards.
*   Subagent task execution may leave child components unimported — verify integration after a wave of tasks.
*   Stub mode: direct `fetch()` hits network; must check `apiConfig.mode === "stub"` and call `handleStubRequest`. Force real mode in fetch-based tests via `vi.mock("@/lib/api/config")`.

**Backend / Build**
*   pnpm 11: COPY `pnpm-workspace.yaml` before `pnpm install` (use `allowBuilds`, not `package.json#pnpm`); `npm install -g pnpm@<exact>` in Docker; regenerate lockfile on major upgrade.
*   Docker production build: `pnpm tsc -b tsconfig.app.json && pnpm vite build` (not `tsc -b` which compiles tests).
*   Separate docker-compose projects: nginx `proxy_pass` must use `resolver 127.0.0.11; set $backend_upstream http://host.docker.internal:8080;` (not `backend:8080`).
*   Excel -> DB rows on upload; parse synchronously for <1000 rows.

**Data**
*   `machine_type` differs between `itm_cashpos` (denomination: ATM100K/ATM50K/CRM) and `atms` (functional: ATM/CRM/CDM) — join on `terminal_id`, never `machine_type`.
*   DB constraint evolution: add new enum value, don't relax prod constraints.

**Python (scheduler scripts)**
*   Windows + `psycopg2` crashes on non-UTF8 `%APPDATA%` path. Use `psycopg[binary]>=3.2.10` + keyword params + override `PGSERVICEFILE/PGPASSFILE/NUL` and `PGSYSCONFDIR=.` before connect. Use `python-dotenv` + `.env`.

* * *

## 23\. Performance / Build Notes (from steering `performance.md`)
*   Use a lighter/faster model for trivial single-file edits, doc updates, and simple fixes; reserve the strongest reasoning model for complex architecture, money/reconciliation logic, and multi-file refactors.
*   Avoid the last ~20% of context window for large refactors / multi-file features / complex debugging.
*   Build fails: analyze error messages, fix incrementally, verify after each fix.

* * *

> All steering files live in `C:\Users\RB Yudha Rangga\OneDrive\Documents\Development\CMS2\.kiro`. This `opencode.md` is the OpenCode equivalent of `.claude/claude.md` + all `.kiro/steering/*.md`, consolidated.
