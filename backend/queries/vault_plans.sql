-- Penetapan vault per ATM oleh ACM (cit-acm-plan). One plan per
-- (vendor_request, acm_area); an ATM belongs to the area of its replenish
-- branch = the active kelolaan branch on the ticket's replenish_date.

-- name: CreateMissingVaultPlans :many
-- FR4.1 / FR7.3: insert a draft plan for every (request in vault_assignment,
-- area) pair that has an ATM but no plan yet. request_id NULL = all such
-- requests (after an ADMIN assigns branches). Returns only newly created rows.
INSERT INTO vendor_request_vault_plans (vendor_request_id, acm_area_id)
SELECT DISTINCT vr.id, ab.acm_area_id
FROM vendor_requests vr
JOIN vendor_request_tickets t ON t.request_number = vr.request_number AND t.is_active
JOIN atms a ON a.terminal_id = t.terminal_id
JOIN LATERAL (
    SELECT COALESCE(p.vendor_branch_id, avp.vendor_branch_id) AS vendor_branch_id
    FROM atm_vendor_packages avp
    LEFT JOIN vendor_packages_branch p ON p.id = avp.vendor_package_id
    WHERE avp.atm_id = a.id AND avp.is_active
      AND avp.effective_start_date <= t.replenish_date
      AND (avp.effective_end_date IS NULL OR avp.effective_end_date >= t.replenish_date)
    ORDER BY avp.effective_start_date DESC, avp.id DESC
    LIMIT 1
) k ON true
JOIN acm_area_branches ab ON ab.vendor_branch_id = k.vendor_branch_id
JOIN acm_areas ar ON ar.id = ab.acm_area_id AND ar.is_active
WHERE vr.status = 'vault_assignment'
  AND (sqlc.narg('request_id')::bigint IS NULL OR vr.id = sqlc.narg('request_id')::bigint)
ON CONFLICT (vendor_request_id, acm_area_id) DO NOTHING
RETURNING id, vendor_request_id, acm_area_id;

-- name: ListAcmAreaMemberIDsByRole :many
-- Notification recipients: active members of the given areas holding role.
SELECT DISTINCT m.user_id
FROM acm_area_members m
JOIN users u ON u.id = m.user_id AND u.is_active AND u.deleted_at IS NULL
JOIN roles r ON r.id = u.role_id
WHERE m.acm_area_id = ANY(sqlc.arg('area_ids')::bigint[]) AND r.role = sqlc.arg('role')::text;

-- name: CancelVaultPlansForRequest :exec
-- FR5.5: cancelling the request cancels every area plan.
UPDATE vendor_request_vault_plans SET status = 'cancelled'
WHERE vendor_request_id = $1 AND status <> 'cancelled';

-- ---------------------------------------------------------------------------
-- S6: rencana vault (read, saldo/kapasitas, penetapan, transisi)
-- ---------------------------------------------------------------------------

-- name: GetVaultPlan :one
-- Plan header with the request and area it belongs to.
SELECT p.id, p.vendor_request_id, p.acm_area_id, p.status, p.submitted_by, p.submitted_at,
       p.approved_by, p.approved_at, p.rejected_by, p.rejected_at, p.rejection_reason,
       vr.request_number, vr.replenish_date, vr.vendor_id, vr.status AS request_status,
       vr.vault_rejection_reason, ar.name AS acm_area_name,
       -- cit-send-vendor FR4.3: the plan went back to draft because a vault vendor rejected it.
       EXISTS (SELECT 1 FROM users ru WHERE ru.id = p.rejected_by AND ru.vendor_id IS NOT NULL) AS rejected_by_vendor
FROM vendor_request_vault_plans p
JOIN vendor_requests vr ON vr.id = p.vendor_request_id
JOIN acm_areas ar ON ar.id = p.acm_area_id
WHERE p.id = $1;

