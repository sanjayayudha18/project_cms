package service

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Sentinel errors for VendorVaultAdminService.
var (
	ErrVendorVaultNotFound     = errors.New("vendor vault not found")
	ErrVendorVaultCodeConflict = errors.New("vault code already exists")
)

var currencyCodeRe = regexp.MustCompile(`^[A-Z]{3}$`)

// VendorVaultSubmitter is the narrow MasterDataChangeService surface
// VendorVaultAdminService needs (maker-checker-native, D1).
type VendorVaultSubmitter interface {
	Submit(ctx context.Context, makerID int64, req SubmitRequest, actorIP string) (db.MasterDataChangeRequest, error)
}

// VendorVaultAdminRepo is the read-only repository surface
// VendorVaultAdminService needs. *repository.VendorVaultAdminRepository satisfies it.
type VendorVaultAdminRepo interface {
	List(ctx context.Context, arg db.ListVendorVaultsAdminParams) ([]db.ListVendorVaultsAdminRow, error)
	Count(ctx context.Context, arg db.CountVendorVaultsAdminParams) (int64, error)
	GetByID(ctx context.Context, id int64) (*db.GetVendorVaultAdminByIDRow, error)
	FindByCode(ctx context.Context, code string) (*int64, error)
	BranchVendorID(ctx context.Context, branchID int64) (*int64, error)
}

// VendorVaultAdminService validates vault create/update/disable/enable and
// stages each via MasterDataChangeService.Submit; VendorVaultApplier writes
// the row once approved. Amounts/coordinates travel as decimal strings
// (never float) and are parsed to numeric only in the applier.
type VendorVaultAdminService struct {
	repo    VendorVaultAdminRepo
	changes VendorVaultSubmitter
}

// NewVendorVaultAdminService creates a VendorVaultAdminService with the given dependencies.
func NewVendorVaultAdminService(repo VendorVaultAdminRepo, changes VendorVaultSubmitter) *VendorVaultAdminService {
	return &VendorVaultAdminService{repo: repo, changes: changes}
}

// VendorVaultUpdatePayload is the editable field set; also the embedded
// tail of VendorVaultPayload. vault_code and vendor_branch_id are immutable.
type VendorVaultUpdatePayload struct {
	Category          string  `json:"category"`
	CurrencyCode      string  `json:"currency_code"`
	MinCapacityAmount *string `json:"min_capacity_amount"`
	MaxCapacityAmount *string `json:"max_capacity_amount"`
	Latitude          *string `json:"latitude"`
	Longitude         *string `json:"longitude"`
	OperatingHours    *string `json:"operating_hours"`
	LocationID        *int64  `json:"location_id"`
}

// VendorVaultPayload is the create request shape and the jsonb payload
// VendorVaultApplier unmarshals for op=create.
type VendorVaultPayload struct {
	VendorBranchID int64  `json:"vendor_branch_id"`
	VaultCode      string `json:"vault_code"`
	VendorVaultUpdatePayload
}

// VendorVault is the read DTO: amounts/coordinates as exact decimal strings.
type VendorVault struct {
	ID                int64
	VendorBranchID    int64
	VaultCode         string
	Category          string
	CurrencyCode      string
	MinCapacityAmount *string
	MaxCapacityAmount *string
	Latitude          *string
	Longitude         *string
	OperatingHours    *string
	LocationID        *int64
	IsActive          bool
	DeletedAt         *time.Time
}

func newVendorVault(id, branchID int64, code, category, currency string, minC, maxC, lat, lon pgtype.Numeric, hours *string, locID *int64, active bool, deleted pgtype.Timestamptz) (VendorVault, error) {
	v := VendorVault{ID: id, VendorBranchID: branchID, VaultCode: code, Category: category, CurrencyCode: currency,
		OperatingHours: hours, LocationID: locID, IsActive: active, DeletedAt: timestamptzToPtr(deleted)}
	var err error
	for _, f := range []struct {
		dst **string
		src pgtype.Numeric
	}{{&v.MinCapacityAmount, minC}, {&v.MaxCapacityAmount, maxC}, {&v.Latitude, lat}, {&v.Longitude, lon}} {
		if *f.dst, err = numericToDecimalStringPtr(f.src); err != nil {
			return VendorVault{}, err
		}
	}
	return v, nil
}

// List returns a page of a vendor's vaults. Read-only, no audit.
func (s *VendorVaultAdminService) List(ctx context.Context, arg db.ListVendorVaultsAdminParams) ([]VendorVault, error) {
	rows, err := s.repo.List(ctx, arg)
	if err != nil {
		return nil, err
	}
	out := make([]VendorVault, len(rows))
	for i, r := range rows {
		if out[i], err = newVendorVault(r.ID, r.VendorBranchID, r.VaultCode, r.Category, r.CurrencyCode, r.MinCapacityAmount, r.MaxCapacityAmount, r.Latitude, r.Longitude, r.OperatingHours, r.LocationID, r.IsActive, r.DeletedAt); err != nil {
			return nil, fmt.Errorf("vault %d: %w", r.ID, err)
		}
	}
	return out, nil
}

// Count returns the total matching List's filters. Read-only, no audit.
func (s *VendorVaultAdminService) Count(ctx context.Context, arg db.CountVendorVaultsAdminParams) (int64, error) {
	return s.repo.Count(ctx, arg)
}

