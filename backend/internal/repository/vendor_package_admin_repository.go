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
// List/Count read the replica (CLAUDE.md Sec 6); every other lookup stays on
// the primary because Submit uses it as the "before" snapshot / pre-check.
type VendorPackageAdminRepository struct {
	queries *db.Queries // primary
	dbRead  *db.Queries // replica: List/Count
}

// NewVendorPackageAdminRepository creates a VendorPackageAdminRepository. Pass the primary
// pool for both arguments when no replica is configured.
func NewVendorPackageAdminRepository(primary, replica db.DBTX) *VendorPackageAdminRepository {
	return &VendorPackageAdminRepository{queries: db.New(primary), dbRead: db.New(replica)}
}

// List returns a page of a vendor's packages (via its branches).
func (r *VendorPackageAdminRepository) List(ctx context.Context, arg db.ListVendorPackagesAdminParams) ([]db.ListVendorPackagesAdminRow, error) {
	return r.dbRead.ListVendorPackagesAdmin(ctx, arg)
}

// Count returns the total number of packages matching List's filters.
func (r *VendorPackageAdminRepository) Count(ctx context.Context, arg db.CountVendorPackagesAdminParams) (int64, error) {
	return r.dbRead.CountVendorPackagesAdmin(ctx, arg)
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

// CountActiveByBranch returns the number of active (non-disabled) packages under branchID.
// Backs the branch-disable guard (VendorBranchAdminService.Disable).
func (r *VendorPackageAdminRepository) CountActiveByBranch(ctx context.Context, branchID int64) (int64, error) {
	return r.queries.CountActiveVendorPackagesByBranch(ctx, &branchID)
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
