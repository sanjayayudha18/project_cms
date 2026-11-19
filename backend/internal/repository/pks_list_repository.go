package repository

import (
	"context"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// PksListRepository backs the Daftar PKS screen and the reminder edit (spec
// vendor-pks-list task 3.3). List/Count/Pending read the replica
// (CLAUDE.md Sec 6); the reminder lock + update take the caller's tx on the
// primary, so there is no write path outside a service-owned transaction.
type PksListRepository struct {
	dbRead *db.Queries // replica
}

// NewPksListRepository wraps the replica pool (pass the primary when none).
func NewPksListRepository(replica db.DBTX) *PksListRepository {
	return &PksListRepository{dbRead: db.New(replica)}
}

// List returns one page of PKS rows (replica).
func (r *PksListRepository) List(ctx context.Context, arg db.ListPksAdminParams) ([]db.ListPksAdminRow, error) {
	return r.dbRead.ListPksAdmin(ctx, arg)
}

// Count returns the total for the same filters as List (replica).
func (r *PksListRepository) Count(ctx context.Context, arg db.CountPksAdminParams) (int64, error) {
	return r.dbRead.CountPksAdmin(ctx, arg)
}

// Pending returns pending vendor_pks create requests matching the filters (replica).
func (r *PksListRepository) Pending(ctx context.Context, arg db.ListPendingPksChangesParams) ([]db.ListPendingPksChangesRow, error) {
	return r.dbRead.ListPendingPksChanges(ctx, arg)
}

// LockForReminder locks one PKS row inside tx (primary); pgx.ErrNoRows = not found.
func (r *PksListRepository) LockForReminder(ctx context.Context, tx db.DBTX, id int64) (db.GetVendorPksForUpdateRow, error) {
	return db.New(tx).GetVendorPksForUpdate(ctx, id)
}

// UpdateRenewalDays sets renewal_days inside tx (primary); pgx.ErrNoRows = not found.
func (r *PksListRepository) UpdateRenewalDays(ctx context.Context, tx db.DBTX, arg db.UpdateVendorPksRenewalDaysParams) (db.UpdateVendorPksRenewalDaysRow, error) {
	return db.New(tx).UpdateVendorPksRenewalDays(ctx, arg)
}
