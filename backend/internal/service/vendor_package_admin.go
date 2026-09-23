package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Sentinel errors for VendorPackageAdminService.
var (
	ErrVendorPackageNotFound     = errors.New("vendor package not found")
	ErrVendorPackageCodeConflict = errors.New("package code already exists for this branch")
)

// packagePriceRe: numeric(20,2) columns -> up to 18 integer digits, at most 2
// decimals, no sign/exponent/fraction syntax. Exact string check, never
// float. Shared with vendor_package_prices_admin.go (vendor_packages itself
// no longer has a price column since migration 010).
var packagePriceRe = regexp.MustCompile(`^[0-9]{1,18}(\.[0-9]{1,2})?$`)

// VendorPackageSubmitter is the narrow MasterDataChangeService surface
// VendorPackageAdminService needs (maker-checker-native, D1).
type VendorPackageSubmitter interface {
	Submit(ctx context.Context, makerID int64, req SubmitRequest, actorIP string) (db.MasterDataChangeRequest, error)
}

// VendorPackageAdminRepo is the read-only repository surface
// VendorPackageAdminService needs. *repository.VendorPackageAdminRepository satisfies it.
type VendorPackageAdminRepo interface {
	List(ctx context.Context, arg db.ListVendorPackagesAdminParams) ([]db.ListVendorPackagesAdminRow, error)
	Count(ctx context.Context, arg db.CountVendorPackagesAdminParams) (int64, error)
	GetByID(ctx context.Context, id int64) (*db.GetVendorPackageAdminByIDRow, error)
	FindByBranchCode(ctx context.Context, branchID int64, code string) (*int64, error)
	BranchVendorID(ctx context.Context, branchID int64) (*int64, error)
}

// VendorPackageAdminService validates package create/disable/enable and
// stages each via MasterDataChangeService.Submit; VendorPackageApplier writes
// the row once approved. Since migration 010, vendor_branch_id and code are
// the package's only fields (both immutable after create -- price now lives
// in vendor_package_prices), so there is no Update.
type VendorPackageAdminService struct {
	repo    VendorPackageAdminRepo
	changes VendorPackageSubmitter
}

// NewVendorPackageAdminService creates a VendorPackageAdminService with the given dependencies.
func NewVendorPackageAdminService(repo VendorPackageAdminRepo, changes VendorPackageSubmitter) *VendorPackageAdminService {
	return &VendorPackageAdminService{repo: repo, changes: changes}
}

// VendorPackagePayload is the create request shape and the jsonb payload
// VendorPackageApplier unmarshals for op=create. vendor_branch_id NULL means
// an internal package (no vendor, never billed); the JSON key is optional.
type VendorPackagePayload struct {
	VendorBranchID *int64 `json:"vendor_branch_id"`
	Code           string `json:"code"`
}

// VendorPackage is the read DTO.
type VendorPackage struct {
	ID             int64
	VendorBranchID *int64
	Code           string
	IsActive       bool
	DeletedAt      *time.Time
}

func newVendorPackage(id int64, branchID *int64, code string, active bool, deleted pgtype.Timestamptz) VendorPackage {
	return VendorPackage{ID: id, VendorBranchID: branchID, Code: code, IsActive: active, DeletedAt: timestamptzToPtr(deleted)}
}

// List returns a page of a vendor's packages. Read-only, no audit.
func (s *VendorPackageAdminService) List(ctx context.Context, arg db.ListVendorPackagesAdminParams) ([]VendorPackage, error) {
	rows, err := s.repo.List(ctx, arg)
	if err != nil {
		return nil, err
	}
	out := make([]VendorPackage, len(rows))
	for i, r := range rows {
		out[i] = newVendorPackage(r.ID, r.VendorBranchID, r.Code, r.IsActive, r.DeletedAt)
	}
	return out, nil
}

// Count returns the total matching List's filters. Read-only, no audit.
func (s *VendorPackageAdminService) Count(ctx context.Context, arg db.CountVendorPackagesAdminParams) (int64, error) {
	return s.repo.Count(ctx, arg)
}

// Get returns vendorID's package by id incl. soft-disabled; nil, nil if absent
// or owned by another vendor. Read-only, no audit.
func (s *VendorPackageAdminService) Get(ctx context.Context, vendorID, id int64) (*VendorPackage, error) {
	r, err := s.repo.GetByID(ctx, id)
	if err != nil || r == nil || r.VendorID != vendorID {
		return nil, err
	}
	p := newVendorPackage(r.ID, r.VendorBranchID, r.Code, r.IsActive, r.DeletedAt)
	return &p, nil
}

// Create validates and stages a new package under one of vendorID's branches.
func (s *VendorPackageAdminService) Create(ctx context.Context, makerID, vendorID int64, req VendorPackagePayload, actorIP string) (db.MasterDataChangeRequest, error) {
	req.Code = strings.TrimSpace(req.Code)
	if req.VendorBranchID == nil || *req.VendorBranchID == 0 {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "vendor_branch_id", Message: "wajib diisi"}
	}
	if req.Code == "" {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "code", Message: "wajib diisi"}
	}

	owner, err := s.repo.BranchVendorID(ctx, *req.VendorBranchID)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("checking branch ownership: %w", err)
	}
	if owner == nil {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "vendor_branch_id", Message: "cabang tidak ditemukan"}
	}
	if *owner != vendorID {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "vendor_branch_id", Message: "cabang bukan milik vendor ini"}
	}

	existing, err := s.repo.FindByBranchCode(ctx, *req.VendorBranchID, req.Code)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("checking package code uniqueness: %w", err)
	}
	if existing != nil {
		return db.MasterDataChangeRequest{}, ErrVendorPackageCodeConflict
	}

	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: "vendor_package", Op: "create", Payload: req}, actorIP)
}

// Disable stages a soft-disable of a package.
func (s *VendorPackageAdminService) Disable(ctx context.Context, makerID, vendorID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	return s.toggle(ctx, makerID, vendorID, id, "disable", actorIP)
}

// Enable stages a re-enable of a package.
func (s *VendorPackageAdminService) Enable(ctx context.Context, makerID, vendorID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	return s.toggle(ctx, makerID, vendorID, id, "enable", actorIP)
}

func (s *VendorPackageAdminService) toggle(ctx context.Context, makerID, vendorID, id int64, op, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("loading vendor package: %w", err)
	}
	if before == nil || before.VendorID != vendorID {
		return db.MasterDataChangeRequest{}, ErrVendorPackageNotFound
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: "vendor_package", Op: op, EntityID: &id, Before: before}, actorIP)
}
