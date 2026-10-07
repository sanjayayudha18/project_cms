-- Notifikasi in-app + outbox email (Phase 0.3, .claude/sdlc/notification/spec.md).

-- name: ListActiveUserRecipientsByIDs :many
SELECT u.id, u.email
FROM users u
WHERE u.id = ANY(sqlc.arg('ids')::bigint[])
  AND u.is_active = true AND u.deleted_at IS NULL;

-- name: ListActiveUserRecipientsByRoles :many
SELECT u.id, u.email
FROM users u
JOIN roles r ON r.id = u.role_id
WHERE r.role = ANY(sqlc.arg('roles')::text[])
  AND u.is_active = true AND u.deleted_at IS NULL;

-- name: ListActiveUserRecipientsByVendors :many
SELECT u.id, u.email
FROM users u
WHERE u.vendor_id = ANY(sqlc.arg('vendor_ids')::bigint[])
  AND u.is_active = true AND u.deleted_at IS NULL;

-- name: ListVendorNotificationPicEmails :many
-- PIC vendor bukan user: email saja, tanpa notifikasi in-app.
SELECT p.email::text AS email
FROM vendor_pics p
WHERE p.vendor_id = ANY(sqlc.arg('vendor_ids')::bigint[])
  AND p.is_notification_recipient = true
  AND p.is_active = true AND p.deleted_at IS NULL
  AND COALESCE(btrim(p.email), '') <> '';

-- name: InsertNotification :one
INSERT INTO notifications (recipient_user_id, type, title, body, link, entity_type, entity_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id;

-- name: InsertNotificationEmail :exec
INSERT INTO notification_emails (notification_id, to_address, subject, body, status)
VALUES ($1, $2, $3, $4, $5);

-- name: ListNotificationsForUser :many
SELECT id, type, title, body, link, is_read, created_at
FROM notifications
WHERE recipient_user_id = sqlc.arg('user_id')
  AND (NOT sqlc.arg('unread_only')::boolean OR is_read = false)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('page_limit') OFFSET sqlc.arg('page_offset');

-- name: CountNotificationsForUser :one
SELECT count(*)
FROM notifications
WHERE recipient_user_id = sqlc.arg('user_id')
  AND (NOT sqlc.arg('unread_only')::boolean OR is_read = false);

-- name: CountUnreadNotifications :one
SELECT count(*) FROM notifications WHERE recipient_user_id = $1 AND is_read = false;

-- name: MarkNotificationRead :one
-- Idempotent: an already-read row still matches (read_at kept), so the
-- handler answers 204; no row = not the caller's (404, no existence leak).
UPDATE notifications
SET is_read = true, read_at = COALESCE(read_at, now())
WHERE id = sqlc.arg('id') AND recipient_user_id = sqlc.arg('user_id')
RETURNING id;

-- name: MarkAllNotificationsRead :execrows
UPDATE notifications
SET is_read = true, read_at = now()
WHERE recipient_user_id = $1 AND is_read = false;

-- name: ClaimPendingNotificationEmails :many
-- Row locks held until the caller's tx ends; SKIP LOCKED keeps a second
-- cmd/api instance from sending the same email.
SELECT id, to_address, subject, body, attempts
FROM notification_emails
WHERE status = 'pending' AND next_attempt_at <= now()
ORDER BY next_attempt_at, id
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkNotificationEmailSent :exec
UPDATE notification_emails
SET status = 'sent', sent_at = now(), attempts = attempts + 1, last_error = NULL
WHERE id = $1;

-- name: MarkNotificationEmailRetry :exec
UPDATE notification_emails
SET attempts = attempts + 1, last_error = sqlc.arg('last_error'), next_attempt_at = sqlc.arg('next_attempt_at')
WHERE id = sqlc.arg('id');

-- name: MarkNotificationEmailFailed :exec
UPDATE notification_emails
SET status = 'failed', attempts = attempts + 1, last_error = sqlc.arg('last_error')
WHERE id = sqlc.arg('id');

-- name: DeleteOldNotificationEmails :execrows
DELETE FROM notification_emails
WHERE created_at < sqlc.arg('before') AND status IN ('sent', 'failed', 'skipped');

-- name: DeleteOldNotifications :execrows
DELETE FROM notifications WHERE created_at < sqlc.arg('before');
