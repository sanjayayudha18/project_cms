-- Pemenuhan vault per request (cit-vault-select spec v3): satu rencana per
-- request dipegang area pemilik region request; ACM membagi nominal per denom
-- ke satu atau lebih vault. All money is whole IDR (bigint), never float.

-- name: CreateMissingVaultPlansByRegion :many
-- FR2.1 / FR1.3: a draft plan for every request in vault_assignment whose
-- region belongs to an active area and that has no live plan yet.
-- request_id / region_code NULL = no filter. Returns only newly created rows.
INSERT INTO vendor_request_vault_plans (vendor_request_id, acm_area_id)
SELECT vr.id, ar.acm_area_id
FROM vendor_requests vr
JOIN acm_area_regions ar ON ar.region_code = vr.region_code
JOIN acm_areas a ON a.id = ar.acm_area_id AND a.is_active
WHERE vr.status = 'vault_assignment'
  AND (sqlc.narg('request_id')::bigint IS NULL OR vr.id = sqlc.narg('request_id')::bigint)
  AND (sqlc.narg('region_code')::text IS NULL OR vr.region_code = sqlc.narg('region_code')::text)
  AND NOT EXISTS (SELECT 1 FROM vendor_request_vault_plans p
                  WHERE p.vendor_request_id = vr.id AND p.status <> 'cancelled')
ON CONFLICT DO NOTHING
RETURNING id, vendor_request_id, acm_area_id;

-- name: ListRequestOrderTotals :many
-- FR2.3: total order per denom of the request (the target the allocation must cover).
SELECT denom, SUM(amount_replenish)::bigint AS amount
FROM vendor_request_items
WHERE vendor_request_id = $1
GROUP BY denom
ORDER BY denom DESC;

-- name: ListVaultCandidateVaults :many
-- FR3.1: eligible vaults (active, CASH/ATM_CASH) of active CASH/ATM_CASH
-- branches with a region code. vault_ids NULL = all; else only those (validation).
SELECT vv.id AS vendor_vault_id, vv.vault_code, vv.category,
       vb.id AS vendor_branch_id, vb.branch_code, vb.branch_name, vb.region_code::text AS region_code,
       vb.vendor_id, v.name AS vendor_name
FROM vendor_vaults vv
JOIN vendor_branches vb ON vb.id = vv.vendor_branch_id
JOIN vendors v ON v.id = vb.vendor_id
WHERE vv.is_active AND vv.deleted_at IS NULL AND vv.category IN ('CASH', 'ATM_CASH')
  AND vb.is_active AND vb.category IN ('CASH', 'ATM_CASH') AND vb.region_code IS NOT NULL
  AND (sqlc.narg('vault_ids')::bigint[] IS NULL OR vv.id = ANY(sqlc.narg('vault_ids')::bigint[]))
ORDER BY v.name, vb.branch_code, vv.vault_code;

-- name: ListVaultSaldo :many
-- FR5.1: saldo akhir 00:00 per vault for DSR report_date = X, summed over the
-- DSR locations mapped to it. unknown = no mapped saldo_akhir row, or a broken
-- (NULL) denom cell.
-- ponytail: amounts truncated to whole rupiah (DSR holds banknote totals, never cents).
WITH saldo_rows AS (
    SELECT m.vendor_vault_id, r.denom_100k_idr, r.denom_50k_idr, r.denom_20k_idr, r.denom_10k_idr,
           r.denom_5k_idr, r.denom_2k_idr, r.denom_1k_idr
    FROM dsr_uploads u
    JOIN vendors vd ON vd.name = u.vendor
    JOIN dsr_daily_rows r ON r.upload_id = u.id AND r.flow = 'saldo_akhir' AND r.section = 'd0'
    JOIN dsr_location_vault_maps m ON m.vendor_id = vd.id AND m.is_active
         AND lower(m.dsr_location) = lower(btrim(r.location))
    WHERE u.report_date = sqlc.arg('report_date')::date AND u.daily_status = 'completed'
)
SELECT vv.id AS vendor_vault_id,
       (COUNT(s.vendor_vault_id) = 0 OR COALESCE(bool_or(
               s.denom_100k_idr IS NULL OR s.denom_50k_idr IS NULL OR s.denom_20k_idr IS NULL
            OR s.denom_10k_idr IS NULL OR s.denom_5k_idr IS NULL OR s.denom_2k_idr IS NULL
            OR s.denom_1k_idr IS NULL), false))::boolean AS unknown,
       COALESCE(TRUNC(SUM(s.denom_100k_idr)), 0)::bigint AS d100000,
       COALESCE(TRUNC(SUM(s.denom_50k_idr)), 0)::bigint AS d50000,
       COALESCE(TRUNC(SUM(s.denom_20k_idr)), 0)::bigint AS d20000,
       COALESCE(TRUNC(SUM(s.denom_10k_idr)), 0)::bigint AS d10000,
       COALESCE(TRUNC(SUM(s.denom_5k_idr)), 0)::bigint AS d5000,
       COALESCE(TRUNC(SUM(s.denom_2k_idr)), 0)::bigint AS d2000,
       COALESCE(TRUNC(SUM(s.denom_1k_idr)), 0)::bigint AS d1000
