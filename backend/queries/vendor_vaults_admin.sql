-- Admin vendor vault management (plan.md Fase 3, T3.2). Mutations route
-- through the master-data maker-checker engine (Fase 2, D1): the
-- create/update/disable/enable queries below are only run by
-- VendorVaultApplier inside the apply-on-approve transaction.

-- name: ListVendorVaultsAdmin :many
-- Scoped to one vendor via its branches. Filters: q (vault_code substring),
-- status ('active'|'disabled'|'all', caller resolves absent to 'active').
SELECT v.id, v.vendor_branch_id, v.vault_code, v.category, v.currency_code,
       v.min_capacity_amount, v.max_capacity_amount, v.latitude, v.longitude,
       v.operating_hours, v.location_id, v.is_active, v.deleted_at
FROM vendor_vaults v
JOIN vendor_branches b ON b.id = v.vendor_branch_id
WHERE b.vendor_id = sqlc.arg('vendor_id')
  AND (sqlc.narg('q')::text IS NULL OR v.vault_code ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND v.deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND v.deleted_at IS NOT NULL)
      )
ORDER BY v.vault_code ASC, v.id ASC
LIMIT sqlc.arg('page_limit')::bigint OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountVendorVaultsAdmin :one
SELECT COUNT(*)
FROM vendor_vaults v
JOIN vendor_branches b ON b.id = v.vendor_branch_id
WHERE b.vendor_id = sqlc.arg('vendor_id')
  AND (sqlc.narg('q')::text IS NULL OR v.vault_code ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND v.deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND v.deleted_at IS NOT NULL)
      );

-- name: GetVendorVaultAdminByID :one
-- Includes soft-deleted rows; doubles as the "before" snapshot / CurrentState (T2.5).
SELECT id, vendor_branch_id, vault_code, category, currency_code,
       min_capacity_amount, max_capacity_amount, latitude, longitude,
       operating_hours, location_id, is_active, deleted_at
FROM vendor_vaults WHERE id = $1;

-- name: FindVendorVaultAdminByCode :one
-- Uniqueness pre-check (incl. soft-deleted): vault_code is globally unique.
SELECT id FROM vendor_vaults WHERE vault_code = $1;

-- name: GetVendorBranchVendorID :one
-- Submit-time check that a vault's branch belongs to the vendor in the URL.
SELECT vendor_id FROM vendor_branches WHERE id = $1;

-- name: CreateVendorVaultAdmin :one
-- vendor_vaults.type is a legacy NOT NULL column (pre-category); new rows
-- mirror category into it.
INSERT INTO vendor_vaults (vendor_branch_id, vault_code, type, category, currency_code,
                           min_capacity_amount, max_capacity_amount, latitude, longitude,
                           operating_hours, location_id)
VALUES (sqlc.arg('vendor_branch_id'), sqlc.arg('vault_code'), sqlc.arg('category'), sqlc.arg('category'),
        sqlc.arg('currency_code'), sqlc.narg('min_capacity_amount'), sqlc.narg('max_capacity_amount'),
        sqlc.narg('latitude'), sqlc.narg('longitude'), sqlc.narg('operating_hours'), sqlc.narg('location_id'))
RETURNING id, vendor_branch_id, vault_code, category, currency_code,
          min_capacity_amount, max_capacity_amount, latitude, longitude,
          operating_hours, location_id, is_active, deleted_at;

-- name: UpdateVendorVaultAdmin :one
-- vault_code and vendor_branch_id are immutable. `AND deleted_at IS NULL`
-- makes a soft-disabled target update 0 rows (mapped to not-found).
UPDATE vendor_vaults
SET category = sqlc.arg('category'),
    type = sqlc.arg('category'),
    currency_code = sqlc.arg('currency_code'),
    min_capacity_amount = sqlc.narg('min_capacity_amount'),
    max_capacity_amount = sqlc.narg('max_capacity_amount'),
    latitude = sqlc.narg('latitude'),
    longitude = sqlc.narg('longitude'),
    operating_hours = sqlc.narg('operating_hours'),
    location_id = sqlc.narg('location_id'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, vendor_branch_id, vault_code, category, currency_code,
          min_capacity_amount, max_capacity_amount, latitude, longitude,
          operating_hours, location_id, is_active, deleted_at;

-- name: DisableVendorVault :exec
-- Soft-disable only (no_hard_delete_test.go extended in T3.7).
UPDATE vendor_vaults SET is_active = false, deleted_at = now() WHERE id = $1;

-- name: EnableVendorVault :exec
UPDATE vendor_vaults SET is_active = true, deleted_at = NULL WHERE id = $1;
