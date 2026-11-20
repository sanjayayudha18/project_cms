-- Daftar PKS + reminder per PKS (spec .kiro/specs/vendor-pks-list/, task 3.1; migration 044).
-- ListPksAdmin / CountPksAdmin / ListPendingPksChanges are reads (replica pool); GetVendorPksForUpdate +
-- UpdateVendorPksRenewalDays run in the primary tx of VendorPksAdminService.UpdateReminder (GR#3 deviation:
-- renewal_days is the only content column updated directly, audit in the same tx).

-- name: ListPksAdmin :many
-- One row per vendor_pks row (history included), FLM_VENDOR only (Req 4.5). status filter: 'terkini' = active
-- row per (vendor, type) (replaced_at IS NULL), 'semua' = every row, anything else = vendor_pks_record_status()
-- label. today = Asia/Jakarta date from Go. Order: vendor name, type, newest effective first, id (A3).
SELECT p.id, p.vendor_id, v.code AS vendor_code, v.name AS vendor_name,
       p.pks_type_id, t.code AS pks_type_code, t.name AS pks_type_name,
       p.wod_number, p.pks_name, p.effective_date, p.expiry_date, p.renewal_days,
       p.replaced_at, p.terminated_at,
       vendor_pks_record_status(p.replaced_at, p.terminated_at, p.effective_date, p.expiry_date,
                                p.renewal_days, sqlc.arg('today')::date)::text AS status,
       (p.expiry_date - sqlc.arg('today')::date)::int AS remaining_days
FROM vendor_pks p
JOIN vendors v ON v.id = p.vendor_id
JOIN vendor_pks_types t ON t.id = p.pks_type_id
WHERE v.kind = 'FLM_VENDOR'
  AND (sqlc.arg('status')::text = 'semua'
        OR (sqlc.arg('status')::text = 'terkini' AND p.replaced_at IS NULL)
        OR vendor_pks_record_status(p.replaced_at, p.terminated_at, p.effective_date, p.expiry_date,
                                    p.renewal_days, sqlc.arg('today')::date) = sqlc.arg('status')::text)
  AND (sqlc.narg('pks_type')::text IS NULL OR t.code = sqlc.narg('pks_type')::text)
  AND (sqlc.narg('vendor')::text IS NULL OR v.code = sqlc.narg('vendor')::text)
  AND (sqlc.narg('q')::text IS NULL
        OR p.pks_name ILIKE '%' || sqlc.narg('q')::text || '%'
        OR p.wod_number ILIKE '%' || sqlc.narg('q')::text || '%')
ORDER BY v.name ASC, t.code ASC, p.effective_date DESC, p.id DESC
LIMIT sqlc.arg('page_limit')::bigint OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountPksAdmin :one
-- Same joins and filters as ListPksAdmin, no LIMIT/OFFSET (Req 4.6).
SELECT COUNT(*)
FROM vendor_pks p
JOIN vendors v ON v.id = p.vendor_id
JOIN vendor_pks_types t ON t.id = p.pks_type_id
WHERE v.kind = 'FLM_VENDOR'
  AND (sqlc.arg('status')::text = 'semua'
        OR (sqlc.arg('status')::text = 'terkini' AND p.replaced_at IS NULL)
        OR vendor_pks_record_status(p.replaced_at, p.terminated_at, p.effective_date, p.expiry_date,
                                    p.renewal_days, sqlc.arg('today')::date) = sqlc.arg('status')::text)
  AND (sqlc.narg('pks_type')::text IS NULL OR t.code = sqlc.narg('pks_type')::text)
  AND (sqlc.narg('vendor')::text IS NULL OR v.code = sqlc.narg('vendor')::text)
  AND (sqlc.narg('q')::text IS NULL
        OR p.pks_name ILIKE '%' || sqlc.narg('q')::text || '%'
        OR p.wod_number ILIKE '%' || sqlc.narg('q')::text || '%');

-- name: ListPendingPksChanges :many
-- Pending master-data changes of entity_type 'vendor_pks' (op create/disable/enable; at most one per
-- (vendor, type), DB index), with the same vendor / pks_type code filters as ListPksAdmin. Feeds the
-- menunggu_persetujuan rows (op create) and the per-(vendor, type) pending marker (Req 4.1, 4.2).
SELECT c.id, c.op, c.entity_id, c.payload, c.maker_id, c.created_at,
       (c.payload->>'vendor_id')::bigint AS vendor_id,
       (c.payload->>'pks_type_id')::bigint AS pks_type_id,
       v.code AS vendor_code, v.name AS vendor_name,
       t.code AS pks_type_code, t.name AS pks_type_name
FROM master_data_change_requests c
JOIN vendors v ON v.id = (c.payload->>'vendor_id')::bigint
JOIN vendor_pks_types t ON t.id = (c.payload->>'pks_type_id')::bigint
WHERE c.entity_type = 'vendor_pks'
  AND c.status = 'pending'
  AND v.kind = 'FLM_VENDOR'
  AND (sqlc.narg('pks_type')::text IS NULL OR t.code = sqlc.narg('pks_type')::text)
  AND (sqlc.narg('vendor')::text IS NULL OR v.code = sqlc.narg('vendor')::text)
ORDER BY v.name ASC, t.code ASC, c.id ASC;

-- name: GetVendorPksForUpdate :one
-- Reminder tx (primary): lock the row so effective/expiry cannot change under the validation (Req 2.1, 2.6).
SELECT id, vendor_id, pks_type_id, wod_number, pks_name, effective_date, expiry_date, renewal_days,
       replaced_at, terminated_at
FROM vendor_pks
WHERE id = $1
FOR UPDATE;

-- name: UpdateVendorPksRenewalDays :one
-- The only content column of vendor_pks updated directly (Req 2.1, 11.3). Zero rows = pgx.ErrNoRows.
UPDATE vendor_pks
SET renewal_days = sqlc.arg('renewal_days')
WHERE id = sqlc.arg('id')
RETURNING id, vendor_id, pks_type_id, wod_number, pks_name, effective_date, expiry_date, renewal_days,
          replaced_at, terminated_at;
