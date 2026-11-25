-- name: ListCatalog :many
-- Full active menu/feature catalog for the Role Management editor (Req 1.1).
-- Includes assignable_roles (NULL = all internal roles; migration 046).
SELECT id, parent_id, key, label, kind, sort_order, is_active, created_at, updated_at, assignable_roles, portal
FROM menu_features
WHERE is_active = true
ORDER BY sort_order ASC, id ASC;

-- name: CountCatalogEntriesByIDs :one
-- Backs CatalogEntriesExist (Req 3.3): caller compares the returned count
-- against len(ids) to detect any id that does not exist in the catalog.
SELECT COUNT(*) FROM menu_features WHERE id = ANY(@ids::bigint[]);

-- name: HasRolePermission :one
-- Backs PermissionEvaluator.HasPermission (Req 1.3, 5.3): true iff a
-- role_permissions row links this exact (role, catalog key) pair. Decision
-- comes purely from the mapping -- no hardcoded role list is consulted here.
SELECT EXISTS (
    SELECT 1
    FROM role_permissions rp
    JOIN roles r ON r.id = rp.role_id
    JOIN menu_features mf ON mf.id = rp.menu_feature_id
    WHERE r.role = @role AND mf.key = @key
) AS exists;

-- name: HasAnyRolePermission :one
-- Any-of variant of HasRolePermission for RequirePermission(keys...) (task 2.3,
-- Req 7.1): true iff the role holds at least one of the catalog keys.
SELECT EXISTS (
    SELECT 1
    FROM role_permissions rp
    JOIN roles r ON r.id = rp.role_id
    JOIN menu_features mf ON mf.id = rp.menu_feature_id
    WHERE r.role = @role AND mf.key = ANY(@keys::text[])
) AS exists;

-- name: ListRoleFeatureKeys :many
-- Catalog keys granted to a role, in menu order, for GET /auth/me/permissions
-- (task 3.1, Req 4.1). Only active catalog rows; Always_On/Locked keys are
-- added by the caller, not read from here.
SELECT mf.key
FROM role_permissions rp
JOIN roles r ON r.id = rp.role_id
JOIN menu_features mf ON mf.id = rp.menu_feature_id
WHERE r.role = @role AND mf.is_active = true
ORDER BY mf.sort_order ASC, mf.id ASC;

-- name: FindRoleByName :one
-- Uniqueness pre-check for CreateRole (Req 2.3), case-sensitive to match the
-- roles.role column's existing seed convention (all-caps role names).
SELECT id FROM roles WHERE role = $1;

-- name: CreateRole :one
-- New role is inserted with no role_permissions rows — zero access by
-- default (Req 2.2, 5.1). Caller grants access via ReplaceRolePermissions.
-- portal/party_scope are validated by the service and backed by roles_portal_prefix_chk +
-- roles_party_scope_portal_chk (migration 047).
INSERT INTO roles (role, description, portal, party_scope)
VALUES ($1, $2, $3, $4)
RETURNING id, role, description, created_at, updated_at, portal, party_scope;

-- name: GetRole :one
-- Read-after-write on the primary pool (Req 8.3).
SELECT id, role, description, created_at, updated_at, portal, party_scope FROM roles WHERE id = $1;

-- name: ListRolesWithPermissions :many
-- Flat role x permission rows for the Role Management list (Req 3.5); the
-- repository groups rows by role_id into RoleWithPermissions. A role with no
-- grants yields one row with all MenuFeature* fields NULL (LEFT JOIN).
-- active_user_count = active, non-deleted users holding the role (role-management-revamp Req 6.3).
SELECT r.id AS role_id, r.role, r.description, r.portal, r.party_scope,
       (SELECT COUNT(*) FROM users u
         WHERE u.role_id = r.id AND u.is_active = true AND u.deleted_at IS NULL)::bigint AS active_user_count,
       mf.id AS menu_feature_id, mf.parent_id AS menu_feature_parent_id,
       mf.key AS menu_feature_key, mf.label AS menu_feature_label, mf.kind AS menu_feature_kind
FROM roles r
LEFT JOIN role_permissions rp ON rp.role_id = r.id
LEFT JOIN menu_features mf ON mf.id = rp.menu_feature_id
ORDER BY r.id ASC, mf.sort_order ASC NULLS LAST, mf.id ASC;

-- name: ListRolePermissionIDs :many
-- "Before" snapshot read on the primary pool ahead of a full-set replace
-- (Req 3.4 audit before/after; Req 8.3 read-after-write).
SELECT menu_feature_id FROM role_permissions WHERE role_id = $1;

-- name: DeleteRolePermissions :exec
-- First half of the full-set replace used by ReplaceRolePermissions (Req 3.1,
-- property 5). Caller runs this and InsertRolePermission in the same tx.
DELETE FROM role_permissions WHERE role_id = $1;

-- name: InsertRolePermission :exec
-- Second half of the full-set replace; ON CONFLICT DO NOTHING mirrors the
-- role_permissions_role_feature_uq constraint (idempotent grant).
INSERT INTO role_permissions (role_id, menu_feature_id, granted_by)
VALUES ($1, $2, $3)
ON CONFLICT (role_id, menu_feature_id) DO NOTHING;

-- name: GetRolePartyScope :one
-- Backs vendorrole.ScopeReader (replica). Unknown role -> no rows; non-vendor role -> NULL.
SELECT party_scope FROM roles WHERE role = $1;

-- name: ListVendorRolesForParty :many
-- Vendor roles whose party_scope contains the party (replenish|vault); replaces vendorrole.ForParty.
SELECT role FROM roles
WHERE portal = 'vendor' AND @party::text = ANY(party_scope)
ORDER BY role;
