package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// DsrLocationMapAdminRepository backs the admin "Mapping DSR" endpoints
// (cit-acm-plan FR2): list screens on the replica, submit-time lookups on the
// primary. Mutations run only inside DsrLocationMapApplier (tx-scoped).
type DsrLocationMapAdminRepository struct {
	queries     *db.Queries
	readQueries *db.Queries
}

// NewDsrLocationMapAdminRepository wraps the primary (dbConn) and replica (readConn) pools.
func NewDsrLocationMapAdminRepository(dbConn, readConn db.DBTX) *DsrLocationMapAdminRepository {
	return &DsrLocationMapAdminRepository{queries: db.New(dbConn), readQueries: db.New(readConn)}
}

// List returns a vendor's mappings filtered by status (replica).
func (r *DsrLocationMapAdminRepository) List(ctx context.Context, arg db.ListDsrLocationMapsAdminParams) ([]db.ListDsrLocationMapsAdminRow, error) {
	return r.readQueries.ListDsrLocationMapsAdmin(ctx, arg)
}

// ListUnmapped returns DSR block labels of the vendor without an active mapping (replica).
func (r *DsrLocationMapAdminRepository) ListUnmapped(ctx context.Context, vendorID int64) ([]db.ListUnmappedDsrLocationsRow, error) {
	return r.readQueries.ListUnmappedDsrLocations(ctx, vendorID)
}

// GetByID returns a mapping incl. disabled rows; nil, nil if absent.
func (r *DsrLocationMapAdminRepository) GetByID(ctx context.Context, id int64) (*db.GetDsrLocationMapAdminByIDRow, error) {
	row, err := r.queries.GetDsrLocationMapAdminByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// FindActive returns the id of another active mapping with the same label; nil, nil if none.
func (r *DsrLocationMapAdminRepository) FindActive(ctx context.Context, vendorID int64, location string, excludeID int64) (*int64, error) {
	id, err := r.queries.FindActiveDsrLocationMap(ctx, db.FindActiveDsrLocationMapParams{VendorID: vendorID, DsrLocation: location, ExcludeID: excludeID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// ActiveVaultVendorID returns the vendor owning an active vault; nil, nil if missing/disabled.
func (r *DsrLocationMapAdminRepository) ActiveVaultVendorID(ctx context.Context, vaultID int64) (*int64, error) {
	id, err := r.queries.GetActiveVaultVendorID(ctx, vaultID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}
