-- Admin ATM Management (Task 2): sqlc queries for the admin ATM CRUD
-- endpoints (.kiro/specs/admin-atm-management). No new tables -- all columns
-- already exist on atms (002, 030). Kept separate from the atm-portal
-- read-only queries (atm_portal.sql).

-- name: ListATMsAdmin :many
-- Admin ATM list. Filters: q (ILIKE on terminal_id), brand, machine_type,
-- deployment_type, priority_class, location_id (all sqlc.narg, NULL = no
-- filter) and status -- the caller resolves absent status to 'active'
-- before calling (same convention as ListVendorsAdmin), so this query only
-- ever sees 'active' | 'disabled' | 'all'. LEFT JOIN locations for
-- location_name. Ordered terminal_id ASC, id ASC.
SELECT a.id, a.terminal_id, a.location_id, l.name AS location_name,
       a.machine_type, a.brand, a.model, a.operation_hours, a.deployment_type,
       a.capacity_amount, a.low_threshold_amount, a.critical_threshold_amount,
       a.blacklisted, a.escrow_account, a.priority_class, a.is_active,
       a.created_at, a.updated_at, a.deleted_at
FROM atms a
LEFT JOIN locations l ON l.id = a.location_id
WHERE (sqlc.narg('q')::text IS NULL
        OR a.terminal_id ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (sqlc.narg('brand')::text IS NULL OR a.brand = sqlc.narg('brand')::text)
  AND (sqlc.narg('machine_type')::text IS NULL OR a.machine_type = sqlc.narg('machine_type')::text)
  AND (sqlc.narg('deployment_type')::text IS NULL OR a.deployment_type = sqlc.narg('deployment_type')::text)
  AND (sqlc.narg('priority_class')::text IS NULL OR a.priority_class = sqlc.narg('priority_class')::text)
  AND (sqlc.narg('location_id')::bigint IS NULL OR a.location_id = sqlc.narg('location_id')::bigint)
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND a.deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND a.deleted_at IS NOT NULL)
      )
ORDER BY a.terminal_id ASC, a.id ASC
LIMIT sqlc.arg('page_limit')::bigint OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountATMsAdmin :one
-- Same filters as ListATMsAdmin, no LIMIT/OFFSET -- pagination total.
SELECT COUNT(*)
FROM atms a
WHERE (sqlc.narg('q')::text IS NULL
        OR a.terminal_id ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (sqlc.narg('brand')::text IS NULL OR a.brand = sqlc.narg('brand')::text)
  AND (sqlc.narg('machine_type')::text IS NULL OR a.machine_type = sqlc.narg('machine_type')::text)
  AND (sqlc.narg('deployment_type')::text IS NULL OR a.deployment_type = sqlc.narg('deployment_type')::text)
  AND (sqlc.narg('priority_class')::text IS NULL OR a.priority_class = sqlc.narg('priority_class')::text)
  AND (sqlc.narg('location_id')::bigint IS NULL OR a.location_id = sqlc.narg('location_id')::bigint)
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND a.deleted_at IS NULL)
        OR (sqlc.arg('status')::text = 'disabled' AND a.deleted_at IS NOT NULL)
      );

-- name: GetATMAdminByID :one
-- Includes soft-deleted rows -- 404 pre-checks on update/disable/enable
-- treat a soft-disabled ATM as not-found for edit, same convention as
-- GetVendorAdminByID/GetUserAdminByID.
SELECT a.id, a.terminal_id, a.location_id, l.name AS location_name,
       a.machine_type, a.brand, a.model, a.operation_hours, a.deployment_type,
       a.capacity_amount, a.low_threshold_amount, a.critical_threshold_amount,
       a.blacklisted, a.escrow_account, a.priority_class, a.is_active,
       a.created_at, a.updated_at, a.deleted_at
FROM atms a
LEFT JOIN locations l ON l.id = a.location_id
WHERE a.id = $1;

-- name: FindATMByTerminalID :one
-- Uniqueness pre-check (incl. soft-deleted) for create/update (Req 3.4, 4.3).
SELECT id FROM atms WHERE terminal_id = $1;

-- name: CreateATMAdmin :one
INSERT INTO atms (terminal_id, location_id, machine_type, brand, model,
                   operation_hours, deployment_type, capacity_amount,
                   low_threshold_amount, critical_threshold_amount,
                   blacklisted, escrow_account, priority_class)
VALUES (sqlc.arg('terminal_id'), sqlc.arg('location_id'), sqlc.arg('machine_type'),
        sqlc.arg('brand'), sqlc.arg('model'), sqlc.arg('operation_hours'),
        sqlc.arg('deployment_type'), sqlc.narg('capacity_amount'),
        sqlc.narg('low_threshold_amount'), sqlc.narg('critical_threshold_amount'),
        sqlc.arg('blacklisted'), sqlc.narg('escrow_account'), sqlc.narg('priority_class'))
RETURNING id, terminal_id, location_id, machine_type, brand, model,
          operation_hours, deployment_type, capacity_amount,
          low_threshold_amount, critical_threshold_amount, blacklisted,
          escrow_account, priority_class, is_active, created_at, updated_at, deleted_at;

-- name: UpdateATMAdmin :one
-- Editable fields only; terminal_id is never touched here (immutable on
-- update, Req 5.6). `AND deleted_at IS NULL` makes a soft-disabled target
-- update as 0 rows affected, mapped to "not found" by the repository.
UPDATE atms
SET location_id = sqlc.arg('location_id'),
    machine_type = sqlc.arg('machine_type'),
    brand = sqlc.arg('brand'),
    model = sqlc.arg('model'),
    operation_hours = sqlc.arg('operation_hours'),
    deployment_type = sqlc.arg('deployment_type'),
    capacity_amount = sqlc.narg('capacity_amount'),
    low_threshold_amount = sqlc.narg('low_threshold_amount'),
    critical_threshold_amount = sqlc.narg('critical_threshold_amount'),
    blacklisted = sqlc.arg('blacklisted'),
    escrow_account = sqlc.narg('escrow_account'),
    priority_class = sqlc.narg('priority_class'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, terminal_id, location_id, machine_type, brand, model,
          operation_hours, deployment_type, capacity_amount,
          low_threshold_amount, critical_threshold_amount, blacklisted,
          escrow_account, priority_class, is_active, created_at, updated_at, deleted_at;

-- name: DisableATM :exec
-- Soft-disable only: is_active=false, deleted_at=now(). No hard DELETE on
-- atms anywhere in this codebase (guard test extended in Task 5).
UPDATE atms SET is_active = false, deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: EnableATM :exec
-- Reverses DisableATM: is_active=true, deleted_at=NULL.
UPDATE atms SET is_active = true, deleted_at = NULL WHERE id = $1;

-- name: ListLocationsForSelect :many
-- Backs the Location select on the ATM form (Req 6). Ordered by name.
SELECT id, name, city_or_regency, province FROM locations ORDER BY name ASC;

-- name: LocationExists :one
-- Existence check for location_id references (400 invalid_reference if false).
SELECT EXISTS(SELECT 1 FROM locations WHERE id = $1);
