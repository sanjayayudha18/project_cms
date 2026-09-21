package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// VendorPackageAdminRepository is the primary-pool repository backing the
// admin vendor package endpoints (plan.md T3.4): reads + submit-time lookups
// for VendorPackageAdminService. Mutation queries run only inside
// VendorPackageApplier (tx-scoped), not through this repository.
//
// ponytail: primary pool for reads too; swap List/Count to dbRead when
// DATABASE_REPLICA_URL wiring lands (same TODO as VendorBranchAdminRepository).
type VendorPackageAdminRepository struct {
	queries *db.Queries
}

// NewVendorPackageAdminRepository creates a VendorPackageAdminRepository wrapping the given database connection.
func NewVendorPackageAdminRepository(dbConn db.DBTX) *VendorPackageAdminRepository {
	return &VendorPackageAdminRepository{queries: db.New(dbConn)}
}

// List returns a page of a vendor's packages (via its branches).
func (r *VendorPackageAdminRepository) List(ctx context.Context, arg db.ListVendorPackagesAdminParams) ([]db.ListVendorPackagesAdminRow, error) {
	return r.queries.ListVendorPackagesAdmin(ctx, arg)
}

// Count returns the total number of packages matching List's filters.
func (r *VendorPackageAdminRepository) Count(ctx context.Context, arg db.CountVendorPackagesAdminParams) (int64, error) {
	return r.queries.CountVendorPackagesAdmin(ctx, arg)
}

// GetByID returns a package by id incl. soft-disabled rows; nil, nil if absent.
func (r *VendorPackageAdminRepository) GetByID(ctx context.Context, id int64) (*db.GetVendorPackageAdminByIDRow, error) {
	row, err := r.queries.GetVendorPackageAdminByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// FindByBranchCode returns the id of the package with (branchID, code) incl. soft-disabled; nil, nil if none.
func (r *VendorPackageAdminRepository) FindByBranchCode(ctx context.Context, branchID int64, code string) (*int64, error) {
	id, err := r.queries.FindVendorPackageAdminByBranchCode(ctx, db.FindVendorPackageAdminByBranchCodeParams{VendorBranchID: branchID, Code: code})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

// BranchVendorID returns the vendor_id owning branchID; nil, nil if the branch doesn't exist.
func (r *VendorPackageAdminRepository) BranchVendorID(ctx context.Context, branchID int64) (*int64, error) {
	id, err := r.queries.GetVendorBranchVendorID(ctx, branchID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}
