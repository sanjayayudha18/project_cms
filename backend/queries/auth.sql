-- name: FindUserByUsername :one
-- Retrieves user with joined role name, excluding soft-deleted users.
SELECT u.id, u.username, u.full_name, u.email, u.password_hash,
       u.auth_source, u.role_id, r.role, u.is_karyawan,
       u.vendor_id, u.is_active, u.deleted_at
FROM users u
JOIN roles r ON r.id = u.role_id
WHERE u.username = $1 AND u.deleted_at IS NULL;

-- name: UpdateLastLogin :exec
-- Updates last_login_at timestamp after successful authentication.
UPDATE users SET last_login_at = now() WHERE id = $1;

-- name: GetUserProfile :one
-- Retrieves user profile for the /me endpoint, excluding soft-deleted users.
SELECT u.id, u.username, u.full_name, u.email, r.role,
       u.is_karyawan, u.vendor_id
FROM users u
JOIN roles r ON r.id = u.role_id
WHERE u.id = $1 AND u.deleted_at IS NULL;
