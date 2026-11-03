package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Sentinel errors for DsrLocationMapAdminService.
var (
	ErrDsrLocationMapNotFound = errors.New("mapping lokasi DSR tidak ditemukan")
	ErrDsrLocationMapConflict = errors.New("lokasi DSR ini sudah dipetakan untuk vendor ini")
)

const dsrLocationMapEntity = "dsr_location_map"

// DsrLocationMapAdminRepo is the read surface DsrLocationMapAdminService needs.
// *repository.DsrLocationMapAdminRepository satisfies it.
type DsrLocationMapAdminRepo interface {
	List(ctx context.Context, arg db.ListDsrLocationMapsAdminParams) ([]db.ListDsrLocationMapsAdminRow, error)
	ListUnmapped(ctx context.Context, vendorID int64) ([]db.ListUnmappedDsrLocationsRow, error)
	GetByID(ctx context.Context, id int64) (*db.GetDsrLocationMapAdminByIDRow, error)
	FindActive(ctx context.Context, vendorID int64, location string, excludeID int64) (*int64, error)
	ActiveVaultVendorID(ctx context.Context, vaultID int64) (*int64, error)
}

// DsrLocationMapAdminService validates "Mapping DSR" changes (cit-acm-plan FR2)
// and stages them via MasterDataChangeService.Submit (which re-checks
// ADMIN/ADMIN_PARAM); DsrLocationMapApplier writes the row once approved.
type DsrLocationMapAdminService struct {
	repo    DsrLocationMapAdminRepo
	changes VendorVaultSubmitter
}

// NewDsrLocationMapAdminService creates a DsrLocationMapAdminService.
func NewDsrLocationMapAdminService(repo DsrLocationMapAdminRepo, changes VendorVaultSubmitter) *DsrLocationMapAdminService {
	return &DsrLocationMapAdminService{repo: repo, changes: changes}
}

// DsrLocationMapPayload is the create payload (jsonb for op=create).
type DsrLocationMapPayload struct {
	VendorID      int64  `json:"vendor_id"`
	DsrLocation   string `json:"dsr_location"`
	VendorVaultID int64  `json:"vendor_vault_id"`
}

// DsrLocationMapUpdatePayload: only the target vault is editable.
type DsrLocationMapUpdatePayload struct {
	VendorVaultID int64 `json:"vendor_vault_id"`
}

// List returns a vendor's mappings. Read-only, no audit.
func (s *DsrLocationMapAdminService) List(ctx context.Context, vendorID int64, status string) ([]db.ListDsrLocationMapsAdminRow, error) {
	return s.repo.List(ctx, db.ListDsrLocationMapsAdminParams{VendorID: vendorID, Status: status})
}

// ListUnmapped returns DSR labels of the vendor with no active mapping. Read-only.
func (s *DsrLocationMapAdminService) ListUnmapped(ctx context.Context, vendorID int64) ([]db.ListUnmappedDsrLocationsRow, error) {
	return s.repo.ListUnmapped(ctx, vendorID)
}

// Create validates and stages a new mapping for vendorID.
func (s *DsrLocationMapAdminService) Create(ctx context.Context, makerID, vendorID int64, req DsrLocationMapPayload, actorIP string) (db.MasterDataChangeRequest, error) {
	req.VendorID = vendorID
	req.DsrLocation = strings.TrimSpace(req.DsrLocation)
	if req.DsrLocation == "" {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "dsr_location", Message: "wajib diisi"}
	}
	if err := s.checkVault(ctx, vendorID, req.VendorVaultID); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	if err := s.checkUnique(ctx, vendorID, req.DsrLocation, 0); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: dsrLocationMapEntity, Op: "create", Payload: req}, actorIP)
}

// Update stages a re-point of an active mapping to another vault of the same vendor.
func (s *DsrLocationMapAdminService) Update(ctx context.Context, makerID, vendorID, id int64, req DsrLocationMapUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, vendorID, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	if !before.IsActive {
		return db.MasterDataChangeRequest{}, ErrDsrLocationMapNotFound
	}
	if err := s.checkVault(ctx, vendorID, req.VendorVaultID); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: dsrLocationMapEntity, Op: "update", EntityID: &id, Payload: req, Before: before}, actorIP)
}

// Disable stages a soft-disable.
func (s *DsrLocationMapAdminService) Disable(ctx context.Context, makerID, vendorID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, vendorID, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: dsrLocationMapEntity, Op: "disable", EntityID: &id, Before: before}, actorIP)
}

// Enable stages a re-enable; refused if the label has been mapped again meanwhile.
func (s *DsrLocationMapAdminService) Enable(ctx context.Context, makerID, vendorID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, vendorID, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	if err := s.checkUnique(ctx, vendorID, before.DsrLocation, id); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: dsrLocationMapEntity, Op: "enable", EntityID: &id, Before: before}, actorIP)
}

// load returns the mapping, treating another vendor's id as not found.
func (s *DsrLocationMapAdminService) load(ctx context.Context, vendorID, id int64) (*db.GetDsrLocationMapAdminByIDRow, error) {
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("loading dsr location map: %w", err)
	}
	if row == nil || row.VendorID != vendorID {
		return nil, ErrDsrLocationMapNotFound
	}
	return row, nil
}

func (s *DsrLocationMapAdminService) checkVault(ctx context.Context, vendorID, vaultID int64) error {
	if vaultID == 0 {
		return &ValidationError{Field: "vendor_vault_id", Message: "wajib diisi"}
	}
	owner, err := s.repo.ActiveVaultVendorID(ctx, vaultID)
	if err != nil {
		return fmt.Errorf("checking vault ownership: %w", err)
	}
	if owner == nil {
		return &ValidationError{Field: "vendor_vault_id", Message: "vault tidak ditemukan atau nonaktif"}
	}
	if *owner != vendorID {
		return &ValidationError{Field: "vendor_vault_id", Message: "vault bukan milik vendor ini"}
	}
	return nil
}

func (s *DsrLocationMapAdminService) checkUnique(ctx context.Context, vendorID int64, location string, excludeID int64) error {
	existing, err := s.repo.FindActive(ctx, vendorID, location, excludeID)
	if err != nil {
		return fmt.Errorf("checking dsr location uniqueness: %w", err)
	}
	if existing != nil {
		return ErrDsrLocationMapConflict
	}
	return nil
}
