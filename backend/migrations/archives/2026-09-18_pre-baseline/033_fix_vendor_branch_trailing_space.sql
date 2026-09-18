-- Trim trailing whitespace from two vendor_branches.branch_name values that
-- migration 006 seeded with a trailing space, unlike every other branch.
--
-- WHY: found while verifying migration 032 (.kiro/specs/update-cit-forecast-browser/
-- tasks.md, Task 0.5). 'Kas Mobil - Surabaya ' and 'Tangerang - Serpong ' (both ROH)
-- have a trailing space in the live vendor_branches table (confirmed via
-- length(branch_name): 21/20 chars vs. the expected 20/19). MASTER_ATM_ESQ.csv.xlsx
-- also has the trailing space in FLMVendorSubRegion for these rows, but the seed
-- generator (scripts/gen_master_atm_esq_seed.py) strips it via norm_text(), so
-- migration 032's region/PAKET/link joins on exact branch_name never matched these
-- two branches -- they were silently skipped (0 vendor_packages, NULL region,
-- no atm_vendor_packages links), same class of bug as the ABACUS/ADVANTAGE vendor-code
-- casing issue fixed in the same task.
--
-- SAFETY:
--   * Only touches the two known-affected rows, matched by trailing-space name.
--     No-op (0 rows) if already trimmed -- safe to re-run.
--   * branch_name has no unique constraint on its own (unique key is
--     (vendor_id, branch_code) per migration 002), so trimming cannot collide with
--     an existing untrimmed row of the same vendor.
--   * After this runs, re-run migrations 031 and 032 (idempotent, WHERE NOT EXISTS)
--     to backfill the region + insert the vendor_packages/atm_vendor_packages rows
--     for these two branches that could not match before.
--
-- ROLLBACK (documented, forward-only): re-append a trailing space, e.g.
--   UPDATE public.vendor_branches SET branch_name = branch_name || ' '
--   WHERE branch_name IN ('Kas Mobil - Surabaya', 'Tangerang - Serpong');

BEGIN;

UPDATE public.vendor_branches
SET branch_name = trim(trailing ' ' from branch_name)
WHERE branch_name IN ('Kas Mobil - Surabaya ', 'Tangerang - Serpong ');

COMMIT;
