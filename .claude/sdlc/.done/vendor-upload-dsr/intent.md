# Intent: vendor upload DSR (Daily / SALDO HARIAN ATM + Rencana Isi)

Author: TODO (product owner). Status: draft — ready for product owner acceptance.
Stage: 1 Plan. Trigger to next stage: product owner accepts this file.

> Supersedes the previous draft of this file, which assumed a single-location
> workbook and synchronous parsing inside the Go handler. Reached via an 8-round
> grilling session on 2026-09-03; see "Resolved decisions" below.

## Problem
Vendors submit a daily Excel workbook (legacy binary `.xls`, e.g.
`Laporan Harian DSR CIMB Niaga TAG JAKARTA Tanggal 15 JULI 2026 ( REVISI ).xls`)
containing the "SALDO HARIAN ATM" cash-balance report (sheet `Daily`, one or more
vault/location blocks) and the next-day fill plan (sheet `Rencana Isi`). Today
there is no self-service path for a vendor to upload it and no structured store
of its contents, so the daily balance data can't be validated, reconciled, fed
into the EOD forecast, or checked against the 09:00 URS deadline.

## Proposed outcome
A vendor logs into the vendor portal, uploads one DSR workbook, and — within the
request/response cycle when possible — sees whether `Daily` and `Rencana Isi`
were each accepted (parsed) or rejected, with enough detail (which rows/ATMs)
to fix a resubmission. Accepted data lands in `atm_dsr_saldo_files` +
`atm_dsr_saldo_rows` (`013_dsr.sql`, extended by `017_...sql` for multi-location)
and `atm_dsr_rencana_isi_files` + `atm_dsr_rencana_isi_rows`.

## Affected users and systems
- Users: Vendor (uploader), Operator/Admin (monitors ingest via the existing
  generic dashboard — no new admin UI in this feature).
- Systems: `frontend/VendorPortal-Vite`, ATM backend (`backend/`, port 8080,
  `internal/dsr`), **`scheduler/dsr/dsr_etl.py`** (new — a standalone Python ETL
  script following the existing `scheduler/dmaa/`, `scheduler/itm/*` pattern),
  **`scheduler/retry_scheduler/`** (existing FastAPI service — gains a 4th
  `FILE_TYPES` entry `dsr` and a new generic trigger endpoint), PostgreSQL
  primary, local filesystem (`FTP_DATA/DSR/`), migrations `013_dsr.sql` +
  `017_atm_dsr_location_and_rencana_isi.sql` (existing schema, no new migration
  needed).

## Constraints
- Vendor auth = local creds only; vendor scoped to own assignments (CLAUDE.md
  Sec 5). `atm_dsr_*_files.vendor` is always written from the authenticated JWT
  identity (`vendors.name`, resolved via `vendor_id` claim), never from the
  workbook's own free-text vendor/company cell.
- Idempotent per file checksum, exactly like `dmaa_etl.py`/`itm_*_etl.py` —
  **not** via the shared `import_jobs` table CLAUDE.md Sec 5 describes. All
  three existing ETLs use their own `*_files.checksum` UNIQUE constraint
  instead; this is a documented, precedent-backed deviation, not an oversight.
- Money as numeric, full IDR, verbatim signs (per `013_dsr.sql` notes). Never
  float.
- The workbook is legacy binary `.xls` (OLE2/BIFF, confirmed by magic bytes
  `D0 CF 11 E0 A1 B1 1A E1`), not OOXML `.xlsx`. Parsing requires `xlrd`
  (Python) — no viable Go library exists for this format, which is *why* the
  vendor-vs-content cross-check was dropped (see resolved decisions) rather
  than built in Go.
- Go and the Python side communicate **only over HTTP** (`retry_scheduler`'s
  REST API) — never direct subprocess/filesystem coupling from Go into Python.
  `retry_scheduler` has no Docker packaging today; it runs as its own process,
  reached over the network like any other backend dependency.
- Writes on primary; any read-after-write in the same flow (building the
  upload response) also reads primary, per Sec 6.
- Broken cells (`#REF!`) ingest as NULL + increment `error_count`; do not abort
  the file. Same tolerance extended to unresolved ATM terminal ids on
  `Rencana Isi` (hard FK to `atms.terminal_id`).

## Resolved decisions (grilling session, 2026-09-03)

**Architecture / trigger**
- Go's vendor-portal upload handler validates (size cap, magic-byte check for
  OLE2/ZIP, filename sanitized against path traversal), writes the file to
  `FTP_DATA/DSR/` as `<vendor_code>__<original_filename>` (vendor_code from
  `vendors.code`, resolved server-side from the JWT `vendor_id` — never
  client-supplied), then calls a new **`POST /process/{file_type}`** endpoint
  on `retry_scheduler` (generic over all 4 file types; reuses the existing
  `RetryExecutor.execute_retry(file_type)` subprocess call verbatim).
