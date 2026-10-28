# CMS Development Plan (Kiro Steering Doc)

> Roadmap for the remaining CMS work, sequenced against the module & table map in `project-context.md` / `.claude/CLAUDE.md` Sec 3. This is a **living plan** — update it when a module lands, a table is approved, or a dependency shifts.
> **Companion docs:** `project-context.md` (source of truth for modules/tables/rules), `tech.md` (fixed stack), `structure.md` (layout), `model-recommendation.md` (per-task Model/Effort convention), `.claude/development-progress.md` (feature-level changelog), `.claude/docs/arsitektur-teknologi.md` (stack + feature status verified from code).

---

## ⛔ Scope guard — read before starting ANY task

**Current focus (in order):** Phase 2 (ATM ops). 2.1 closed, 2.2a–2.2d built (manual browser checks with the user). **Next: 2.3 DSR late report (awaiting PO acceptance) → 2.4 problem-ATM exclusion.** Cross-cutting item X.1 (CompanyPortal app-shell revamp) is in spec. Phase 0 (infra) and Phase 1 (close in-flight) are **done** — only their manual browser checks remain, which are the user's job.
Anything not listed under Phase 2 or the cross-cutting section is **out of scope right now**. If a request touches it, STOP and ask.

Rules:
1. **One task = one item in this file.** If the work isn't an item here, add it here first (with user OK), then build it.
2. **No new master-data features.** Master data (vendors/branches/vaults/PICs/packages/prices/ATMs/regions) is feature-complete for MVP. Only bugfixes allowed.
3. **Do not touch Phases 3–7** (finance, escrow, cash count, CIT, EOD) until Phase 2 lands, unless the user explicitly re-prioritizes.
4. **Do not wire mock screens** (see "Mock frontends") ad hoc — each gets wired only as part of its owning phase item.
5. New table/column → note it in `.claude/CLAUDE.md` Sec 3 when applied (schema is mutable in this stage, Sec 0a). Migrations are sequential and applied with `go run ./cmd/migrate up` (never `psql -f`). **Next free: `033`** (`022` reserved for `vendor-pks-cis-limit`, `024` dropped).
6. Accepted known gaps (listed below) are **not** to be "fixed along the way".

---

## Status snapshot (as of 2026-10-23)

Overall completion: **~55%** (rough estimate; was ~45-50% on 2026-10-08). "Done" = wired into a route, backed by real queries, tested — never a folder or a mock UI.

Branch: `dev1` is **144 commits ahead** of `main` (`main` at `87b2d78`; `dev1` latest `aad7de4`, vendor-party-scope task 12). Merge `dev1` → `main` is still pending. Uncommitted on `dev1`: vendor-party-scope task 4/9/10 hunks + `openapi.yaml` / `schema.d.ts` (staged), docs, and the new `app-shell-revamp` spec.

Migrations: baseline `001_baseline_schema.sql` + `002_baseline_seed.sql` (squashed 2026-09-18; old `001`–`040` in `backend/migrations/archives/2026-09-18_pre-baseline/`). Current range `001`–`032` (gaps: `022` reserved, `024` dropped), tracked in `schema_migrations` by the custom runner `backend/cmd/migrate` (BE-03, `88a3139`). Dev baselined `--through 031` on 2026-10-15; `032` applied 2026-10-20.

### Built and wired (verified in code)

