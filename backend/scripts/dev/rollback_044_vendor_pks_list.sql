-- rollback_044_vendor_pks_list.sql
-- Undo backend/migrations/044_vendor_pks_list.sql (spec .kiro/specs/vendor-pks-list/, task 1.2, Req 1.1, 9.1).
-- DEV ONLY. Run by hand, with the user's OK:
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -X -f backend/scripts/dev/rollback_044_vendor_pks_list.sql
-- Stop the new backend first: after this script the pre-044 backend (status via vendor_pks_status(),
-- no renewal_days, no settings.pks menu) is the one that matches the schema.
-- Dropping renewal_days discards any per-PKS reminder value (dev data only; 044 backfill is not reversible).
-- vendor_pks_status() / vendor_pks_validity_months() were never dropped by 044, so nothing to restore.

BEGIN;

-- 4. role_permissions + menu (settings.pks, and the ATM-SPV 'settings' parent grant 044 added)
DELETE FROM public.role_permissions rp
USING public.menu_features mf
WHERE rp.menu_feature_id = mf.id AND mf.key = 'settings.pks';

-- ATM-SPV 'settings' grant: remove only when ATM-SPV holds no other settings.* child (it may predate 044).
DELETE FROM public.role_permissions rp
USING public.roles r, public.menu_features mf
WHERE rp.role_id = r.id AND r.role = 'ATM-SPV'
  AND rp.menu_feature_id = mf.id AND mf.key = 'settings'
  AND NOT EXISTS (
      SELECT 1
      FROM public.role_permissions rp2
      JOIN public.menu_features c ON c.id = rp2.menu_feature_id
      WHERE rp2.role_id = r.id AND c.parent_id = mf.id AND c.key <> 'settings.pks'
  );

DELETE FROM public.menu_features WHERE key = 'settings.pks';

-- 3. status function
DROP FUNCTION IF EXISTS public.vendor_pks_record_status(timestamptz, timestamptz, date, date, int, date);

-- 2. renewal_days (both CHECKs go with the column; dropped explicitly for clarity)
ALTER TABLE public.vendor_pks
    DROP CONSTRAINT IF EXISTS vendor_pks_renewal_days_le_term_chk,
    DROP CONSTRAINT IF EXISTS vendor_pks_renewal_days_chk,
    DROP COLUMN IF EXISTS renewal_days;

-- 5. table comment back to the 043 wording
COMMENT ON TABLE public.vendor_pks IS
    'PKS (perjanjian kerja sama) per vendor per type dengan riwayat (D1; 043). Satu PKS aktif per (vendor, type) (replaced_at IS NULL, vendor_pks_active_uq); vendor boleh punya subset type. Ditulis hanya lewat maker-checker master data (entity_type vendor_pks). pks_name/effective_date/expiry_date/pks_type_id/wod_number tidak pernah di-UPDATE; tidak pernah di-hard-delete. Riwayat batalkan terminate hanya di audit_logs.';

-- migration bookkeeping: let `migrate up` apply 044 again
DELETE FROM public.schema_migrations WHERE version = '044_vendor_pks_list.sql';

COMMIT;
