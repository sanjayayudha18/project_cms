-- Master-data CSV export (plan.md Fase 5, T5.1). Read-only. Each query is one
-- keyset page: `WHERE id > after_id ORDER BY id LIMIT batch_size`, so the
-- exporter streams any table size with bounded memory (no OFFSET rescans).
-- `status` is 'active' | 'disabled' | 'all' (caller resolves absent to
-- 'active'), same meaning as the admin list endpoints.
--
-- Every column comes back as a plain string/bool/int so the CSV shaping needs
-- no NULL handling: NULL -> '' (an empty CSV cell means NULL on import too),
-- numerics and dates are cast to text in SQL so amounts keep their exact
-- stored scale and never pass through float.
--
-- SHARED FILE: backend-exq generates these queries too (its sqlc.yaml lists
-- this path), so the export (backend-exq) and the importer's current-state
-- comparison (backend/) format cells from the same SQL (vendor-data-export,
-- design "Kenapa SQL juga dibagi"). Optional scope `vendor_id` / `branch_id`:
-- NULL = every row (global export, importer). Regenerate in BOTH modules.

-- name: ExportVendorsBatch :many
SELECT v.id,
       v.code,
       v.name,
       COALESCE(v.legal_name, '')::text     AS legal_name,
       COALESCE(v.npwp, '')::text           AS npwp,
       COALESCE(v.contact_email, '')::text  AS contact_email,
       COALESCE(v.contact_phone, '')::text  AS contact_phone,
       COALESCE(v.hq_address, '')::text     AS hq_address,
       v.is_active
FROM vendors v
WHERE v.id > sqlc.arg('after_id')::bigint
  AND (sqlc.narg('vendor_id')::bigint IS NULL OR v.id = sqlc.narg('vendor_id')::bigint)
  AND (sqlc.arg('status')::text = 'all'
       OR (sqlc.arg('status')::text = 'active' AND v.deleted_at IS NULL)
       OR (sqlc.arg('status')::text = 'disabled' AND v.deleted_at IS NOT NULL))
ORDER BY v.id
LIMIT sqlc.arg('batch_size')::int;

-- name: ExportVendorBranchesBatch :many
SELECT b.id,
       v.code                               AS vendor_code,
       b.branch_code,
       b.branch_name,
       COALESCE(b.region, '')::text         AS region,
       COALESCE(b.region_code, '')::text    AS region_code,
       b.category,
       COALESCE(b.location_id::text, '')::text AS location_id,
       b.is_active
FROM vendor_branches b
JOIN vendors v ON v.id = b.vendor_id
WHERE b.id > sqlc.arg('after_id')::bigint
  AND (sqlc.narg('vendor_id')::bigint IS NULL OR b.vendor_id = sqlc.narg('vendor_id')::bigint)
  AND (sqlc.arg('status')::text = 'all'
       OR (sqlc.arg('status')::text = 'active' AND b.deleted_at IS NULL)
       OR (sqlc.arg('status')::text = 'disabled' AND b.deleted_at IS NOT NULL))
ORDER BY b.id
LIMIT sqlc.arg('batch_size')::int;

-- name: ExportVendorVaultsBatch :many
SELECT vl.id,
       v.code                                       AS vendor_code,
       b.branch_code,
       vl.vault_code,
       COALESCE(vl.category, '')::text              AS category,
       COALESCE(vl.currency_code, '')::text         AS currency_code,
       COALESCE(vl.min_capacity_amount::text, '')::text AS min_capacity_amount,
       COALESCE(vl.max_capacity_amount::text, '')::text AS max_capacity_amount,
       COALESCE(vl.latitude::text, '')::text        AS latitude,
       COALESCE(vl.longitude::text, '')::text       AS longitude,
       COALESCE(vl.operating_hours, '')::text       AS operating_hours,
       COALESCE(vl.location_id::text, '')::text     AS location_id,
       vl.is_active
FROM vendor_vaults vl
JOIN vendor_branches b ON b.id = vl.vendor_branch_id
JOIN vendors v ON v.id = b.vendor_id
WHERE vl.id > sqlc.arg('after_id')::bigint
  AND (sqlc.narg('vendor_id')::bigint IS NULL OR v.id = sqlc.narg('vendor_id')::bigint)
  AND (sqlc.narg('branch_id')::bigint IS NULL OR b.id = sqlc.narg('branch_id')::bigint)
  AND (sqlc.arg('status')::text = 'all'
       OR (sqlc.arg('status')::text = 'active' AND vl.deleted_at IS NULL)
       OR (sqlc.arg('status')::text = 'disabled' AND vl.deleted_at IS NOT NULL))
