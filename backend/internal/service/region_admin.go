// Package service: RegionAdminService backs the admin Region endpoints
// (.kiro/specs/region-management). Like rolemgmt.PermissionService, it is
// immediate-apply-with-audit -- a documented deviation from Golden Rule #3
// (design.md "Documented Deviation"): no maker-checker, but every mutation
// writes an audit_logs row inside the same transaction, and a failed audit
// write rolls back the mutation.
package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Sentinel errors. Handlers map these to HTTP status codes per design.md's
// error table (Req 2-4, 8): ErrRegionCodeConflict -> 409,
// ErrRegionCodeImmutable -> 400, ErrRegionHasActiveLocations -> 409,
// ErrRegionStatusUnchanged -> 409, ErrRegionNotFound -> 404. Authorization
// reuses the existing package-level ErrNotAuthorized (vendor_request.go) ->
// 403, rather than declaring a duplicate.
var (
	ErrRegionNotFound           = errors.New("region not found")
	ErrRegionCodeConflict       = errors.New("region code already exists")
	ErrRegionCodeImmutable      = errors.New("region code cannot be changed")
	ErrRegionHasActiveLocations = errors.New("region still has active locations")
	ErrRegionStatusUnchanged    = errors.New("region status unchanged")
)

// regionCodeRe restricts a region code to letters and digits only (Req
// 2.4/2.7, A2/A3), 1-20 characters after trimming (checked separately).
var regionCodeRe = regexp.MustCompile(`^[A-Za-z0-9]+$`)

// regionAdminRoles are re-checked at the service layer independent of the
// route guard (Req 5.2) -- "enforce at middleware AND service", same
// convention as masterDataMakerRoles/rolemgmt.authorizedActorRoles.
var regionAdminRoles = map[string]bool{
	"ADMIN":       true,
	"ADMIN_PARAM": true,
}

func isRegionAdminActor(actorRole string) bool {
	return regionAdminRoles[strings.ToUpper(strings.TrimSpace(actorRole))]
}

// normalizeRegionCode trims edge whitespace and uppercases, so uniqueness is
// enforced on the normalized form (Req 2.1, 2.7).
func normalizeRegionCode(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}

// RegionAdminRepo is the repository surface RegionAdminService needs.
// *repository.RegionAdminRepository satisfies this automatically; narrow so
// tests can fake it without a DB (mirrors rolemgmt.Repo's convention).
type RegionAdminRepo interface {
	List(ctx context.Context, arg db.ListRegionsAdminParams) ([]db.ListRegionsAdminRow, error)
	Count(ctx context.Context, arg db.CountRegionsAdminParams) (int64, error)
	GetByID(ctx context.Context, id int64) (*db.GetRegionAdminByIDRow, error)
	FindByCode(ctx context.Context, code string) (*int64, error)
	CountActiveLocations(ctx context.Context, regionID int64) (int64, error)
	CreateTx(ctx context.Context, tx pgx.Tx, arg db.CreateRegionAdminParams) (db.Region, error)
	UpdateNameTx(ctx context.Context, tx pgx.Tx, id int64, region string) (db.Region, error)
	SetActiveTx(ctx context.Context, tx pgx.Tx, id int64, active bool) (db.Region, error)
}

// RegionAdminPool is the transaction-opening surface RegionAdminService
// needs. *pgxpool.Pool satisfies this via Begin -- mirrors rolemgmt.Pool's
// narrow-interface convention.
type RegionAdminPool interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// RegionAdminService owns validation, code normalization, the service-layer
// authorization re-check, referential-integrity checks, and the audit-write
// guarantee for region create/update/disable/enable (Req 2-7). Mutations
// open their own transaction and build audit.NewWriter(tx) fresh inside it
// (rather than an injected pool-scoped writer) so a failed audit write rolls
// back the mutation -- same convention as rolemgmt.PermissionService.
type RegionAdminService struct {
	repo RegionAdminRepo
	pool RegionAdminPool
}

// NewRegionAdminService creates a RegionAdminService with the given dependencies.
func NewRegionAdminService(repo RegionAdminRepo, pool RegionAdminPool) *RegionAdminService {
	return &RegionAdminService{repo: repo, pool: pool}
}

