package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// VendorPackagePriceAdminRepository is the primary-pool repository backing
// the admin vendor package price endpoints (vendor-pricing plan.md P20):
// reads + submit-time lookups for VendorPackagePriceAdminService. Mutation
// queries run only inside VendorPackagePriceApplier (tx-scoped), not through
// this repository.
//
// List/Count read the replica (CLAUDE.md Sec 6); every other lookup stays on
// the primary because Submit uses it as the "before" snapshot / pre-check.
type VendorPackagePriceAdminRepository struct {
	queries *db.Queries // primary
	dbRead  *db.Queries // replica: List/Count
}

// NewVendorPackagePriceAdminRepository creates a VendorPackagePriceAdminRepository. Pass the primary
// pool for both arguments when no replica is configured.
func NewVendorPackagePriceAdminRepository(primary, replica db.DBTX) *VendorPackagePriceAdminRepository {
	return &VendorPackagePriceAdminRepository{queries: db.New(primary), dbRead: db.New(replica)}
}

// List returns a page of a vendor's package prices.
func (r *VendorPackagePriceAdminRepository) List(ctx context.Context, arg db.ListVendorPackagePricesAdminParams) ([]db.ListVendorPackagePricesAdminRow, error) {
	return r.dbRead.ListVendorPackagePricesAdmin(ctx, arg)
}

// Count returns the total number of package prices matching List's filters.
func (r *VendorPackagePriceAdminRepository) Count(ctx context.Context, arg db.CountVendorPackagePricesAdminParams) (int64, error) {
	return r.dbRead.CountVendorPackagePricesAdmin(ctx, arg)
}

// GetByID returns a package price by id incl. expired rows; nil, nil if absent.
func (r *VendorPackagePriceAdminRepository) GetByID(ctx context.Context, id int64) (*db.GetVendorPackagePriceAdminByIDRow, error) {
	row, err := r.queries.GetVendorPackagePriceAdminByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// VendorKind returns vendorID's kind ("FLM_VENDOR"|"INTERNAL") and is_active;
// nil, nil if the vendor doesn't exist. Backs the submit-time guard that
// INTERNAL vendors (ROH) never get a price row.
func (r *VendorPackagePriceAdminRepository) VendorKind(ctx context.Context, vendorID int64) (*db.GetVendorKindForPackagePriceRow, error) {
	row, err := r.queries.GetVendorKindForPackagePrice(ctx, vendorID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}
