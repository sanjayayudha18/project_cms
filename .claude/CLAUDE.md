# PROJECT\_CONTEXT.md — Cash Management System (CMS) for ATM & CIT
> **AI: READ THIS FIRST, EVERY SESSION.** Single source of truth. If your work conflicts with this, STOP and ask. Never invent endpoints, tables, columns, env vars, or modules.
> **For simple explanations of CMS concepts**, see [eli5.md](./eli5.md) — use when explaining to teammates or stakeholders.
> **What's built vs pending**: [development-progress.md](./development-progress.md) — update it when a spec finishes or a feature changes status.
* * *
## 0\. Golden Rules
1. Stack is FIXED (Sec 2). No new libs/frameworks without approval.
2. Follow module & table map (Sec 3). No modules/tables not listed unless asked.
3. Every state-changing action respects maker-checker + writes audit\_log (Sec 5).
4. Money = numeric / integer minor units, never float (Sec 6).
5. Plan before non-trivial work. Small diffs. One concern per change.
6. DB topology: write/update -> primary; read/report/dashboard -> read replica (Sec 6).
7. Two frontends: internal (LDAP) + vendor portal (local auth). Keep separate (Sec 5).
8. Two backend entrypoints, one codebase: `cmd/api` (transactional, office hours) + EOD pipeline in Python (`backend_python/`, file-driven; scheduler scans/retries 05:00–09:00 WIB). Shared `internal/*` for Go (Sec 14).
9. EOD batch output is the source of truth for the transactional module. Write to DB, signal readiness before transactional reads (Sec 14).
10. Manual browser check/testing of a change (clicking through the UI, verifying a flow end-to-end) is done by the **user**, not the AI. Ship the code + automated checks (unit/integration tests, typecheck, build); state plainly that manual browser verification is outstanding instead of attempting or claiming it.

* * *
## 0a. Development Stage & DB Schema Policy
**Current phase**: Early development (MVP/greenfield). Schema is **MUTABLE** — database changes (new tables, columns, migrations) are expected and encouraged as requirements evolve. No need to wait for formal approval on schema changes during this stage, though structural decisions that affect the module/table map (Sec 3) should still be flagged in CLAUDE.md after they are applied.

* * *
## 1\. Overview
*   **Name**: CROWN THE Cash Management System
*   **Goal**: E2E ATM cash management: vendor replenishment, daily DSR reporting, forecasting & scheduling, cash count (vault + selective machine), reconciliation vs Corebanking escrow, vendor invoice validation & approval.
*   **Stage**: Greenfield, built with AI — **not vibe-coded**. All development goes through the AI-Native SDLC (Sec 4a): intent → spec → plan → tests → review → incidents, each stage gated by a human (small fixes per Sec 4a rule 3 excepted).
*   **Roles**: Admin, Operator, Manager (approver), Vendor, Branch/Internal User.

* * *
## 1a. graphify (codebase navigation)

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.

Rules:
- For codebase questions, first run `graphify query "<question>"` when graphify-out/graph.json exists. Use `graphify path "<A>" "<B>"` for relationships and `graphify explain "<concept>"` for focused concepts. These return a scoped subgraph, usually much smaller than GRAPH_REPORT.md or raw grep output.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).


## 2\. Tech Stack (SOURCE OF TRUTH)

| Layer | Choice |
| ---| --- |
| Backend | Go + Chi (v5) |
| DB pool | pgx v5 / pgxpool — primary + replica pools created in `backend/cmd/api/main.go` (no shared `pkg/database`) |
| SQL codegen | sqlc **v1.31.1** (pinned) — `backend/sqlc.yaml`, queries in `backend/queries/` |
| EOD / ETL | Python — FastAPI + pandas + psycopg (`backend_python/`, Sec 14) |
| Frontend | React + Vite, served via Nginx |
| Database | PostgreSQL (external, NON-dockerized) — primary + read replica |
| Cache | Redis (Memorystore in prod) |
| Auth internal | LDAP -> JWT |
| Auth vendor | Local credentials (CMS DB) -> JWT |
| Email | Company SMTP relay |
| Container | Docker + docker-compose (local) |
| Cloud | GCP (Sec 10) |

