-- Admin vendor PIC management (plan.md Fase 3, T3.3). Mutations route
-- through the master-data maker-checker engine (Fase 2, D1): the
-- create/update/disable/enable queries below are only run by
-- VendorPicApplier inside the apply-on-approve transaction.

-- name: ListVendorPicsAdmin :many
-- Filters: q (name substring), status ('active'|'disabled'|'all', caller
-- resolves absent to 'active').
SELECT id, vendor_id, vendor_branch_id, name, "position", phone, email,
       is_notification_recipient, is_active, deleted_at
FROM vendor_pics
WHERE vendor_id = sqlc.arg('vendor_id')
  AND (sqlc.narg('q')::text IS NULL OR name ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND deleted_at IS NOT NULL)
      )
ORDER BY name ASC, id ASC
LIMIT sqlc.arg('page_limit')::bigint OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountVendorPicsAdmin :one
SELECT COUNT(*)
FROM vendor_pics
WHERE vendor_id = sqlc.arg('vendor_id')
  AND (sqlc.narg('q')::text IS NULL OR name ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND deleted_at IS NOT NULL)
      );

-- name: GetVendorPicAdminByID :one
-- Includes soft-deleted rows; doubles as the "before" snapshot / CurrentState (T2.5).
SELECT id, vendor_id, vendor_branch_id, name, "position", phone, email,
       is_notification_recipient, is_active, deleted_at
FROM vendor_pics WHERE id = $1;

-- name: CountActiveNotificationPics :one
-- Backs the "vendor has no notification recipient" warning (T3.3).
SELECT COUNT(*) FROM vendor_pics
WHERE vendor_id = $1 AND is_notification_recipient AND deleted_at IS NULL;

-- name: CreateVendorPicAdmin :one
INSERT INTO vendor_pics (vendor_id, vendor_branch_id, name, "position", phone, email, is_notification_recipient)
VALUES (sqlc.arg('vendor_id'), sqlc.narg('vendor_branch_id'), sqlc.arg('name'), sqlc.narg('position'),
        sqlc.narg('phone'), sqlc.narg('email'), sqlc.arg('is_notification_recipient'))
RETURNING id, vendor_id, vendor_branch_id, name, "position", phone, email,
          is_notification_recipient, is_active, deleted_at;

-- name: UpdateVendorPicAdmin :one
-- vendor_id is immutable. `AND deleted_at IS NULL` makes a soft-disabled
-- target update 0 rows (mapped to not-found).
UPDATE vendor_pics
SET vendor_branch_id = sqlc.narg('vendor_branch_id'),
    name = sqlc.arg('name'),
    "position" = sqlc.narg('position'),
    phone = sqlc.narg('phone'),
    email = sqlc.narg('email'),
    is_notification_recipient = sqlc.arg('is_notification_recipient'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, vendor_id, vendor_branch_id, name, "position", phone, email,
          is_notification_recipient, is_active, deleted_at;

-- name: DisableVendorPic :exec
-- Soft-disable only (no_hard_delete_test.go extended in T3.7).
UPDATE vendor_pics SET is_active = false, deleted_at = now() WHERE id = $1;

-- name: EnableVendorPic :exec
UPDATE vendor_pics SET is_active = true, deleted_at = NULL WHERE id = $1;
