-- Row counts for the export preview (spec export-menu-revamp, task 1.1).
-- Each query copies the FROM/JOIN/WHERE of its Export*Batch twin exactly, minus
-- `id > after_id`, ORDER BY and LIMIT, so the preview total equals the number of
-- rows the export worker writes. JOINs are kept even where no column is
-- selected: an inner JOIN can drop rows and the count must drop them too.
--
-- CHANGE THESE TOGETHER WITH THE BATCH QUERIES (backend/queries/master_data_export.sql
-- and backend-exq/queries/vendor_export.sql). Property 2 (task 3.3) catches drift.

-- name: CountVendors :one
SELECT count(*)::bigint
FROM vendors v
WHERE (sqlc.narg('vendor_id')::bigint IS NULL OR v.id = sqlc.narg('vendor_id')::bigint)
  AND (sqlc.arg('status')::text = 'all'
       OR (sqlc.arg('status')::text = 'active' AND v.deleted_at IS NULL)
       OR (sqlc.arg('status')::text = 'disabled' AND v.deleted_at IS NOT NULL));

-- name: CountVendorBranches :one
SELECT count(*)::bigint
FROM vendor_branches b
JOIN vendors v ON v.id = b.vendor_id
WHERE (sqlc.narg('vendor_id')::bigint IS NULL OR b.vendor_id = sqlc.narg('vendor_id')::bigint)
  AND (sqlc.arg('status')::text = 'all'
       OR (sqlc.arg('status')::text = 'active' AND b.deleted_at IS NULL)
       OR (sqlc.arg('status')::text = 'disabled' AND b.deleted_at IS NOT NULL));

-- name: CountVendorVaults :one
SELECT count(*)::bigint
FROM vendor_vaults vl
JOIN vendor_branches b ON b.id = vl.vendor_branch_id
JOIN vendors v ON v.id = b.vendor_id
WHERE (sqlc.narg('vendor_id')::bigint IS NULL OR v.id = sqlc.narg('vendor_id')::bigint)
  AND (sqlc.narg('branch_id')::bigint IS NULL OR b.id = sqlc.narg('branch_id')::bigint)
  AND (sqlc.arg('status')::text = 'all'
       OR (sqlc.arg('status')::text = 'active' AND vl.deleted_at IS NULL)
       OR (sqlc.arg('status')::text = 'disabled' AND vl.deleted_at IS NOT NULL));

-- name: CountVendorPics :one
SELECT count(*)::bigint
FROM vendor_pics p
JOIN vendors v ON v.id = p.vendor_id
LEFT JOIN vendor_branches b ON b.id = p.vendor_branch_id
WHERE (sqlc.narg('vendor_id')::bigint IS NULL OR p.vendor_id = sqlc.narg('vendor_id')::bigint)
  AND (sqlc.narg('branch_id')::bigint IS NULL OR p.vendor_branch_id = sqlc.narg('branch_id')::bigint)
  AND (sqlc.arg('status')::text = 'all'
       OR (sqlc.arg('status')::text = 'active' AND p.deleted_at IS NULL)
       OR (sqlc.arg('status')::text = 'disabled' AND p.deleted_at IS NOT NULL));

-- name: CountATMs :one
SELECT count(*)::bigint
FROM atms a
WHERE (sqlc.arg('status')::text = 'all'
       OR (sqlc.arg('status')::text = 'active' AND a.deleted_at IS NULL)
       OR (sqlc.arg('status')::text = 'disabled' AND a.deleted_at IS NOT NULL));

-- name: CountATMAssignments :one
SELECT count(*)::bigint
FROM atm_vendor_packages av
JOIN atms a ON a.id = av.atm_id
LEFT JOIN vendor_packages_branch p ON p.id = av.vendor_package_id
JOIN vendor_branches b ON b.id = COALESCE(p.vendor_branch_id, av.vendor_branch_id)
JOIN vendors v ON v.id = b.vendor_id
WHERE (sqlc.arg('status')::text = 'all'
       OR (sqlc.arg('status')::text = 'active' AND av.is_active)
       OR (sqlc.arg('status')::text = 'disabled' AND NOT av.is_active));

-- name: CountDsrLocationMaps :one
SELECT count(*)::bigint
FROM dsr_location_vault_maps m
JOIN vendors v         ON v.id = m.vendor_id
JOIN vendor_vaults vl  ON vl.id = m.vendor_vault_id
JOIN vendor_branches b ON b.id = vl.vendor_branch_id
WHERE (sqlc.narg('vendor_id')::bigint IS NULL OR m.vendor_id = sqlc.narg('vendor_id')::bigint)
  AND (sqlc.narg('branch_id')::bigint IS NULL OR vl.vendor_branch_id = sqlc.narg('branch_id')::bigint)
  AND (sqlc.arg('status')::text = 'all'
       OR (sqlc.arg('status')::text = 'active' AND m.deleted_at IS NULL)
       OR (sqlc.arg('status')::text = 'disabled' AND m.deleted_at IS NOT NULL));

-- name: CountPackagePrices :one
SELECT count(*)::bigint
FROM vendor_package_prices vpp
JOIN vendors v ON v.id = vpp.vendor_id
LEFT JOIN vendor_branches b ON b.id = vpp.vendor_branch_id
LEFT JOIN atms a ON a.id = vpp.atm_id
WHERE vpp.vendor_id = sqlc.arg('vendor_id')::bigint
  AND (sqlc.narg('branch_id')::bigint IS NULL OR vpp.vendor_branch_id = sqlc.narg('branch_id')::bigint)
  AND (sqlc.arg('price_level')::text = 'all'
       OR (sqlc.arg('price_level')::text = 'atm' AND vpp.atm_id IS NOT NULL)
       OR (sqlc.arg('price_level')::text = 'branch' AND vpp.vendor_branch_id IS NOT NULL)
       OR (sqlc.arg('price_level')::text = 'vendor' AND vpp.atm_id IS NULL AND vpp.vendor_branch_id IS NULL))
  AND (sqlc.arg('status')::text = 'all'
       OR (sqlc.arg('status')::text = 'active'
           AND (vpp.effective_end_date IS NULL OR vpp.effective_end_date >= sqlc.arg('as_of')::date))
       OR (sqlc.arg('status')::text = 'disabled'
           AND vpp.effective_end_date IS NOT NULL AND vpp.effective_end_date < sqlc.arg('as_of')::date));
