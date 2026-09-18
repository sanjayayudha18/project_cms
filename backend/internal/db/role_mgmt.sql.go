// Hand-written to match sqlc's exact output convention. sqlc generate is
// blocked by a pre-existing bug in migration 017 (project-context.md Sec 12);
// mirrors internal/db/{audit,approval}.sql.go until 017 is fixed and this file
// can be regenerated.
// source: role_mgmt.sql

package db

import (
	"context"
)

const listCatalog = `-- name: ListCatalog :many
SELECT id, parent_id, key, label, kind, sort_order, is_active, created_at, updated_at
FROM menu_features
WHERE is_active = true
ORDER BY sort_order ASC, id ASC
`

// Full active menu/feature catalog for the Role Management editor (Req 1.1).
func (q *Queries) ListCatalog(ctx context.Context) ([]MenuFeature, error) {
	rows, err := q.db.Query(ctx, listCatalog)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []MenuFeature{}
	for rows.Next() {
		var i MenuFeature
		if err := rows.Scan(
			&i.ID,
			&i.ParentID,
			&i.Key,
			&i.Label,
			&i.Kind,
			&i.SortOrder,
			&i.IsActive,
			&i.CreatedAt,
			&i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const countCatalogEntriesByIDs = `-- name: CountCatalogEntriesByIDs :one
SELECT COUNT(*) FROM menu_features WHERE id = ANY($1::bigint[])
`

// Backs CatalogEntriesExist (Req 3.3): caller compares the returned count
// against len(ids) to detect any id that does not exist in the catalog.
func (q *Queries) CountCatalogEntriesByIDs(ctx context.Context, ids []int64) (int64, error) {
	row := q.db.QueryRow(ctx, countCatalogEntriesByIDs, ids)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const hasRolePermission = `-- name: HasRolePermission :one
SELECT EXISTS (
    SELECT 1
    FROM role_permissions rp
    JOIN roles r ON r.id = rp.role_id
    JOIN menu_features mf ON mf.id = rp.menu_feature_id
    WHERE r.role = $1 AND mf.key = $2
) AS exists
`

type HasRolePermissionParams struct {
	Role string `json:"role"`
	Key  string `json:"key"`
}

// Backs PermissionEvaluator.HasPermission (Req 1.3, 5.3): true iff a
// role_permissions row links this exact (role, catalog key) pair. Decision
// comes purely from the mapping -- no hardcoded role list is consulted here.
func (q *Queries) HasRolePermission(ctx context.Context, arg HasRolePermissionParams) (bool, error) {
	row := q.db.QueryRow(ctx, hasRolePermission, arg.Role, arg.Key)
	var exists bool
	err := row.Scan(&exists)
	return exists, err
}

const findRoleByName = `-- name: FindRoleByName :one
SELECT id FROM roles WHERE role = $1
`

// Uniqueness pre-check for CreateRole (Req 2.3), case-sensitive to match the
// roles.role column's existing seed convention (all-caps role names).
func (q *Queries) FindRoleByName(ctx context.Context, role string) (int64, error) {
	row := q.db.QueryRow(ctx, findRoleByName, role)
	var id int64
	err := row.Scan(&id)
	return id, err
}

const createRole = `-- name: CreateRole :one
INSERT INTO roles (role, description)
VALUES ($1, $2)
RETURNING id, role, description, created_at, updated_at
`

type CreateRoleParams struct {
	Role        string  `json:"role"`
	Description *string `json:"description"`
}

// New role is inserted with no role_permissions rows — zero access by
// default (Req 2.2, 5.1). Caller grants access via ReplaceRolePermissions.
func (q *Queries) CreateRole(ctx context.Context, arg CreateRoleParams) (Role, error) {
	row := q.db.QueryRow(ctx, createRole, arg.Role, arg.Description)
	var i Role
	err := row.Scan(
		&i.ID,
		&i.Role,
		&i.Description,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const getRole = `-- name: GetRole :one
SELECT id, role, description, created_at, updated_at FROM roles WHERE id = $1
`

// Read-after-write on the primary pool (Req 8.3).
func (q *Queries) GetRole(ctx context.Context, id int64) (Role, error) {
	row := q.db.QueryRow(ctx, getRole, id)
	var i Role
	err := row.Scan(
		&i.ID,
		&i.Role,
		&i.Description,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const listRolesWithPermissions = `-- name: ListRolesWithPermissions :many
SELECT r.id AS role_id, r.role, r.description,
       mf.id AS menu_feature_id, mf.parent_id AS menu_feature_parent_id,
       mf.key AS menu_feature_key, mf.label AS menu_feature_label, mf.kind AS menu_feature_kind
FROM roles r
LEFT JOIN role_permissions rp ON rp.role_id = r.id
LEFT JOIN menu_features mf ON mf.id = rp.menu_feature_id
ORDER BY r.id ASC, mf.sort_order ASC NULLS LAST, mf.id ASC
`

type ListRolesWithPermissionsRow struct {
	RoleID              int64   `json:"role_id"`
	Role                string  `json:"role"`
	Description         *string `json:"description"`
	MenuFeatureID       *int64  `json:"menu_feature_id"`
	MenuFeatureParentID *int64  `json:"menu_feature_parent_id"`
	MenuFeatureKey      *string `json:"menu_feature_key"`
	MenuFeatureLabel    *string `json:"menu_feature_label"`
	MenuFeatureKind     *string `json:"menu_feature_kind"`
}

// Flat role x permission rows for the Role Management list (Req 3.5); the
// repository groups rows by role_id into RoleWithPermissions. A role with no
// grants yields one row with all MenuFeature* fields NULL (LEFT JOIN).
func (q *Queries) ListRolesWithPermissions(ctx context.Context) ([]ListRolesWithPermissionsRow, error) {
	rows, err := q.db.Query(ctx, listRolesWithPermissions)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ListRolesWithPermissionsRow{}
	for rows.Next() {
		var i ListRolesWithPermissionsRow
		if err := rows.Scan(
			&i.RoleID,
			&i.Role,
			&i.Description,
			&i.MenuFeatureID,
			&i.MenuFeatureParentID,
			&i.MenuFeatureKey,
			&i.MenuFeatureLabel,
			&i.MenuFeatureKind,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listRolePermissionIDs = `-- name: ListRolePermissionIDs :many
SELECT menu_feature_id FROM role_permissions WHERE role_id = $1
`

// "Before" snapshot read on the primary pool ahead of a full-set replace
// (Req 3.4 audit before/after; Req 8.3 read-after-write).
func (q *Queries) ListRolePermissionIDs(ctx context.Context, roleID int64) ([]int64, error) {
	rows, err := q.db.Query(ctx, listRolePermissionIDs, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []int64{}
	for rows.Next() {
		var menuFeatureID int64
		if err := rows.Scan(&menuFeatureID); err != nil {
			return nil, err
		}
		items = append(items, menuFeatureID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const deleteRolePermissions = `-- name: DeleteRolePermissions :exec
DELETE FROM role_permissions WHERE role_id = $1
`

// First half of the full-set replace used by ReplaceRolePermissions (Req 3.1,
// property 5). Caller runs this and InsertRolePermission in the same tx.
func (q *Queries) DeleteRolePermissions(ctx context.Context, roleID int64) error {
	_, err := q.db.Exec(ctx, deleteRolePermissions, roleID)
	return err
}

const insertRolePermission = `-- name: InsertRolePermission :exec
INSERT INTO role_permissions (role_id, menu_feature_id, granted_by)
VALUES ($1, $2, $3)
ON CONFLICT (role_id, menu_feature_id) DO NOTHING
`

type InsertRolePermissionParams struct {
	RoleID        int64 `json:"role_id"`
	MenuFeatureID int64 `json:"menu_feature_id"`
	GrantedBy     int64 `json:"granted_by"`
}

// Second half of the full-set replace; ON CONFLICT DO NOTHING mirrors the
// role_permissions_role_feature_uq constraint (idempotent grant).
func (q *Queries) InsertRolePermission(ctx context.Context, arg InsertRolePermissionParams) error {
	_, err := q.db.Exec(ctx, insertRolePermission, arg.RoleID, arg.MenuFeatureID, arg.GrantedBy)
	return err
}
