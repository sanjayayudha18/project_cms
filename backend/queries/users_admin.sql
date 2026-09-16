-- Admin User & Vendor Management (Task 2): sqlc queries for the admin user
-- CRUD endpoints (Req 2-5). Kept separate from auth.sql to avoid churn on the
-- existing login/self-service auth queries.

-- name: ListUsersAdmin :many
-- Admin user list (Req 2). Filters: q (case-insensitive substring across
-- username/full_name/email), role (exact match on roles.role), vendor_id,
-- and status — the caller resolves absent status to 'active' before calling
-- (Resolved Decision 5), so this query only ever sees 'active' | 'disabled' | 'all'.
-- Ordered full_name ASC, id ASC (Req 2.7 stable tiebreaker).
SELECT u.id, u.username, u.full_name, u.email, r.role, u.is_karyawan,
       u.auth_source, u.vendor_id, u.is_active, u.deleted_at,
       u.supervisor_id, u.approval_level, u.last_login_at
FROM users u
JOIN roles r ON r.id = u.role_id
WHERE (sqlc.narg('q')::text IS NULL
        OR u.username ILIKE '%' || sqlc.narg('q')::text || '%'
        OR u.full_name ILIKE '%' || sqlc.narg('q')::text || '%'
        OR u.email ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (sqlc.narg('role')::text IS NULL OR r.role = sqlc.narg('role')::text)
  AND (sqlc.narg('vendor_id')::bigint IS NULL OR u.vendor_id = sqlc.narg('vendor_id')::bigint)
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND u.deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND u.deleted_at IS NOT NULL)
      )
ORDER BY u.full_name ASC, u.id ASC
LIMIT sqlc.arg('page_limit')::bigint OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountUsersAdmin :one
-- Same filters as ListUsersAdmin, no LIMIT/OFFSET — pagination total (Req 2.8).
SELECT COUNT(*)
FROM users u
JOIN roles r ON r.id = u.role_id
WHERE (sqlc.narg('q')::text IS NULL
        OR u.username ILIKE '%' || sqlc.narg('q')::text || '%'
        OR u.full_name ILIKE '%' || sqlc.narg('q')::text || '%'
        OR u.email ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (sqlc.narg('role')::text IS NULL OR r.role = sqlc.narg('role')::text)
  AND (sqlc.narg('vendor_id')::bigint IS NULL OR u.vendor_id = sqlc.narg('vendor_id')::bigint)
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND u.deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND u.deleted_at IS NOT NULL)
      );

-- name: GetUserAdminByID :one
-- Unlike auth.sql's FindUserByID, this INCLUDES soft-deleted rows — the admin
-- API needs to tell "not found" (404) apart from "disabled" (also 404 for
-- edit per Req 4.4, but the disable/enable existence pre-check needs to see
-- it). Never selects password_hash.
SELECT u.id, u.username, u.full_name, u.email, r.role, u.role_id, u.is_karyawan,
       u.auth_source, u.employee_id, u.vendor_id, u.is_active, u.deleted_at,
       u.supervisor_id, u.approval_level, u.last_login_at
FROM users u
JOIN roles r ON r.id = u.role_id
WHERE u.id = $1;

-- name: FindUserAdminByUsername :one
-- Uniqueness pre-check (incl. soft-deleted) for create (Req 3.5). Named
-- distinctly from auth.sql's FindUserByUsername (different projection, no
-- deleted_at filter) to avoid a sqlc name collision.
SELECT id FROM users WHERE username = $1;

-- name: FindUserAdminByEmail :one
-- Uniqueness pre-check (incl. soft-deleted) for create (Req 3.5).
SELECT id FROM users WHERE email = $1;

-- name: FindUserAdminByEmailExcludingID :one
-- Uniqueness pre-check for update (Req 4.5): a collision with a DIFFERENT record.
SELECT id FROM users WHERE email = $1 AND id != $2;

-- name: FindUserAdminByEmployeeID :one
-- Uniqueness pre-check (incl. soft-deleted) for create (Req 3.5). employee_id
-- is nullable; callers only run this when employee_id is non-empty.
SELECT id FROM users WHERE employee_id = $1;

-- name: FindUserAdminByEmployeeIDExcludingID :one
-- Uniqueness pre-check for update (Req 4.5): a collision with a DIFFERENT record.
SELECT id FROM users WHERE employee_id = $1 AND id != $2;

-- name: CreateUserAdmin :one
-- Single INSERT (Resolved Decision 5): for auth_source=local, password_hash
-- and must_change_password=true are set in THIS insert — no follow-up
-- SetInitialPassword call, so there is one write and one audit entry, and no
-- window where the user row exists without its password. For auth_source=ldap,
-- password_hash is NULL and must_change_password stays false (column default).
INSERT INTO users (
    role_id, employee_id, username, full_name, email, is_karyawan,
    auth_source, password_hash, vendor_id, supervisor_id, approval_level,
    must_change_password
) VALUES (
    sqlc.arg('role_id'), sqlc.narg('employee_id'), sqlc.arg('username'),
    sqlc.arg('full_name'), sqlc.arg('email'), sqlc.arg('is_karyawan'),
    sqlc.arg('auth_source'), sqlc.narg('password_hash'), sqlc.narg('vendor_id'),
    sqlc.narg('supervisor_id'), sqlc.narg('approval_level'),
    sqlc.arg('must_change_password')
)
RETURNING id, username, full_name, email, role_id, is_karyawan, auth_source,
          employee_id, vendor_id, is_active, deleted_at, supervisor_id,
          approval_level, last_login_at;

-- name: UpdateUserAdmin :one
-- Editable fields only (Req 4.1); username/auth_source/password_hash are
-- never touched here. `AND deleted_at IS NULL` makes a soft-disabled target
-- update as 0 rows affected, which the repository maps to "not found" (Req 4.4).
UPDATE users
SET full_name = sqlc.arg('full_name'),
    email = sqlc.arg('email'),
    role_id = sqlc.arg('role_id'),
    is_karyawan = sqlc.arg('is_karyawan'),
    employee_id = sqlc.narg('employee_id'),
    vendor_id = sqlc.narg('vendor_id'),
    supervisor_id = sqlc.narg('supervisor_id'),
    approval_level = sqlc.narg('approval_level'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, username, full_name, email, role_id, is_karyawan, auth_source,
          employee_id, vendor_id, is_active, deleted_at, supervisor_id,
          approval_level, last_login_at;

-- name: GetRoleByName :one
-- Resolves a role text (e.g. "APPACCESS") to its role_id for create/update
-- (Req 3.6, 4.7). No role repository exists yet — this is its first query.
SELECT id, role FROM roles WHERE role = $1;

-- name: ListRoles :many
-- Populates the Role select in User_Form_Dialog (Req 11.2).
SELECT id, role FROM roles ORDER BY role;

-- name: VendorExistsAdmin :one
-- Reference check for vendor_id on user create/update (Req 3.7, 4.7) — only
-- an active vendor is a valid reference, matching Vendor_Admin's own 404-on-
-- disabled convention (Requirement 7.7).
SELECT EXISTS(SELECT 1 FROM vendors WHERE id = $1 AND deleted_at IS NULL) AS exists;

-- name: UserAdminExists :one
-- Reference check for supervisor_id on user create/update (Req 3.7, 4.7) —
-- only an active (non-disabled) user is a valid supervisor.
SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL) AS exists;
