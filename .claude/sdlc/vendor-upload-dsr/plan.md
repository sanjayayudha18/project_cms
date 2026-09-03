# Plan: vendor upload DSR (Daily / SALDO HARIAN ATM + Rencana Isi)

Status: draft — approved in Claude Code plan mode. Stage: 3 Build. Reads: `spec.md`.
Trigger to next stage: engineer accepts this plan -> implementation -> `tests.md`.

> Produced in Claude Code plan mode on 2026-09-03. Confirmed against the real
> codebase (not just `spec.md`'s assumptions) during planning; three
> corrections surfaced and were resolved with the product owner before this
> plan was finalized — see "Corrections from spec.md" below.

## Corrections from spec.md
Planning-time exploration of the actual repo found three places where
`spec.md`'s assumptions didn't match reality:

1. **`internal/dsr` doesn't exist.** The Go backend is organized by layer
   (`internal/handler`, `internal/service`, `internal/db` — sqlc-generated,
   no `internal/repository` for new code), not by domain module as
   CLAUDE.md's Sec 3 map describes. This is the first DSR-related Go code in
   the ATM backend — there's no existing module to slot into.
2. **`audit_logs` and `notifications` tables don't exist anywhere**, and no
   existing Go code writes to either — spec.md's FR15/FR16 assumed
   infrastructure that hasn't been built. **Decision (product owner,
   2026-09-03): scope down.** Drop the generic audit-log/notification
   requirements from this feature. Add one small nullable
   `uploaded_by_user_id` column to the two DSR file tables instead — the
   vendor/created_at/status/error_message columns already there, plus this
   one, are enough accountability for this feature alone. Vendor
   notification is deferred entirely (portal polling only for v1). Building
   the real `audit_logs`/`notifications` platform tables is a separate,
   deliberately-scoped feature, not a side effect of this one.
3. **`pkg/middleware.RateLimiter` is login-attempt-shaped** (username+IP,
   increment-on-failure), not a generic per-key request counter. Reusing it
   for "N uploads/hour/vendor" would distort its purpose. This plan uses a
   small, local Redis `INCR`+`EXPIRE` counter scoped to this feature instead
   of forcing it into that existing type.

Everything else from `spec.md` (FR1-FR14, FR17-FR19) stands as specified.

## Architecture
```
Vendor -> POST /api/v1/dsr/uploads (Go, backend/, port 8080)
            |  validate (size/magic-bytes/filename), resolve vendor from JWT
            |  write FTP_DATA/DSR/<vendor_code>__<filename>
            v
          POST http://retry_scheduler/process/dsr   (new endpoint, ~25s block)
            |
            v
          scheduler/dsr/dsr_etl.py (new)             <- drains FTP_DATA/DSR/
            |  parses Daily + Rencana Isi, writes via psycopg
            v
          atm_dsr_saldo_files/rows, atm_dsr_rencana_isi_files/rows
            ^
            |  Go re-reads by checksum (primary), builds response
Vendor <---+
```
Safety net: `retry_scheduler`'s existing cron-scan / late-check / auto-retry
covers `dsr` once registered as a 4th `FILE_TYPES` entry — no new scheduling
code needed there.

## Files to change

### 1. Migration — `backend/migrations/018_dsr_upload_accountability.sql` (new)
Add nullable `uploaded_by_user_id bigint REFERENCES users(id) ON DELETE SET
NULL` to `atm_dsr_saldo_files` and `atm_dsr_rencana_isi_files`. No other
schema change — `013_dsr.sql` + `017_...sql` already cover everything else.
Mirror existing migration style (`BEGIN;`/`COMMIT;`, `ALTER TABLE IF EXISTS
... ADD COLUMN IF NOT EXISTS`, `COMMENT ON COLUMN`).

