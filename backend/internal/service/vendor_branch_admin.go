package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Sentinel errors for VendorBranchAdminService.
var (
	ErrVendorBranchNotFound     = errors.New("vendor branch not found")
	ErrVendorBranchCodeConflict = errors.New("branch code already exists")
)

// VendorBranchSubmitter is the narrow MasterDataChangeService surface
// VendorBranchAdminService needs to route mutations into the maker-checker
// chain (plan.md Fase 2, D1) instead of writing directly.
type VendorBranchSubmitter interface {
	Submit(ctx context.Context, makerID int64, req SubmitRequest, actorIP string) (db.MasterDataChangeRequest, error)
}

// VendorBranchAdminRepo is the read-only repository surface
// VendorBranchAdminService needs. *repository.VendorBranchAdminRepository
// satisfies this automatically. Mutations are not part of this interface --
// they happen later, in VendorBranchApplier, once a submitted change is
// approved.
type VendorBranchAdminRepo interface {
	List(ctx context.Context, arg db.ListVendorBranchesAdminParams) ([]db.ListVendorBranchesAdminRow, error)
	Count(ctx context.Context, arg db.CountVendorBranchesAdminParams) (int64, error)
	GetByID(ctx context.Context, id int64) (*db.GetVendorBranchAdminByIDRow, error)
	FindByCode(ctx context.Context, code string) (*int64, error)
}

// VendorBranchAdminService owns validation and uniqueness resolution for
// vendor branch create/update/disable/enable, then stages the change via
// MasterDataChangeService.Submit. Unlike VendorAdminService (built before
// the Fase 2 maker-checker engine existed, hence T4.1's later retrofit),
// this is a new endpoint built maker-checker-native from day one (D1) --
// every mutation returns a pending db.MasterDataChangeRequest, never the
// entity itself.
type VendorBranchAdminService struct {
	repo    VendorBranchAdminRepo
	changes VendorBranchSubmitter
}

// NewVendorBranchAdminService creates a VendorBranchAdminService with the given dependencies.
func NewVendorBranchAdminService(repo VendorBranchAdminRepo, changes VendorBranchSubmitter) *VendorBranchAdminService {
	return &VendorBranchAdminService{repo: repo, changes: changes}
}

// VendorBranchPayload is both the create request shape and the jsonb
// payload VendorBranchApplier.Apply unmarshals for op=create -- one struct
// avoids a second near-identical type and a manual field-by-field copy.
type VendorBranchPayload struct {
	VendorID   int64   `json:"vendor_id"`
	BranchCode string  `json:"branch_code"`
	BranchName string  `json:"branch_name"`
	LocationID *int64  `json:"location_id"`
	Region     *string `json:"region"`
}

// VendorBranchUpdatePayload is update's payload shape. branch_code and
// vendor_id are immutable (see queries/vendor_branches_admin.sql's
// UpdateVendorBranchAdmin), so they're excluded here.
type VendorBranchUpdatePayload struct {
	BranchName string  `json:"branch_name"`
	LocationID *int64  `json:"location_id"`
	Region     *string `json:"region"`
}

// List returns a page of a vendor's branches matching the given filters.
// Read-only, never writes an audit entry.
func (s *VendorBranchAdminService) List(ctx context.Context, arg db.ListVendorBranchesAdminParams) ([]db.ListVendorBranchesAdminRow, error) {
	return s.repo.List(ctx, arg)
}

// Count returns the total number of branches matching List's filters.
// Read-only, never writes an audit entry.
func (s *VendorBranchAdminService) Count(ctx context.Context, arg db.CountVendorBranchesAdminParams) (int64, error) {
	return s.repo.Count(ctx, arg)
}

// Get returns a vendor branch by id, including soft-deleted rows. Returns
// nil, nil if not found. Read-only, never writes an audit entry.
func (s *VendorBranchAdminService) Get(ctx context.Context, id int64) (*db.GetVendorBranchAdminByIDRow, error) {
	return s.repo.GetByID(ctx, id)
}

// Create validates and stages a new vendor branch.
func (s *VendorBranchAdminService) Create(ctx context.Context, makerID int64, req VendorBranchPayload, actorIP string) (db.MasterDataChangeRequest, error) {
	req.BranchCode = strings.TrimSpace(req.BranchCode)
	req.BranchName = strings.TrimSpace(req.BranchName)
	if req.VendorID == 0 {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "vendor_id", Message: "wajib diisi"}
	}
	if req.BranchCode == "" {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "branch_code", Message: "wajib diisi"}
	}
	if req.BranchName == "" {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "branch_name", Message: "wajib diisi"}
	}

	existing, err := s.repo.FindByCode(ctx, req.BranchCode)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("checking branch code uniqueness: %w", err)
	}
	if existing != nil {
		return db.MasterDataChangeRequest{}, ErrVendorBranchCodeConflict
	}

	return s.changes.Submit(ctx, makerID, SubmitRequest{
		EntityType: "vendor_branch",
		Op:         "create",
		Payload:    req,
	}, actorIP)
}

// Update validates and stages an update to an existing vendor branch. A
// missing or soft-disabled target id is ErrVendorBranchNotFound.
func (s *VendorBranchAdminService) Update(ctx context.Context, makerID, id int64, req VendorBranchUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("loading vendor branch: %w", err)
	}
	if before == nil || before.DeletedAt.Valid {
		return db.MasterDataChangeRequest{}, ErrVendorBranchNotFound
	}

	req.BranchName = strings.TrimSpace(req.BranchName)
	if req.BranchName == "" {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "branch_name", Message: "wajib diisi"}
	}

	return s.changes.Submit(ctx, makerID, SubmitRequest{
		EntityType: "vendor_branch",
		Op:         "update",
		EntityID:   &id,
		Payload:    req,
		Before:     before,
	}, actorIP)
}

// Disable stages a soft-disable of a vendor branch.
func (s *VendorBranchAdminService) Disable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("loading vendor branch: %w", err)
	}
	if before == nil {
		return db.MasterDataChangeRequest{}, ErrVendorBranchNotFound
	}

	return s.changes.Submit(ctx, makerID, SubmitRequest{
		EntityType: "vendor_branch",
		Op:         "disable",
		EntityID:   &id,
		Before:     before,
	}, actorIP)
}

// Enable stages a re-enable of a vendor branch.
func (s *VendorBranchAdminService) Enable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("loading vendor branch: %w", err)
	}
	if before == nil {
		return db.MasterDataChangeRequest{}, ErrVendorBranchNotFound
	}

	return s.changes.Submit(ctx, makerID, SubmitRequest{
		EntityType: "vendor_branch",
		Op:         "enable",
		EntityID:   &id,
		Before:     before,
	}, actorIP)
}
