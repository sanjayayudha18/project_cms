# CMS Development Plan (Kiro Steering Doc)

> Roadmap for the remaining CMS work, sequenced against the module & table map in `project-context.md` / `.claude/CLAUDE.md` Sec 3. This is a **living plan** — update it when a module lands, a table is approved, or a dependency shifts.
> **Companion docs:** `project-context.md` (source of truth for modules/tables/rules), `tech.md` (fixed stack), `structure.md` (layout), `model-recommendation.md` (per-task Model/Effort convention), `.claude/development-progress.md` (feature-level changelog).

---

## ⛔ Scope guard — read before starting ANY task

**Current focus (in order):** Phase 2 (ATM ops; next: 2.2a rencana CIT ACM — 2.1 closed 2026-10-08, CROWN builds no forecast engine). Phase 0 (infra) and Phase 1 (close in-flight) are **done** — only their manual browser checks remain, which are the user's job.
Anything not listed under Phase 2 is **out of scope right now**. If a request touches it, STOP and ask.

Rules:
1. **One task = one item in this file.** If the work isn't an item here, add it here first (with user OK), then build it.
2. **No new master-data features.** Master data (vendors/branches/vaults/PICs/packages/prices/ATMs/regions) is feature-complete for MVP. Only bugfixes allowed.
3. **Do not touch Phases 3–7** (finance, escrow, cash count, CIT, EOD) until Phase 2 lands, unless the user explicitly re-prioritizes.
4. **Do not wire mock screens** (see "Mock frontends") ad hoc — each gets wired only as part of its owning phase item.
5. New table/column → note it in `.claude/CLAUDE.md` Sec 3 when applied (schema is mutable in this stage, Sec 0a). Migrations are sequential — **next free: `027`** (019–021, 023, 025, 026 applied; 022 reserved for `vendor-pks-cis-limit`, 024 was dropped — see Phase 1.4).
6. Accepted known gaps (listed below) are **not** to be "fixed along the way".

---

## Status snapshot (as of 2026-10-08)

Overall completion: **~45-50%**. "Done" = wired into a route, backed by real queries, tested — never a folder or a mock UI.

Branch: `dev1` is ahead of `main` (`main` at `87b2d78`; all of Phase 0 + replenish ticket live on `dev1`, latest `291f488`). Merge `dev1` → `main` is pending.

Migrations were squashed on 2026-09-18 into `001_baseline_schema.sql` + `002_baseline_seed.sql` (old `001`–`040` in `backend/migrations/archives/2026-09-18_pre-baseline/`). Current range: `001`–`026` (gaps: `022` reserved, `024` dropped). Next free: `027`.

### Built and wired (verified in code)

| Area | Endpoint / location | Notes |
| --- | --- | --- |
| Auth | `/api/v1/auth` | LDAP + local, JWT, password lifecycle, absolute 1h session (`SESSION_MAX_LIFETIME`) |
| Approval | `/api/v1/approvals`, `/api/v1/admin/approval` | Maker-checker orchestrator, chain resolver, delegation/leave |
| Audit | `/api/v1/audit-logs` + viewer frontend (`features/audit-log`) | Append-only writer + viewer, read indexes (mig `006`) |
| Role management | `/api/v1/admin/roles` | Immediate-apply + audit-in-tx (documented deviation, CLAUDE.md Sec 12) |
| Admin users | `/api/v1/admin/users` | APPACCESS, immediate apply |
| Master data (maker-checker) | `/api/v1/admin/vendors` (+ `/branches`, `/branches/{id}/atms`, `/vaults`, `/pics`, `/packages`, `/package-prices`), `/api/v1/admin/atms` (+ `/assignments`), `/api/v1/admin/master-data/{changes,export,import}` | All writes stage → 202 → apply-on-approve via `Applier`. CSV import/export. Migrations `003`–`017` |
| Regions | `/api/v1/admin/regions` + `features/admin-regions` | Immediate-apply + audit. Mig `018` applied; spec in `specs/done/` |
| ATM portal | `/api/v1/atm-portal` | Read-only monitoring |
| DMAA forecast | `/api/v1/dmaa-forecast` | Read-only viewer of DMAA data. CROWN has **no forecast engine** — DMAA `amount_replenish` is the final order amount (user 2026-10-08) |
| DSR (ATM) | `/api/v1/dsr` | Vendor DSR upload, two-phase dry-run/confirm; Python ETL in `backend_python/service_dsr_etl` |
| Vendor request | `/api/v1/vendor-requests` | DMAA → vendor replenishment request, maker-checker state machine |
| EOD ETL (Python) | `backend_python/{dmaa,itm,eod_retry_scheduler}` + `features/eod-monitoring` | Python ETL + FastAPI retry scheduler + monitoring page — this **is** the EOD runtime (decision 2026-09-25, CLAUDE.md Sec 14) |
| Import/export jobs + idempotency (Phase 0.2) | `import_jobs` (mig `019`/`020`) + Python helper + frontend | Idempotent per file hash; Go helper deferred to the first Go file ingest (Phase 3 invoice / 4 escrow). Over-quota notification = first consumer |
| Notification (Phase 0.3) | `internal/notification`, `/api/v1/notifications` + CompanyPortal bell + VendorPortal API | In-app + SMTP outbox worker (stdlib). Mig `025`. `eb457cf`, review R1–R3 fixed |
| ATM visit quota | mig `021` | Per-ATM replenish visit quota (`atm-visit-quota/`), merged |
| ATM package source (branch / vendor-wide) | mig `023` | Kelolaan ATM paket khusus (`atm-package-source/`), merged |
| Replenish ticket per ATM | mig `026` | Nomor tiket replenish (`replenish-ticket/`); some `region_code` still empty |
| Forecast browser summary | `features/forecast` (summary view) | Vendor × region ringkasan viewer (`forecast-browser-summary/`), merged `7f6d16b` |