### 2. Python ETL — `scheduler/dsr/` (new directory)
- `dsr_etl.py` — modeled on `scheduler/dmaa/dmaa_etl.py`: no file argument,
  globs `FTP_DATA/DSR/*`, splits the `<vendor_code>__` prefix off each
  filename to resolve `vendor` (looked up against `vendors.code` — an
  unknown code is a bug, not a vendor error, since Go authenticated it),
  computes checksum, parses `Daily` via `pandas.ExcelFile(path,
  engine="xlrd")` and `Rencana Isi` similarly, applies the idempotency rule
  (exact checksum -> skip; same `(report_date, vendor)` different checksum
  -> delete + reinsert), archives to `FTP_DATA/DSR/backups/` on completion,
  raises on hard failure (file stays for the existing not_processed/retry
  flow). Connects via `psycopg` using the same `DB_HOST`/`DB_PORT`/
  `DB_NAME`/`DB_USER`/`DB_PASS` env convention as `dmaa_etl.py`.
- `requirements.txt` — `pandas`, `psycopg[binary]`, `python-dotenv`, `xlrd`
  (new — none of the existing ETLs need it).
- `test_dsr_etl.py` — one runnable smoke test, modeled on
  `scheduler/itm/cashpos/test_itm_cashpos_etl.py`: exercise the row-parsing
  functions against a small synthetic workbook fixture, and the
  checksum-skip / replace-on-different-checksum branches of `process_file`.

### 3. `retry_scheduler` changes (existing service)
- `config.py` — add `"dsr"` to `FILE_TYPES`; `FILE_PATTERNS["dsr"]`; add
  `sla_dsr: str = "09:00"` to `Settings` (included in
  `validate_time_format`/`get_sla_time`); `etl_dsr_script: Path =
  Path("scheduler/dsr/dsr_etl.py")`; `dsr_input_dir`/`dsr_not_processed_dir`
  following the existing per-type optional-override pattern.
- `routers/process.py` (new) — `POST /process/{file_type}`, protected by the
  existing `require_auth` dependency, validates `file_type in FILE_TYPES`,
  calls the already-existing `RetryExecutor.execute_retry(file_type)`,
  returns `{status, file_type, duration_ms}`. Serializes against
  `SchedulerService`'s existing lock (not a second, separate lock) so manual
  triggers and the cron scan can never race each other.
- `main.py` — mount the new router.

### 4. Go backend — new DSR upload slice
- `backend/queries/dsr.sql` (new) — sqlc queries: `GetVendorByID`,
  `GetDsrSaldoFileByChecksum`/`ByReportDateVendor`, `GetDsrRencanaIsiFile...`
  (same two lookups), `ListDsrSaldoRowErrors` (NULL denom_* rows),
  `ListDsrRencanaIsiRowErrors` (`atm_id IS NULL` rows), `ListDsrUploads`
  (paginated, vendor-scoped). `sqlc generate` produces
  `backend/internal/db/dsr.sql.go` (generated, not hand-written).
- `backend/internal/service/dsr_upload.go` (new):
  - `ValidateUpload` — size <= 10MB, magic-byte check (`D0 CF 11 E0 A1 B1 1A
    E1` or `PK\x03\x04`), filename sanitized to a safe basename.
  - `ResolveVendor(ctx, vendorID)` — code + name via the new sqlc query.
  - `WriteAndTrigger` — write `FTP_DATA/DSR/<vendor_code>__<name>`, POST to
    `retry_scheduler`'s `/process/dsr` with a ~25s HTTP client timeout; on
    timeout/connection error, log and proceed rather than fail the request
    (FR11).
  - `BuildUploadResult` — reads both file tables (+ error rows) on primary,
    by checksum first, falling back to `(report_date, vendor)` if the
    checksum row isn't there yet (`status: pending`).
  - `CheckUploadRate` — Redis `INCR` on
    `dsr:upload:<vendor_id>:<hour-bucket>` + `EXPIRE`, capped at 10/hour.
- `backend/internal/handler/dsr_upload_handler.go` (new) — `POST /uploads`
  (multipart `"file"`), `GET /uploads/daily/{id}`, `GET
  /uploads/rencana-isi/{id}`, `GET /uploads` (list). Follows
  `atm_portal_handler.go`'s conventions (`writeJSON`/`writeError`, `VendorID`
  from the `pkg/middleware` auth context) — 403 if `VendorID == nil`.
