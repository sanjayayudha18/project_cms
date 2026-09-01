# PROJECT_CONTEXT.md — CMS ATM & CIT (CLAUDE)

# PROJECT\_CONTEXT.md — Cash Management System (CMS) for ATM & CIT
> **AI: READ THIS FIRST, EVERY SESSION.** Single source of truth. If your work conflicts with this, STOP and ask. Never invent endpoints, tables, columns, env vars, or modules.
> **For simple explanations of CMS concepts**, see [eli5.md](./.claude/eli5.md) — use when explaining to teammates or stakeholders.
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
| DB pool | pgx / pgxpool (`pkg/database`) |
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
frontend/CodexCash-Vite/     # internal app, LDAP login
frontend/VendorPortal-Vite/  # vendor portal, local login
```

### Modules (create ONLY these unless told)
**Platform Core**: `internal/auth` (LDAP + local, JWT, /me) · `internal/user` · `internal/audit` · `internal/approval` (maker-checker) · `internal/document` · `internal/notification` (in-app + SMTP) · `internal/export` (CSV/XLSX/PDF)

**Master Data**: `internal/vendor` · `internal/vendorpic` · `internal/vault` · `internal/location` · `internal/atm` · `internal/assignment`

**ATM Operations**: `internal/dsr` · `internal/replenishment` · `internal/forecast` (H+2) · `internal/cashcount` (vault + selective machine cash count — BA/checklist/photo/e-sign/3-way reconciliation; tables not yet approved, see Sec 3 DB map note)

**Finance**: `internal/invoice` (upload + validate + approve — NO payment execution) · `internal/reconciliation`

**CIT**: `internal/cit` · `internal/journal` · CIT reconciliation (order vs DSR vs journal)

**Integration**: `internal/corebanking` (escrow **batch file** ingest + parse -> feeds reconciliation)
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
*   **Consistent response**: all endpoints return `pkg/response` envelope.

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
- [ ] Uses pkg/response envelope
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
## 13\. UI/UX & Brand
**Brand anchor**: CIMB Niaga Red — `#E4142A` → `oklch(56% 0.223 27)`. Use OKLCH for all colors; build shade scales by holding chroma+hue constant and varying lightness. Never `#000`/`#fff`; tint neutrals slightly toward the brand hue.

**Two themes, one per frontend:**
*   **Internal app** (`frontend/CodexCash-Vite`) → **Option A "Merah Sirih"**. Warm off-white neutrals, red as a ≤10% accent (primary buttons, active states, key figures). Optimized for data-dense screens operators stare at all day.
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
*   `cmd/batch` — End-of-Day runner. Triggered by cron/scheduler at midnight (date rollover), processes, then exits/idles. Idle during office hours.

Both run on the **same VM**. Because their active windows don't overlap, each gets the full machine when it runs — no resource contention, no "borrowing" mechanism needed. This is intentional: never run heavy batch work concurrently with live transactional traffic.

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

``````
***
all file live in "C:\Users\RB Yudha Rangga\OneDrive\Documents\Development\CMS2\.kiro"