package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// CashCountAreaAdminRepository backs the admin "Area Cash Count" endpoints (ScDD 5.1):
// list screens on the replica, submit-time lookups on the primary. Mutations run only
// inside CashCountAreaApplier (tx-scoped).
type CashCountAreaAdminRepository struct {
	queries     *db.Queries
	readQueries *db.Queries
}

// NewCashCountAreaAdminRepository wraps the primary (dbConn) and replica (readConn) pools.
func NewCashCountAreaAdminRepository(dbConn, readConn db.DBTX) *CashCountAreaAdminRepository {
	return &CashCountAreaAdminRepository{queries: db.New(dbConn), readQueries: db.New(readConn)}
}

// List returns areas (with region codes) filtered by status (replica).
func (r *CashCountAreaAdminRepository) List(ctx context.Context, status string) ([]db.ListCashCountAreasAdminRow, error) {
	return r.readQueries.ListCashCountAreasAdmin(ctx, status)
}

// GetByID returns an area incl. disabled rows; nil, nil if absent.
func (r *CashCountAreaAdminRepository) GetByID(ctx context.Context, id int64) (*db.GetCashCountAreaAdminByIDRow, error) {
	row, err := r.queries.GetCashCountAreaAdminByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// FindByCode returns the id of the area with this code (any status); nil, nil if none.
func (r *CashCountAreaAdminRepository) FindByCode(ctx context.Context, code string) (*int64, error) {
	id, err := r.queries.FindCashCountAreaByCode(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// RegionConflicts returns the given region codes already owned by another active area.
func (r *CashCountAreaAdminRepository) RegionConflicts(ctx context.Context, regionCodes []string, excludeAreaID int64) ([]db.ListCashCountRegionConflictsRow, error) {
	return r.queries.ListCashCountRegionConflicts(ctx, db.ListCashCountRegionConflictsParams{RegionCodes: regionCodes, ExcludeAreaID: excludeAreaID})
}

// UnknownRegionCodes returns the given region codes no active vendor_branches row uses.
func (r *CashCountAreaAdminRepository) UnknownRegionCodes(ctx context.Context, regionCodes []string) ([]string, error) {
	return r.queries.ListUnknownCashCountRegionCodes(ctx, regionCodes)
}

// ActiveExecutorCount returns the active EXECUTOR PICs mapped to the area.
func (r *CashCountAreaAdminRepository) ActiveExecutorCount(ctx context.Context, areaID int64) (int64, error) {
	return r.queries.CountActiveCashCountExecutorsInArea(ctx, &areaID)
}
