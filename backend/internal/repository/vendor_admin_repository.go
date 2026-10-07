package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// VendorAdminRepository is the primary-pool repository backing the admin
// vendor endpoints (Req 6-8): list/get and the submit-time lookups
// (FindByCode, CountActiveUsers for the Req 8.5 warning). It has no write
// methods on purpose -- since T4.1 every vendor create/update/disable/enable
// goes through MasterDataChangeService.Submit and is applied by
// service.VendorApplier inside the approval transaction, so a direct write
// path here would be a way around maker-checker.
//
// ponytail: uses dbPool for both reads and writes; swap List/Count to the
// dbRead pool when DATABASE_REPLICA_URL wiring lands (same TODO convention
// as AuditLogRepository / UserAdminRepository).
type VendorAdminRepository struct {
	queries *db.Queries
}

// NewVendorAdminRepository creates a VendorAdminRepository wrapping the given database connection.
func NewVendorAdminRepository(dbConn db.DBTX) *VendorAdminRepository {
	return &VendorAdminRepository{queries: db.New(dbConn)}
}

// List returns a page of vendors matching the given filters (Req 6).
func (r *VendorAdminRepository) List(ctx context.Context, arg db.ListVendorsAdminParams) ([]db.ListVendorsAdminRow, error) {
	return r.queries.ListVendorsAdmin(ctx, arg)
}

// Count returns the total number of vendors matching the given filters (same
// filter fields as List, without pagination) (Req 6.6).
func (r *VendorAdminRepository) Count(ctx context.Context, arg db.CountVendorsAdminParams) (int64, error) {
	return r.queries.CountVendorsAdmin(ctx, arg)
}

// GetByID returns a vendor by id, including soft-deleted rows. Returns nil,
// nil if no matching vendor is found.
func (r *VendorAdminRepository) GetByID(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error) {
	row, err := r.queries.GetVendorAdminByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// FindByCode returns the id of the vendor with the given code, including
// soft-deleted rows (Req 7.3 uniqueness pre-check). Returns nil, nil if no
// matching vendor is found.
func (r *VendorAdminRepository) FindByCode(ctx context.Context, code string) (*int64, error) {
	id, err := r.queries.FindVendorAdminByCode(ctx, code)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

// CountActiveUsers returns the number of active (non-disabled) users linked
// to the given vendor -- backs the Req 8.5 linked-users warning on disable.
func (r *VendorAdminRepository) CountActiveUsers(ctx context.Context, vendorID int64) (int64, error) {
	return r.queries.CountActiveUsersByVendor(ctx, &vendorID)
}