### Partial
- **Read replica (Phase 0.1 — ✅ done 2026-10-07)** — `dbReadPool` in `cmd/api/main.go` now routes ATM portal, DMAA viewer, role mgmt, RBAC lists, audit viewer; vendor/ATM admin repos split (List/Count → replica, GetByID/pre-check → primary). Python `db_read_pool` for EOD monitoring APIs. Deliberately kept on primary: DSR upload read-after-write, master-data change badge reads.

### Not built
- **ATM ops engines:** `internal/replenishment`, `internal/cashcount` (`internal/forecast` dropped 2026-10-08 — forecast comes from DMAA, see 2.1)
- **Finance:** `internal/invoice`, `internal/reconciliation`
- **Platform:** `internal/document` (deferred to Phase 3/5), `internal/export` (XLSX/PDF — CSV exists for master data only; deferred)
- **Integration:** `internal/corebanking` (escrow ingest)
- **CIT backend (`backend-cit/`):** `/health` + auth-protected empty group only; every `internal/*` package is a stub file
- **EOD Final Realisasi** calc + run table + late-completion alert (Python pipeline has ETL + retry only)
- **Unmigrated tables:** `documents`, `export_jobs`, `replenishment_instructions`, `invoice_*`, `*_reconciliation_results`, all `cit_*`, all `escrow_*`, `cash_count_schedules`/`cash_count_evidences` (names approved, not migrated)

### Mock frontends (hardcoded data, not API-backed)
`features/dashboard` (MetricStrip, AttentionPanel, ReplenishmentSummary) · `features/cit` · `features/cash-flow` · `features/forecast` · `features/invoice` · `features/reconciliation` · `features/replenishment`. Each is wired only by its owning phase item.

### Accepted known gaps (do NOT fix unless asked)
- `atm_vendor_packages.vendor_package_id` isn't validated against the ATM's `price_machine_group`/`price_class` (user decision 2026-09-23).
- `vendor_packages_branch` and `atm_vendor_packages` were truncated by mig `011`; dev was re-seeded 2026-09-24 (Phase 1.4 ✅). A fresh DB built from migrations alone still has them empty.
- `vendor_package_prices` seed is partial (no SSI Paket 3/6, no per-ATM Harga Khusus / Biaya Tambahan).
- `backend/internal/handler/integration_test.go` runs only `001`+`002`; `003`–`026` aren't exercised by it.

---

## Sequencing strategy

**Close in-flight → infra → ATM ops.** Shared infrastructure first so business modules don't each re-invent async jobs, notifications, document storage, and replica reads. CIT stays in its own late phase (low load, CLAUDE.md Sec 10).

