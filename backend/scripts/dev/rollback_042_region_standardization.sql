-- rollback_042_region_standardization.sql
-- Undo backend/migrations/042_region_standardization.sql from migration_042_backup
-- (spec .kiro/specs/region-standardization/, Req 8.3). Run by hand, with the
-- user's OK, only while migration_042_backup still exists:
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -X -f backend/scripts/dev/rollback_042_region_standardization.sql
-- Stop the new backend first: after this script the old backend (with
-- vendor_branches.region) is the one that matches the schema.
-- Data written after 042 is not undone: branches go back to their backup
-- values; rows created after 042 keep theirs.

BEGIN;

DO $$
BEGIN
    IF to_regclass('public.migration_042_backup') IS NULL THEN
        RAISE EXCEPTION 'rollback 042: migration_042_backup not found — nothing to restore';
    END IF;
END $$;

ALTER TABLE public.vendor_branches DROP CONSTRAINT IF EXISTS vendor_branches_region_code_fk;
ALTER TABLE public.vendor_branches ADD COLUMN region text;
COMMENT ON COLUMN public.vendor_branches.region IS
    'Vendor regional grouping from MASTER_ATM_ESQ.FLMVendorRegion (e.g. "Jakarta Timur"). Distinct from regions.region (ATM geographic area) and branch_name (vendor sub-region). See migration 030.';

UPDATE public.vendor_branches b
   SET region = k.payload->>'region', region_code = k.payload->>'region_code'
  FROM public.migration_042_backup k
 WHERE k.entity = 'vendor_branches' AND k.entity_key = b.id::text;

UPDATE public.regions r
   SET code = k.payload->>'code', region = k.payload->>'region'
  FROM public.migration_042_backup k
 WHERE k.entity = 'regions' AND k.entity_key = r.id::text;

DELETE FROM public.acm_area_regions;
INSERT INTO public.acm_area_regions
SELECT (jsonb_populate_record(NULL::public.acm_area_regions, k.payload)).*
  FROM public.migration_042_backup k WHERE k.entity = 'acm_area_regions';

DELETE FROM public.cash_count_area_regions;
INSERT INTO public.cash_count_area_regions
SELECT (jsonb_populate_record(NULL::public.cash_count_area_regions, k.payload)).*
  FROM public.migration_042_backup k WHERE k.entity = 'cash_count_area_regions';

UPDATE public.cash_count_pics p SET region_code = k.payload->>'region_code'
  FROM public.migration_042_backup k
 WHERE k.entity = 'cash_count_pics' AND k.entity_key = p.id::text;

DROP TABLE public.region_cities;
DROP TABLE public.migration_042_backup;
DELETE FROM public.schema_migrations WHERE version = '042_region_standardization.sql';

COMMIT;
