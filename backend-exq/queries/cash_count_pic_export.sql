-- Mapping PIC cash count CSV export (ScDD 5.1, AC-5.1-12). Read-only, keyset page by id like
-- master_data_export.sql. Columns = upload columns pic_role,username,region_code,area_code:
-- COORDINATOR rows have area_code '', EXECUTOR rows have region_code ''. `status`: active | disabled | all.

-- name: ExportCashCountPicsBatch :many
SELECT p.id,
       p.pic_role,
       u.username,
       COALESCE(p.region_code, '')::text AS region_code,
       COALESCE(a.code, '')::text        AS area_code
FROM cash_count_pics p
JOIN users u ON u.id = p.user_id
LEFT JOIN cash_count_areas a ON a.id = p.area_id
WHERE p.id > sqlc.arg('after_id')::bigint
  AND (sqlc.arg('status')::text = 'all'
       OR (sqlc.arg('status')::text = 'active' AND p.is_active)
       OR (sqlc.arg('status')::text = 'disabled' AND NOT p.is_active))
ORDER BY p.id
LIMIT sqlc.arg('batch_size')::int;

-- name: CountCashCountPics :one
SELECT count(*)::bigint
FROM cash_count_pics p
WHERE sqlc.arg('status')::text = 'all'
   OR (sqlc.arg('status')::text = 'active' AND p.is_active)
   OR (sqlc.arg('status')::text = 'disabled' AND NOT p.is_active);
