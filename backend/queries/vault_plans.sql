-- Rencana pemenuhan vault (cit-acm-plan, revised by cit-vault-select v3):
-- one live plan per request, held by the ACM area of the request's region.
-- Allocation rows / tickets: queries/vault_allocations.sql, vault_tickets.sql.

-- name: ListAcmAreaMemberIDsByRole :many
-- Notification recipients: active members of the given areas holding role.
SELECT DISTINCT m.user_id
FROM acm_area_members m
JOIN users u ON u.id = m.user_id AND u.is_active AND u.deleted_at IS NULL
JOIN roles r ON r.id = u.role_id
WHERE m.acm_area_id = ANY(sqlc.arg('area_ids')::bigint[]) AND r.role = sqlc.arg('role')::text;

-- name: CancelVaultPlansForRequest :exec
-- FR5.5: cancelling the request cancels its plan.
UPDATE vendor_request_vault_plans SET status = 'cancelled'
WHERE vendor_request_id = $1 AND status <> 'cancelled';

-- ---------------------------------------------------------------------------
-- Rencana vault (read, transisi)
-- ---------------------------------------------------------------------------

-- name: GetVaultPlan :one
-- Plan header with the request and area it belongs to.
SELECT p.id, p.vendor_request_id, p.acm_area_id, p.status, p.submitted_by, p.submitted_at,
       p.approved_by, p.approved_at, p.rejected_by, p.rejected_at, p.rejection_reason,
       p.buffer_reason, vr.request_number, vr.replenish_date, vr.vendor_id, vr.region_code,
       vr.status AS request_status, vr.vault_rejection_reason, ar.name AS acm_area_name,
       -- cit-send-vendor FR4.3: the plan went back to draft because a vault vendor rejected it.
       EXISTS (SELECT 1 FROM users ru WHERE ru.id = p.rejected_by AND ru.vendor_id IS NOT NULL) AS rejected_by_vendor
FROM vendor_request_vault_plans p
JOIN vendor_requests vr ON vr.id = p.vendor_request_id
JOIN acm_areas ar ON ar.id = p.acm_area_id
WHERE p.id = $1;

-- name: LockVaultPlan :one
SELECT id, vendor_request_id, acm_area_id, status, submitted_by, buffer_reason
FROM vendor_request_vault_plans WHERE id = $1 FOR UPDATE;

-- name: ListVaultPlans :many
-- user_id NULL = every area (ADMIN read-only); else areas the user belongs to.
SELECT p.id, p.vendor_request_id, p.acm_area_id, p.status, p.submitted_at, p.updated_at,
       vr.request_number, vr.replenish_date, vr.status AS request_status, ar.name AS acm_area_name,
       (SELECT COUNT(*) FROM vendor_request_vault_allocations x WHERE x.vault_plan_id = p.id) AS allocation_count,
       (SELECT COUNT(*) FROM vendor_request_vault_allocations x WHERE x.vault_plan_id = p.id AND x.capacity_warning) AS warning_count
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
-- cit-vault-select FR2.3: the request's ATMs (read-only context for the
-- allocation) with their replenish branch = active kelolaan on the replenish date.
SELECT DISTINCT ON (i.terminal_id)
       i.terminal_id, vb.id AS replenish_branch_id, vb.branch_code AS replenish_branch_code,
       vb.branch_name AS replenish_branch_name, vb.region_code AS replenish_region_code,
       vb.vendor_id AS replenish_vendor_id, v.name AS replenish_vendor_name
FROM vendor_requests vr
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
WHERE vr.id = $1
ORDER BY i.terminal_id;

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

-- name: ListVaultPlansForRequest :many
-- The live plan of a request (at most one, FR2.2).
SELECT p.id, p.acm_area_id, ar.name AS acm_area_name, p.status, p.submitted_by, p.approved_by,
       p.rejection_reason
FROM vendor_request_vault_plans p JOIN acm_areas ar ON ar.id = p.acm_area_id
WHERE p.vendor_request_id = $1 AND p.status <> 'cancelled'
ORDER BY ar.name;

-- name: ResetVaultPlansToDraft :exec
-- FR5.2 / cit-send-vendor FR6.2: ATM-SPV rejects, or the request is re-approved after an edit -> plan back to draft.
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
