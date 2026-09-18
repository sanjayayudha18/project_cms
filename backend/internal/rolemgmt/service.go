package rolemgmt

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

const pgUniqueViolation = "23505"

// roleNameRe restricts role names to uppercase letters, digits, and hyphens
// -- matches the frontend's createRoleSchema regex (design.md) and this
// codebase's existing seed convention (ADMIN, ATM-USER, BRANCH-ATM-SPV, ...).
var roleNameRe = regexp.MustCompile(`^[A-Z0-9-]+$`)

// authorizedActorRoles are re-checked at the service layer independent of the
// route guard (Req 4.3/4.4) -- "enforce at middleware AND service".
var authorizedActorRoles = map[string]bool{
	"APPACCESS": true,
	"ADMIN":     true,
}

// Repo is the repository surface PermissionService needs. *Repository
// satisfies this automatically; narrow so tests can fake it without a DB
// (mirrors VendorAdminRepo's narrow-interface convention).
type Repo interface {
	ListCatalog(ctx context.Context) ([]db.MenuFeature, error)
	CatalogEntriesExist(ctx context.Context, ids []int64) (bool, error)
	ListRolesWithPermissions(ctx context.Context) ([]db.ListRolesWithPermissionsRow, error)
	FindRoleByName(ctx context.Context, name string) (*int64, error)
	GetRole(ctx context.Context, id int64) (*db.Role, error)
	ListRolePermissionIDs(ctx context.Context, roleID int64) ([]int64, error)
	ReplaceRolePermissions(ctx context.Context, tx pgx.Tx, roleID int64, featureIDs []int64, grantedBy int64) error
}

// Pool is the transaction-opening surface PermissionService needs.
// *pgxpool.Pool satisfies this via Begin. Narrow so tests can fake a
// failing/no-op tx without a real DB.
type Pool interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// PermissionService owns validation, uniqueness resolution, the service-layer
// authorization re-check, and the audit-write guarantee for Role Management
// create/update (Req 2-4, 6). Apply-immediately-with-audit, no maker-checker
// -- documented deviation from Golden Rule #3 (design.md "Documented
// Deviation"). Mutations open their own transaction and build
// audit.NewWriter(tx) fresh inside it (rather than taking an injected
// pool-scoped audit writer) so a failed audit write rolls back the mutation
// -- same convention VendorRequestService uses and explicitly prefers over
// design.md's originally-sketched injected-writer shape, for the same reason:
// a pool-scoped writer would write outside the transaction and break
// atomicity.
type PermissionService struct {
	repo Repo
	pool Pool
}

// NewPermissionService creates a PermissionService with the given dependencies.
func NewPermissionService(repo Repo, pool Pool) *PermissionService {
	return &PermissionService{repo: repo, pool: pool}
}

// CreateRoleRequest holds the data for a POST /api/v1/admin/roles request (Req 2.1).
type CreateRoleRequest struct {
	Role        string
	Description string
}

// isAuthorizedActor reports whether actorRole is APPACCESS or ADMIN,
// case-insensitively (matches middleware.RequireRoles' convention).
func isAuthorizedActor(actorRole string) bool {
	return authorizedActorRoles[strings.ToUpper(strings.TrimSpace(actorRole))]
}

// ListRoles returns every role with its mapped catalog entries (Req 3.5).
// Replica read, no audit.
func (s *PermissionService) ListRoles(ctx context.Context) ([]db.ListRolesWithPermissionsRow, error) {
	return s.repo.ListRolesWithPermissions(ctx)
}

// ListCatalog returns the full active menu/feature catalog (Req 1.1).
// Replica read, no audit.
func (s *PermissionService) ListCatalog(ctx context.Context) ([]db.MenuFeature, error) {
	return s.repo.ListCatalog(ctx)
}

