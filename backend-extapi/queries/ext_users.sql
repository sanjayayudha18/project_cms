-- ext-user-api: user management on behalf of external clients. Always the
-- primary pool; writes run inside the TxRunner transaction.

-- name: LockUserForExt :one
-- Role comes from a scalar subquery, not a JOIN: with a JOIN, a concurrent
-- role_id change makes Postgres re-check the join against the old roles row
-- after the lock wait, the row drops out and the caller gets a false 404.
-- The subquery runs on the locked (latest) row version.
SELECT u.id, u.auth_source, u.deleted_at, u.is_active,
       (SELECT r.role FROM roles r WHERE r.id = u.role_id)::text AS role
FROM users u
WHERE u.id = $1
FOR UPDATE;

-- name: GetExtUser :one
SELECT u.id, u.username, u.full_name, u.email, r.role, u.employee_id, u.supervisor_id,
       u.approval_level, u.is_active, u.auth_source, u.deleted_at, u.created_at, u.updated_at
FROM users u
JOIN roles r ON r.id = u.role_id
WHERE u.id = $1;

-- name: GetExtUserByUsername :one
SELECT u.id, u.username, u.full_name, u.email, r.role, u.employee_id, u.supervisor_id,
       u.approval_level, u.is_active, u.auth_source, u.deleted_at, u.created_at, u.updated_at
FROM users u
JOIN roles r ON r.id = u.role_id
WHERE u.username = $1;

-- name: FindUserIDByUsername :one
SELECT id FROM users WHERE username = $1;

-- name: FindUserIDByEmail :one
SELECT id FROM users WHERE email = $1;

-- name: FindUserIDByEmailExcludingID :one
SELECT id FROM users WHERE email = $1 AND id != $2;

-- name: FindUserIDByEmployeeID :one
SELECT id FROM users WHERE employee_id = $1;

-- name: FindUserIDByEmployeeIDExcludingID :one
SELECT id FROM users WHERE employee_id = $1 AND id != $2;

-- name: GetRoleIDByName :one
SELECT id FROM roles WHERE role = $1;

-- name: CreateExtUser :one
INSERT INTO users (role_id, employee_id, username, full_name, email, is_karyawan,
                   auth_source, password_hash, must_change_password, supervisor_id, approval_level)
VALUES (sqlc.arg('role_id'), sqlc.narg('employee_id'), sqlc.arg('username'), sqlc.arg('full_name'),
        sqlc.arg('email'), true, 'ldap', NULL, false, sqlc.narg('supervisor_id'),
        sqlc.narg('approval_level'))
RETURNING id;

-- name: UpdateExtUser :execrows
UPDATE users
SET full_name = sqlc.arg('full_name'),
    email = sqlc.arg('email'),
    role_id = sqlc.arg('role_id'),
    employee_id = sqlc.narg('employee_id'),
    supervisor_id = sqlc.narg('supervisor_id'),
    approval_level = sqlc.narg('approval_level'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: DeactivateExtUser :execrows
UPDATE users
SET is_active = false, deleted_at = now(), updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: ReactivateExtUser :execrows
UPDATE users
SET is_active = true, deleted_at = NULL, updated_at = now()
WHERE id = $1 AND deleted_at IS NOT NULL;
