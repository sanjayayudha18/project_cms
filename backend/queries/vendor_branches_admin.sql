-- Admin vendor branch management (plan.md Fase 3, T3.1). Mutations route
-- through the master-data maker-checker engine (Fase 2, D1) -- these
-- create/update/disable/enable queries are only ever run by
-- VendorBranchApplier inside the apply-on-approve transaction, never
-- directly by VendorBranchAdminService.

-- name: ListVendorBranchesAdmin :many
-- Scoped to one vendor (Req: /api/v1/admin/vendors/{id}/branches). Filters:
-- q (case-insensitive substring across branch_code/branch_name) and status
-- ('active' | 'disabled' | 'all') -- caller resolves absent status to
-- 'active' (mirrors ListVendorsAdmin's convention).
SELECT id, vendor_id, branch_code, branch_name, location_id, region, category, is_active, deleted_at
FROM vendor_branches
WHERE vendor_id = sqlc.arg('vendor_id')
  AND (sqlc.narg('q')::text IS NULL
        OR branch_code ILIKE '%' || sqlc.narg('q')::text || '%'
        OR branch_name ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND deleted_at IS NOT NULL)
      )
ORDER BY branch_name ASC, id ASC
LIMIT sqlc.arg('page_limit')::bigint OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountVendorBranchesAdmin :one
-- Same filters as ListVendorBranchesAdmin, no LIMIT/OFFSET -- pagination total.
SELECT COUNT(*)
FROM vendor_branches
WHERE vendor_id = sqlc.arg('vendor_id')
  AND (sqlc.narg('q')::text IS NULL
        OR branch_code ILIKE '%' || sqlc.narg('q')::text || '%'
        OR branch_name ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND deleted_at IS NOT NULL)
      );

-- name: GetVendorBranchAdminByID :one
-- Includes soft-deleted rows -- used both for the read endpoint and as the
-- "before" snapshot / CurrentState staleness check (T2.5).
SELECT id, vendor_id, branch_code, branch_name, location_id, region, category, is_active, deleted_at
FROM vendor_branches WHERE id = $1;

-- name: FindVendorBranchAdminByCode :one
-- Uniqueness pre-check (incl. soft-deleted) -- branch_code is globally
-- unique (uq_vendor_branches_branch_code), not just within a vendor.
SELECT id FROM vendor_branches WHERE branch_code = $1;

-- name: CreateVendorBranchAdmin :one
INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, location_id, region, category)
VALUES (sqlc.arg('vendor_id'), sqlc.arg('branch_code'), sqlc.arg('branch_name'),
        sqlc.narg('location_id'), sqlc.narg('region'), sqlc.arg('category'))
RETURNING id, vendor_id, branch_code, branch_name, location_id, region, category, is_active, deleted_at;

-- name: UpdateVendorBranchAdmin :one
-- branch_code is immutable (not editable here, mirrors vendors.code /
-- atms.terminal_id convention). `AND deleted_at IS NULL` makes a
-- soft-disabled target update as 0 rows affected, mapped to "not found" by
-- the applier/service.
UPDATE vendor_branches
SET branch_name = sqlc.arg('branch_name'),
    location_id = sqlc.narg('location_id'),
    region = sqlc.narg('region'),
    category = sqlc.arg('category'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, vendor_id, branch_code, branch_name, location_id, region, category, is_active, deleted_at;

-- name: DisableVendorBranch :exec
-- Soft-disable only: is_active=false, deleted_at=now(). No hard DELETE
-- anywhere in this codebase (no_hard_delete_test.go, extended for this
-- table in T3.7).
UPDATE vendor_branches SET is_active = false, deleted_at = now() WHERE id = $1;

-- name: EnableVendorBranch :exec
-- Reverses DisableVendorBranch: is_active=true, deleted_at=NULL.
UPDATE vendor_branches SET is_active = true, deleted_at = NULL WHERE id = $1;
