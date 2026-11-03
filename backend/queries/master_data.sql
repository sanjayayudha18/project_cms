-- Master data maker-checker (plan.md Fase 1/2): sqlc queries for
-- master_data_change_requests, the generic staging table every master-data
-- create/update/disable/enable goes through before being applied.

-- name: CreateMasterDataChangeRequest :one
-- Submit (T2.1). entity_id is NULL for op='create'.
INSERT INTO master_data_change_requests (entity_type, entity_id, op, payload, before, maker_id, batch_id)
VALUES (sqlc.arg('entity_type'), sqlc.narg('entity_id'), sqlc.arg('op'), sqlc.arg('payload'),
        sqlc.narg('before'), sqlc.arg('maker_id'), sqlc.narg('batch_id'))
RETURNING id, entity_type, entity_id, op, payload, before, status, maker_id, approval_request_id, batch_id, error, created_at, updated_at;

-- name: FindPendingMasterDataChangeRequest :one
-- Idempotency guard (T2.2): reject a second submit while one is already
-- pending for this entity. entity_type/entity_id pair matches the table's
-- partial unique index.
SELECT id, status FROM master_data_change_requests
WHERE entity_type = sqlc.arg('entity_type')
  AND entity_id IS NOT DISTINCT FROM sqlc.narg('entity_id')
  AND status = 'pending';

-- name: GetMasterDataChangeRequestByID :one
SELECT id, entity_type, entity_id, op, payload, before, status, maker_id, approval_request_id, batch_id, error, created_at, updated_at
FROM master_data_change_requests WHERE id = $1;

-- name: SetMasterDataChangeRequestApprovalID :exec
-- Links the staged row to the approval_requests row Orchestrator.SubmitForApproval created (T2.1).
UPDATE master_data_change_requests SET approval_request_id = sqlc.arg('approval_request_id')
WHERE id = sqlc.arg('id');

-- name: MarkMasterDataChangeRequestApproved :exec
-- Approval reached its final step (T2.4): status flips to 'approved' before
-- the apply attempt is made, durably and independent of whether apply then
-- succeeds -- if apply fails and its own transaction rolls back, this
-- status persists as the documented "approved-but-not-applied" state.
UPDATE master_data_change_requests SET status = 'approved'
WHERE id = sqlc.arg('id');

-- name: MarkMasterDataChangeRequestApplied :exec
-- Apply-on-approve success path (T2.4), same transaction as the entity mutation.
UPDATE master_data_change_requests SET status = 'applied', error = NULL
WHERE id = sqlc.arg('id');

-- name: MarkMasterDataChangeRequestStale :exec
-- Staleness detected at apply time (T2.5): current entity state no longer matches `before`.
UPDATE master_data_change_requests SET status = 'stale', error = sqlc.arg('error')
WHERE id = sqlc.arg('id');

-- name: MarkMasterDataChangeRequestApplyFailed :exec
-- Apply failed and rolled back (T2.4): stays approved, not applied, error recorded for retry/diagnosis.
UPDATE master_data_change_requests SET error = sqlc.arg('error')
WHERE id = sqlc.arg('id');

-- name: MarkMasterDataChangeRequestRejected :exec
-- Reject path (T2.6): no entity mutation, just the status transition.
UPDATE master_data_change_requests SET status = 'rejected'
WHERE id = sqlc.arg('id');

-- name: ListMasterDataChangeRequests :many
-- Read endpoint (T2.7): filter by entity_type, entity_id, and/or status, all
-- optional. entity_id lets a caller look up the pending/applied change
-- history for one specific row (e.g. a per-row "pending approval" badge).
SELECT id, entity_type, entity_id, op, payload, before, status, maker_id, approval_request_id, batch_id, error, created_at, updated_at
FROM master_data_change_requests
WHERE (sqlc.narg('entity_type')::text IS NULL OR entity_type = sqlc.narg('entity_type')::text)
  AND (sqlc.narg('entity_id')::bigint IS NULL OR entity_id = sqlc.narg('entity_id')::bigint)
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('page_limit')::bigint OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountMasterDataChangeRequests :one
-- Same filters as ListMasterDataChangeRequests, for pagination total.
SELECT COUNT(*) FROM master_data_change_requests
WHERE (sqlc.narg('entity_type')::text IS NULL OR entity_type = sqlc.narg('entity_type')::text)
  AND (sqlc.narg('entity_id')::bigint IS NULL OR entity_id = sqlc.narg('entity_id')::bigint)
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text);

-- name: CreateMasterDataImportBatch :one
-- Import confirm (T5.4). Runs in the same transaction as the change-request inserts.
INSERT INTO master_data_import_batches (entity, file_hash, maker_id, row_count, file_name)
VALUES (sqlc.arg('entity'), sqlc.arg('file_hash'), sqlc.arg('maker_id'), sqlc.arg('row_count'), sqlc.narg('file_name'))
RETURNING id, entity, file_hash, maker_id, row_count, created_at, file_name;

