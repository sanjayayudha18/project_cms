-- Restore automatic updated_at maintenance, and index three uncovered foreign keys.
--
-- WHY: the schema declares updated_at on 16 tables but nothing ever set it, so every row's
-- updated_at was frozen at insert time and anything reading it was silently wrong.
-- This is a regression, not a new feature: archives/001_create_roles_vendors_users.sql
-- already had set_updated_at() with trg_<table>_set_updated_at on roles/vendors/users.
-- Those were lost when the schema was rebuilt from the pgAdmin ERD export
-- (002_cms_tables.sql), because the ERD tool does not emit functions or triggers.
-- Naming below matches the original so the two are recognisably the same thing.
--
-- SAFETY:
--   * DDL only, no data migration. Adding a trigger takes a brief ACCESS EXCLUSIVE lock to
--     update the catalog; it does not rewrite or touch existing rows.
--   * Existing rows keep their current (stale) updated_at. Backfilling would be a lie --
--     we do not know when those rows actually changed. From here forward the value is real.
--   * Indexes are built inline (not CONCURRENTLY) on purpose: the target tables hold
--     1,892 / 348 / 348 rows, so the build is milliseconds and keeping the migration atomic
--     is worth more than avoiding a sub-second write lock. If these tables ever reach
--     millions of rows, switch to CREATE INDEX CONCURRENTLY and drop the BEGIN/COMMIT
--     wrapper -- CONCURRENTLY cannot run inside a transaction block.
--   * Re-runnable: every statement is IF NOT EXISTS / OR REPLACE / DROP-then-CREATE.
--
-- ROLLBACK (forward-only shop, so documented rather than shipped as a .down.sql -- a down
-- file in this directory would also be picked up by sqlc's schema glob):
--   DO $$ DECLARE t text; BEGIN
--     FOREACH t IN ARRAY ARRAY['atm_vendor_packages','atms','currencies','denoms',
--       'dmaa_files','atm_dsr_saldo_files','itm_cashpos_files','itm_replenish_files','locations',
--       'regions','roles','users','vendor_branches','vendor_packages','vendor_vaults',
--       'vendors']
--     LOOP EXECUTE format('DROP TRIGGER IF EXISTS trg_%s_set_updated_at ON public.%I', t, t);
--     END LOOP;
--   END $$;
--   DROP FUNCTION IF EXISTS public.set_updated_at();
--   DROP INDEX IF EXISTS public.locations_region_idx;
--   DROP INDEX IF EXISTS public.vendor_branches_location_idx;
--   DROP INDEX IF EXISTS public.vendor_branches_vendor_idx;

BEGIN;

-- ============================================================
-- 1. updated_at maintenance
-- ============================================================
CREATE OR REPLACE FUNCTION public.set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

COMMENT ON FUNCTION public.set_updated_at()
    IS 'BEFORE UPDATE trigger: stamps updated_at = now(). Attached to every table declaring an updated_at column; see migration 014.';

-- One trigger per table carrying updated_at. Listed explicitly rather than discovered from
-- information_schema, so that covering a new table is a deliberate edit in a migration and
-- never a silent side effect of running this file again later.
--
-- The WHEN clause means an UPDATE that changes nothing does not bump updated_at, so the
-- column records actual changes rather than write attempts.
DO $$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'atm_vendor_packages',
        'atms',
        'currencies',
        'denoms',
        'dmaa_files',
        'atm_dsr_saldo_files',
        'itm_cashpos_files',
        'itm_replenish_files',
        'locations',
        'regions',
        'roles',
        'users',
        'vendor_branches',
        'vendor_packages',
        'vendor_vaults',
        'vendors'
    ]
    LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS trg_%s_set_updated_at ON public.%I', t, t);
        EXECUTE format(
            'CREATE TRIGGER trg_%s_set_updated_at
                 BEFORE UPDATE ON public.%I
                 FOR EACH ROW
                 WHEN (OLD.* IS DISTINCT FROM NEW.*)
                 EXECUTE FUNCTION public.set_updated_at()', t, t);
    END LOOP;
END;
$$;

-- ============================================================
-- 2. Missing foreign-key indexes
-- ============================================================
-- Without these, deleting a parent row (or joining from it) sequential-scans the child.
-- Named to match the convention already used in 002_cms_tables.sql: <table>_<col>_idx.
CREATE INDEX IF NOT EXISTS locations_region_idx
    ON public.locations (region_id);

CREATE INDEX IF NOT EXISTS vendor_branches_location_idx
    ON public.vendor_branches (location_id);

CREATE INDEX IF NOT EXISTS vendor_branches_vendor_idx
    ON public.vendor_branches (vendor_id);

COMMIT;

-- Statistics have never been gathered on this database (22/22 tables with NULL last_analyze
-- AND last_autoanalyze), so the planner cannot make good use of these indexes yet.
-- Run once after applying, outside this transaction:
--     ANALYZE;
