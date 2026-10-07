-- name: GetUserApprovalInfo :one
-- Reads a user's position in the reporting-line hierarchy for chain walking.
SELECT id, supervisor_id, approval_level FROM users WHERE id = $1;

-- name: FindActiveLeave :one
-- Returns one row if the user has a leave window covering `at`, else pgx.ErrNoRows.
-- Named args (user_id, at) so sqlc generates FindActiveLeaveParams{UserID, At}
-- matching internal/approval/repository.go's IsOnLeave call site.
SELECT id FROM user_leaves
WHERE user_id = sqlc.arg('user_id') AND start_at <= sqlc.arg('at') AND end_at > sqlc.arg('at')
LIMIT 1;

-- name: FindActiveDelegate :one
-- Returns the delegate covering `at` for the given approver, else pgx.ErrNoRows.
-- Most recently started delegation wins if more than one somehow overlaps.
-- Named args (from_user_id, at) so sqlc generates FindActiveDelegateParams{FromUserID, At}
-- matching internal/approval/repository.go's FindActiveDelegate call site.
SELECT to_user_id FROM approval_delegations
WHERE from_user_id = sqlc.arg('from_user_id') AND start_at <= sqlc.arg('at') AND end_at > sqlc.arg('at')
ORDER BY start_at DESC
LIMIT 1;

-- name: FindApprovalPolicy :one
-- Resolves (document_type, amount) -> required_level. is_active + the
-- non-overlap constraint on approval_policies (022) guarantee at most one row.
-- Named args (document_type, amount) so sqlc generates
-- FindApprovalPolicyParams{DocumentType, Amount} matching
-- internal/approval/repository.go's FindPolicy call site.
SELECT required_level FROM approval_policies
WHERE document_type = sqlc.arg('document_type')
  AND sqlc.arg('amount') >= min_amount AND sqlc.arg('amount') < max_amount AND is_active
LIMIT 1;

-- name: FindApprovalRequestByDocument :one
-- Idempotency check for submit: an existing row for (document_type, document_id)
-- means "already submitted", not "submit again".
SELECT * FROM approval_requests WHERE document_type = $1 AND document_id = $2;

-- name: CreateApprovalRequest :one
INSERT INTO approval_requests (maker_id, document_type, document_id, amount, required_level, status)
VALUES ($1, $2, $3, $4, $5, 'pending')
RETURNING *;

-- name: GetApprovalRequest :one
SELECT * FROM approval_requests WHERE id = $1;

-- name: UpdateApprovalRequestStatus :one
UPDATE approval_requests SET status = $2, updated_at = now() WHERE id = $1
RETURNING *;

-- name: CreateApprovalStep :one
INSERT INTO approval_steps (request_id, step_level, assigned_approver_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListApprovalSteps :many
SELECT * FROM approval_steps WHERE request_id = $1 ORDER BY step_level ASC;

-- name: FindPendingApprovalStep :one
-- The lowest-level step still pending is the one currently active — steps are
-- approved strictly in step_level order.
SELECT * FROM approval_steps
WHERE request_id = $1 AND status = 'pending'
ORDER BY step_level ASC
LIMIT 1;

-- name: UpdateApprovalStepStatus :one
UPDATE approval_steps SET status = $2, acted_by_id = $3, acted_at = $4 WHERE id = $1
RETURNING *;

-- name: UpdateUserHierarchy :one
-- Admin-only: set a user's supervisor_id / approval_level (RBAC-Setup Task 8).
UPDATE users SET supervisor_id = $2, approval_level = $3, updated_at = now()
WHERE id = $1
RETURNING id, supervisor_id, approval_level;

-- name: CreateApprovalDelegation :one
-- Rejected by approval_delegations_no_overlap (025) if the range overlaps an
-- existing delegation for the same from_user_id.
INSERT INTO approval_delegations (from_user_id, to_user_id, start_at, end_at, reason)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: RevokeApprovalDelegation :one
-- "Cabut": shortens end_at to now instead of deleting, so the delegation's
-- history stays auditable. No-op (0 rows) if it already ended.
UPDATE approval_delegations SET end_at = $2
WHERE id = $1 AND end_at > $2
RETURNING *;

-- name: CreateUserLeave :one
-- sqlc v1.31.1's inflector mis-singularizes "user_leaves" -> "UserLeafe"
-- (wrongly applies the knife/knives "-fe" rule to the irregular
-- leaf/leaves). After every `sqlc generate`, hand-fix "UserLeafe" ->
-- "UserLeave" in internal/db/models.go and internal/db/approval.sql.go
-- (same drift-revert discipline as the audit.sql.go "IP" casing) —
-- internal/approval/repository.go and admin.go depend on db.UserLeave.
INSERT INTO user_leaves (user_id, start_at, end_at, reason)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListUserHierarchy :many
-- RBAC settings menu: full user hierarchy + role for the read-only admin view.
-- vendor_name is null for internal (LDAP) users, matching users.vendor_id.
SELECT u.id, u.username, u.full_name, u.supervisor_id, u.approval_level, r.role, u.auth_source,
       u.vendor_id, v.name AS vendor_name
FROM users u
JOIN roles r ON r.id = u.role_id
LEFT JOIN vendors v ON v.id = u.vendor_id
ORDER BY u.id;

-- name: ListDelegations :many
-- Active (non-revoked) delegations for the RBAC settings list view.
-- SELECT * (full table columns) so sqlc reuses the shared ApprovalDelegation
-- model type instead of generating a distinct Row type -- RbacReader and
-- internal/approval/repository.go both depend on db.ApprovalDelegation.
SELECT *
FROM approval_delegations
WHERE end_at > now()
ORDER BY start_at DESC;

-- name: ListLeaves :many
-- SELECT * so sqlc reuses db.UserLeave (see ListDelegations comment).
SELECT *
FROM user_leaves
ORDER BY start_at DESC;

-- name: ListApprovalPolicies :many
-- SELECT * so sqlc reuses db.ApprovalPolicy (see ListDelegations comment).
SELECT *
FROM approval_policies
ORDER BY document_type, min_amount;

-- name: CreateApprovalPolicy :one
-- RETURNING * so sqlc reuses db.ApprovalPolicy (see ListDelegations comment).
INSERT INTO approval_policies (document_type, min_amount, max_amount, required_level)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetApprovalPolicy :one
-- Before-state read for the audit trail on update. SELECT * so sqlc reuses
-- db.ApprovalPolicy (see ListDelegations comment).
SELECT * FROM approval_policies WHERE id = $1;

-- name: UpdateApprovalPolicy :one
-- RETURNING * so sqlc reuses db.ApprovalPolicy (see ListDelegations comment).
UPDATE approval_policies
SET document_type = $2, min_amount = $3, max_amount = $4, required_level = $5, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListPendingStepsForApprover :many
-- The caller's inbox: pending steps directly assigned to them on requests
-- that are still pending. Does not follow delegation — a delegate sees their
-- own inbox only, not the absent approver's (out of scope for Task 7).
SELECT s.id AS step_id, s.request_id, s.step_level,
       r.document_type, r.document_id, r.amount, r.maker_id
FROM approval_steps s
JOIN approval_requests r ON r.id = s.request_id
WHERE s.assigned_approver_id = $1 AND s.status = 'pending' AND r.status = 'pending'
ORDER BY s.created_at ASC;
