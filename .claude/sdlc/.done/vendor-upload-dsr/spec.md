# Spec: vendor upload DSR (Daily / SALDO HARIAN ATM + Rencana Isi)

Status: draft. Stage: 2 Design. Reads: `intent.md`.
Trigger to next stage: product owner accepts this spec -> Claude Code plan mode.
Skills applied: security (upload validation, rate limiting), api-design. Flag
contradictions below.

> **Post-spec update (2026-09-04):** the single-phase FR5/FR11 flow below
> (Go blocks on `retry_scheduler`, returns `pending` on timeout) was
> superseded during implementation by a two-phase dry-run/confirm flow — see
> `plan.md`'s post-plan update and `testing.md` for the current behavior.
> `retry_scheduler` itself was later split into
> `backend_python/service_dsr_etl/` (owns `dsr` only, port 8090 -- what every
> FR below actually means when it says `retry_scheduler`) and
> `backend_python/eod_retry_scheduler/` (owns
> `dmaa`/`itm_cashpos`/`itm_replenish`, port 8091). The whole tree also moved
> from `scheduler/` to `backend_python/` at the repo root (2026-09-04).

> Regenerated from `intent.md` (2026-09-03, 8-round grilling session).
> Replaces the previous draft, which assumed single-location, sync-in-handler
> parsing with no `retry_scheduler`/Python involvement.

## Requirements

### Functional
- FR1: Authenticated vendor uploads one workbook (`.xls` legacy binary or
  `.xlsx`) via the vendor portal, multipart, single `file` field.
- FR2: Before any disk write, the Go handler validates: size <= 10MB, magic
  bytes match OLE2 (`D0 CF 11 E0 A1 B1 1A E1`) or ZIP (`PK..`) regardless of
  extension, and the filename is reduced to a sanitized basename (reject/strip
  path separators and `..` segments).
- FR3: The Go handler resolves `vendor_code` (`vendors.code`) and vendor
  display name (`vendors.name`) server-side from the JWT `vendor_id` claim.
  Any client-supplied vendor identifier is ignored.
- FR4: The validated file is written to `FTP_DATA/DSR/` as
  `<vendor_code>__<sanitized_original_filename>`.
- FR5: The Go handler calls `POST /process/dsr` on `retry_scheduler`
  (generic `POST /process/{file_type}` endpoint), blocking up to ~25s for a
  synchronous result.
