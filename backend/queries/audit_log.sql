-- Read-path queries for the Audit Log Viewer (audit-log-viewer spec, Task 2).
-- Separate from queries/audit.sql (the writer's CreateAuditLog + the
-- entity-scoped ListAuditLogsByEntity) to keep the read/list/paginate surface
-- for the admin viewer distinct from the generic per-entity trail helper.
--
-- All filters are optional (sqlc.narg): NULL means "no filter on this field".
-- limit/offset are named args (not positional) so they mix cleanly with the
-- narg filters under one sqlc version.

-- name: ListAuditLogs :many
SELECT id, actor_id, action, entity_type, entity_id, before, after, ip, created_at
FROM audit_logs
WHERE (sqlc.narg('actor_id')::bigint       IS NULL OR actor_id    = sqlc.narg('actor_id'))
  AND (sqlc.narg('action')::text           IS NULL OR action      = sqlc.narg('action'))
  AND (sqlc.narg('entity_type')::text      IS NULL OR entity_type = sqlc.narg('entity_type'))
  AND (sqlc.narg('entity_id')::bigint      IS NULL OR entity_id   = sqlc.narg('entity_id'))
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR created_at >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz   IS NULL OR created_at <= sqlc.narg('date_to'))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountAuditLogs :one
-- Mirrors ListAuditLogs' WHERE exactly (minus ORDER BY/LIMIT/OFFSET) so the
-- count matches the filtered result set for pagination math.
SELECT count(*)
FROM audit_logs
WHERE (sqlc.narg('actor_id')::bigint       IS NULL OR actor_id    = sqlc.narg('actor_id'))
  AND (sqlc.narg('action')::text           IS NULL OR action      = sqlc.narg('action'))
  AND (sqlc.narg('entity_type')::text      IS NULL OR entity_type = sqlc.narg('entity_type'))
  AND (sqlc.narg('entity_id')::bigint      IS NULL OR entity_id   = sqlc.narg('entity_id'))
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR created_at >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz   IS NULL OR created_at <= sqlc.narg('date_to'));

-- name: GetAuditLogByID :one
SELECT id, actor_id, action, entity_type, entity_id, before, after, ip, created_at
FROM audit_logs
WHERE id = $1;
