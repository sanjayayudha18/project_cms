package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Sentinel errors for CashCountChecklistItemAdminService.
var (
	ErrCashCountChecklistItemNotFound = errors.New("item checklist tidak ditemukan")
	ErrCashCountChecklistItemConflict = errors.New("nomor poin sudah dipakai item aktif lain pada kategori/hari yang sama")
)

const (
	cashCountChecklistEntity = "cash_count_checklist_item"

	checklistCategoryMachine    = "SELECTED_MACHINE"
	checklistCategoryNonMachine = "NON_MACHINE"
	checklistDayMachine         = "DAY1"
	checklistDayVault           = "DAY2"
	checklistTextMaxLen         = 500
)

// CashCountChecklistItemAdminRepo is the read surface the service needs.
// *repository.CashCountChecklistItemAdminRepository satisfies it.
type CashCountChecklistItemAdminRepo interface {
	List(ctx context.Context, arg db.ListCashCountChecklistItemsAdminParams) ([]db.CashCountChecklistItem, error)
	GetByID(ctx context.Context, id int64) (*db.GetCashCountChecklistItemAdminByIDRow, error)
	FindActivePoint(ctx context.Context, category, day string, pointNo int16, excludeID int64) (*int64, error)
}

// CashCountChecklistItemAdminService validates checklist-item changes (AC-5.1-06) and stages
// them via MasterDataChangeService.Submit (which enforces the CC-CCR maker role).
type CashCountChecklistItemAdminService struct {
	repo    CashCountChecklistItemAdminRepo
	changes VendorVaultSubmitter
}

// NewCashCountChecklistItemAdminService creates a CashCountChecklistItemAdminService.
func NewCashCountChecklistItemAdminService(repo CashCountChecklistItemAdminRepo, changes VendorVaultSubmitter) *CashCountChecklistItemAdminService {
	return &CashCountChecklistItemAdminService{repo: repo, changes: changes}
}

// CashCountChecklistItemPayload is the create payload (jsonb for op=create).
// Day is nil for NON_MACHINE.
type CashCountChecklistItemPayload struct {
	Category string  `json:"category"`
	Day      *string `json:"day"`
	PointNo  int16   `json:"point_no"`
	ItemText string  `json:"item_text"`
}

// CashCountChecklistItemUpdatePayload: category is immutable; day/point/text editable.
type CashCountChecklistItemUpdatePayload struct {
	Day      *string `json:"day"`
	PointNo  int16   `json:"point_no"`
	ItemText string  `json:"item_text"`
}

// List returns items filtered by category (empty = all) and status. Read-only, no audit.
func (s *CashCountChecklistItemAdminService) List(ctx context.Context, category, status string) ([]db.CashCountChecklistItem, error) {
	return s.repo.List(ctx, db.ListCashCountChecklistItemsAdminParams{Category: category, Status: status})
}

// Create validates and stages a new item.
func (s *CashCountChecklistItemAdminService) Create(ctx context.Context, makerID int64, req CashCountChecklistItemPayload, actorIP string) (db.MasterDataChangeRequest, error) {
	req.Category = strings.TrimSpace(req.Category)
	if req.Category != checklistCategoryMachine && req.Category != checklistCategoryNonMachine {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "category", Message: "harus SELECTED_MACHINE atau NON_MACHINE"}
	}
	day, text, err := validateChecklistFields(req.Category, req.Day, req.PointNo, req.ItemText)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	req.Day, req.ItemText = day, text
	if err := s.checkUnique(ctx, req.Category, req.Day, req.PointNo, 0); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: cashCountChecklistEntity, Op: "create", Payload: req}, actorIP)
}

// Update stages an edit of an active item.
func (s *CashCountChecklistItemAdminService) Update(ctx context.Context, makerID, id int64, req CashCountChecklistItemUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	if !before.IsActive {
		return db.MasterDataChangeRequest{}, ErrCashCountChecklistItemNotFound
	}
	day, text, err := validateChecklistFields(before.Category, req.Day, req.PointNo, req.ItemText)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	req.Day, req.ItemText = day, text
	if err := s.checkUnique(ctx, before.Category, req.Day, req.PointNo, id); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: cashCountChecklistEntity, Op: "update", EntityID: &id, Payload: req, Before: before}, actorIP)
}

// Disable stages a soft-disable.
func (s *CashCountChecklistItemAdminService) Disable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: cashCountChecklistEntity, Op: "disable", EntityID: &id, Before: before}, actorIP)
}

// Enable stages a re-enable; refused if the point number was reused meanwhile.
func (s *CashCountChecklistItemAdminService) Enable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	if err := s.checkUnique(ctx, before.Category, before.Day, before.PointNo, id); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: cashCountChecklistEntity, Op: "enable", EntityID: &id, Before: before}, actorIP)
}

func (s *CashCountChecklistItemAdminService) load(ctx context.Context, id int64) (*db.GetCashCountChecklistItemAdminByIDRow, error) {
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("loading checklist item: %w", err)
	}
	if row == nil {
		return nil, ErrCashCountChecklistItemNotFound
	}
	return row, nil
}

func (s *CashCountChecklistItemAdminService) checkUnique(ctx context.Context, category string, day *string, pointNo int16, excludeID int64) error {
	d := ""
	if day != nil {
		d = *day
	}
	existing, err := s.repo.FindActivePoint(ctx, category, d, pointNo, excludeID)
	if err != nil {
		return fmt.Errorf("checking checklist point uniqueness: %w", err)
	}
	if existing != nil {
		return ErrCashCountChecklistItemConflict
	}
	return nil
}

// validateChecklistFields applies the AC-5.1-06 rules and returns the normalised day + text:
// day is required (DAY1|DAY2) for SELECTED_MACHINE and must be absent for NON_MACHINE.
func validateChecklistFields(category string, day *string, pointNo int16, itemText string) (*string, string, error) {
	text := strings.TrimSpace(itemText)
	switch {
	case text == "":
		return nil, "", &ValidationError{Field: "item_text", Message: "wajib diisi"}
	case utf8.RuneCountInString(text) > checklistTextMaxLen:
		return nil, "", &ValidationError{Field: "item_text", Message: "maksimal 500 karakter"}
	case pointNo <= 0:
		return nil, "", &ValidationError{Field: "point_no", Message: "harus lebih dari 0"}
	}
	var d *string
	if day != nil {
		if v := strings.TrimSpace(*day); v != "" {
			d = &v
		}
	}
	if category == checklistCategoryNonMachine {
		if d != nil {
			return nil, "", &ValidationError{Field: "day", Message: "hanya untuk kategori SELECTED_MACHINE"}
		}
		return nil, text, nil
	}
	if d == nil {
		return nil, "", &ValidationError{Field: "day", Message: "wajib diisi (DAY1 atau DAY2) untuk SELECTED_MACHINE"}
	}
	if *d != checklistDayMachine && *d != checklistDayVault {
		return nil, "", &ValidationError{Field: "day", Message: "harus DAY1 atau DAY2"}
	}
	return d, text, nil
}
