# PROJECT_CONTEXT.md — CMS ATM & CIT (CLAUDE)

# PROJECT\_CONTEXT.md — Cash Management System (CMS) for ATM & CIT
> **AI: READ THIS FIRST, EVERY SESSION.** Single source of truth. If your work conflicts with this, STOP and ask. Never invent endpoints, tables, columns, env vars, or modules.
> **For simple explanations of CMS concepts**, see [eli5.md](./.claude/eli5.md) — use when explaining to teammates or stakeholders.
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
8. Two backend entrypoints, one codebase: `cmd/api` (transactional, office hours) + `cmd/batch` (EOD, midnight). Shared `internal/*` (Sec 14).
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
frontend/CompanyPortal-Vite/ # internal app, LDAP login
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
*   **Core**: `audit_logs`, `approval_requests`, `master_data_change_requests` (staged master-data changes — see Sec 12), `master_data_import_batches` (CSV import files, one approval per batch, idempotent per SHA-256 — Sec 12), `documents`, `notifications`, `import_jobs`, `export_jobs`
*   **Auth/Core — Role Management** (approved, `.kiro/specs/role-management`, migration `040_role_permissions.sql`, not yet applied):
    *   `menu_features` — Menu_Feature_Catalog. `id` bigint identity PK · `parent_id` bigint self-FK → `menu_features(id)` ON DELETE RESTRICT, nullable (NULL = top-level menu, non-NULL = feature under a menu) · `key` text UNIQUE (stable machine key, e.g. `settings.roles`) · `label` text · `kind` text CHECK IN ('menu','feature') + CHECK hierarchy (`menu`⇔`parent_id IS NULL`) · `sort_order` integer default 0 · `is_active` boolean default true · `created_at`/`updated_at` timestamptz default now(). Index on `parent_id`.
    *   `role_permissions` — Role_Permission_Mapping (many-to-many `roles`↔`menu_features`; row presence = grant). `id` bigint identity PK · `role_id` bigint FK → `roles(id)` ON DELETE CASCADE · `menu_feature_id` bigint FK → `menu_features(id)` ON DELETE CASCADE · `granted_by` bigint FK → `users(id)` · `created_at` timestamptz default now(). UNIQUE `(role_id, menu_feature_id)`; indexes on `role_id` and `menu_feature_id`.
