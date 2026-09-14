-- Additive columns for MASTER_ATM_ESQ source fields that had no home in the schema.
--
-- WHY: the MASTER_ATM_ESQ master-data extract (document/MASTER_ATM_ESQ.csv.xlsx,
-- 14 columns) carries three fields that the current schema drops on ingest:
--   * FLMVendorRegion  -- the vendor's regional grouping (51 distinct values, e.g.
--     "Jakarta Timur", "Sumatera"). This is DISTINCT from AreaMesin (the ATM's
--     geographic machine area, -> regions.region) and from FLMVendorSubRegion (the
--     vendor's branch, -> vendor_branches.branch_name). It belongs to the vendor's
--     branch hierarchy, so it lands on vendor_branches (a sub-region rolls up into it).
--   * Escrow           -- the 12-digit escrow account number per ATM (825 non-null in
--     the source, e.g. 800035037200). One account per ATM, so it lands on atms.
--   * PriorityClass    -- VIP / Non VIP / Industri. Measured per-ATM in the source:
--     it varies WITHIN a single vendor branch in 15 of 348 branches, so it is an ATM
--     attribute, not a package or branch property. Lands on atms (decision 1C of
--     .kiro/specs/update-cit-forecast-browser/seed-proposal-atm-vendor-packages.md).
--     PAKET (the other per-ATM source field) is modelled as vendor_packages.code, seeded
--     separately (migration 031).
-- All verified against the real file; see .kiro/steering/lessons-learned.md
-- ("MASTER_ATM_ESQ actual columns" / "PAKET and PriorityClass are per-ATM").
--
-- SAFETY:
--   * DDL only, additive, no data touched. Both columns are nullable with no default,
--     so ADD COLUMN is a catalog-only change (Postgres 11+): no table rewrite, only a
--     brief ACCESS EXCLUSIVE lock to update the catalog. Existing rows read NULL until
--     backfilled by the MASTER_ATM_ESQ seed (separate migration).
--   * IF NOT EXISTS: re-runnable, and a no-op if a later re-import already added them.
--   * Escrow stored as text (not numeric): it is an account identifier, not a monetary
--     amount -- leading digits are significant and it is never arithmetic. Storing it as
--     a number would risk precision loss and is semantically wrong.
--   * No index added here. Neither column has a known query-time filter/join yet; add a
--     targeted index in a later migration if one appears (e.g. escrow reconciliation
--     looking up by escrow_account).
--
-- ROLLBACK (forward-only shop, so documented rather than shipped as a .down.sql -- a down
-- file in this directory would also be picked up by sqlc's schema glob):
--   ALTER TABLE public.vendor_branches DROP COLUMN IF EXISTS region;
--   ALTER TABLE public.atms            DROP COLUMN IF EXISTS escrow_account;
--   ALTER TABLE public.atms            DROP COLUMN IF EXISTS priority_class;

BEGIN;

ALTER TABLE public.vendor_branches
    ADD COLUMN IF NOT EXISTS region text;

COMMENT ON COLUMN public.vendor_branches.region
    IS 'Vendor regional grouping from MASTER_ATM_ESQ.FLMVendorRegion (e.g. "Jakarta Timur"). '
       'Distinct from regions.region (ATM geographic area) and branch_name (vendor sub-region). See migration 030.';

ALTER TABLE public.atms
    ADD COLUMN IF NOT EXISTS escrow_account text;

COMMENT ON COLUMN public.atms.escrow_account
    IS 'Escrow account number per ATM from MASTER_ATM_ESQ.Escrow (12-digit identifier, stored as text). See migration 030.';

ALTER TABLE public.atms
    ADD COLUMN IF NOT EXISTS priority_class text;

COMMENT ON COLUMN public.atms.priority_class
    IS 'ATM priority tier from MASTER_ATM_ESQ.PriorityClass: VIP | Non VIP | Industri. '
       'Per-ATM (varies within a vendor branch), so stored on atms not vendor_packages. See migration 030.';

COMMIT;
