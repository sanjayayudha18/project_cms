package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// VendorBranchAdminRepository is the primary-pool repository backing the
// admin vendor branch CRUD endpoints (plan.md T3.1): list/get/find-by-code
// for VendorBranchAdminService (Submit-time validation + read), and
// create/update/disable/enable for VendorBranchApplier (apply-on-approve).
//
// ponytail: uses dbPool for both reads and writes; swap List/Count to the
// dbRead pool when DATABASE_REPLICA_URL wiring lands (same TODO convention
// as VendorAdminRepository/ATMAdminRepository).
type VendorBranchAdminRepository struct {
	queries *db.Queries
}

// NewVendorBranchAdminRepository creates a VendorBranchAdminRepository wrapping the given database connection.
func NewVendorBranchAdminRepository(dbConn db.DBTX) *VendorBranchAdminRepository {
	return &VendorBranchAdminRepository{queries: db.New(dbConn)}
}

// List returns a page of a vendor's branches matching the given filters.
func (r *VendorBranchAdminRepository) List(ctx context.Context, arg db.ListVendorBranchesAdminParams) ([]db.ListVendorBranchesAdminRow, error) {
	return r.queries.ListVendorBranchesAdmin(ctx, arg)
}

// Count returns the total number of branches matching List's filters.
func (r *VendorBranchAdminRepository) Count(ctx context.Context, arg db.CountVendorBranchesAdminParams) (int64, error) {
	return r.queries.CountVendorBranchesAdmin(ctx, arg)
}

// GetByID returns a vendor branch by id, including soft-deleted rows.
// Returns nil, nil if no matching row is found.
func (r *VendorBranchAdminRepository) GetByID(ctx context.Context, id int64) (*db.GetVendorBranchAdminByIDRow, error) {
	row, err := r.queries.GetVendorBranchAdminByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// FindByCode returns the id of the branch with the given branch_code
// (globally unique, incl. soft-deleted). Returns nil, nil if none found.
func (r *VendorBranchAdminRepository) FindByCode(ctx context.Context, code string) (*int64, error) {
	id, err := r.queries.FindVendorBranchAdminByCode(ctx, code)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}
