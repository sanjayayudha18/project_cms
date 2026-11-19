-- Daftar PKS export (spec .kiro/specs/vendor-pks-list/, task 6.1, Req 3.4/10.1/10.3). Read-only.
-- SHARED FILE: backend-exq generates these queries (its sqlc.yaml lists this path); backend does not.
--
-- One row per vendor_pks row (history included), FLM_VENDOR only, no pending rows (A2). Filters are the same
-- as ListPksAdmin (vendor_pks_list.sql) — keep in sync (Property 6: screen and export agree):
--   status   : 'terkini' = replaced_at IS NULL, 'semua' = every row, else a vendor_pks_record_status() label
--   pks_type : NULL or vendor_pks_types.code;  vendor : NULL or vendors.code;  q : NULL or ILIKE on name / WOD
--   today    : Asia/Jakarta date from Go.
-- Masa Berlaku (hari) = expiry_date - effective_date; remaining days are not exported.
--
-- Keyset on (vendor code, type code, 9999-12-31 - effective_date, id) = newest effective first without a mixed
-- sort direction. First page: after_vendor_code = '', after_type_code = '', after_rev_effective = 0, after_id = 0.
-- Director = the vendor's single active vendor_pics.is_director row (partial unique index), so the LEFT JOIN
-- never multiplies rows. Function results are cast explicitly so sqlc does not infer their types.

-- name: ExportVendorPksBatch :many
SELECT p.id,
       v.code                                                        AS vendor_code,
       v.name                                                        AS vendor_name,
       d.name                                                        AS director_name,
       d.phone                                                       AS director_phone,
       d.email                                                       AS director_email,
       t.code                                                        AS pks_type_code,
       p.wod_number                                                  AS wod_number,
       p.pks_name                                                    AS pks_name,
       p.effective_date                                              AS effective_date,
       p.expiry_date                                                 AS expiry_date,
       (p.expiry_date - p.effective_date)::int                       AS validity_days,
       p.renewal_days                                                AS renewal_days,
       vendor_pks_record_status(p.replaced_at, p.terminated_at, p.effective_date, p.expiry_date,
                                p.renewal_days, sqlc.arg('today')::date)::text AS pks_status,
       (DATE '9999-12-31' - p.effective_date)::int                   AS rev_effective
FROM vendor_pks p
JOIN vendors v ON v.id = p.vendor_id
JOIN vendor_pks_types t ON t.id = p.pks_type_id
LEFT JOIN vendor_pics d ON d.vendor_id = v.id AND d.is_director AND d.deleted_at IS NULL
WHERE v.kind = 'FLM_VENDOR'
  AND (v.code, t.code, (DATE '9999-12-31' - p.effective_date)::int, p.id)
        > (sqlc.arg('after_vendor_code')::text, sqlc.arg('after_type_code')::text,
           sqlc.arg('after_rev_effective')::int, sqlc.arg('after_id')::bigint)
  AND (sqlc.arg('status')::text = 'semua'
        OR (sqlc.arg('status')::text = 'terkini' AND p.replaced_at IS NULL)
        OR vendor_pks_record_status(p.replaced_at, p.terminated_at, p.effective_date, p.expiry_date,
                                    p.renewal_days, sqlc.arg('today')::date) = sqlc.arg('status')::text)
  AND (sqlc.narg('pks_type')::text IS NULL OR t.code = sqlc.narg('pks_type')::text)
  AND (sqlc.narg('vendor')::text IS NULL OR v.code = sqlc.narg('vendor')::text)
  AND (sqlc.narg('q')::text IS NULL
        OR p.pks_name ILIKE '%' || sqlc.narg('q')::text || '%'
        OR p.wod_number ILIKE '%' || sqlc.narg('q')::text || '%')
ORDER BY v.code, t.code, (DATE '9999-12-31' - p.effective_date)::int, p.id
LIMIT sqlc.arg('batch_size')::int;

-- name: CountVendorPksReport :one
-- Same joins and filters as ExportVendorPksBatch, without keyset/LIMIT.
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