Binding rules (from `project-context.md`):
- State-machine + DB-backed orchestration for long flows (`how_to_handle_flowprocess.md`), never one long sync endpoint.
- Maker-checker via the existing `approval.Orchestrator` — never a second state machine.
- Money = numeric/integer minor units. Writes → primary, reports/dashboards → replica.
- Every state change writes `audit_logs`. File ingests idempotent per file hash.

> **Numbering ≠ execution order.** Phases are listed by number, but Phase 1 (close in-flight) and Phase 0 (infra) ran *before* Phase 2. Both are now done (bar manual browser checks); **active work is Phase 2**. Numbers are kept stable because other docs reference them (e.g. "0.3", "2.1").

---

## Phase 0 — Infrastructure foundations

### 0.1 Finish read-replica routing — ✅ DONE 2026-10-07 (see `.claude/bugfixes.md`)
- Replace remaining `ponytail:` dbRead TODOs in `cmd/api/main.go` (roles, atm-portal, dsr List/Count). Pool already exists.
- Python side: add a replica pool (`DATABASE_REPLICA_URL`, fallback to primary) in `backend_python/lib/database.py` and use it for the read-only monitoring APIs (`/status`, `/summary`, `/late`, `/audit`) of `eod_retry_scheduler` + `service_dsr_etl`. Deferred here from `.claude/sdlc/import-export-jobs/spec.md` decision C2 (2026-09-28).
- **Test:** replica on reads, primary on writes + read-after-write.
- **Model:** Sonnet, Effort: Low.

### 0.2 `import_jobs` / `export_jobs` + idempotency helper — ✅ done (Python helper + EOD; Go helper deferred to the first Go file ingest; T8 review pending)
- Migration `019`+: status, `file_hash` unique, processing_date, counts, error. Shared helper enforcing idempotency per file hash. Reconcile with existing `master_data_import_batches` (SHA-256) and `dsr_uploads` rather than duplicating.
- **Unblocks:** every file ingest (invoice, escrow, forecast inputs).
- **Model:** Opus, Effort: High.

### 0.3 `internal/notification` (in-app + SMTP) — ✅ built 2026-10-07 (stage 5 review done, R1–R3 fixed; merge + manual browser check outstanding) (`.claude/sdlc/notification/`)
- `notifications` table. In-app write + company SMTP relay. Needed by DSR-late alert, replenishment publish, cash count.
- **Model:** Sonnet, Effort: Medium.

### 0.4 `internal/document` (storage) — ⏸ deferred to Phase 3/5 2026-10-07 (intent accepted, `.claude/sdlc/document/`)
- `documents` table. Upload → GCS (local dir in dev); metadata in DB. Reused by invoice, cash count, escrow.
- **Model:** Sonnet, Effort: Medium.

