-- name: CreateAuditLog :one
-- Appends one audit trail row. Append-only: no update/delete query is defined here.
INSERT INTO audit_logs (actor_id, action, entity_type, entity_id, before, after, ip)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;
