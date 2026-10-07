-- atm-visit-quota (.claude/sdlc/atm-visit-quota/spec.md): kuota kunjungan
-- replenish per ATM. Kuota = package_frequencies.cr_frequency (read-only) of
-- the ATM's active package; sisa lives in atm_visit_quotas, each counted
-- visit in atm_visits.

-- name: ResolveAtmQuota :one
-- One terminal's atm id + active package (same LATERAL + avp.id DESC
-- tie-breaker as ListForecastForDate) + its cr_frequency. package_code /
-- cr_frequency are NULL when there is no active package, the ATM has no
-- price_machine_group, or the label has no package_frequencies row: the
-- caller treats that as "kuota tidak diketahui", never a guess.
SELECT a.id AS atm_id,
       a.terminal_id,
       pkg.package_code AS package_code,
       active_pkg.package AS vendor_package_label, -- vendor-wide label (migration 023); NULL for a branch package
       pf.cr_frequency  AS cr_frequency
FROM atms a
LEFT JOIN LATERAL (
    SELECT avp.vendor_package_id, avp.package AS package, avp.vendor_branch_id AS vendor_branch_id
    FROM atm_vendor_packages avp
    WHERE avp.atm_id = a.id
      AND avp.is_active = true
      AND avp.effective_start_date <= sqlc.arg('as_of_date')::date
      AND (avp.effective_end_date IS NULL OR avp.effective_end_date >= sqlc.arg('as_of_date')::date)
    ORDER BY avp.effective_start_date DESC, avp.id DESC
    LIMIT 1
) active_pkg ON true
LEFT JOIN vendor_packages_branch pkg ON pkg.id = active_pkg.vendor_package_id
LEFT JOIN package_frequencies pf
       ON pf.package_code = COALESCE(pkg.package_code, active_pkg.package) AND pf.machine_group = a.price_machine_group
WHERE a.terminal_id = sqlc.arg('terminal_id')::text;

-- name: GetAtmVisitQuotaForUpdate :one
-- Row lock: serializes concurrent visits/resets/cancels on one ATM so the
-- remaining counter never loses an update.
SELECT * FROM atm_visit_quotas WHERE atm_id = $1 FOR UPDATE;

-- name: EnsureAtmVisitQuota :exec
-- First visit of an ATM with a known kuota creates its row full
-- (remaining = quota_total, reset_by NULL = otomatis); the caller then locks
-- and decrements it like any other row. DO NOTHING keeps an existing row
-- (and its snapshot) untouched, and two concurrent first visits can't fail
-- on the primary key.
INSERT INTO atm_visit_quotas (atm_id, package_code, quota_total, remaining, reset_at)
VALUES (sqlc.arg('atm_id')::bigint, sqlc.arg('package_code')::text, sqlc.arg('quota_total')::int,
        sqlc.arg('quota_total')::int, clock_timestamp())
ON CONFLICT (atm_id) DO NOTHING;

-- name: AdjustAtmVisitQuota :one
-- delta = -1 for a visit, +1 for a cancelled visit.
UPDATE atm_visit_quotas
SET remaining = remaining + sqlc.arg('delta')::int
WHERE atm_id = sqlc.arg('atm_id')::bigint
RETURNING *;

-- name: ResetAtmVisitQuota :one
-- reset_at / atm_visits.created_at use clock_timestamp(), not now(): they
-- define the period boundary (visit belongs to the period iff created_at >=
-- reset_at), so they must order correctly even within one transaction.
INSERT INTO atm_visit_quotas (atm_id, package_code, quota_total, remaining, reset_at, reset_by)
VALUES (sqlc.arg('atm_id')::bigint, sqlc.arg('package_code')::text, sqlc.arg('quota_total')::int,
        sqlc.arg('quota_total')::int, clock_timestamp(), sqlc.arg('reset_by')::bigint)
