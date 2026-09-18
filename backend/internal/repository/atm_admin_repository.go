package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// ATMAdminRepository is the primary-pool repository backing the admin ATM
// CRUD endpoints (.kiro/specs/admin-atm-management Req 2-6): list/get/
// create/update/disable/enable, plus the location reference lookups the
// service layer needs before writing.
//
// Money columns (capacity_amount/low_threshold_amount/critical_threshold_
// amount) and timestamps pass through as pgtype.Numeric/pgtype.Timestamptz
// unconverted, same as VendorAdminRepository/UserAdminRepository -- the
// decimal-string <-> pgtype.Numeric conversion already exists as a
// service-package convention (numericToDecimalString(Ptr) in
// atm_portal_cashpos.go/atm_portal_profile.go), so ATMAdminService reuses
// those directly instead of duplicating the algorithm here.
//
// ponytail: uses dbPool for both reads and writes; swap List/Get/
// ListLocations to the dbRead pool when DATABASE_REPLICA_URL wiring lands
// (same TODO convention as AuditLogRepository / VendorAdminRepository).
type ATMAdminRepository struct {
	queries *db.Queries
}

// NewATMAdminRepository creates an ATMAdminRepository wrapping the given database connection.
func NewATMAdminRepository(dbConn db.DBTX) *ATMAdminRepository {
	return &ATMAdminRepository{queries: db.New(dbConn)}
}

// List returns a page of ATMs matching the given filters.
func (r *ATMAdminRepository) List(ctx context.Context, arg db.ListATMsAdminParams) ([]db.ListATMsAdminRow, error) {
	return r.queries.ListATMsAdmin(ctx, arg)
}

// Count returns the total number of ATMs matching the given filters (same
// filter fields as List, without pagination).
func (r *ATMAdminRepository) Count(ctx context.Context, arg db.CountATMsAdminParams) (int64, error) {
	return r.queries.CountATMsAdmin(ctx, arg)
}

// GetByID returns an ATM by id, including soft-deleted rows. Returns nil,
// nil if no matching ATM is found.
func (r *ATMAdminRepository) GetByID(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error) {
	row, err := r.queries.GetATMAdminByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// FindByTerminalID returns the id of the ATM with the given terminal_id,
// including soft-deleted rows (uniqueness pre-check). Returns nil, nil if
// no matching ATM is found.
func (r *ATMAdminRepository) FindByTerminalID(ctx context.Context, terminalID string) (*int64, error) {
	id, err := r.queries.FindATMByTerminalID(ctx, terminalID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

// Create inserts a new ATM.
func (r *ATMAdminRepository) Create(ctx context.Context, arg db.CreateATMAdminParams) (db.CreateATMAdminRow, error) {
	return r.queries.CreateATMAdmin(ctx, arg)
}

// Update overwrites an ATM's editable fields (terminal_id excluded --
// immutable). Returns pgx.ErrNoRows if the target id does not exist or is
// soft-disabled (the query filters deleted_at IS NULL) -- the caller maps
// that to 404.
func (r *ATMAdminRepository) Update(ctx context.Context, arg db.UpdateATMAdminParams) (db.UpdateATMAdminRow, error) {
	return r.queries.UpdateATMAdmin(ctx, arg)
}

// Disable soft-disables an ATM: is_active=false, deleted_at=now().
func (r *ATMAdminRepository) Disable(ctx context.Context, id int64) error {
	return r.queries.DisableATM(ctx, id)
}

// Enable reverses Disable: is_active=true, deleted_at=NULL.
func (r *ATMAdminRepository) Enable(ctx context.Context, id int64) error {
	return r.queries.EnableATM(ctx, id)
}

// ListLocations returns every location for the ATM form's Location select,
// ordered by name.
func (r *ATMAdminRepository) ListLocations(ctx context.Context) ([]db.ListLocationsForSelectRow, error) {
	return r.queries.ListLocationsForSelect(ctx)
}

// LocationExists reports whether the given location_id references an
// existing location row.
func (r *ATMAdminRepository) LocationExists(ctx context.Context, locationID int64) (bool, error) {
	return r.queries.LocationExists(ctx, locationID)
}
