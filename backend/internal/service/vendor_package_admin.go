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
	ErrVendorPackageNotFound = errors.New("vendor package price not found")
	ErrVendorPackageOverlap  = errors.New("harga tumpang tindih dengan periode aktif lain untuk kombinasi paket/kelompok mesin/kelas/tingkat yang sama")
)

// packagePriceRe: numeric(20,2) columns -> up to 18 integer digits, at most 2
// decimals, no sign/exponent/fraction syntax. Exact string check, never
// float. Shared with vendor_package_price_admin.go's validatePriceContent.
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
	BranchVendorID(ctx context.Context, branchID int64) (*int64, error)
}

// VendorPackageAdminService validates a branch's special-price create/update/
// disable and stages each via MasterDataChangeService.Submit;
// VendorPackageApplier writes the row once approved (migration 016). No
// Enable: a row is effective-dated history like vendor_package_prices, not a
// togglable entity -- "disable" ends its validity, a new period is a new row.
// Distinct from VendorPackagePriceAdminService: this table holds a branch's
// OWN special price ("harga khusus cabang"), not the vendor-wide PT/branch/
// ATM-override tree (that stays in vendor_package_prices, unchanged).
type VendorPackageAdminService struct {
	repo    VendorPackageAdminRepo
	changes VendorPackageSubmitter
}

// NewVendorPackageAdminService creates a VendorPackageAdminService with the given dependencies.
func NewVendorPackageAdminService(repo VendorPackageAdminRepo, changes VendorPackageSubmitter) *VendorPackageAdminService {
	return &VendorPackageAdminService{repo: repo, changes: changes}
}

// VendorPackageContentPayload is the editable field set on Update; also
// embedded in the create payload. Same shape as VendorPackagePriceContentPayload
// (base_price/sla_note/effective_end_date) -- reused directly rather than
// duplicating validatePriceContent for an identical field set.
type VendorPackageContentPayload = VendorPackagePriceContentPayload

// VendorPackageCreatePayload is the create request shape and the jsonb
// payload VendorPackageApplier unmarshals for op=create. vendor_branch_id
// is required (this table is always branch-scoped, unlike vendor_packages
// pre-migration-009's internal-package NULL case, which never applied here).
// Grain fields are immutable after create -- a grain change is a new price
// period.
type VendorPackageCreatePayload struct {
	VendorBranchID     int64  `json:"vendor_branch_id"`
	PackageCode        string `json:"package_code"`
	MachineGroup       string `json:"machine_group"`
	PriceClass         string `json:"price_class"`
	TierMin            int64  `json:"tier_min"`
	TierMax            *int64 `json:"tier_max"`
	AtmID              *int64 `json:"atm_id"`
	Currency           string `json:"currency"`
	EffectiveStartDate string `json:"effective_start_date"`
	VendorPackageContentPayload
}

// VendorPackagePayload is an alias kept for the jsonb payload shape
// VendorPackageApplier unmarshals for op=create.
type VendorPackagePayload = VendorPackageCreatePayload

// VendorPackage is the read DTO: money as an exact decimal string.
type VendorPackage struct {
	ID                 int64
	VendorBranchID     *int64
	PackageCode        string
	MachineGroup       string
	PriceClass         string
	TierMin            int64
	TierMax            *int64
	BasePrice          *string
	AtmID              *int64
	SlaNote            *string
	Currency           string
	EffectiveStartDate string
	EffectiveEndDate   *string
}

func tierMax64(v *int32) *int64 {
	if v == nil {
		return nil
	}
	out := int64(*v)
	return &out
}

func newVendorPackageFromRow(
	id int64, branchID *int64, packageCode, machineGroup, priceClass string, tierMin int32, tierMax *int32,
	basePrice pgtype.Numeric, atmID *int64, slaNote *string, currency string, start, end pgtype.Date,
) (VendorPackage, error) {
	base, err := numericToDecimalStringPtr(basePrice)
	if err != nil {
		return VendorPackage{}, fmt.Errorf("base_price: %w", err)
	}
	return VendorPackage{
		ID: id, VendorBranchID: branchID, PackageCode: packageCode,
		MachineGroup: machineGroup, PriceClass: priceClass,
		TierMin: int64(tierMin), TierMax: tierMax64(tierMax),
		BasePrice: base, AtmID: atmID, SlaNote: slaNote, Currency: currency,
		EffectiveStartDate: *priceDateToStringPtr(start), EffectiveEndDate: priceDateToStringPtr(end),
	}, nil
}

func fromListPackageRow(r db.ListVendorPackagesAdminRow) (VendorPackage, error) {
	return newVendorPackageFromRow(r.ID, r.VendorBranchID, r.PackageCode, r.MachineGroup, r.PriceClass,
		r.TierMin, r.TierMax, r.BasePrice, r.AtmID, r.SlaNote, r.Currency,
		r.EffectiveStartDate, r.EffectiveEndDate)
}

func fromGetPackageRow(r *db.GetVendorPackageAdminByIDRow) (VendorPackage, error) {
	return newVendorPackageFromRow(r.ID, r.VendorBranchID, r.PackageCode, r.MachineGroup, r.PriceClass,
		r.TierMin, r.TierMax, r.BasePrice, r.AtmID, r.SlaNote, r.Currency,
		r.EffectiveStartDate, r.EffectiveEndDate)
}