// CreateRegionRequest holds the data for a POST /api/v1/admin/regions request (Req 2.1).
type CreateRegionRequest struct {
	Code   string
	Region string
}

// UpdateRegionNameRequest holds the data for a PUT /api/v1/admin/regions/{id}
// request. Code is optional and present only so the handler/service can
// detect and reject an attempt to change it (Req 3.2) -- nil/empty means "not
// sent", treated the same as resending the current value.
type UpdateRegionNameRequest struct {
	Code   *string
	Region string
}

// List returns a page of regions matching the given filters (replica read,
// Req 1.1-1.8). Read-only, no audit.
func (s *RegionAdminService) List(ctx context.Context, arg db.ListRegionsAdminParams) ([]db.ListRegionsAdminRow, error) {
	return s.repo.List(ctx, arg)
}

// Count returns the total number of regions matching the given filters (Req
// 1.2, 1.5). Read-only, no audit.
func (s *RegionAdminService) Count(ctx context.Context, arg db.CountRegionsAdminParams) (int64, error) {
	return s.repo.Count(ctx, arg)
}

// Get returns a single region by id (Req 8.4). Read-only, no audit.
func (s *RegionAdminService) Get(ctx context.Context, id int64) (*db.GetRegionAdminByIDRow, error) {
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("loading region: %w", err)
	}
	if row == nil {
		return nil, ErrRegionNotFound
	}
	return row, nil
}

// validateRegionName enforces Req 2.3/3.3: 1-100 characters after trimming.
func validateRegionName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", &ValidationError{Field: "region", Message: "nama tampilan wajib diisi"}
	}
	if len(trimmed) > 100 {
		return "", &ValidationError{Field: "region", Message: "nama tampilan maksimal 100 karakter"}
	}
	return trimmed, nil
}

// validateRegionCode enforces Req 2.4/2.7: 1-20 characters after trimming,
// alphanumeric only. Returns the normalized (trimmed+uppercased) form.
func validateRegionCode(code string) (string, error) {
	normalized := normalizeRegionCode(code)
	if normalized == "" {
		return "", &ValidationError{Field: "code", Message: "kode wajib diisi"}
	}
	if len(normalized) > 20 {
		return "", &ValidationError{Field: "code", Message: "kode maksimal 20 karakter"}
	}
	if !regionCodeRe.MatchString(normalized) {
		return "", &ValidationError{Field: "code", Message: "kode hanya boleh berisi huruf dan angka"}
	}
	return normalized, nil
}