// Get returns a vault by id incl. soft-deleted; nil, nil if absent. Read-only, no audit.
func (s *VendorVaultAdminService) Get(ctx context.Context, id int64) (*VendorVault, error) {
	r, err := s.repo.GetByID(ctx, id)
	if err != nil || r == nil {
		return nil, err
	}
	v, err := newVendorVault(r.ID, r.VendorBranchID, r.VaultCode, r.Category, r.CurrencyCode, r.MinCapacityAmount, r.MaxCapacityAmount, r.Latitude, r.Longitude, r.OperatingHours, r.LocationID, r.IsActive, r.DeletedAt)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// Create validates and stages a new vault under vendorID's branch.
func (s *VendorVaultAdminService) Create(ctx context.Context, makerID, vendorID int64, req VendorVaultPayload, actorIP string) (db.MasterDataChangeRequest, error) {
	req.VaultCode = strings.TrimSpace(req.VaultCode)
	if req.VendorBranchID == 0 {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "vendor_branch_id", Message: "wajib diisi"}
	}
	if req.VaultCode == "" {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "vault_code", Message: "wajib diisi"}
	}
	if err := validateVaultFields(&req.VendorVaultUpdatePayload); err != nil {
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

	existing, err := s.repo.FindByCode(ctx, req.VaultCode)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("checking vault code uniqueness: %w", err)
	}
	if existing != nil {
		return db.MasterDataChangeRequest{}, ErrVendorVaultCodeConflict
	}

	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: "vendor_vault", Op: "create", Payload: req}, actorIP)
}

// Update validates and stages an update; missing/soft-disabled target is ErrVendorVaultNotFound.
func (s *VendorVaultAdminService) Update(ctx context.Context, makerID, id int64, req VendorVaultUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("loading vendor vault: %w", err)
	}
	if before == nil || before.DeletedAt.Valid {
		return db.MasterDataChangeRequest{}, ErrVendorVaultNotFound
	}
	if err := validateVaultFields(&req); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: "vendor_vault", Op: "update", EntityID: &id, Payload: req, Before: before}, actorIP)
}

// Disable stages a soft-disable of a vault.
func (s *VendorVaultAdminService) Disable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	return s.toggle(ctx, makerID, id, "disable", actorIP)
}

// Enable stages a re-enable of a vault.
func (s *VendorVaultAdminService) Enable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	return s.toggle(ctx, makerID, id, "enable", actorIP)
}

func (s *VendorVaultAdminService) toggle(ctx context.Context, makerID, id int64, op, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("loading vendor vault: %w", err)
	}
	if before == nil {
		return db.MasterDataChangeRequest{}, ErrVendorVaultNotFound
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: "vendor_vault", Op: op, EntityID: &id, Before: before}, actorIP)
}

// validateVaultFields normalizes (trims) and validates the editable fields:
// category ATM|CASH, currency = 3 uppercase letters, capacities >= 0 with
// min <= max, coordinates within range. Decimals are checked via big.Rat --
// never float.
func validateVaultFields(p *VendorVaultUpdatePayload) error {
	p.Category = strings.TrimSpace(p.Category)
	p.CurrencyCode = strings.ToUpper(strings.TrimSpace(p.CurrencyCode))
	if p.Category != "ATM" && p.Category != "CASH" {
		return &ValidationError{Field: "category", Message: "harus ATM atau CASH"}
	}
	if !currencyCodeRe.MatchString(p.CurrencyCode) {
		return &ValidationError{Field: "currency_code", Message: "wajib 3 huruf (mis. IDR)"}
	}

	minC, err := parseDecimalField("min_capacity_amount", p.MinCapacityAmount)
	if err != nil {
		return err
	}
	maxC, err := parseDecimalField("max_capacity_amount", p.MaxCapacityAmount)
	if err != nil {
		return err
	}
	if minC != nil && minC.Sign() < 0 {
		return &ValidationError{Field: "min_capacity_amount", Message: "tidak boleh negatif"}
	}
	if maxC != nil && maxC.Sign() < 0 {
		return &ValidationError{Field: "max_capacity_amount", Message: "tidak boleh negatif"}
	}
	if minC != nil && maxC != nil && minC.Cmp(maxC) > 0 {
		return &ValidationError{Field: "min_capacity_amount", Message: "tidak boleh lebih besar dari kapasitas maksimum"}
	}

	if err := validateCoordinate("latitude", p.Latitude, 90); err != nil {
		return err
	}
	return validateCoordinate("longitude", p.Longitude, 180)
}

func validateCoordinate(field string, v *string, limit int64) error {
	r, err := parseDecimalField(field, v)
	if err != nil || r == nil {
		return err
	}
	lim := new(big.Rat).SetInt64(limit)
	if r.Cmp(lim) > 0 || r.Cmp(new(big.Rat).Neg(lim)) < 0 {
		return &ValidationError{Field: field, Message: fmt.Sprintf("harus antara -%d dan %d", limit, limit)}
	}
	return nil
}

// parseDecimalField parses an optional decimal string; nil/blank => nil (NULL).
func parseDecimalField(field string, v *string) (*big.Rat, error) {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil, nil
	}
	s := strings.TrimSpace(*v)
	r, ok := new(big.Rat).SetString(s)
	// big.Rat also accepts fractions ("1/2"); numeric columns don't.
	if !ok || strings.Contains(s, "/") {
		return nil, &ValidationError{Field: field, Message: "bukan angka desimal yang valid"}
	}
	return r, nil
}