> Frontend: Dockerized. Backend: Dockerized or local. Database: NOT dockerized.
* * *
## 2a. Commands

```bash
# Go (run per module; go.work links them)
cd backend && go test ./...          # ATM backend (integration tests skip when DATABASE_URL is empty)
cd backend-cit && go test ./...
cd pkg && go test ./...
cd backend && go run ./cmd/api       # ATM API :8080  (backend-cit: go run ./cmd/api → :8081)

# sqlc (after editing backend/queries/*.sql)
cd backend && sqlc generate          # MUST be v1.31.1 — see note below

# Frontends (pnpm; same scripts in both apps)
pnpm --dir frontend/CompanyPortal-Vite run dev|build|test|lint     # dev :5173
pnpm --dir frontend/VendorPortal-Vite  run dev|build|test|lint     # dev :5174

# Python EOD (run from backend_python/ so `lib` imports resolve)
cd backend_python && python -m eod_retry_scheduler.run   # :8091
cd backend_python && python -m service_dsr_etl.run       # :8090
cd backend_python && pytest dsr itm/cashpos

# Migrations: plain numbered SQL in backend/migrations/ (no migrate tool), applied by hand to dev
psql "postgres://…@localhost:5432/cms" -f backend/migrations/0NN_<name>.sql
```

*   **sqlc inflector quirk**: v1.31.1 generates `UserLeafe` for the `user_leaves` table. After every `sqlc generate`, hand-fix `UserLeafe` → `UserLeave` in `backend/internal/db/` before building.
*   **Dev DB host**: `backend/.env` uses `host.docker.internal`, which only resolves inside a container — from a host shell use `localhost:5432`.
*   Browser verification is the user's job (Golden Rule #10); `dev` commands are for the user or the preview tool, not for claiming a UI check.
* * *
## 3\. Architecture, Modules & Data Map
### Backend layout — Go workspace, three modules

Repo root is a Go workspace (`go.work`) linking three sibling modules. `pkg/` is shared infra with **no** dependency on either backend; `backend/` and `backend-cit/` each depend on `pkg/` only — never on each other (acyclic, compiler-enforced).

```plain
CMS2/
  go.work                    # links backend/, backend-cit/, pkg/
  pkg/                       # shared: auth, middleware, config, response
    auth/                    # JWT TokenService, blacklist, Provider/UserRepository interfaces
    middleware/              # RequireAuth, RequireRoles, rate limiter
    config/                  # env config loader, Load(defaultPort)
    response/                # {success,data} envelope (adopted by backend-cit; NOT by backend — see below)
  backend/                   # ATM backend — own go.mod, port 8080
    cmd/api/main.go          # bootstrap + route registration
    internal/<module>/       # ATM-specific: auth (login service), handler, service, repository
    migrations/              # sole owner of ALL DB migrations (CIT tables included)
  backend-cit/               # CIT backend — own go.mod, port 8081
    cmd/api/main.go          # health check + RequireAuth-protected route group; validates tokens, issues none
    internal/<module>/       # CIT-specific: cit, journal, dsr, reconciliation, integration, handler, service, repository
```

**ATM's `internal/handler` keeps its existing flat JSON response shape** (e.g. `{"access_token":...}`) for wire compatibility with `CompanyPortal-Vite`/`VendorPortal-Vite` — it does NOT use `pkg/response`'s envelope. New CIT endpoints in `backend-cit` should use `pkg/response`.

### Frontends (two separate SPAs)

```plain
frontend/CompanyPortal-Vite/ # internal app, LDAP login
frontend/VendorPortal-Vite/  # vendor portal, local login
```

