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
*   **Stage**: Greenfield, vibe-coded with AI.
*   **Roles**: Admin, Operator, Manager (approver), Vendor, Branch/Internal User.

* * *
## graphify

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
## 3a. Business Rules & Requirements (from URS v0.3 Phase 1 — `UR New Template v0.3 - E2E Cash Management System v.4 - Phase 1.docx-20260918115415.md` at repo root; older Rev1 .docx in `archives/`)
> Source-of-truth requirements doc. Anything below not yet reflected in code/schema is a **spec**, not an implemented behavior — check code before assuming it's live.
> Per-feature flow diagrams (Mermaid, editable): `.claude/feature-flows/<feature>/feature-flow.md`.

*   **Functional requirements (all High)**: FNC 001 ATM Cash Forecasting (DSR intake, replenishment instruction, projection) · FNC 002 Cash Count (scheduling, reconciliation, progress + result report; vault ATM/Cash + selektif mesin) · FNC 003 Dashboard (daily instruction amount + term ID, DSR lateness recap, cash count daily progress + monthly report).
*   **Order ATM formula** (daily forecasting/replenishment, `cmd/api` — distinct from the EOD `Final Realisasi` formula in Sec 14, which is a different calc for a different job):
    `Order ATM = (Saldo DSR + Proyeksi Refund) − (Rekomendasi DMAA + Rencana Isi Hari-H)`
    *   Fallback: DMAA recommendation exists but DSR missing → compute from DMAA recommendation alone. URS open question ("apakah rumus ini masih valid?") **resolved 2026-09-25: formula + fallback confirmed valid as written.**
    *   `Rencana Isi Hari-H` = previous day's order to be filled on H; it reduces the vendor's physical balance.
    *   `Proyeksi Refund` per ATM = opening balance (H) − predicted transactions (H and H+1), from DMAA/Data Science horizon.
    *   Results groupable by vendor · vault · denomination (cash need per vendor).