### 0.5 `internal/export` (XLSX/PDF) — ⏸ deferred 2026-10-07 (user): no XLSX/PDF consumer yet; decide deps when a Phase 2 report needs it
- Extend beyond master-data CSV. **New deps (excelize, PDF tool) need approval first** (Golden Rule #1).
- **Model:** Sonnet, Effort: Medium.

> Async worker: dropped as a standalone item. Decide per-feature (goroutine + DB job table vs Redis queue) when a feature needs it — no new dependency until then.

---

## Phase 1 — Close in-flight work ✅ DONE (automated part; manual browser checks remain with the user)

### 1.1 Region management wrap-up — ✅ DONE 2026-09-25
- Mig `018` confirmed applied on dev. Integration tests (14) green against real Postgres; added rapid property tests (4.3/4.4/4.9), component tests (9.3), hub + route-guard tests (10.3). Fixed `SettingsHubPage.test.tsx`, which had been broken since the UI revamp (master cards moved behind the "Data master" tab). Spec moved to `.kiro/specs/done/`.

### 1.2 Vendor branch drill-down verification (`.claude/sdlc/perbaikan-rbac/plan.md`)
- CRUD shipped in `f11bc40`; 17 checklist items still unverified. Decided 2026-09-25:
  - **Filter contract — ✅ already built:** optional `branch_id` on vault/PIC/package list + count. PIC vendor-wide scope uses the existing `vendor_wide_only=true` (same semantics as the agreed `scope=vendor`; kept as-is to avoid API churn — `scope=all` = no param, `scope=branch` = `branch_id`).
  - **Disable branch with active children — ✅ done 2026-09-25:** 409 at submit (existing) + re-check at apply (`ensureBranchHasNoActiveChildren`, new) mapped to 409 in the approval handler; integration test added. No cascade.
  - **Remaining:** the other checklist items in that plan are UI/flow checks → **manual browser verification by the user** (Golden Rule #10).
- Only verify + fix gaps listed there. No new UI.
- **Model:** Sonnet, Effort: Medium.

### 1.3 DSR upload re-verification (`.claude/sdlc/vendor-upload-dsr/`) — ✅ DONE 2026-10-07 (automated part)
- Every `tests.md` case re-checked against the two-phase flow: T1/T3/T5/T7 pass, T4 removed (out of scope), T6 obsolete. New tests: Go vendor-scoping (T5) + Python negative-amount (T7).
- **Remaining:** T2 live-DB integration test (same/changed checksum) and the manual browser check (user, Golden Rule #10).

### 1.4 Dev data re-seed — ✅ DONE 2026-10-07
- Checked dev DB 2026-10-07: both tables were already re-seeded on 2026-09-24 (`vendor_packages_branch` 93 rows, `atm_vendor_packages` 1,163 active rows, all branch mode). No seed migration needed; the drafted `024_seed_dev_vendor_packages_branch.sql` was dropped (it would have tripped `atm_vendor_packages_no_overlap` against the existing rows).

---

## Phase 2 — ATM operations (daily transactional core) ← **CURRENT**

### 2.1 Forecast / Order ATM — ✅ CLOSED 2026-10-08 (no engine; re-scoped by user)
- **CROWN does no forecasting.** The DMAA team sends `Order_All_*.xlsx`; `backend_python/dmaa/dmaa_etl.py` loads it into `dmaa_files` + `dmaa_atm_forecast` (idempotent per SHA-256). DMAA `amount_replenish` **is the final order amount** — CROWN does not recompute it.
- Already built on top of it: DMAA viewer (`/api/v1/dmaa-forecast`), Forecast Browser + vendor × region summary, Vendor Request (DMAA → replenishment order, maker-checker, duplicate-order guard via `is_requested`), replenish ticket per ATM, visit quota.
- **Dropped:** `internal/forecast`, `forecast_runs` / `forecast_results` tables (never migrated), the in-CROWN formula `Forecast Amount − Saldo DSR + Forecast Refund`.
- **Open questions (user, 2026-10-08):** see "Open questions" under Phase 2.

### 2.2 Replenishment — split 2026-10-08 (user) into 2.2a → 2.2b → 2.2c
Vendor Request `approved` = replenishment instruction (no `replenishment_instructions` table). CIT functions live in `backend/` (not `backend-cit`).

#### 2.2a Penetapan branch vault (penyedia uang) per ATM oleh ACM — ✅ committed `ef6dbfe`
- `.claude/sdlc/cit-acm-plan/` (intent + spec revisi 2, 2026-10-08). Flow: ATM-USER request → ATM-SPV approve → request `vault_assignment` → dipecah per Area ACM (branch replenish) → ACM-USER pilih **satu branch vault per ATM** (rekomendasi: vendor sama + region sama → vendor lain region sama → urgent lintas region) dengan saldo DSR + kapasitas sebagai **peringatan** → ACM-SPV approve → ATM-SPV/BRANCH-ATM-SPV approve → request `ready`. Fase berhenti di sini.
- Branch replenish (pelaksana, kelolaan) vs branch vault (penyedia uang) mulai dibedakan. New: roles ACM-USER/ACM-SPV, Area ACM (ADMIN, immediate + audit), DSR location → vault mapping (maker-checker), ETL saves SALDO AKHIR per vault. No pindah buku/escrow.
- **Depends on:** Vendor Request + tickets ✅, DSR ✅, 0.3 ✅. **Model:** Opus, Effort: High.

#### 2.2b Kirim CIT approved ke vendor — ✅ built 2026-10-09 (stage 4, uncommitted; review + manual browser check next)
- `.claude/sdlc/cit-send-vendor/` (intent draft 2026-10-09): `ready` → auto `sent_to_vendor`; branch replenish + branch vault see it in VendorPortal Orders, accept/reject per (request × branch); all accepted → `vendor_accepted` (gate laporan selesai). Pickup flow out of scope.
- **Depends on:** 2.2a.

#### 2.2c Realisasi vs order (was 2.2 intent)
- `.claude/sdlc/replenishment-realisasi/intent.md` (Q1–Q14 answered 2026-10-08): daily classification `itm_replenish` vs ticket `replenish_date` in working days (new `holidays`, ADMIN, immediate-apply + audit); vendor reports date + amount via VendorPortal (replaces ATM-USER completion input, SPV approves); ITM wins; vendor sees own results; reports amount≠order + trip/realisasi, CSV. Wire mock `ReplenishmentScreen.tsx` + dashboard `ReplenishmentSummary`.
- **Depends on:** 2.2a, 2.2b.
- **Model:** Opus, Effort: High.

### 2.3 DSR late/missing report
- Monthly late/missing-DSR-per-vendor report (09:00 deadline, FLM penalty basis) + late notification.
- **Depends on:** 0.3 (0.5 for XLSX).
- **Model:** Sonnet, Effort: Medium.

### 2.4 Problem-ATM exclusion + FSD input lists — ⏳ not started (scoped 2026-10-08)
- User decision: CROWN (not DMAA) must exclude/flag "problem" ATMs and handle the FSD v1.0 input lists (complaint/project/problem/adjustment) before a Vendor Request is created from DMAA rows.
- Needs a full AI-DLC chain (`.claude/sdlc/problem-atm-exclusion/`): where problem status comes from, upload format, effect on Forecast Browser / Vendor Request. Likely schema change.
- **Depends on:** 2.1 ✅, 0.2 ✅ (idempotent upload).

### Phase 2 decisions (user, 2026-10-08)
1. DMAA `amount_replenish` is final — no in-CROWN formula (CLAUDE.md Sec 3a updated).
2. Refund: DMAA will send it **later**; `dmaa_etl.py` keeps writing `amount_refund = 0` until the file format gains a refund column — then extend the ETL (small change, re-check `REQUIRED_COLUMNS`).
3. Problem-ATM exclusion + FSD input lists stay with CROWN → item 2.4.
4. CLAUDE.md Sec 1/3/3a/7/14 corrected: `internal/forecast`, `forecast_runs`/`forecast_results` removed.

---

## Later phases — parked (do not start without explicit re-prioritization)

| Phase | Item | Gate |
| --- | --- | --- |
| 3 Finance | `internal/invoice` (validate + approve only, NO payment), `internal/reconciliation` | Phase 2 data, 0.4 |
| 4 Integration | `internal/corebanking` escrow batch ingest | 0.2, 3 |
| 5 Cash count | `internal/cashcount` (vault + selective machine) | Table names approved; column design in Phase 5 spec. 0.3, 0.4, 4 |
| 6 CIT | `backend-cit`: orders, journal, CIT DSR, CIT reconciliation | ATM ops solid |
| 7 EOD | Extend Python pipeline: Final Realisasi calc, run table (`eod_runs`), `success`-only reads, not-done-before-office-hours email | Phase 2, 0.3 |

---

## Resolved decisions (2026-09-25)

1. **EOD runtime:** stays Python (`backend_python/`). No Go `cmd/batch`. CLAUDE.md Sec 14 updated.
2. **Order ATM formula:** valid as written (CLAUDE.md Sec 3a). *Superseded 2026-10-01 by the FSD v1.0 formula — see Sec 3a.*
3. **Branch `branch_id` filter + disable-with-children:** see Phase 1.2.
4. **Cash count tables:** `cash_count_schedules` + `cash_count_evidences` names approved into CLAUDE.md Sec 3; columns + any extra tables decided in the Phase 5 spec. Phase stays parked.

---

## Dependency graph

```
Phase 1 (close in-flight) ─> Phase 0 (infra) ─> Phase 2 (ATM ops) ─┬─> 3 Finance ─> 4 Escrow ─> 5 Cash count
                                                                    ├─> 7 EOD
                                                                    └─> 6 CIT
```

---

## Working conventions

1. Spec first for non-trivial work: `.kiro/specs/<feature>/` (requirements → design → tasks) or `.claude/sdlc/<feature>/plan.md`. Finished specs move to `.kiro/specs/done/`.
2. Every task carries **`Model:` + `Effort:`** (`model-recommendation.md`).
3. Tests ship with every feature; coverage ≥80% on `internal/*`. Manual browser verification is done by the user (Golden Rule #10).
4. When an item lands: update the **Status snapshot** here, `.claude/development-progress.md`, and CLAUDE.md Sec 3/12 if schema changed.
5. Keep "done" honest: wired + DB-backed + tested.