- FR6: `retry_scheduler` serializes ETL subprocess invocations across all
  file types with a single `asyncio.Lock`, held until the subprocess actually
  exits (not released on a caller's timeout).
- FR7: `scheduler/dsr/dsr_etl.py` (new, no file argument, drains the whole
  `FTP_DATA/DSR/` folder) parses sheet `Daily` (multi-location, sections
  `d0`/`d1`, flows `saldo_awal`/`penerimaan`/`pengeluaran`/`status_cadangan`,
  per `013_dsr.sql` + `017_...sql`) into `atm_dsr_saldo_files` +
  `atm_dsr_saldo_rows`, and sheet `Rencana Isi` into
  `atm_dsr_rencana_isi_files` + `atm_dsr_rencana_isi_rows` — the two sheets
  are ingested and reported **independently**; either may succeed alone.
- FR8: Idempotency: an exact checksum match against an existing file record
  is a no-op (`status: skipped`, existing record returned unchanged). A file
  with the same `(report_date, vendor)` but a **different** checksum (a
  `( REVISI )` resubmission) replaces the prior record: delete the existing
  file row (children cascade), insert the new one. No superseded-version
  history is kept.
- FR9: Derived/subtotal rows (Subtotal, SALDO AKHIR/SEMENTARA, TOTAL, Sub
  Total) are never stored — recomputed on read, per existing schema
  convention.
- FR10: Broken cells (`#REF!`) on `Daily` -> NULL + increment
  `error_count`, file continues. An unresolved ATM terminal id on
  `Rencana Isi` (hard FK to `atms.terminal_id`) -> that row is skipped (not
  inserted), `error_count` incremented, file continues — mirrors
  `dmaa_etl.py`'s missing-terminal handling.
- FR11: If Go's call to `retry_scheduler` exceeds ~25s or fails outright
  (service unreachable), the upload response reports `status: pending` for
  the affected sheet(s) rather than failing the upload — the file is already
  durably written and `retry_scheduler`'s cron-scan is the fallback trigger.
- FR12: The upload response contains two independent result objects
  (`daily`, `rencana_isi`), each with `status`
  (`pending|processing|completed|failed|skipped`), `row_count`,
  `success_count`, `error_count`, and a **row-level error list** (e.g.
  unresolved `atm_terminal_id` values, row numbers with NULL denom cells) —
  not just an aggregate count.
- FR13: A vendor may only upload/see files scoped to their own `vendor_id`
  (JWT claim), enforced at both middleware and service layer.
- FR14: A file whose `report_date` implies arrival after 09:00 WIB is
  accepted and flagged (`sla_dsr = "09:00"` in `retry_scheduler`), never
  rejected — feeds the URS's monthly FLM late/missing report via the existing
  generic late-detection machinery.
- FR15: On ingest failure or a late-flagged file, the vendor receives an
  **in-app** notification via the existing `internal/notification` module.
  No SMTP channel for this feature.
- FR16: Every upload attempt (success, skip, or failure) writes one
  `audit_logs` entry (who, when, checksum, outcome). No maker-checker gate —
  ingest is a recorded fact, not an approval-gated state change.
- FR17: The upload endpoint is rate-limited **per vendor** (existing
  `pkg/middleware` Redis-backed limiter), tighter than the platform default,
  because every upload serializes through the single shared lock in FR6.
- FR18: `dsr` is registered as a 4th entry in `retry_scheduler`'s
  `FILE_TYPES`/`FILE_PATTERNS`/`etl_dsr_script` config, inheriting the
  existing cron-scan, late-detection, and **unmodified** blind auto-retry
  behavior (same as `dmaa`/`itm_cashpos`/`itm_replenish` — no DSR-specific
  retry logic).
- FR19: No new admin UI. DSR ingest status is visible through
  `retry_scheduler`'s existing generic `status`/`late`/`summary`/`audit`
  endpoints once FR18 lands.

### Non-functional
- Vendor-visible response time <= 30s (URS NFR); Go's own blocking budget to
  `retry_scheduler` is ~25s, leaving headroom for the rest of the request.
- Amounts numeric, full IDR, signs verbatim (matches vendor's own sign
  convention on disbursement lines). Timestamps timestamptz, stored UTC,
  displayed Asia/Jakarta.
- Upload size cap: 10MB.
- No new database migration — schema is fully covered by `013_dsr.sql` +
  `017_atm_dsr_location_and_rencana_isi.sql`.
- No new Go dependency (no `.xls`/`.xlsx` parsing library needed in Go — all
  parsing happens in Python). New Python dependency: `xlrd`, added to
  `scheduler/dsr/requirements.txt`.

## API surface (proposed — confirm before build)

**Vendor-facing** (`backend/`, port 8080, `internal/dsr`; flat JSON per ATM
backend's existing wire-compatibility convention, **not** `pkg/response`):

```
POST /api/vendor/dsr/uploads          (multipart "file")
  -> {
       checksum: string,
       daily:        { file_id, status, report_date, row_count,
                        success_count, error_count, errors: [...] },
       rencana_isi:  { file_id, status, plan_date, row_count,
                        success_count, error_count, errors: [...] }
     }

GET  /api/vendor/dsr/uploads/daily/{file_id}
  -> file record + row-level errors, scoped to caller's vendor_id

GET  /api/vendor/dsr/uploads/rencana-isi/{file_id}
  -> file record + row-level errors, scoped to caller's vendor_id

GET  /api/vendor/dsr/uploads          (?date=, paginated)
  -> list of { report_date, daily: {status,...}, rencana_isi: {status,...} }
     scoped to caller's vendor_id
```

**Internal, Go -> Python** (`retry_scheduler`, existing `require_auth`
api_key/JWT, existing envelope shape):

```
POST /process/{file_type}             (generic; called with file_type="dsr")
  -> { status: "success"|"failed", file_type, duration_ms, stdout_tail? }
```

## Data model
No migration needed. Existing tables (unchanged):
- `atm_dsr_saldo_files`, `atm_dsr_saldo_rows` (`013_dsr.sql`, extended by
  `017_...sql` for `location` + `saldo_gabungan_total_idr`).
- `atm_dsr_rencana_isi_files`, `atm_dsr_rencana_isi_rows` (`017_...sql`).
- `vendors.code` / `vendors.name` — source of the filename prefix and the
  authoritative `vendor` value written to both `*_files` tables.
- `retry_scheduler`'s own tables (`retry_file_tracking`, `scan_run`, audit —
  `012_retry_scheduler.sql`) — gain `dsr` as a `file_type` value, no schema
  change.

## Out of scope
- Reconciliation of DSR vs escrow/journal (separate feature).
- EOD Final Realisasi computation (`cmd/batch`, separate).
- The workbook's non-`Daily`/`Rencana Isi` sheets (`Berita Acara`,
  `REKON ATM/CRM/CDM`, `BA cartridge <lokasi>`, `penyelesaian klaim sel
  kurang`, `LOK`) — map to cash-count/BA and reconciliation, not-yet-approved
  modules per CLAUDE.md Sec 3a.
- Invoice reconciliation.
- Email notification to vendors (in-app only).
- DSR-specific admin dashboard.
- Versioned history of superseded (`REVISI`) ingests.

## Flagged concerns (resolve with policy owner before build)
- [ ] Exact numeric thresholds — upload size cap (10MB proposed), per-vendor
  rate limit (10/hour proposed), Go->`retry_scheduler` timeout (25s proposed)
  — are defaults from the grilling session, not signed off by an ops/capacity
  owner. Confirm before shipping to prod load.
- [ ] `retry_scheduler`'s `/process/{file_type}` endpoint needs a shared
  credential Go authenticates with (reuses existing `require_auth`
  api_key/JWT mode) — confirm provisioning path via Secret Manager (CLAUDE.md
  Sec 10 #9), not a hardcoded value in either service's `.env.example`.
- [ ] `scheduler/` (including the new `scheduler/dsr/`) has no Docker
  packaging today (confirmed in `.kiro/specs/eod-retry-scheduler/tasks.md`).
  This feature doesn't require adding one, but ops should confirm how
  `retry_scheduler` + the ETL scripts are actually kept running in the target
  environment (systemd unit, supervisor, manual — not specified anywhere in
  the repo today).

## Acceptance
Traceable to FR1-FR19. Each becomes a test in `tests.md`.
