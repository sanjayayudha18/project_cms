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

// packagePriceRe: vendor_packages.price is numeric(20,2) -> up to 18 integer
// digits, at most 2 decimals, no sign/exponent/fraction syntax. Exact string
// check, never float.
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

// VendorPackageAdminService validates package create/update/disable/enable and
// stages each via MasterDataChangeService.Submit; VendorPackageApplier writes
// the row once approved. Price travels as a decimal string (never float) and
// is parsed to numeric only in the applier.
type VendorPackageAdminService struct {
	repo    VendorPackageAdminRepo
	changes VendorPackageSubmitter
}

// NewVendorPackageAdminService creates a VendorPackageAdminService with the given dependencies.
func NewVendorPackageAdminService(repo VendorPackageAdminRepo, changes VendorPackageSubmitter) *VendorPackageAdminService {
	return &VendorPackageAdminService{repo: repo, changes: changes}
}

// VendorPackageUpdatePayload is the editable field set; also the embedded
// tail of VendorPackagePayload. vendor_branch_id and code are immutable.
type VendorPackageUpdatePayload struct {
	PriorityClass string `json:"priority_class"`
	Price         string `json:"price"`
}

// VendorPackagePayload is the create request shape and the jsonb payload
// VendorPackageApplier unmarshals for op=create.
type VendorPackagePayload struct {
	VendorBranchID int64  `json:"vendor_branch_id"`
	Code           string `json:"code"`
	VendorPackageUpdatePayload
}

// VendorPackage is the read DTO: price as an exact decimal string.
type VendorPackage struct {
	ID             int64
	VendorBranchID int64
	Code           string
	PriorityClass  string
	Price          string
	IsActive       bool
	DeletedAt      *time.Time
}

func newVendorPackage(id, branchID int64, code, class string, price pgtype.Numeric, active bool, deleted pgtype.Timestamptz) (VendorPackage, error) {
	p, err := numericToDecimalStringPtr(price)
	if err != nil {
		return VendorPackage{}, err
	}
	out := VendorPackage{ID: id, VendorBranchID: branchID, Code: code, PriorityClass: class, IsActive: active, DeletedAt: timestamptzToPtr(deleted)}
	if p != nil {
		out.Price = *p
	}
	return out, nil
}

// List returns a page of a vendor's packages. Read-only, no audit.
func (s *VendorPackageAdminService) List(ctx context.Context, arg db.ListVendorPackagesAdminParams) ([]VendorPackage, error) {
	rows, err := s.repo.List(ctx, arg)
	if err != nil {
		return nil, err
	}
	out := make([]VendorPackage, len(rows))
	for i, r := range rows {
		if out[i], err = newVendorPackage(r.ID, r.VendorBranchID, r.Code, r.PriorityClass, r.Price, r.IsActive, r.DeletedAt); err != nil {
			return nil, fmt.Errorf("package %d: %w", r.ID, err)
		}
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
	p, err := newVendorPackage(r.ID, r.VendorBranchID, r.Code, r.PriorityClass, r.Price, r.IsActive, r.DeletedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Create validates and stages a new package under one of vendorID's branches.
func (s *VendorPackageAdminService) Create(ctx context.Context, makerID, vendorID int64, req VendorPackagePayload, actorIP string) (db.MasterDataChangeRequest, error) {
	req.Code = strings.TrimSpace(req.Code)
	if req.VendorBranchID == 0 {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "vendor_branch_id", Message: "wajib diisi"}
	}
	if req.Code == "" {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "code", Message: "wajib diisi"}
	}
	if err := validatePackageFields(&req.VendorPackageUpdatePayload); err != nil {
		return db.MasterDataChangeRequest{}, err
	}

	owner, err := s.repo.BranchVendorID(ctx, req.VendorBranchID)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("checking branch ownership: %w", err)
	}
	if owner == nil {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "vendor_branch_id", Message: "cabang tidak ditemukan"}
	}
	if *owner != vendorID {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "vendor_branch_id", Message: "cabang bukan milik vendor ini"}
	}

	existing, err := s.repo.FindByBranchCode(ctx, req.VendorBranchID, req.Code)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("checking package code uniqueness: %w", err)
	}
	if existing != nil {
		return db.MasterDataChangeRequest{}, ErrVendorPackageCodeConflict
	}

	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: "vendor_package", Op: "create", Payload: req}, actorIP)
}

// Update validates and stages an update; missing/soft-disabled/foreign target is ErrVendorPackageNotFound.
func (s *VendorPackageAdminService) Update(ctx context.Context, makerID, vendorID, id int64, req VendorPackageUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("loading vendor package: %w", err)
	}
	if before == nil || before.VendorID != vendorID || before.DeletedAt.Valid {
		return db.MasterDataChangeRequest{}, ErrVendorPackageNotFound
	}
	if err := validatePackageFields(&req); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: "vendor_package", Op: "update", EntityID: &id, Payload: req, Before: before}, actorIP)
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

// validatePackageFields normalizes (trims) and validates: priority_class
// required (free text -- the only value in existing data is "ALL" and no
// domain list is defined), price a non-negative decimal with at most 2
// decimals (numeric(20,2) would otherwise round silently) and at most 18
// integer digits.
func validatePackageFields(p *VendorPackageUpdatePayload) error {
	p.PriorityClass = strings.TrimSpace(p.PriorityClass)
	p.Price = strings.TrimSpace(p.Price)
	if p.PriorityClass == "" {
		return &ValidationError{Field: "priority_class", Message: "wajib diisi"}
	}
	if p.Price == "" {
		return &ValidationError{Field: "price", Message: "wajib diisi"}
	}
	if !packagePriceRe.MatchString(p.Price) {
		return &ValidationError{Field: "price", Message: "harus angka desimal tidak negatif, maksimal 18 digit bulat dan 2 desimal"}
	}
	return nil
}
