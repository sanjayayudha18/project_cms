-- name: ListATMsWithCashPos :many
-- Paginated, filtered, sorted ATM list joined with the latest itm_replenish
-- record per terminal. location_name/address/region are NOT columns on
-- atms — they require joining locations (via atms.location_id) and
-- regions (via locations.region_id). Status is computed here via a CASE
-- expression (Property 1 precedence: unconfigured > no_data > critical >
-- low > normal) inside the `sub` subquery so it can be filtered on and
-- sorted by in the outer query — a SELECT-list alias cannot be referenced
-- directly in the WHERE clause that produced it.
--
-- All filter args are plain (non-nullable) strings matching
-- ListATMsParams in design.md: an empty string ("") or "all" (for status)
-- means "no filter applied", so callers always pass concrete values,
-- never NULL. status is comma-separated multi-select (requirements.md Req
-- 89.3), same pattern as machine_type/brand — corrects an earlier
-- single-value assumption carried over from design.md's API contract table.
WITH sub AS (
    SELECT
        a.terminal_id,
        l.name AS location_name,
        l.address_line1 AS address,
        a.machine_type,
        a.brand,
        a.deployment_type,
        r.region,
        a.low_threshold_amount,
        a.critical_threshold_amount,
        lcp.replenish_date AS last_replenish_date,
        lcp.replenish_time AS last_replenish_time,
        lcp.refund_total,
        lcp.replenish_total,
        lcp.escrow,
        CASE
            WHEN a.low_threshold_amount IS NULL THEN 'unconfigured'
            WHEN lcp.refund_total IS NULL THEN 'no_data'
            WHEN a.critical_threshold_amount IS NOT NULL
                 AND lcp.refund_total <= a.critical_threshold_amount THEN 'critical'
            WHEN lcp.refund_total <= a.low_threshold_amount THEN 'low'
            ELSE 'normal'
        END AS status
    FROM atms a
    JOIN locations l ON l.id = a.location_id
    JOIN regions r ON r.id = l.region_id
    LEFT JOIN LATERAL (
        SELECT
            cp.replenish_date,
            cp.replenish_time,
            cp.refund_total,
            cp.replenish_total,
            cp.escrow
        FROM itm_replenish cp
        WHERE cp.terminal_id = a.terminal_id
        ORDER BY cp.replenish_date DESC, cp.replenish_time DESC
        LIMIT 1
    ) lcp ON true
    WHERE a.is_active = true
      AND a.deleted_at IS NULL
)
SELECT
    sub.terminal_id,
    sub.location_name,
    sub.address,
    sub.machine_type,
    sub.brand,
    sub.deployment_type,
    sub.low_threshold_amount,
    sub.critical_threshold_amount,
    sub.last_replenish_date,
    sub.last_replenish_time,
    sub.refund_total,
    sub.replenish_total,
    sub.escrow,
    sub.status
FROM sub
WHERE
    (sqlc.arg('search')::text = ''
        OR sub.terminal_id ILIKE '%' || sqlc.arg('search')::text || '%'
        OR sub.location_name ILIKE '%' || sqlc.arg('search')::text || '%')
    AND (sqlc.arg('status')::text = 'all'
        OR sub.status = ANY(string_to_array(sqlc.arg('status')::text, ',')))
    AND (sqlc.arg('machine_type')::text = ''
        OR sub.machine_type = ANY(string_to_array(sqlc.arg('machine_type')::text, ',')))
    AND (sqlc.arg('brand')::text = ''
        OR sub.brand = ANY(string_to_array(sqlc.arg('brand')::text, ',')))
    AND (sqlc.arg('deployment_type')::text = '' OR sub.deployment_type = sqlc.arg('deployment_type')::text)
    AND (sqlc.arg('region')::text = '' OR sub.region ILIKE '%' || sqlc.arg('region')::text || '%')
    AND (sqlc.arg('date_from')::text = '' OR sub.last_replenish_date >= sqlc.arg('date_from')::date)
    AND (sqlc.arg('date_to')::text = '' OR sub.last_replenish_date <= sqlc.arg('date_to')::date)
ORDER BY
    CASE WHEN sqlc.arg('sort_by')::text = 'terminal_id' AND sqlc.arg('sort_order')::text = 'asc' THEN sub.terminal_id END ASC,
    CASE WHEN sqlc.arg('sort_by')::text = 'terminal_id' AND sqlc.arg('sort_order')::text = 'desc' THEN sub.terminal_id END DESC,
    CASE WHEN sqlc.arg('sort_by')::text = 'location' AND sqlc.arg('sort_order')::text = 'asc' THEN sub.location_name END ASC,
    CASE WHEN sqlc.arg('sort_by')::text = 'location' AND sqlc.arg('sort_order')::text = 'desc' THEN sub.location_name END DESC,
    CASE WHEN sqlc.arg('sort_by')::text = 'last_replenish_date' AND sqlc.arg('sort_order')::text = 'asc' THEN sub.last_replenish_date END ASC,
    CASE WHEN sqlc.arg('sort_by')::text = 'last_replenish_date' AND sqlc.arg('sort_order')::text = 'desc' THEN sub.last_replenish_date END DESC,
    CASE WHEN sqlc.arg('sort_by')::text = 'refund_total' AND sqlc.arg('sort_order')::text = 'asc' THEN sub.refund_total END ASC,
    CASE WHEN sqlc.arg('sort_by')::text = 'refund_total' AND sqlc.arg('sort_order')::text = 'desc' THEN sub.refund_total END DESC,
    CASE WHEN sqlc.arg('sort_by')::text = 'replenish_total' AND sqlc.arg('sort_order')::text = 'asc' THEN sub.replenish_total END ASC,
    CASE WHEN sqlc.arg('sort_by')::text = 'replenish_total' AND sqlc.arg('sort_order')::text = 'desc' THEN sub.replenish_total END DESC,
    CASE WHEN sqlc.arg('sort_by')::text = 'status' AND sqlc.arg('sort_order')::text = 'asc' THEN sub.status END ASC,
    CASE WHEN sqlc.arg('sort_by')::text = 'status' AND sqlc.arg('sort_order')::text = 'desc' THEN sub.status END DESC,
    sub.terminal_id ASC
