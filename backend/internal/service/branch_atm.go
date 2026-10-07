package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// ErrBranchNotFound is returned both when the named vendor branch doesn't
// exist and when it exists but belongs to a different vendor than the one
// in the request path (Requirement 1.4/1.5). Returning the same not-found
// for both avoids leaking another vendor's branch ids to an
// authorized-but-scoped operator -- same convention as
// VendorPackageAdminService's "other vendor -> not found" behavior.
var ErrBranchNotFound = errors.New("cabang tidak ditemukan")

// BranchATMRepo is the read-only repository surface BranchATMService needs.
// *repository.BranchATMRepository (bound to the read-replica pool) satisfies it.
type BranchATMRepo interface {
	List(ctx context.Context, arg db.ListBranchATMsParams) ([]db.ListBranchATMsRow, error)
	Count(ctx context.Context, branchID int64) (int64, error)
	BranchVendorID(ctx context.Context, branchID int64) (*int64, error)
}

// BranchATMService resolves the ATMs managed by a vendor branch for the
// read-only "ATM" sub-tab (.kiro/specs/vendor-branch-atms). Read-only, no
// audit -- display-only, no mutations.
type BranchATMService struct {
	repo BranchATMRepo
}

// NewBranchATMService creates a BranchATMService with the given repository.
func NewBranchATMService(repo BranchATMRepo) *BranchATMService {
	return &BranchATMService{repo: repo}
}

// ManagedATM is the read DTO for one ATM managed by a vendor branch.
// Pointer fields model nullable columns (location may be absent per Req
// 2.6; priority_class is nullable on atms).
type ManagedATM struct {
	ATMID                 int64
	TerminalID            string
	LocationName          *string
	LocationCityOrRegency *string
	PriorityClass         *string
	IsActive              bool
	PackageCode           string
}

// ListManagedATMsResult is a page of ManagedATMs plus the total across all pages.
type ListManagedATMsResult struct {
	ATMs  []ManagedATM
	Total int64
}

// List returns a page of ATMs managed by branchID, ordered by terminal_id
// ascending. It enforces that branchID exists and belongs to vendorID
// (Requirement 1.4/1.5) before reading; a branch with zero managed ATMs
// returns an empty result and Total 0 (Requirement 1.6), not an error.
func (s *BranchATMService) List(ctx context.Context, vendorID, branchID, pageLimit, pageOffset int64) (ListManagedATMsResult, error) {
	owner, err := s.repo.BranchVendorID(ctx, branchID)
	if err != nil {
		return ListManagedATMsResult{}, fmt.Errorf("checking branch ownership: %w", err)
	}
	if owner == nil || *owner != vendorID {
		return ListManagedATMsResult{}, ErrBranchNotFound
	}

	rows, err := s.repo.List(ctx, db.ListBranchATMsParams{
		VendorBranchID: &branchID,
		PageLimit:      pageLimit,
		PageOffset:     pageOffset,
	})
	if err != nil {
		return ListManagedATMsResult{}, fmt.Errorf("listing branch atms: %w", err)
	}
	total, err := s.repo.Count(ctx, branchID)
	if err != nil {
		return ListManagedATMsResult{}, fmt.Errorf("counting branch atms: %w", err)
	}

	atms := make([]ManagedATM, len(rows))
	for i, r := range rows {
		atms[i] = ManagedATM{
			ATMID:                 r.AtmID,
			TerminalID:            r.TerminalID,
			LocationName:          r.LocationName,
			LocationCityOrRegency: r.LocationCityOrRegency,
			PriorityClass:         r.PriorityClass,
			IsActive:              r.IsActive,
			PackageCode:           r.PackageCode,
		}
	}
	return ListManagedATMsResult{ATMs: atms, Total: total}, nil
}
