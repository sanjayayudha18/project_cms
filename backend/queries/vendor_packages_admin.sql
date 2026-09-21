-- Admin vendor package management (plan.md Fase 3, T3.4). Mutations route
-- through the master-data maker-checker engine (Fase 2, D1): the
-- create/update/disable/enable queries below are only run by
-- VendorPackageApplier inside the apply-on-approve transaction.

-- name: ListVendorPackagesAdmin :many
-- Scoped to one vendor via its branches. Filters: q (code substring),
-- status ('active'|'disabled'|'all', caller resolves absent to 'active').
SELECT p.id, p.vendor_branch_id, p.code, p.priority_class, p.price, p.is_active, p.deleted_at
FROM vendor_packages p
JOIN vendor_branches b ON b.id = p.vendor_branch_id
WHERE b.vendor_id = sqlc.arg('vendor_id')
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
FROM vendor_packages p
JOIN vendor_branches b ON b.id = p.vendor_branch_id
WHERE b.vendor_id = sqlc.arg('vendor_id')
  AND (sqlc.narg('q')::text IS NULL OR p.code ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND p.deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND p.deleted_at IS NOT NULL)
      );

-- name: GetVendorPackageAdminByID :one
-- Includes soft-deleted rows and the owning vendor_id (for URL scoping);
-- doubles as the "before" snapshot / CurrentState (T2.5).
SELECT p.id, p.vendor_branch_id, b.vendor_id, p.code, p.priority_class, p.price, p.is_active, p.deleted_at
FROM vendor_packages p
JOIN vendor_branches b ON b.id = p.vendor_branch_id
WHERE p.id = $1;

-- name: FindVendorPackageAdminByBranchCode :one
-- Uniqueness pre-check (incl. soft-disabled): (vendor_branch_id, code) is unique.
SELECT id FROM vendor_packages WHERE vendor_branch_id = $1 AND code = $2;

-- name: CreateVendorPackageAdmin :one
INSERT INTO vendor_packages (vendor_branch_id, code, priority_class, price)
VALUES ($1, $2, $3, $4)
RETURNING id, vendor_branch_id, code, priority_class, price, is_active, deleted_at;

-- name: UpdateVendorPackageAdmin :one
-- vendor_branch_id and code are immutable. `AND deleted_at IS NULL` makes a
-- soft-disabled target update 0 rows (mapped to not-found).
UPDATE vendor_packages
SET priority_class = sqlc.arg('priority_class'),
    price = sqlc.arg('price'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, vendor_branch_id, code, priority_class, price, is_active, deleted_at;

-- name: DisableVendorPackage :exec
-- Soft-disable only (no_hard_delete_test.go extended in T3.7).
UPDATE vendor_packages SET is_active = false, deleted_at = now() WHERE id = $1;

-- name: EnableVendorPackage :exec
UPDATE vendor_packages SET is_active = true, deleted_at = NULL WHERE id = $1;