*   **Forecast input uploads** (review of DMAA H0 recommendation): complaint-handling/recon list (skip if DMAA already recommended emergency/planned yesterday, else add as emergency order) · ATM project list from business units (replace/new/relocation → add as emergency/planned) · problem-ATM list (exclude) · adjustment order (replaces DMAA nominal for listed IDs). Merged into a **draft order** → tiered approval (maker-checker) → publish replenishment instruction + notification.
*   DSR daily upload deadline: **09:00**. Monthly report of late/missing DSR per vendor feeds FLM penalty basis (columns: report date [not send date], vendor + vault area, received-at, status OK/TELAT). Email/in-app notification on late DSR.
*   Duplicate-order prevention: an ATM with an emergency/adhoc order issued up to H-1, or active on H0, gets no additional order (excluded from the Data Science forecast file).
*   ATM in "problem" status (pending part, vandalism, etc.) is excluded from replenishment recommendations.
*   **Pemenuhan dana (fund fulfillment)**: branch/Cash Management sets source location, nominal per denomination, pickup date + time, providing vault; vendor FLM sees it in-app. **Pengambilan dana**: FLM inputs officer + vehicle (nama, KTP, NIP, perusahaan, keperluan, nominal + per denom, jumlah lembar, no. kendaraan, tanggal) → maker-checker approval → system issues downloadable surat tugas → verification + handover (serah terima) between Cash Management and FLM.
*   Replenishment result is classified (holiday-adjusted): on-schedule · early (1–2 days) · late (1–2 days) · not done (>2 days off or skipped) · **replenished without an order** (realisasi vs order).
*   **Required reports**: refund per ID (filter vendor/denom) · fill amount ≠ order (with ID detail) · transactions (tarik/setor) · ATM profile · order vs transaction · user report · user log (last login) · trip/realisasi per vendor per ATM ID (incl. highest).
*   **Dashboard**: status, aging, SLA, historical trend, exception indicators; filter + drill-down; export CSV/XLSX/PDF.
*   **Master data (vendor)**: vendor legal identity, active/inactive, NPWP, notification PIC; vault address, coordinates (optional), operating hours, capacity, category ATM/Cash; PIC jabatan/phone/email; kelolaan vendor → ATM and/or branch/customer. Changes via maker-checker + audit trail; bulk import/export CSV/XLSX with structure + content validation.
*   **Cash count (vault, monthly)**: risk category from escrow (SIBS/MIS) balance analysis drives a random/non-patterned visit schedule (ignores holidays/non-working days; considers regional PIC availability). Assigned PIC gets email notification, can accept/reject (reject → history kept, then reschedule or reassign). On accept, a surat tugas is issued. Berita Acara (BA) is filled digitally on-site, DSR column auto-fills from vendor's uploaded DSR, photo evidence attached, dual e-sign (vendor + bank PIC). BA templates: vault ATM, vault Cash, valas; plus checklist parameters. Final docs (BA, checklist, photos) downloadable/printable. Monthly recap = 3-way reconciliation: cash count vs. escrow (H-1, auto from MIS/SIBS) vs. proofing (manual input), diffs flagged for follow-up; per-escrow status Complete / On-progress / Not Complete + findings; vendor performance evaluation.
*   **Cash count selektif (machine-level)**: same flow as vault cash count, scoped to specific ATMs per supervision instruction.
*   **Invoice reconciliation**: vendor uploads invoice + supporting docs; CIMB Niaga internal team uploads ATM master data (active/terminated ATM, price, trip package, category VIP/Industri/Regular); system auto-reconciles; internal team can manually adjust against vendor disputes (sanggahan).
*   **NFR targets**: 24×7 availability outside planned maintenance · dashboard load ≤3s (p95) · DSR upload ≤30s/doc · journal-post initial response ≤5s with async status confirm ≤2min · 300 concurrent active users · horizontal scalability for vendor portal · Data Centers: Bintaro & NTT · operating hours 07:00–20:00 (uptime 24×7) · responsive/mobile-usable UI + basic accessibility · structured logging, telemetry, metrics, operational alerts · certified e-sign optional.
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
**Brand anchor**: CIMB Niaga Red — `#E4142A` → `oklch(56% 0.223 27)`. Use OKLCH for all colors; build shade scales by holding chroma+hue constant and varying lightness. Never `#000`/`#fff`; tint neutrals slightly toward the brand hue.

**Two themes, one per frontend:**
*   **Internal app** (`frontend/CompanyPortal-Vite`) → **Option A "Merah Sirih"**. Warm off-white neutrals, red as a ≤10% accent (primary buttons, active states, key figures). Optimized for data-dense screens operators stare at all day.
    *   Primary `oklch(56% 0.223 27)` · Primary Deep `oklch(47% 0.185 27)` · Red Tint `oklch(94% 0.03 25)` · Surface `oklch(98.6% 0.006 40)` · Text `oklch(26% 0.02 30)`
*   **Vendor portal** (`frontend/VendorPortal-Vite`) → **Option B "Merah Menyala"**. Bold, brand-forward: maroon-red top bar, full-red active sidebar. Strong CIMB identity from first load, especially on login.
    *   Primary `oklch(54% 0.233 27)` · Maroon Bar `oklch(40% 0.155 26)` · Maroon Deep `oklch(30% 0.11 25)` · Surface `oklch(99.5% 0.003 40)` · Text `oklch(25% 0.02 28)`

**Rules:**
*   Font: one family in multiple weights (hierarchy via scale + weight, not two competing fonts). Use `tabular-nums` for all money/metrics tables.
*   **Accessibility**: never encode status with color alone — always pair red/green with a label or icon (color-blind users). Deep red on white is safe for bold text/buttons only; do NOT use it for small thin text (insufficient contrast).
*   Red is an accent, not wallpaper (internal). Don't scatter it everywhere — it works because it's rare.
*   Money as `tabular-nums`, currency shown explicitly (IDR), amounts right-aligned in tables.
*   Palette reference / live mockups: see the CMS palette showcase artifact.

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
