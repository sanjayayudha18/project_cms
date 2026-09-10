-- name: FindUserByUsername :one
-- Retrieves user with joined role name, excluding soft-deleted users.
SELECT u.id, u.username, u.full_name, u.email, u.password_hash,
       u.auth_source, u.role_id, r.role, u.is_karyawan,
       u.vendor_id, u.is_active, u.deleted_at,
       u.supervisor_id, u.approval_level, u.password_changed_at,
       u.failed_login_attempts, u.locked_until, u.must_change_password
FROM users u
JOIN roles r ON r.id = u.role_id
WHERE u.username = $1 AND u.deleted_at IS NULL;

-- name: FindUserByID :one
-- Same shape as FindUserByUsername but keyed by id — used by self-service
-- actions (change-password) where the caller is already authenticated via
-- JWT and only has UserID, not username.
SELECT u.id, u.username, u.full_name, u.email, u.password_hash,
       u.auth_source, u.role_id, r.role, u.is_karyawan,
       u.vendor_id, u.is_active, u.deleted_at,
       u.supervisor_id, u.approval_level, u.password_changed_at,
       u.failed_login_attempts, u.locked_until, u.must_change_password
FROM users u
JOIN roles r ON r.id = u.role_id
WHERE u.id = $1 AND u.deleted_at IS NULL;

-- name: UpdateLastLogin :exec
-- Updates last_login_at timestamp after successful authentication.
UPDATE users SET last_login_at = now() WHERE id = $1;

-- name: MarkPasswordExpired :exec
-- Forces a password change on next login when the local-password 90-day
-- expiry policy rejects a login attempt (auth_source=local|local_dev only).
UPDATE users SET must_change_password = true WHERE id = $1;

-- name: IncrementFailedLogin :one
-- Increments failed_login_attempts by 1 and returns the new count. The
-- lockout threshold (auth.MaxFailedLogins) is decided by the service layer,
-- not here, so it stays unit-testable without a live database.
UPDATE users SET failed_login_attempts = failed_login_attempts + 1
WHERE id = $1
RETURNING failed_login_attempts;

-- name: LockAccount :exec
-- Locks a local-password account until the given timestamp
-- (auth_source=local|local_dev only; called by the service after
-- IncrementFailedLogin reaches auth.MaxFailedLogins).
UPDATE users SET locked_until = $2 WHERE id = $1;

-- name: ResetLockout :exec
-- Clears failed_login_attempts and locked_until after a successful login.
UPDATE users SET failed_login_attempts = 0, locked_until = NULL WHERE id = $1;

-- name: SetPassword :exec
-- Self-service change-password (Task 5): sets a new bcrypt hash, marks it as
-- freshly changed, clears must_change_password, and resets the lockout
-- counters — a legitimate password change is as good as a successful login
-- for lockout-recovery purposes.
UPDATE users
SET password_hash = $2,
    password_changed_at = now(),
    must_change_password = false,
    failed_login_attempts = 0,
    locked_until = NULL
WHERE id = $1;

-- name: SetInitialPassword :exec
-- APPACCESS admin action (Task 6): sets a new bcrypt hash for a target user
-- and forces a change on their next login (must_change_password=true) —
-- the opposite of SetPassword's must_change_password=false. No old password
-- required; this is a set, not a self-service change.
UPDATE users
SET password_hash = $2,
    password_changed_at = now(),
    must_change_password = true,
    failed_login_attempts = 0,
    locked_until = NULL
WHERE id = $1;

-- name: DeactivateUser :exec
-- Soft-delete only (Task 7). Sets is_active=false, deleted_at=now(). There
-- is no DELETE FROM users query anywhere in this codebase: audit_logs rows
-- (actor_id -> users.id) must stay linked forever, so a user row is never
-- physically removed, audited or not. FindUserByUsername already filters
-- deleted_at IS NULL, so a deactivated user's login is closed automatically.
UPDATE users SET is_active = false, deleted_at = now() WHERE id = $1;

-- name: ReactivateUser :exec
-- Reverses DeactivateUser (optional admin action): clears deleted_at, sets
-- is_active back to true.
UPDATE users SET is_active = true, deleted_at = NULL WHERE id = $1;

-- name: GetUserProfile :one
-- Retrieves user profile for the /me endpoint, excluding soft-deleted users.
SELECT u.id, u.username, u.full_name, u.email, r.role,
       u.is_karyawan, u.vendor_id,
       u.supervisor_id, u.approval_level
FROM users u
JOIN roles r ON r.id = u.role_id
WHERE u.id = $1 AND u.deleted_at IS NULL;
