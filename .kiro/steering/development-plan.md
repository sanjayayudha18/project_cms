# CMS Development Plan (Kiro Steering Doc)

> Roadmap for the remaining CMS work, sequenced against the module & table map in `project-context.md` Sec 2. This is a **living plan**, not a contract — update it when a module lands, a table is approved, or a dependency shifts.
> **Companion docs:** `project-context.md` (source of truth for modules/tables/rules), `tech.md` (fixed stack), `structure.md` (layout), `model-recommendation.md` (per-task Model/Effort convention).

---

## Status snapshot (as of 2026-09-17)

Overall completion: **~35-40%**. Measured by reading actual wired routes, migrations (latest is `039`), real DB-backed modules, and specs — **not** folders or mock UIs. "Done" = wired into a route, backed by real queries, tested.

### Built and wired (verified in code)

| Area | Module / endpoint | Notes |
| --- | --- | --- |
| Auth | `internal/auth` (`/api/v1/auth`) | LDAP + local login, JWT, password lifecycle (migrations 026-027), APPACCESS role |
| Approval | `internal/approval` (`/api/v1/approvals`, `/api/v1/admin/approval`) | Maker-checker orchestrator, chain resolver, delegation/leave (migrations 021-025) |
| Audit | `internal/audit` + audit-log viewer backend (`/api/v1/audit-logs`) | Append-only writer (migrations 023, 029); viewer frontend Task 7-8 open |
| Admin users | `/api/v1/admin/users` | APPACCESS-guarded CRUD |
| Admin vendors | `/api/v1/admin/vendors` | ADMIN/ADMIN_PARAM CRUD |
| Admin ATMs | `/api/v1/admin/atms` | `admin-atm-management` spec complete (Tasks 0-10) |
| ATM portal | `/api/v1/atm-portal` | Read-only monitoring |
| DMAA forecast | `/api/v1/dmaa-forecast` | Read-only **viewer** — not the forecast engine |
| DSR (ATM) | `/api/v1/dsr` | ATM DSR upload path |
| Vendor request | `/api/v1/vendor-requests` | DMAA → CIT replenishment request, maker-checker state machine |

### Not built (from Sec 2 map)

- **ATM ops engines:** `internal/replenishment`, `internal/forecast` (H+2 engine), `internal/cashcount`
- **Finance:** `internal/invoice`, `internal/reconciliation`
- **Platform:** `internal/document`, `internal/notification`, `internal/export`
- **Integration:** `internal/corebanking` (escrow ingest)
- **Entire CIT backend (`backend-cit/`):** `cit`, `journal`, CIT `dsr`, CIT `reconciliation`, `integration` — all empty skeletons, `/health` only
- **Infrastructure:** read-replica pool (`dbRead`), `cmd/batch` EOD runner + monitoring, asynq worker
- **Mock frontends not API-backed:** invoice (`invoices.json`), reconciliation (`reconciliation-exceptions.json`), replenishment (`replenishment-schedules.json`)
- **Unmigrated tables:** `documents`, `notifications`, `import_jobs`, `export_jobs`, `forecast_*` (beyond viewer), `invoice_*`, `*_reconciliation_results`, all `cit_*`, all `escrow_*`, `cash_count_*`

> Caveat: several "done" backend items were verified via httptest/unit/integration tests but **not against a live DB** (external Postgres unreachable at `host.docker.internal`). Migrations 026/027 in particular are noted "not yet verified against a live DB."

---

## Sequencing strategy

