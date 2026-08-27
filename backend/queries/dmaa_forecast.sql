-- name: ListDmaaForecast :many
-- Paginated, filtered, sorted dmaa_atm_forecast rows for the DMAA Forecast
-- Viewer. Empty-string filters mean "no filter applied" (same convention as
-- ListItmCashpos). Sort keys are allowlisted in the service layer. The
-- trailing composite-PK tiebreaker keeps pagination deterministic when the
-- chosen sort column has ties.
SELECT
    terminal_id,
    dmaa_file_id,
    periode_pred,
    denom,
    amount_replenish,
    amount_refund,
    created_at
FROM dmaa_atm_forecast
WHERE
    (sqlc.arg('date_from')::text = '' OR periode_pred >= sqlc.arg('date_from')::date)
    AND (sqlc.arg('date_to')::text = '' OR periode_pred <= sqlc.arg('date_to')::date)
    AND (sqlc.arg('terminal_id')::text = '' OR terminal_id ILIKE '%' || sqlc.arg('terminal_id')::text || '%')
ORDER BY
    CASE WHEN sqlc.arg('sort_by')::text = 'terminal_id' AND sqlc.arg('sort_order')::text = 'asc' THEN terminal_id END ASC,
    CASE WHEN sqlc.arg('sort_by')::text = 'terminal_id' AND sqlc.arg('sort_order')::text = 'desc' THEN terminal_id END DESC,
    CASE WHEN sqlc.arg('sort_by')::text = 'dmaa_file_id' AND sqlc.arg('sort_order')::text = 'asc' THEN dmaa_file_id END ASC,
    CASE WHEN sqlc.arg('sort_by')::text = 'dmaa_file_id' AND sqlc.arg('sort_order')::text = 'desc' THEN dmaa_file_id END DESC,
    CASE WHEN sqlc.arg('sort_by')::text = 'periode_pred' AND sqlc.arg('sort_order')::text = 'asc' THEN periode_pred END ASC,
    CASE WHEN sqlc.arg('sort_by')::text = 'periode_pred' AND sqlc.arg('sort_order')::text = 'desc' THEN periode_pred END DESC,
    CASE WHEN sqlc.arg('sort_by')::text = 'denom' AND sqlc.arg('sort_order')::text = 'asc' THEN denom END ASC,
    CASE WHEN sqlc.arg('sort_by')::text = 'denom' AND sqlc.arg('sort_order')::text = 'desc' THEN denom END DESC,
    CASE WHEN sqlc.arg('sort_by')::text = 'amount_replenish' AND sqlc.arg('sort_order')::text = 'asc' THEN amount_replenish END ASC,
    CASE WHEN sqlc.arg('sort_by')::text = 'amount_replenish' AND sqlc.arg('sort_order')::text = 'desc' THEN amount_replenish END DESC,
    CASE WHEN sqlc.arg('sort_by')::text = 'amount_refund' AND sqlc.arg('sort_order')::text = 'asc' THEN amount_refund END ASC,
    CASE WHEN sqlc.arg('sort_by')::text = 'amount_refund' AND sqlc.arg('sort_order')::text = 'desc' THEN amount_refund END DESC,
    CASE WHEN sqlc.arg('sort_by')::text = 'created_at' AND sqlc.arg('sort_order')::text = 'asc' THEN created_at END ASC,
    CASE WHEN sqlc.arg('sort_by')::text = 'created_at' AND sqlc.arg('sort_order')::text = 'desc' THEN created_at END DESC,
    dmaa_file_id ASC,
    terminal_id ASC,
    periode_pred ASC,
    denom ASC
LIMIT sqlc.arg('page_size')::int OFFSET (sqlc.arg('page')::int - 1) * sqlc.arg('page_size')::int;

-- name: CountDmaaForecast :one
-- Mirrors ListDmaaForecast's WHERE exactly (minus ORDER BY/LIMIT/OFFSET) so
-- the count matches the filtered result set for pagination math.
SELECT count(*)
FROM dmaa_atm_forecast
WHERE
    (sqlc.arg('date_from')::text = '' OR periode_pred >= sqlc.arg('date_from')::date)
    AND (sqlc.arg('date_to')::text = '' OR periode_pred <= sqlc.arg('date_to')::date)
    AND (sqlc.arg('terminal_id')::text = '' OR terminal_id ILIKE '%' || sqlc.arg('terminal_id')::text || '%');
