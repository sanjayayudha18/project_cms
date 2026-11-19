-- vendor_pks_types: master type PKS (migration 043, .kiro/specs/vendor-pks-type/ task 2.1).
-- Writes happen only in the master-data Applier (maker-checker); reads via the repo.

-- name: ListPksTypes :many
-- Replica read. in_use = a vendor_pks row of this type with replaced_at IS NULL
-- (active PKS, terminated ones included) -- same rule the disable guard uses.
SELECT t.id, t.code, t.name, t.is_active, t.created_at, t.updated_at,
       EXISTS (
         SELECT 1 FROM vendor_pks p
         WHERE p.pks_type_id = t.id AND p.replaced_at IS NULL
       ) AS in_use
FROM vendor_pks_types t
ORDER BY t.code;

-- name: GetPksType :one
SELECT id, code, name, is_active, created_at, updated_at
FROM vendor_pks_types
WHERE id = $1;

-- name: GetPksTypeByCode :one
-- Case-insensitive, matches vendor_pks_types_code_uq (upper(code)).
SELECT id, code, name, is_active, created_at, updated_at
FROM vendor_pks_types
WHERE upper(code) = upper(sqlc.arg('code')::text);

-- name: ListActivePksTypes :many
-- Vendor list columns / filter, PKS type picker.
SELECT id, code, name, is_active, created_at, updated_at
FROM vendor_pks_types
WHERE is_active = true
ORDER BY code;

-- name: CreatePksType :one
-- code is stored pre-normalized (trim + uppercase) by the service; 23505 on
-- vendor_pks_types_code_uq -> ErrPksTypeConflict in the applier.
INSERT INTO vendor_pks_types (code, name)
VALUES (sqlc.arg('code'), sqlc.arg('name'))
RETURNING id, code, name, is_active, created_at, updated_at;

-- name: UpdatePksTypeName :one
-- Name only -- code is immutable. Guarded by is_active: no row = stale.
-- updated_at is set by trg_vendor_pks_types_set_updated_at.
UPDATE vendor_pks_types
SET name = sqlc.arg('name')
WHERE id = sqlc.arg('id') AND is_active = true
RETURNING id, code, name, is_active, created_at, updated_at;

-- name: DisablePksType :one
-- Soft-disable. Run in the disable tx after LockPksType + PksTypeInUse.
-- No row = already inactive (stale).
UPDATE vendor_pks_types
SET is_active = false
WHERE id = $1 AND is_active = true
RETURNING id, code, name, is_active, created_at, updated_at;

-- name: EnablePksType :one
UPDATE vendor_pks_types
SET is_active = true
WHERE id = $1 AND is_active = false
RETURNING id, code, name, is_active, created_at, updated_at;

-- name: LockPksType :one
-- First statement of the disable tx: a concurrent vendor_pks create (FOR SHARE on
-- the same row) cannot slip a PKS in between PksTypeInUse and the disable.
SELECT id, code, is_active FROM vendor_pks_types WHERE id = $1 FOR UPDATE;

-- name: LockPksTypeShare :one
-- vendor_pks create apply: blocks a concurrent disable until this tx ends.
SELECT id, code, is_active FROM vendor_pks_types WHERE id = $1 FOR SHARE;

-- name: PksTypeInUse :one
-- Disable guard: any vendor_pks row of this type that is not replaced
-- (active or terminated). Run after LockPksType.
SELECT EXISTS (
  SELECT 1 FROM vendor_pks WHERE pks_type_id = $1 AND replaced_at IS NULL
) AS in_use;

-- name: GetPendingPksTypeChange :one
-- Pending update/disable/enable on an existing type (entity_id = type id).
-- At most one (master_data_change_requests_pending_uq).
SELECT c.id, c.op, c.entity_id, c.payload, c.maker_id,
       u.full_name AS maker_name, c.created_at
FROM master_data_change_requests c
JOIN users u ON u.id = c.maker_id
WHERE c.entity_type = 'vendor_pks_type'
  AND c.status = 'pending'
  AND c.entity_id = sqlc.arg('id')::bigint;

-- name: PksTypeCodePending :one
-- Duplicate pre-check on create: a pending create vendor_pks_type with this code
-- (matches master_data_change_requests_pks_type_code_pending_uq).
SELECT EXISTS (
  SELECT 1 FROM master_data_change_requests
  WHERE entity_type = 'vendor_pks_type'
    AND op = 'create'
    AND status = 'pending'
    AND upper(payload->>'code') = upper(sqlc.arg('code')::text)
) AS pending;
