-- 035_export_jobs_reports.sql
-- report-export-migration (.kiro/specs/report-export-migration/design.md, Data Models): the
-- DSR late and replenishment report CSVs move onto the backend-exq export jobs of 034.
-- Additive: one nullable column, and the kind/scope constraints are replaced by versions that
-- admit the four report kinds while staying as strict as before for the master-data kinds.
-- Rows written under 034 still pass every new constraint.

BEGIN;

ALTER TABLE public.export_jobs ADD COLUMN report_params jsonb;

ALTER TABLE public.export_jobs DROP CONSTRAINT export_jobs_kind_chk;
ALTER TABLE public.export_jobs ADD CONSTRAINT export_jobs_kind_chk CHECK (kind IN (
    'vendor', 'cabang', 'harga_paket', 'master_data_global',
    'dsr_late_daily', 'dsr_late_summary', 'replenishment_amount_mismatch', 'replenishment_trips'));

-- status_filter hanya untuk Kind master data; report_params hanya untuk Kind laporan.
ALTER TABLE public.export_jobs ALTER COLUMN status_filter DROP NOT NULL;
ALTER TABLE public.export_jobs ADD CONSTRAINT export_jobs_params_kind_chk CHECK (
       (kind IN ('vendor', 'cabang', 'harga_paket', 'master_data_global')
        AND status_filter IS NOT NULL AND report_params IS NULL)
    OR (kind IN ('dsr_late_daily', 'dsr_late_summary', 'replenishment_amount_mismatch', 'replenishment_trips')
        AND status_filter IS NULL AND report_params IS NOT NULL AND jsonb_typeof(report_params) = 'object'));

ALTER TABLE public.export_jobs DROP CONSTRAINT export_jobs_scope_chk;
ALTER TABLE public.export_jobs ADD CONSTRAINT export_jobs_scope_chk CHECK (
       (kind = 'master_data_global' AND vendor_id IS NULL     AND vendor_branch_id IS NULL)
    OR (kind = 'vendor'             AND vendor_id IS NOT NULL AND vendor_branch_id IS NULL)
    OR (kind = 'cabang'             AND vendor_id IS NOT NULL AND vendor_branch_id IS NOT NULL)
    OR (kind = 'harga_paket'        AND vendor_id IS NOT NULL)
    OR (kind IN ('dsr_late_daily', 'dsr_late_summary', 'replenishment_amount_mismatch', 'replenishment_trips')
        AND vendor_id IS NULL AND vendor_branch_id IS NULL));

COMMENT ON COLUMN public.export_jobs.report_params IS
    'Filter laporan (month | date_from,date_to, vendor_id, vendor_branch_id, region_code, status, classification) untuk Kind laporan; NULL untuk Kind master data. Migrasi 035.';

COMMIT;
