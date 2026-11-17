package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Sentinel errors for CashCountAreaAdminService.
var (
	ErrCashCountAreaNotFound     = errors.New("area cash count tidak ditemukan")
	ErrCashCountAreaConflict     = errors.New("kode area cash count sudah dipakai")
	ErrCashCountAreaHasExecutors = errors.New("area masih memiliki PIC Executor aktif")
)

const cashCountAreaEntity = "cash_count_area"

var (
	cashCountAreaCodeRe   = regexp.MustCompile(`^[A-Z0-9_]{2,20}$`)
	cashCountRegionCodeRe = regexp.MustCompile(`^[A-Z0-9]{2,10}$`)
)

// CashCountAreaAdminRepo is the read surface CashCountAreaAdminService needs.
// *repository.CashCountAreaAdminRepository satisfies it.
type CashCountAreaAdminRepo interface {
	List(ctx context.Context, status string) ([]db.ListCashCountAreasAdminRow, error)
	GetByID(ctx context.Context, id int64) (*db.GetCashCountAreaAdminByIDRow, error)
	FindByCode(ctx context.Context, code string) (*int64, error)
	RegionConflicts(ctx context.Context, regionCodes []string, excludeAreaID int64) ([]db.ListCashCountRegionConflictsRow, error)
	UnknownRegionCodes(ctx context.Context, regionCodes []string) ([]string, error)
	ActiveExecutorCount(ctx context.Context, areaID int64) (int64, error)
}

// CashCountAreaAdminService validates "Area Cash Count" changes (ScDD 5.1, AC-5.1-08) and
// stages them via MasterDataChangeService.Submit (maker role CC-CWM, checked there);
// CashCountAreaApplier writes the rows once approved.
type CashCountAreaAdminService struct {
	repo    CashCountAreaAdminRepo
	changes VendorVaultSubmitter
}

// NewCashCountAreaAdminService creates a CashCountAreaAdminService.
func NewCashCountAreaAdminService(repo CashCountAreaAdminRepo, changes VendorVaultSubmitter) *CashCountAreaAdminService {
	return &CashCountAreaAdminService{repo: repo, changes: changes}
}

// CashCountAreaPayload is the create payload (jsonb for op=create).
type CashCountAreaPayload struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	RegionCodes []string `json:"region_codes"`
}

// CashCountAreaUpdatePayload: code is immutable; name and the full region set are editable.
type CashCountAreaUpdatePayload struct {
	Name        string   `json:"name"`
	RegionCodes []string `json:"region_codes"`
}

// List returns areas. Read-only, no audit.
func (s *CashCountAreaAdminService) List(ctx context.Context, status string) ([]db.ListCashCountAreasAdminRow, error) {
	return s.repo.List(ctx, status)
}

// Create validates and stages a new area.
func (s *CashCountAreaAdminService) Create(ctx context.Context, makerID int64, req CashCountAreaPayload, actorIP string) (db.MasterDataChangeRequest, error) {
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	if !cashCountAreaCodeRe.MatchString(req.Code) {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "code", Message: "wajib 2–20 karakter huruf besar, angka, atau garis bawah"}
	}
	name, err := normalizeCashCountAreaName(req.Name)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	req.Name = name
	if req.RegionCodes, err = s.checkRegions(ctx, req.RegionCodes, 0); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	existing, err := s.repo.FindByCode(ctx, req.Code)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("checking area code uniqueness: %w", err)
	}
	if existing != nil {
		return db.MasterDataChangeRequest{}, ErrCashCountAreaConflict
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: cashCountAreaEntity, Op: "create", Payload: req}, actorIP)
}

// Update stages a rename + full region set replacement of an active area.
func (s *CashCountAreaAdminService) Update(ctx context.Context, makerID, id int64, req CashCountAreaUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	if !before.IsActive {
		return db.MasterDataChangeRequest{}, ErrCashCountAreaNotFound
	}
	name, err := normalizeCashCountAreaName(req.Name)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	req.Name = name
	if req.RegionCodes, err = s.checkRegions(ctx, req.RegionCodes, id); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: cashCountAreaEntity, Op: "update", EntityID: &id, Payload: req, Before: before}, actorIP)
}

// Disable stages a soft-disable; refused while an active EXECUTOR PIC uses the area.
// The applier also drops the area's region links so the regions become free again.
func (s *CashCountAreaAdminService) Disable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	n, err := s.repo.ActiveExecutorCount(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("counting active executors: %w", err)
	}
	if n > 0 {
		return db.MasterDataChangeRequest{}, ErrCashCountAreaHasExecutors
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: cashCountAreaEntity, Op: "disable", EntityID: &id, Before: before}, actorIP)
}

// Enable stages a re-enable (regions are re-assigned afterwards via Update).
func (s *CashCountAreaAdminService) Enable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: cashCountAreaEntity, Op: "enable", EntityID: &id, Before: before}, actorIP)
}

func (s *CashCountAreaAdminService) load(ctx context.Context, id int64) (*db.GetCashCountAreaAdminByIDRow, error) {
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("loading cash count area: %w", err)
	}
	if row == nil {
		return nil, ErrCashCountAreaNotFound
	}
	return row, nil
}

func normalizeCashCountAreaName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := len([]rune(name)); n < 1 || n > 100 {
		return "", &ValidationError{Field: "name", Message: "wajib diisi, maksimal 100 karakter"}
	}
	return name, nil
}

// checkRegions normalises (trim, upper, dedupe, sorted) and validates region codes: format,
// each used by an active vendor_branches row (L648), none owned by another active area.
func (s *CashCountAreaAdminService) checkRegions(ctx context.Context, in []string, excludeAreaID int64) ([]string, error) {
	seen := make(map[string]struct{}, len(in))
	codes := make([]string, 0, len(in))
	for _, c := range in {
		c = strings.ToUpper(strings.TrimSpace(c))
		if !cashCountRegionCodeRe.MatchString(c) {
			return nil, &ValidationError{Field: "region_codes", Message: fmt.Sprintf("kode region %q tidak valid (2–10 huruf besar/angka)", c)}
		}
		if _, dup := seen[c]; !dup {
			seen[c] = struct{}{}
			codes = append(codes, c)
		}
	}
	if len(codes) == 0 {
		return nil, &ValidationError{Field: "region_codes", Message: "minimal satu region wajib diisi"}
	}
	sort.Strings(codes)

	unknown, err := s.repo.UnknownRegionCodes(ctx, codes)
	if err != nil {
		return nil, fmt.Errorf("checking region codes: %w", err)
	}
	if len(unknown) > 0 {
		return nil, &ValidationError{Field: "region_codes", Message: "region tidak dipakai cabang vendor aktif: " + strings.Join(unknown, ", ")}
	}
	conflicts, err := s.repo.RegionConflicts(ctx, codes, excludeAreaID)
	if err != nil {
		return nil, fmt.Errorf("checking region ownership: %w", err)
	}
	if len(conflicts) > 0 {
		c := conflicts[0]
		return nil, &ValidationError{Field: "region_codes", Message: fmt.Sprintf("region %s sudah dimiliki area %s", c.RegionCode, c.AreaCode)}
	}
	return codes, nil
}
