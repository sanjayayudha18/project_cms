-- Add users.vendor_branch_id so a vendor user can be scoped to a specific branch
-- (e.g. the "Bijak Jakarta" branch), not just to the vendor.
--
-- WHY: public.users only had vendor_id, which scopes a login to a whole VENDOR. Vendor
-- operations are run per branch, so a vendor user needs to be pinned to one vendor_branch.
--
-- SAFETY:
--   * Additive, nullable column + FK + index. No data rewrite.
--   * NULLable on purpose: internal LDAP users have no vendor/branch, and existing vendor
--     users predate this column. A NOT NULL default is impossible (no sensible default
--     branch) and would break the 9 rows seeded by 003_seed_roles_users.sql.
--   * Plain FK to vendor_branches(id). Postgres cannot cheaply enforce that
--     vendor_branch_id belongs to the same vendor as vendor_id; that consistency is the
--     application's responsibility (enforced when a user is created/assigned). Documented
--     here so it is not mistaken for a gap.
--   * Index mirrors the existing users_vendor_idx / users_role_idx convention. Table is
--     small (single-digit thousands of rows), so an inline build locks for milliseconds.

BEGIN;

ALTER TABLE public.users
    ADD COLUMN IF NOT EXISTS vendor_branch_id bigint;

COMMENT ON COLUMN public.users.vendor_branch_id
    IS 'Optional: pins a vendor user to one vendor_branches row. Must belong to the same vendor as vendor_id (enforced in app). NULL for internal/LDAP users.';

ALTER TABLE public.users
    DROP CONSTRAINT IF EXISTS users_vendor_branch_fk;

ALTER TABLE public.users
    ADD CONSTRAINT users_vendor_branch_fk FOREIGN KEY (vendor_branch_id)
    REFERENCES public.vendor_branches (id) MATCH SIMPLE
    ON UPDATE NO ACTION
    ON DELETE NO ACTION;

CREATE INDEX IF NOT EXISTS users_vendor_branch_idx
    ON public.users(vendor_branch_id);

COMMIT;
