-- Admin User & Vendor Management (Task 2): sqlc queries for the admin vendor
-- CRUD endpoints (Req 6-8). Vendors have no admin queries anywhere yet.

-- name: ListVendorsAdmin :many
-- Admin vendor list (Req 6). Filters: q (case-insensitive substring across
-- code/name) and status — the caller resolves absent status to 'active'
-- before calling (Resolved Decision 5), so this query only ever sees
-- 'active' | 'disabled' | 'all'. Ordered name ASC, id ASC (Req 6.5).
SELECT id, code, name, contact_email, contact_phone, hq_address, is_active, deleted_at
FROM vendors
WHERE (sqlc.narg('q')::text IS NULL
        OR code ILIKE '%' || sqlc.narg('q')::text || '%'
        OR name ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND deleted_at IS NOT NULL)
      )
ORDER BY name ASC, id ASC
LIMIT sqlc.arg('page_limit')::bigint OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountVendorsAdmin :one
-- Same filters as ListVendorsAdmin, no LIMIT/OFFSET — pagination total (Req 6.6).
SELECT COUNT(*)
FROM vendors
WHERE (sqlc.narg('q')::text IS NULL
        OR code ILIKE '%' || sqlc.narg('q')::text || '%'
        OR name ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND deleted_at IS NOT NULL)
      );

-- name: GetVendorAdminByID :one
-- Includes soft-deleted rows — 404 pre-checks on update/disable/enable treat
-- a soft-disabled vendor as not-found for edit (Req 7.7), consistent with
-- GetUserAdminByID's convention.
SELECT id, code, name, contact_email, contact_phone, hq_address, is_active, deleted_at
FROM vendors WHERE id = $1;

-- name: FindVendorAdminByCode :one
-- Uniqueness pre-check (incl. soft-deleted) for create (Req 7.3).
SELECT id FROM vendors WHERE code = $1;

-- name: CreateVendorAdmin :one
INSERT INTO vendors (code, name, contact_email, contact_phone, hq_address)
VALUES (sqlc.arg('code'), sqlc.arg('name'), sqlc.narg('contact_email'),
        sqlc.narg('contact_phone'), sqlc.narg('hq_address'))
RETURNING id, code, name, contact_email, contact_phone, hq_address, is_active, deleted_at;

-- name: UpdateVendorAdmin :one
-- Editable fields only (Req 7.5); code is never touched here (immutable,
-- Req 7.6). `AND deleted_at IS NULL` makes a soft-disabled target update as 0
-- rows affected, mapped to "not found" by the repository (Req 7.7).
UPDATE vendors
SET name = sqlc.arg('name'),
    contact_email = sqlc.narg('contact_email'),
    contact_phone = sqlc.narg('contact_phone'),
    hq_address = sqlc.narg('hq_address'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, code, name, contact_email, contact_phone, hq_address, is_active, deleted_at;

-- name: DisableVendor :exec
-- Soft-disable only (Req 8.1): is_active=false, deleted_at=now(). No hard
-- DELETE on vendors anywhere in this codebase (Req 8.3, Task 6 guard).
UPDATE vendors SET is_active = false, deleted_at = now() WHERE id = $1;

-- name: EnableVendor :exec
-- Reverses DisableVendor (Req 8.2): is_active=true, deleted_at=NULL.
UPDATE vendors SET is_active = true, deleted_at = NULL WHERE id = $1;

-- name: CountActiveUsersByVendor :one
-- Backs the Req 8.5 linked-active-users warning shown when a vendor with
-- active (non-disabled) users is disabled — the disable still succeeds.
SELECT COUNT(*) FROM users WHERE vendor_id = $1 AND deleted_at IS NULL;
