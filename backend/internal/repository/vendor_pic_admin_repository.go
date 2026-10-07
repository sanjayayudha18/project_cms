package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// VendorPicAdminRepository is the primary-pool repository backing the admin
// vendor PIC endpoints (plan.md T3.3): reads + submit-time lookups for
// VendorPicAdminService. Mutation queries run only inside VendorPicApplier
// (tx-scoped), not through this repository.
//
// ponytail: primary pool for reads too; swap List/Count to dbRead when
// DATABASE_REPLICA_URL wiring lands (same TODO as VendorBranchAdminRepository).
type VendorPicAdminRepository struct {
	queries *db.Queries
}

// NewVendorPicAdminRepository creates a VendorPicAdminRepository wrapping the given database connection.
func NewVendorPicAdminRepository(dbConn db.DBTX) *VendorPicAdminRepository {
	return &VendorPicAdminRepository{queries: db.New(dbConn)}
}

// List returns a page of a vendor's PICs.
func (r *VendorPicAdminRepository) List(ctx context.Context, arg db.ListVendorPicsAdminParams) ([]db.ListVendorPicsAdminRow, error) {
	return r.queries.ListVendorPicsAdmin(ctx, arg)
}

// Count returns the total number of PICs matching List's filters.
func (r *VendorPicAdminRepository) Count(ctx context.Context, arg db.CountVendorPicsAdminParams) (int64, error) {
	return r.queries.CountVendorPicsAdmin(ctx, arg)
}

// GetByID returns a PIC by id incl. soft-deleted rows; nil, nil if absent.
func (r *VendorPicAdminRepository) GetByID(ctx context.Context, id int64) (*db.GetVendorPicAdminByIDRow, error) {
	row, err := r.queries.GetVendorPicAdminByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// CountNotificationRecipients returns the number of active PICs of vendorID flagged as notification recipients.
func (r *VendorPicAdminRepository) CountNotificationRecipients(ctx context.Context, vendorID int64) (int64, error) {
	return r.queries.CountActiveNotificationPics(ctx, vendorID)
}

// CountActiveByBranch returns the number of active (non-disabled) branch-scoped
// PICs under branchID (vendor-wide PICs, vendor_branch_id IS NULL, are excluded).
// Backs the branch-disable guard (VendorBranchAdminService.Disable).
func (r *VendorPicAdminRepository) CountActiveByBranch(ctx context.Context, branchID int64) (int64, error) {
	return r.queries.CountActiveVendorPicsByBranch(ctx, &branchID)
}

// BranchVendorID returns the vendor_id owning branchID; nil, nil if the branch doesn't exist.
func (r *VendorPicAdminRepository) BranchVendorID(ctx context.Context, branchID int64) (*int64, error) {
	id, err := r.queries.GetVendorBranchVendorID(ctx, branchID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}
