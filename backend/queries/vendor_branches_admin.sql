-- Admin vendor branch management (plan.md Fase 3, T3.1; cis_limit moved to vendor_vaults in migration 040).
-- Mutations route through the master-data maker-checker engine (Fase 2, D1) -- these
-- create/update/disable/enable queries are only ever run by VendorBranchApplier
-- inside the apply-on-approve transaction on primary pool (`db`), never directly by VendorBranchAdminService.
-- Topology:
--   * ListVendorBranchesAdmin / CountVendorBranchesAdmin -> `dbRead` (replica).
--   * GetVendorBranchAdminByID / FindVendorBranchAdminByCode -> `db` (primary).

-- name: ListVendorBranchesAdmin :many
-- Scoped to one vendor (Req: /api/v1/admin/vendors/{id}/branches). Filters:
-- q (case-insensitive substring across branch_code/branch_name) and status
-- ('active' | 'disabled' | 'all') -- caller resolves absent status to
-- 'active' (mirrors ListVendorsAdmin's convention).
-- region = name of the branch's standard region (regions.region via
-- region_code, region-standardization Req 4.2); vendor_branches.region was
-- dropped in migration 042.
SELECT vb.id, vb.vendor_id, vb.branch_code, vb.branch_name, vb.location_id, r.region, vb.region_code,
       vb.category, vb.is_active, vb.deleted_at
FROM vendor_branches vb
LEFT JOIN regions r ON r.code = vb.region_code
WHERE vb.vendor_id = sqlc.arg('vendor_id')
  AND (sqlc.narg('q')::text IS NULL
        OR vb.branch_code ILIKE '%' || sqlc.narg('q')::text || '%'
        OR vb.branch_name ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND vb.deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND vb.deleted_at IS NOT NULL)
      )
ORDER BY vb.branch_name ASC, vb.id ASC
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
-- "before" snapshot / CurrentState staleness check (T2.5). No region name
-- here on purpose: renaming a region must not make pending branch changes stale.
SELECT id, vendor_id, branch_code, branch_name, location_id, region_code, category, is_active, deleted_at
FROM vendor_branches WHERE id = $1;

-- name: FindVendorBranchAdminByCode :one
-- Uniqueness pre-check (incl. soft-deleted) -- branch_code is globally
-- unique (uq_vendor_branches_branch_code), not just within a vendor.
SELECT id FROM vendor_branches WHERE branch_code = $1;

-- name: CreateVendorBranchAdmin :one
INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, location_id, region_code, category)
VALUES (sqlc.arg('vendor_id'), sqlc.arg('branch_code'), sqlc.arg('branch_name'),
        sqlc.narg('location_id'), sqlc.narg('region_code'), sqlc.arg('category'))
RETURNING id, vendor_id, branch_code, branch_name, location_id, region_code, category, is_active, deleted_at;

-- name: UpdateVendorBranchAdmin :one
-- branch_code is immutable (not editable here, mirrors vendors.code /
-- atms.terminal_id convention). `AND deleted_at IS NULL` makes a
-- soft-disabled target update as 0 rows affected, mapped to "not found" by
-- the applier/service.
UPDATE vendor_branches
SET branch_name = sqlc.arg('branch_name'),
    location_id = sqlc.narg('location_id'),
    region_code = sqlc.narg('region_code'),
    category = sqlc.arg('category'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, vendor_id, branch_code, branch_name, location_id, region_code, category, is_active, deleted_at;

-- name: DisableVendorBranch :exec
-- Soft-disable only: is_active=false, deleted_at=now(). No hard DELETE
-- anywhere in this codebase (no_hard_delete_test.go, extended for this
-- table in T3.7).
UPDATE vendor_branches SET is_active = false, deleted_at = now() WHERE id = $1;

-- name: EnableVendorBranch :exec
-- Reverses DisableVendorBranch: is_active=true, deleted_at=NULL.
UPDATE vendor_branches SET is_active = true, deleted_at = NULL WHERE id = $1;

-- name: FindActiveRegionByCode :one
-- Submit/CSV dry-run check (region-standardization Req 3.3, 3.5): region_code
-- must name an active standard region. Primary pool (read-after-write safe).
SELECT code FROM regions WHERE code = $1 AND is_active = true;

-- name: LockActiveRegionByCodeForShare :one
-- Apply-time check inside the approve tx (Req 3.4): FOR SHARE blocks a
-- concurrent region disable (FOR UPDATE) until the branch apply commits, so
-- the disable then sees the branch and is rejected (Property 4).
SELECT code FROM regions WHERE code = $1 AND is_active = true FOR SHARE;
