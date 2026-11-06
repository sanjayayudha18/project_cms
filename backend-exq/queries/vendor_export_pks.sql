-- Vendor export with Direktur Utama + active PKS (spec vendor-export-pks, task 3.1). Read-only.
-- backend-exq only (NOT shared): ExportVendorsBatch stays untouched because the
-- importer in backend/ reads it for the current-state comparison.

-- name: ExportVendorsWithPksBatch :many
-- Vendor columns + predicates: COPY of ExportVendorsBatch (backend/queries/master_data_export.sql);
-- keep in sync, Property 2 checks it on a real DB. Director/PKS joins: COPY of ExportVendorPksBatch
-- (backend/queries/vendor_pks_report.sql). Both joins hit partial unique indexes (vendor_pics_director_uq,
-- vendor_pks_active_uq), so they never add rows and CountVendors stays the row count.
-- COALESCE on function results keeps them non-null for sqlc/pgx: 0/'' = no active PKS or INTERNAL vendor.
SELECT v.id,
       v.code,
       v.name,
       COALESCE(v.legal_name, '')::text     AS legal_name,
       COALESCE(v.npwp, '')::text           AS npwp,
       COALESCE(v.contact_email, '')::text  AS contact_email,
       COALESCE(v.contact_phone, '')::text  AS contact_phone,
       COALESCE(v.hq_address, '')::text     AS hq_address,
       v.is_active,
       d.name                               AS director_name,
       d.phone                              AS director_phone,
       d.email                              AS director_email,
       ap.pks_name                          AS pks_name,
       ap.effective_date                    AS effective_date,
       ap.expiry_date                       AS expiry_date,
       COALESCE(vendor_pks_validity_months(ap.effective_date, ap.expiry_date), 0)::int AS validity_months,
       COALESCE(ap.expiry_date - sqlc.arg('today')::date, 0)::int                      AS remaining_days,
       COALESCE(vendor_pks_status(v.kind, ap.expiry_date, ap.terminated_at,
                sqlc.arg('today')::date, sqlc.arg('renewal_months')::int), '')::text   AS pks_status
FROM vendors v
LEFT JOIN vendor_pks ap ON ap.vendor_id = v.id AND ap.replaced_at IS NULL
LEFT JOIN vendor_pics d ON d.vendor_id = v.id AND d.is_director AND d.deleted_at IS NULL
WHERE v.id > sqlc.arg('after_id')::bigint
  AND (sqlc.narg('vendor_id')::bigint IS NULL OR v.id = sqlc.narg('vendor_id')::bigint)
  AND (sqlc.arg('status')::text = 'all'
       OR (sqlc.arg('status')::text = 'active' AND v.deleted_at IS NULL)
       OR (sqlc.arg('status')::text = 'disabled' AND v.deleted_at IS NOT NULL))
ORDER BY v.id
LIMIT sqlc.arg('batch_size')::int;
