# CMS ATM & CIT — Project Context (Kiro Steering Doc)

> **READ THIS FIRST, EVERY SESSION.** Single source of truth for the Cash Management System. If your work conflicts with this, STOP and ask. Never invent endpoints, tables, columns, env vars, or modules.

---

## Golden Rules

1. Stack is FIXED (see tech.md). No new libs/frameworks without approval.
2. Follow module & table map (Sec 3). No modules/tables not listed unless asked.
3. Every state-changing action respects maker-checker + writes audit_log.
4. Money = numeric / integer minor units, never float.
5. Plan before non-trivial work. Small diffs. One concern per change.
6. DB topology: write/update → primary; read/report/dashboard → read replica.
7. Two frontends: internal (LDAP) + vendor portal (local auth). Keep separate.
8. Two backend entrypoints, one codebase: `cmd/api` (transactional, office hours) + `cmd/batch` (EOD, midnight). Shared `internal/*`.
9. EOD batch output is the source of truth for the transactional module. Write to DB, signal readiness before transactional reads.

---

## 1. Overview

- **Name**: Cash Management System (CMS) — ATM & CIT
- **Goal**: E2E ATM cash management: vendor replenishment, daily DSR reporting, forecasting & scheduling, cash count (vault + selective machine), reconciliation vs Corebanking escrow, vendor invoice validation & approval.
- **Stage**: Greenfield, vibe-coded with AI.
- **Roles**: Admin, Operator, Manager (approver), Vendor, Branch/Internal User.

---

## 2. Architecture, Modules & Data Map

### Backend layout

```
backend/
  cmd/api/main.go        # transactional server entrypoint
  cmd/batch/main.go      # EOD runner entrypoint
  configs/               # env config loader
  pkg/middleware/        # auth + role middleware
  pkg/response/          # consistent JSON envelope
  pkg/database/          # pgxpool: primary (write) + replica (read)
  internal/<module>/     # one folder per domain module
```

### Frontends (two separate SPAs)

```
frontend/CodexCash-Vite/     # internal app, LDAP login
frontend/VendorPortal-Vite/  # vendor portal, local login
```

### Modules (create ONLY these unless told)

**Platform Core**: `internal/auth` (LDAP + local, JWT, /me) · `internal/user` · `internal/audit` · `internal/approval` (maker-checker) · `internal/document` · `internal/notification` (in-app + SMTP) · `internal/export` (CSV/XLSX/PDF)

**Master Data**: `internal/vendor` · `internal/vendorpic` · `internal/vault` · `internal/location` · `internal/atm` · `internal/assignment`

**ATM Operations**: `internal/dsr` · `internal/replenishment` · `internal/forecast` (H+2)

**Finance**: `internal/invoice` (upload + validate + approve — NO payment execution) · `internal/reconciliation`

**CIT**: `internal/cit` · `internal/journal` · CIT reconciliation (order vs DSR vs journal)

**Integration**: `internal/corebanking` (escrow batch file ingest + parse → feeds reconciliation)

### DB logical groups (canonical table names)

- **Auth**: `roles`, `users` (add `auth_source` = ldap|local; password hash only for local)
- **Core**: `audit_logs`, `approval_requests`, `documents`, `notifications`, `import_jobs`, `export_jobs`
- **Master**: `vendors`, `vendor_pics`, `vendor_vaults`, `locations`, `atms`, `vendor_assignments`
- **ATM**: `atm_dsr_uploads`, `atm_dsr_rows`, `replenishment_instructions`, `forecast_runs`, `forecast_results`
- **Finance**: `invoice_uploads`, `invoice_items`, `invoice_reconciliation_results`
- **CIT**: `cit_orders`, `cit_handover_evidences`, `cit_journals`, `cit_dsr_uploads`, `cit_reconciliation_results`
- **Integration**: `escrow_batch_files`, `escrow_batch_rows`, `escrow_reconciliation_results`

> Need a new table/column? Propose here FIRST, get approval, then migrate.

---

## 3. Cross-Cutting Domain Rules (NON-NEGOTIABLE)

- **Auth split**: Internal → LDAP bind (no local password). Vendor → local creds (bcrypt/argon2), separate frontend + login route. Both issue same JWT; role + auth_source in claims. Vendors scoped to own assignments only.
- **Maker-checker**: any create/update/delete on financial or master data → `approval_requests` (pending → approved/rejected). Effect applies only after approval. Maker != checker.
- **Invoice**: CMS validates & approves only. Does NOT trigger/execute payment. Terminal = approved (handed off downstream).
- **Escrow reconciliation**: source = batch file from Corebanking. Ingest → parse into `escrow_batch_rows` → reconcile vs CMS cash position → store deltas. Idempotent per file hash.
- **Audit**: every state change writes `audit_logs` (who, what, before/after, when, ip).
- **RBAC**: enforce at middleware AND service layer.
- **Idempotency**: all file ingests (DSR/invoice/escrow) idempotent per file hash → `import_jobs`.
- **Consistent response**: all endpoints return `pkg/response` envelope.

---

## 4. Money, Data Integrity & DB Topology

