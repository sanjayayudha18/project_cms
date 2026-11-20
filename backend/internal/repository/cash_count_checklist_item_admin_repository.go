package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// CashCountChecklistItemAdminRepository backs the admin "Item checklist BA" endpoints
// (ScDD 5.1): list on the replica, submit-time lookups on the primary.
// Mutations run only inside CashCountChecklistItemApplier (tx-scoped).
type CashCountChecklistItemAdminRepository struct {
	queries     *db.Queries
	readQueries *db.Queries
}

// NewCashCountChecklistItemAdminRepository wraps the primary (dbConn) and replica (readConn) pools.
func NewCashCountChecklistItemAdminRepository(dbConn, readConn db.DBTX) *CashCountChecklistItemAdminRepository {
	return &CashCountChecklistItemAdminRepository{queries: db.New(dbConn), readQueries: db.New(readConn)}
}

// List returns items filtered by category (empty = all) and status (replica).
func (r *CashCountChecklistItemAdminRepository) List(ctx context.Context, arg db.ListCashCountChecklistItemsAdminParams) ([]db.CashCountChecklistItem, error) {
	return r.readQueries.ListCashCountChecklistItemsAdmin(ctx, arg)
}

// GetByID returns an item incl. disabled rows; nil, nil if absent.
func (r *CashCountChecklistItemAdminRepository) GetByID(ctx context.Context, id int64) (*db.GetCashCountChecklistItemAdminByIDRow, error) {
	row, err := r.queries.GetCashCountChecklistItemAdminByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// FindActivePoint returns the id of another active item with the same (category, day, point_no);
// nil, nil if none. day = "" for NON_MACHINE; excludeID 0 = none.
func (r *CashCountChecklistItemAdminRepository) FindActivePoint(ctx context.Context, category, day string, pointNo int16, excludeID int64) (*int64, error) {
	id, err := r.queries.FindActiveCashCountChecklistPoint(ctx, db.FindActiveCashCountChecklistPointParams{
		Category: category, Day: day, PointNo: pointNo, ExcludeID: excludeID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}