-- name: LockVaultPlan :one
SELECT id, vendor_request_id, acm_area_id, status, submitted_by
FROM vendor_request_vault_plans WHERE id = $1 FOR UPDATE;

-- name: ListVaultPlans :many
-- user_id NULL = every area (ADMIN read-only); else areas the user belongs to.
SELECT p.id, p.vendor_request_id, p.acm_area_id, p.status, p.submitted_at, p.updated_at,
       vr.request_number, vr.replenish_date, vr.status AS request_status, ar.name AS acm_area_name,
       (SELECT COUNT(*) FROM vendor_request_vault_assignments x WHERE x.vault_plan_id = p.id) AS assigned_count,
       (SELECT COUNT(*) FROM vendor_request_vault_assignments x WHERE x.vault_plan_id = p.id AND x.capacity_warning) AS warning_count
FROM vendor_request_vault_plans p
JOIN vendor_requests vr ON vr.id = p.vendor_request_id
JOIN acm_areas ar ON ar.id = p.acm_area_id
WHERE (sqlc.narg('user_id')::bigint IS NULL
       OR EXISTS (SELECT 1 FROM acm_area_members m WHERE m.acm_area_id = p.acm_area_id AND m.user_id = sqlc.narg('user_id')::bigint))
  AND (sqlc.arg('status')::text = '' OR p.status = sqlc.arg('status')::text)
  AND (sqlc.narg('area_id')::bigint IS NULL OR p.acm_area_id = sqlc.narg('area_id')::bigint)
  AND (sqlc.narg('date_from')::date IS NULL OR vr.replenish_date >= sqlc.narg('date_from')::date)
  AND (sqlc.narg('date_to')::date IS NULL OR vr.replenish_date <= sqlc.narg('date_to')::date)
ORDER BY vr.replenish_date DESC NULLS LAST, p.id DESC
LIMIT 500;

-- name: IsAcmAreaMember :one
SELECT EXISTS (SELECT 1 FROM acm_area_members WHERE acm_area_id = $1 AND user_id = $2);

-- name: ListVaultPlanAtms :many
-- ATMs of the plan, with their current assignment. An ATM with an assignment
-- row stays in the plan holding that row even if ADMIN later moves its branch
-- to another area (FR7.3: removing a branch does not change existing plans;
-- review R1). An ATM without any row follows its replenish branch's current
-- area (kelolaan on the replenish date), so a newly assigned branch is picked up.
SELECT DISTINCT ON (i.terminal_id)
       i.terminal_id, vb.id AS replenish_branch_id, vb.branch_code AS replenish_branch_code,
       vb.branch_name AS replenish_branch_name, vb.region_code AS replenish_region_code,
       vb.vendor_id AS replenish_vendor_id, v.name AS replenish_vendor_name,
       asg.id AS assignment_id, asg.vault_branch_id, asg.tier, asg.is_urgent, asg.urgent_reason,
       asg.saldo_snapshot, asg.capacity_snapshot, asg.capacity_warning
FROM vendor_request_vault_plans p
JOIN vendor_requests vr ON vr.id = p.vendor_request_id
JOIN vendor_request_items i ON i.vendor_request_id = vr.id
JOIN atms a ON a.terminal_id = i.terminal_id
JOIN LATERAL (
    SELECT COALESCE(pk.vendor_branch_id, avp.vendor_branch_id) AS vendor_branch_id
    FROM atm_vendor_packages avp
    LEFT JOIN vendor_packages_branch pk ON pk.id = avp.vendor_package_id
    WHERE avp.atm_id = a.id AND avp.is_active
      AND avp.effective_start_date <= vr.replenish_date
      AND (avp.effective_end_date IS NULL OR avp.effective_end_date >= vr.replenish_date)
    ORDER BY avp.effective_start_date DESC, avp.id DESC
    LIMIT 1
) k ON true
JOIN vendor_branches vb ON vb.id = k.vendor_branch_id
JOIN vendors v ON v.id = vb.vendor_id
LEFT JOIN vendor_request_vault_assignments asg ON asg.vault_plan_id = p.id AND asg.terminal_id = i.terminal_id
WHERE p.id = $1
  AND (asg.id IS NOT NULL
       OR (EXISTS (SELECT 1 FROM acm_area_branches ab
                   WHERE ab.vendor_branch_id = k.vendor_branch_id AND ab.acm_area_id = p.acm_area_id)
           AND NOT EXISTS (SELECT 1 FROM vendor_request_vault_assignments o
                           JOIN vendor_request_vault_plans op ON op.id = o.vault_plan_id AND op.status <> 'cancelled'
                           WHERE o.vendor_request_id = vr.id AND o.terminal_id = i.terminal_id AND o.vault_plan_id <> p.id)))