- Monetary/cash amounts as **numeric** (or integer minor units). NEVER float/double.
- Currency: default IDR, always store code explicitly.
- Timestamps: timestamptz, store UTC, display Asia/Jakarta.
- **Read/Write split**: Primary pool → all writes + transactional reads. Replica pool → reporting, dashboards, cash monitoring, exports, heavy reads. Repos expose `db` (primary) vs `dbRead` (replica). Never write on replica. Read-after-write in same flow uses primary.
- Reconciliation results reproducible & explainable (store inputs + deltas).
- Keep raw rows (`atm_dsr_rows`, `escrow_batch_rows`) separate from summaries.

---

## 5. Glossary

| Term | Meaning |
|------|---------|
| DSR | Daily Status Report: cash position per ATM/CRM per day |
| CIT | Cash in Transit: physical pickup/delivery by vendor |
| Replenishment | Instruction + execution to refill ATM cash |
| Vault | Vendor cash vault holding bank cash |
| Escrow reconciliation | Match CMS cash position vs Corebanking escrow batch file |
| Forecast (H+2) | 2-day-ahead cash need → shortage → vendor cash need |
| Cash count | Physical count: vault (vendor) + selective machine |
| Maker-checker | Two-person approval control |
| Invoice reconciliation | Validate invoice vs executed CIT/replenishment; approve only, no payment |

---

## 6. Env & Config

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

- Never commit `.env`. Never log secrets/JWTs/LDAP/SMTP creds.
- docker-compose (local): backend + frontend-internal + frontend-vendor + redis. Postgres external.

---

## 7. GCP Deployment

| # | Product | Role |
|---|---------|------|
| 1 | Compute Engine | Frontend (internal + vendor) |
| 2 | Compute Engine | Backend (Go+Chi) |
| 3 | CloudSQL Postgres | Primary (write) |
| 4 | CloudSQL Postgres | Read replica (reporting/dashboard) |
| 5 | Cloud Storage | Uploads/exports/escrow batch files |
| 6 | Memorystore Redis | Cache/sessions |
| 7 | Artifact Registry | Docker images |
| 8 | Cloud Build | CI: build → test → scan → push |
| 9 | Secret Manager | Secrets (JWT, LDAP, SMTP, DB) |
| 10 | Cloud DNS | Domains (internal + vendor) |
| 11 | Cloud Armor | WAF (esp. vendor portal) |
| 12 | Cloud Logging | Structured logs |
| 13 | Cloud Monitoring | Metrics/alerts (incl. replica lag) |

**Pipeline**: Cloud Build → lint + go test + vite build (x2) → build images → push to Artifact Registry with immutable tag (git SHA, not latest) → deploy to Compute Engine.

---

## 8. Backend Split: Transactional vs Batch (EOD)

**One codebase, two entrypoints.** Domain logic stays shared in `internal/*`.

- `cmd/api` — transactional server. Always on, serves the frontends. Active during office hours.
- `cmd/batch` — End-of-Day runner. Triggered by cron/scheduler at midnight, processes, then exits/idles.

Both run on the **same VM**. Active windows don't overlap — no resource contention.

**What the batch/EOD does:**
- Ingest & compute from DSR (saldo akhir 00:00 per vault), Opti Cash forecast (Order H-1 & H), horizon H-2 (refund validation).
- Compute Final Realisasi = rekomendasi DMAA − (saldo DSR + refund horizon H-2), per vendor.
- Produce the daily summaries the transactional module reads the next working day.

**Handoff rules (NON-NEGOTIABLE):**
- EOD output written to **DB = source of truth**. Redis only as cache, never store of record.
- Every EOD run tracked in a run table: `processing_date`, status (running/success/failed), started_at, finished_at, records_processed, error.
- Transactional module reads a `processing_date` only after its run is marked **success**. Never read partial/in-progress data.
- On completion, emit domain event (`EODCompleted` / `SummaryReady`).
- Batch ingests are idempotent per file/`processing_date` (re-run safe).

**EOD Monitoring (admin / app-support only):**
- Dedicated dashboard: per-run status, duration, records processed, failures with error detail.
- Email alert on: EOD failure, AND EOD not completed before office-hours start.

---

## 9. Definition of Done

- [ ] Matches requirement + module/table map
- [ ] Correct auth path (LDAP internal / local vendor), scoped RBAC
- [ ] Maker-checker + audit_log wired
- [ ] Reads on replica, writes on primary
- [ ] Money as numeric, timestamps timestamptz
- [ ] Tests passing incl. auth/RBAC/money cases
- [ ] No secrets/config hardcoded; .env.example updated
- [ ] Uses pkg/response envelope
- [ ] Builds cleanly in Docker

---

## 10. Resolved Decisions

- Escrow integration: batch file → `internal/corebanking` ingest → reconciliation.
- Invoice payment: validate & approve only. No payment execution.
- Frontends: two separate SPAs. Internal = LDAP; Vendor = local creds in CMS.
- Email: company SMTP relay.
- DB topology: primary for write/update, read replica for reporting/dashboard/cash monitoring.
