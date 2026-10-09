-- Area ACM (cit-acm-plan FR7): ADMIN-managed, immediate-apply + audit in the
-- same tx (documented GR#3 deviation, pattern regions_admin.sql). Branch and
-- member links are plain link tables (no soft delete); changes are audited.

-- name: ListAcmAreasAdmin :many
SELECT a.id, a.name, a.is_active, a.deleted_at, a.created_at,
       (SELECT COUNT(*) FROM acm_area_branches b WHERE b.acm_area_id = a.id) AS branch_count,
       (SELECT COUNT(*) FROM acm_area_members m WHERE m.acm_area_id = a.id) AS member_count
FROM acm_areas a
WHERE (sqlc.arg('status')::text = 'all'
       OR (sqlc.arg('status')::text = 'active' AND a.is_active)
       OR (sqlc.arg('status')::text = 'disabled' AND NOT a.is_active))
ORDER BY lower(a.name), a.id;

-- name: GetAcmAreaAdmin :one
SELECT id, name, is_active, deleted_at, created_by, created_at, updated_at
FROM acm_areas WHERE id = $1;

-- name: ListAcmAreaBranches :many
SELECT b.vendor_branch_id, vb.branch_code, vb.branch_name, vb.region_code, vb.is_active AS branch_is_active,
       v.id AS vendor_id, v.name AS vendor_name
FROM acm_area_branches b
JOIN vendor_branches vb ON vb.id = b.vendor_branch_id
JOIN vendors v ON v.id = vb.vendor_id
WHERE b.acm_area_id = $1
ORDER BY v.name, vb.branch_code;

-- name: ListAcmAreaMembers :many
SELECT m.user_id, u.username, u.full_name, r.role, u.is_active AS user_is_active
FROM acm_area_members m
JOIN users u ON u.id = m.user_id
JOIN roles r ON r.id = u.role_id
WHERE m.acm_area_id = $1
ORDER BY u.full_name, u.id;

-- name: CreateAcmArea :one
INSERT INTO acm_areas (name, created_by) VALUES ($1, $2)
RETURNING id, name, is_active, deleted_at, created_by, created_at, updated_at;

-- name: RenameAcmArea :one
UPDATE acm_areas SET name = $2 WHERE id = $1
RETURNING id, name, is_active, deleted_at, created_by, created_at, updated_at;

-- name: SetAcmAreaActive :one
UPDATE acm_areas
SET is_active = sqlc.arg('is_active')::boolean,
    deleted_at = CASE WHEN sqlc.arg('is_active')::boolean THEN NULL ELSE now() END
WHERE id = sqlc.arg('id')
RETURNING id, name, is_active, deleted_at, created_by, created_at, updated_at;

-- name: ClearAcmAreaBranches :exec
-- Link-table replace-all / release on disable (branch unique across areas).
DELETE FROM acm_area_branches WHERE acm_area_id = $1;

-- name: AddAcmAreaBranch :exec
INSERT INTO acm_area_branches (acm_area_id, vendor_branch_id, created_by) VALUES ($1, $2, $3);

-- name: ClearAcmAreaMembers :exec
DELETE FROM acm_area_members WHERE acm_area_id = $1;

-- name: AddAcmAreaMember :exec
INSERT INTO acm_area_members (acm_area_id, user_id, created_by) VALUES ($1, $2, $3);

-- name: ListBranchesInOtherAcmAreas :many
-- Branches from the requested set already held by another area (-> 409).
SELECT b.vendor_branch_id, a.id AS acm_area_id, a.name AS acm_area_name
FROM acm_area_branches b JOIN acm_areas a ON a.id = b.acm_area_id
WHERE b.vendor_branch_id = ANY(sqlc.arg('branch_ids')::bigint[]) AND b.acm_area_id <> sqlc.arg('acm_area_id');

-- name: CountActiveVendorBranches :one
SELECT COUNT(*) FROM vendor_branches WHERE id = ANY(sqlc.arg('ids')::bigint[]) AND is_active;

-- name: CountActiveAcmUsers :one
-- Members must be active users holding ACM-USER or ACM-SPV.
SELECT COUNT(*) FROM users u JOIN roles r ON r.id = u.role_id
WHERE u.id = ANY(sqlc.arg('ids')::bigint[]) AND u.is_active AND r.role IN ('ACM-USER', 'ACM-SPV');

-- name: ListAcmEligibleUsers :many
-- Picker source for the member editor.
SELECT u.id, u.username, u.full_name, r.role
FROM users u JOIN roles r ON r.id = u.role_id
WHERE u.is_active AND r.role IN ('ACM-USER', 'ACM-SPV')
ORDER BY u.full_name, u.id;

-- name: ListUnassignedVaultAssignmentBranches :many
-- FR7.4: replenish branches that have an ATM on a request waiting for vault
-- assignment but belong to no ACM area -- those requests cannot progress.
SELECT vb.id AS vendor_branch_id, vb.branch_code, vb.branch_name, v.name AS vendor_name,
       COUNT(DISTINCT vr.id) AS request_count
FROM vendor_requests vr
JOIN vendor_request_tickets t ON t.request_number = vr.request_number AND t.is_active
JOIN atms a ON a.terminal_id = t.terminal_id
JOIN LATERAL (
    SELECT COALESCE(p.vendor_branch_id, avp.vendor_branch_id) AS vendor_branch_id
    FROM atm_vendor_packages avp
    LEFT JOIN vendor_packages_branch p ON p.id = avp.vendor_package_id
    WHERE avp.atm_id = a.id AND avp.is_active
      AND avp.effective_start_date <= t.replenish_date
      AND (avp.effective_end_date IS NULL OR avp.effective_end_date >= t.replenish_date)
    ORDER BY avp.effective_start_date DESC, avp.id DESC
    LIMIT 1
) k ON true
JOIN vendor_branches vb ON vb.id = k.vendor_branch_id
JOIN vendors v ON v.id = vb.vendor_id
WHERE vr.status = 'vault_assignment'
  AND NOT EXISTS (SELECT 1 FROM acm_area_branches ab WHERE ab.vendor_branch_id = vb.id)
GROUP BY vb.id, vb.branch_code, vb.branch_name, v.name
ORDER BY v.name, vb.branch_code;

-- name: ListAcmBranchOptions :many
-- Branch picker: every active vendor branch with the area currently holding it (NULL = free).
SELECT vb.id AS vendor_branch_id, vb.branch_code, vb.branch_name, vb.region_code, v.name AS vendor_name,
       a.id AS acm_area_id, a.name AS acm_area_name
FROM vendor_branches vb
JOIN vendors v ON v.id = vb.vendor_id
LEFT JOIN acm_area_branches ab ON ab.vendor_branch_id = vb.id
LEFT JOIN acm_areas a ON a.id = ab.acm_area_id
WHERE vb.is_active
ORDER BY v.name, vb.branch_code;
