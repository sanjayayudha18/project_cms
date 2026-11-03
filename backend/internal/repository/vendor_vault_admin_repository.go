package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// VendorVaultAdminRepository is the primary-pool repository backing the
// admin vendor vault endpoints (plan.md T3.2): reads + submit-time lookups
// for VendorVaultAdminService. Mutation queries run only inside
// VendorVaultApplier (tx-scoped), not through this repository.
//
// List/Count read the replica (CLAUDE.md Sec 6); every other lookup stays on
// the primary because Submit uses it as the "before" snapshot / pre-check.
type VendorVaultAdminRepository struct {
	queries *db.Queries // primary
	dbRead  *db.Queries // replica: List/Count
}

// NewVendorVaultAdminRepository creates a VendorVaultAdminRepository. Pass the primary
// pool for both arguments when no replica is configured.
func NewVendorVaultAdminRepository(primary, replica db.DBTX) *VendorVaultAdminRepository {
	return &VendorVaultAdminRepository{queries: db.New(primary), dbRead: db.New(replica)}
}

// List returns a page of a vendor's vaults (via its branches).
func (r *VendorVaultAdminRepository) List(ctx context.Context, arg db.ListVendorVaultsAdminParams) ([]db.ListVendorVaultsAdminRow, error) {
	return r.dbRead.ListVendorVaultsAdmin(ctx, arg)
}

// Count returns the total number of vaults matching List's filters.
func (r *VendorVaultAdminRepository) Count(ctx context.Context, arg db.CountVendorVaultsAdminParams) (int64, error) {
	return r.dbRead.CountVendorVaultsAdmin(ctx, arg)
}

// GetByID returns a vault by id incl. soft-deleted rows; nil, nil if absent.
func (r *VendorVaultAdminRepository) GetByID(ctx context.Context, id int64) (*db.GetVendorVaultAdminByIDRow, error) {
	row, err := r.queries.GetVendorVaultAdminByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// FindByCode returns the id of the vault with vault_code (incl. soft-deleted); nil, nil if none.
func (r *VendorVaultAdminRepository) FindByCode(ctx context.Context, code string) (*int64, error) {
	id, err := r.queries.FindVendorVaultAdminByCode(ctx, code)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

// CountActiveByBranch returns the number of active (non-disabled) vaults under branchID.
// Backs the branch-disable guard (VendorBranchAdminService.Disable).
func (r *VendorVaultAdminRepository) CountActiveByBranch(ctx context.Context, branchID int64) (int64, error) {
	return r.queries.CountActiveVendorVaultsByBranch(ctx, branchID)
}

// BranchVendorID returns the vendor_id owning branchID; nil, nil if the branch doesn't exist.
func (r *VendorVaultAdminRepository) BranchVendorID(ctx context.Context, branchID int64) (*int64, error) {
	id, err := r.queries.GetVendorBranchVendorID(ctx, branchID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}