FROM vendor_vaults vv
LEFT JOIN saldo_rows s ON s.vendor_vault_id = vv.id
WHERE vv.id = ANY(sqlc.arg('vault_ids')::bigint[])
GROUP BY vv.id;

-- name: ListVaultAllocatedTotals :many
-- FR5.2: amount already allocated per (vault, denom) on replenish date X by
-- live vault-flow requests (status rule = cit-acm-plan FR3.2 revisi 2026-10-08),
-- excluding the plan being edited (FR5.3). Unallocated orders do not count (V8).
SELECT a.vendor_vault_id, m.denom, SUM(m.amount)::bigint AS amount
FROM vendor_request_vault_allocations a
JOIN vendor_request_vault_allocation_amounts m ON m.allocation_id = a.id
JOIN vendor_request_vault_plans p ON p.id = a.vault_plan_id AND p.status <> 'cancelled'
JOIN vendor_requests vr ON vr.id = a.vendor_request_id
WHERE vr.replenish_date = sqlc.arg('replenish_date')::date
  AND vr.vault_flow AND NOT vr.is_canceled AND vr.status NOT IN ('cancelled', 'rejected')
  AND a.vault_plan_id <> sqlc.arg('exclude_plan_id')::bigint
GROUP BY a.vendor_vault_id, m.denom;

-- name: ClearVaultPlanAllocations :exec
-- Replace-all while the plan is draft (amounts cascade; audited in service).
DELETE FROM vendor_request_vault_allocations WHERE vault_plan_id = $1;

-- name: InsertVaultAllocation :one
INSERT INTO vendor_request_vault_allocations
    (vault_plan_id, vendor_request_id, vendor_vault_id, vault_branch_id, tier, is_urgent, urgent_reason)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id;

-- name: InsertVaultAllocationAmount :exec
INSERT INTO vendor_request_vault_allocation_amounts (allocation_id, denom, amount) VALUES ($1, $2, $3);

-- name: UpdateVaultAllocationSnapshot :exec
UPDATE vendor_request_vault_allocations
SET saldo_snapshot = $2, capacity_snapshot = $3, capacity_warning = $4
WHERE id = $1;

-- name: SetVaultPlanBufferReason :exec
UPDATE vendor_request_vault_plans SET buffer_reason = $2 WHERE id = $1;

-- name: ListVaultPlanAllocations :many
-- FR9.1/FR9.2: allocation rows of a plan with vault, branch and active ticket.
SELECT a.id, a.vault_plan_id, a.vendor_vault_id, vv.vault_code, vv.category AS vault_category,
       a.vault_branch_id, vb.branch_code, vb.branch_name, vb.region_code AS branch_region_code,
       vb.vendor_id, v.name AS vendor_name, a.tier, a.is_urgent, a.urgent_reason,
       a.saldo_snapshot, a.capacity_snapshot, a.capacity_warning, t.ticket_number
FROM vendor_request_vault_allocations a
JOIN vendor_vaults vv ON vv.id = a.vendor_vault_id
JOIN vendor_branches vb ON vb.id = a.vault_branch_id
JOIN vendors v ON v.id = vb.vendor_id
LEFT JOIN vendor_request_vault_tickets t
       ON t.vendor_request_id = a.vendor_request_id AND t.vendor_vault_id = a.vendor_vault_id AND t.is_active
WHERE a.vault_plan_id = $1
ORDER BY a.tier, vb.branch_code, vv.vault_code;

-- name: ListVaultPlanAllocationAmounts :many
SELECT m.allocation_id, m.denom, m.amount
FROM vendor_request_vault_allocation_amounts m
JOIN vendor_request_vault_allocations a ON a.id = m.allocation_id
WHERE a.vault_plan_id = $1
ORDER BY m.allocation_id, m.denom DESC;
