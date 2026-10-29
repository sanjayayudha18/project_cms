-- Append-only: insert only, no update/delete query exists here (Req 8.10).
-- Same columns as backend/queries/audit.sql CreateAuditLog.

-- name: InsertAuditLog :one
INSERT INTO audit_logs (actor_id, action, entity_type, entity_id, before, after, ip)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;