// Create validates and inserts a new region, then writes a region_created
// audit entry inside the same transaction (Req 2.1-2.7, 5.2, 6.1-6.3). A
// failed audit write rolls back the insert -- no region can exist without an
// audit trail (design property 4).
func (s *RegionAdminService) Create(ctx context.Context, actorID int64, actorRole string, req CreateRegionRequest, actorIP string) (db.Region, error) {
	if !isRegionAdminActor(actorRole) {
		return db.Region{}, ErrNotAuthorized
	}

	code, err := validateRegionCode(req.Code)
	if err != nil {
		return db.Region{}, err
	}
	name, err := validateRegionName(req.Region)
	if err != nil {
		return db.Region{}, err
	}

	existing, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		return db.Region{}, fmt.Errorf("checking region code uniqueness: %w", err)
	}
	if existing != nil {
		return db.Region{}, ErrRegionCodeConflict
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.Region{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	created, err := s.repo.CreateTx(ctx, tx, db.CreateRegionAdminParams{Code: code, Region: &name})
	if err != nil {
		if isUniqueViolation(err) {
			// Pre-check above missed a race; the DB constraint is the source of truth.
			return db.Region{}, ErrRegionCodeConflict
		}
		return db.Region{}, fmt.Errorf("creating region: %w", err)
	}

	if err := audit.NewWriter(tx).Write(ctx, audit.Entry{
		ActorID:    actorID,
		Action:     "region_created",
		EntityType: "region",
		EntityID:   created.ID,
		After:      created,
		IP:         actorIP,
	}); err != nil {
		return db.Region{}, fmt.Errorf("write audit log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Region{}, fmt.Errorf("commit create region: %w", err)
	}
	return created, nil
}

// UpdateName changes a region's display name, then writes a
// region_updated audit entry (before/after) inside the same transaction (Req
// 3.1-3.6, 5.2, 6.1-6.3). Rejects an attempt to change code (Req 3.2). A
// failed audit write rolls back the update (design property 4).
func (s *RegionAdminService) UpdateName(ctx context.Context, actorID int64, actorRole string, id int64, req UpdateRegionNameRequest, actorIP string) (db.Region, error) {
	if !isRegionAdminActor(actorRole) {
		return db.Region{}, ErrNotAuthorized
	}

	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return db.Region{}, fmt.Errorf("loading region: %w", err)
	}
	if before == nil {
		return db.Region{}, ErrRegionNotFound
	}

	if req.Code != nil && normalizeRegionCode(*req.Code) != before.Code {
		return db.Region{}, ErrRegionCodeImmutable
	}

	name, err := validateRegionName(req.Region)
	if err != nil {
		return db.Region{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.Region{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	updated, err := s.repo.UpdateNameTx(ctx, tx, id, name)
	if err != nil {
		return db.Region{}, fmt.Errorf("updating region name: %w", err)
	}

	if err := audit.NewWriter(tx).Write(ctx, audit.Entry{
		ActorID:    actorID,
		Action:     "region_updated",
		EntityType: "region",
		EntityID:   id,
		Before:     map[string]any{"region": before.Region},
		After:      map[string]any{"region": updated.Region},
		IP:         actorIP,
	}); err != nil {
		return db.Region{}, fmt.Errorf("write audit log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Region{}, fmt.Errorf("commit update region name: %w", err)
	}
	return updated, nil
}

// Disable soft-deletes an active region (is_active=false, deleted_at=now()),
// blocked while any location still references it, then writes a
// region_deactivated audit entry inside the same transaction (Req 4.1/4.2,
// 4.5-4.7, 5.2, 6.1-6.3, 7.2). A failed audit write rolls back the status
// change (design property 4).
func (s *RegionAdminService) Disable(ctx context.Context, actorID int64, actorRole string, id int64, actorIP string) (db.Region, error) {
	return s.setActive(ctx, actorID, actorRole, id, false, actorIP)
}

// Enable reactivates an inactive region (is_active=true, deleted_at=NULL),
// then writes a region_reactivated audit entry inside the same transaction
// (Req 4.3, 4.5-4.7, 5.2, 6.1-6.3).
func (s *RegionAdminService) Enable(ctx context.Context, actorID int64, actorRole string, id int64, actorIP string) (db.Region, error) {
	return s.setActive(ctx, actorID, actorRole, id, true, actorIP)
}

func (s *RegionAdminService) setActive(ctx context.Context, actorID int64, actorRole string, id int64, active bool, actorIP string) (db.Region, error) {
	if !isRegionAdminActor(actorRole) {
		return db.Region{}, ErrNotAuthorized
	}

	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return db.Region{}, fmt.Errorf("loading region: %w", err)
	}
	if before == nil {
		return db.Region{}, ErrRegionNotFound
	}
	if before.IsActive == active {
		// No-op status change: reject without writing audit (Req 4.7).
		return db.Region{}, ErrRegionStatusUnchanged
	}

	if !active {
		count, err := s.repo.CountActiveLocations(ctx, id)
		if err != nil {
			return db.Region{}, fmt.Errorf("counting dependent locations: %w", err)
		}
		if count > 0 {
			return db.Region{}, fmt.Errorf("%w: %d", ErrRegionHasActiveLocations, count)
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.Region{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	updated, err := s.repo.SetActiveTx(ctx, tx, id, active)
	if err != nil {
		return db.Region{}, fmt.Errorf("updating region status: %w", err)
	}

	action := "region_deactivated"
	if active {
		action = "region_reactivated"
	}
	if err := audit.NewWriter(tx).Write(ctx, audit.Entry{
		ActorID:    actorID,
		Action:     action,
		EntityType: "region",
		EntityID:   id,
		Before:     map[string]any{"is_active": before.IsActive, "deleted_at": before.DeletedAt},
		After:      map[string]any{"is_active": updated.IsActive, "deleted_at": updated.DeletedAt},
		IP:         actorIP,
	}); err != nil {
		return db.Region{}, fmt.Errorf("write audit log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Region{}, fmt.Errorf("commit region status change: %w", err)
	}
	return updated, nil
}
