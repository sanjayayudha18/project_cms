package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// BranchATMRepository is the read-replica repository backing the read-only
// "ATM" sub-tab on the vendor branch detail page (.kiro/specs/
// vendor-branch-atms): resolves the ATMs managed by a vendor branch through
// its packages' atm_vendor_packages assignments. Display-only, no mutations.
type BranchATMRepository struct {
	queries *db.Queries
}

// NewBranchATMRepository creates a BranchATMRepository wrapping the given database connection (dbRead replica pool).
func NewBranchATMRepository(dbConn db.DBTX) *BranchATMRepository {
	return &BranchATMRepository{queries: db.New(dbConn)}
}

// List returns a page of ATMs managed by the branch named in arg.VendorBranchID.
func (r *BranchATMRepository) List(ctx context.Context, arg db.ListBranchATMsParams) ([]db.ListBranchATMsRow, error) {
	return r.queries.ListBranchATMs(ctx, arg)
}

// Count returns the total number of ATMs managed by branchID.
func (r *BranchATMRepository) Count(ctx context.Context, branchID int64) (int64, error) {
	return r.queries.CountBranchATMs(ctx, &branchID)
}

// BranchVendorID returns the vendor_id owning branchID; nil, nil if the branch doesn't exist.
func (r *BranchATMRepository) BranchVendorID(ctx context.Context, branchID int64) (*int64, error) {
	id, err := r.queries.GetVendorBranchVendorID(ctx, branchID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}
