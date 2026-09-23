-- Plan: .claude/sdlc/vendor-branch-tipe/plan.md (D1). Adds a tipe/category
-- field to vendor_branches, reusing the same enum vendor_vaults.category
-- already uses (migration 008) instead of inventing a new one.
ALTER TABLE vendor_branches
    ADD COLUMN category text NOT NULL DEFAULT 'ATM';

ALTER TABLE vendor_branches
    ADD CONSTRAINT vendor_branches_category_chk CHECK (category IN ('ATM', 'CASH', 'ATM_CASH'));

COMMENT ON COLUMN vendor_branches.category IS 'Tipe cabang: ATM, CASH, or ATM_CASH -- mirrors vendor_vaults.category. Added per .claude/sdlc/vendor-branch-tipe/plan.md; existing rows default to ATM pending re-seed.';
