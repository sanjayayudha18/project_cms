-- name: GetActiveUserRole :one
-- Current role of an active, not-deleted user (read on primary at download time,
-- so a revoked role takes effect immediately, Req 7.6). No row = not allowed.
SELECT r.role
FROM users u
JOIN roles r ON r.id = u.role_id
WHERE u.id = sqlc.arg('user_id')::bigint
  AND u.is_active
  AND u.deleted_at IS NULL;