### Actual code layout (ATM `backend/internal/`)
Layered, not one package per domain: `handler/` (HTTP) → `service/` (business rules, maker-checker `Submit`, appliers) → `repository/` (read-only repos over sqlc) → `db/` (sqlc-generated, do not hand-edit except the known inflector fix in Sec 2a). Cross-cutting packages: `approval/`, `audit/`, `auth/`, `rolemgmt/`. New ATM code follows this layering; the domain list below is the **logical scope** (what may exist), not a package list.

### Modules / domains (create ONLY these unless told)
**Platform Core**: `internal/auth` (LDAP + local, JWT, /me) · `internal/user` · `internal/audit` · `internal/approval` (maker-checker) · `internal/document` · `internal/notification` (in-app + SMTP) · `internal/export` (CSV/XLSX/PDF)

**Master Data**: `internal/vendor` · `internal/vendorpic` · `internal/vault` · `internal/location` · `internal/atm` · `internal/assignment`

**ATM Operations**: `internal/dsr` · `internal/replenishment` · `internal/forecast` (H+2) · `internal/cashcount` (vault + selective machine cash count — BA/checklist/photo/e-sign/3-way reconciliation; tables not yet approved, see Sec 3 DB map note)

**Finance**: `internal/invoice` (upload + validate + approve — NO payment execution) · `internal/reconciliation`

**CIT**: `internal/cit` · `internal/journal` · CIT reconciliation (order vs DSR vs journal)