| Area | Endpoint / location | Notes |
| --- | --- | --- |
| Auth | `/api/v1/auth` | Local + JWT, password lifecycle, absolute 1h session (`SESSION_MAX_LIFETIME`). Vendor roles rejected on CompanyPortal (login + refresh) |
| Approval | `/api/v1/approvals`, `/api/v1/admin/approval` | Maker-checker orchestrator, chain resolver, delegation/leave |
| Audit | `/api/v1/audit-logs` + `features/audit-log` | Append-only writer + viewer, read indexes (mig `006`) |
| Role management | `/api/v1/admin/roles` | APPACCESS/ADMIN. Immediate-apply + audit-in-tx (documented deviation, CLAUDE.md Sec 12) |
| Admin users | `/api/v1/admin/users` | **APPACCESS + ADMIN** (ADMIN_PARAM removed, vendor-party-scope D8). Vendor role + branch scope + `vendor-options` lookup |
| Master data (maker-checker) | `/api/v1/admin/vendors` (+ `/branches`, `/branches/{id}/atms`, `/vaults`, `/pics`, `/packages`, `/package-prices`, `/dsr-location-maps`), `/api/v1/admin/atms` (+ `/assignments`), `/api/v1/admin/master-data/{changes,export,import}` | All writes stage → 202 → apply-on-approve via `Applier`. CSV import/export (PIC `roles` column since mig `032`) |
| Regions | `/api/v1/admin/regions` + `features/admin-regions` | Immediate-apply + audit. Mig `018` |
| ACM areas | `/api/v1/admin/acm-areas` | ADMIN, area per region (cit-vault-select v3) |
| Holidays | `/api/v1/admin/holidays` | ADMIN, immediate-apply + audit-in-tx (GR#3 deviation). Mig `031` |
| ATM portal | `/api/v1/atm-portal` | Read-only monitoring (replica) |
| DMAA forecast | `/api/v1/dmaa-forecast` | Read-only viewer. CROWN has **no forecast engine**; DMAA `amount_replenish` is the final order amount |
| Forecast browser summary | `features/forecast` (summary view) | Vendor × region ringkasan (`7f6d16b`) |
| DSR (ATM) | `/api/v1/dsr` | Vendor DSR upload, two-phase dry-run/confirm; Python ETL `service_dsr_etl` |
| Vendor request | `/api/v1/vendor-requests` | DMAA → replenishment order, maker-checker state machine, vault review. OpenAPI contract pilot (`backend/api/openapi.yaml`) |
| Vault plans (ACM) | `/api/v1/vault-plans` | ACM-USER/ACM-SPV/ADMIN. Allocate request nominal to vaults + vault fulfillment ticket (migs `027`, `028`, `030`) |
| Vendor orders | `/api/v1/vendor/replenish-orders` | VendorPortal accept/reject per (request × branch), scoped per vendor party role (migs `029`, `032`) |
| Replenishment realisasi | `/api/v1/vendor/replenish-realizations`, `/api/v1/replenishment` + Python `replenish_classify/` | Realisasi vs order classification in working days, vendor report, CSV. Mig `031` |
| Notification (Phase 0.3) | `internal/notification`, `/api/v1/notifications` + CP bell + VP API | In-app + SMTP outbox worker (stdlib). Mig `025` |
| Import jobs + idempotency (Phase 0.2) | `import_jobs` (migs `019`/`020`) + Python helper + frontend | Idempotent per file hash; Go helper deferred to the first Go file ingest |
| EOD ETL (Python) | `backend_python/{dmaa,itm,eod_retry_scheduler,replenish_classify}` + `features/eod-monitoring` | This **is** the EOD runtime (decision 2026-09-25) |
| ATM visit quota / package source / replenish ticket | migs `021`, `023`, `026` | Merged features; some branch `region_code` still empty |
| Platform hardening | both backends + both portals | Read replica routing (0.1), `RequestLogger` + `/ready`, 1 MiB body limit + 30s timeout, trusted client IP, money as decimal strings, CSP headers, lazy routes, FE coverage gate (CP 84/80/80/85, VP 93/84/90/93), Go coverage gate, GitHub Actions CI |

### Not built
- **ATM ops:** 2.3 DSR late report, 2.4 problem-ATM exclusion. `internal/cashcount` (Phase 5).
- **Finance:** `internal/invoice`, `internal/reconciliation`.
- **Platform:** `internal/document` (deferred to Phase 3/5), `internal/export` XLSX/PDF (CSV only today; deferred).
- **Integration:** `internal/corebanking` (escrow ingest).
- **`backend-exq/`** (renamed from `backend-cit` 2026-10-20, `8b41612`): export + dashboard monitoring queries. Skeleton only, not deployed. CIT functions live in `backend/` (decision 2026-10-08).
- **EOD Final Realisasi** calc + `eod_runs` table + late-completion alert.
- **Vendor PKS / CIS limit** (`vendor-pks-cis-limit/`, mig `022` reserved): intent draft, awaiting PO.
- **Unmigrated tables:** `documents`, `export_jobs`, `invoice_*`, `*_reconciliation_results`, all `cit_*`, all `escrow_*`, `cash_count_schedules`/`cash_count_evidences` (names approved, not migrated). `replenishment_instructions` is dropped: an approved Vendor Request is the instruction.

### Mock frontends (hardcoded data, not API-backed)
Mock screens were removed from menus/routes with a "Data contoh" banner on the dashboard (FE-05, `d25ae2b`).
- **CompanyPortal:** `features/dashboard` (MetricStrip, AttentionPanel), `invoice`, `reconciliation`, `cit` (CitTracker), `cash-flow`, part of `dsr` (`useDsrData`).
- **VendorPortal:** schedule, invoices, evidence.
Each is wired only by its owning phase item.

### Accepted known gaps (do NOT fix unless asked)
- `atm_vendor_packages.vendor_package_id` isn't validated against the ATM's `price_machine_group`/`price_class` (user decision 2026-09-23).
- `vendor_packages_branch` and `atm_vendor_packages` were truncated by mig `011`; dev was re-seeded 2026-09-24. A fresh DB built from migrations alone still has them empty.
- `vendor_package_prices` seed is partial (no SSI Paket 3/6, no per-ATM Harga Khusus / Biaya Tambahan).
- `backend/internal/handler/integration_test.go` runs only `001`+`002`; `003`–`032` aren't exercised by it (CI `go-integration` does apply them through `cmd/migrate up`).
- `backend-exq` image still runs as root without HEALTHCHECK (BE-06 open item; not deployed).

---

## Sequencing strategy

**Close in-flight → infra → ATM ops.** Shared infrastructure first so business modules don't each re-invent async jobs, notifications, document storage, and replica reads. CIT functions are built in `backend/` as part of Phase 2.2.

Binding rules (from `project-context.md`):
- State-machine + DB-backed orchestration for long flows (`how_to_handle_flowprocess.md`), never one long sync endpoint.
- Maker-checker via the existing `approval.Orchestrator` — never a second state machine.
- Money = numeric/integer minor units (decimal strings on the wire). Writes → primary, reports/dashboards → replica.
- Every state change writes `audit_logs`. File ingests idempotent per file hash.
- New module starts with its OpenAPI spec (`backend/api/openapi.yaml`); existing uncovered modules stay hand-typed.

> **Numbering ≠ execution order.** Phase 1 (close in-flight) and Phase 0 (infra) ran *before* Phase 2. Numbers are kept stable because other docs reference them (e.g. "0.3", "2.1").

---

## Phase 0 — Infrastructure foundations ✅ DONE (0.4, 0.5 deferred)

| Item | Status |
| --- | --- |
| 0.1 Read-replica routing (Go + Python `db_read_pool`) | ✅ 2026-10-07. Kept on primary on purpose: DSR upload read-after-write, master-data change badge reads |
| 0.2 `import_jobs` + idempotency helper | ✅ Python helper + EOD (migs `019`/`020`). Go helper deferred to the first Go file ingest (Phase 3 invoice / 4 escrow) |
| 0.3 `internal/notification` (in-app + SMTP) | ✅ `eb457cf`, review R1–R3 fixed. Real SMTP send waits for relay creds |
| 0.4 `internal/document` | ⏸ deferred to Phase 3/5 (intent accepted, `.claude/sdlc/document/`) |
| 0.5 `internal/export` (XLSX/PDF) | ⏸ deferred; new deps (excelize, PDF tool) need approval (Golden Rule #1) |

> Async worker: dropped as a standalone item. Decide per-feature (goroutine + DB job table vs Redis queue vs Python pipeline).

---

## Phase 1 — Close in-flight work ✅ DONE (automated part)

| Item | Status |
| --- | --- |
| 1.1 Region management wrap-up | ✅ 2026-09-25, spec in `.kiro/specs/.done/` |
| 1.2 Vendor branch drill-down (`perbaikan-rbac/`) | ✅ filter contract + disable-with-children 409. Remaining: approve/reject/apply-failure + conflict tests, manual browser check (user) |
| 1.3 DSR upload re-verification | ✅ 2026-10-07. Remaining: T2 live-DB integration test, manual browser check (user) |
| 1.4 Dev data re-seed | ✅ 2026-10-07 (already re-seeded 2026-09-24) |

---

## Phase 2 — ATM operations (daily transactional core) ← **CURRENT**

### 2.1 Forecast / Order ATM — ✅ CLOSED 2026-10-08 (no engine)
- DMAA sends `Order_All_*.xlsx`; `backend_python/dmaa/dmaa_etl.py` loads `dmaa_files` + `dmaa_atm_forecast` (idempotent per SHA-256). DMAA `amount_replenish` **is the final order amount**.
- Built on top: DMAA viewer, Forecast Browser + vendor × region summary, Vendor Request (duplicate-order guard via `is_requested`), replenish ticket, visit quota.
- Dropped: `internal/forecast`, `forecast_runs` / `forecast_results`.

### 2.2 Replenishment (CIT functions in `backend/`)
Vendor Request `approved` = replenishment instruction. Flow end to end: `.claude/feature-flows/10-order-atm-lifecycle/feature-flow.md`.

#### 2.2a Penetapan vault oleh ACM — ✅ built (v1 `ef6dbfe`, v3 `94c4189`)
- v1 (`cit-acm-plan/`, migs `027`/`028`): ACM-USER/ACM-SPV roles, DSR location → vault mapping (maker-checker), ETL saves SALDO AKHIR per vault.
- **v3 (`cit-vault-select/`, mig `030`) replaces the per-ATM assignment:** ACM splits the request nominal across vaults (CASH / ATM_CASH), each allocation gets a vault fulfillment ticket, ACM area is per **region** (not per branch). Dev seed fixed in `e79e4f0`.
- Remaining: manual browser check (user), Docker build.
- **Model:** Opus, Effort: High.

#### 2.2b Kirim CIT approved ke vendor — ✅ built `42e7b08` (mig `029`)
- `cit-send-vendor/`: `ready` → `sent_to_vendor`; branch replenish + branch vault accept/reject in VendorPortal Orders; all accepted → `vendor_accepted`. Pickup flow out of scope.
- Remaining: manual browser check (user, `vendor.aba` on `REP-ABA-MDN-20261014-001`).

#### 2.2c Realisasi vs order — ✅ built `de5f3af` / `9a5af6d` (mig `031`)
- `replenishment-realisasi/`: Python `replenish_classify` job (after `itm_replenish`, cron 09:15 WIB, manual run) classifies on-schedule / early / late / not done / without order in working days (`holidays`); vendor reports date + amount; ITM wins; CSV reports. Review R1–R5 fixed.
- Remaining: manual browser check + ITM test (user), Docker build.
- **Model:** Opus, Effort: High.

#### 2.2d Pisah akun vendor per peran + cakupan cabang — 🟡 tasks 1–14 done (`950ffe1` … `aad7de4`, mig `032`)
- `.kiro/specs/vendor-party-scope/`: new roles `VENDOR-ATM-USER` / `VENDOR-VAULT-USER` (`VENDOR-USER` keeps both parties), per-branch scope on orders/realisasi/notifications, PIC roles (immediate + audit, GR#3 deviation), `/auth/me` vendor context.
- Automated checkpoint 2026-10-21 green (Go 2067 pass / 0 skip, CP 1425, VP 250).
- Remaining: commit the staged/unstaged hunks, manual browser check (user, `vendor.abacus.*` seed accounts).
- **Model:** Opus, Effort: High.

### 2.3 DSR late/missing report — ⏳ NEXT (intent draft 2026-09-28, `dsr-late-report/`)
- Monthly late/missing-DSR-per-vendor report (09:00 deadline, FLM penalty basis) + late notification.
- Needs PO acceptance of the intent first; includes a DSR per-branch schema change.
- **Depends on:** 0.3 ✅ (0.5 only if XLSX is required).
- **Model:** Sonnet, Effort: Medium.

### 2.4 Problem-ATM exclusion + FSD input lists — ⏳ not started (scoped 2026-10-08)
- CROWN (not DMAA) excludes/flags "problem" ATMs and handles the FSD v1.0 input lists (complaint/project/problem/adjustment) before a Vendor Request is created from DMAA rows.
- Needs a full AI-DLC chain (`.claude/sdlc/problem-atm-exclusion/`). Likely schema change.
- **Depends on:** 2.1 ✅, 0.2 ✅.

### Phase 2 decisions (user, 2026-10-08)
1. DMAA `amount_replenish` is final — no in-CROWN formula.
2. Refund: DMAA sends it **later**; `dmaa_etl.py` writes `amount_refund = 0` until the file gains a refund column.
3. Problem-ATM exclusion + FSD input lists stay with CROWN → 2.4.
4. CIT functions are built in `backend/`, not in a separate CIT backend.

---

## Cross-cutting items (not tied to a phase)

| Item | Status | Next |
| --- | --- | --- |
| X.1 CompanyPortal app-shell revamp (`.kiro/specs/app-shell-revamp/`) | 🟡 implementasi selesai 2026-10-23 (task 1–10: sidebar/topbar/Ctrl+K palette baru, font self-hosted, token `--shell-*`, `Header.tsx` dihapus; deviasi di `ui_design.md`); belum di-commit. Follow-up `.kiro/specs/sidebar-settings-footer/` (Pengaturan pindah dari Topbar ke footer Sidebar, EnvBadge dihapus; PO D2/D4 2026-10-27) sudah dibangun, belum di-commit | gate kualitas task 11 (lint, coverage 84/80/80/85, build + `check-bundle.mjs`) outstanding; cek browser manual (user, GR#10), termasuk follow-up footer: Expanded/Rail, ADMIN vs ATM-USER, tooltip Pengaturan di Rail |
| X.2 Tech debt FE (`fe-tech-debt/`, `FE_debt.md`) | 🟡 FE-01/02/03/04/05/07/08/09/10/11/12/13 done | manual browser checks (user); A5, B–E on hold |
| X.3 Tech debt BE (`be-tech-debt/`, `BE_debt.md`) | 🟡 Fase A–G done (CI, coverage gate, migrate runner, logging/`/ready`, limits, PyJWT, file splits) | first GitHub Actions run after push; GCP log-based alert (user); `backend-exq` image hardening |
| X.4 OpenAPI contract (BE-10, `openapi-contract/`) | ✅ pilot done `a1b78c9` (`/vendor-requests*`, `/vendor/replenish-orders*`) | extend per new module |
| X.5 CI move to Jenkins (`ci-jenkins/`) | ⏸ on hold (user 2026-10-15); GitHub Actions stays live | resume ~Dec 2026 |
| X.6 Merge `dev1` → `main` | ⏳ pending (144 commits) | after the open manual checks / code-owner review |

---

## Later phases — parked (do not start without explicit re-prioritization)

| Phase | Item | Gate |
| --- | --- | --- |
| 3 Finance | `internal/invoice` (validate + approve only, NO payment), `internal/reconciliation` | Phase 2 data, 0.4 |
| 4 Integration | `internal/corebanking` escrow batch ingest | 0.2, 3 |
| 5 Cash count | `internal/cashcount` (vault + selective machine) | Table names approved; columns in Phase 5 spec. 0.3, 0.4, 4 |
| 6 CIT remainder | Journal, CIT DSR, CIT reconciliation (in `backend/`); export/dashboard queries in `backend-exq` | ATM ops solid |
| 7 EOD | Extend Python pipeline: Final Realisasi calc, `eod_runs`, `success`-only reads, not-done-before-office-hours email | Phase 2, 0.3 |

---

## Resolved decisions

1. **EOD runtime** (2026-09-25): Python (`backend_python/`). No Go `cmd/batch`.
2. **Order ATM formula**: superseded — DMAA `amount_replenish` is final (2026-10-08).
3. **Branch `branch_id` filter + disable-with-children**: see Phase 1.2.
4. **Cash count tables**: `cash_count_schedules` + `cash_count_evidences` names approved; columns decided in the Phase 5 spec.
5. **CIT location** (2026-10-08): CIT functions in `backend/`; `backend-cit` renamed `backend-exq` for export + dashboard queries (2026-10-20).
6. **Vault assignment** (2026-10-13): nominal allocation per vault + ACM area per region (v3) replaces per-ATM vault assignment (v1).
7. **Migrations** (2026-10-15): custom runner `backend/cmd/migrate`; golang-migrate not used.

---

## Dependency graph

```
Phase 1 ─> Phase 0 ─> Phase 2 (2.1 ✅ ─> 2.2a ✅ ─> 2.2b ✅ ─> 2.2c ✅ ─> 2.2d 🟡 ; 2.3 ⏳ ; 2.4 ⏳)
                         │
                         ├─> 3 Finance ─> 4 Escrow ─> 5 Cash count
                         ├─> 7 EOD
                         └─> 6 CIT remainder
```

---

## Working conventions

1. Spec first for non-trivial work: `.kiro/specs/<feature>/` (requirements → design → tasks) or `.claude/sdlc/<feature>/`. Finished specs move to `.kiro/specs/.done/` / `.claude/sdlc/.done/`.
2. Every task carries **`Model:` + `Effort:`** (`model-recommendation.md`).
3. Tests ship with every feature; coverage ≥80% on `internal/*`, FE thresholds enforced by `pnpm run test:coverage`. Manual browser verification is done by the user (Golden Rule #10).
4. When an item lands: update the **Status snapshot** here, `.claude/development-progress.md`, and CLAUDE.md Sec 3/12 if schema changed.
5. Keep "done" honest: wired + DB-backed + tested.
