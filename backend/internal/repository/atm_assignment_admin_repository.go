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
// List/Count read the replica (CLAUDE.md Sec 6); every other lookup stays on
// the primary because Submit uses it as the "before" snapshot / pre-check.
type ATMAssignmentAdminRepository struct {
	queries *db.Queries // primary
	dbRead  *db.Queries // replica: List/Count
}

// NewATMAssignmentAdminRepository creates a ATMAssignmentAdminRepository. Pass the primary
// pool for both arguments when no replica is configured.
func NewATMAssignmentAdminRepository(primary, replica db.DBTX) *ATMAssignmentAdminRepository {
	return &ATMAssignmentAdminRepository{queries: db.New(primary), dbRead: db.New(replica)}
}

// List returns a page of an ATM's assignments.
func (r *ATMAssignmentAdminRepository) List(ctx context.Context, arg db.ListATMAssignmentsAdminParams) ([]db.ListATMAssignmentsAdminRow, error) {
	return r.dbRead.ListATMAssignmentsAdmin(ctx, arg)
}

// Count returns the total number of assignments matching List's filters.
func (r *ATMAssignmentAdminRepository) Count(ctx context.Context, arg db.CountATMAssignmentsAdminParams) (int64, error) {
	return r.dbRead.CountATMAssignmentsAdmin(ctx, arg)
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

// PackageActive reports whether the vendor package price row exists and is
// current or open-ended (migration 016: effective-dated, no more deleted_at).
func (r *ATMAssignmentAdminRepository) PackageActive(ctx context.Context, packageID int64) (bool, error) {
	row, err := r.queries.GetVendorPackageAdminByID(ctx, packageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !row.EffectiveEndDate.Valid || !row.EffectiveEndDate.Time.Before(time.Now()), nil
}

// The next four are the vendor-wide (migration 023) read lookups; the method
// names mirror db.Queries so the same checks can also run inside the apply tx.

// GetATMPriceGroup returns the ATM's generated price_machine_group/price_class.
func (r *ATMAssignmentAdminRepository) GetATMPriceGroup(ctx context.Context, id int64) (db.GetATMPriceGroupRow, error) {
	return r.queries.GetATMPriceGroup(ctx, id)
}

// CheckAssignmentVendorBranch returns the branch id when it belongs to an active FLM vendor.
func (r *ATMAssignmentAdminRepository) CheckAssignmentVendorBranch(ctx context.Context, arg db.CheckAssignmentVendorBranchParams) (int64, error) {
	return r.queries.CheckAssignmentVendorBranch(ctx, arg)
}

// VendorTariffExistsForATM reports whether the vendor has a matching tariff for the label on arg.AsOf.
func (r *ATMAssignmentAdminRepository) VendorTariffExistsForATM(ctx context.Context, arg db.VendorTariffExistsForATMParams) (bool, error) {
	return r.queries.VendorTariffExistsForATM(ctx, arg)
}

// ListATMPackageOptions lists the vendor-wide tariff rows matching the ATM.
func (r *ATMAssignmentAdminRepository) ListATMPackageOptions(ctx context.Context, arg db.ListATMPackageOptionsParams) ([]db.ListATMPackageOptionsRow, error) {
	return r.queries.ListATMPackageOptions(ctx, arg)
}
