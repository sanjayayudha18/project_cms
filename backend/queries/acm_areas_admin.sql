-- Area ACM (cit-acm-plan FR7): ADMIN-managed, immediate-apply + audit in the
-- same tx (documented GR#3 deviation, pattern regions_admin.sql). Branch and
-- member links are plain link tables (no soft delete); changes are audited.

-- name: ListAcmAreasAdmin :many
SELECT a.id, a.name, a.is_active, a.deleted_at, a.created_at,
       (SELECT COUNT(*) FROM acm_area_branches b WHERE b.acm_area_id = a.id) AS branch_count,
       (SELECT COUNT(*) FROM acm_area_members m WHERE m.acm_area_id = a.id) AS member_count,
       (SELECT COUNT(*) FROM acm_area_regions g WHERE g.acm_area_id = a.id) AS region_count
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

-- ---------------------------------------------------------------------------
-- cit-vault-select FR1: Area ACM per region (replaces branch links for new plans)
-- ---------------------------------------------------------------------------

-- name: ListAcmAreaRegions :many
SELECT region_code FROM acm_area_regions WHERE acm_area_id = $1 ORDER BY region_code;

-- name: ClearAcmAreaRegions :exec
-- Link-table replace-all / release on disable (region unique across areas).
DELETE FROM acm_area_regions WHERE acm_area_id = $1;

-- name: AddAcmAreaRegion :exec
INSERT INTO acm_area_regions (acm_area_id, region_code, created_by) VALUES ($1, $2, $3);

-- name: ListRegionsInOtherAcmAreas :many
-- Regions from the requested set already held by another area (-> 409).
SELECT r.region_code, a.id AS acm_area_id, a.name AS acm_area_name
FROM acm_area_regions r JOIN acm_areas a ON a.id = r.acm_area_id
WHERE r.region_code = ANY(sqlc.arg('region_codes')::text[]) AND r.acm_area_id <> sqlc.arg('acm_area_id');

-- name: ListAcmRegionOptions :many
-- Region picker: every region code used by an active vendor branch, with the
-- area currently holding it (NULL = free).
SELECT c.region_code, COUNT(*) AS branch_count, a.id AS acm_area_id, a.name AS acm_area_name
FROM (SELECT region_code::text AS region_code FROM vendor_branches WHERE is_active AND region_code IS NOT NULL) c
LEFT JOIN acm_area_regions r ON r.region_code = c.region_code
LEFT JOIN acm_areas a ON a.id = r.acm_area_id
GROUP BY c.region_code, a.id, a.name
ORDER BY c.region_code;

-- name: ListVaultPlanRegionWarnings :many
-- FR1.4: requests waiting in vault_assignment without a live plan because their
-- region has no active area, or the request has no region_code (old number format).
SELECT vr.id AS vendor_request_id, vr.request_number, vr.region_code, vr.replenish_date
FROM vendor_requests vr
WHERE vr.status = 'vault_assignment'
  AND NOT EXISTS (SELECT 1 FROM vendor_request_vault_plans p
                  WHERE p.vendor_request_id = vr.id AND p.status <> 'cancelled')
  AND NOT EXISTS (SELECT 1 FROM acm_area_regions r JOIN acm_areas a ON a.id = r.acm_area_id AND a.is_active
                  WHERE r.region_code = vr.region_code)
ORDER BY vr.replenish_date, vr.request_number;
