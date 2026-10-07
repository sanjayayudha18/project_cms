-- regions: add soft-disable columns so a region can be deactivated through
-- the immediate-apply-with-audit admin flow (Region Management, Req 4), same
-- convention as vendor_packages/vendor_branches (is_active + deleted_at,
-- never a hard DELETE). uq_regions_code + trg_regions_set_updated_at already
-- exist in the baseline.
--
-- SAFETY:
--   * Additive. is_active NOT NULL DEFAULT true backfills all 13 existing
--     regions as active; deleted_at is nullable.
--   * Forward-only: no down migration (project convention).
BEGIN;

ALTER TABLE public.regions
    ADD COLUMN is_active boolean NOT NULL DEFAULT true,
    ADD COLUMN deleted_at timestamp with time zone;

COMMENT ON COLUMN public.regions.deleted_at IS
  'Soft-disable timestamp (set with is_active=false). Never hard-delete: locations.region_id references this table.';

COMMIT;