**Integration**: `internal/corebanking` (escrow **batch file** ingest + parse -> feeds reconciliation)
### DB logical groups (canonical table names)
*   **Auth**: `roles`, `users` (add `auth_source` = ldap|local; password hash only for local)
*   **Core**: `audit_logs`, `approval_requests`, `master_data_change_requests` (staged master-data changes — see Sec 12), `master_data_import_batches` (CSV import files, one approval per batch, idempotent per SHA-256 — Sec 12), `documents`, `notifications`, `import_jobs`, `export_jobs`
*   **Auth/Core — Role Management** (approved, `.kiro/specs/role-management`; originally migration `040_role_permissions.sql`, now part of `001_baseline_schema.sql` + seeded in `002_baseline_seed.sql` — applied):
    *   Columns + constraints: see [docs/data-map.md](./docs/data-map.md#role-management-tables-menu_features-role_permissions). `role_permissions` row presence = grant; new roles start with zero access.
*   **Master**: `vendors` (incl. `legal_name`, `npwp`, `kind` = FLM_VENDOR|INTERNAL — see vendor-pricing note below), `vendor_branches`, `branch_coverage_areas`, `vendor_vaults` (category ATM/CASH/ATM_CASH), `vendor_pics`, `vendor_packages_branch` (renamed from `vendor_packages`, migration `011`; as of migration `016` a branch-scoped special price table — "harga khusus cabang" — see Sec 12), `cimb_branches`, `package_frequencies`, `vendor_package_prices` (vendor-wide PT/branch/ATM-override price tree, unrelated to and unchanged by migration 016; identifier column split into `package`/`package_code` by migration `017` — see below), `locations`, `atms` (incl. `cimb_branch_id`, `price_machine_group`, `price_class` generated columns), `atm_vendor_packages` (= ATM assignments/kelolaan ATM: effective-dated, no overlapping active period per ATM; FK to `vendor_packages_branch`). **There is no `vendor_assignments` table** — earlier drafts of this map used that name; the real names above follow the DB (decision D4, Sec 12). Most are soft-disabled (`is_active`/`deleted_at`), never hard-deleted, **except `branch_coverage_areas`** (pure link table, no soft-delete columns; removed by deleting the row, FK `ON DELETE CASCADE` from `vendor_branches`) **and `vendor_packages_branch`/`vendor_package_prices`** (effective-dated history since migration 016/009 respectively — "disable" closes `effective_end_date`, never a row toggle, never hard-deleted either).
    *   Detailed per-table notes + migration history (pricing 009/010, code split 017, regions 018, `branch_coverage_areas` 007, vault category 008): [docs/data-map.md](./docs/data-map.md). Rules that always apply:
    *   `ROH` (`vendors.kind='INTERNAL'`) is an internal CIMB-branch unit, not an FLM vendor, never billed; its branches live in `cimb_branches`.
    *   Two separate price tables — don't conflate: `vendor_package_prices` = vendor-wide PT/branch/ATM-override tree; `vendor_packages_branch` = a branch's own special price. Both effective-dated; "disable" closes `effective_end_date`.
    *   `vendor_package_prices.package` = label ("PAKET 3", joins `package_frequencies.package_code`); `vendor_package_prices.package_code` = unique per-row code, **server-generated at apply time, never client-supplied**.
    *   `atms.price_machine_group`/`price_class` are generated columns; unmapped → NULL so pricing lookups fail hard, never mis-price.
    *   `vendor_vaults.type` is always mirrored from `category` — never set it independently.
    *   `branch_coverage_areas` is read-only today; if it gets a screen, use the master-data maker-checker pattern (Sec 12).
    *   `regions` has soft-delete (018); disable is blocked while an active `locations` row references it.
    *   Known gaps: handler `integration_test.go` only runs migrations 001+002; ATM assignment doesn't validate `machine_group`/`price_class` match (accepted 2026-09-23).
*   **ATM**: `atm_dsr_uploads`, `atm_dsr_rows`, `replenishment_instructions`, `forecast_runs`, `forecast_results`, `cash_count_schedules`, `cash_count_evidences` (table names approved 2026-09-25; columns defined by the Phase 5 spec)
*   **Finance**: `invoice_uploads`, `invoice_items`, `invoice_reconciliation_results`
*   **CIT**: `cit_orders`, `cit_handover_evidences`, `cit_journals`, `cit_dsr_uploads`, `cit_reconciliation_results`
*   **Integration**: `escrow_batch_files`, `escrow_batch_rows`, `escrow_reconciliation_results`
> Need a new table/column? Propose here FIRST, get approval, then migrate.
* * *
## 3a. Business Rules & Requirements
> Full URS summary (FNC 001–003, forecast uploads, pemenuhan/pengambilan dana, reports, dashboard, cash count, invoice recon, NFR, DGCC): [docs/requirements.md](./docs/requirements.md). Anything there not in code/schema is a **spec**, not live behaviour — check code first. Flow diagrams: `.claude/feature-flows/<feature>/feature-flow.md`.

Always-on rules (money/recon — never re-derive):
*   **Order ATM** = `(Saldo DSR + Proyeksi Refund) − (Rekomendasi DMAA + Rencana Isi Hari-H)`; DSR missing → from DMAA recommendation alone (confirmed 2026-09-25). Distinct from the EOD `Final Realisasi` formula (Sec 14).
*   DSR upload deadline **09:00**; late/missing DSR feeds the FLM penalty report.
*   No extra order for an ATM with an emergency/adhoc order issued up to H-1 or active on H0; ATMs in "problem" status are excluded from recommendations.
*   Key NFRs: dashboard ≤3s p95 · DSR upload ≤30s/doc · 300 concurrent users · office hours 07:00–20:00, uptime 24×7.
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
## 4a. AI-Native SDLC (per-feature artifact chain)
> Ref: Claude Academy "AI-Native SDLC Playbook". Full how-to + templates: [`.claude/sdlc/README.md`](./sdlc/README.md). Reference example: `.claude/sdlc/vendor-upload-dsr/`.

Every non-trivial feature lives in `.claude/sdlc/<feature>/` and moves through 6 stages. Each stage produces **one committed file** that the next stage reads. Together those files are the audit trail.

| Stage | Artifact | Human gate (AI never self-approves) | AI does |
| ---| ---| ---| --- |
| 1 Plan | `intent.md` | Product owner accepts | Capture problem/outcome/constraints in the originator's words; list open questions, don't answer them |
| 2 Design | `spec.md` | Product owner accepts | Requirements (FR/NFR), API, data model, out-of-scope; flag Sec 4 #7 items (auth/money/recon/migration/delete) |
| 3 Build | `plan.md` | Engineer accepts plan | Plan mode first: files, order, risks, proof → then small diffs; keep `plan.md` in sync with the diff |
| 4 Test | `tests.md` | Build + tests green | Feedback loop: run `go test ./...` / `npm test` / typecheck / build, fix until green; trace tests → spec reqs |
| 5 Deploy | `review.md` | Code owner merges | Review passes (bugs / security / compliance), Important vs nit, Sec 11 DoD checklist |
| 6 Maintain | `incidents.md` | On-call | Log incidents, add a regression test/eval per incident; a breach → new `intent.md` (loop closes) |

**Rules for the AI:**
1. Before coding a feature, check `.claude/sdlc/<feature>/`. Missing → start at `intent.md`. Never skip a stage or start the next one before the previous artifact is accepted by a human.
2. Work only from the latest accepted artifact. If code and artifact disagree, STOP and ask. Don't silently fix either one.
3. Small fixes/typos/single-file bugs are exempt, so there's no artifact chain for them. Anything touching Sec 4 #7 or the schema is never exempt.
4. Manual browser verification is the user's job (Golden Rule #10). Record it in `tests.md` as "outstanding" and never mark it done.
5. When a stage finishes, update the feature row in `.claude/sdlc/README.md` → "Current features" and in `development-progress.md`.
6. Lessons from incidents/reviews go to `.claude/lessons-learned.md`. A durable rule goes into this file, as a versioned change, reviewed like code.
7. Legacy `.kiro/specs/*` (requirements/design/tasks) stay valid for features already started there. New features use `.claude/sdlc/`.

* * *
## 5\. Cross-Cutting Domain Rules (NON-NEGOTIABLE)
*   **Auth split**: Internal -> LDAP bind (no local password). Vendor -> local creds (bcrypt/argon2), separate frontend + login route. Both issue same JWT; role + auth\_source in claims. Vendors scoped to own assignments only.
*   **Maker-checker**: any create/update/delete on financial or master data -> `approval_requests` (pending -> approved/rejected). Effect applies only after approval. Maker != checker.
*   **Invoice**: CMS validates & approves only. Does NOT trigger/execute payment. Terminal = approved (handed off downstream).
*   **Escrow reconciliation**: source = batch file from Corebanking. Ingest -> parse into `escrow_batch_rows` -> reconcile vs CMS cash position -> store deltas. Idempotent per file hash.
*   **Audit**: every state change writes `audit_logs` (who, what, before/after, when, ip).
*   **RBAC**: enforce at middleware AND service layer.
*   **Idempotency**: all file ingests (DSR/invoice/escrow) idempotent per file hash -> `import_jobs`.
*   **Consistent response**: `backend-cit` endpoints return the `pkg/response` envelope. Exception: ATM `backend/internal/handler` keeps its existing flat JSON shape for frontend wire compatibility (Sec 3) — match the neighbouring handlers there, don't mix shapes.

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
## 8\. Testing
*   Go: `go test ./...`. Table-driven services. Mock repos for unit tests.
*   Integration: real Postgres for repo/API tests.
*   Frontend: component tests for critical flows on BOTH frontends.
*   Coverage >= 80% on `internal/*`. No merge with failing/skipped tests.
*   Always test: LDAP vs local auth, maker-checker gates, RBAC denials, money math, reconciliation deltas, replica-vs-primary routing.

* * *
## 9\. Env, Config & Docker

```plain
APP_ENV=development
PORT=8080
DATABASE_URL=postgres://user:pass@primary-host:5432/cms
DATABASE_REPLICA_URL=postgres://user:pass@replica-host:5432/cms
REDIS_URL=redis://localhost:6379
JWT_SECRET=change_me
SESSION_MAX_LIFETIME=1h
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
## 10\. GCP Deployment
> Product map (Compute Engine, CloudSQL primary + replica, GCS, Memorystore, Artifact Registry, Cloud Build, Secret Manager, DNS, Armor, Logging, Monitoring), split-VM plan, pipeline: [docs/deployment.md](./docs/deployment.md).
*   Today: ONE VM runs `backend` (8080) + `backend-cit` (8081) as separate containers. 2028: one VM per backend — zero code change.
*   Both backends must share the **same** CloudSQL, **same** Redis (JWT blacklist + rate limits) and **same** `JWT_SECRET`; only ATM issues tokens, CIT only validates.
*   Images tagged by git SHA (never `latest`); keep the previous SHA deployable per service for rollback.

* * *
## 11\. Definition of Done
- [ ] Matches requirement + module/table map (Sec 3)
- [ ] Correct auth path (LDAP internal / local vendor), scoped RBAC
- [ ] Maker-checker + audit\_log wired (Sec 5)
- [ ] Reads on replica, writes on primary (Sec 6)
- [ ] Money as numeric, timestamps timestamptz
- [ ] Tests passing incl. auth/RBAC/money cases
- [ ] No secrets/config hardcoded; .env.example updated
- [ ] Response shape per Sec 5 (`pkg/response` envelope in `backend-cit`; flat JSON matching neighbours in ATM `backend`)
- [ ] Builds cleanly in Docker

* * *
## 12\. Resolved Decisions
*   Escrow integration: batch file -> `internal/corebanking` ingest -> reconciliation.
*   Invoice payment: validate & approve only. No payment execution.
*   Frontends: two separate SPAs. Internal = LDAP; Vendor = local creds in CMS.
*   Email: company SMTP relay.
*   DB topology: primary for write/update, read replica for reporting/dashboard/cash monitoring.
*   Backend topology: `backend/` (ATM) and `backend-cit/` (CIT) are separate Go modules sharing `pkg/` (Sec 3). Deployed together on ONE Compute Engine VM today (two containers); planned to split into two separate VMs in 2028 (Sec 10) — the module split already makes that a zero-code-change deployment change when it happens.
*   Cash count (vault + selective machine) is confirmed in-scope per URS v0.3 Rev1, and its table names (`cash_count_schedules`, `cash_count_evidences`) were **approved 2026-09-25** into Sec 3; column design (and any extra BA/checklist/e-sign tables) is decided in the Phase 5 spec.
*   **Master-data maker-checker (D1–D4)** — every create/update/disable/enable on `vendors`, `vendor_branches`, `vendor_vaults`, `vendor_pics`, `vendor_packages_branch`, `vendor_package_prices`, `atms`, `atm_vendor_packages` is staged: `MasterDataChangeService.Submit` → `master_data_change_requests` + `approval_requests` → endpoint returns **202**; the row changes only on final approval, via a per-entity `Applier` registered in `cmd/api/main.go`, in one tx with an audit entry. Stale → not applied; apply failure → 409. RBAC at route group (`ADMIN`/`ADMIN_PARAM`) **and** inside `Submit`. CSV import = one batch, one approval, all-or-nothing. Bulk format is CSV (stdlib) only for now. **New master-data entities must follow: queries → read-only repo → service `Submit` → `Applier` in `main.go` → handler 202.**
*   **`admin/users`** apply immediately (not maker-checker), guarded `APPACCESS`. Local-user create issues a temp password + `must_change_password=true`.
*   **Role Management (and planned Region Management) — documented deviation from Golden Rule #3**: apply immediately, but the `audit_logs` write is in the **same transaction** (audit failure rolls back) and `APPACCESS`/`ADMIN` is re-checked at route + service layer.
*   **Soft-delete only** for `users`, `vendors`, `atms` (and other master data) — `is_active=false` + `deleted_at`, never `DELETE FROM`; enforced by `internal/repository/no_hard_delete_test.go`. `atms.terminal_id` is immutable after create.
*   **Migration baseline (2026-09-18)**: `001_baseline_schema.sql` + `002_baseline_seed.sql` (dev password hash `password123`); old 001–040 live in `backend/migrations/archives/2026-09-18_pre-baseline/`. New migrations continue the sequence (currently up to `018_`).
*   `vendor_packages_branch` and `atm_vendor_packages` were truncated by migration 011 — **need re-seeding on dev** before those screens show data.
*   Full rationale, affected files, and per-migration history: [docs/decisions.md](./docs/decisions.md).

* * *
## 13\. UI/UX & Brand
> Full palette + rules live in [rules/frontend-ui.md](./rules/frontend-ui.md), auto-loaded for `frontend/**`. Internal app = "Merah Sirih" (red ≤10% accent); vendor portal = "Merah Menyala" (brand-forward). Everywhere, including exports: money in `tabular-nums`, currency (IDR) explicit, right-aligned; never encode status by colour alone.

* * *
## 14\. Backend Split: Transactional vs Batch (EOD)
**One codebase, two entrypoints.** Domain logic stays shared in `internal/*`. Two ways to run it:
*   `cmd/api` — transactional server. Always on, serves the frontends. Active during office hours, idle at night.
*   `cmd/batch` — End-of-Day runner. Now the Python pipeline (see decision below): file-driven, its scheduler scans/retries 05:00–09:00 WIB, idle outside that window.

> **Decision 2026-09-25: EOD stays in Python**, not a Go `cmd/batch`. The live pipeline is `backend_python/` with the monitoring page `features/eod-monitoring`. Read `cmd/batch` below as "the Python EOD pipeline"; the handoff rules (DB = source of truth, run table, read only `success` runs, idempotent per `processing_date`, alerts) still apply unchanged.

### `backend_python/` layout (verified 2026-09-29)

```plain
backend_python/
  dmaa/dmaa_etl.py                  # FTP_DATA/DMAA/Order_All_*.xlsx -> dmaa_files, dmaa_atm_forecast (bak/ = old versions, unused)
  itm/cashpos/itm_cashpos_etl.py    # FTP_DATA/ITM CSV cash-position snapshot -> itm_cashpos_files, itm_cashpos (+ test)
  itm/replenish/itm_replenish_etl.py# FTP_DATA/ITM CSV -> itm_replenish_files, itm_replenish
  dsr/dsr_etl.py                    # vendor DSR workbook (sheets 'Daily' + 'Rencana Isi') -> dsr_uploads, dsr_daily_rows, dsr_rencana_isi_rows (+ test)
  lib/                              # shared: database.py (pool), schemas, services/ (scheduler_service, detector, retry_executor, audit_service), utils/ (checksum, timezone WIB)
  eod_retry_scheduler/              # FastAPI, port 8091, env prefix RETRY_  — runs/retries dmaa, itm_cashpos, itm_replenish
  service_dsr_etl/                  # FastAPI, port 8090, env prefix DSR_ETL_ — DSR only (vendor-upload dry-run/commit)
```

*   **ETL scripts** are standalone CLIs: read input from `FTP_DATA/<DMAA|DSR|ITM>/` (repo root), idempotent per SHA-256 file checksum, move processed files to `backup/`. Money parsed as `Decimal`, never float.
*   **DSR filename contract** (written by Go `backend/internal/service/dsr_upload.go`): `<vendor_code>__<uploaded_by_user_id>__<original_filename>` (2-part form without user id also accepted). DSR amounts are **full IDR** despite the "(x 1.000)" header label.
*   **The two FastAPI services** share the same app shape (`main.py` lifespan creates a DB pool + starts `SchedulerService`) and the same routes: `GET /health`, `/status`, `/status/{file_id}/history`, `/summary`, `/late`, `/audit`; `POST /process/{file_type}`, `/retry/{file_id}`. `service_dsr_etl` adds `POST /process/dsr/dry-run` + `/process/dsr/commit`. Auth via `*_AUTH_MODE` (`api_key` | `jwt`) + `*_AUTH_SECRET`.
*   **Why DSR is split out**: DSR has a vendor-upload dry-run/commit flow; DMAA/ITM are batch-shaped. Don't merge them back.
*   **Scheduler**: scans every `*_SCAN_INTERVAL_MINUTES` (15) between `SCAN_CRON_START_HOUR`–`END_HOUR` (05–09 WIB), auto-retries up to `MAX_AUTO_RETRIES` (3) every `RETRY_INTERVAL_MINUTES` (30). SLA (WIB): DMAA 06:00, ITM cashpos/replenish 07:00, DSR 09:00 — past SLA → `late_detections`.
*   **Own tracking tables**: `retry_file_tracking`, `retry_audit_logs`, `late_detections`, `scan_runs` (these play the "run table" role above).
*   Config: each service has its own `.env.example` (`RETRY_*` / `DSR_ETL_*`, incl. `*_DATABASE_URL`, `*_ETL_*_SCRIPT` paths, `*_FTP_DATA_ROOT`). Never commit `backend_python/.env`.

Both run on the **same VM**. Their windows **partly overlap**: the Python scheduler scans/retries 05:00–09:00 WIB while office hours start 07:00, so 07:00–09:00 is shared with live transactional traffic. The ETLs are light (file-sized, one file at a time) and most files land before their SLA (DMAA 06:00, ITM 07:00), so the overlap is mostly DSR ingest (SLA 09:00) and retries. Rule: keep heavy/bulk batch work (backfills, reprocessing many dates) out of 07:00–20:00; if overlap ever causes contention, move EOD services to their own VM/container limits rather than adding a "borrowing" mechanism.

> This EOD `Final Realisasi` formula is separate from the daily `Order ATM` formula (Sec 3a) — don't conflate the two, they answer different questions (backdated realisasi summary vs. same-day/H-2 order calc).

**What the batch/EOD does** (backdated summary work, mirrors corebanking EOD):
*   Ingest & compute from DSR (saldo akhir 00:00 per vault), Opti Cash forecast (Order H-1 & H), horizon H-2 (refund validation).
*   Compute Final Realisasi = rekomendasi DMAA − (saldo DSR + refund horizon H-2), per vendor.
*   Produce the daily summaries the transactional module reads the next working day.

**Handoff rules (NON-NEGOTIABLE):**
*   EOD output is written to **DB = source of truth** (durable, queryable, auditable). Redis only as a cache layer on top, never the store of record for EOD results.
*   Every EOD run is tracked in a run table (`forecast_runs` / an `eod_runs` table): `processing_date`, status (running/success/failed), started\_at, finished\_at, records\_processed, error.
*   Transactional module reads a `processing_date` only after its run is marked **success**. Never read partial/in-progress data.
*   On completion, emit a domain event (`EODCompleted` / `SummaryReady`) — fits the in-process event-driven model.
*   Batch ingests are idempotent per file/`processing_date` (re-run safe).

**EOD Monitoring (admin / app-support only):**
*   Dedicated dashboard page: per-run status, `processing_date`, duration, records processed, last success, failures with error detail.
*   **Email alert** to admin/app-support on: EOD failure, AND EOD not completed before office-hours start (the critical case — stops operators working on stale data). Sent via company SMTP.
*   Scope this page + alerts to admin/app-support roles only.

* * *
## 15\. Where docs live
*   Legacy feature specs + steering docs: `.kiro/` (`.kiro/specs/<feature>/`, `.kiro/steering/`) — still valid for features started there (Sec 4a rule 7).
*   New feature artifacts: `.claude/sdlc/<feature>/` (Sec 4a).
*   Detail moved out of this file (read on demand): `.claude/docs/` — `requirements.md` (URS), `data-map.md` (table/column notes), `decisions.md` (decision log), `deployment.md` (GCP). Keep this file to always-on rules; put long history there.
*   Path-scoped rules (auto-loaded by path): `.claude/rules/` — `frontend-ui.md` for `frontend/**`.
