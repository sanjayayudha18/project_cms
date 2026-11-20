-- Vendor export with Direktur Utama (spec vendor-export-pks, task 3.1; PKS join dropped by vendor-pks-type task 2.4,
-- PKS is exported by the vendor_pks report instead). Read-only.
-- backend-exq only (NOT shared): ExportVendorsBatch stays untouched because the
-- importer in backend/ reads it for the current-state comparison.

-- name: ExportVendorsWithPksBatch :many
-- Vendor columns + predicates: COPY of ExportVendorsBatch (backend/queries/master_data_export.sql);
-- keep in sync, Property 2 checks it on a real DB. Director join hits partial unique index
-- vendor_pics_director_uq, so it never adds rows and CountVendors stays the row count.
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
       d.email                              AS director_email
FROM vendors v
LEFT JOIN vendor_pics d ON d.vendor_id = v.id AND d.is_director AND d.deleted_at IS NULL
WHERE v.id > sqlc.arg('after_id')::bigint
  AND (sqlc.narg('vendor_id')::bigint IS NULL OR v.id = sqlc.narg('vendor_id')::bigint)
  AND (sqlc.arg('status')::text = 'all'
       OR (sqlc.arg('status')::text = 'active' AND v.deleted_at IS NULL)
       OR (sqlc.arg('status')::text = 'disabled' AND v.deleted_at IS NOT NULL))
ORDER BY v.id
LIMIT sqlc.arg('batch_size')::int;
