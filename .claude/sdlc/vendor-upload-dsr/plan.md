# Plan: vendor upload DSR (from intent.md + spec.md)

Status: draft. Stage: 3 Build. Reads: `spec.md`.
Produced in Claude Code plan mode. Keep in sync with the diff (update in the same commit).

## Files that change
<!-- Name real files. Fill during plan mode after reading the codebase. -->
- backend/internal/dsr/                 (parser, service, repository, handler) - TODO confirm existing layout
- backend/cmd/api/main.go               (register routes)
- frontend/VendorPortal-Vite/src/features/dsr/  (upload UI + status)
- backend/migrations/                   (none expected - 013_dsr.sql already defines schema)

## Order of work
1. Repository: insert `atm_dsr_saldo_files` (checksum-idempotent) + bulk insert rows. Primary pool.
2. Parser: xlsx sheet `Daily` -> leaf rows; `#REF!` -> NULL + error_count; skip derived rows.
3. Service: checksum, dedupe, parse, cross-check stated totals vs SUM(), set status.
4. Handler: `POST /uploads`, `GET /uploads/{id}`, `GET /uploads`; vendor scoping.
5. Audit: write `audit_logs` on ingest (who, what, when, ip).
6. Vendor portal: upload form + status badge (poll `GET /uploads/{id}`).

## Risks
- `atm_dsr_saldo_files.vendor` is free text; vendor scoping (FR8) needs resolved identity - risk to auth correctness.
- Sample workbook is `#REF!`-riddled; parser must be defensive, not abort (013_dsr.sql).
- Cross-check math (saldo_akhir = awal + SUM penerimaan + SUM pengeluaran) must respect verbatim negative signs.

## Proof (maps to tests.md)
- Parse the sample `DSR_DATA/*.xlsx` -> expected file record + row counts.
- Duplicate checksum -> no second parse.
- `#REF!` cell -> NULL + error_count incremented.
- Vendor A cannot read Vendor B's upload (RBAC denial).

## Alternatives not taken
- Async/queue parse: rejected, files <1000 rows -> sync (tech.md principle 8).
- Normalized long denom layout: rejected, wide layout is deliberate (013_dsr.sql).