**Dependency-first.** Build the shared infrastructure that many business modules depend on before the modules themselves — this avoids each module re-inventing async processing, notifications, document storage, and replica reads. CIT gets its **own dedicated phase** once ATM operations are solid (it's low-load and not urgent per `project-context.md` Sec 9).

Rules that bind every phase (from `project-context.md`):
- State-machine + DB-backed orchestration for long flows (`how_to_handle_flowprocess`), never one long sync endpoint.
- Maker-checker via the existing `approval.Orchestrator` — never a second state machine (Sec 2 "Approval integration pattern").
- Money = numeric/integer minor units. Writes → primary, reports/dashboards → replica.
- Every state change writes `audit_logs`. File ingests idempotent per file hash.
- New table/column → propose in `project-context.md` Sec 2 **first**, get approval, then migrate. Migrations are additive, numbered sequentially (**next free: `040`**).

---

## Phase 0 — Infrastructure foundations

Prerequisite plumbing that later phases assume exists.

### 0.1 Read-replica pool wiring
- Wire `pkg/database` to expose `db` (primary) + `dbRead` (replica) from `DATABASE_URL` / `DATABASE_REPLICA_URL`. Replace the `ponytail:` TODO comments in `cmd/api/main.go` on read-only List/Get endpoints.
- **Depends on:** nothing.
- **Test:** replica routing on reads, primary on writes + read-after-write.
- **Model:** Sonnet, Effort: Medium — wiring across existing repos; a wrong pool choice on a write is a correctness bug but caught by tests.

### 0.2 `import_jobs` / `export_jobs` tables + idempotency helper
- Migration `040` (import_jobs) + `041` (export_jobs): status, file_hash unique, processing_date, counts, error. Shared `ImportJob` helper enforcing idempotency per file hash.
- **Depends on:** nothing. Unblocks every file-ingest module (DSR summarize, invoice, escrow).
- **Model:** Opus, Effort: High — data migration + idempotency contract that every ingest relies on; getting the unique key wrong corrupts re-run safety.

### 0.3 asynq worker entrypoint
- New `cmd/worker` entrypoint + Redis-backed asynq server; task registration skeleton. Wire into `docker-compose.yml`.
- **Depends on:** nothing.
- **Model:** Sonnet, Effort: Medium — standard worker bootstrap within the fixed stack.

### 0.4 `internal/notification` (in-app + SMTP)
- Migration `042` (`notifications` table). Service with in-app write + SMTP relay send (company relay, Sec 8). asynq task for async send.
- **Depends on:** 0.3.
- **Model:** Sonnet, Effort: Medium — SMTP + table-backed inbox; no money/auth risk.

### 0.5 `internal/document` (storage)
- Migration `043` (`documents` table). Upload → GCS bucket (Sec 9 #5) / local dir in dev; metadata in DB. Reused by cash count evidence, invoice supporting docs, escrow files.
- **Depends on:** nothing.
- **Model:** Sonnet, Effort: Medium — file storage abstraction; watch for path/hash handling.

### 0.6 `internal/export` (CSV/XLSX/PDF)
- excelize for XLSX, Typst CLI for PDF. Export job tracked in `export_jobs` (0.2), generated async (0.3).
- **Depends on:** 0.2, 0.3.
- **Model:** Sonnet, Effort: Medium — templated generation within known libraries.

---

## Phase 1 — Finish in-flight work

Close what's already started before opening new fronts.

### 1.1 Audit-log viewer frontend (Tasks 7-8)
- `AuditFilterBar` + `AuditLogTable` + detail drawer, per `.kiro/specs/audit-log-viewer/tasks.md`.
- **Depends on:** nothing (backend done).
- **Model:** Sonnet, Effort: Medium — data table + filters, established pattern.

### 1.2 Live-DB verification pass
- Apply migrations `021`-`039` against a reachable Postgres; confirm APPACCESS seed (027), password-policy columns (026), and audit indexes. Record results in `project-context.md`.
- **Depends on:** reachable DB.
- **Model:** Sonnet, Effort: Medium — verification + triage of any migration drift.

---

## Phase 2 — ATM operations (the daily transactional core)

The `cmd/api` office-hours flows operators use every day.

### 2.1 `internal/forecast` (H+2 engine)
- Migrations for `forecast_runs` / `forecast_results` (propose in Sec 2 first — currently viewer-only). Compute `Order ATM = (Saldo DSR + Proyeksi Refund) − (Rekomendasi DMAA + Rencana Isi Hari-H)` (Sec 2a). Exclude "problem" ATMs; duplicate-order prevention per period. Run as a job (0.3).
- **Depends on:** 0.2, 0.3; DSR data.
- **Model:** Opus, Effort: High — money-adjacent formula + scheduling rules; wrong output drives wrong replenishment.

### 2.2 `internal/replenishment`
- Migration for `replenishment_instructions`. Instruction lifecycle as a state machine (`how_to_handle_flowprocess`); 4-category holiday-adjusted classification (on-schedule/early/late/not-done). Maker-checker via orchestrator. Wire the mock `ReplenishmentScreen.tsx` to the real API.
- **Depends on:** 2.1.
- **Model:** Opus, Effort: High — state machine + maker-checker + classification rules.

### 2.3 DSR summarize + late/missing report
- Extend DSR ingest to persist raw rows separately from summaries, dedupe per file hash (0.2), and produce the monthly late/missing-DSR-per-vendor report (09:00 deadline, FLM penalty basis, Sec 2a).
- **Depends on:** 0.2, 0.6.
- **Model:** Sonnet, Effort: Medium — ingest + reporting on an existing upload path.

---

## Phase 3 — Finance (money — golden-rule danger zone)

### 3.1 `internal/invoice` (upload + validate + approve)
- Migrations `invoice_uploads` / `invoice_items` / `invoice_reconciliation_results`. Vendor uploads invoice + supporting docs (0.5); internal team uploads ATM master (price, trip package, VIP/Industri/Regular). Auto-reconcile; manual adjust against vendor sanggahan. **Validate & approve only — NO payment execution** (Sec 4). Maker-checker via orchestrator. Wire mock `InvoiceFlow.tsx`.
- **Depends on:** 0.5, Phase 2 (executed replenishment/CIT to validate against).
- **Model:** Opus, Effort: High — money reconciliation + approval + dispute handling.

### 3.2 `internal/reconciliation` (ATM)
- Migration `*_reconciliation_results`. Reproducible, explainable deltas (store inputs + deltas, Sec 5). Wire mock `ReconciliationScreen.tsx`.
- **Depends on:** Phase 2 data.
- **Model:** Opus, Effort: High — money math + reproducibility contract.

---

## Phase 4 — Integration (escrow)

### 4.1 `internal/corebanking` (escrow batch ingest)
- Migrations `escrow_batch_files` / `escrow_batch_rows` / `escrow_reconciliation_results`. Ingest → parse rows → reconcile vs CMS cash position → store deltas. **Idempotent per file hash** (Sec 4). Raw rows separate from summaries.
- **Depends on:** 0.2, 3.2 (reconciliation patterns).
- **Model:** Opus, Effort: High — file ingest + money reconciliation, idempotency-critical.

---

## Phase 5 — Cash count (pending table approval)

> Cash count is confirmed in-scope (URS v0.3 Rev1) but its **module/tables are a proposal pending approval** (Sec 3a/12). **Do not start until `cash_count_schedules` / `cash_count_evidences` are approved into Sec 2.**

### 5.1 `internal/cashcount` (vault + selective machine)
- Risk-category scheduling from escrow (SIBS/MIS) balance; PIC email accept/reject → surat tugas; digital Berita Acara with DSR auto-fill; photo evidence (0.5); dual e-sign; monthly 3-way reconciliation (cash count vs escrow H-1 vs proofing).
- **Depends on:** table approval, 0.4, 0.5, 4.1.
- **Model:** Opus, Effort: High — multi-step human+file workflow + reconciliation + e-sign.

---

## Phase 6 — CIT backend (`backend-cit/`)

> Dedicated phase. `backend-cit` is currently a pure skeleton (`/health` only). Uses `pkg/response` envelope. All CIT tables owned by `backend/migrations/`. `backend-cit` validates JWTs, never issues them.

### 6.1 CIT `internal/cit` (orders + handover)
- Migrations `cit_orders` / `cit_handover_evidences`. Order lifecycle state machine; handover evidence via document store (0.5).
- **Model:** Opus, Effort: High — order state machine feeding money flows.

### 6.2 CIT `internal/journal`
- Migration `cit_journals`. Journal posting (initial response ≤5s, async confirm ≤2min, Sec 2a NFR).
- **Model:** Opus, Effort: High — money/journal danger zone.

### 6.3 CIT `internal/dsr` + vendor-portal upload
- Migration `cit_dsr_uploads`. `Vendor CIT` uploads, `Vendor CIT Supervisor` approves (supervisor != uploader on the gate, Sec 4). Wire VendorPortal-Vite upload flow.
- **Model:** Sonnet, Effort: Medium — upload + supervisor approval gate.

### 6.4 CIT reconciliation (order vs DSR vs journal)
- Migration `cit_reconciliation_results`. 3-way match, reproducible deltas.
- **Model:** Opus, Effort: High — money reconciliation.

---

## Phase 7 — EOD batch + monitoring

### 7.1 `cmd/batch` EOD runner
- Second entrypoint sharing `internal/*` (Sec 10). Compute `Final Realisasi = rekomendasi DMAA − (saldo DSR + refund horizon H-2)` per vendor. Write to DB (source of truth); `eod_runs` / `forecast_runs` run table (running/success/failed). Transactional module reads only `success` runs. Idempotent per `processing_date`. Emit `EODCompleted`.
- **Depends on:** Phase 2.
- **Model:** Opus, Effort: High — EOD money summary + handoff contract; stale/partial reads corrupt next-day ops.

### 7.2 EOD monitoring page + alerts
- Admin/app-support-only dashboard: per-run status, duration, records, failures. Email alert on failure AND on not-completed-before-office-hours (0.4).
- **Depends on:** 7.1, 0.4.
- **Model:** Sonnet, Effort: Medium — monitoring UI + alert wiring, role-scoped.

---

## Dependency graph (summary)

```
Phase 0 (infra) ─┬─> Phase 2 (ATM ops) ─┬─> Phase 3 (finance) ─> Phase 4 (escrow)
                 │                       └─> Phase 7 (EOD)
                 ├─> Phase 1 (in-flight, parallel)
                 ├─> Phase 5 (cash count — gated on table approval)
                 └─> Phase 6 (CIT — dedicated, after ATM ops solid)
```

---

## Working conventions for every phase task

1. Author a proper spec under `.kiro/specs/<feature>/` (requirements → design → tasks) before non-trivial work.
2. Every task carries a **`Model:` + `Effort:`** note (`model-recommendation.md`).
3. Propose any new table/column in `project-context.md` Sec 2 **before** migrating.
4. Tests ship with every feature; coverage ≥80% on `internal/*` (Sec 7).
5. Update the **Status snapshot** in this file when a module lands.

---

## Conventions

- Update this plan whenever a phase item completes, a dependency changes, or a table gets approved.
- Keep the Status snapshot honest: "done" means wired + DB-backed + tested, never a folder or a mock UI.
- Percentages are estimates for planning, not commitments.