- `backend/cmd/api/main.go` — mount
  `r.With(custommw.RequireAuth(tokenService)).Mount("/api/v1/dsr",
  dsrHandler.Routes())`, matching the existing `atm-portal`/`dmaa-forecast`
  mount pattern; wire an `http.Client` (25s timeout) pointed at
  `retry_scheduler`'s base URL (new env var, `.env.example` updated).

## Order of work
1. Migration (`018_...sql`) — apply locally first, everything else depends
   on the new column existing.
2. `scheduler/dsr/dsr_etl.py` + `requirements.txt` + `test_dsr_etl.py` —
   buildable and testable in isolation against a local Postgres, no
   dependency on the Go side or `retry_scheduler` yet.
3. `retry_scheduler` additions (`config.py`, `routers/process.py`,
   `main.py`) — wires the ETL into the existing service; testable via
   `pytest` + a direct `curl` to `/process/dsr` once step 2's script exists.
4. Go slice (`queries/dsr.sql` -> `sqlc generate` -> `service` -> `handler`
   -> `main.go` wiring) — last, since it's the caller of step 3's endpoint.
5. End-to-end manual verification (see below) once all four are in place.

## Risks
- **Legacy `.xls` parsing quirks**: `013_dsr.sql`'s own comments note the
  real sample file is "riddled with `#REF!`" errors — `dsr_etl.py`'s
  tolerance for broken cells needs to be verified against the actual
  `DSR_DATA` sample, not just unit-test fixtures, before calling this done.
- **`retry_scheduler` has no Docker packaging** (confirmed in
  `.kiro/specs/eod-retry-scheduler/tasks.md`) and no documented process
  supervision (systemd/manual) — this plan doesn't fix that gap, it just
  doesn't make it worse. Local dev/testing will run it directly
  (`uvicorn`/`python -m scheduler.retry_scheduler`).
- **Shared lock contention**: reusing `SchedulerService`'s existing lock for
  the new manual-trigger path means a slow cron scan could make a vendor's
  upload wait close to the full 25s timeout before Go gives up and returns
  `pending`. Acceptable per the design (Q1-Q2/round 5 of the intent
  interview), but worth watching in practice.

## Alternatives not taken
- **Go spawning `dsr_etl.py` directly via `exec.Command`** — rejected: would
  couple Go's container to a Python runtime + filesystem access it doesn't
  have today, and breaks the established Go<->`retry_scheduler` HTTP
  boundary the existing system already uses.
- **New `audit_logs`/`notifications` tables built as part of this feature**
  — rejected by the product owner in favor of scoping down (see
  "Corrections from spec.md" #2); revisit as its own feature.
- **Reusing `pkg/middleware.RateLimiter` for upload throttling** — rejected;
  its login-attempt semantics don't fit a plain request-count limit, and
  bending it would make the type harder to reason about for its actual
  purpose (auth brute-force protection).

## Proof (maps to tests.md)
- `dsr_etl.py`: `test_dsr_etl.py` (synthetic fixture) + a manual run against
  the real `DSR_DATA/Laporan Harian DSR CIMB Niaga TAG JAKARTA...` sample
  pointed at a local Postgres, confirming `atm_dsr_saldo_rows`/
  `atm_dsr_rencana_isi_rows` populate correctly and multi-location blocks
  resolve `location` per row.
- `retry_scheduler`: existing `pytest` suite + a new test for
  `routers/process.py` (auth required, unknown `file_type` -> 422,
  concurrent-call serialization via the shared lock).
- Go: `go test ./...`, with new tests covering size/magic-byte rejection,
  vendor scoping (a vendor JWT can't see another vendor's `file_id`), the
  pending-on-timeout path, and checksum-skip / replace-on-REVISI, against a
  real Postgres (matching `atm_portal_integration_test.go`'s existing
  integration-test convention).
- End-to-end: upload the real sample file through the new endpoint against a
  local stack (Go + `retry_scheduler` both running), confirm the response
  reports both `daily` and `rencana_isi` results, re-upload the same file
  and confirm `skipped`.
