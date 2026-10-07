-- Admin vendor branch package price ("harga khusus cabang") management
-- (migration 016). Mutations route through the master-data maker-checker
-- engine (Fase 2, D1): the create/update/disable queries below are only run
-- by VendorPackageApplier inside the apply-on-approve transaction. No Enable
-- (effective-dated history, not a togglable entity) -- same convention as
-- vendor_package_prices_admin.sql; "disable" ends validity via
-- effective_end_date, a new period is a new row.

-- name: ListVendorPackagesAdmin :many
-- Scoped to one vendor via its branches. Filters: q (package_code
-- substring), status ('active'|'disabled'|'all', caller resolves absent to
-- 'active'), vendor_branch_id (optional branch drill-down, NULL = all
-- branches). status 'active' = current or open-ended (effective_end_date
-- IS NULL OR >= today), 'disabled' = effective_end_date in the past.
SELECT p.id, p.vendor_branch_id, p.package_code, p.machine_group, p.price_class,
       p.tier_min, p.tier_max, p.base_price, p.atm_id, p.sla_note, p.currency,
       p.effective_start_date, p.effective_end_date
FROM vendor_packages_branch p
JOIN vendor_branches b ON b.id = p.vendor_branch_id
WHERE b.vendor_id = sqlc.arg('vendor_id')
  AND (sqlc.narg('vendor_branch_id')::bigint IS NULL OR p.vendor_branch_id = sqlc.narg('vendor_branch_id')::bigint)
  AND (sqlc.narg('q')::text IS NULL OR p.package_code ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND (p.effective_end_date IS NULL OR p.effective_end_date >= CURRENT_DATE))
        OR (sqlc.arg('status')::text = 'disabled' AND p.effective_end_date < CURRENT_DATE)
      )
ORDER BY p.package_code ASC, p.id ASC
LIMIT sqlc.arg('page_limit')::bigint OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountVendorPackagesAdmin :one
SELECT COUNT(*)
FROM vendor_packages_branch p
JOIN vendor_branches b ON b.id = p.vendor_branch_id
WHERE b.vendor_id = sqlc.arg('vendor_id')
  AND (sqlc.narg('vendor_branch_id')::bigint IS NULL OR p.vendor_branch_id = sqlc.narg('vendor_branch_id')::bigint)
  AND (sqlc.narg('q')::text IS NULL OR p.package_code ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND (p.effective_end_date IS NULL OR p.effective_end_date >= CURRENT_DATE))
        OR (sqlc.arg('status')::text = 'disabled' AND p.effective_end_date < CURRENT_DATE)
      );

-- name: CountActiveVendorPackagesByBranch :one
-- Backs the branch-disable guard: refuse disabling a branch with a current
-- or open-ended special price row.
SELECT COUNT(*) FROM vendor_packages_branch
WHERE vendor_branch_id = $1 AND (effective_end_date IS NULL OR effective_end_date >= CURRENT_DATE);

-- name: GetVendorPackageAdminByID :one
-- Includes expired rows and the owning vendor_id (for URL scoping); doubles
-- as the "before" snapshot / CurrentState.
SELECT p.id, p.vendor_branch_id, b.vendor_id, p.package_code, p.machine_group, p.price_class,
       p.tier_min, p.tier_max, p.base_price, p.atm_id, p.sla_note, p.currency,
       p.effective_start_date, p.effective_end_date
FROM vendor_packages_branch p
JOIN vendor_branches b ON b.id = p.vendor_branch_id
WHERE p.id = $1;

-- name: CreateVendorPackageAdmin :one
-- Grain (vendor_branch_id, package_code, machine_group, price_class, tier,
-- atm_id, currency, effective_start_date) is immutable after create -- a
-- grain change is a new price period, same convention as
-- CreateVendorPackagePriceAdmin.
INSERT INTO vendor_packages_branch
    (vendor_branch_id, package_code, machine_group, price_class, tier_min, tier_max,
     base_price, atm_id, sla_note, currency, effective_start_date, effective_end_date)
VALUES (sqlc.arg('vendor_branch_id'), sqlc.arg('package_code'), sqlc.arg('machine_group'), sqlc.arg('price_class'),
        sqlc.arg('tier_min'), sqlc.narg('tier_max'),
        sqlc.narg('base_price'), sqlc.narg('atm_id'), sqlc.narg('sla_note'), sqlc.arg('currency'),
        sqlc.arg('effective_start_date'), sqlc.narg('effective_end_date'))
RETURNING id, vendor_branch_id, package_code, machine_group, price_class, tier_min, tier_max,
          base_price, atm_id, sla_note, currency, effective_start_date, effective_end_date;

-- name: UpdateVendorPackageAdmin :one
-- Only the content fields (base_price, sla_note, effective_end_date) are
-- editable; grain fields stay immutable, same convention as
-- UpdateVendorPackagePriceAdmin. An already-ended row's update is 0 rows
-- (mapped to not-found).
UPDATE vendor_packages_branch
SET base_price = sqlc.narg('base_price'), sla_note = sqlc.narg('sla_note'),
    effective_end_date = sqlc.narg('effective_end_date'), updated_at = now()
WHERE id = sqlc.arg('id')
  AND (effective_end_date IS NULL OR effective_end_date >= CURRENT_DATE)
RETURNING id, vendor_branch_id, package_code, machine_group, price_class, tier_min, tier_max,
          base_price, atm_id, sla_note, currency, effective_start_date, effective_end_date;

-- name: DisableVendorPackage :exec
-- Ends the price period as of yesterday. No-op on an already-ended row.
UPDATE vendor_packages_branch
SET effective_end_date = CURRENT_DATE - 1, updated_at = now()
WHERE id = $1
  AND (effective_end_date IS NULL OR effective_end_date >= CURRENT_DATE);
