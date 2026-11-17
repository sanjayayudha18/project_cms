-- Admin kategori ATM Anomaly cash count (ScDD 5.1, AC-5.1-05). Mutations run only inside the
-- applier (apply-on-approve tx). Soft-disable only.

-- name: ListCashCountAnomalyCategoriesAdmin :many
SELECT id, code, name, description, is_active, deleted_at, created_at, updated_at
FROM cash_count_anomaly_categories
WHERE sqlc.arg('status')::text = 'all'
   OR (sqlc.arg('status')::text = 'active' AND is_active)
   OR (sqlc.arg('status')::text = 'disabled' AND NOT is_active)
ORDER BY code, id;

-- name: GetCashCountAnomalyCategoryAdminByID :one
-- Includes disabled rows; doubles as the "before" snapshot / CurrentState.
SELECT id, code, name, description, is_active, deleted_at
FROM cash_count_anomaly_categories WHERE id = $1;

-- name: FindCashCountAnomalyCategoryByCode :one
-- Uniqueness pre-check. code is UNIQUE over ALL rows (disabled included), so no is_active filter.
SELECT id FROM cash_count_anomaly_categories WHERE code = sqlc.arg('code')::text LIMIT 1;

-- name: CreateCashCountAnomalyCategory :one
INSERT INTO cash_count_anomaly_categories (code, name, description)
VALUES (sqlc.arg('code'), sqlc.arg('name'), sqlc.narg('description'))
RETURNING id, code, name, description, is_active, deleted_at;

-- name: UpdateCashCountAnomalyCategory :one
-- code is immutable. Active rows only (no row => applier error => 409).
UPDATE cash_count_anomaly_categories
SET name = sqlc.arg('name'), description = sqlc.narg('description')
WHERE id = sqlc.arg('id') AND is_active
RETURNING id, code, name, description, is_active, deleted_at;

-- name: DisableCashCountAnomalyCategory :exec
UPDATE cash_count_anomaly_categories SET is_active = false, deleted_at = now() WHERE id = $1;

-- name: EnableCashCountAnomalyCategory :exec
UPDATE cash_count_anomaly_categories SET is_active = true, deleted_at = NULL WHERE id = $1;