ORDER BY i.terminal_id;

-- name: ListVaultCandidateBranches :many
-- Every branch that can provide cash: active, CASH/ATM_CASH, with a region code.
SELECT vb.id, vb.branch_code, vb.branch_name, vb.region_code::text AS region_code, vb.category,
       vb.vendor_id, v.name AS vendor_name
FROM vendor_branches vb
JOIN vendors v ON v.id = vb.vendor_id
WHERE vb.is_active AND vb.category IN ('CASH', 'ATM_CASH') AND vb.region_code IS NOT NULL
ORDER BY v.name, vb.branch_code;

-- name: ListVaultBranchSaldo :many
-- FR3.1: saldo akhir 00:00 per branch for DSR report_date = X, summed over the
-- branch's active vaults via the DSR location map. unknown = no active vault,
-- or a vault without a mapped saldo_akhir row, or a broken (NULL) denom cell.
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
SELECT b.id AS vendor_branch_id,
       (COUNT(vv.id) = 0 OR COALESCE(bool_or(s.vendor_vault_id IS NULL
            OR s.denom_100k_idr IS NULL OR s.denom_50k_idr IS NULL OR s.denom_20k_idr IS NULL
            OR s.denom_10k_idr IS NULL OR s.denom_5k_idr IS NULL OR s.denom_2k_idr IS NULL
            OR s.denom_1k_idr IS NULL), false))::boolean AS unknown,
       COALESCE(TRUNC(SUM(s.denom_100k_idr)), 0)::bigint AS d100000,
       COALESCE(TRUNC(SUM(s.denom_50k_idr)), 0)::bigint AS d50000,
       COALESCE(TRUNC(SUM(s.denom_20k_idr)), 0)::bigint AS d20000,
       COALESCE(TRUNC(SUM(s.denom_10k_idr)), 0)::bigint AS d10000,
       COALESCE(TRUNC(SUM(s.denom_5k_idr)), 0)::bigint AS d5000,
       COALESCE(TRUNC(SUM(s.denom_2k_idr)), 0)::bigint AS d2000,
       COALESCE(TRUNC(SUM(s.denom_1k_idr)), 0)::bigint AS d1000
FROM vendor_branches b
LEFT JOIN vendor_vaults vv ON vv.vendor_branch_id = b.id AND vv.deleted_at IS NULL
LEFT JOIN saldo_rows s ON s.vendor_vault_id = vv.id
WHERE b.id = ANY(sqlc.arg('branch_ids')::bigint[])
GROUP BY b.id;

-- name: ListVaultDemand :many
-- FR3.2/3.3 input: every ATM order (per denom) of vault-flow requests for
-- replenish date X that are still live, with its replenish branch and, if
-- assigned in a non-cancelled plan, its vault branch.
SELECT vr.id AS vendor_request_id, i.terminal_id, i.denom, SUM(i.amount_replenish)::bigint AS amount,
       k.vendor_branch_id AS replenish_branch_id, asg.vault_branch_id
