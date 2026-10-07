-- 015_vendor_packages_branch_tiering.sql
-- Add tier + effective-date columns to vendor_packages_branch, structurally
-- aligned with vendor_package_prices (tier_min/tier_max, effective_start/end
-- date) -- explicit user request, 2026-09-23. NO price columns: pricing stays
-- exclusively in vendor_package_prices (Sec 3/12 CLAUDE.md decision), this
-- table remains a kelolaan/frequency link, now with the same tier/period
-- shape so a package row can describe which kelolaan-count tier and date
-- range it applies to.
--
-- Additive + safe: vendor_packages_branch was TRUNCATEd in migration 011 and
-- is currently empty, so DEFAULTs here never need to backfill real rows.

BEGIN;

ALTER TABLE public.vendor_packages_branch
    ADD COLUMN tier_min integer NOT NULL DEFAULT 1,
    ADD COLUMN tier_max integer,
    ADD COLUMN effective_start_date date NOT NULL DEFAULT CURRENT_DATE,
    ADD COLUMN effective_end_date date;

ALTER TABLE public.vendor_packages_branch
    ADD CONSTRAINT vendor_packages_branch_tier_chk
        CHECK (tier_min >= 1 AND (tier_max IS NULL OR tier_max >= tier_min)),
    ADD CONSTRAINT vendor_packages_branch_period_chk
        CHECK (effective_end_date IS NULL OR effective_end_date >= effective_start_date);

COMMENT ON COLUMN public.vendor_packages_branch.tier_min IS 'Kelolaan-count tier lower bound, same grain as vendor_package_prices.tier_min. Migrasi 015.';
COMMENT ON COLUMN public.vendor_packages_branch.tier_max IS 'Kelolaan-count tier upper bound, inclusive; NULL = open-ended. Migrasi 015.';
COMMENT ON COLUMN public.vendor_packages_branch.effective_start_date IS 'Package period start, same grain as vendor_package_prices.effective_start_date. Migrasi 015.';
COMMENT ON COLUMN public.vendor_packages_branch.effective_end_date IS 'Package period end; NULL = open-ended. Migrasi 015.';

COMMIT;
