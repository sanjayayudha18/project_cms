-- Hari libur (replenishment-realisasi FR1, migration 031). ADMIN, langsung berlaku + audit dalam tx
-- (deviasi GR#3, pola regions_admin.sql). Soft-disable via is_active/deleted_at, tidak pernah DELETE.

-- name: ListHolidaysAdmin :many
-- Filters: year (narg, NULL = semua) + status all|active|inactive. Urut tanggal.
SELECT h.id, h.holiday_date, h.name, h.is_active, h.created_by, h.created_at, h.updated_at, h.deleted_at
FROM holidays h
WHERE (sqlc.narg('year')::int IS NULL OR EXTRACT(YEAR FROM h.holiday_date)::int = sqlc.narg('year')::int)
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND h.is_active)
        OR (sqlc.arg('status')::text = 'inactive' AND NOT h.is_active)
      )
ORDER BY h.holiday_date ASC, h.id ASC
LIMIT sqlc.arg('page_limit')::bigint OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountHolidaysAdmin :one
SELECT COUNT(*)
FROM holidays h
WHERE (sqlc.narg('year')::int IS NULL OR EXTRACT(YEAR FROM h.holiday_date)::int = sqlc.narg('year')::int)
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND h.is_active)
        OR (sqlc.arg('status')::text = 'inactive' AND NOT h.is_active)
      );

-- name: GetHolidayAdminByID :one
-- Includes inactive rows; 404 decided by the service.
SELECT * FROM holidays WHERE id = $1;

-- name: FindActiveHolidayByDate :one
-- Uniqueness pre-check for create / enable / import (holidays_active_date_uq is the backstop).
SELECT id FROM holidays WHERE holiday_date = $1 AND is_active;

-- name: CreateHoliday :one
INSERT INTO holidays (holiday_date, name, created_by)
VALUES (sqlc.arg('holiday_date')::date, btrim(sqlc.arg('name')::text), sqlc.arg('created_by')::bigint)
RETURNING *;

-- name: UpdateHolidayName :one
UPDATE holidays SET name = btrim(sqlc.arg('name')::text)
WHERE id = sqlc.arg('id')::bigint
RETURNING *;

-- name: SetHolidayActive :one
UPDATE holidays
SET is_active = sqlc.arg('is_active')::boolean,
    deleted_at = CASE WHEN sqlc.arg('is_active')::boolean THEN NULL ELSE now() END
WHERE id = sqlc.arg('id')::bigint
RETURNING *;
