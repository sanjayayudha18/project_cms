-- Admin User & Vendor Management (Task 2): sqlc queries for the admin vendor
-- CRUD endpoints (Req 6-8). Vendors have no admin queries anywhere yet.

-- name: ListVendorsAdmin :many
-- Admin vendor list (Req 6) + direktur (vendor-pks 8.x). Filters: q (case-insensitive substring across
-- code/name), status — the caller resolves absent status to 'active' before calling (Resolved Decision 5),
-- so this query only sees 'active' | 'disabled' | 'all' — and the PAIR pks_type_id + pks_status (both NULL =
-- no filter; the service rejects only one of the two, vendor-pks-type Req 4.4). PKS per type is NOT selected
-- here: ListVendorPksSummaries fills it per page. The ap join hits vendor_pks_active_uq
-- (vendor_id, pks_type_id) WHERE replaced_at IS NULL, so it never duplicates rows. PKS status is computed by
-- vendor_pks_status() from today (Asia/Jakarta, from Go) and renewal_months; INTERNAL vendors yield NULL and
-- never match a pks_status filter (Req 4.6). Ordered name ASC, id ASC (Req 6.5).
SELECT v.id, v.code, v.name, v.legal_name, v.npwp, v.contact_email, v.contact_phone, v.hq_address, v.is_active, v.deleted_at,
       v.kind,
       dp.id AS director_id, dp.name AS director_name, dp.phone AS director_phone, dp.email AS director_email
FROM vendors v
LEFT JOIN vendor_pks ap ON ap.vendor_id = v.id AND ap.pks_type_id = sqlc.narg('pks_type_id')::bigint AND ap.replaced_at IS NULL
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
  AND (sqlc.narg('kind')::text IS NULL OR v.kind = sqlc.narg('kind')::text)
ORDER BY v.name ASC, v.id ASC
LIMIT sqlc.arg('page_limit')::bigint OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountVendorsAdmin :one
-- Same joins and filters as ListVendorsAdmin, no LIMIT/OFFSET — pagination total (Req 6.6).
SELECT COUNT(*)
FROM vendors v
LEFT JOIN vendor_pks ap ON ap.vendor_id = v.id AND ap.pks_type_id = sqlc.narg('pks_type_id')::bigint AND ap.replaced_at IS NULL
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
  AND (sqlc.narg('kind')::text IS NULL OR v.kind = sqlc.narg('kind')::text);

-- name: ListVendorPksSummaries :many
-- PKS summary per (vendor, active type) for one page of vendors (vendor-pks-type Req 4.1, 4.3, 4.5, 4.6;
-- design DD7). FLM_VENDOR only — INTERNAL vendors yield no rows. CROSS JOIN over active types, LEFT JOIN
-- the active PKS (vendor_pks_active_uq); no PKS -> status 'belum_diisi' from vendor_pks_status() and
-- remaining_days 0 (the handler maps belum_diisi to null). pks_status is COALESCEd to '' only to keep it non-null for sqlc/pgx.
SELECT v.id AS vendor_id, t.code, t.name,
       COALESCE(vendor_pks_status(v.kind, ap.expiry_date, ap.terminated_at, sqlc.arg('today')::date, sqlc.arg('renewal_months')::int), '')::text AS pks_status,
       COALESCE(ap.expiry_date - sqlc.arg('today')::date, 0)::int AS remaining_days
FROM vendors v
CROSS JOIN vendor_pks_types t
LEFT JOIN vendor_pks ap ON ap.vendor_id = v.id AND ap.pks_type_id = t.id AND ap.replaced_at IS NULL
WHERE v.id = ANY(sqlc.arg('vendor_ids')::bigint[])
  AND v.kind = 'FLM_VENDOR'
  AND t.is_active
ORDER BY v.id ASC, t.code ASC;

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
