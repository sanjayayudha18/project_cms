-- Admin User & Vendor Management (Task 2): sqlc queries for the admin vendor
-- CRUD endpoints (Req 6-8). Vendors have no admin queries anywhere yet.

-- name: ListVendorsAdmin :many
-- COALESCE keeps these non-null for sqlc/pgx (a NULL scan into int32 fails): 0/'' means
-- "no active PKS" (pks_id IS NULL) or, for pks_status, an INTERNAL vendor. Filter below uses the raw function.
-- Admin vendor list (Req 6) + PKS/direktur (vendor-pks 8.1-8.5). Filters: q
-- (case-insensitive substring across code/name), status — the caller resolves
-- absent status to 'active' before calling (Resolved Decision 5), so this
-- query only ever sees 'active' | 'disabled' | 'all' — and pks_status (NULL =
-- no filter). PKS status is computed by vendor_pks_status() from today
-- (Asia/Jakarta, from Go) and renewal_months. Ordered name ASC, id ASC (Req 6.5).
SELECT v.id, v.code, v.name, v.legal_name, v.npwp, v.contact_email, v.contact_phone, v.hq_address, v.is_active, v.deleted_at,
       v.kind,
       dp.id AS director_id, dp.name AS director_name, dp.phone AS director_phone, dp.email AS director_email,
       ap.id AS pks_id, ap.pks_name, ap.effective_date AS pks_effective_date, ap.expiry_date AS pks_expiry_date,
       COALESCE(vendor_pks_validity_months(ap.effective_date, ap.expiry_date), 0)::int AS pks_validity_months,
       COALESCE(ap.expiry_date - sqlc.arg('today')::date, 0)::int AS pks_remaining_days,
       COALESCE(vendor_pks_status(v.kind, ap.expiry_date, ap.terminated_at, sqlc.arg('today')::date, sqlc.arg('renewal_months')::int), '')::text AS pks_status
FROM vendors v
LEFT JOIN vendor_pks ap ON ap.vendor_id = v.id AND ap.replaced_at IS NULL
LEFT JOIN vendor_pics dp ON dp.vendor_id = v.id AND dp.is_director AND dp.deleted_at IS NULL
WHERE (sqlc.narg('q')::text IS NULL
        OR v.code ILIKE '%' || sqlc.narg('q')::text || '%'
        OR v.name ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND v.deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND v.deleted_at IS NOT NULL)
      )
  AND (sqlc.narg('pks_status')::text IS NULL
        OR vendor_pks_status(v.kind, ap.expiry_date, ap.terminated_at, sqlc.arg('today')::date, sqlc.arg('renewal_months')::int) = sqlc.narg('pks_status')::text)
ORDER BY v.name ASC, v.id ASC
LIMIT sqlc.arg('page_limit')::bigint OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountVendorsAdmin :one
-- Same joins and filters as ListVendorsAdmin, no LIMIT/OFFSET — pagination total (Req 6.6).
SELECT COUNT(*)
FROM vendors v
LEFT JOIN vendor_pks ap ON ap.vendor_id = v.id AND ap.replaced_at IS NULL
WHERE (sqlc.narg('q')::text IS NULL
        OR v.code ILIKE '%' || sqlc.narg('q')::text || '%'
        OR v.name ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND v.deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND v.deleted_at IS NOT NULL)
      )
  AND (sqlc.narg('pks_status')::text IS NULL
        OR vendor_pks_status(v.kind, ap.expiry_date, ap.terminated_at, sqlc.arg('today')::date, sqlc.arg('renewal_months')::int) = sqlc.narg('pks_status')::text);

-- name: GetVendorDirectorPic :one
-- Active Director_PIC (Direktur Utama) of one vendor, or no row (vendor-pks
-- D32, DD11). Separate from GetVendorAdminByID, which is the maker-checker
-- `before` snapshot shape and must not change.
SELECT id, name, phone, email
FROM vendor_pics
WHERE vendor_id = $1 AND is_director AND deleted_at IS NULL;

-- name: GetVendorAdminByID :one
-- Includes soft-deleted rows — 404 pre-checks on update/disable/enable treat
-- a soft-disabled vendor as not-found for edit (Req 7.7), consistent with
-- GetUserAdminByID's convention.
SELECT id, code, name, legal_name, npwp, contact_email, contact_phone, hq_address, is_active, deleted_at
FROM vendors WHERE id = $1;

-- name: FindVendorAdminByCode :one
-- Uniqueness pre-check (incl. soft-deleted) for create (Req 7.3).
SELECT id FROM vendors WHERE code = $1;

-- name: CreateVendorAdmin :one
INSERT INTO vendors (code, name, legal_name, npwp, contact_email, contact_phone, hq_address)
VALUES (sqlc.arg('code'), sqlc.arg('name'), sqlc.narg('legal_name'), sqlc.narg('npwp'),
        sqlc.narg('contact_email'), sqlc.narg('contact_phone'), sqlc.narg('hq_address'))
RETURNING id, code, name, legal_name, npwp, contact_email, contact_phone, hq_address, is_active, deleted_at;

-- name: UpdateVendorAdmin :one
-- Editable fields only (Req 7.5); code is never touched here (immutable,
-- Req 7.6). `AND deleted_at IS NULL` makes a soft-disabled target update as 0
-- rows affected, mapped to "not found" by the repository (Req 7.7).
UPDATE vendors
SET name = sqlc.arg('name'),
    legal_name = sqlc.narg('legal_name'),
    npwp = sqlc.narg('npwp'),
    contact_email = sqlc.narg('contact_email'),
    contact_phone = sqlc.narg('contact_phone'),
    hq_address = sqlc.narg('hq_address'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, code, name, legal_name, npwp, contact_email, contact_phone, hq_address, is_active, deleted_at;

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
