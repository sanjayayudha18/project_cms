-- Admin vendor vault management (plan.md Fase 3, T3.2; revamped in migration 040).
-- Mutations route through the master-data maker-checker engine (Fase 2, D1):
-- create/update/disable/enable queries run ONLY inside VendorVaultApplier on the
-- primary pool (`db`) within the apply-on-approve transaction.
-- Topology:
--   * ListVendorVaultsAdmin / CountVendorVaultsAdmin -> `dbRead` (replica).
--   * GetVendorVaultAdminByID / Find* / Generate* / CountActive* / GetVendorBranchVendorID
--     -> `db` (primary, submit-time pre-checks & "before" snapshot).

-- name: ListVendorVaultsAdmin :many
-- Scoped to one vendor via its branches (dbRead). Filters: q (vault_code / vault_name substring),
-- status ('active'|'disabled'|'all', caller resolves absent to 'active'),
-- vendor_branch_id (optional branch drill-down, NULL = all branches).
SELECT v.id, v.vendor_branch_id, b.branch_name, b.region,
       v.vault_code, v.vault_name, v.category, v.currency_code, v.alamat,
       v.min_balance, v.max_balance, v.cis_limit_amount, v.cis_limit_currency,
       v.latitude, v.longitude, v.operating_hours, v.location_id,
       v.escrow_cash, v.escrow_atm, v.is_active, v.deleted_at
FROM vendor_vaults v
JOIN vendor_branches b ON b.id = v.vendor_branch_id
WHERE b.vendor_id = sqlc.arg('vendor_id')
  AND (sqlc.narg('vendor_branch_id')::bigint IS NULL OR v.vendor_branch_id = sqlc.narg('vendor_branch_id')::bigint)
  AND (sqlc.narg('q')::text IS NULL
        OR v.vault_code ILIKE '%' || sqlc.narg('q')::text || '%'
        OR v.vault_name ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND v.deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND v.deleted_at IS NOT NULL)
      )
ORDER BY v.vault_code ASC, v.id ASC
LIMIT sqlc.arg('page_limit')::bigint OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountVendorVaultsAdmin :one
-- Routing: dbRead. Same filters as ListVendorVaultsAdmin.
SELECT COUNT(*)
FROM vendor_vaults v
JOIN vendor_branches b ON b.id = v.vendor_branch_id
WHERE b.vendor_id = sqlc.arg('vendor_id')
  AND (sqlc.narg('vendor_branch_id')::bigint IS NULL OR v.vendor_branch_id = sqlc.narg('vendor_branch_id')::bigint)
  AND (sqlc.narg('q')::text IS NULL
        OR v.vault_code ILIKE '%' || sqlc.narg('q')::text || '%'
        OR v.vault_name ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND v.deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND v.deleted_at IS NOT NULL)
      );

-- name: CountActiveVendorVaultsByBranch :one
-- Routing: db (primary). Backs the branch-disable guard: refuse disabling a branch with active vaults.
SELECT COUNT(*) FROM vendor_vaults WHERE vendor_branch_id = $1 AND deleted_at IS NULL;

-- name: GetVendorVaultAdminByID :one
-- Routing: db (primary). Includes soft-deleted rows; doubles as the "before" snapshot / CurrentState (T2.5).
SELECT v.id, v.vendor_branch_id, b.branch_name, b.region,
       v.vault_code, v.vault_name, v.category, v.currency_code, v.alamat,
       v.min_balance, v.max_balance, v.cis_limit_amount, v.cis_limit_currency,
       v.latitude, v.longitude, v.operating_hours, v.location_id,
       v.escrow_cash, v.escrow_atm, v.is_active, v.deleted_at
FROM vendor_vaults v
JOIN vendor_branches b ON b.id = v.vendor_branch_id
WHERE v.id = $1;

-- name: FindVendorVaultAdminByCode :one
-- Routing: db (primary). Uniqueness pre-check (incl. soft-deleted): vault_code is globally unique.
SELECT id FROM vendor_vaults WHERE vault_code = $1;

-- name: FindVendorVaultAdminByBranchAndName :one
-- Routing: db (primary). Uniqueness pre-check (incl. soft-deleted): (vendor_branch_id, vault_name) is unique.
SELECT id
FROM vendor_vaults
WHERE vendor_branch_id = sqlc.arg('vendor_branch_id')
  AND vault_name = sqlc.arg('vault_name');

