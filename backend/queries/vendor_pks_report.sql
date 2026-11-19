-- Vendor + PKS report export (vendor-pks task 4.3, Req 10; reshaped by vendor-pks-type task 2.4, Req 6.1/6.2). Read-only.
-- SHARED FILE: backend-exq generates these queries too (its sqlc.yaml lists
-- this path). Regenerate in BOTH modules.
--
-- Two branches (design DD9):
--   * ExportVendorPksBatch / CountVendorPksReport: one row per ACTIVE vendor_pks
--     (replaced_at IS NULL, vendor_pks_active_uq = one per (vendor, type)), with type + WOD.
--   * ExportVendorPksMissingBatch / CountVendorPksMissing: pks_status = belum_diisi, one row per
--     FLM_VENDOR without an active PKS of the requested type; PKS cells stay empty (A3).
--
-- Predicates q / status are copied verbatim from ListVendorsAdmin (vendors_admin.sql); keep in sync.
-- `status` is 'active' | 'disabled' | 'all' (caller resolves absent to 'active').
-- `pks_type` NULL = every type; otherwise a vendor_pks_types.code (caller validated it, active).
-- `pks_status` NULL = no filter; otherwise one of aktif | need_renewal | expired | terminated
-- (belum_diisi uses the Missing branch). `today` = Asia/Jakarta date from Go (D30),
-- `renewal_months` = VENDOR_PKS_RENEWAL_MONTHS.
--
-- Keyset page on (vendor code, type code): first page after_vendor_code = '' and after_type_code = ''.
-- Director = the vendor's single active vendor_pics.is_director row (partial unique index
-- vendor_pics_director_uq), so the LEFT JOIN never multiplies rows.
-- Function results are cast explicitly so sqlc does not infer their types.

-- name: ExportVendorPksBatch :many
-- COALESCE keeps validity/remaining/status non-null for sqlc/pgx (a NULL scan into int32 fails).
SELECT v.id,
       v.code                                                        AS vendor_code,
       v.name                                                        AS vendor_name,
       d.name                                                        AS director_name,
       d.phone                                                       AS director_phone,
       d.email                                                       AS director_email,
       t.code                                                        AS pks_type_code,
       ap.wod_number                                                 AS wod_number,
       ap.pks_name                                                   AS pks_name,
       ap.effective_date                                             AS effective_date,
       ap.expiry_date                                                AS expiry_date,
       COALESCE(vendor_pks_validity_months(ap.effective_date, ap.expiry_date), 0)::int AS validity_months,
       COALESCE(ap.expiry_date - sqlc.arg('today')::date, 0)::int               AS remaining_days,
       COALESCE(vendor_pks_status(v.kind, ap.expiry_date, ap.terminated_at,
                         sqlc.arg('today')::date, sqlc.arg('renewal_months')::int), '')::text AS pks_status
FROM vendor_pks ap
JOIN vendors v ON v.id = ap.vendor_id
JOIN vendor_pks_types t ON t.id = ap.pks_type_id
LEFT JOIN vendor_pics d ON d.vendor_id = v.id AND d.is_director AND d.deleted_at IS NULL
WHERE ap.replaced_at IS NULL
  AND (v.code, t.code) > (sqlc.arg('after_vendor_code')::text, sqlc.arg('after_type_code')::text)
  AND (sqlc.narg('q')::text IS NULL
        OR v.code ILIKE '%' || sqlc.narg('q')::text || '%'
        OR v.name ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND v.deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND v.deleted_at IS NOT NULL)
      )
  AND (sqlc.narg('pks_type')::text IS NULL OR t.code = sqlc.narg('pks_type')::text)
  AND (sqlc.narg('pks_status')::text IS NULL
        OR vendor_pks_status(v.kind, ap.expiry_date, ap.terminated_at,
                             sqlc.arg('today')::date, sqlc.arg('renewal_months')::int) = sqlc.narg('pks_status')::text)
ORDER BY v.code, t.code
LIMIT sqlc.arg('batch_size')::int;

-- name: CountVendorPksReport :one
-- Same joins and filters as ExportVendorPksBatch, without keyset/LIMIT.
SELECT COUNT(*)
FROM vendor_pks ap
JOIN vendors v ON v.id = ap.vendor_id
JOIN vendor_pks_types t ON t.id = ap.pks_type_id
WHERE ap.replaced_at IS NULL
  AND (sqlc.narg('q')::text IS NULL
        OR v.code ILIKE '%' || sqlc.narg('q')::text || '%'
        OR v.name ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND v.deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND v.deleted_at IS NOT NULL)
      )
  AND (sqlc.narg('pks_type')::text IS NULL OR t.code = sqlc.narg('pks_type')::text)
  AND (sqlc.narg('pks_status')::text IS NULL
        OR vendor_pks_status(v.kind, ap.expiry_date, ap.terminated_at,
                             sqlc.arg('today')::date, sqlc.arg('renewal_months')::int) = sqlc.narg('pks_status')::text);

-- name: ExportVendorPksMissingBatch :many
-- belum_diisi: FLM_VENDOR without an active PKS of type `pks_type` (required here). PKS cells are empty in the CSV.
-- Keyset on vendor code only (one row per vendor).
SELECT v.id,
       v.code                                                        AS vendor_code,
       v.name                                                        AS vendor_name,
       d.name                                                        AS director_name,
       d.phone                                                       AS director_phone,
       d.email                                                       AS director_email,
       t.code                                                        AS pks_type_code
FROM vendors v
JOIN vendor_pks_types t ON t.code = sqlc.arg('pks_type')::text
LEFT JOIN vendor_pics d ON d.vendor_id = v.id AND d.is_director AND d.deleted_at IS NULL
WHERE v.kind = 'FLM_VENDOR'
  AND NOT EXISTS (SELECT 1 FROM vendor_pks ap
                  WHERE ap.vendor_id = v.id AND ap.pks_type_id = t.id AND ap.replaced_at IS NULL)
  AND v.code > sqlc.arg('after_vendor_code')::text
  AND (sqlc.narg('q')::text IS NULL
        OR v.code ILIKE '%' || sqlc.narg('q')::text || '%'
        OR v.name ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND v.deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND v.deleted_at IS NOT NULL)
      )
ORDER BY v.code
LIMIT sqlc.arg('batch_size')::int;

-- name: CountVendorPksMissing :one
-- Same joins and filters as ExportVendorPksMissingBatch, without keyset/LIMIT.
SELECT COUNT(*)
FROM vendors v
JOIN vendor_pks_types t ON t.code = sqlc.arg('pks_type')::text
WHERE v.kind = 'FLM_VENDOR'
  AND NOT EXISTS (SELECT 1 FROM vendor_pks ap
                  WHERE ap.vendor_id = v.id AND ap.pks_type_id = t.id AND ap.replaced_at IS NULL)
  AND (sqlc.narg('q')::text IS NULL
        OR v.code ILIKE '%' || sqlc.narg('q')::text || '%'
        OR v.name ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND v.deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND v.deleted_at IS NOT NULL)
      );
