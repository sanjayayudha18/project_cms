package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// VendorAdminRepository is the primary-pool repository backing the admin
// vendor CRUD endpoints (Req 6-8): list/get/create/update/disable/enable,
// plus the linked-active-users count for the Req 8.5 warning.
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

// Create inserts a new vendor (Req 7.1).
func (r *VendorAdminRepository) Create(ctx context.Context, arg db.CreateVendorAdminParams) (db.CreateVendorAdminRow, error) {
	return r.queries.CreateVendorAdmin(ctx, arg)
}

// Update overwrites a vendor's editable fields (Req 7.5). Returns
// pgx.ErrNoRows if the target id does not exist or is soft-disabled (the
// query filters deleted_at IS NULL) -- the caller maps that to 404 (Req 7.7).
func (r *VendorAdminRepository) Update(ctx context.Context, arg db.UpdateVendorAdminParams) (db.UpdateVendorAdminRow, error) {
	return r.queries.UpdateVendorAdmin(ctx, arg)
}

// Disable soft-disables a vendor: is_active=false, deleted_at=now() (Req 8.1).
func (r *VendorAdminRepository) Disable(ctx context.Context, id int64) error {
	return r.queries.DisableVendor(ctx, id)
}

// Enable reverses Disable: is_active=true, deleted_at=NULL (Req 8.2).
func (r *VendorAdminRepository) Enable(ctx context.Context, id int64) error {
	return r.queries.EnableVendor(ctx, id)
}

// CountActiveUsers returns the number of active (non-disabled) users linked
// to the given vendor -- backs the Req 8.5 linked-users warning on disable.
func (r *VendorAdminRepository) CountActiveUsers(ctx context.Context, vendorID int64) (int64, error) {
	return r.queries.CountActiveUsersByVendor(ctx, &vendorID)
}
