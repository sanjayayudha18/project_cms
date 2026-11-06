-- Vendor + PKS report export (vendor-pks task 4.3, Req 10). Read-only.
-- SHARED FILE: backend-exq generates these queries too (its sqlc.yaml lists
-- this path). Regenerate in BOTH modules.
--
-- Predicates q / status are copied verbatim from ListVendorsAdmin
-- (vendors_admin.sql); keep the two in sync. `status` is 'active' | 'disabled'
-- | 'all' (caller resolves absent to 'active'). `pks_status` NULL = no filter;
-- otherwise one key of reports.PksStatuses. Vendor INTERNAL has status NULL, so
-- it never matches a pks_status filter. `today` = Asia/Jakarta date from Go
-- (D30), `renewal_months` = VENDOR_PKS_RENEWAL_MONTHS.
--
-- Keyset page on (name, id): first page after_name = '' and after_id = 0.
-- Director = the vendor's single active vendor_pics.is_director row (partial
-- unique index vendor_pics_director_uq), so the LEFT JOIN never multiplies rows.
-- Active PKS = vendor_pks.replaced_at IS NULL (vendor_pks_active_uq).
-- Function results are cast explicitly so sqlc does not infer their types.

-- name: ExportVendorPksBatch :many
-- COALESCE keeps these non-null for sqlc/pgx (a NULL scan into int32 fails): 0/'' means
-- "no active PKS" (pks_id IS NULL) or, for pks_status, an INTERNAL vendor. Filter below uses the raw function.
SELECT v.id,
       v.code                                                        AS vendor_code,
       v.name                                                        AS vendor_name,
       d.name                                                        AS director_name,
       d.phone                                                       AS director_phone,
       d.email                                                       AS director_email,
       ap.pks_name                                                   AS pks_name,
       ap.effective_date                                             AS effective_date,
       ap.expiry_date                                                AS expiry_date,
       COALESCE(vendor_pks_validity_months(ap.effective_date, ap.expiry_date), 0)::int AS validity_months,
       COALESCE(ap.expiry_date - sqlc.arg('today')::date, 0)::int               AS remaining_days,
       COALESCE(vendor_pks_status(v.kind, ap.expiry_date, ap.terminated_at,
                         sqlc.arg('today')::date, sqlc.arg('renewal_months')::int), '')::text AS pks_status
FROM vendors v
LEFT JOIN vendor_pks ap ON ap.vendor_id = v.id AND ap.replaced_at IS NULL
LEFT JOIN vendor_pics d ON d.vendor_id = v.id AND d.is_director AND d.deleted_at IS NULL
WHERE (v.name, v.id) > (sqlc.arg('after_name')::text, sqlc.arg('after_id')::bigint)
  AND (sqlc.narg('q')::text IS NULL
        OR v.code ILIKE '%' || sqlc.narg('q')::text || '%'
        OR v.name ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND v.deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND v.deleted_at IS NOT NULL)
      )
  AND (sqlc.narg('pks_status')::text IS NULL
        OR vendor_pks_status(v.kind, ap.expiry_date, ap.terminated_at,
                             sqlc.arg('today')::date, sqlc.arg('renewal_months')::int) = sqlc.narg('pks_status')::text)
ORDER BY v.name, v.id
LIMIT sqlc.arg('batch_size')::int;

-- name: CountVendorPksReport :one
-- Same joins and filters as ExportVendorPksBatch, without keyset/LIMIT.
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
        OR vendor_pks_status(v.kind, ap.expiry_date, ap.terminated_at,
                             sqlc.arg('today')::date, sqlc.arg('renewal_months')::int) = sqlc.narg('pks_status')::text);
