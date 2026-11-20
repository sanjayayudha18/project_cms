package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// CashCountAnomalyCategoryAdminRepository backs the admin "Kategori ATM Anomaly"
// endpoints (ScDD 5.1): list on the replica, submit-time lookups on the primary.
// Mutations run only inside CashCountAnomalyCategoryApplier (tx-scoped).
type CashCountAnomalyCategoryAdminRepository struct {
	queries     *db.Queries
	readQueries *db.Queries
}

// NewCashCountAnomalyCategoryAdminRepository wraps the primary (dbConn) and replica (readConn) pools.
func NewCashCountAnomalyCategoryAdminRepository(dbConn, readConn db.DBTX) *CashCountAnomalyCategoryAdminRepository {
	return &CashCountAnomalyCategoryAdminRepository{queries: db.New(dbConn), readQueries: db.New(readConn)}
}

// List returns categories filtered by status active|disabled|all (replica).
func (r *CashCountAnomalyCategoryAdminRepository) List(ctx context.Context, status string) ([]db.CashCountAnomalyCategory, error) {
	return r.readQueries.ListCashCountAnomalyCategoriesAdmin(ctx, status)
}

// GetByID returns a category incl. disabled rows; nil, nil if absent.
func (r *CashCountAnomalyCategoryAdminRepository) GetByID(ctx context.Context, id int64) (*db.GetCashCountAnomalyCategoryAdminByIDRow, error) {
	row, err := r.queries.GetCashCountAnomalyCategoryAdminByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// FindByCode returns the id of the category with this code (any status); nil, nil if none.
func (r *CashCountAnomalyCategoryAdminRepository) FindByCode(ctx context.Context, code string) (*int64, error) {
	id, err := r.queries.FindCashCountAnomalyCategoryByCode(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}
