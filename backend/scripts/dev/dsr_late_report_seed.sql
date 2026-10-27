-- DEV ONLY — not a migration. Test data for the DSR late report (dsr-late-report, Phase 2.3):
--   1. one vault per active Bijak branch that has no active vault yet (vault_code VLT_<branch_code>, type ATM_CASH, IDR);
--   2. dsr_location_vault_maps for every distinct DSR location label of the Bijak upload report_date 2026-07-15,
--      all mapped to the vault of the vendor's first active branch (dev data; any vault of the vendor will do).
-- Idempotent: safe to re-run (NOT EXISTS guards on every insert). Written directly (no audit_logs) — dev data,
-- not a user action. Labels must match the DSR file exactly (btrim); a wrong label shows as an unmapped location.
-- Run: psql "postgres://…@localhost:5432/cms" -f backend/scripts/dev/dsr_late_report_seed.sql
BEGIN;

-- 1. Vault per branch when missing.
INSERT INTO public.vendor_vaults (vendor_branch_id, vault_code, type, currency_code, is_active)
SELECT vb.id, 'VLT_' || vb.branch_code, 'ATM_CASH', 'IDR', true
FROM public.vendor_branches vb
JOIN public.vendors v ON v.id = vb.vendor_id
WHERE v.name = 'Bijak' AND vb.is_active
  AND NOT EXISTS (SELECT 1 FROM public.vendor_vaults vv WHERE vv.vendor_branch_id = vb.id AND vv.is_active)
  AND NOT EXISTS (SELECT 1 FROM public.vendor_vaults vv WHERE vv.vault_code = 'VLT_' || vb.branch_code);

-- 2. Location labels of the Bijak upload 2026-07-15 -> vault of the first active branch.
INSERT INTO public.dsr_location_vault_maps (vendor_id, dsr_location, vendor_vault_id)
SELECT v.id, l.loc, vault.id
FROM public.vendors v
JOIN (
    SELECT DISTINCT btrim(r.location) AS loc
    FROM public.dsr_daily_rows r
    JOIN public.dsr_uploads u ON u.id = r.upload_id
    WHERE u.vendor = 'Bijak' AND u.report_date = DATE '2026-07-15'
      AND btrim(coalesce(r.location, '')) <> ''
) l ON true
JOIN LATERAL (
    SELECT vv.id
    FROM public.vendor_vaults vv
    JOIN public.vendor_branches vb ON vb.id = vv.vendor_branch_id
    WHERE vb.vendor_id = v.id AND vv.is_active
    ORDER BY vb.id, vv.id
    LIMIT 1
) vault ON true
WHERE v.name = 'Bijak'
  AND NOT EXISTS (
      SELECT 1 FROM public.dsr_location_vault_maps m
      WHERE m.vendor_id = v.id AND m.is_active AND lower(m.dsr_location) = lower(l.loc)
  );

COMMIT;
