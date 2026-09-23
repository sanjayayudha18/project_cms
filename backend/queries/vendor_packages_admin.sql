-- Admin vendor package management (plan.md Fase 3, T3.4). Mutations route
-- through the master-data maker-checker engine (Fase 2, D1): the
-- create/update/disable/enable queries below are only run by
-- VendorPackageApplier inside the apply-on-approve transaction.

-- name: ListVendorPackagesAdmin :many
-- Scoped to one vendor via its branches. Filters: q (code substring),
-- status ('active'|'disabled'|'all', caller resolves absent to 'active'),
-- vendor_branch_id (optional branch drill-down, NULL = all branches).
-- No price/priority_class here since migration 010: this table is now a
-- pure kelolaan/frequency link, prices moved to vendor_package_prices.
SELECT p.id, p.vendor_branch_id, p.code, p.is_active, p.deleted_at
FROM vendor_packages_branch p
JOIN vendor_branches b ON b.id = p.vendor_branch_id
WHERE b.vendor_id = sqlc.arg('vendor_id')
  AND (sqlc.narg('vendor_branch_id')::bigint IS NULL OR p.vendor_branch_id = sqlc.narg('vendor_branch_id')::bigint)
  AND (sqlc.narg('q')::text IS NULL OR p.code ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND p.deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND p.deleted_at IS NOT NULL)
      )
ORDER BY p.code ASC, p.id ASC
LIMIT sqlc.arg('page_limit')::bigint OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountVendorPackagesAdmin :one
SELECT COUNT(*)
FROM vendor_packages_branch p
JOIN vendor_branches b ON b.id = p.vendor_branch_id
WHERE b.vendor_id = sqlc.arg('vendor_id')
  AND (sqlc.narg('vendor_branch_id')::bigint IS NULL OR p.vendor_branch_id = sqlc.narg('vendor_branch_id')::bigint)
  AND (sqlc.narg('q')::text IS NULL OR p.code ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND p.deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND p.deleted_at IS NOT NULL)
      );

-- name: CountActiveVendorPackagesByBranch :one
-- Backs the branch-disable guard: refuse disabling a branch with active packages.
SELECT COUNT(*) FROM vendor_packages_branch WHERE vendor_branch_id = $1 AND deleted_at IS NULL;

-- name: GetVendorPackageAdminByID :one
-- Includes soft-deleted rows and the owning vendor_id (for URL scoping);
-- doubles as the "before" snapshot / CurrentState (T2.5).
SELECT p.id, p.vendor_branch_id, b.vendor_id, p.code, p.is_active, p.deleted_at
FROM vendor_packages_branch p
JOIN vendor_branches b ON b.id = p.vendor_branch_id
WHERE p.id = $1;

-- name: FindVendorPackageAdminByBranchCode :one
-- Uniqueness pre-check (incl. soft-disabled): (vendor_branch_id, code) is unique.
SELECT id FROM vendor_packages_branch WHERE vendor_branch_id = $1 AND code = $2;

-- name: CreateVendorPackageAdmin :one
-- vendor_branch_id and code are the only fields left (migration 010 dropped
-- price/priority_class) -- both immutable after create, so there is no
-- UpdateVendorPackageAdmin anymore; a code/branch change is a new package.
INSERT INTO vendor_packages_branch (vendor_branch_id, code)
VALUES ($1, $2)
RETURNING id, vendor_branch_id, code, is_active, deleted_at;

-- name: DisableVendorPackage :exec
-- Soft-disable only (no_hard_delete_test.go extended in T3.7).
UPDATE vendor_packages_branch SET is_active = false, deleted_at = now() WHERE id = $1;

-- name: EnableVendorPackage :exec
UPDATE vendor_packages_branch SET is_active = true, deleted_at = NULL WHERE id = $1;
