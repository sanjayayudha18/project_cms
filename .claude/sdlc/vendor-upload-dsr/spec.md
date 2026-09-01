# Spec: vendor upload DSR (SALDO HARIAN ATM)

Status: draft. Stage: 2 Design. Reads: `intent.md`.
Trigger to next stage: product owner accepts this spec -> Claude Code plan mode.
Skills applied: TODO (security, api-design, brand/UX). Flag contradictions below.

## Requirements
### Functional
- FR1: Authenticated vendor uploads one `.xlsx` DSR workbook via the vendor portal.
- FR2: System computes a checksum; a duplicate checksum returns the existing file record (idempotent, no re-parse).
- FR3: System parses sheet `Daily` into leaf line-items per `013_dsr.sql` (sections d0/d1, flows saldo_awal/penerimaan/pengeluaran/status_cadangan).
- FR4: Derived rows (subtotals, SALDO AKHIR/SEMENTARA, TOTAL) are NOT stored.
- FR5: Vendor-stated closing balances land on `atm_dsr_saldo_files` for cross-check.
- FR6: Broken cells (`#REF!`) -> NULL + increment `error_count`; parse does not abort.
- FR7: Vendor sees processing status (pending | processing | completed | failed) + error_message.
- FR8: A vendor may only upload/see files for their own vendor scope.

### Non-functional
- DSR upload <= 30s per doc (URS NFR). Sync parse in handler (<1000 rows).
- Amounts numeric, full IDR, signs verbatim. Timestamps timestamptz (UTC).

## API surface (proposed - confirm before build)
- `POST /api/vendor/dsr/uploads`  (multipart .xlsx) -> `{ id, status, checksum }`
- `GET  /api/vendor/dsr/uploads/{id}` -> file record + counts
- `GET  /api/vendor/dsr/uploads` -> list scoped to vendor
- Response shape: ATM backend flat JSON (NOT pkg/response envelope) - CLAUDE.md Sec 3.

## Data model
Existing (no migration needed): `atm_dsr_saldo_files`, `atm_dsr_saldo_rows` (`013_dsr.sql`).
Confirm: link `atm_dsr_saldo_files.vendor` (text) to authenticated vendor scope - FK to `vendors.code` is noted as "not yet" in the migration.

## Out of scope
- Reconciliation of DSR vs escrow/journal (separate feature).
- EOD Final Realisasi computation (cmd/batch, separate).
- Invoice / cash-count flows.

## Flagged concerns (resolve with policy owner before build)
- [ ] Auth scoping: `atm_dsr_saldo_files.vendor` is free text today; enforcing "own scope only" (FR8) needs a resolved vendor identity. Blocks FR8.
- [ ] Maker-checker: is DSR ingest a state change requiring approval, or audit-only? (open question from intent.md)
- [ ] Storage of raw .xlsx (Cloud Storage) vs parsed-only - affects GCS_BUCKET usage + retention/PDP.
- [ ] 09:00 deadline behaviour (block vs flag).

## Acceptance
Traceable to FR1-FR8. Each becomes a test in `tests.md`.
