-- Master-data CSV import reference sets (plan.md Fase 5, T5.3). backend/ only:
-- moved out of master_data_export.sql, which backend-exq also generates
-- (vendor-data-export task 3.1).

-- name: ImportPackageKeys :many
-- Active vendor packages by natural key, for the import dry-run's FK check on
-- atm-assignments (plan.md T5.3). Read on the PRIMARY (validation of a write flow).
-- Migration 016 dropped is_active/deleted_at from vendor_packages_branch --
-- "active" now means an open (or not-yet-ended) price period, same
-- convention as DisableVendorPackage's own WHERE guard.
SELECT p.id, v.code AS vendor_code, b.branch_code, p.package_code
FROM vendor_packages_branch p
JOIN vendor_branches b ON b.id = p.vendor_branch_id
JOIN vendors v ON v.id = b.vendor_id
WHERE p.effective_end_date IS NULL OR p.effective_end_date >= CURRENT_DATE;

-- name: ImportVendorPackageLabels :many
-- Vendor-wide package labels per FLM vendor (atm-assignments import, package_source
-- 'vendor'): the vendor has a PT-/branch-level tariff that is still open or not yet
-- ended. Whether the tariff fits a given ATM is checked at confirm (FR9).
SELECT DISTINCT v.code AS vendor_code, vpp.package::text AS package
FROM vendor_package_prices vpp
JOIN vendors v ON v.id = vpp.vendor_id
WHERE v.is_active AND v.deleted_at IS NULL AND v.kind = 'FLM_VENDOR'
  AND vpp.atm_id IS NULL
  AND (vpp.effective_end_date IS NULL OR vpp.effective_end_date >= CURRENT_DATE);

-- name: ImportLocationIDs :many
SELECT id FROM locations;