FROM vendor_requests vr
JOIN vendor_request_items i ON i.vendor_request_id = vr.id
JOIN atms a ON a.terminal_id = i.terminal_id
LEFT JOIN LATERAL (
    SELECT COALESCE(pk.vendor_branch_id, avp.vendor_branch_id) AS vendor_branch_id
    FROM atm_vendor_packages avp
    LEFT JOIN vendor_packages_branch pk ON pk.id = avp.vendor_package_id
    WHERE avp.atm_id = a.id AND avp.is_active
      AND avp.effective_start_date <= vr.replenish_date
      AND (avp.effective_end_date IS NULL OR avp.effective_end_date >= vr.replenish_date)
    ORDER BY avp.effective_start_date DESC, avp.id DESC
    LIMIT 1
) k ON true
LEFT JOIN vendor_request_vault_assignments asg ON asg.vendor_request_id = vr.id AND asg.terminal_id = i.terminal_id
     AND EXISTS (SELECT 1 FROM vendor_request_vault_plans pp WHERE pp.id = asg.vault_plan_id AND pp.status <> 'cancelled')
WHERE vr.replenish_date = sqlc.arg('replenish_date')::date
  AND vr.vault_flow AND NOT vr.is_canceled AND vr.status NOT IN ('cancelled', 'rejected')
GROUP BY vr.id, i.terminal_id, i.denom, k.vendor_branch_id, asg.vault_branch_id;

-- name: ClearVaultPlanAssignments :exec
-- Replace-all while the plan is draft (spec data model: draft rows are replaced, audited).
DELETE FROM vendor_request_vault_assignments WHERE vault_plan_id = $1;

-- name: InsertVaultAssignment :one
INSERT INTO vendor_request_vault_assignments
    (vault_plan_id, vendor_request_id, terminal_id, replenish_branch_id, vault_branch_id, tier,
     is_urgent, urgent_reason)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id;

-- name: UpdateVaultAssignmentSnapshot :exec
UPDATE vendor_request_vault_assignments
SET saldo_snapshot = $2, capacity_snapshot = $3, capacity_warning = $4
WHERE id = $1;

-- name: SetVaultPlanStatus :exec
-- One query per plan transition; stamps only the columns of the target status.
UPDATE vendor_request_vault_plans
SET status = sqlc.arg('status')::text,
    submitted_by = CASE WHEN sqlc.arg('status')::text = 'pending_acm_approval' THEN sqlc.narg('actor_id')::bigint ELSE submitted_by END,
    submitted_at = CASE WHEN sqlc.arg('status')::text = 'pending_acm_approval' THEN now() ELSE submitted_at END,
    approved_by  = CASE WHEN sqlc.arg('status')::text = 'acm_approved' THEN sqlc.narg('actor_id')::bigint
                        WHEN sqlc.arg('status')::text = 'draft' THEN NULL ELSE approved_by END,
    approved_at  = CASE WHEN sqlc.arg('status')::text = 'acm_approved' THEN now()
                        WHEN sqlc.arg('status')::text = 'draft' THEN NULL ELSE approved_at END,
    rejected_by  = CASE WHEN sqlc.narg('reason')::text IS NOT NULL THEN sqlc.narg('actor_id')::bigint ELSE rejected_by END,
    rejected_at  = CASE WHEN sqlc.narg('reason')::text IS NOT NULL THEN now() ELSE rejected_at END,
    rejection_reason = CASE WHEN sqlc.narg('reason')::text IS NOT NULL THEN sqlc.narg('reason')::text ELSE rejection_reason END
WHERE id = sqlc.arg('id')::bigint;

-- name: ListAllVaultPlansForRequest :many
-- cit-send-vendor FR6.2: re-approval after an edit reopens every plan, including ones cancelled because
-- an earlier edit left their area without ATMs (the unique (request, area) blocks recreating them).
SELECT id, acm_area_id, status FROM vendor_request_vault_plans WHERE vendor_request_id = $1 ORDER BY id;

-- name: ListVaultPlansForRequest :many
SELECT p.id, p.acm_area_id, ar.name AS acm_area_name, p.status, p.submitted_by, p.approved_by,
       p.rejection_reason
