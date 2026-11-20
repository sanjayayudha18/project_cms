package repository

import (
	"context"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// PksTypeRepository backs the admin "Type PKS" endpoints: the list screen on
// the replica, submit-time lookups on the primary. Writes run only in PksTypeApplier.
type PksTypeRepository struct {
	queries     *db.Queries
	readQueries *db.Queries
}

// NewPksTypeRepository wraps the primary (dbConn) and replica (readConn) pools.
func NewPksTypeRepository(dbConn, readConn db.DBTX) *PksTypeRepository {
	return &PksTypeRepository{queries: db.New(dbConn), readQueries: db.New(readConn)}
}

// List returns all types with in_use (replica).
func (r *PksTypeRepository) List(ctx context.Context) ([]db.ListPksTypesRow, error) {
	return r.readQueries.ListPksTypes(ctx)
}

// GetByID returns a type incl. disabled; nil, nil if absent.
func (r *PksTypeRepository) GetByID(ctx context.Context, id int64) (*db.VendorPksType, error) {
	return nilIfNoRows(r.queries.GetPksType(ctx, id))
}

// FindByCode returns a type of any status; nil, nil if none.
func (r *PksTypeRepository) FindByCode(ctx context.Context, code string) (*db.VendorPksType, error) {
	return nilIfNoRows(r.queries.GetPksTypeByCode(ctx, code))
}

// CodePending reports a pending create with this code.
func (r *PksTypeRepository) CodePending(ctx context.Context, code string) (bool, error) {
	return r.queries.PksTypeCodePending(ctx, code)
}

// InUse reports a non-replaced vendor_pks row of this type (terminated included).
func (r *PksTypeRepository) InUse(ctx context.Context, id int64) (bool, error) {
	return r.queries.PksTypeInUse(ctx, id)
}
