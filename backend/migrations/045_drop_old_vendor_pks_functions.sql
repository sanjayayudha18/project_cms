-- 045_drop_old_vendor_pks_functions.sql
-- Migrasi pembersih vendor-pks-list (.kiro/specs/vendor-pks-list/, task 10, Req 3.6).
-- Drop fungsi lama dari 022 yang sudah diganti vendor_pks_record_status() (044):
--   * vendor_pks_status(text, date, timestamptz, date, int)
--   * vendor_pks_validity_months(date, date)
-- Prasyarat (dicek 2026-11-19): tidak ada pemanggil di backend, backend-exq, backend_python (kode, query,
-- sqlc, test); tidak ada view/fungsi/constraint DB yang bergantung; 044 tidak memanggilnya.
-- Jalankan setelah rilis 044 berjalan di semua lingkungan (binary lama masih memanggil fungsi ini).
-- DROP tanpa IF EXISTS dan tanpa CASCADE: gagal keras bila fungsi hilang atau masih ada dependensi.
-- Rollback dev: backend/scripts/dev/rollback_045_drop_old_vendor_pks_functions.sql

BEGIN;

DROP FUNCTION public.vendor_pks_status(text, date, timestamptz, date, int);
DROP FUNCTION public.vendor_pks_validity_months(date, date);

COMMIT;
