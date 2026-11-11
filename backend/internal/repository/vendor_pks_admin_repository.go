package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// VendorPksAdminRepository backs the admin PKS endpoints (spec vendor-pks,
// task 7.1). Every read is on the primary: the detail shows the pending change
// right after a submit (Req 4.4), and submit pre-checks need read-after-write.
// Mutations run only inside VendorPksApplier (tx-scoped).
type VendorPksAdminRepository struct {
	queries *db.Queries
}

// NewVendorPksAdminRepository wraps the primary pool.
func NewVendorPksAdminRepository(dbConn db.DBTX) *VendorPksAdminRepository {
	return &VendorPksAdminRepository{queries: db.New(dbConn)}
}

// nilIfNoRows maps pgx.ErrNoRows to (nil, nil).
func nilIfNoRows[T any](row T, err error) (*T, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// Vendor returns id, kind, deleted_at of a vendor; nil, nil if absent.
func (r *VendorPksAdminRepository) Vendor(ctx context.Context, vendorID int64) (*db.GetVendorPksVendorRow, error) {
	return nilIfNoRows(r.queries.GetVendorPksVendor(ctx, vendorID))
}

// Active returns the vendor's active PKS (replaced_at IS NULL); nil, nil if none.
func (r *VendorPksAdminRepository) Active(ctx context.Context, vendorID int64) (*db.VendorPk, error) {
	return nilIfNoRows(r.queries.GetActiveVendorPks(ctx, vendorID))
}

// GetByID returns one PKS row (the Before snapshot); nil, nil if absent.
func (r *VendorPksAdminRepository) GetByID(ctx context.Context, id int64) (*db.VendorPk, error) {
	return nilIfNoRows(r.queries.GetVendorPksByID(ctx, id))
}

// ListByVendor returns the active PKS first, then history newest-replaced first.
func (r *VendorPksAdminRepository) ListByVendor(ctx context.Context, vendorID int64) ([]db.ListVendorPksByVendorRow, error) {
	return r.queries.ListVendorPksByVendor(ctx, vendorID)
}

// Pending returns the vendor's pending vendor_pks change; nil, nil if none.
func (r *VendorPksAdminRepository) Pending(ctx context.Context, vendorID int64) (*db.GetPendingVendorPksChangeRow, error) {
	return nilIfNoRows(r.queries.GetPendingVendorPksChange(ctx, vendorID))
}

// Status evaluates vendor_pks_status() for already-loaded inputs; "" = no status (INTERNAL).
func (r *VendorPksAdminRepository) Status(ctx context.Context, arg db.GetVendorPksStatusParams) (string, error) {
	return r.queries.GetVendorPksStatus(ctx, arg)
}
