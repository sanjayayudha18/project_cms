-- Queries for the vendor DSR upload feature (.claude/sdlc/vendor-upload-dsr/).
-- Upload rows are written by backend_python/dsr/dsr_etl.py (Python), not this Go service --
-- these queries only READ dsr_uploads / dsr_daily_rows / dsr_rencana_isi_rows
-- (020_dsr_daily_rencana_isi.sql) to resolve the vendor and build the upload response.
--
-- One workbook = one dsr_uploads row carrying BOTH sheets, so the paired
-- per-sheet lookups and the UNION over two file tables the old schema needed
-- are gone. The API still reports a file_id per sheet; both now carry the same
-- upload id, keeping the vendor-portal contract unchanged.

-- name: GetVendorByID :one
-- Resolves vendor_code + display name from the JWT's vendor_id claim -- never trust a
-- client-supplied vendor identifier (intent.md constraint).
SELECT id, code, name
FROM vendors
WHERE id = sqlc.arg('id')::bigint
  AND is_active = true
  AND deleted_at IS NULL;

-- name: GetDsrUploadByChecksum :one
SELECT id, filename, checksum, vendor, report_date, daily_status_date, rencana_isi_plan_date,
       daily_status, daily_row_count, daily_error_count,
       rencana_isi_status, rencana_isi_row_count, rencana_isi_error_count,
       saldo_akhir_0000_idr, saldo_sementara_0900_idr, status_cadangan_idr,
       saldo_gabungan_idr, rencana_isi_subtotal_idr,
       error_message, processed_at, created_at
FROM dsr_uploads
WHERE checksum = sqlc.arg('checksum')::text;

-- name: GetDsrUploadByIDForVendor :one
-- Vendor scoping enforced here too (not just middleware/service layer) -- a vendor
-- JWT can never fetch another vendor's upload id even via a crafted request.
SELECT id, filename, checksum, vendor, report_date, daily_status_date, rencana_isi_plan_date,
       daily_status, daily_row_count, daily_error_count,
       rencana_isi_status, rencana_isi_row_count, rencana_isi_error_count,
       saldo_akhir_0000_idr, saldo_sementara_0900_idr, status_cadangan_idr,
       saldo_gabungan_idr, rencana_isi_subtotal_idr,
       error_message, processed_at, created_at
FROM dsr_uploads
WHERE id = sqlc.arg('id')::bigint
  AND vendor = sqlc.arg('vendor')::text;

-- name: ListDsrDailyRowErrors :many
-- Rows with a broken (#REF!/unparseable) denom cell -- the documented ingest
-- tolerance: NULL + daily_error_count++, never abort the file.
SELECT row_no, section, flow, line_label, memo_no
FROM dsr_daily_rows
WHERE upload_id = sqlc.arg('upload_id')::bigint
  AND (denom_100k_idr IS NULL OR denom_50k_idr IS NULL OR denom_20k_idr IS NULL
       OR denom_10k_idr IS NULL OR denom_5k_idr IS NULL OR denom_2k_idr IS NULL
       OR denom_1k_idr IS NULL)
ORDER BY row_no;

-- name: ListDsrRencanaIsiRowErrors :many
-- Rows whose vendor-sent ATM terminal id didn't resolve against atms.terminal_id.
-- These now INGEST (atm_id NULL) instead of being rejected by a hard FK, so this
-- list is the vendor's fix-up worklist, not a record of dropped rows.
SELECT row_no, atm_terminal_id, atm_location
FROM dsr_rencana_isi_rows
WHERE upload_id = sqlc.arg('upload_id')::bigint
  AND atm_id IS NULL
ORDER BY row_no;

-- name: ListDsrDailyRows :many
-- Full row detail for the vendor-facing "view uploaded DSR" screen.
SELECT row_no, section, flow, line_label, memo_no,
       denom_100k_idr, denom_50k_idr, denom_20k_idr, denom_10k_idr,
       denom_5k_idr, denom_2k_idr, denom_1k_idr,
       line_total_idr, remarks
FROM dsr_daily_rows
WHERE upload_id = sqlc.arg('upload_id')::bigint
ORDER BY row_no;

-- name: ListDsrRencanaIsiRows :many
-- Full row detail for the vendor-facing "view uploaded DSR" screen.
SELECT row_no, atm_terminal_id, atm_location, denom_config,
       fill_100k_idr, fill_50k_idr, splank_balance_0800_idr, remarks
FROM dsr_rencana_isi_rows
WHERE upload_id = sqlc.arg('upload_id')::bigint
ORDER BY row_no;

-- name: ListDsrUploadsByVendor :many
-- One row per uploaded workbook for this vendor, newest first. Both sheets'
-- statuses come back on the same row -- no per-date follow-up query needed.
SELECT id, report_date, daily_status, rencana_isi_status
FROM dsr_uploads
WHERE vendor = sqlc.arg('vendor')::text
  AND (sqlc.arg('date_from')::text = '' OR report_date >= sqlc.arg('date_from')::date)
  AND (sqlc.arg('date_to')::text = '' OR report_date <= sqlc.arg('date_to')::date)
ORDER BY report_date DESC
LIMIT sqlc.arg('page_size')::int OFFSET (sqlc.arg('page')::int - 1) * sqlc.arg('page_size')::int;

-- name: CountDsrUploadsByVendor :one
-- Mirrors ListDsrUploadsByVendor's filters for pagination total.
SELECT COUNT(*)
FROM dsr_uploads
WHERE vendor = sqlc.arg('vendor')::text
  AND (sqlc.arg('date_from')::text = '' OR report_date >= sqlc.arg('date_from')::date)
  AND (sqlc.arg('date_to')::text = '' OR report_date <= sqlc.arg('date_to')::date);
