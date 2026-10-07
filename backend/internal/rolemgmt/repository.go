package rolemgmt

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Repository is the repository backing Role Management (Req 8: writes +
// read-after-write -> primary, list/catalog reads -> replica).
//
// ponytail: dbRead points at the same connection as db until
// DATABASE_REPLICA_URL wiring lands (same TODO convention as
// VendorAdminRepository / AuditLogRepository).
type Repository struct {
	db     *db.Queries // primary: writes + read-after-write
	dbRead *db.Queries // replica: list/catalog reads
}

// NewRepository creates a Repository. Pass the primary pool for both
// arguments until the replica pool is wired (see struct doc).
func NewRepository(primary, replica db.DBTX) *Repository {
	return &Repository{db: db.New(primary), dbRead: db.New(replica)}
}

// HasPermission reports whether roleName is mapped to the catalog entry with
// the given featureKey (replica read; Req 1.3, 5.3 — decision comes purely
// from role_permissions, no hardcoded role list).
func (r *Repository) HasPermission(ctx context.Context, roleName, featureKey string) (bool, error) {
	return r.dbRead.HasRolePermission(ctx, db.HasRolePermissionParams{Role: roleName, Key: featureKey})
}

// ListCatalog returns the full active menu/feature catalog (replica read, Req 1.1).
func (r *Repository) ListCatalog(ctx context.Context) ([]db.MenuFeature, error) {
	return r.dbRead.ListCatalog(ctx)
}

// CatalogEntriesExist reports whether every id in ids exists in
// menu_features (replica read; Req 3.3 validation). An empty ids slice is
// vacuously true (a permission update that grants nothing is always valid).
func (r *Repository) CatalogEntriesExist(ctx context.Context, ids []int64) (bool, error) {
	distinct := distinctInt64s(ids)
	if len(distinct) == 0 {
		return true, nil
	}
	count, err := r.dbRead.CountCatalogEntriesByIDs(ctx, distinct)
	if err != nil {
		return false, err
	}
	return int(count) == len(distinct), nil
}

// ListRolesWithPermissions returns every role with its mapped catalog
// entries, flattened one row per (role, granted entry) pair — a role with no
// grants yields one row with all MenuFeature* fields nil (replica read, Req 3.5).
func (r *Repository) ListRolesWithPermissions(ctx context.Context) ([]db.ListRolesWithPermissionsRow, error) {
	return r.dbRead.ListRolesWithPermissions(ctx)
}

// FindRoleByName returns the id of the role with the given name, or nil if
// none exists (Req 2.3 uniqueness pre-check).
func (r *Repository) FindRoleByName(ctx context.Context, name string) (*int64, error) {
	id, err := r.db.FindRoleByName(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

// CreateRole inserts a new role on the primary pool. The new role has no
// role_permissions rows -- zero access by default (Req 2.2, 5.1).
func (r *Repository) CreateRole(ctx context.Context, arg db.CreateRoleParams) (db.Role, error) {
	return r.db.CreateRole(ctx, arg)
}

// GetRole reads a role back from the primary pool (read-after-write, Req 8.3).
// Returns nil, nil if no matching role is found.
func (r *Repository) GetRole(ctx context.Context, id int64) (*db.Role, error) {
	role, err := r.db.GetRole(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &role, nil
}

// ListRolePermissionIDs returns the catalog entry ids currently granted to a
// role, read from the primary pool -- the "before" snapshot taken ahead of a
// full-set replace (Req 3.4 audit before/after; Req 8.3 read-after-write).
func (r *Repository) ListRolePermissionIDs(ctx context.Context, roleID int64) ([]int64, error) {
	return r.db.ListRolePermissionIDs(ctx, roleID)
}

// ReplaceRolePermissions performs a full-set replace (delete-then-insert) for
// a role inside the caller's transaction (Req 3.1, design property 5).
// grantedBy is recorded on every inserted row. The caller (service layer) is
// responsible for beginning/committing/rolling back tx and for writing the
// audit entry in the same transaction (Req 6.3).
func (r *Repository) ReplaceRolePermissions(ctx context.Context, tx pgx.Tx, roleID int64, featureIDs []int64, grantedBy int64) error {
	q := db.New(tx)
	if err := q.DeleteRolePermissions(ctx, roleID); err != nil {
		return err
	}
	for _, featureID := range distinctInt64s(featureIDs) {
		if err := q.InsertRolePermission(ctx, db.InsertRolePermissionParams{
			RoleID:        roleID,
			MenuFeatureID: featureID,
			GrantedBy:     grantedBy,
		}); err != nil {
			return err
		}
	}
	return nil
}

// distinctInt64s returns ids with duplicates removed, preserving first
// occurrence order. Used so a caller-submitted set with duplicate ids doesn't
// throw off the CatalogEntriesExist count check or insert redundant rows
// (role_permissions_role_feature_uq would reject them anyway, but this keeps
// the grant loop from doing needless round-trips).
func distinctInt64s(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