ON CONFLICT (atm_id) DO UPDATE
SET package_code = EXCLUDED.package_code,
    quota_total  = EXCLUDED.quota_total,
    remaining    = EXCLUDED.quota_total,
    reset_at     = clock_timestamp(),
    reset_by     = EXCLUDED.reset_by
RETURNING *;

-- name: InsertAtmVisit :one
-- ON CONFLICT DO NOTHING: a second approve of the same request for the same
-- ATM returns no row (pgx.ErrNoRows) and must not touch the quota (FR3.1).
INSERT INTO atm_visits (atm_id, vendor_request_id, quota_known, created_at)
VALUES (sqlc.arg('atm_id')::bigint, sqlc.arg('vendor_request_id')::bigint, sqlc.arg('quota_known')::boolean, clock_timestamp())
ON CONFLICT (vendor_request_id, atm_id) DO NOTHING
RETURNING *;

-- name: MarkAtmVisitOverQuota :exec
UPDATE atm_visits SET is_over_quota = true WHERE id = $1;

-- name: GetAtmVisitForUpdate :one
SELECT * FROM atm_visits WHERE id = $1 FOR UPDATE;

-- name: CancelAtmVisit :one
UPDATE atm_visits
SET cancelled_at = now(),
    cancelled_by = sqlc.arg('cancelled_by')::bigint,
    cancel_reason = sqlc.arg('cancel_reason')::text
WHERE id = sqlc.arg('id')::bigint AND cancelled_at IS NULL
RETURNING *;

-- name: GetAtmVisitQuotaByAtm :one
SELECT q.*, u.full_name AS reset_by_name
FROM atm_visit_quotas q
LEFT JOIN users u ON u.id = q.reset_by
WHERE q.atm_id = $1;

-- name: ListAtmVisitsSince :many
-- Visits of the current period (since the last reset), newest first,
-- cancelled ones included so the screen can show them struck through.
SELECT av.id, av.vendor_request_id, vr.request_number, av.quota_known, av.is_over_quota,
       av.created_at, av.cancelled_at, av.cancel_reason, cu.full_name AS cancelled_by_name
FROM atm_visits av
JOIN vendor_requests vr ON vr.id = av.vendor_request_id
LEFT JOIN users cu ON cu.id = av.cancelled_by
WHERE av.atm_id = sqlc.arg('atm_id')::bigint
  AND av.created_at >= sqlc.arg('since')::timestamptz
ORDER BY av.created_at DESC, av.id DESC;

-- name: ListAtmsForVendorQuotaReset :many
-- Every ATM whose active package belongs to the vendor, with its kuota
-- (cr_frequency NULL = tidak diketahui -> skipped by the caller).
SELECT a.id AS atm_id,
       a.terminal_id,
       pkg.package_code AS package_code,
       active_pkg.package AS vendor_package_label, -- vendor-wide label (migration 023); NULL for a branch package
       pf.cr_frequency  AS cr_frequency
FROM atms a
JOIN LATERAL (
    SELECT avp.vendor_package_id, avp.package AS package, avp.vendor_branch_id AS vendor_branch_id
    FROM atm_vendor_packages avp
    WHERE avp.atm_id = a.id
      AND avp.is_active = true
      AND avp.effective_start_date <= sqlc.arg('as_of_date')::date
      AND (avp.effective_end_date IS NULL OR avp.effective_end_date >= sqlc.arg('as_of_date')::date)
    ORDER BY avp.effective_start_date DESC, avp.id DESC
    LIMIT 1
) active_pkg ON true
LEFT JOIN vendor_packages_branch pkg ON pkg.id = active_pkg.vendor_package_id
JOIN vendor_branches vb ON vb.id = COALESCE(pkg.vendor_branch_id, active_pkg.vendor_branch_id)
LEFT JOIN package_frequencies pf
       ON pf.package_code = COALESCE(pkg.package_code, active_pkg.package) AND pf.machine_group = a.price_machine_group
WHERE vb.vendor_id = sqlc.arg('vendor_id')::bigint
ORDER BY a.terminal_id;
