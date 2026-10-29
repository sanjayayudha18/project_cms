-- Export-only entities of vendor-data-export (design "CSV entitas export-only").
-- Keyset pages like the shared master_data_export.sql: id > after_id ORDER BY id.
-- Cells are plain text: NULL -> '', numeric/date ::text (never through float).

-- name: ExportDsrLocationMapsBatch :many
-- vendor_id scopes m.vendor_id; branch_id scopes the mapped vault's cabang.
-- status active = m.deleted_at IS NULL.
SELECT m.id,
       v.code                          AS vendor_code,
       b.branch_code,
       vl.vault_code,
       m.dsr_location,
       m.is_active
FROM dsr_location_vault_maps m
JOIN vendors v         ON v.id = m.vendor_id
JOIN vendor_vaults vl  ON vl.id = m.vendor_vault_id
JOIN vendor_branches b ON b.id = vl.vendor_branch_id
WHERE m.id > sqlc.arg('after_id')::bigint
  AND (sqlc.narg('vendor_id')::bigint IS NULL OR m.vendor_id = sqlc.narg('vendor_id')::bigint)
  AND (sqlc.narg('branch_id')::bigint IS NULL OR vl.vendor_branch_id = sqlc.narg('branch_id')::bigint)
  AND (sqlc.arg('status')::text = 'all'
       OR (sqlc.arg('status')::text = 'active' AND m.deleted_at IS NULL)
       OR (sqlc.arg('status')::text = 'disabled' AND m.deleted_at IS NOT NULL))
ORDER BY m.id
LIMIT sqlc.arg('batch_size')::int;

-- name: ExportPackagePricesBatch :many
-- vendor_id required. branch_id (optional) = vpp.vendor_branch_id.
-- price_level 'vendor'|'branch'|'atm'|'all'. status by effective_end_date vs as_of
-- (WIB date from the caller): active = open-ended or ends on/after as_of.
SELECT vpp.id,
       v.code                                         AS vendor_code,
       (CASE WHEN vpp.atm_id IS NOT NULL THEN 'atm'
             WHEN vpp.vendor_branch_id IS NOT NULL THEN 'branch'
             ELSE 'vendor' END)::text                 AS price_level,
       COALESCE(b.branch_code, '')::text              AS branch_code,
       COALESCE(a.terminal_id, '')::text              AS terminal_id,
       vpp.package,
       vpp.package_code,
       vpp.machine_group,
       vpp.price_class,
       vpp.tier_min::text                             AS tier_min,
       COALESCE(vpp.tier_max::text, '')::text         AS tier_max,
       COALESCE(vpp.base_price::text, '')::text       AS base_price,
       vpp.currency::text                             AS currency,
       COALESCE(vpp.sla_note, '')::text               AS sla_note,
       vpp.effective_start_date::text                 AS effective_start_date,
       COALESCE(vpp.effective_end_date::text, '')::text AS effective_end_date
FROM vendor_package_prices vpp
JOIN vendors v ON v.id = vpp.vendor_id
LEFT JOIN vendor_branches b ON b.id = vpp.vendor_branch_id
LEFT JOIN atms a ON a.id = vpp.atm_id
WHERE vpp.id > sqlc.arg('after_id')::bigint
  AND vpp.vendor_id = sqlc.arg('vendor_id')::bigint
  AND (sqlc.narg('branch_id')::bigint IS NULL OR vpp.vendor_branch_id = sqlc.narg('branch_id')::bigint)
  AND (sqlc.arg('price_level')::text = 'all'
       OR (sqlc.arg('price_level')::text = 'atm' AND vpp.atm_id IS NOT NULL)
       OR (sqlc.arg('price_level')::text = 'branch' AND vpp.vendor_branch_id IS NOT NULL)
       OR (sqlc.arg('price_level')::text = 'vendor' AND vpp.atm_id IS NULL AND vpp.vendor_branch_id IS NULL))
  AND (sqlc.arg('status')::text = 'all'
       OR (sqlc.arg('status')::text = 'active'
           AND (vpp.effective_end_date IS NULL OR vpp.effective_end_date >= sqlc.arg('as_of')::date))
       OR (sqlc.arg('status')::text = 'disabled'
           AND vpp.effective_end_date IS NOT NULL AND vpp.effective_end_date < sqlc.arg('as_of')::date))
ORDER BY vpp.id
LIMIT sqlc.arg('batch_size')::int;
