-- Admin Region Management (.kiro/specs/region-management, Task 2): sqlc
-- queries for the admin region CRUD endpoints. No new tables -- regions
-- already exists (001_baseline_schema.sql); migration 018 added
-- is_active/deleted_at for soft-disable.
--
-- NOTE on "active locations": the design doc's referential-integrity check
-- (Req 4.2/7.2) assumes a `locations.is_active` column, but `locations` has
-- no such column and no soft-delete concept at all (confirmed against
-- 001_baseline_schema.sql -- this feature's requirements explicitly exclude
-- changing `locations`). Every `locations` row is therefore treated as
-- implicitly active: "dependent active locations" below means "any
-- locations row referencing the region", with no is_active filter.

-- name: ListRegionsAdmin :many
-- Filters: q (ILIKE on code OR region, case-insensitive; region is nullable
-- so a NULL never matches, which is correct) and status (sqlc.arg,
-- all|active|inactive). location_count = dependent locations rows (see NOTE
-- above -- no is_active filter exists on locations). Ordered code ASC, id ASC.
SELECT r.id, r.code, r.region, r.created_at, r.updated_at, r.is_active, r.deleted_at,
       COUNT(l.id) AS location_count
FROM regions r
LEFT JOIN locations l ON l.region_id = r.id
WHERE (sqlc.narg('q')::text IS NULL
        OR r.code ILIKE '%' || sqlc.narg('q')::text || '%'
        OR r.region ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND r.is_active = true)
        OR (sqlc.arg('status')::text = 'inactive' AND r.is_active = false)
      )
GROUP BY r.id
ORDER BY r.code ASC, r.id ASC
LIMIT sqlc.arg('page_limit')::bigint OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountRegionsAdmin :one
-- Same filters as ListRegionsAdmin, no LIMIT/OFFSET/JOIN -- pagination total.
SELECT COUNT(*)
FROM regions r
WHERE (sqlc.narg('q')::text IS NULL
        OR r.code ILIKE '%' || sqlc.narg('q')::text || '%'
        OR r.region ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND r.is_active = true)
        OR (sqlc.arg('status')::text = 'inactive' AND r.is_active = false)
      );

-- name: GetRegionAdminByID :one
-- Includes soft-deleted rows -- 404 pre-checks on update/disable/enable are
-- decided by the service, same convention as GetATMAdminByID.
SELECT r.id, r.code, r.region, r.created_at, r.updated_at, r.is_active, r.deleted_at,
       COUNT(l.id) AS location_count
FROM regions r
LEFT JOIN locations l ON l.region_id = r.id
WHERE r.id = $1
GROUP BY r.id;

-- name: FindRegionByCode :one
-- Uniqueness pre-check (incl. soft-deleted) for create (Req 2.2, 2.7).
SELECT id FROM regions WHERE code = $1;

-- name: CreateRegionAdmin :one
-- code is stored pre-normalized (trim+uppercase) by the service; is_active
-- defaults to true (migration 018).
INSERT INTO regions (code, region)
VALUES (sqlc.arg('code'), sqlc.arg('region'))
RETURNING id, code, region, created_at, updated_at, is_active, deleted_at;

-- name: UpdateRegionName :one
-- Display name only -- code is immutable after create (Req 3.1/3.2).
-- updated_at is set by the existing trg_regions_set_updated_at trigger, not
-- here.
UPDATE regions
SET region = sqlc.arg('region')
WHERE id = sqlc.arg('id')
RETURNING id, code, region, created_at, updated_at, is_active, deleted_at;

-- name: DisableRegionAdmin :one
-- Soft-disable: is_active=false, deleted_at=now(). WHERE guard makes it
-- idempotent-safe (0 rows affected if already inactive); the service still
-- pre-checks status to return a clear 409 (Req 4.7).
UPDATE regions
SET is_active = false, deleted_at = now()
WHERE id = $1 AND is_active = true
RETURNING id, code, region, created_at, updated_at, is_active, deleted_at;

-- name: EnableRegionAdmin :one
-- Reverses DisableRegionAdmin: is_active=true, deleted_at=NULL.
UPDATE regions
SET is_active = true, deleted_at = NULL
WHERE id = $1 AND is_active = false
RETURNING id, code, region, created_at, updated_at, is_active, deleted_at;

-- name: CountActiveLocationsByRegion :one
-- Referential-integrity check backing Disable (Req 4.2/7.2). See NOTE above:
-- locations has no is_active column, so this counts every locations row
-- referencing the region (all are implicitly active).
SELECT COUNT(*) FROM locations WHERE region_id = $1;
