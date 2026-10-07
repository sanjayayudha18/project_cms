-- name: CreateAuditLog :one
-- Appends one audit trail row. Append-only: no update/delete query is defined here.
INSERT INTO audit_logs (actor_id, action, entity_type, entity_id, before, after, ip)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListAuditLogsByEntity :many
-- Generic append-only audit trail read for any (entity_type, entity_id) pair
-- -- backs GET /{id}/audit-log for vendor_request (Req 16.4) and is reusable
-- by any future entity without a new query. Ascending by created_at so the
-- trail reads chronologically (oldest first).
SELECT * FROM audit_logs
WHERE entity_type = $1 AND entity_id = $2
ORDER BY created_at ASC;