FROM vendor_request_vault_plans p JOIN acm_areas ar ON ar.id = p.acm_area_id
WHERE p.vendor_request_id = $1 AND p.status <> 'cancelled'
ORDER BY ar.name;

-- name: ListVaultAssignmentsForRequest :many
-- ATM-SPV review panel (FR5.1): every assignment of the request's live plans.
SELECT x.vault_plan_id, x.terminal_id, x.replenish_branch_id, rb.branch_code AS replenish_branch_code,
       x.vault_branch_id, vb.branch_code AS vault_branch_code, vb.branch_name AS vault_branch_name,
       v.name AS vault_vendor_name, x.tier, x.is_urgent, x.urgent_reason,
       x.saldo_snapshot, x.capacity_snapshot, x.capacity_warning
FROM vendor_request_vault_assignments x
JOIN vendor_request_vault_plans p ON p.id = x.vault_plan_id AND p.status <> 'cancelled'
JOIN vendor_branches rb ON rb.id = x.replenish_branch_id
JOIN vendor_branches vb ON vb.id = x.vault_branch_id
JOIN vendors v ON v.id = vb.vendor_id
WHERE x.vendor_request_id = $1
ORDER BY x.terminal_id;

-- name: CountRequestAtmsWithoutApprovedVault :one
-- FR4.6 gate: ATMs of the request not yet covered by an acm_approved plan
-- (includes ATMs whose branch has no area -> no plan at all).
SELECT COUNT(DISTINCT i.terminal_id)
FROM vendor_request_items i
WHERE i.vendor_request_id = $1
  AND NOT EXISTS (
        SELECT 1 FROM vendor_request_vault_assignments x
        JOIN vendor_request_vault_plans p ON p.id = x.vault_plan_id AND p.status = 'acm_approved'
        WHERE x.vendor_request_id = i.vendor_request_id AND x.terminal_id = i.terminal_id);

-- name: DeleteVaultAssignmentsOfRemovedAtms :many
-- cit-send-vendor FR6.2: an edit (draft only) removed ATMs from the request; their vault assignments go
-- (draft rows, same rule as replace-all in 2.2a S1; caller audits). ATMs still in the request keep theirs.
DELETE FROM vendor_request_vault_assignments a
WHERE a.vendor_request_id = $1
  AND NOT EXISTS (SELECT 1 FROM vendor_request_items i WHERE i.vendor_request_id = a.vendor_request_id AND i.terminal_id = a.terminal_id)
RETURNING a.terminal_id;

-- name: ResetVaultPlansToDraft :exec
-- FR5.2: ATM-SPV rejects the recommendation -> every area plan back to draft.
UPDATE vendor_request_vault_plans
SET status = 'draft', approved_by = NULL, approved_at = NULL
WHERE vendor_request_id = $1 AND status <> 'cancelled';

-- name: SetVendorRequestVaultReview :exec
-- vault_review -> sent_to_vendor (cit-send-vendor S1; ready = legacy) | vault_assignment by the ATM-SPV (FR5.2).
UPDATE vendor_requests
SET status = sqlc.arg('status')::text,
    vault_reviewed_by = sqlc.arg('actor_id')::bigint,
    vault_reviewed_at = now(),
    vault_rejection_reason = sqlc.narg('reason')::text,
    vendor_sent = vendor_sent OR sqlc.arg('status')::text = 'sent_to_vendor',
    sent_at = CASE WHEN sqlc.arg('status')::text = 'sent_to_vendor' THEN now() ELSE sent_at END
WHERE id = sqlc.arg('id')::bigint;

-- name: SetVendorRequestStatusOnly :exec
-- System step vault_assignment -> vault_review (FR4.6); nothing else stamped.
UPDATE vendor_requests SET status = sqlc.arg('status')::text WHERE id = sqlc.arg('id')::bigint;