*   **Master**: `vendors` (incl. `legal_name`, `npwp`, `kind` = FLM_VENDOR|INTERNAL — see vendor-pricing note below), `vendor_branches`, `branch_coverage_areas`, `vendor_vaults` (category ATM/CASH/ATM_CASH), `vendor_pics`, `vendor_packages_branch` (renamed from `vendor_packages`, migration `011`; as of migration `016` a branch-scoped special price table — "harga khusus cabang" — see Sec 12), `cimb_branches`, `package_frequencies`, `vendor_package_prices` (vendor-wide PT/branch/ATM-override price tree, unrelated to and unchanged by migration 016; identifier column split into `package`/`package_code` by migration `017` — see below), `locations`, `atms` (incl. `cimb_branch_id`, `price_machine_group`, `price_class` generated columns), `atm_vendor_packages` (= ATM assignments/kelolaan ATM: effective-dated, no overlapping active period per ATM; FK to `vendor_packages_branch`). **There is no `vendor_assignments` table** — earlier drafts of this map used that name; the real names above follow the DB (decision D4, Sec 12). Most are soft-disabled (`is_active`/`deleted_at`), never hard-deleted, **except `branch_coverage_areas`** (pure link table, no soft-delete columns; removed by deleting the row, FK `ON DELETE CASCADE` from `vendor_branches`) **and `vendor_packages_branch`/`vendor_package_prices`** (effective-dated history since migration 016/009 respectively — "disable" closes `effective_end_date`, never a row toggle, never hard-deleted either).
    *   **Vendor package pricing** (plan: `.claude/sdlc/vendor-pricing/plan.md`; migrations `009_vendor_package_prices.sql` + `010_vendor_package_prices_data.sql`, applied to dev DB 2026-09-22). `vendor_packages.price`/`priority_class` were dropped — every row there is now just a kelolaan/frequency link, never a price. `ROH` (`vendors.kind='INTERNAL'`) is not an FLM vendor: it marks ATMs sited at a CIMB branch office, staffed by branch personnel, never billed. Its 300 `ROH_0xx` branches moved to the new `cimb_branches` table (backfilled onto `atms.cimb_branch_id`); its ~485 per-branch packages collapsed into 2 internal packages (`vendor_packages.vendor_branch_id IS NULL`). Real prices live in `vendor_package_prices` — three levels in one table (PT base / branch override / per-ATM override, distinguished by which of `vendor_branch_id`/`atm_id` is set, nullable per-field so an override can touch one price field and inherit the rest), effective-dated, `EXCLUDE USING gist` prevents overlapping tier+date ranges for the same vendor/package/machine-group/class/level. CR/FLM contract frequency is separate, in `package_frequencies(package_code, machine_group)` — frequency differs by machine type (e.g. PAKET 3 ATM = FLM 4, but PAKET 3 CDM/CRM = FLM 6), so it can't live as a column on `vendor_packages`. `atms.price_machine_group`/`price_class` are generated columns bridging the DB's 3-valued `machine_type`/`priority_class` onto the document's 2×2 tariff grid (ATM vs CDM_CRM; REGULAR vs VIP_INDUSTRI) — unmapped values resolve to NULL so lookups fail hard instead of silently mis-pricing. Seed data (P19) is **partial**: Advantage/Bijak/TAG/Abacus fully seeded from `archives/Harga Paket per vendor FLM.docx`; SSI only has Paket 4/5 (doc has no Paket 3/6 SSI pricing); per-ATM "Harga Khusus" (17 ATMs) and "Biaya Tambahan Operasional" (3 ATMs) are **not seeded** — both need a human-verified `vendor_id`/`atm_id` the source document doesn't state, deferred to the P20 admin screen. Backend (queries → repo → service with maker-checker → handler → `Applier`) and the CompanyPortal screen **are now built** — see `vendor-package-code-split` note below and Sec 12.
    *   **`vendor_package_prices` identifier split into `package`/`package_code`** (plan: `.kiro/specs/vendor-package-code-split`; migration `017_vendor_package_code_split.sql`, applied to dev DB 2026-09-24). The column that used to hold the label ("PAKET 3") and double as the grouping key was **renamed** `package_code` → `package` (data, `NOT NULL`, and its role in `vpp_no_overlap`/`vpp_lookup_idx` all preserved — those two were rebuilt to reference `package` instead, otherwise byte-for-byte identical to migration 009's definitions). A **new** `package_code text` column was added, backfilled deterministically as `PKG<digits-of-package>_<vendor.code>_<3-digit per-vendor sequence ordered by id>` (digitless labels fall back to `PKG0_...`), then locked down with `NOT NULL` + a global `UNIQUE(package_code)` (`vendor_package_prices_package_code_key`). Any join from a price row to its frequency now matches `vendor_package_prices.package = package_frequencies.package_code` (that table's own `package_code` column is unchanged — it's still the label). On create, `package_code` is generated server-side by `VendorPackagePriceApplier.nextPackageCode` at apply-on-approve time (never client-supplied) inside the same transaction as the insert, serialized per-vendor via `pg_advisory_xact_lock`, with the global unique constraint as the final collision backstop (`23505` → `ErrVendorPackageCodeConflict`). The admin API (`/api/v1/admin/vendors/{vendorId}/package-prices`) now returns both fields; list/count filter by the `package` label (query param renamed from `package_code`, no legacy alias). `PackagePricesPanel.tsx` shows two columns — "Kode Paket" (`package_code`, mono/tabular) and "Paket" (`package`) — and the create form collects only the `package` label.
    *   Known gap: `backend/internal/handler/integration_test.go` only runs migrations `001`+`002` against the test DB, so `003`–`010` (including this pricing schema) are not yet exercised by that integration test. Pre-existing, tracked separately.
    *   `regions` gained soft-delete columns (migration `018_regions_soft_delete.sql`, `.kiro/specs/region-management`, additive, not yet applied to dev DB) — `+ is_active boolean NOT NULL DEFAULT true` (backfills all 13 existing rows as active), `+ deleted_at timestamptz NULL`. `uq_regions_code` and the `updated_at` trigger already existed in the baseline, so this migration only adds the two soft-disable columns. Enables the upcoming "Manajemen Region" admin CRUD (immediate-apply-with-audit, like `role-management`, not maker-checker) — disable is blocked while any active `locations` row still references the region; never a hard `DELETE`.
    *   `branch_coverage_areas` (migration `007_branch_coverage_areas.sql`, applied 2026-09-22) — kota yang dilayani sebuah cabang vendor, **many-to-many**: satu cabang melayani banyak kota dan satu kota dilayani banyak cabang vendor, sehingga tidak muat di `vendor_branches.region` (satu kolom text, dan `branch_code` UNIQUE). `id` bigint identity PK · `vendor_branch_id` bigint FK → `vendor_branches(id)` ON DELETE CASCADE · `city` text · `created_at` timestamptz default now(). UNIQUE `(vendor_branch_id, city)`; index on `city`. Seeded with 171 baris dari `archives/vault dan cashpoint/vendor_vault_region_branch.csv` (55 cabang × 87 kota). **Read-only saat ini** — belum punya queries/repo/handler; bila kelak dikelola dari layar, wajib mengikuti pola maker-checker master data (Sec 12: queries → repo read-only → service `Submit` → `Applier` di `main.go` → handler 202), bukan menulis tabel langsung.
    *   `vendor_vaults.category` diperluas dari biner `('ATM','CASH')` menjadi `('ATM','CASH','ATM_CASH')` di migration `008_vendor_vault_types.sql` (applied 2026-09-22): daftar master vendor menyebut sebagian cabang melayani ATM sekaligus CASH. Sebelumnya seluruh 348 baris adalah placeholder `'ATM'` hasil backfill di `003_master_data.sql`; kini 99 cabang non-ROH terisi nilai sebenarnya (44 ATM_CASH / 43 CASH / 12 ATM), sedangkan 308 cabang ROH tidak ada di daftar itu dan masih placeholder. Kolom legacy `vendor_vaults.type` **selalu di-mirror dari `category`** (lihat `Create`/`UpdateVendorVaultAdmin` di `backend/queries/vendor_vaults_admin.sql`) — jangan perlakukan sebagai field independen. Migration 008 juga menambah vendor `BRINKS`/`KEJAR`/`PROSEGUR` + 51 cabang baru (total 407 cabang).
*   **ATM**: `atm_dsr_uploads`, `atm_dsr_rows`, `replenishment_instructions`, `forecast_runs`, `forecast_results` — *(proposed, NOT yet approved — see Sec 3a)* `cash_count_schedules`, `cash_count_evidences`
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
    *   Fallback: DMAA recommendation exists but DSR missing → compute from DMAA recommendation alone. **Open question in URS** ("apakah rumus ini masih valid?") — confirm with business before treating as final.
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
*   **Admin CRUD for `users`/`vendors`** (`.kiro/specs/admin-user-vendor-management`): `backend/internal/handler/admin_user_handler.go` mounts `/api/v1/admin/users` (list/get/create/update/disable/enable), guarded `APPACCESS`; `admin_vendor_handler.go` mounts `/api/v1/admin/vendors` (same verbs), guarded `ADMIN`/`ADMIN_PARAM`. Disable is **soft-delete only** — `is_active=false` + `deleted_at`, never a hard `DELETE FROM`; `internal/repository/no_hard_delete_test.go` enforces this for both tables. A local (`auth_source=local`) user create issues a system-generated **temporary password** with `must_change_password=true` in the same insert, forcing a change on next login — no separate provisioning call. A search-index migration originally scoped for these screens was **deferred** (not applied); see the spec's own Task 0/1 notes and follow-ups.md for the exact status. **Superseded for `vendors` (2026-09-20, plan T4.1/T4.3):** vendor create/update/disable/enable are no longer applied immediately — they are staged for approval and return 202 (see the "Master-data maker-checker" bullet below); vendors also gained `legal_name` + `npwp`. `admin/users` is unchanged (still immediate, `APPACCESS`).
*   **Role Management** (`.kiro/specs/role-management`, in progress): data-driven menu/feature permission layer for the ATM backend + a "Manajemen Peran" screen under Pengaturan in `CompanyPortal-Vite`, scoped to `APPACCESS`/`ADMIN`. **Deliberate, documented deviation from Golden Rule #3**: create/update on `menu_features`/`role_permissions` apply **immediately** (no `approval_requests`, no maker-checker) — the substitute control is (a) a mandatory `audit_logs` write in the **same DB transaction** as the mutation (audit-write failure rolls back the change, so no permission/role change can exist without an audit trail), and (b) re-checked `APPACCESS`/`ADMIN` authorization at both the route-guard and service layers. New roles are born with zero access until explicitly granted. This does not alter any existing `RequireRoles(...)` guard on other routes.
*   **Admin CRUD for `atms`** (`.kiro/specs/admin-atm-management`): `backend/internal/handler/admin_atm_handler.go` mounts `/api/v1/admin/atms` (list/get/create/update/disable/enable, plus a read-only `/locations` lookup for the form's Location select), guarded `ADMIN`/`ADMIN_PARAM`. Disable is **soft-delete only** — `is_active=false` + `deleted_at`, never a hard `DELETE FROM`; `no_hard_delete_test.go` was extended to cover `atms` too. `terminal_id` is immutable after create (rejected with 400 on an attempted change). The existing read-only `/api/v1/atm-portal` monitoring viewer is unchanged by this spec. Unlike the sibling users/vendors spec, this one **approved and applied** an additive trigram search index — `backend/migrations/039_admin_atm_search_index.sql` (`CREATE EXTENSION pg_trgm` + a GIN index on `atms.terminal_id`) — since `atms` already held ~1900 rows and `q` is the admin list's primary lookup; `pg_trgm` only accelerates substring matches of 3+ characters (a 1-2 char search still seq-scans, which is fine at this volume). Frontend screen lives at `/settings/admin/atms` (`frontend/CompanyPortal-Vite/src/features/admin-atms/`), surfaced as a "Manajemen ATM" card in the Settings hub (`SettingsHubPage.tsx`) — no top-level `NAV_CONFIG` entry. **Superseded for the write path (2026-09-20, plan T4.2):** ATM create/update/disable/enable are staged for approval and return 202, no longer applied immediately; `terminal_id` immutability is unchanged (and now also enforced at the applier/query layer). The frontend screen still assumes the old immediate-apply response until plan Fase 6 updates it.
*   **Master-data maker-checker & scope** (plan: `.claude/sdlc/master-data/plan.md`; migrations `003_master_data.sql`, `004_vendor_packages_soft_disable.sql`). Decisions: **D1** — every master-data create/update/disable/enable (`vendors`, `vendor_branches`, `vendor_vaults`, `vendor_pics`, `vendor_packages`, `atms`, `atm_vendor_packages`) goes through maker-checker (Golden Rule #3): `MasterDataChangeService.Submit` writes a row to `master_data_change_requests` (payload + "before" snapshot) and routes it into `approval_requests` (`document_type='master_data'`); the endpoint returns **202** `{change_request_id, status:"pending", entity_type, op}`. The entity row changes **only when the final approval passes**, in one transaction with a `master_data_applied` audit entry, via a per-entity `Applier` (registry in `cmd/api/main.go`). If the entity changed since submit the change is marked `stale`, not applied; failures at apply (e.g. an ATM assignment period overlap, a duplicate code) leave it approved-but-not-applied and surface as a clean 409. Approve/reject go through `/api/v1/approvals` (login only — approver authority is the approval hierarchy, maker ≠ checker). RBAC at both layers: the `masterDataAdmin` route group (`ADMIN`/`ADMIN_PARAM`) **and** a re-check inside `Submit`. Endpoints: `/api/v1/admin/vendors` (+ `/{id}/branches|vaults|pics|packages`), `/api/v1/admin/atms` (+ `/{id}/assignments`), read-only `/api/v1/admin/master-data/changes`. CSV import/export under `/api/v1/admin/master-data/{export,import}/{entity}` (export streams from the replica; import `dry-run` then `confirm` reads the primary): `confirm` stages the whole file as ONE batch (`master_data_change_requests.batch_id`) under ONE approval; approving applies all rows in one transaction, all-or-nothing. **D2** — bulk import/export starts as **CSV** (`encoding/csv`, stdlib); XLSX/PDF deferred; no new dependency. **D3** — kelolaan branch/customer deferred; kelolaan scope is vendor → ATM (`vendor_packages` + `atm_vendor_packages`). **D4** — table names follow the DB (`vendor_branches`, `vendor_vaults`, `vendor_packages`, `atm_vendor_packages`), not `vendor_assignments`. New master-data entities must follow this pattern (queries → read-only repo → service that stages via `Submit` → `Applier` registered in `main.go` → handler returning 202), never write the table directly from the API path.

*   **Vendor package pricing schema** (2026-09-22, migrations `009`+`010`, applied to dev DB): see the detailed note under Sec 3 Master data. Summary of the decision: `vendor_packages.price`/`priority_class` were fake data (all 560 rows were `price=0.00`) and got dropped; real per-vendor pricing now lives in a new `vendor_package_prices` table (PT/branch/ATM levels, effective-dated); `ROH` was reclassified as an internal unit (`vendors.kind='INTERNAL'`), not an FLM vendor, with its CIMB branch-office data moved to a new `cimb_branches` table. Schema only — backend and frontend for this are not built.
*   **`vendor_packages` renamed to `vendor_packages_branch` + data cleared** (2026-09-23, migration `011_rename_vendor_packages_branch.sql`, applied to dev DB, explicit user request). `ALTER TABLE ... RENAME` then `TRUNCATE ... CASCADE`. Because `atm_vendor_packages.vendor_package_id` is a **NOT NULL FK** to this table, the cascade also emptied `atm_vendor_packages` (ATM↔vendor kelolaan assignments) — confirmed with the user before applying (562 `vendor_packages` rows + 1862 `atm_vendor_packages` rows at the time, both now 0). Indexes/constraints/trigger/table-comment carried over automatically from the rename (not individually renamed). All `backend/queries/*.sql` references updated (`vendor_packages_admin.sql`, `atm_assignments_admin.sql`, `vendor_request.sql`, `master_data_export.sql`) and `sqlc generate` re-run — the sqlc-generated Go struct is now `db.VendorPackagesBranch` (was `db.VendorPackage`, unused directly outside `internal/db`, so no service-layer signature changes). `no_hard_delete_test.go`'s guarded-table list updated to match. **Needs re-seeding** before the Cabang → Paket admin screens (`/settings/admin/vendors/:id/branches/:id`) or ATM assignment screens show real data again — both tables are currently empty on dev.
*   **`vendor_package_prices` addon columns dropped** (2026-09-23, migration `012_drop_vendor_package_price_addons.sql`, applied to dev DB, explicit user request). Drops `add_cr_price`, `add_flm_price`, `extra_amount`; `vpp_amount_chk` narrowed to just `base_price IS NULL OR base_price >= 0`. No seeded data depended on these columns — P19/P20 seeding (Sec 3 Master data) was `base_price`-only; the per-ATM "Harga Khusus"/"Biaya Tambahan Operasional" rows that would have used them were never seeded. `internal/db/models.go`'s `VendorPackagePrice` struct already reflected the drop (sqlc had been regenerated against the migration file before this migration was applied to dev) — no further sqlc regen needed. No service/handler code referenced these columns (pricing backend is not built yet, Sec 3).
*   **`vendor_packages_branch` gained tier/effective-date columns** (2026-09-23, migration `015_vendor_packages_branch_tiering.sql`, applied to dev DB via `localhost:5432` — `host.docker.internal` in `backend/.env` only resolves from inside a container, not from a host shell). Adds `tier_min`/`tier_max`/`effective_start_date`/`effective_end_date`. **Superseded same day by migration 016 below** — the "no price columns, price stays only in `vendor_package_prices`" design here didn't hold; read 016 for the current shape. The "Harga Paket" tab this migration's turn added to `VendorBranchDetailPage` was also reverted in the 016 turn.
*   **`vendor_packages_branch` becomes a branch-scoped special price table** (2026-09-23, migration `016_vendor_packages_branch_pricing.sql`, explicit user request, plan confirmed via two rounds of AskUserQuestion, applied to dev DB via `localhost:5432`). Final columns: `id, vendor_branch_id (NOT NULL), package_code, machine_group, price_class, tier_min, tier_max, base_price, atm_id, sla_note, currency, effective_start_date, effective_end_date, created_at, updated_at` — `code` renamed to `package_code`; `is_active`/`deleted_at` dropped; `machine_group`/`price_class`/`base_price`/`atm_id`/`sla_note`/`currency` added. Old unique `(vendor_branch_id, code)` replaced with `vpb_no_overlap` (`EXCLUDE USING gist`, same shape as `vpp_no_overlap`) since a `package_code` can now have several rows (one per machine_group/price_class/tier/period). This is deliberately **separate from `vendor_package_prices`**, not a replacement of it: `vendor_package_prices` (unchanged, still shown via `PackagePricesPanel` on `/settings/admin/vendors/:id`) holds the vendor-wide PT/branch/ATM-override price tree; `vendor_packages_branch` (shown as "Paket Cabang" tab on `/settings/admin/vendors/:id/branches/:id`, `PackagesPanel.tsx`) holds a branch's own special/custom price ("harga khusus cabang"), entered directly, independent of the vendor-wide tree. Effective-dated history like `vendor_package_prices` — no Enable, "disable" (`AdminVendorPackageHandler.Disable`, `POST /{id}/disable`) closes `effective_end_date`. Service/applier/handler (`vendor_package_admin.go`, `masterdata_applier_vendor_package.go`, `admin_vendor_package_handler.go`) rewritten to mirror `VendorPackagePriceAdminService`'s grain-vs-content split; `VendorPackageContentPayload` is a Go type alias for `VendorPackagePriceContentPayload` (identical shape, avoids duplicating `validatePriceContent`). **Known gap, accepted by explicit user decision**: `atm_vendor_packages.vendor_package_id` (ATM assignment / kelolaan, `admin_atm_assignment_handler.go`) still points at one row here; now that a `package_code` can have multiple rows, nothing validates that the assigned row's `machine_group`/`price_class` matches the ATM's own `atms.price_machine_group`/`price_class` — flagged, not implemented (out of scope per user decision 2026-09-23). Downstream query renames (`p.code` → `p.package_code`): `atm_assignments_admin.sql`, `master_data_export.sql`, `vendor_request.sql`. `PackageActive` (`atm_assignment_admin_repository.go`) switched from checking `DeletedAt` to checking `EffectiveEndDate` current/open-ended.
*   **`vendor_package_prices` identifier split (Migration 017)** (2026-09-24, `.kiro/specs/vendor-package-code-split`, migration `017_vendor_package_code_split.sql`, applied to dev DB via `localhost:5432`). See the detailed note under Sec 3 Master data. Summary: the pre-existing `package_code` column (label, "PAKET 3", the grouping key `vpp_no_overlap`/`vpp_lookup_idx` partition on) was renamed to `package`; a new per-row `package_code` was added and backfilled `PKG<digits>_<vendor.code>_<seq3>`, then locked with `NOT NULL` + global `UNIQUE`. `package_code` is now server-generated at apply-on-approve time (`VendorPackagePriceApplier.nextPackageCode`, per-vendor `pg_advisory_xact_lock` + unique-constraint backstop) — never client-supplied. sqlc regenerated clean on the pinned v1.31.1 (the `UserLeafe`→`UserLeave` inflector quirk recurred and was hand-fixed per the sqlc note above). Service (`vendor_package_price_admin.go`), applier, handler, and `PackagePricesPanel.tsx`/`VendorPackagePriceFormDialog.tsx` all updated; list/count filter param renamed `package_code`→`package` with no legacy alias (only consumer is the CompanyPortal panel, updated in lockstep).
*   **Migration baseline (2026-09-18)**: `backend/migrations/` was squashed into `001_baseline_schema.sql` (pg_dump schema-only of the fully migrated DB, incl. `btree_gist` + `pg_trgm`) + `002_baseline_seed.sql` (reference/master data: roles, users [dev hash `password123`], menu_features, role_permissions, regions, locations, vendors, vendor_*, atms, atm_vendor_packages, approval_policies). Old 001–040 moved to `backend/migrations/archives/2026-09-18_pre-baseline/` — file names cited elsewhere in this doc (e.g. 039/040) now live there. New migrations continue from `003_`. Note: old `012_retry_scheduler.sql` tables were never applied to the DB, so they are NOT in the baseline.

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