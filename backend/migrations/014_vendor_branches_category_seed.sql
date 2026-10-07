-- Seeds vendor_branches.category (added in 013) and cleans up
-- vendor_branches.region, per user request with the master vendor/tipe
-- list (2026-09-23).
--
-- Category: rather than re-parsing the pasted "<Vendor> <City>  <Tipe>"
-- text and matching it back to branch_name by string (error-prone, and
-- migration 008 already seeded the exact same tipe data onto
-- vendor_vaults.category per branch_code -- confirmed by cross-checking
-- the pasted list against it: 96/99 exact matches, the other 3 only
-- differed because those branches were renamed to longer names in 008
-- step 2, same category value), this copies vendor_vaults.category onto
-- the owning vendor_branches row via the vendor_branch_id FK. Each branch
-- has at most one vault in practice (DISTINCT ON guards against the
-- unenforced 1:1 anyway); branches with no vault (8 hub branches, see
-- migration 007) or ROH branches (not in the master tipe list) keep the
-- 'ATM' default from migration 013.
--
-- Region: strips the leading vendor name that migration 003's seed baked
-- into vendor_branches.region (e.g. "Abacus Bali" -> "Bali", "Advantage
-- Jawa Barat" -> "Jawa Barat"). Only rows whose region actually starts
-- with their own vendor's name are touched -- ROH branches and the one
-- pre-existing exception (Advantage Padang -> "Sumatera") never matched
-- and are left as-is.
BEGIN;

UPDATE vendor_branches vb
SET category = sub.category, updated_at = now()
FROM (
    SELECT DISTINCT ON (vendor_branch_id) vendor_branch_id, category
    FROM vendor_vaults
    ORDER BY vendor_branch_id, is_active DESC, id ASC
) sub
WHERE sub.vendor_branch_id = vb.id
  AND vb.category IS DISTINCT FROM sub.category;

UPDATE vendor_branches vb
SET region = btrim(regexp_replace(vb.region, '^' || v.name || '\s+', '')),
    updated_at = now()
FROM vendors v
WHERE v.id = vb.vendor_id
  AND vb.region ~ ('^' || v.name || '\s');

COMMIT;
