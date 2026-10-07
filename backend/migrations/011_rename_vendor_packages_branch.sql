-- Renames vendor_packages -> vendor_packages_branch and clears its data
-- (explicit user request, 2026-09-23). vendor_packages_branch is referenced
-- by atm_vendor_packages.vendor_package_id (NOT NULL FK), so TRUNCATE
-- CASCADE also empties atm_vendor_packages (ATM<->vendor assignments) --
-- confirmed with the user before applying (562 vendor_packages rows, 1862
-- atm_vendor_packages rows at the time this migration was written).
-- Indexes, constraints, the trigger and the table comment carry over
-- automatically on rename; only the table name itself changes.

ALTER TABLE public.vendor_packages RENAME TO vendor_packages_branch;

TRUNCATE TABLE public.vendor_packages_branch CASCADE;
