-- vendor_packages: add soft-disable columns so packages can be retired
-- through maker-checker (plan.md T3.4), same convention as vendor_branches /
-- vendor_vaults / vendor_pics (is_active + deleted_at, never a hard DELETE).
--
-- SAFETY:
--   * Additive. is_active NOT NULL DEFAULT true backfills all existing rows
--     (560 at time of writing) as active; deleted_at is nullable.
--   * Forward-only: no down migration ships (project convention).

BEGIN;

ALTER TABLE public.vendor_packages
    ADD COLUMN is_active boolean NOT NULL DEFAULT true,
    ADD COLUMN deleted_at timestamp with time zone;

COMMENT ON COLUMN public.vendor_packages.deleted_at IS 'Soft-disable timestamp (set with is_active=false). Never hard-delete: atm_vendor_packages references this table.';

COMMIT;
