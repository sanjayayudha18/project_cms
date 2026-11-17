-- Admin parameter kategori vault cash count (ScDD 5.1, AC-5.1-01/03/04). One row = one set of
-- thresholds; exactly one active set (superseded_at IS NULL). Only op `create` exists (= submit a
-- new set): both mutations below run only inside the applier (apply-on-approve tx), in this order.

-- name: ListCashCountCategoryParamsAdmin :many
-- Active set first, then history (newest first). A handful of rows: unpaged.
SELECT id, medium_min_amount, medium_max_amount, currency_code, onsite_high, onsite_medium, onsite_low,
       period_months, created_by, effective_from, superseded_at, created_at
FROM cash_count_category_params
ORDER BY (superseded_at IS NULL) DESC, effective_from DESC, id DESC;

-- name: GetActiveCashCountCategoryParams :one
-- The single active set; doubles as the "before" snapshot / CurrentState and the classifier input.
SELECT id, medium_min_amount, medium_max_amount, currency_code, onsite_high, onsite_medium, onsite_low,
       period_months, created_by, effective_from, superseded_at, created_at
FROM cash_count_category_params
WHERE superseded_at IS NULL;

-- name: SupersedeActiveCashCountCategoryParams :exec
-- Step 1 of apply: close the active set. Must run before the insert (active-unique index).
UPDATE cash_count_category_params SET superseded_at = now() WHERE superseded_at IS NULL;

-- name: CreateCashCountCategoryParams :one
-- Step 2 of apply: new active set. effective_from defaults to now() = same tx time as step 1.
INSERT INTO cash_count_category_params
    (medium_min_amount, medium_max_amount, onsite_high, onsite_medium, onsite_low, created_by)
VALUES (sqlc.arg('medium_min_amount'), sqlc.arg('medium_max_amount'),
        sqlc.arg('onsite_high'), sqlc.arg('onsite_medium'), sqlc.arg('onsite_low'), sqlc.arg('created_by'))
RETURNING id, medium_min_amount, medium_max_amount, currency_code, onsite_high, onsite_medium, onsite_low,
          period_months, created_by, effective_from, superseded_at, created_at;
