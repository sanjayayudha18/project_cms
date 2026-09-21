package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// ATMAssignmentAdminRepository is the primary-pool repository backing the
// admin ATM assignment endpoints (plan.md T3.5): reads + submit-time lookups
// for ATMAssignmentAdminService. Mutation queries run only inside
// ATMAssignmentApplier (tx-scoped), not through this repository.
//
// ponytail: primary pool for reads too; swap List/Count to dbRead when
// DATABASE_REPLICA_URL wiring lands (same TODO as VendorBranchAdminRepository).
type ATMAssignmentAdminRepository struct {
	queries *db.Queries
}

// NewATMAssignmentAdminRepository creates an ATMAssignmentAdminRepository wrapping the given database connection.
func NewATMAssignmentAdminRepository(dbConn db.DBTX) *ATMAssignmentAdminRepository {
	return &ATMAssignmentAdminRepository{queries: db.New(dbConn)}
}

// List returns a page of an ATM's assignments.
func (r *ATMAssignmentAdminRepository) List(ctx context.Context, arg db.ListATMAssignmentsAdminParams) ([]db.ListATMAssignmentsAdminRow, error) {
	return r.queries.ListATMAssignmentsAdmin(ctx, arg)
}

// Count returns the total number of assignments matching List's filters.
func (r *ATMAssignmentAdminRepository) Count(ctx context.Context, arg db.CountATMAssignmentsAdminParams) (int64, error) {
	return r.queries.CountATMAssignmentsAdmin(ctx, arg)
}

// GetByID returns an assignment by id incl. disabled rows; nil, nil if absent.
func (r *ATMAssignmentAdminRepository) GetByID(ctx context.Context, id int64) (*db.GetATMAssignmentAdminByIDRow, error) {
	row, err := r.queries.GetATMAssignmentAdminByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// HasOverlap reports whether another ACTIVE assignment of atmID overlaps the
// inclusive period [start, end] (nil end = open-ended). excludeID skips one
// row (the assignment being edited; 0 for create).
func (r *ATMAssignmentAdminRepository) HasOverlap(ctx context.Context, atmID, excludeID int64, start time.Time, end *time.Time) (bool, error) {
	arg := db.FindOverlappingATMAssignmentParams{AtmID: atmID, ExcludeID: excludeID, StartDate: pgtype.Date{Time: start, Valid: true}}
	if end != nil {
		arg.EndDate = pgtype.Date{Time: *end, Valid: true}
	}
	_, err := r.queries.FindOverlappingATMAssignment(ctx, arg)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// ATMActive reports whether the ATM exists and is not soft-deleted.
func (r *ATMAssignmentAdminRepository) ATMActive(ctx context.Context, atmID int64) (bool, error) {
	_, err := r.queries.ATMActiveForAssignment(ctx, atmID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// PackageActive reports whether the vendor package exists and is not soft-disabled.
func (r *ATMAssignmentAdminRepository) PackageActive(ctx context.Context, packageID int64) (bool, error) {
	row, err := r.queries.GetVendorPackageAdminByID(ctx, packageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !row.DeletedAt.Valid, nil
}