- `retry_scheduler` runs `scheduler/dsr/dsr_etl.py`, which follows the DMAA/ITM
  contract exactly: no file argument, drains the whole `FTP_DATA/DSR/` folder,
  archives processed files locally (no GCS), raises on the first unhandled
  error.
- Go's HTTP call blocks up to ~25s (under the URS's ≤30s/doc NFR) for an
  instant vendor result; on timeout it detaches (does **not** kill the
  subprocess) and returns `status: pending` for the vendor to poll. If the call
  to `retry_scheduler` fails outright (service down, network error), Go still
  returns `pending` rather than failing the upload — the file is already
  safely on disk and the existing cron-scan is the safety net either way.
- The concurrency lock guarding subprocess invocation lives inside
  `retry_scheduler` (a single Python process, one `asyncio.Lock`), not in Go —
  released on actual subprocess exit, not on Go's timeout, so a slow run
  correctly makes the next upload queue behind it rather than race it.
- `dsr` is registered in `retry_scheduler`'s `FILE_TYPES`, `FILE_PATTERNS`,
  `sla_dsr = "09:00"`, and `etl_dsr_script` — inheriting the existing generic
  cron-scan, late-detection, and (unchanged, un-specialized) blind auto-retry
  behavior for free.

**Scope**
- Only sheets `Daily` (multi-location, `017_...sql`) and `Rencana Isi` are
  parsed. The workbook's other sheets (`Berita Acara`, `REKON ATM/CRM/CDM`,
  `BA cartridge <lokasi>`, `penyelesaian klaim sel kurang`, `LOK`) are
  explicitly out of scope — they map to the separate, not-yet-approved
  cash-count/BA and reconciliation features (CLAUDE.md Sec 3a).
- The two sheets ingest and report independently (matches the existing
  migration comment: "not FK'd together, either may succeed alone"). One
  upload response reports two separate results, each with row-level error
  detail (e.g. which ATM terminal ids were unresolved on `Rencana Isi`, which
  cells were `#REF!` on `Daily`) read from the child tables — not just an
  aggregate count.

**Idempotency & re-upload**
- Exact checksum match on an existing file → skip (no-op), matching DMAA.
- Same `(report_date, vendor)` but a different checksum (a `( REVISI )`
  resubmission) → **replace**: delete the prior file row (children cascade),
  insert the new one. No versioning/history of superseded ingests.

**Auth, audit, and money-adjacent rules**
- Audit-only, no maker-checker — ingest records a vendor-reported fact, it
  doesn't move money or change master data (consistent with DMAA/ITM
  precedent). A later feature that *edits* a DSR row would need maker-checker;
  raw ingest doesn't.
- No cross-check between the authenticated vendor and the workbook's internal
  free-text vendor/company cell — that cell was never going to be trusted for
  authorization anyway (vendor comes from JWT either way), and building the
  check would require a Go `.xls`-reading dependency that doesn't exist.
- Unknown ATM terminal id on `Rencana Isi` → skip that row, increment
  `error_count`, file still completes (mirrors `dmaa_etl.py`'s
  missing-terminal handling exactly).

**Deadline, retention, notification**
- A file arriving after 09:00 WIB is accepted and flagged (feeds the URS's
  monthly FLM late/missing penalty report), never blocked.
- Raw workbook retained via local backup dir (`FTP_DATA/DSR/backups/`),
  matching DMAA/ITM precedent — no Cloud Storage for this feature.
- Vendor gets an **in-app** notification (existing `internal/notification`
  module) on failure/lateness; no SMTP channel for this feature.

**Security**
- Upload validated before any disk write: size cap (~10MB), magic-byte check
  (reject anything that isn't OLE2 or ZIP regardless of extension), filename
  reduced to a sanitized basename before being used to build the on-disk path.
- Per-vendor rate limit on the upload endpoint (via existing
  `pkg/middleware` Redis-backed limiter, no new infra) — tighter than a
  generic default, because every upload serializes through the single shared
  lock in `retry_scheduler`, so one vendor's misbehavior has a direct latency
  cost to every other vendor, especially around the shared 09:00 deadline.

**Admin visibility**
- No new admin UI. DSR appears automatically in `retry_scheduler`'s existing
  generic `status`/`late`/`summary`/`audit` endpoints once registered as a
  `FILE_TYPES` member — that dashboard was purpose-built to be file-type
  agnostic.

## Deferred / explicitly out of scope
- Reconciliation of DSR vs escrow/journal (separate feature).
- EOD Final Realisasi computation (`cmd/batch`, separate).
- Invoice / cash-count / Berita Acara flows (separate, not-yet-approved
  modules per CLAUDE.md Sec 3a).
- Email notification to vendors (in-app only for now).
- DSR-specific admin dashboard (generic one is in scope; a specialized view is
  a future feature if requested).
- Versioned history of superseded (`REVISI`) ingests — replace-only for now.

## Open questions
None blocking. Exact tuning values (upload size cap, rate-limit thresholds,
Go→`retry_scheduler` HTTP timeout budget) are implementation defaults to set
during `spec.md`/`plan.md`, not policy decisions requiring further sign-off.
