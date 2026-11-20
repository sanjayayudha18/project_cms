package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// CashCountCategoryParamsAdminRepository backs "Parameter Kategori Vault"
// (ScDD 5.1, AC-5.1-01/04): history list on the replica, the active set on the
// primary (submit-time snapshot). Mutations run only inside
// CashCountCategoryParamsApplier (tx-scoped).
type CashCountCategoryParamsAdminRepository struct {
	queries     *db.Queries
	readQueries *db.Queries
}

// NewCashCountCategoryParamsAdminRepository wraps the primary (dbConn) and replica (readConn) pools.
func NewCashCountCategoryParamsAdminRepository(dbConn, readConn db.DBTX) *CashCountCategoryParamsAdminRepository {
	return &CashCountCategoryParamsAdminRepository{queries: db.New(dbConn), readQueries: db.New(readConn)}
}

// List returns every set, active first, then history newest first (replica).
func (r *CashCountCategoryParamsAdminRepository) List(ctx context.Context) ([]db.CashCountCategoryParam, error) {
	return r.readQueries.ListCashCountCategoryParamsAdmin(ctx)
}

// GetActive returns the single active set (primary); nil, nil if none.
func (r *CashCountCategoryParamsAdminRepository) GetActive(ctx context.Context) (*db.CashCountCategoryParam, error) {
	row, err := r.queries.GetActiveCashCountCategoryParams(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