ORDER BY vl.id
LIMIT sqlc.arg('batch_size')::int;

-- name: ExportVendorPicsBatch :many
-- branch_code is '' for a vendor-wide PIC (vendor_branch_id NULL). A branch_id
-- scope matches only that branch's PICs, never vendor-wide ones (D3).
SELECT p.id,
       v.code                                       AS vendor_code,
       COALESCE(b.branch_code, '')::text            AS branch_code,
       p.name,
       COALESCE(p."position", '')::text             AS position,
       COALESCE(p.phone, '')::text                  AS phone,
       COALESCE(p.email, '')::text                  AS email,
       p.is_notification_recipient,
       p.is_active,
       p.roles
FROM vendor_pics p
JOIN vendors v ON v.id = p.vendor_id
LEFT JOIN vendor_branches b ON b.id = p.vendor_branch_id
WHERE p.id > sqlc.arg('after_id')::bigint
  AND (sqlc.narg('vendor_id')::bigint IS NULL OR p.vendor_id = sqlc.narg('vendor_id')::bigint)
  AND (sqlc.narg('branch_id')::bigint IS NULL OR p.vendor_branch_id = sqlc.narg('branch_id')::bigint)
  AND (sqlc.arg('status')::text = 'all'
       OR (sqlc.arg('status')::text = 'active' AND p.deleted_at IS NULL)
       OR (sqlc.arg('status')::text = 'disabled' AND p.deleted_at IS NOT NULL))
ORDER BY p.id
LIMIT sqlc.arg('batch_size')::int;

-- name: ExportATMsBatch :many
SELECT a.id,
       a.terminal_id,
       a.location_id,
       a.machine_type,
       a.brand,
       a.model,
       a.operation_hours,
       a.deployment_type,
       COALESCE(a.capacity_amount::text, '')::text           AS capacity_amount,
       COALESCE(a.low_threshold_amount::text, '')::text      AS low_threshold_amount,
       COALESCE(a.critical_threshold_amount::text, '')::text AS critical_threshold_amount,
       a.blacklisted,
       COALESCE(a.escrow_account, '')::text                  AS escrow_account,
       COALESCE(a.priority_class, '')::text                  AS priority_class,
       a.is_active
FROM atms a
WHERE a.id > sqlc.arg('after_id')::bigint
  AND (sqlc.arg('status')::text = 'all'
       OR (sqlc.arg('status')::text = 'active' AND a.deleted_at IS NULL)
       OR (sqlc.arg('status')::text = 'disabled' AND a.deleted_at IS NOT NULL))
ORDER BY a.id
LIMIT sqlc.arg('batch_size')::int;

-- name: ExportATMAssignmentsBatch :many
-- Kelolaan ATM: atm_vendor_packages rows keyed by terminal_id + package. Two
-- sources (migration 023): package_source 'branch' = a vendor_packages_branch
-- row (package_code = its code), 'vendor' = a vendor-wide label (package_code =
-- the label). vendor_code/branch_code are always the managing branch. This table
-- has no deleted_at; is_active is the disable flag, so status filters on it.
SELECT av.id,
       a.terminal_id,
       v.code                                  AS vendor_code,
       b.branch_code,
       CASE WHEN av.vendor_package_id IS NULL THEN 'vendor' ELSE 'branch' END::text AS package_source,
       COALESCE(p.package_code, av.package)::text AS package_code,
       av.effective_start_date::text           AS effective_start_date,
       COALESCE(av.effective_end_date::text, '')::text AS effective_end_date,
       av.is_active
FROM atm_vendor_packages av
JOIN atms a ON a.id = av.atm_id
LEFT JOIN vendor_packages_branch p ON p.id = av.vendor_package_id
JOIN vendor_branches b ON b.id = COALESCE(p.vendor_branch_id, av.vendor_branch_id)
JOIN vendors v ON v.id = b.vendor_id
WHERE av.id > sqlc.arg('after_id')::bigint
  AND (sqlc.arg('status')::text = 'all'
       OR (sqlc.arg('status')::text = 'active' AND av.is_active)
       OR (sqlc.arg('status')::text = 'disabled' AND NOT av.is_active))
ORDER BY av.id
LIMIT sqlc.arg('batch_size')::int;
