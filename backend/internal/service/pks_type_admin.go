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

// Sentinel errors for PksTypeAdminService.
var (
	ErrPksTypeNotFound = errors.New("type PKS tidak ditemukan")
	ErrPksTypeConflict = errors.New("kode type PKS sudah dipakai")
	ErrPksTypeInUse    = errors.New("type PKS masih dipakai PKS vendor")
)

const pksTypeEntity = "vendor_pks_type"

var pksTypeCodeRe = regexp.MustCompile(`^[A-Z0-9]{2,10}$`)

// PksTypeAdminRepo is the read surface the service needs.
// *repository.PksTypeRepository (task 3.3) satisfies it.
type PksTypeAdminRepo interface {
	List(ctx context.Context) ([]db.ListPksTypesRow, error)
	GetByID(ctx context.Context, id int64) (*db.VendorPksType, error)
	// FindByCode matches any status (case-insensitive); nil, nil if none.
	FindByCode(ctx context.Context, code string) (*db.VendorPksType, error)
	// CodePending reports a pending create change with this code.
	CodePending(ctx context.Context, code string) (bool, error)
	// InUse reports a vendor_pks row of this type with replaced_at IS NULL (terminated included).
	InUse(ctx context.Context, id int64) (bool, error)
}

// PksTypeAdminService validates "Type PKS" changes and stages them via
// MasterDataChangeService.Submit (which re-checks ADMIN/ADMIN_PARAM); the applier
// writes once approved.
type PksTypeAdminService struct {
	repo    PksTypeAdminRepo
	changes VendorVaultSubmitter
}

// NewPksTypeAdminService creates the service.
func NewPksTypeAdminService(repo PksTypeAdminRepo, changes VendorVaultSubmitter) *PksTypeAdminService {
	return &PksTypeAdminService{repo: repo, changes: changes}
}

// PksTypeCreatePayload is the create payload.
type PksTypeCreatePayload struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// PksTypeUpdatePayload: code is immutable, only the name changes.
type PksTypeUpdatePayload struct {
	Name string `json:"name"`
}

// List returns all types with in_use (replica). Read-only, no audit.
func (s *PksTypeAdminService) List(ctx context.Context) ([]db.ListPksTypesRow, error) {
	return s.repo.List(ctx)
}

// Get returns one type, or ErrPksTypeNotFound.
func (s *PksTypeAdminService) Get(ctx context.Context, id int64) (*db.VendorPksType, error) {
	return s.load(ctx, id)
}

// Create validates and stages a new type.
func (s *PksTypeAdminService) Create(ctx context.Context, makerID int64, req PksTypeCreatePayload, actorIP string) (db.MasterDataChangeRequest, error) {
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	if !pksTypeCodeRe.MatchString(req.Code) {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "code", Message: "wajib 2–10 karakter huruf besar atau angka"}
	}
	name, err := normalizePksTypeName(req.Name)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	req.Name = name
	existing, err := s.repo.FindByCode(ctx, req.Code)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("checking pks type code: %w", err)
	}
	if existing != nil {
		return db.MasterDataChangeRequest{}, ErrPksTypeConflict
	}
	pending, err := s.repo.CodePending(ctx, req.Code)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("checking pending pks type code: %w", err)
	}
	if pending {
		return db.MasterDataChangeRequest{}, ErrPksTypeConflict
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: pksTypeEntity, Op: "create", Payload: req}, actorIP)
}

// Update stages a name change of an active type.
func (s *PksTypeAdminService) Update(ctx context.Context, makerID, id int64, req PksTypeUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	if !before.IsActive {
		return db.MasterDataChangeRequest{}, ErrPksTypeNotFound
	}
	req.Name, err = normalizePksTypeName(req.Name)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: pksTypeEntity, Op: "update", EntityID: &id, Payload: req, Before: before}, actorIP)
}

// Disable stages a soft-disable; refused while any PKS (terminated included) uses the type.
// The applier re-checks under a row lock, this is the early 409.
func (s *PksTypeAdminService) Disable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	inUse, err := s.repo.InUse(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("checking pks type in use: %w", err)
	}
	if inUse {
		return db.MasterDataChangeRequest{}, ErrPksTypeInUse
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: pksTypeEntity, Op: "disable", EntityID: &id, Before: before}, actorIP)
}

// Enable stages a re-enable (code is unique over all rows, so no clash is possible).
func (s *PksTypeAdminService) Enable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: pksTypeEntity, Op: "enable", EntityID: &id, Before: before}, actorIP)
}

func (s *PksTypeAdminService) load(ctx context.Context, id int64) (*db.VendorPksType, error) {
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("loading pks type: %w", err)
	}
	if row == nil {
		return nil, ErrPksTypeNotFound
	}
	return row, nil
}

// normalizePksTypeName trims and checks name length 1–100.
func normalizePksTypeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 100 {
		return "", &ValidationError{Field: "name", Message: "wajib diisi, maksimal 100 karakter"}
	}
	return name, nil
}