-- name: GenerateNextVendorVaultName :one
-- Routing: db (primary). Generates <vendorPrefix>_<area>_V<3-digit> for a branch when vault_name is omitted on create.
SELECT public.next_vendor_vault_name(sqlc.arg('vendor_branch_id'))::text AS vault_name;

-- name: GetVendorBranchVendorID :one
-- Routing: db (primary). Submit-time check that a vault's branch belongs to the vendor in the URL.
SELECT vendor_id FROM vendor_branches WHERE id = $1;

-- name: CreateVendorVaultAdmin :one
-- Routing: db (primary, tx inside VendorVaultApplier).
-- vendor_vaults.type is a legacy NOT NULL column (pre-category); new rows mirror category into it.
-- When vault_name is NULL or blank, trg_vendor_vaults_fill_vault_name populates <vendorPrefix>_<area>_V<3-digit>.
INSERT INTO vendor_vaults (
    vendor_branch_id, vault_code, vault_name, type, category, currency_code, alamat,
    min_balance, max_balance, cis_limit_amount,
    latitude, longitude, operating_hours, location_id, escrow_cash, escrow_atm
)
VALUES (
    sqlc.arg('vendor_branch_id'),
    sqlc.arg('vault_code'),
    COALESCE(NULLIF(btrim(sqlc.narg('vault_name')::text), ''), public.next_vendor_vault_name(sqlc.arg('vendor_branch_id'))),
    sqlc.arg('category'),
    sqlc.arg('category'),
    sqlc.arg('currency_code'),
    sqlc.narg('alamat'),
    sqlc.narg('min_balance'),
    sqlc.narg('max_balance'),
    sqlc.narg('cis_limit_amount'),
    sqlc.narg('latitude'),
    sqlc.narg('longitude'),
    sqlc.narg('operating_hours'),
    sqlc.narg('location_id'),
    sqlc.narg('escrow_cash'),
    sqlc.narg('escrow_atm')
)
RETURNING id, vendor_branch_id, vault_code, vault_name, category, currency_code, alamat,
          min_balance, max_balance, cis_limit_amount, cis_limit_currency,
          latitude, longitude, operating_hours, location_id,
          escrow_cash, escrow_atm, is_active, deleted_at;

-- name: UpdateVendorVaultAdmin :one
-- Routing: db (primary, tx inside VendorVaultApplier).
-- vault_code and vendor_branch_id are immutable; if vault_name is NULL/blank, the existing vault_name is kept.
-- `AND deleted_at IS NULL` makes a soft-disabled target update 0 rows (mapped to not-found).
UPDATE vendor_vaults
SET vault_name = COALESCE(NULLIF(btrim(sqlc.narg('vault_name')::text), ''), vault_name),
    category = sqlc.arg('category'),
    type = sqlc.arg('category'),
    currency_code = sqlc.arg('currency_code'),
    alamat = sqlc.narg('alamat'),
    min_balance = sqlc.narg('min_balance'),
    max_balance = sqlc.narg('max_balance'),
    cis_limit_amount = sqlc.narg('cis_limit_amount'),
    latitude = sqlc.narg('latitude'),
    longitude = sqlc.narg('longitude'),
    operating_hours = sqlc.narg('operating_hours'),
    location_id = sqlc.narg('location_id'),
    escrow_cash = sqlc.narg('escrow_cash'),
    escrow_atm = sqlc.narg('escrow_atm'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, vendor_branch_id, vault_code, vault_name, category, currency_code, alamat,
          min_balance, max_balance, cis_limit_amount, cis_limit_currency,
          latitude, longitude, operating_hours, location_id,
          escrow_cash, escrow_atm, is_active, deleted_at;

-- name: DisableVendorVault :exec
-- Routing: db (primary, tx inside VendorVaultApplier). Soft-disable only (no_hard_delete_test.go).
UPDATE vendor_vaults SET is_active = false, deleted_at = now() WHERE id = $1;

-- name: EnableVendorVault :exec
-- Routing: db (primary, tx inside VendorVaultApplier).
UPDATE vendor_vaults SET is_active = true, deleted_at = NULL WHERE id = $1;
