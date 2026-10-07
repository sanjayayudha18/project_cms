package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// RegionAdminRepository is the primary/replica-split repository backing the
// admin Region endpoints (.kiro/specs/region-management). Unlike
// ATMAdminRepository (maker-checker, no direct writes), region mutations are
// immediate-apply-with-audit (design.md "Documented Deviation"), so this
// repository exposes Tx-accepting mutation methods that share the caller's
// transaction with the audit write (mirrors rolemgmt.Repository).
type RegionAdminRepository struct {
	db     *db.Queries // primary: writes + read-after-write + FindByCode pre-check
	dbRead *db.Queries // replica: List/Count
}

// NewRegionAdminRepository creates a RegionAdminRepository. primary routes
// writes/read-after-write, replica routes List/Count (Req 9.1-9.3).
func NewRegionAdminRepository(primary, replica db.DBTX) *RegionAdminRepository {
	return &RegionAdminRepository{db: db.New(primary), dbRead: db.New(replica)}
}

// List returns a page of regions matching the given filters (replica read, Req 1.7, 9.2).
func (r *RegionAdminRepository) List(ctx context.Context, arg db.ListRegionsAdminParams) ([]db.ListRegionsAdminRow, error) {
	return r.dbRead.ListRegionsAdmin(ctx, arg)
}

// Count returns the total number of regions matching the given filters (replica read, Req 9.2).
func (r *RegionAdminRepository) Count(ctx context.Context, arg db.CountRegionsAdminParams) (int64, error) {
	return r.dbRead.CountRegionsAdmin(ctx, arg)
}

// GetByID returns a region by id, including soft-deleted rows (primary pool,
// read-after-write-safe, Req 9.3). Returns nil, nil if no matching region is found.
func (r *RegionAdminRepository) GetByID(ctx context.Context, id int64) (*db.GetRegionAdminByIDRow, error) {
	row, err := r.db.GetRegionAdminByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// FindByCode returns the id of the region with the given (already-normalized)
// code, including soft-deleted rows -- uniqueness pre-check on the primary
// pool (Req 2.2/2.7). Returns nil, nil if no matching region is found.
func (r *RegionAdminRepository) FindByCode(ctx context.Context, code string) (*int64, error) {
	id, err := r.db.FindRegionByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

// CountActiveLocations returns the number of locations rows referencing
// regionID -- the referential-integrity check backing Disable (Req 4.2/7.2).
// See regions_admin.sql's NOTE: locations has no is_active column, so this
// counts every referencing row (all are implicitly active).
func (r *RegionAdminRepository) CountActiveLocations(ctx context.Context, regionID int64) (int64, error) {
	return r.db.CountActiveLocationsByRegion(ctx, regionID)
}

// CreateTx inserts a new region inside the caller's transaction (primary
// pool). The caller (service layer) is responsible for beginning/committing/
// rolling back tx and for writing the audit entry in the same transaction
// (Req 2.5/2.6).
func (r *RegionAdminRepository) CreateTx(ctx context.Context, tx pgx.Tx, arg db.CreateRegionAdminParams) (db.Region, error) {
	return db.New(tx).CreateRegionAdmin(ctx, arg)
}

// UpdateNameTx updates a region's display name inside the caller's
// transaction (Req 3.1, 3.5/3.6). code is never touched here -- immutable
// after create, enforced by the service (Req 3.2).
func (r *RegionAdminRepository) UpdateNameTx(ctx context.Context, tx pgx.Tx, id int64, region string) (db.Region, error) {
	return db.New(tx).UpdateRegionName(ctx, db.UpdateRegionNameParams{ID: id, Region: &region})
}

// SetActiveTx toggles is_active/deleted_at inside the caller's transaction
// (Req 4.1/4.3, 4.5/4.6): active=true enables (is_active=true,
// deleted_at=NULL), active=false disables (is_active=false, deleted_at=now()).
// The WHERE guard on each underlying query makes a no-op toggle affect 0
// rows, surfacing as pgx.ErrNoRows here -- the service pre-checks status
// before calling, so this is defense in depth (Req 4.7).
func (r *RegionAdminRepository) SetActiveTx(ctx context.Context, tx pgx.Tx, id int64, active bool) (db.Region, error) {
	q := db.New(tx)
	if active {
		return q.EnableRegionAdmin(ctx, id)
	}
	return q.DisableRegionAdmin(ctx, id)
}