// CreateRole validates and inserts a new role with zero access, then writes
// a role_created audit entry inside the same transaction (Req 2.1-2.5,
// 4.3-4.4, 5.1, 6.1-6.3). A failed audit write rolls back the insert -- no
// role can exist without an audit trail (Req 6.3, design property 4).
func (s *PermissionService) CreateRole(ctx context.Context, actorID int64, actorRole string, req CreateRoleRequest, actorIP string) (db.Role, error) {
	if !isAuthorizedActor(actorRole) {
		return db.Role{}, ErrNotAuthorized
	}

	role := strings.TrimSpace(req.Role)
	if role == "" {
		return db.Role{}, &ValidationError{Field: "role", Message: "nama peran wajib diisi"}
	}
	if !roleNameRe.MatchString(role) {
		return db.Role{}, &ValidationError{Field: "role", Message: "gunakan huruf kapital, angka, dan tanda hubung"}
	}

	existing, err := s.repo.FindRoleByName(ctx, role)
	if err != nil {
		return db.Role{}, fmt.Errorf("checking role name uniqueness: %w", err)
	}
	if existing != nil {
		return db.Role{}, ErrRoleNameConflict
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.Role{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	created, err := db.New(tx).CreateRole(ctx, db.CreateRoleParams{
		Role:        role,
		Description: nilIfEmpty(strings.TrimSpace(req.Description)),
	})
	if err != nil {
		if isUniqueViolation(err) {
			// Pre-check above missed a race; the DB constraint is the source of truth.
			return db.Role{}, ErrRoleNameConflict
		}
		return db.Role{}, fmt.Errorf("creating role: %w", err)
	}

	if err := audit.NewWriter(tx).Write(ctx, audit.Entry{
		ActorID:    actorID,
		Action:     "role_created",
		EntityType: "role",
		EntityID:   created.ID,
		After:      created,
		IP:         actorIP,
	}); err != nil {
		return db.Role{}, fmt.Errorf("write audit log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Role{}, fmt.Errorf("commit create role: %w", err)
	}
	return created, nil
}

// UpdateRolePermissions replaces the full set of catalog entries granted to
// a role, then writes a role_permissions_updated audit entry (before/after)
// inside the same transaction (Req 3.1-3.4, 4.3-4.4, 6.1-6.3). A failed audit
// write rolls back the replace (Req 6.3, design property 4); an invalid
// catalog entry id leaves the existing mapping untouched (Req 3.3, design
// property 6).
func (s *PermissionService) UpdateRolePermissions(ctx context.Context, actorID int64, actorRole string, roleID int64, featureIDs []int64, actorIP string) ([]int64, error) {
	if !isAuthorizedActor(actorRole) {
		return nil, ErrNotAuthorized
	}

	role, err := s.repo.GetRole(ctx, roleID)
	if err != nil {
		return nil, fmt.Errorf("loading role: %w", err)
	}
	if role == nil {
		return nil, ErrRoleNotFound
	}

	ok, err := s.repo.CatalogEntriesExist(ctx, featureIDs)
	if err != nil {
		return nil, fmt.Errorf("checking catalog entries: %w", err)
	}
	if !ok {
		return nil, ErrCatalogEntryNotFound
	}

	before, err := s.repo.ListRolePermissionIDs(ctx, roleID)
	if err != nil {
		return nil, fmt.Errorf("loading current permissions: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.repo.ReplaceRolePermissions(ctx, tx, roleID, featureIDs, actorID); err != nil {
		return nil, fmt.Errorf("replacing role permissions: %w", err)
	}

	if err := audit.NewWriter(tx).Write(ctx, audit.Entry{
		ActorID:    actorID,
		Action:     "role_permissions_updated",
		EntityType: "role",
		EntityID:   roleID,
		Before:     map[string]any{"menu_feature_ids": before},
		After:      map[string]any{"menu_feature_ids": featureIDs},
		IP:         actorIP,
	}); err != nil {
		return nil, fmt.Errorf("write audit log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit update role permissions: %w", err)
	}
	return featureIDs, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}
