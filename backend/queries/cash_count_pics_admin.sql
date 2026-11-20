-- Admin mapping PIC cash count (ScDD 5.1, AC-5.1-09/10). COORDINATOR = user per region_code,
-- EXECUTOR = user per area. The user guard (active, auth_source 'ldap', role CC-COORDINATOR /
-- CC-EXECUTOR) and the region/area guard are re-applied inside the INSERT/UPDATE: state can change
-- between submit and approve (no row => applier error => 409). Mutations run only inside the applier.

-- name: ListCashCountPicsAdmin :many
-- pic_role = '' means both roles.
SELECT p.id, p.pic_role, p.user_id, u.username, u.full_name, p.region_code, p.area_id,
       a.code AS area_code, a.name AS area_name, p.is_active, p.deleted_at
FROM cash_count_pics p
JOIN users u ON u.id = p.user_id
LEFT JOIN cash_count_areas a ON a.id = p.area_id
WHERE (sqlc.arg('pic_role')::text = '' OR p.pic_role = sqlc.arg('pic_role')::text)
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND p.is_active)
        OR (sqlc.arg('status')::text = 'disabled' AND NOT p.is_active)
      )
ORDER BY p.pic_role, COALESCE(p.region_code, a.code), u.username, p.id;

-- name: GetCashCountPicAdminByID :one
-- Includes disabled rows; doubles as the "before" snapshot / CurrentState.
SELECT id, pic_role, user_id, region_code, area_id, is_active, deleted_at
FROM cash_count_pics WHERE id = $1;

-- name: GetCashCountPicEligibleUser :one
-- Submit pre-check: user is active, ldap and holds the role required for pic_role
-- (COORDINATOR -> CC-COORDINATOR, EXECUTOR -> CC-EXECUTOR). No row = not eligible.
SELECT u.id
FROM users u JOIN roles r ON r.id = u.role_id
WHERE u.id = sqlc.arg('user_id') AND u.is_active AND u.deleted_at IS NULL AND u.auth_source = 'ldap'
  AND r.role = CASE sqlc.arg('pic_role')::text WHEN 'COORDINATOR' THEN 'CC-COORDINATOR' ELSE 'CC-EXECUTOR' END;

-- name: CashCountRegionCodeInUse :one
-- Submit pre-check: some active vendor_branches row uses this region_code.
SELECT EXISTS (SELECT 1 FROM vendor_branches b WHERE b.region_code = sqlc.arg('region_code')::text AND b.is_active) AS in_use;

-- name: GetActiveCashCountAreaID :one
-- Submit pre-check: area exists and is active. No row = missing or disabled.
SELECT id FROM cash_count_areas WHERE id = $1 AND is_active;

-- name: FindActiveCashCountPic :one
-- Uniqueness pre-check: active mapping with the same (pic_role, user, region/area), excluding
-- exclude_id (0 = none). region_code '' / area_id 0 when not applicable.
SELECT id FROM cash_count_pics
WHERE is_active
  AND pic_role = sqlc.arg('pic_role')::text
  AND user_id = sqlc.arg('user_id')::bigint
  AND COALESCE(region_code, '') = sqlc.arg('region_code')::text
  AND COALESCE(area_id, 0) = sqlc.arg('area_id')::bigint
  AND id <> sqlc.arg('exclude_id')::bigint
LIMIT 1;

-- name: CreateCashCountPic :one
-- Guarded INSERT ... SELECT: user eligible + target (region in use / area active) at apply time.
INSERT INTO cash_count_pics (pic_role, user_id, region_code, area_id)
SELECT p.pic_role, u.id, p.region_code, p.area_id
FROM (SELECT sqlc.arg('pic_role')::text AS pic_role,
             sqlc.narg('region_code')::text AS region_code,
             sqlc.narg('area_id')::bigint AS area_id) p
JOIN users u ON u.id = sqlc.arg('user_id') AND u.is_active AND u.deleted_at IS NULL AND u.auth_source = 'ldap'
JOIN roles r ON r.id = u.role_id
             AND r.role = CASE p.pic_role WHEN 'COORDINATOR' THEN 'CC-COORDINATOR' ELSE 'CC-EXECUTOR' END
WHERE (p.pic_role = 'COORDINATOR'
       AND EXISTS (SELECT 1 FROM vendor_branches b WHERE b.region_code = p.region_code AND b.is_active))
   OR (p.pic_role = 'EXECUTOR'
       AND EXISTS (SELECT 1 FROM cash_count_areas a WHERE a.id = p.area_id AND a.is_active))
RETURNING id, pic_role, user_id, region_code, area_id, is_active, deleted_at;

-- name: UpdateCashCountPic :one
-- pic_role is immutable; user and target (region for COORDINATOR, area for EXECUTOR) editable.
-- Same apply-time guards as create. Active rows only.
UPDATE cash_count_pics m
SET user_id = sqlc.arg('user_id'),
    region_code = sqlc.narg('region_code'),
    area_id = sqlc.narg('area_id')
WHERE m.id = sqlc.arg('id') AND m.is_active
  AND EXISTS (SELECT 1 FROM users u JOIN roles r ON r.id = u.role_id
              WHERE u.id = sqlc.arg('user_id') AND u.is_active AND u.deleted_at IS NULL AND u.auth_source = 'ldap'
                AND r.role = CASE m.pic_role WHEN 'COORDINATOR' THEN 'CC-COORDINATOR' ELSE 'CC-EXECUTOR' END)
  AND ((m.pic_role = 'COORDINATOR'
        AND EXISTS (SELECT 1 FROM vendor_branches b WHERE b.region_code = sqlc.narg('region_code')::text AND b.is_active))
    OR (m.pic_role = 'EXECUTOR'
        AND EXISTS (SELECT 1 FROM cash_count_areas a WHERE a.id = sqlc.narg('area_id')::bigint AND a.is_active)))
RETURNING id, pic_role, user_id, region_code, area_id, is_active, deleted_at;

-- name: DisableCashCountPic :exec
-- Soft-disable only ("hapus" = nonaktif).
UPDATE cash_count_pics SET is_active = false, deleted_at = now() WHERE id = $1;

-- name: EnableCashCountPic :execrows
-- Guarded like create: user must still be eligible and the target still valid (0 affected =>
-- applier 409). The active-unique index rejects a duplicate mapping created meanwhile.
UPDATE cash_count_pics m SET is_active = true, deleted_at = NULL
WHERE m.id = $1
  AND EXISTS (SELECT 1 FROM users u JOIN roles r ON r.id = u.role_id
              WHERE u.id = m.user_id AND u.is_active AND u.deleted_at IS NULL AND u.auth_source = 'ldap'
                AND r.role = CASE m.pic_role WHEN 'COORDINATOR' THEN 'CC-COORDINATOR' ELSE 'CC-EXECUTOR' END)
  AND ((m.pic_role = 'COORDINATOR'
        AND EXISTS (SELECT 1 FROM vendor_branches b WHERE b.region_code = m.region_code AND b.is_active))
    OR (m.pic_role = 'EXECUTOR'
        AND EXISTS (SELECT 1 FROM cash_count_areas a WHERE a.id = m.area_id AND a.is_active)));