// List returns a page of a vendor's branch special prices. Read-only, no audit.
func (s *VendorPackageAdminService) List(ctx context.Context, arg db.ListVendorPackagesAdminParams) ([]VendorPackage, error) {
	rows, err := s.repo.List(ctx, arg)
	if err != nil {
		return nil, err
	}
	out := make([]VendorPackage, len(rows))
	for i, r := range rows {
		if out[i], err = fromListPackageRow(r); err != nil {
			return nil, fmt.Errorf("vendor package %d: %w", r.ID, err)
		}
	}
	return out, nil
}

// Count returns the total matching List's filters. Read-only, no audit.
func (s *VendorPackageAdminService) Count(ctx context.Context, arg db.CountVendorPackagesAdminParams) (int64, error) {
	return s.repo.Count(ctx, arg)
}

// Get returns vendorID's branch price by id incl. expired; nil, nil if absent
// or owned by another vendor. Read-only, no audit.
func (s *VendorPackageAdminService) Get(ctx context.Context, vendorID, id int64) (*VendorPackage, error) {
	r, err := s.repo.GetByID(ctx, id)
	if err != nil || r == nil || r.VendorID != vendorID {
		return nil, err
	}
	p, err := fromGetPackageRow(r)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// validatePackageGrain normalizes and validates the identity fields, same
// rules as validatePriceGrain (vendor_package_prices) minus the currency-vs-
// vendor_branch_id-and-atm_id exclusivity check -- atm_id here is a further
// override WITHIN the row's own branch, not an alternative to it.
func validatePackageGrain(p *VendorPackageCreatePayload) (time.Time, error) {
	if p.VendorBranchID == 0 {
		return time.Time{}, &ValidationError{Field: "vendor_branch_id", Message: "wajib diisi"}
	}
	p.PackageCode = strings.TrimSpace(p.PackageCode)
	if p.PackageCode == "" {
		return time.Time{}, &ValidationError{Field: "package_code", Message: "wajib diisi"}
	}
	p.MachineGroup = strings.TrimSpace(p.MachineGroup)
	if p.MachineGroup != "ATM" && p.MachineGroup != "CDM_CRM" {
		return time.Time{}, &ValidationError{Field: "machine_group", Message: "harus ATM atau CDM_CRM"}
	}
	p.PriceClass = strings.TrimSpace(p.PriceClass)
	if p.PriceClass != "REGULAR" && p.PriceClass != "VIP_INDUSTRI" {
		return time.Time{}, &ValidationError{Field: "price_class", Message: "harus REGULAR atau VIP_INDUSTRI"}
	}
	if p.TierMin == 0 {
		p.TierMin = 1
	}
	if p.TierMin < 1 {
		return time.Time{}, &ValidationError{Field: "tier_min", Message: "minimal 1"}
	}
	if p.TierMax != nil && *p.TierMax < p.TierMin {
		return time.Time{}, &ValidationError{Field: "tier_max", Message: "tidak boleh kurang dari tier_min"}
	}
	p.Currency = strings.TrimSpace(strings.ToUpper(p.Currency))
	if p.Currency == "" {
		p.Currency = "IDR"
	}
	if len(p.Currency) != 3 {
		return time.Time{}, &ValidationError{Field: "currency", Message: "harus kode 3 huruf"}
	}
	p.EffectiveStartDate = strings.TrimSpace(p.EffectiveStartDate)
	if p.EffectiveStartDate == "" {
		p.EffectiveStartDate = time.Now().Format(priceDateLayout)
	}
	start, err := time.Parse(priceDateLayout, p.EffectiveStartDate)
	if err != nil {
		return time.Time{}, &ValidationError{Field: "effective_start_date", Message: "wajib format YYYY-MM-DD"}
	}
	return start, nil
}

// Create validates and stages a new branch special price under one of
// vendorID's branches.
func (s *VendorPackageAdminService) Create(ctx context.Context, makerID, vendorID int64, req VendorPackageCreatePayload, actorIP string) (db.MasterDataChangeRequest, error) {
	if _, err := validatePackageGrain(&req); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	if err := validatePriceContent(&req.VendorPackageContentPayload); err != nil {
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

	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: "vendor_package", Op: "create", Payload: req}, actorIP)
}

// Update validates and stages an edit of the content fields; a missing,
// already-ended or other-vendor target is ErrVendorPackageNotFound.
func (s *VendorPackageAdminService) Update(ctx context.Context, makerID, vendorID, id int64, req VendorPackageContentPayload, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, vendorID, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	if before.EffectiveEndDate.Valid && before.EffectiveEndDate.Time.Before(time.Now()) {
		return db.MasterDataChangeRequest{}, ErrVendorPackageNotFound
	}
	if err := validatePriceContent(&req); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	if req.EffectiveEndDate != nil {
		end, err := time.Parse(priceDateLayout, *req.EffectiveEndDate)
		if err == nil && end.Before(before.EffectiveStartDate.Time) {
			return db.MasterDataChangeRequest{}, &ValidationError{Field: "effective_end_date", Message: "tidak boleh sebelum tanggal mulai"}
		}
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: "vendor_package", Op: "update", EntityID: &id, Payload: req, Before: before}, actorIP)
}

// Disable stages ending the price period as of yesterday.
func (s *VendorPackageAdminService) Disable(ctx context.Context, makerID, vendorID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, vendorID, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: "vendor_package", Op: "disable", EntityID: &id, Before: before}, actorIP)
}

func (s *VendorPackageAdminService) load(ctx context.Context, vendorID, id int64) (*db.GetVendorPackageAdminByIDRow, error) {
	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("loading vendor package: %w", err)
	}
	if before == nil || before.VendorID != vendorID {
		return nil, ErrVendorPackageNotFound
	}
	return before, nil
}
