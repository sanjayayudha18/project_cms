# Intent: vendor upload DSR (SALDO HARIAN ATM)

Author: TODO (product owner). Status: draft.
Stage: 1 Plan. Trigger to next stage: product owner accepts this file.

## Problem
<!-- What can't we do today, in your own words. -->
Vendors submit the daily "SALDO HARIAN ATM" cash-balance report (DSR) as an Excel
workbook. Today there is no self-service path for a vendor to upload it and no
structured store of its contents, so the daily balance data can't be validated,
reconciled, or fed into the EOD forecast.

## Proposed outcome
<!-- What "better" looks like. -->
A vendor logs into the vendor portal, uploads one DSR workbook, and sees whether
it was accepted (parsed) or rejected (with a reason). Accepted data lands in
`atm_dsr_saldo_files` + `atm_dsr_saldo_rows`.

## Affected users and systems
- Users: Vendor (uploader), Operator/Admin (monitor ingest).
- Systems: `frontend/VendorPortal-Vite`, `backend` ATM API (`internal/dsr`),
  PostgreSQL primary, Cloud Storage (raw file), migration `013_dsr.sql` (existing schema).

## Constraints
<!-- Pull the non-negotiables from .claude/CLAUDE.md / .kiro steering. -->
- Vendor auth = local creds only; vendor scoped to own assignments (CLAUDE.md Sec 5).
- Idempotent per file checksum -> `import_jobs` / `atm_dsr_saldo_files.checksum` (Sec 5).
- Money as numeric, full IDR, verbatim signs (013_dsr.sql notes). Never float.
- Sync parse (<1000 rows) in the API handler; no queue (tech.md principle 8).
- Broken cells (`#REF!`) ingest as NULL + increment error_count; do not abort batch.
- Writes on primary; status read-after-write on primary (Sec 6).

## Open questions
- [ ] Does upload need maker-checker, or is ingest a system action with audit only?
- [ ] DSR deadline 09:00 (URS): does late upload block, or just flag for FLM penalty report?
- [ ] Is the raw .xlsx retained in Cloud Storage, or only the parsed rows?
- [ ] One workbook = one (report_date, vendor); how to handle re-upload of a corrected file?