LIMIT sqlc.arg('page_size')::int OFFSET (sqlc.arg('page')::int - 1) * sqlc.arg('page_size')::int;

-- name: CountATMsWithCashPos :one
-- Mirrors ListATMsWithCashPos's FROM/JOIN/WHERE (including all dynamic
-- filters) exactly, minus ORDER BY/LIMIT/OFFSET, so the count matches the
-- filtered result set for pagination math (Property 6).
WITH sub AS (
    SELECT
        a.terminal_id,
        l.name AS location_name,
        a.machine_type,
        a.brand,
        a.deployment_type,
        r.region,
        lcp.replenish_date AS last_replenish_date,
        CASE
            WHEN a.low_threshold_amount IS NULL THEN 'unconfigured'
            WHEN lcp.refund_total IS NULL THEN 'no_data'
            WHEN a.critical_threshold_amount IS NOT NULL
                 AND lcp.refund_total <= a.critical_threshold_amount THEN 'critical'
            WHEN lcp.refund_total <= a.low_threshold_amount THEN 'low'
            ELSE 'normal'
        END AS status
    FROM atms a
    JOIN locations l ON l.id = a.location_id
    JOIN regions r ON r.id = l.region_id
    LEFT JOIN LATERAL (
        SELECT cp.replenish_date, cp.replenish_time, cp.refund_total
        FROM itm_replenish cp
        WHERE cp.terminal_id = a.terminal_id
        ORDER BY cp.replenish_date DESC, cp.replenish_time DESC
        LIMIT 1
    ) lcp ON true
    WHERE a.is_active = true
      AND a.deleted_at IS NULL
)
SELECT COUNT(*)
FROM sub
WHERE
    (sqlc.arg('search')::text = ''
        OR sub.terminal_id ILIKE '%' || sqlc.arg('search')::text || '%'
        OR sub.location_name ILIKE '%' || sqlc.arg('search')::text || '%')
    AND (sqlc.arg('status')::text = 'all'
        OR sub.status = ANY(string_to_array(sqlc.arg('status')::text, ',')))
    AND (sqlc.arg('machine_type')::text = ''
        OR sub.machine_type = ANY(string_to_array(sqlc.arg('machine_type')::text, ',')))
    AND (sqlc.arg('brand')::text = ''
        OR sub.brand = ANY(string_to_array(sqlc.arg('brand')::text, ',')))
    AND (sqlc.arg('deployment_type')::text = '' OR sub.deployment_type = sqlc.arg('deployment_type')::text)
    AND (sqlc.arg('region')::text = '' OR sub.region ILIKE '%' || sqlc.arg('region')::text || '%')
    AND (sqlc.arg('date_from')::text = '' OR sub.last_replenish_date >= sqlc.arg('date_from')::date)
    AND (sqlc.arg('date_to')::text = '' OR sub.last_replenish_date <= sqlc.arg('date_to')::date);

-- name: GetATMSummary :one
-- Global status counts across all active ATMs, independent of any list
-- filters (Property 10). Does not join locations/regions since it only
-- aggregates status, and does not accept filter args.
SELECT
    COUNT(*) AS total,
    COUNT(*) FILTER (WHERE sub.status = 'critical') AS critical,
    COUNT(*) FILTER (WHERE sub.status = 'low') AS low,
    COUNT(*) FILTER (WHERE sub.status = 'normal') AS normal,
    COUNT(*) FILTER (WHERE sub.status = 'unconfigured') AS unconfigured,
    COUNT(*) FILTER (WHERE sub.status = 'no_data') AS no_data
FROM (
    SELECT
        CASE
            WHEN a.low_threshold_amount IS NULL THEN 'unconfigured'
            WHEN lcp.refund_total IS NULL THEN 'no_data'
            WHEN a.critical_threshold_amount IS NOT NULL
                 AND lcp.refund_total <= a.critical_threshold_amount THEN 'critical'
            WHEN lcp.refund_total <= a.low_threshold_amount THEN 'low'
            ELSE 'normal'
        END AS status
    FROM atms a
    LEFT JOIN LATERAL (
        SELECT cp.refund_total
        FROM itm_replenish cp
        WHERE cp.terminal_id = a.terminal_id
        ORDER BY cp.replenish_date DESC, cp.replenish_time DESC
        LIMIT 1
    ) lcp ON true
    WHERE a.is_active = true
      AND a.deleted_at IS NULL
) sub;

-- name: GetLastUpdated :one
-- Most recent replenish_date + replenish_time across all itm_replenish
-- records, used for the data-freshness indicator. Returns NULL when no
-- cashpos records exist yet.
SELECT
    (replenish_date + replenish_time)::timestamp AS last_updated
FROM itm_replenish
ORDER BY replenish_date DESC, replenish_time DESC
LIMIT 1;
