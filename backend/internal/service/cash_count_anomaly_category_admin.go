package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Sentinel errors for CashCountAnomalyCategoryAdminService.
var (
	ErrCashCountAnomalyCategoryNotFound = errors.New("kategori ATM Anomaly tidak ditemukan")
	ErrCashCountAnomalyCategoryConflict = errors.New("kode kategori ATM Anomaly sudah dipakai")
)

const cashCountAnomalyCategoryEntity = "cash_count_anomaly_category"

var cashCountCodeRe = regexp.MustCompile(`^[A-Z0-9_]{2,20}$`)

// CashCountAnomalyCategoryAdminRepo is the read surface the service needs.
// *repository.CashCountAnomalyCategoryAdminRepository satisfies it.
type CashCountAnomalyCategoryAdminRepo interface {
	List(ctx context.Context, status string) ([]db.CashCountAnomalyCategory, error)
	GetByID(ctx context.Context, id int64) (*db.GetCashCountAnomalyCategoryAdminByIDRow, error)
	FindByCode(ctx context.Context, code string) (*int64, error)
}

// CashCountAnomalyCategoryAdminService validates "Kategori ATM Anomaly" changes
// (ScDD 5.1, AC-5.1-05) and stages them via MasterDataChangeService.Submit, which
// checks the maker role (CC-CCR) per entity type; the applier writes once approved.
type CashCountAnomalyCategoryAdminService struct {
	repo    CashCountAnomalyCategoryAdminRepo
	changes VendorVaultSubmitter
}

// NewCashCountAnomalyCategoryAdminService creates the service.
func NewCashCountAnomalyCategoryAdminService(repo CashCountAnomalyCategoryAdminRepo, changes VendorVaultSubmitter) *CashCountAnomalyCategoryAdminService {
	return &CashCountAnomalyCategoryAdminService{repo: repo, changes: changes}
}

// CashCountAnomalyCategoryPayload is the create payload.
type CashCountAnomalyCategoryPayload struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

// CashCountAnomalyCategoryUpdatePayload: code is immutable.
type CashCountAnomalyCategoryUpdatePayload struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

// List returns categories by status (active|disabled|all). Read-only, no audit.
func (s *CashCountAnomalyCategoryAdminService) List(ctx context.Context, status string) ([]db.CashCountAnomalyCategory, error) {
	return s.repo.List(ctx, status)
}

// Create validates and stages a new category.
func (s *CashCountAnomalyCategoryAdminService) Create(ctx context.Context, makerID int64, req CashCountAnomalyCategoryPayload, actorIP string) (db.MasterDataChangeRequest, error) {
	req.Code = strings.TrimSpace(req.Code)
	if !cashCountCodeRe.MatchString(req.Code) {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "code", Message: "wajib 2–20 karakter huruf besar, angka, atau garis bawah"}
	}
	name, desc, err := normalizeAnomalyText(req.Name, req.Description)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	req.Name, req.Description = name, desc
	existing, err := s.repo.FindByCode(ctx, req.Code)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("checking anomaly category code: %w", err)
	}
	if existing != nil {
		return db.MasterDataChangeRequest{}, ErrCashCountAnomalyCategoryConflict
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: cashCountAnomalyCategoryEntity, Op: "create", Payload: req}, actorIP)
}

// Update stages a name/description change of an active category.
func (s *CashCountAnomalyCategoryAdminService) Update(ctx context.Context, makerID, id int64, req CashCountAnomalyCategoryUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	if !before.IsActive {
		return db.MasterDataChangeRequest{}, ErrCashCountAnomalyCategoryNotFound
	}
	req.Name, req.Description, err = normalizeAnomalyText(req.Name, req.Description)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: cashCountAnomalyCategoryEntity, Op: "update", EntityID: &id, Payload: req, Before: before}, actorIP)
}

// Disable stages a soft-disable.
func (s *CashCountAnomalyCategoryAdminService) Disable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	return s.stageToggle(ctx, makerID, id, "disable", actorIP)
}

// Enable stages a re-enable (code stays unique over all rows, so no clash is possible).
func (s *CashCountAnomalyCategoryAdminService) Enable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	return s.stageToggle(ctx, makerID, id, "enable", actorIP)
}

func (s *CashCountAnomalyCategoryAdminService) stageToggle(ctx context.Context, makerID, id int64, op, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: cashCountAnomalyCategoryEntity, Op: op, EntityID: &id, Before: before}, actorIP)
}

func (s *CashCountAnomalyCategoryAdminService) load(ctx context.Context, id int64) (*db.GetCashCountAnomalyCategoryAdminByIDRow, error) {
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("loading anomaly category: %w", err)
	}
	if row == nil {
		return nil, ErrCashCountAnomalyCategoryNotFound
	}
	return row, nil
}

// normalizeAnomalyText trims and checks name (1–100) and optional description (≤500; blank → nil).
func normalizeAnomalyText(name string, description *string) (string, *string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 100 {
		return "", nil, &ValidationError{Field: "name", Message: "wajib diisi, maksimal 100 karakter"}
	}
	if description == nil {
		return name, nil, nil
	}
	d := strings.TrimSpace(*description)
	if d == "" {
		return name, nil, nil
	}
	if utf8.RuneCountInString(d) > 500 {
		return "", nil, &ValidationError{Field: "description", Message: "maksimal 500 karakter"}
	}
	return name, &d, nil
}
