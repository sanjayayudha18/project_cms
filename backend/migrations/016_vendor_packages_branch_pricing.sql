-- 016_vendor_packages_branch_pricing.sql
-- Turns vendor_packages_branch into a branch-scoped special-price table,
-- same field shape as vendor_package_prices (explicit user request,
-- 2026-09-23, plan confirmed via AskUserQuestion). vendor_package_prices
-- itself is UNCHANGED and stays the vendor-wide (PT/branch/ATM override)
-- price source for /settings/admin/vendors/:id -- this table is specifically
-- for "cabang vendor punya harga khusus" (this branch has a custom price),
-- entered directly on the branch detail page.
--
-- Table was empty (TRUNCATEd in migration 011, never re-seeded), so this is
-- a structural change only, no data to migrate.
--
-- BREAKING: atm_vendor_packages.vendor_package_id (NOT NULL FK, ATM
-- assignment / kelolaan feature) still points at one row here. Before this
-- migration, one row = one package-at-a-branch, so an assignment was
-- unambiguous. After this migration, a package_code can have several rows
-- (one per machine_group/price_class/tier/period, like vendor_package_prices)
-- -- an assignment now points at one specific priced row. Per explicit user
-- decision, this migration does NOT add validation that the chosen row's
-- machine_group/price_class matches the assigned ATM's own attributes --
-- flagged as a known gap, not silently swept under.

BEGIN;

ALTER TABLE public.vendor_packages_branch
    DROP CONSTRAINT vendor_packages_branch_code_uq;

ALTER TABLE public.vendor_packages_branch
    RENAME COLUMN code TO package_code;

ALTER TABLE public.vendor_packages_branch
    DROP COLUMN is_active,
    DROP COLUMN deleted_at;

ALTER TABLE public.vendor_packages_branch
    ADD COLUMN machine_group text,
    ADD COLUMN price_class   text,
    ADD COLUMN base_price    numeric(20,2),
    ADD COLUMN atm_id        bigint REFERENCES public.atms(id),
    ADD COLUMN sla_note      text,
    ADD COLUMN currency      char(3) NOT NULL DEFAULT 'IDR';

-- Backfill NOT NULL grain columns before enforcing NOT NULL -- table is
-- empty, but UPDATE is here for correctness if that assumption ever changes.
UPDATE public.vendor_packages_branch SET machine_group = 'ATM' WHERE machine_group IS NULL;
UPDATE public.vendor_packages_branch SET price_class = 'REGULAR' WHERE price_class IS NULL;

ALTER TABLE public.vendor_packages_branch
    ALTER COLUMN machine_group SET NOT NULL,
    ALTER COLUMN price_class SET NOT NULL;

ALTER TABLE public.vendor_packages_branch
    ADD CONSTRAINT vpb_machine_group_chk CHECK (machine_group IN ('ATM', 'CDM_CRM')),
    ADD CONSTRAINT vpb_price_class_chk   CHECK (price_class IN ('REGULAR', 'VIP_INDUSTRI')),
    ADD CONSTRAINT vpb_amount_chk        CHECK (base_price IS NULL OR base_price >= 0);

-- Same overlap guard as vpp_no_overlap (migration 009): prevents two rows
-- covering the same package/machine/class/tier/period at the same branch.
-- COALESCE(atm_id, 0) for the same reason vpp_no_overlap needs it -- EXCLUDE
-- treats NULL as never-colliding, so without it two branch-level rows (both
-- atm_id NULL) for the same grain+period would NOT be caught.
ALTER TABLE public.vendor_packages_branch
    ADD CONSTRAINT vpb_no_overlap EXCLUDE USING gist (
        vendor_branch_id              WITH =,
        package_code                  WITH =,
        machine_group                 WITH =,
        price_class                   WITH =,
        COALESCE(atm_id, 0)           WITH =,
        int4range(tier_min, COALESCE(tier_max, 999999999), '[]') WITH &&,
        daterange(effective_start_date, effective_end_date, '[]') WITH &&
    );

COMMENT ON TABLE public.vendor_packages_branch
    IS 'Branch-specific special/custom package price ("harga khusus cabang"), effective-dated with history (never hard-deleted -- "disable" closes the period via effective_end_date, no re-enable). Distinct from vendor_package_prices, which holds the vendor-wide PT/branch/ATM-override price tree shown on the vendor page. Migrasi 016.';
COMMENT ON COLUMN public.vendor_packages_branch.atm_id IS 'Optional further override: this branch price applies to one specific ATM only. Migrasi 016.';
COMMENT ON COLUMN public.vendor_packages_branch.base_price IS 'NULLABLE like vendor_package_prices.base_price -- not defined here means this branch has no special price for this grain (falls back to vendor_package_prices).';

COMMIT;
