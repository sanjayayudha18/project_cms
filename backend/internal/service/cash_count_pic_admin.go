package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Sentinel errors for CashCountPicAdminService.
var (
	ErrCashCountPicNotFound = errors.New("mapping PIC cash count tidak ditemukan")
	ErrCashCountPicConflict = errors.New("mapping PIC yang sama sudah aktif")
)

const (
	cashCountPicEntity = "cash_count_pic"
	picRoleCoordinator = "COORDINATOR"
	picRoleExecutor    = "EXECUTOR"
)

// CashCountPicAdminRepo is the read surface CashCountPicAdminService needs.
// *repository.CashCountPicAdminRepository satisfies it.
type CashCountPicAdminRepo interface {
	List(ctx context.Context, arg db.ListCashCountPicsAdminParams) ([]db.ListCashCountPicsAdminRow, error)
	GetByID(ctx context.Context, id int64) (*db.GetCashCountPicAdminByIDRow, error)
	UserEligible(ctx context.Context, userID int64, picRole string) (bool, error)
	RegionInUse(ctx context.Context, regionCode string) (bool, error)
	AreaActive(ctx context.Context, areaID int64) (bool, error)
	FindActive(ctx context.Context, arg db.FindActiveCashCountPicParams) (bool, error)
}

// CashCountPicAdminService validates "Mapping PIC" changes (AC-5.1-09/10) and stages them via
// MasterDataChangeService.Submit (which re-checks the maker role CC-CWM); CashCountPicApplier
// writes the row once approved and re-guards user/target at apply time.
type CashCountPicAdminService struct {
	repo    CashCountPicAdminRepo
	changes VendorVaultSubmitter
}

// NewCashCountPicAdminService creates a CashCountPicAdminService.
func NewCashCountPicAdminService(repo CashCountPicAdminRepo, changes VendorVaultSubmitter) *CashCountPicAdminService {
	return &CashCountPicAdminService{repo: repo, changes: changes}
}

// CashCountPicPayload is the create payload (jsonb for op=create). COORDINATOR uses
// RegionCode, EXECUTOR uses AreaID.
type CashCountPicPayload struct {
	PicRole    string  `json:"pic_role"`
	UserID     int64   `json:"user_id"`
	RegionCode *string `json:"region_code"`
	AreaID     *int64  `json:"area_id"`
}

// CashCountPicUpdatePayload: pic_role is immutable; user and target are editable.
type CashCountPicUpdatePayload struct {
	UserID     int64   `json:"user_id"`
	RegionCode *string `json:"region_code"`
	AreaID     *int64  `json:"area_id"`
}

// List returns PIC mappings. picRole "" = both. Read-only, no audit.
func (s *CashCountPicAdminService) List(ctx context.Context, picRole, status string) ([]db.ListCashCountPicsAdminRow, error) {
	return s.repo.List(ctx, db.ListCashCountPicsAdminParams{PicRole: picRole, Status: status})
}

// Create validates and stages a new mapping.
func (s *CashCountPicAdminService) Create(ctx context.Context, makerID int64, req CashCountPicPayload, actorIP string) (db.MasterDataChangeRequest, error) {
	req.PicRole = strings.TrimSpace(req.PicRole)
	if req.PicRole != picRoleCoordinator && req.PicRole != picRoleExecutor {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "pic_role", Message: "harus COORDINATOR atau EXECUTOR"}
	}
	if err := s.validate(ctx, req.PicRole, req.UserID, &req.RegionCode, &req.AreaID, 0); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: cashCountPicEntity, Op: "create", Payload: req}, actorIP)
}

// Update stages a change of user/target on an active mapping.
func (s *CashCountPicAdminService) Update(ctx context.Context, makerID, id int64, req CashCountPicUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	if !before.IsActive {
		return db.MasterDataChangeRequest{}, ErrCashCountPicNotFound
	}
	if err := s.validate(ctx, before.PicRole, req.UserID, &req.RegionCode, &req.AreaID, id); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: cashCountPicEntity, Op: "update", EntityID: &id, Payload: req, Before: before}, actorIP)
}

// Disable stages a soft-disable ("hapus" = nonaktif).
func (s *CashCountPicAdminService) Disable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: cashCountPicEntity, Op: "disable", EntityID: &id, Before: before}, actorIP)
}

// Enable stages a re-enable; refused if the same mapping became active meanwhile.
// User/target validity is re-guarded by the applier.
func (s *CashCountPicAdminService) Enable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	if err := s.checkUnique(ctx, before.PicRole, before.UserID, picDeref(before.RegionCode), picDerefInt(before.AreaID), id); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: cashCountPicEntity, Op: "enable", EntityID: &id, Before: before}, actorIP)
}

func (s *CashCountPicAdminService) load(ctx context.Context, id int64) (*db.GetCashCountPicAdminByIDRow, error) {
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("loading cash count pic: %w", err)
	}
	if row == nil {
		return nil, ErrCashCountPicNotFound
	}
	return row, nil
}

// validate normalises the target for picRole (clearing the field that does not apply, so a
// stray value never reaches the CHECK constraint) and runs the AC-5.1-09 checks.
func (s *CashCountPicAdminService) validate(ctx context.Context, picRole string, userID int64, region **string, area **int64, excludeID int64) error {
	if userID == 0 {
		return &ValidationError{Field: "user_id", Message: "wajib diisi"}
	}
	ok, err := s.repo.UserEligible(ctx, userID, picRole)
	if err != nil {
		return fmt.Errorf("checking pic user: %w", err)
	}
	if !ok {
		role := "CC-EXECUTOR"
		if picRole == picRoleCoordinator {
			role = "CC-COORDINATOR"
		}
		return &ValidationError{Field: "user_id", Message: "user harus aktif, login LDAP, dan berperan " + role}
	}

	var regionCode string
	var areaID int64
	if picRole == picRoleCoordinator {
		regionCode = strings.ToUpper(strings.TrimSpace(picDeref(*region)))
		if regionCode == "" {
			return &ValidationError{Field: "region_code", Message: "wajib diisi untuk COORDINATOR"}
		}
		inUse, err := s.repo.RegionInUse(ctx, regionCode)
		if err != nil {
			return fmt.Errorf("checking region code: %w", err)
		}
		if !inUse {
			return &ValidationError{Field: "region_code", Message: "region_code tidak dipakai cabang vendor aktif"}
		}
		*region, *area = &regionCode, nil
	} else {
		areaID = picDerefInt(*area)
		if areaID == 0 {
			return &ValidationError{Field: "area_id", Message: "wajib diisi untuk EXECUTOR"}
		}
		active, err := s.repo.AreaActive(ctx, areaID)
		if err != nil {
			return fmt.Errorf("checking cash count area: %w", err)
		}
		if !active {
			return &ValidationError{Field: "area_id", Message: "area tidak ditemukan atau nonaktif"}
		}
		*region, *area = nil, &areaID
	}
	return s.checkUnique(ctx, picRole, userID, regionCode, areaID, excludeID)
}

func (s *CashCountPicAdminService) checkUnique(ctx context.Context, picRole string, userID int64, regionCode string, areaID, excludeID int64) error {
	dup, err := s.repo.FindActive(ctx, db.FindActiveCashCountPicParams{
		PicRole: picRole, UserID: userID, RegionCode: regionCode, AreaID: areaID, ExcludeID: excludeID,
	})
	if err != nil {
		return fmt.Errorf("checking cash count pic uniqueness: %w", err)
	}
	if dup {
		return ErrCashCountPicConflict
	}
	return nil
}

func picDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func picDerefInt(n *int64) int64 {
	if n == nil {
		return 0
	}
	return *n
}
