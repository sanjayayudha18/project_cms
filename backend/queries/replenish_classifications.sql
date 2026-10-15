-- Klasifikasi realisasi vs order (replenishment-realisasi FR4-FR6, migration 031). Read-only from Go: the
-- Python job replenish_classify writes these tables. Every caller reads on the replica (Sec 6).
-- Date filter = report_date (jadwal tiket, or realisasi for no_order).

-- name: ListReplenishClassifications :many
-- Layar Replenishment (FR4.1), report amount != order (FR6.1, only_diff + final_only), VendorPortal
-- Realisasi (FR5.2, vendor_id + optional vendor_branch_id scope).
SELECT c.id, c.ticket_id, c.terminal_id, c.vendor_id, c.vendor_branch_id, c.scheduled_date, c.realized_date,
       c.realized_time, c.report_date, c.workday_offset, c.classification, c.is_final, c.has_amount_diff,
       c.vendor_mismatch, c.vendor_report_late, c.classified_at,
       t.ticket_number, vr.id AS request_id, vr.request_number,
       v.name AS vendor_name, vb.branch_code, vb.branch_name, vb.region_code
FROM replenish_classifications c
LEFT JOIN vendor_request_tickets t ON t.id = c.ticket_id
LEFT JOIN vendor_requests vr ON vr.request_number = t.request_number
LEFT JOIN vendors v ON v.id = c.vendor_id
LEFT JOIN vendor_branches vb ON vb.id = c.vendor_branch_id
WHERE c.report_date BETWEEN sqlc.arg('date_from')::date AND sqlc.arg('date_to')::date
  AND (sqlc.narg('vendor_id')::bigint IS NULL OR c.vendor_id = sqlc.narg('vendor_id')::bigint)
  AND (sqlc.narg('vendor_branch_id')::bigint IS NULL OR c.vendor_branch_id = sqlc.narg('vendor_branch_id')::bigint)
  AND (sqlc.narg('region_code')::text IS NULL OR vb.region_code = sqlc.narg('region_code')::text)
  AND (sqlc.narg('classification')::text IS NULL OR c.classification = sqlc.narg('classification')::text)
  AND (NOT sqlc.arg('only_diff')::boolean OR c.has_amount_diff)
  AND (NOT sqlc.arg('final_only')::boolean OR c.is_final)
ORDER BY c.report_date, c.terminal_id, c.id
LIMIT sqlc.arg('page_limit')::bigint OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountReplenishClassifications :one
SELECT COUNT(*)
FROM replenish_classifications c
LEFT JOIN vendor_branches vb ON vb.id = c.vendor_branch_id
WHERE c.report_date BETWEEN sqlc.arg('date_from')::date AND sqlc.arg('date_to')::date
  AND (sqlc.narg('vendor_id')::bigint IS NULL OR c.vendor_id = sqlc.narg('vendor_id')::bigint)
  AND (sqlc.narg('vendor_branch_id')::bigint IS NULL OR c.vendor_branch_id = sqlc.narg('vendor_branch_id')::bigint)
  AND (sqlc.narg('region_code')::text IS NULL OR vb.region_code = sqlc.narg('region_code')::text)
  AND (sqlc.narg('classification')::text IS NULL OR c.classification = sqlc.narg('classification')::text)
  AND (NOT sqlc.arg('only_diff')::boolean OR c.has_amount_diff)
  AND (NOT sqlc.arg('final_only')::boolean OR c.is_final);

-- name: ListReplenishClassificationAmounts :many
-- Nominal per denom for one page of classifications (order vs ITM vs vendor, selisih = ITM - order).
SELECT classification_id, denom, order_amount, itm_amount, vendor_amount, diff
FROM replenish_classification_amounts
WHERE classification_id = ANY(sqlc.arg('ids')::bigint[])
ORDER BY classification_id, denom DESC;

-- name: SummarizeReplenishClassifications :many
-- Dashboard (FR4.2): rows per classification + flags in the range.
SELECT c.classification,
       COUNT(*)::bigint AS total,
       COUNT(*) FILTER (WHERE c.has_amount_diff)::bigint AS amount_diff,
       COUNT(*) FILTER (WHERE c.vendor_mismatch)::bigint AS vendor_mismatch,
       COUNT(*) FILTER (WHERE c.vendor_report_late)::bigint AS vendor_report_late
FROM replenish_classifications c
WHERE c.report_date BETWEEN sqlc.arg('date_from')::date AND sqlc.arg('date_to')::date
  AND (sqlc.narg('vendor_id')::bigint IS NULL OR c.vendor_id = sqlc.narg('vendor_id')::bigint)
GROUP BY c.classification
ORDER BY c.classification;

-- name: SummarizeReplenishInstructions :many
-- Dashboard FNC 003: nominal instruksi harian + jumlah ATM, from the instructions themselves (active
-- tickets of requests in an instruction status, Q2b), so it shows before the job classified anything.
SELECT t.replenish_date,
       COUNT(DISTINCT t.terminal_id)::bigint AS terminals,
       COALESCE(SUM(i.amount_replenish), 0)::bigint AS order_amount
FROM vendor_request_tickets t
JOIN vendor_requests vr ON vr.request_number = t.request_number
JOIN vendor_request_items i ON i.vendor_request_id = vr.id AND i.terminal_id = t.terminal_id
WHERE t.is_active AND NOT vr.is_canceled
  AND vr.status IN ('vendor_accepted', 'completion_pending', 'completed', 'approved', 'ready')
  AND t.replenish_date BETWEEN sqlc.arg('date_from')::date AND sqlc.arg('date_to')::date
  AND (sqlc.narg('vendor_id')::bigint IS NULL OR vr.vendor_id = sqlc.narg('vendor_id')::bigint)
GROUP BY t.replenish_date
ORDER BY t.replenish_date;

-- name: ReportReplenishTrips :many
-- Trip / realisasi per vendor per ATM (FR6.2): fills seen by ITM in the range (matched or no_order),
-- most trips first.
SELECT c.vendor_id, v.name AS vendor_name, c.terminal_id, COUNT(*)::bigint AS trips,
       MIN(c.realized_date)::date AS first_date, MAX(c.realized_date)::date AS last_date
FROM replenish_classifications c
LEFT JOIN vendors v ON v.id = c.vendor_id
WHERE c.realized_date BETWEEN sqlc.arg('date_from')::date AND sqlc.arg('date_to')::date
  AND c.classification <> 'void'
  AND (sqlc.narg('vendor_id')::bigint IS NULL OR c.vendor_id = sqlc.narg('vendor_id')::bigint)
GROUP BY c.vendor_id, v.name, c.terminal_id
ORDER BY trips DESC, v.name NULLS LAST, c.terminal_id;
