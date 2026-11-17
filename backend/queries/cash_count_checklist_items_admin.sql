-- Admin item checklist Berita Acara cash count (ScDD 5.1, AC-5.1-06/07). No seed (K15).
-- Mutations run only inside the applier (apply-on-approve tx). Soft-disable only.

-- name: ListCashCountChecklistItemsAdmin :many
-- category = '' means all categories.
SELECT id, category, day, point_no, item_text, is_active, deleted_at, created_at, updated_at
FROM cash_count_checklist_items
WHERE (sqlc.arg('category')::text = '' OR category = sqlc.arg('category')::text)
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND is_active)
        OR (sqlc.arg('status')::text = 'disabled' AND NOT is_active)
      )
ORDER BY category, COALESCE(day, ''), point_no, id;

-- name: GetCashCountChecklistItemAdminByID :one
-- Includes disabled rows; doubles as the "before" snapshot / CurrentState.
SELECT id, category, day, point_no, item_text, is_active, deleted_at
FROM cash_count_checklist_items WHERE id = $1;

-- name: FindActiveCashCountChecklistPoint :one
-- Uniqueness pre-check: an active item with the same (category, day) and point_no, excluding
-- exclude_id (0 = none). day = '' for NON_MACHINE.
SELECT id FROM cash_count_checklist_items
WHERE is_active
  AND category = sqlc.arg('category')::text
  AND COALESCE(day, '') = sqlc.arg('day')::text
  AND point_no = sqlc.arg('point_no')::smallint
  AND id <> sqlc.arg('exclude_id')::bigint
LIMIT 1;

-- name: CreateCashCountChecklistItem :one
-- day NULL for NON_MACHINE (table CHECK enforces category <-> day).
INSERT INTO cash_count_checklist_items (category, day, point_no, item_text)
VALUES (sqlc.arg('category'), sqlc.narg('day'), sqlc.arg('point_no'), sqlc.arg('item_text'))
RETURNING id, category, day, point_no, item_text, is_active, deleted_at;

-- name: UpdateCashCountChecklistItem :one
-- category stays immutable; day/point_no/text editable (same CHECK + active-unique guard). Active rows only.
UPDATE cash_count_checklist_items
SET day = sqlc.narg('day'), point_no = sqlc.arg('point_no'), item_text = sqlc.arg('item_text')
WHERE id = sqlc.arg('id') AND is_active
RETURNING id, category, day, point_no, item_text, is_active, deleted_at;

-- name: DisableCashCountChecklistItem :exec
UPDATE cash_count_checklist_items SET is_active = false, deleted_at = now() WHERE id = $1;

-- name: EnableCashCountChecklistItem :exec
-- The active-unique index rejects re-enabling a point_no that was reused meanwhile.
UPDATE cash_count_checklist_items SET is_active = true, deleted_at = NULL WHERE id = $1;
