-- 037_export_jobs_vendor_pks.sql
-- vendor-pks (.kiro/specs/vendor-pks/, task 2.1). Additive, forward-only. Two contents:
--   1. export_jobs: kind 'vendor_pks' (laporan PKS vendor) joins the report kinds of 035. The kind/params/scope
--      constraints are replaced by versions that admit it; rows written under 035 still pass.
--   2. vendor_pics_director_roles_chk: Direktur Utama (is_director, column from 022) never has roles.
--      Needs vendor_pics.roles from 032 and vendor_pics.is_director from 022 (022 must be applied first).
--
-- ROLLBACK (documented, forward-only shop):
--   ALTER TABLE public.vendor_pics DROP CONSTRAINT vendor_pics_director_roles_chk;
--   then restore export_jobs_kind_chk / export_jobs_params_kind_chk / export_jobs_scope_chk as in 035
--   (after deleting any export_jobs rows with kind = 'vendor_pks').

BEGIN;

ALTER TABLE public.export_jobs DROP CONSTRAINT export_jobs_kind_chk;
ALTER TABLE public.export_jobs ADD CONSTRAINT export_jobs_kind_chk CHECK (kind IN (
    'vendor', 'cabang', 'harga_paket', 'master_data_global',
    'dsr_late_daily', 'dsr_late_summary', 'replenishment_amount_mismatch', 'replenishment_trips',
    'vendor_pks'));

ALTER TABLE public.export_jobs DROP CONSTRAINT export_jobs_params_kind_chk;
ALTER TABLE public.export_jobs ADD CONSTRAINT export_jobs_params_kind_chk CHECK (
       (kind IN ('vendor', 'cabang', 'harga_paket', 'master_data_global')
        AND status_filter IS NOT NULL AND report_params IS NULL)
    OR (kind IN ('dsr_late_daily', 'dsr_late_summary', 'replenishment_amount_mismatch', 'replenishment_trips',
                 'vendor_pks')
        AND status_filter IS NULL AND report_params IS NOT NULL AND jsonb_typeof(report_params) = 'object'));

ALTER TABLE public.export_jobs DROP CONSTRAINT export_jobs_scope_chk;
ALTER TABLE public.export_jobs ADD CONSTRAINT export_jobs_scope_chk CHECK (
       (kind = 'master_data_global' AND vendor_id IS NULL     AND vendor_branch_id IS NULL)
    OR (kind = 'vendor'             AND vendor_id IS NOT NULL AND vendor_branch_id IS NULL)
    OR (kind = 'cabang'             AND vendor_id IS NOT NULL AND vendor_branch_id IS NOT NULL)
    OR (kind = 'harga_paket'        AND vendor_id IS NOT NULL)
    OR (kind IN ('dsr_late_daily', 'dsr_late_summary', 'replenishment_amount_mismatch', 'replenishment_trips',
                 'vendor_pks')
        AND vendor_id IS NULL AND vendor_branch_id IS NULL));

COMMENT ON COLUMN public.export_jobs.report_params IS
    'Filter laporan (month | date_from,date_to, vendor_id, vendor_branch_id, region_code, status, classification; vendor_pks: q, status, pks_status) untuk Kind laporan; NULL untuk Kind master data. Migrasi 035, 037.';

-- Direktur Utama tidak punya peran vendor (replenish/vault/cashcount).
ALTER TABLE public.vendor_pics ADD CONSTRAINT vendor_pics_director_roles_chk
    CHECK (NOT is_director OR roles = '{}'::text[]);

COMMIT;
