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
-- No priority_class here since migration 010 dropped vendor_packages_branch.priority_class.
-- Two sources (migration 023): 'branch' = vendor_package_id -> vendor_packages_branch;
-- 'vendor' = vendor_id + vendor_branch_id + package (label). package_code and the
-- managing branch/vendor resolve through COALESCE so callers see one shape.
SELECT a.id, a.atm_id, a.vendor_package_id,
       COALESCE(p.package_code, a.package)::text AS package_code,
       a.effective_start_date, a.effective_end_date, a.is_active,
       vb.id AS vendor_branch_id,
       vb.vendor_id,
       CASE WHEN a.vendor_package_id IS NULL THEN 'vendor' ELSE 'branch' END::text AS source
FROM atm_vendor_packages a
LEFT JOIN vendor_packages_branch p ON p.id = a.vendor_package_id
LEFT JOIN vendor_branches vb ON vb.id = COALESCE(p.vendor_branch_id, a.vendor_branch_id) -- NULL for internal (ROH) packages
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
SELECT a.id, a.atm_id, a.vendor_package_id,
       COALESCE(p.package_code, a.package)::text AS package_code,
       a.effective_start_date, a.effective_end_date, a.is_active,
       a.vendor_id, a.vendor_branch_id, a.package,
       CASE WHEN a.vendor_package_id IS NULL THEN 'vendor' ELSE 'branch' END::text AS source
FROM atm_vendor_packages a
LEFT JOIN vendor_packages_branch p ON p.id = a.vendor_package_id
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
INSERT INTO atm_vendor_packages (atm_id, vendor_package_id, vendor_id, vendor_branch_id, package, effective_start_date, effective_end_date)
VALUES (sqlc.arg('atm_id'), sqlc.narg('vendor_package_id'), sqlc.narg('vendor_id'), sqlc.narg('vendor_branch_id'), sqlc.narg('package'),
        sqlc.arg('effective_start_date'), sqlc.narg('effective_end_date'))
RETURNING id, atm_id, vendor_package_id, vendor_id, vendor_branch_id, package, effective_start_date, effective_end_date, is_active;

-- name: UpdateATMAssignmentAdmin :one
-- atm_id is immutable. Only an active row can be edited (a disabled one is
-- re-enabled first, which re-runs the overlap guard). The source mode never
-- changes (the service rejects it; avp_source_chk is the backstop).
UPDATE atm_vendor_packages
SET vendor_package_id = sqlc.narg('vendor_package_id'),
    vendor_id = sqlc.narg('vendor_id'),
    vendor_branch_id = sqlc.narg('vendor_branch_id'),
    package = sqlc.narg('package'),
    effective_start_date = sqlc.arg('effective_start_date'),
    effective_end_date = sqlc.narg('effective_end_date'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND is_active
RETURNING id, atm_id, vendor_package_id, vendor_id, vendor_branch_id, package, effective_start_date, effective_end_date, is_active;

-- name: DisableATMAssignment :exec
-- Soft-disable only; releases the period from the exclusion constraint.
UPDATE atm_vendor_packages SET is_active = false WHERE id = $1;

-- name: EnableATMAssignment :exec
-- May raise the exclusion violation if the period was re-assigned meanwhile.
UPDATE atm_vendor_packages SET is_active = true WHERE id = $1;

-- name: CloseATMAssignmentsBeforeStart :exec
-- Automatic handover (create with no dates): the running period(s) of this ATM
-- end the day before the new period starts.
UPDATE atm_vendor_packages
SET effective_end_date = sqlc.arg('start_date')::date - 1, updated_at = now()
WHERE atm_id = sqlc.arg('atm_id') AND is_active
  AND effective_start_date < sqlc.arg('start_date')::date
  AND (effective_end_date IS NULL OR effective_end_date >= sqlc.arg('start_date')::date);

-- name: DisableATMAssignmentsStartingOn :exec
-- Automatic handover: a period that began the same day can't be closed to
-- start-1 (end < start), so it is soft-disabled instead.
UPDATE atm_vendor_packages
SET is_active = false, updated_at = now()
WHERE atm_id = sqlc.arg('atm_id') AND is_active
  AND effective_start_date = sqlc.arg('start_date')::date;

-- name: GetATMPriceGroup :one
-- price_machine_group / price_class are generated columns; NULL when the ATM's
-- machine_type/priority_class is unmapped (vendor-wide assignment then fails hard).
SELECT price_machine_group, price_class FROM atms WHERE id = $1 AND deleted_at IS NULL;

-- name: CheckAssignmentVendorBranch :one
-- Vendor-wide assignment (FR9.2-3): the vendor is an active FLM vendor (not
-- INTERNAL/ROH, which never has tariffs) and the branch belongs to it and is active.
SELECT vb.id
FROM vendor_branches vb
JOIN vendors v ON v.id = vb.vendor_id
WHERE vb.id = sqlc.arg('vendor_branch_id') AND vb.vendor_id = sqlc.arg('vendor_id')
  AND vb.is_active AND vb.deleted_at IS NULL
  AND v.is_active AND v.deleted_at IS NULL AND v.kind = 'FLM_VENDOR';

-- name: VendorTariffExistsForATM :one
-- FR9.4: some tariff row of this vendor/label matches the ATM's machine group and
-- price class and is effective on as_of. A row scoped to another ATM (atm_id set)
-- does not count; PT-wide and branch rows do.
SELECT EXISTS (
    SELECT 1
    FROM atms a
    JOIN vendor_package_prices vpp
      ON vpp.vendor_id = sqlc.arg('vendor_id')
     AND vpp.package = sqlc.arg('package')
     AND vpp.machine_group = a.price_machine_group
     AND vpp.price_class = a.price_class
     AND (vpp.atm_id IS NULL OR vpp.atm_id = a.id)
     AND vpp.effective_start_date <= sqlc.arg('as_of')::date
     AND (vpp.effective_end_date IS NULL OR vpp.effective_end_date >= sqlc.arg('as_of')::date)
    WHERE a.id = sqlc.arg('atm_id') AND a.deleted_at IS NULL
) AS exists;

-- name: ListATMPackageOptions :many
-- FR11: the vendor's tariff rows that pass the same check as
-- VendorTariffExistsForATM, for the assignment dialog's vendor-wide dropdown.
-- One row per tariff (code + tier + price) so the dropdown shows what the label
-- means; the assignment still stores only the label (avp.package).
SELECT vpp.package::text AS package,
       vpp.package_code,
       vpp.tier_min,
       vpp.tier_max,
       COALESCE(vpp.base_price::text, '')::text AS base_price,
       vpp.currency::text AS currency
FROM atms a
JOIN vendor_package_prices vpp
  ON vpp.vendor_id = sqlc.arg('vendor_id')
 AND vpp.machine_group = a.price_machine_group
 AND vpp.price_class = a.price_class
 AND (vpp.atm_id IS NULL OR vpp.atm_id = a.id)
 AND vpp.effective_start_date <= sqlc.arg('as_of')::date
 AND (vpp.effective_end_date IS NULL OR vpp.effective_end_date >= sqlc.arg('as_of')::date)
WHERE a.id = sqlc.arg('atm_id') AND a.deleted_at IS NULL
ORDER BY vpp.package, vpp.tier_min, vpp.package_code;
