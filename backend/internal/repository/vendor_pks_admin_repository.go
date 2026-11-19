package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// VendorPksAdminRepository backs the admin PKS endpoints (spec vendor-pks,
// task 7.1; per type: vendor-pks-type task 4.1). Every read is on the primary:
// the detail shows the pending change right after a submit, and submit
// pre-checks need read-after-write. Mutations run only inside VendorPksApplier
// (tx-scoped).
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

// Active returns the active PKS of (vendor, type); nil, nil if none.
func (r *VendorPksAdminRepository) Active(ctx context.Context, vendorID, pksTypeID int64) (*db.GetActiveVendorPksRow, error) {
	return nilIfNoRows(r.queries.GetActiveVendorPks(ctx, db.GetActiveVendorPksParams{VendorID: vendorID, PksTypeID: pksTypeID}))
}

// GetByID returns one PKS row (the Before snapshot); nil, nil if absent.
func (r *VendorPksAdminRepository) GetByID(ctx context.Context, id int64) (*db.GetVendorPksByIDRow, error) {
	return nilIfNoRows(r.queries.GetVendorPksByID(ctx, id))
}

// ListByVendor returns, per type code, the active PKS first, then history newest-replaced first.
func (r *VendorPksAdminRepository) ListByVendor(ctx context.Context, vendorID int64) ([]db.ListVendorPksByVendorRow, error) {
	return r.queries.ListVendorPksByVendor(ctx, vendorID)
}

// Pending returns the vendor's pending vendor_pks changes (at most one per type).
func (r *VendorPksAdminRepository) Pending(ctx context.Context, vendorID int64) ([]db.GetPendingVendorPksChangesRow, error) {
	return r.queries.GetPendingVendorPksChanges(ctx, vendorID)
}

// PksType returns a PKS type by id (active or not); nil, nil if absent.
func (r *VendorPksAdminRepository) PksType(ctx context.Context, id int64) (*db.VendorPksType, error) {
	return nilIfNoRows(r.queries.GetPksType(ctx, id))
}

// PksTypeByCode returns a PKS type by code, case-insensitive; nil, nil if absent.
func (r *VendorPksAdminRepository) PksTypeByCode(ctx context.Context, code string) (*db.VendorPksType, error) {
	return nilIfNoRows(r.queries.GetPksTypeByCode(ctx, code))
}

// WodTaken reports whether a No. WOD is used by any PKS row or pending create.
func (r *VendorPksAdminRepository) WodTaken(ctx context.Context, wod string) (bool, error) {
	return r.queries.WodNumberTaken(ctx, wod)
}

// Status evaluates vendor_pks_status() for already-loaded inputs; "" = no status (INTERNAL).
func (r *VendorPksAdminRepository) Status(ctx context.Context, arg db.GetVendorPksStatusParams) (string, error) {
	return r.queries.GetVendorPksStatus(ctx, arg)
}
