-- PKS vendor (spec .kiro/specs/vendor-pks/, task 4.2). Mutations
-- (MarkVendorPksReplaced, InsertVendorPks, SetVendorPksReplacedBy,
-- TerminateVendorPks, UnterminateVendorPks) run ONLY from VendorPksApplier inside
-- the apply-on-approve tx. No query updates pks_name / effective_date /
-- expiry_date: a PKS row is immutable apart from replaced_* and terminated_*.

-- name: GetVendorPksVendor :one
-- Submit pre-check (primary pool).
SELECT id, kind, deleted_at FROM vendors WHERE id = $1;

-- name: LockVendorForPks :one
-- Applier: serialises every PKS apply of one vendor, even a vendor with no
-- active PKS yet.
SELECT id, kind, deleted_at FROM vendors WHERE id = $1 FOR UPDATE;

-- name: GetActiveVendorPks :one
SELECT id, vendor_id, pks_name, effective_date, expiry_date, created_by,
       replaced_at, replaced_by_pks_id, terminated_at, terminated_by,
       termination_reason, created_at, updated_at
FROM vendor_pks
WHERE vendor_id = $1 AND replaced_at IS NULL;

-- name: GetVendorPksByID :one
-- Doubles as the "before" snapshot and the applier's CurrentState: same shape.
SELECT id, vendor_id, pks_name, effective_date, expiry_date, created_by,
       replaced_at, replaced_by_pks_id, terminated_at, terminated_by,
       termination_reason, created_at, updated_at
FROM vendor_pks WHERE id = $1;

-- name: ListVendorPksByVendor :many
-- Active PKS first, then history newest-replaced first.
SELECT id, vendor_id, pks_name, effective_date, expiry_date, created_by,
       replaced_at, replaced_by_pks_id, terminated_at, terminated_by,
       termination_reason, created_at, updated_at,
       vendor_pks_validity_months(effective_date, expiry_date)::int AS validity_months
FROM vendor_pks
WHERE vendor_id = $1
ORDER BY replaced_at DESC NULLS FIRST, id DESC;

-- name: GetPendingVendorPksChange :one
SELECT c.id, c.op, c.entity_id, c.payload, c.maker_id,
       u.full_name AS maker_name, c.created_at
FROM master_data_change_requests c
JOIN users u ON u.id = c.maker_id
WHERE c.entity_type = 'vendor_pks'
  AND c.status = 'pending'
  AND (c.payload->>'vendor_id')::bigint = sqlc.arg('vendor_id')::bigint;

-- name: GetVendorPksStatus :one
-- Detail screen (task 7.1): the same vendor_pks_status() rule the list, filter
-- and export use (Req 6.8). Arguments come from the already-loaded vendor + active
-- PKS; NULL expiry = no active PKS. '' for an INTERNAL vendor (function NULL).
SELECT COALESCE(vendor_pks_status(sqlc.arg('kind')::text, sqlc.narg('expiry_date')::date,
                         sqlc.narg('terminated_at')::timestamptz, sqlc.arg('today')::date,
                         sqlc.arg('renewal_months')::int), '')::text AS pks_status;

-- name: MarkVendorPksReplaced :exec
UPDATE vendor_pks SET replaced_at = sqlc.arg('replaced_at')
WHERE id = sqlc.arg('id') AND replaced_at IS NULL;

-- name: InsertVendorPks :one
INSERT INTO vendor_pks (vendor_id, pks_name, effective_date, expiry_date, created_by)
VALUES (sqlc.arg('vendor_id'), sqlc.arg('pks_name'), sqlc.arg('effective_date'),
        sqlc.arg('expiry_date'), sqlc.arg('created_by'))
RETURNING id, vendor_id, pks_name, effective_date, expiry_date, created_by,
          replaced_at, replaced_by_pks_id, terminated_at, terminated_by,
          termination_reason, created_at, updated_at;

-- name: SetVendorPksReplacedBy :exec
UPDATE vendor_pks SET replaced_by_pks_id = sqlc.arg('replaced_by_pks_id')
WHERE id = sqlc.arg('id') AND replaced_at IS NOT NULL;

-- name: TerminateVendorPks :one
-- Zero rows (pgx.ErrNoRows) = stale: replaced or already terminated.
UPDATE vendor_pks
SET terminated_at = now(),
    terminated_by = sqlc.arg('terminated_by'),
    termination_reason = sqlc.arg('termination_reason')
WHERE id = sqlc.arg('id') AND replaced_at IS NULL AND terminated_at IS NULL
RETURNING id, vendor_id, pks_name, effective_date, expiry_date, created_by,
          replaced_at, replaced_by_pks_id, terminated_at, terminated_by,
          termination_reason, created_at, updated_at;

-- name: UnterminateVendorPks :one
-- Zero rows (pgx.ErrNoRows) = stale: replaced or not terminated.
UPDATE vendor_pks
SET terminated_at = NULL, terminated_by = NULL, termination_reason = NULL
WHERE id = $1 AND replaced_at IS NULL AND terminated_at IS NOT NULL
RETURNING id, vendor_id, pks_name, effective_date, expiry_date, created_by,
          replaced_at, replaced_by_pks_id, terminated_at, terminated_by,
          termination_reason, created_at, updated_at;
