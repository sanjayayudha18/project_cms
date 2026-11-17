-- Admin area cash count + region links (ScDD 5.1, AC-5.1-08). Mutations run only inside the
-- applier (apply-on-approve tx). Areas soft-disable; cash_count_area_regions is a pure link table
-- (rewritten by the applier: delete the area's links, then insert the new set).
-- region_code = vendor_branches.region_code (not regions).

-- name: ListCashCountAreasAdmin :many
SELECT a.id, a.code, a.name, a.is_active, a.deleted_at,
       COALESCE((SELECT array_agg(r.region_code ORDER BY r.region_code)
                 FROM cash_count_area_regions r WHERE r.area_id = a.id), '{}')::text[] AS region_codes
FROM cash_count_areas a
WHERE sqlc.arg('status')::text = 'all'
   OR (sqlc.arg('status')::text = 'active' AND a.is_active)
   OR (sqlc.arg('status')::text = 'disabled' AND NOT a.is_active)
ORDER BY a.code, a.id;

-- name: GetCashCountAreaAdminByID :one
-- Includes disabled rows + region list; doubles as the "before" snapshot / CurrentState.
SELECT a.id, a.code, a.name, a.is_active, a.deleted_at,
       COALESCE((SELECT array_agg(r.region_code ORDER BY r.region_code)
                 FROM cash_count_area_regions r WHERE r.area_id = a.id), '{}')::text[] AS region_codes
FROM cash_count_areas a WHERE a.id = $1;

-- name: FindCashCountAreaByCode :one
-- Uniqueness pre-check. code is UNIQUE over ALL rows (disabled included), so no is_active filter.
SELECT id FROM cash_count_areas WHERE code = sqlc.arg('code')::text LIMIT 1;

-- name: ListCashCountRegionConflicts :many
-- Pre-check: which of the given region_codes already belong to ANOTHER ACTIVE area
-- (exclude_area_id = 0 on create).
SELECT r.region_code, r.area_id, a.code AS area_code
FROM cash_count_area_regions r JOIN cash_count_areas a ON a.id = r.area_id
WHERE r.region_code = ANY (sqlc.arg('region_codes')::text[])
  AND a.is_active
  AND r.area_id <> sqlc.arg('exclude_area_id')::bigint
ORDER BY r.region_code;

-- name: ListUnknownCashCountRegionCodes :many
-- Pre-check: given region_codes that no ACTIVE vendor_branches row uses (L648).
SELECT c.region_code::text AS region_code
FROM unnest(sqlc.arg('region_codes')::text[]) AS c(region_code)
WHERE NOT EXISTS (
        SELECT 1 FROM vendor_branches b WHERE b.region_code = c.region_code AND b.is_active)
ORDER BY c.region_code;

-- name: CountActiveCashCountExecutorsInArea :one
-- Pre-check for disable: active EXECUTOR PICs still mapped to the area.
SELECT count(*) FROM cash_count_pics
WHERE area_id = $1 AND pic_role = 'EXECUTOR' AND is_active;

-- name: CreateCashCountArea :one
INSERT INTO cash_count_areas (code, name)
VALUES (sqlc.arg('code'), sqlc.arg('name'))
RETURNING id, code, name, is_active, deleted_at;

-- name: UpdateCashCountArea :one
-- code is immutable. Active rows only.
UPDATE cash_count_areas SET name = sqlc.arg('name')
WHERE id = sqlc.arg('id') AND is_active
RETURNING id, code, name, is_active, deleted_at;

-- name: DisableCashCountArea :execrows
-- Guarded: 0 rows affected when an active EXECUTOR PIC still uses the area => applier 409.
UPDATE cash_count_areas a SET is_active = false, deleted_at = now()
WHERE a.id = $1
  AND NOT EXISTS (SELECT 1 FROM cash_count_pics p
                  WHERE p.area_id = a.id AND p.pic_role = 'EXECUTOR' AND p.is_active);

-- name: EnableCashCountArea :exec
UPDATE cash_count_areas SET is_active = true, deleted_at = NULL WHERE id = $1;

-- name: DeleteCashCountAreaRegions :exec
-- Link table (no soft-delete): applier clears the area's links, then re-inserts the new set.
DELETE FROM cash_count_area_regions WHERE area_id = $1;

-- name: InsertCashCountAreaRegion :execrows
-- Guarded: inserts only while the area is active and an active vendor_branches row still uses the
-- region_code (0 affected => applier 409). PK(region_code) rejects a region owned by another area.
INSERT INTO cash_count_area_regions (region_code, area_id, created_by)
SELECT sqlc.arg('region_code')::text, a.id, sqlc.arg('created_by')::bigint
FROM cash_count_areas a
WHERE a.id = sqlc.arg('area_id') AND a.is_active
  AND EXISTS (SELECT 1 FROM vendor_branches b WHERE b.region_code = sqlc.arg('region_code')::text AND b.is_active);
