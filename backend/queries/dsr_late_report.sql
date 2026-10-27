-- dsr-late-report (Phase 2.3, .claude/sdlc/dsr-late-report/spec.md FR2/FR3, plan D2/D3).
-- Read-only, run on the replica. Status is computed on read (spec #7), never stored.
--
-- Every report query starts with the SAME CTE chain; keep the copies identical.
-- backend_python/dsr_late_notify/queries.py copies obliged + hits for X = today (D2);
-- change both sides together and keep internal/repository/testdata/dsr_late_cases.json green.
--
--   days           X in [max(month_start, start_date), min(month end, as_of WIB date)]   (FR2.5)
--   counted_vaults active vault of an FLM vendor branch, created_at WIB <= X,
--                  with >= 1 active dsr_location_vault_maps row                       (FR2.2, S5)
--   obliged        branch with >= 1 counted vault on X, active on X:
--                  created_at WIB <= X AND (is_active OR deleted_at WIB > X)           (FR2.3, S3)
--   hits           receipts of the branch's vendor for X whose label maps (lower = lower)
--                  to a counted vault of that branch, received by as_of                 (FR3.1/3.2)
--   first_hit      earliest hit per (X, branch); deadline = X 09:00 Asia/Jakarta, inclusive
--                  at whole seconds (K1: 09:00:00.xxx = OK, 09:00:01 = late)
--   status         ok | late | pending (no receipt, as_of <= deadline, only today: K4) | not_sent
--
-- Dates are converted with AT TIME ZONE 'Asia/Jakarta' so the DB session TZ never matters (D3).
-- as_of is passed in (normally now()) so tests and the Python job pin "now".

-- name: ListDsrLateDaily :many
-- Detail harian (FR4.2): one row per (report_date, branch). page_limit NULL = all rows (CSV).
-- No receipt: first_received_at NULL, receipt_checksum ''. late_minutes is 0 unless status = 'late'.
WITH days AS (
    SELECT d::date AS report_date
    FROM generate_series(
        GREATEST(sqlc.arg('month_start')::date, sqlc.arg('start_date')::date),
        LEAST((sqlc.arg('month_start')::date + interval '1 month - 1 day')::date,
              (sqlc.arg('as_of')::timestamptz AT TIME ZONE 'Asia/Jakarta')::date),
        interval '1 day') AS d
),
counted_vaults AS (
    SELECT dy.report_date, v.id AS vault_id, v.vendor_branch_id, b.vendor_id
    FROM days dy
    JOIN vendor_vaults v ON v.is_active
                        AND (v.created_at AT TIME ZONE 'Asia/Jakarta')::date <= dy.report_date
    JOIN vendor_branches b ON b.id = v.vendor_branch_id
    JOIN vendors vd ON vd.id = b.vendor_id AND vd.kind = 'FLM_VENDOR'
    WHERE EXISTS (SELECT 1 FROM dsr_location_vault_maps m
                  WHERE m.is_active AND m.vendor_id = b.vendor_id AND m.vendor_vault_id = v.id)
),
obliged AS (
    SELECT DISTINCT cv.report_date, cv.vendor_branch_id
    FROM counted_vaults cv
    JOIN vendor_branches b ON b.id = cv.vendor_branch_id
    WHERE (b.created_at AT TIME ZONE 'Asia/Jakarta')::date <= cv.report_date
      AND (b.is_active OR (b.deleted_at AT TIME ZONE 'Asia/Jakarta')::date > cv.report_date)
),
hits AS (
    SELECT cv.report_date, cv.vendor_branch_id, r.id AS receipt_id, r.received_at, r.checksum, l.label
    FROM dsr_receipts r
    CROSS JOIN LATERAL unnest(r.locations) AS l(label)
    JOIN dsr_location_vault_maps m ON m.is_active AND m.vendor_id = r.vendor_id
                                  AND lower(m.dsr_location) = lower(l.label)
    JOIN counted_vaults cv ON cv.vault_id = m.vendor_vault_id
                          AND cv.vendor_id = r.vendor_id
                          AND cv.report_date = r.report_date
    WHERE r.received_at <= sqlc.arg('as_of')::timestamptz
),
first_hit AS (
    SELECT DISTINCT ON (report_date, vendor_branch_id) report_date, vendor_branch_id, receipt_id, received_at, checksum
    FROM hits
    ORDER BY report_date, vendor_branch_id, received_at, receipt_id
),
report AS (
    SELECT o.report_date, o.vendor_branch_id,
           ((o.report_date + time '09:00') AT TIME ZONE 'Asia/Jakarta') AS deadline,
           f.received_at, f.checksum,
           (SELECT array_agg(DISTINCT h.label ORDER BY h.label) FROM hits h
            WHERE h.receipt_id = f.receipt_id AND h.report_date = o.report_date
              AND h.vendor_branch_id = o.vendor_branch_id) AS labels
    FROM obliged o
    LEFT JOIN first_hit f ON f.report_date = o.report_date AND f.vendor_branch_id = o.vendor_branch_id
),
status AS (
    SELECT rp.*,
           CASE WHEN rp.received_at IS NOT NULL AND date_trunc('second', rp.received_at) <= rp.deadline THEN 'ok'
                WHEN rp.received_at IS NOT NULL THEN 'late'
                WHEN sqlc.arg('as_of')::timestamptz <= rp.deadline THEN 'pending'
                ELSE 'not_sent' END AS status
    FROM report rp
)
SELECT s.report_date::date AS report_date,
       vd.id AS vendor_id, vd.code AS vendor_code, vd.name AS vendor_name,
       b.id AS vendor_branch_id, b.branch_code, b.branch_name, b.region_code,
       s.deadline::timestamptz AS deadline,
       s.received_at::timestamptz AS first_received_at,
       COALESCE(s.checksum, '')::text AS receipt_checksum,
       COALESCE(s.labels, '{}')::text[] AS matched_locations,
       s.status::text AS status,
       (CASE WHEN s.status = 'late'
             THEN ceil(extract(epoch FROM date_trunc('second', s.received_at) - s.deadline) / 60)
             ELSE 0 END)::int AS late_minutes
FROM status s
JOIN vendor_branches b ON b.id = s.vendor_branch_id
JOIN vendors vd ON vd.id = b.vendor_id
WHERE (sqlc.narg('vendor_id')::bigint IS NULL OR vd.id = sqlc.narg('vendor_id')::bigint)
  AND (sqlc.narg('vendor_branch_id')::bigint IS NULL OR b.id = sqlc.narg('vendor_branch_id')::bigint)
  AND (sqlc.narg('status')::text IS NULL OR s.status = sqlc.narg('status')::text)
ORDER BY s.report_date, vd.name, b.branch_code, b.id
LIMIT sqlc.narg('page_limit')::bigint OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountDsrLateDaily :one
-- Same CTE + filters as ListDsrLateDaily, no paging -- pagination total.
WITH days AS (
    SELECT d::date AS report_date
    FROM generate_series(
        GREATEST(sqlc.arg('month_start')::date, sqlc.arg('start_date')::date),
        LEAST((sqlc.arg('month_start')::date + interval '1 month - 1 day')::date,
              (sqlc.arg('as_of')::timestamptz AT TIME ZONE 'Asia/Jakarta')::date),
        interval '1 day') AS d
),
counted_vaults AS (
    SELECT dy.report_date, v.id AS vault_id, v.vendor_branch_id, b.vendor_id
    FROM days dy
    JOIN vendor_vaults v ON v.is_active
                        AND (v.created_at AT TIME ZONE 'Asia/Jakarta')::date <= dy.report_date
    JOIN vendor_branches b ON b.id = v.vendor_branch_id
    JOIN vendors vd ON vd.id = b.vendor_id AND vd.kind = 'FLM_VENDOR'
    WHERE EXISTS (SELECT 1 FROM dsr_location_vault_maps m
                  WHERE m.is_active AND m.vendor_id = b.vendor_id AND m.vendor_vault_id = v.id)
),
obliged AS (
    SELECT DISTINCT cv.report_date, cv.vendor_branch_id
    FROM counted_vaults cv
    JOIN vendor_branches b ON b.id = cv.vendor_branch_id
    WHERE (b.created_at AT TIME ZONE 'Asia/Jakarta')::date <= cv.report_date
      AND (b.is_active OR (b.deleted_at AT TIME ZONE 'Asia/Jakarta')::date > cv.report_date)
),
hits AS (
    SELECT cv.report_date, cv.vendor_branch_id, r.id AS receipt_id, r.received_at, r.checksum, l.label
    FROM dsr_receipts r
    CROSS JOIN LATERAL unnest(r.locations) AS l(label)
    JOIN dsr_location_vault_maps m ON m.is_active AND m.vendor_id = r.vendor_id
                                  AND lower(m.dsr_location) = lower(l.label)
    JOIN counted_vaults cv ON cv.vault_id = m.vendor_vault_id
                          AND cv.vendor_id = r.vendor_id
                          AND cv.report_date = r.report_date
    WHERE r.received_at <= sqlc.arg('as_of')::timestamptz
),
first_hit AS (
    SELECT DISTINCT ON (report_date, vendor_branch_id) report_date, vendor_branch_id, receipt_id, received_at, checksum
    FROM hits
    ORDER BY report_date, vendor_branch_id, received_at, receipt_id
),
report AS (
    SELECT o.report_date, o.vendor_branch_id,
           ((o.report_date + time '09:00') AT TIME ZONE 'Asia/Jakarta') AS deadline,
           f.received_at
    FROM obliged o
    LEFT JOIN first_hit f ON f.report_date = o.report_date AND f.vendor_branch_id = o.vendor_branch_id
),
status AS (
    SELECT rp.*,
           CASE WHEN rp.received_at IS NOT NULL AND date_trunc('second', rp.received_at) <= rp.deadline THEN 'ok'
                WHEN rp.received_at IS NOT NULL THEN 'late'
                WHEN sqlc.arg('as_of')::timestamptz <= rp.deadline THEN 'pending'
                ELSE 'not_sent' END AS status
    FROM report rp
)
SELECT count(*)
FROM status s
JOIN vendor_branches b ON b.id = s.vendor_branch_id
WHERE (sqlc.narg('vendor_id')::bigint IS NULL OR b.vendor_id = sqlc.narg('vendor_id')::bigint)
  AND (sqlc.narg('vendor_branch_id')::bigint IS NULL OR b.id = sqlc.narg('vendor_branch_id')::bigint)
  AND (sqlc.narg('status')::text IS NULL OR s.status = sqlc.narg('status')::text);

-- name: ListDsrLateSummary :many
-- Rekap per cabang (FR4.3): same CTE, aggregated per branch over the month.
WITH days AS (
    SELECT d::date AS report_date
    FROM generate_series(
        GREATEST(sqlc.arg('month_start')::date, sqlc.arg('start_date')::date),
        LEAST((sqlc.arg('month_start')::date + interval '1 month - 1 day')::date,
              (sqlc.arg('as_of')::timestamptz AT TIME ZONE 'Asia/Jakarta')::date),
        interval '1 day') AS d
),
counted_vaults AS (
    SELECT dy.report_date, v.id AS vault_id, v.vendor_branch_id, b.vendor_id
    FROM days dy
    JOIN vendor_vaults v ON v.is_active
                        AND (v.created_at AT TIME ZONE 'Asia/Jakarta')::date <= dy.report_date
    JOIN vendor_branches b ON b.id = v.vendor_branch_id
    JOIN vendors vd ON vd.id = b.vendor_id AND vd.kind = 'FLM_VENDOR'
    WHERE EXISTS (SELECT 1 FROM dsr_location_vault_maps m
                  WHERE m.is_active AND m.vendor_id = b.vendor_id AND m.vendor_vault_id = v.id)
),
obliged AS (
    SELECT DISTINCT cv.report_date, cv.vendor_branch_id
    FROM counted_vaults cv
    JOIN vendor_branches b ON b.id = cv.vendor_branch_id
    WHERE (b.created_at AT TIME ZONE 'Asia/Jakarta')::date <= cv.report_date
      AND (b.is_active OR (b.deleted_at AT TIME ZONE 'Asia/Jakarta')::date > cv.report_date)
),
hits AS (
    SELECT cv.report_date, cv.vendor_branch_id, r.id AS receipt_id, r.received_at, r.checksum, l.label
    FROM dsr_receipts r
    CROSS JOIN LATERAL unnest(r.locations) AS l(label)
    JOIN dsr_location_vault_maps m ON m.is_active AND m.vendor_id = r.vendor_id
                                  AND lower(m.dsr_location) = lower(l.label)
    JOIN counted_vaults cv ON cv.vault_id = m.vendor_vault_id
                          AND cv.vendor_id = r.vendor_id
                          AND cv.report_date = r.report_date
    WHERE r.received_at <= sqlc.arg('as_of')::timestamptz
),
first_hit AS (
    SELECT DISTINCT ON (report_date, vendor_branch_id) report_date, vendor_branch_id, receipt_id, received_at, checksum
    FROM hits
    ORDER BY report_date, vendor_branch_id, received_at, receipt_id
),
report AS (
    SELECT o.report_date, o.vendor_branch_id,
           ((o.report_date + time '09:00') AT TIME ZONE 'Asia/Jakarta') AS deadline,
           f.received_at
    FROM obliged o
    LEFT JOIN first_hit f ON f.report_date = o.report_date AND f.vendor_branch_id = o.vendor_branch_id
),
status AS (
    SELECT rp.*,
           CASE WHEN rp.received_at IS NOT NULL AND date_trunc('second', rp.received_at) <= rp.deadline THEN 'ok'
                WHEN rp.received_at IS NOT NULL THEN 'late'
                WHEN sqlc.arg('as_of')::timestamptz <= rp.deadline THEN 'pending'
                ELSE 'not_sent' END AS status
    FROM report rp
)
SELECT vd.id AS vendor_id, vd.code AS vendor_code, vd.name AS vendor_name,
       b.id AS vendor_branch_id, b.branch_code, b.branch_name, b.region_code,
       count(*)::int AS obliged_days,
       (count(*) FILTER (WHERE s.status = 'ok'))::int AS ok_days,
       (count(*) FILTER (WHERE s.status = 'late'))::int AS late_days,
       (count(*) FILTER (WHERE s.status = 'not_sent'))::int AS not_sent_days,
       (count(*) FILTER (WHERE s.status = 'pending'))::int AS pending_days,
       COALESCE(sum(ceil(extract(epoch FROM date_trunc('second', s.received_at) - s.deadline) / 60))
                FILTER (WHERE s.status = 'late'), 0)::int AS late_minutes_total
FROM status s
JOIN vendor_branches b ON b.id = s.vendor_branch_id
JOIN vendors vd ON vd.id = b.vendor_id
WHERE (sqlc.narg('vendor_id')::bigint IS NULL OR vd.id = sqlc.narg('vendor_id')::bigint)
  AND (sqlc.narg('vendor_branch_id')::bigint IS NULL OR b.id = sqlc.narg('vendor_branch_id')::bigint)
GROUP BY vd.id, vd.code, vd.name, b.id, b.branch_code, b.branch_name, b.region_code
ORDER BY vd.name, b.branch_code, b.id;

-- name: ListDsrLateUnmappedVaults :many
-- Peringatan FR3.4(a) / S2: active vaults (created by the last report day) of FLM branches
-- active on some report day, without an active DSR location map. branch_excluded = the
-- branch has no counted vault on any report day, so it is left out of the report.
WITH days AS (
    SELECT d::date AS report_date
    FROM generate_series(
        GREATEST(sqlc.arg('month_start')::date, sqlc.arg('start_date')::date),
        LEAST((sqlc.arg('month_start')::date + interval '1 month - 1 day')::date,
              (sqlc.arg('as_of')::timestamptz AT TIME ZONE 'Asia/Jakarta')::date),
        interval '1 day') AS d
),
counted_vaults AS (
    SELECT dy.report_date, v.id AS vault_id, v.vendor_branch_id, b.vendor_id
    FROM days dy
    JOIN vendor_vaults v ON v.is_active
                        AND (v.created_at AT TIME ZONE 'Asia/Jakarta')::date <= dy.report_date
    JOIN vendor_branches b ON b.id = v.vendor_branch_id
    JOIN vendors vd ON vd.id = b.vendor_id AND vd.kind = 'FLM_VENDOR'
    WHERE EXISTS (SELECT 1 FROM dsr_location_vault_maps m
                  WHERE m.is_active AND m.vendor_id = b.vendor_id AND m.vendor_vault_id = v.id)
)
SELECT vd.id AS vendor_id, vd.name AS vendor_name,
       b.id AS vendor_branch_id, b.branch_code, b.branch_name,
       v.id AS vendor_vault_id, v.vault_code,
       NOT EXISTS (SELECT 1 FROM counted_vaults cv WHERE cv.vendor_branch_id = b.id) AS branch_excluded
FROM vendor_vaults v
JOIN vendor_branches b ON b.id = v.vendor_branch_id
JOIN vendors vd ON vd.id = b.vendor_id AND vd.kind = 'FLM_VENDOR'
WHERE v.is_active
  AND (v.created_at AT TIME ZONE 'Asia/Jakarta')::date <= (SELECT max(report_date) FROM days)
  AND EXISTS (SELECT 1 FROM days dy
              WHERE (b.created_at AT TIME ZONE 'Asia/Jakarta')::date <= dy.report_date
                AND (b.is_active OR (b.deleted_at AT TIME ZONE 'Asia/Jakarta')::date > dy.report_date))
  AND NOT EXISTS (SELECT 1 FROM dsr_location_vault_maps m
                  WHERE m.is_active AND m.vendor_id = b.vendor_id AND m.vendor_vault_id = v.id)
  AND (sqlc.narg('vendor_id')::bigint IS NULL OR vd.id = sqlc.narg('vendor_id')::bigint)
ORDER BY vd.name, b.branch_code, v.vault_code, v.id;

-- name: ListDsrLateUnmappedLabels :many
-- Peringatan FR3.4(b): receipt labels (resolved vendor, report day in range) with no
-- active DSR location map for that vendor. Counted for no branch (FR3.2).
WITH days AS (
    SELECT d::date AS report_date
    FROM generate_series(
        GREATEST(sqlc.arg('month_start')::date, sqlc.arg('start_date')::date),
        LEAST((sqlc.arg('month_start')::date + interval '1 month - 1 day')::date,
              (sqlc.arg('as_of')::timestamptz AT TIME ZONE 'Asia/Jakarta')::date),
        interval '1 day') AS d
)
SELECT vd.id AS vendor_id, vd.name AS vendor_name, l.label::text AS dsr_location,
       count(DISTINCT r.id)::int AS receipt_count,
       min(r.report_date)::date AS first_report_date,
       max(r.report_date)::date AS last_report_date
FROM dsr_receipts r
JOIN days dy ON dy.report_date = r.report_date
JOIN vendors vd ON vd.id = r.vendor_id
CROSS JOIN LATERAL unnest(r.locations) AS l(label)
WHERE NOT EXISTS (SELECT 1 FROM dsr_location_vault_maps m
                  WHERE m.is_active AND m.vendor_id = r.vendor_id AND lower(m.dsr_location) = lower(l.label))
  AND (sqlc.narg('vendor_id')::bigint IS NULL OR vd.id = sqlc.narg('vendor_id')::bigint)
GROUP BY vd.id, vd.name, l.label
ORDER BY vd.name, l.label;

-- name: ListDsrLateUnresolvedVendors :many
-- Peringatan FR3.4(c) / FR1.4: receipts in range whose vendor label matched no vendors.name.
-- Not filtered by vendor_id: these receipts have none.
WITH days AS (
    SELECT d::date AS report_date
    FROM generate_series(
        GREATEST(sqlc.arg('month_start')::date, sqlc.arg('start_date')::date),
        LEAST((sqlc.arg('month_start')::date + interval '1 month - 1 day')::date,
              (sqlc.arg('as_of')::timestamptz AT TIME ZONE 'Asia/Jakarta')::date),
        interval '1 day') AS d
)
SELECT r.vendor_label,
       count(*)::int AS receipt_count,
       min(r.report_date)::date AS first_report_date,
       max(r.report_date)::date AS last_report_date
FROM dsr_receipts r
JOIN days dy ON dy.report_date = r.report_date
WHERE r.vendor_id IS NULL
GROUP BY r.vendor_label
ORDER BY r.vendor_label;
