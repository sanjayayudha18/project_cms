-- rollback_043_vendor_pks_type.sql
-- Undo backend/migrations/043_vendor_pks_type.sql (spec .kiro/specs/vendor-pks-type/, task 1.2, Req 9.1).
-- DEV ONLY. Run by hand, with the user's OK:
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -X -f backend/scripts/dev/rollback_043_vendor_pks_type.sql
-- Stop the new backend first: after this script the pre-043 backend (one active PKS per vendor,
-- no type / No. WOD) is the one that matches the schema.
-- Aborts unless vendor_pks is empty and no vendor_pks / vendor_pks_type change is pending:
-- dropping pks_type_id / wod_number would lose data, and the per-vendor indexes cannot be rebuilt
-- over per-type rows. Clear the rows (or resolve the changes) first if you really want to roll back.

BEGIN;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM public.vendor_pks) THEN
        RAISE EXCEPTION 'rollback 043: vendor_pks berisi data; kosongkan dulu (rollback hanya untuk dev)';
    END IF;
    IF EXISTS (SELECT 1 FROM public.master_data_change_requests
               WHERE entity_type IN ('vendor_pks', 'vendor_pks_type') AND status = 'pending') THEN
        RAISE EXCEPTION 'rollback 043: ada change vendor_pks / vendor_pks_type pending; selesaikan dulu';
    END IF;
END
$$;

-- 5. pending-change indexes: drop the new ones, restore the per-vendor one (022, D24)
DROP INDEX IF EXISTS public.master_data_change_requests_pks_type_code_pending_uq;
DROP INDEX IF EXISTS public.master_data_change_requests_vendor_pks_wod_pending_uq;
DROP INDEX IF EXISTS public.master_data_change_requests_vendor_pks_pending_uq;
CREATE UNIQUE INDEX master_data_change_requests_vendor_pks_pending_uq
    ON public.master_data_change_requests (((payload->>'vendor_id')::bigint))
    WHERE entity_type = 'vendor_pks' AND status = 'pending';

-- 4. vendor_pks indexes: drop the new ones, restore the per-vendor one (022)
DROP INDEX IF EXISTS public.vendor_pks_type_active_idx;
DROP INDEX IF EXISTS public.vendor_pks_wod_uq;
DROP INDEX IF EXISTS public.vendor_pks_active_uq;
CREATE UNIQUE INDEX vendor_pks_active_uq
    ON public.vendor_pks (vendor_id)
    WHERE replaced_at IS NULL;

-- 3. columns (the wod CHECK goes with the column; the FK with pks_type_id)
ALTER TABLE public.vendor_pks
    DROP COLUMN IF EXISTS wod_number,
    DROP COLUMN IF EXISTS pks_type_id;

COMMENT ON TABLE public.vendor_pks IS
    'PKS (perjanjian kerja sama) per vendor dengan riwayat (D1). Satu PKS aktif per vendor (replaced_at IS NULL, vendor_pks_active_uq). Ditulis hanya lewat maker-checker master data (entity_type vendor_pks). pks_name/effective_date/expiry_date tidak pernah di-UPDATE; tidak pernah di-hard-delete. Riwayat batalkan terminate hanya di audit_logs.';
COMMENT ON COLUMN public.vendor_pks.replaced_at IS
    'NULL = PKS aktif vendor. Diisi saat PKS baru disetujui dan menggantikannya (D17).';

-- 2. master type table (trigger drops with it)
DROP TABLE IF EXISTS public.vendor_pks_types;

-- migration bookkeeping: let `migrate up` apply 043 again
DELETE FROM public.schema_migrations WHERE version = '043_vendor_pks_type.sql';

COMMIT;
