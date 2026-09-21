-- Admin ATM assignment (kelolaan ATM) management (plan.md Fase 3, T3.5): an
-- assignment is an atm_vendor_packages row -- one ATM under one vendor
-- package for an effective period. Mutations route through the master-data
-- maker-checker engine (Fase 2, D1): create/update/disable/enable below are
-- only run by ATMAssignmentApplier inside the apply-on-approve transaction.
-- The exclusion constraint atm_vendor_packages_no_overlap (T1.6) is the
-- authoritative overlap guard; FindOverlappingATMAssignment is only the
-- early, friendly submit-time check.

-- name: ListATMAssignmentsAdmin :many
-- status: 'active'|'disabled'|'all' on is_active (the table has no deleted_at;
-- is_active=false is what releases the period from the exclusion constraint).
SELECT a.id, a.atm_id, a.vendor_package_id, p.code AS package_code, p.priority_class,
       a.effective_start_date, a.effective_end_date, a.is_active
FROM atm_vendor_packages a
JOIN vendor_packages p ON p.id = a.vendor_package_id
WHERE a.atm_id = sqlc.arg('atm_id')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND a.is_active)
        OR (sqlc.arg('status')::text = 'disabled' AND NOT a.is_active)
      )
ORDER BY a.effective_start_date DESC, a.id DESC
LIMIT sqlc.arg('page_limit')::bigint OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountATMAssignmentsAdmin :one
SELECT COUNT(*)
FROM atm_vendor_packages a
WHERE a.atm_id = sqlc.arg('atm_id')
  AND (
        sqlc.arg('status')::text = 'all'
        OR (sqlc.arg('status')::text = 'active' AND a.is_active)
        OR (sqlc.arg('status')::text = 'disabled' AND NOT a.is_active)
      );

-- name: GetATMAssignmentAdminByID :one
-- Doubles as the "before" snapshot / CurrentState (T2.5).
SELECT a.id, a.atm_id, a.vendor_package_id, p.code AS package_code, p.priority_class,
       a.effective_start_date, a.effective_end_date, a.is_active
FROM atm_vendor_packages a
JOIN vendor_packages p ON p.id = a.vendor_package_id
WHERE a.id = $1;

-- name: FindOverlappingATMAssignment :one
-- Same predicate as the exclusion constraint: active rows of the same ATM whose
-- inclusive daterange overlaps [start, end] (NULL end = unbounded). exclude_id
-- lets an update ignore its own row (pass 0 for create).
SELECT id FROM atm_vendor_packages
WHERE atm_id = sqlc.arg('atm_id')
  AND is_active
  AND id <> sqlc.arg('exclude_id')
  AND daterange(effective_start_date, effective_end_date, '[]')
      && daterange(sqlc.arg('start_date')::date, sqlc.narg('end_date')::date, '[]')
LIMIT 1;

-- name: ATMActiveForAssignment :one
-- Submit-time check that the ATM exists and is not soft-deleted.
SELECT id FROM atms WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateATMAssignmentAdmin :one
INSERT INTO atm_vendor_packages (atm_id, vendor_package_id, effective_start_date, effective_end_date)
VALUES (sqlc.arg('atm_id'), sqlc.arg('vendor_package_id'), sqlc.arg('effective_start_date'), sqlc.narg('effective_end_date'))
RETURNING id, atm_id, vendor_package_id, effective_start_date, effective_end_date, is_active;

-- name: UpdateATMAssignmentAdmin :one
-- atm_id is immutable. Only an active row can be edited (a disabled one is
-- re-enabled first, which re-runs the overlap guard).
UPDATE atm_vendor_packages
SET vendor_package_id = sqlc.arg('vendor_package_id'),
    effective_start_date = sqlc.arg('effective_start_date'),
    effective_end_date = sqlc.narg('effective_end_date'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND is_active
RETURNING id, atm_id, vendor_package_id, effective_start_date, effective_end_date, is_active;

-- name: DisableATMAssignment :exec
-- Soft-disable only; releases the period from the exclusion constraint.
UPDATE atm_vendor_packages SET is_active = false WHERE id = $1;

-- name: EnableATMAssignment :exec
-- May raise the exclusion violation if the period was re-assigned meanwhile.
UPDATE atm_vendor_packages SET is_active = true WHERE id = $1;