-- name: FindOpenMasterDataImportBatch :one
-- Idempotency lookup (T5.4): the latest batch of this file whose changes were
-- not rejected/stale (a rejected/stale file may be uploaded again). head_id is
-- the first change request, the approval's document_id.
SELECT b.id, b.entity, b.file_hash, b.maker_id, b.row_count, b.created_at, b.file_name,
       h.id AS head_id, h.approval_request_id, h.status AS head_status
FROM master_data_import_batches b
JOIN LATERAL (
    SELECT c.id, c.approval_request_id, c.status
    FROM master_data_change_requests c
    WHERE c.batch_id = b.id
    ORDER BY c.id
    LIMIT 1
) h ON true
WHERE b.entity = sqlc.arg('entity') AND b.file_hash = sqlc.arg('file_hash')
  AND h.status NOT IN ('rejected', 'stale')
ORDER BY b.id DESC
LIMIT 1;

-- name: ListMasterDataChangeRequestsByBatch :many
SELECT id, entity_type, entity_id, op, payload, before, status, maker_id, approval_request_id, batch_id, error, created_at, updated_at
FROM master_data_change_requests
WHERE batch_id = sqlc.arg('batch_id')
ORDER BY id;

-- name: ListMasterDataImportBatches :many
-- Import history (master-data-import-revamp 1.2). Status is derived from the head
-- change request (first row of the batch); decided_* from the last acted step.
SELECT b.id, b.entity, b.file_name, b.row_count, b.created_at, b.maker_id,
       u.full_name AS maker_name,
       h.id AS head_id, h.status AS head_status, h.error AS head_error, h.approval_request_id,
       d.acted_at AS decided_at, du.full_name AS decided_by_name
FROM master_data_import_batches b
JOIN users u ON u.id = b.maker_id
JOIN LATERAL (
    SELECT c.id, c.status, c.error, c.approval_request_id
    FROM master_data_change_requests c
    WHERE c.batch_id = b.id
    ORDER BY c.id
    LIMIT 1
) h ON true
LEFT JOIN LATERAL (
    SELECT s.acted_at, s.acted_by_id
    FROM approval_steps s
    WHERE s.request_id = h.approval_request_id AND s.status <> 'pending'
    ORDER BY s.acted_at DESC NULLS LAST
    LIMIT 1
) d ON true
LEFT JOIN users du ON du.id = d.acted_by_id
WHERE (sqlc.narg('entity')::text IS NULL OR b.entity = sqlc.narg('entity')::text)
  AND (sqlc.narg('status')::text IS NULL OR h.status = sqlc.narg('status')::text)
  AND (sqlc.narg('maker_id')::bigint IS NULL OR b.maker_id = sqlc.narg('maker_id')::bigint)
ORDER BY b.id DESC
LIMIT sqlc.arg('page_limit')::bigint OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountMasterDataImportBatches :one
-- Same filters as ListMasterDataImportBatches, for pagination total.
SELECT COUNT(*)
FROM master_data_import_batches b
JOIN LATERAL (
    SELECT c.status
    FROM master_data_change_requests c
    WHERE c.batch_id = b.id
    ORDER BY c.id
    LIMIT 1
) h ON true
WHERE (sqlc.narg('entity')::text IS NULL OR b.entity = sqlc.narg('entity')::text)
  AND (sqlc.narg('status')::text IS NULL OR h.status = sqlc.narg('status')::text)
  AND (sqlc.narg('maker_id')::bigint IS NULL OR b.maker_id = sqlc.narg('maker_id')::bigint);

-- name: GetMasterDataImportBatch :one
SELECT b.id, b.entity, b.file_name, b.row_count, b.created_at, b.maker_id,
       u.full_name AS maker_name,
       h.id AS head_id, h.status AS head_status, h.error AS head_error, h.approval_request_id,
       d.acted_at AS decided_at, du.full_name AS decided_by_name
FROM master_data_import_batches b
JOIN users u ON u.id = b.maker_id
JOIN LATERAL (
    SELECT c.id, c.status, c.error, c.approval_request_id
    FROM master_data_change_requests c
    WHERE c.batch_id = b.id
    ORDER BY c.id
    LIMIT 1
) h ON true
LEFT JOIN LATERAL (
    SELECT s.acted_at, s.acted_by_id
    FROM approval_steps s
    WHERE s.request_id = h.approval_request_id AND s.status <> 'pending'
    ORDER BY s.acted_at DESC NULLS LAST
    LIMIT 1
) d ON true
LEFT JOIN users du ON du.id = d.acted_by_id
WHERE b.id = sqlc.arg('id');

-- name: ListPendingApproversForRequest :many
-- Assigned approvers of the lowest pending step of an approval request (import notifications).
SELECT s.assigned_approver_id
FROM approval_steps s
WHERE s.request_id = sqlc.arg('request_id')
  AND s.status = 'pending'
  AND s.step_level = (
      SELECT MIN(p.step_level) FROM approval_steps p
      WHERE p.request_id = sqlc.arg('request_id') AND p.status = 'pending'
  )
ORDER BY s.assigned_approver_id;

-- name: SetMasterDataBatchApprovalID :exec
-- Links every change request of the batch to the batch's single approval request.
UPDATE master_data_change_requests SET approval_request_id = sqlc.arg('approval_request_id')
WHERE batch_id = sqlc.arg('batch_id');
