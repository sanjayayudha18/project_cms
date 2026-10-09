-- Admin mapping lokasi DSR -> vault (cit-acm-plan FR2). Mutations route through
-- the master-data maker-checker engine: create/update/disable/enable below run
-- only inside DsrLocationMapApplier (apply-on-approve tx). A DSR upload belongs
-- to a vendor via dsr_uploads.vendor = vendors.name.

-- name: ListDsrLocationMapsAdmin :many
-- All mappings of one vendor (a vendor has a handful of DSR blocks: unpaged).
SELECT m.id, m.vendor_id, m.dsr_location, m.vendor_vault_id, v.vault_code,
       v.vendor_branch_id, b.branch_name, m.is_active, m.deleted_at
FROM dsr_location_vault_maps m
JOIN vendor_vaults v ON v.id = m.vendor_vault_id
JOIN vendor_branches b ON b.id = v.vendor_branch_id
WHERE m.vendor_id = sqlc.arg('vendor_id')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND m.is_active)
        OR (sqlc.arg('status')::text = 'disabled' AND NOT m.is_active)
      )
ORDER BY lower(m.dsr_location), m.id;

-- name: GetDsrLocationMapAdminByID :one
-- Includes disabled rows; doubles as the "before" snapshot / CurrentState.
SELECT id, vendor_id, dsr_location, vendor_vault_id, is_active, deleted_at
FROM dsr_location_vault_maps WHERE id = $1;

-- name: FindActiveDsrLocationMap :one
-- Uniqueness pre-check: active mapping with the same label (case-insensitive)
-- for this vendor, excluding exclude_id (0 = none).
SELECT id FROM dsr_location_vault_maps
WHERE vendor_id = sqlc.arg('vendor_id') AND is_active
  AND lower(dsr_location) = lower(sqlc.arg('dsr_location')::text)
  AND id <> sqlc.arg('exclude_id')::bigint
LIMIT 1;

-- name: GetActiveVaultVendorID :one
-- Owning vendor of an ACTIVE vault; no row = vault missing or disabled.
SELECT b.vendor_id
FROM vendor_vaults v JOIN vendor_branches b ON b.id = v.vendor_branch_id
WHERE v.id = $1 AND v.deleted_at IS NULL;

-- name: ListUnmappedDsrLocations :many
-- DSR block labels this vendor has sent that have no active mapping, with the
-- latest report_date they appeared on.
SELECT r.location::text AS dsr_location, MAX(u.report_date)::date AS last_report_date
FROM dsr_daily_rows r
JOIN dsr_uploads u ON u.id = r.upload_id
JOIN vendors vd ON vd.name = u.vendor
WHERE vd.id = sqlc.arg('vendor_id') AND r.location IS NOT NULL
  AND NOT EXISTS (
        SELECT 1 FROM dsr_location_vault_maps m
        WHERE m.vendor_id = vd.id AND m.is_active AND lower(m.dsr_location) = lower(btrim(r.location))
      )
GROUP BY r.location
ORDER BY r.location;

-- name: CreateDsrLocationMap :one
-- Guarded: inserts only if the vault is still active and owned by the vendor
-- at apply time (no row => applier error => 409).
INSERT INTO dsr_location_vault_maps (vendor_id, dsr_location, vendor_vault_id)
SELECT sqlc.arg('vendor_id'), sqlc.arg('dsr_location'), v.id
FROM vendor_vaults v JOIN vendor_branches b ON b.id = v.vendor_branch_id
WHERE v.id = sqlc.arg('vendor_vault_id') AND v.deleted_at IS NULL AND b.vendor_id = sqlc.arg('vendor_id')
RETURNING id, vendor_id, dsr_location, vendor_vault_id, is_active, deleted_at;

-- name: UpdateDsrLocationMap :one
-- Only the target vault is editable (label + vendor immutable). Same apply-time guard.
UPDATE dsr_location_vault_maps m
SET vendor_vault_id = sqlc.arg('vendor_vault_id')
WHERE m.id = sqlc.arg('id') AND m.is_active
  AND EXISTS (
        SELECT 1 FROM vendor_vaults v JOIN vendor_branches b ON b.id = v.vendor_branch_id
        WHERE v.id = sqlc.arg('vendor_vault_id') AND v.deleted_at IS NULL AND b.vendor_id = m.vendor_id
      )
RETURNING id, vendor_id, dsr_location, vendor_vault_id, is_active, deleted_at;

-- name: DisableDsrLocationMap :exec
-- Soft-disable only.
UPDATE dsr_location_vault_maps SET is_active = false, deleted_at = now() WHERE id = $1;

-- name: EnableDsrLocationMap :exec
-- The active-unique index rejects re-enabling a label that has been re-mapped meanwhile.
UPDATE dsr_location_vault_maps SET is_active = true, deleted_at = NULL WHERE id = $1;
