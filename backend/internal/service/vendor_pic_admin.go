package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// ErrVendorPicNotFound is returned when a PIC is missing or belongs to another vendor.
var ErrVendorPicNotFound = errors.New("vendor pic not found")

// NoNotificationRecipientWarning is surfaced (never blocking) when a vendor
// has no active PIC flagged is_notification_recipient.
const NoNotificationRecipientWarning = "vendor belum memiliki PIC aktif penerima notifikasi"

// phoneRe: optional leading +, then 6-15 digits (after spaces/dashes/parens are stripped).
var phoneRe = regexp.MustCompile(`^\+?[0-9]{6,15}$`)

// VendorPicSubmitter is the narrow MasterDataChangeService surface
// VendorPicAdminService needs (maker-checker-native, D1).
type VendorPicSubmitter interface {
	Submit(ctx context.Context, makerID int64, req SubmitRequest, actorIP string) (db.MasterDataChangeRequest, error)
}

// VendorPicAdminRepo is the read-only repository surface VendorPicAdminService
// needs. *repository.VendorPicAdminRepository satisfies it.
type VendorPicAdminRepo interface {
	List(ctx context.Context, arg db.ListVendorPicsAdminParams) ([]db.ListVendorPicsAdminRow, error)
	Count(ctx context.Context, arg db.CountVendorPicsAdminParams) (int64, error)
	GetByID(ctx context.Context, id int64) (*db.GetVendorPicAdminByIDRow, error)
	CountNotificationRecipients(ctx context.Context, vendorID int64) (int64, error)
	BranchVendorID(ctx context.Context, branchID int64) (*int64, error)
}

// VendorPicAdminService validates PIC create/update/disable/enable and stages
// each via MasterDataChangeService.Submit; VendorPicApplier writes the row
// once approved.
type VendorPicAdminService struct {
	repo    VendorPicAdminRepo
	changes VendorPicSubmitter
}

// NewVendorPicAdminService creates a VendorPicAdminService with the given dependencies.
func NewVendorPicAdminService(repo VendorPicAdminRepo, changes VendorPicSubmitter) *VendorPicAdminService {
	return &VendorPicAdminService{repo: repo, changes: changes}
}

// VendorPicUpdatePayload is the editable field set; also the embedded tail of
// VendorPicPayload. vendor_id is immutable.
type VendorPicUpdatePayload struct {
	VendorBranchID          *int64  `json:"vendor_branch_id"`
	Name                    string  `json:"name"`
	Position                *string `json:"position"`
	Phone                   *string `json:"phone"`
	Email                   *string `json:"email"`
	IsNotificationRecipient bool    `json:"is_notification_recipient"`
}

// VendorPicPayload is the jsonb payload VendorPicApplier unmarshals for op=create.
type VendorPicPayload struct {
	VendorID int64 `json:"vendor_id"`
	VendorPicUpdatePayload
}

// VendorPic is the read DTO.
type VendorPic struct {
	ID                      int64
	VendorID                int64
	VendorBranchID          *int64
	Name                    string
	Position                *string
	Phone                   *string
	Email                   *string
	IsNotificationRecipient bool
	IsActive                bool
	DeletedAt               *time.Time
}

// List returns a page of a vendor's PICs. Read-only, no audit.
func (s *VendorPicAdminService) List(ctx context.Context, arg db.ListVendorPicsAdminParams) ([]VendorPic, error) {
	rows, err := s.repo.List(ctx, arg)
	if err != nil {
		return nil, err
	}
	out := make([]VendorPic, len(rows))
	for i, r := range rows {
		out[i] = VendorPic{ID: r.ID, VendorID: r.VendorID, VendorBranchID: r.VendorBranchID, Name: r.Name, Position: r.Position,
			Phone: r.Phone, Email: r.Email, IsNotificationRecipient: r.IsNotificationRecipient, IsActive: r.IsActive,
			DeletedAt: timestamptzToPtr(r.DeletedAt)}
	}
	return out, nil
}

// Count returns the total matching List's filters. Read-only, no audit.
func (s *VendorPicAdminService) Count(ctx context.Context, arg db.CountVendorPicsAdminParams) (int64, error) {
	return s.repo.Count(ctx, arg)
}

// Get returns vendorID's PIC by id incl. soft-deleted; nil, nil if absent or
// owned by another vendor. Read-only, no audit.
func (s *VendorPicAdminService) Get(ctx context.Context, vendorID, id int64) (*VendorPic, error) {
	r, err := s.repo.GetByID(ctx, id)
	if err != nil || r == nil || r.VendorID != vendorID {
		return nil, err
	}
	return &VendorPic{ID: r.ID, VendorID: r.VendorID, VendorBranchID: r.VendorBranchID, Name: r.Name, Position: r.Position,
		Phone: r.Phone, Email: r.Email, IsNotificationRecipient: r.IsNotificationRecipient, IsActive: r.IsActive,
		DeletedAt: timestamptzToPtr(r.DeletedAt)}, nil
}

// Warnings returns non-blocking advisories for a vendor's PIC set: currently
// only "no active notification recipient".
func (s *VendorPicAdminService) Warnings(ctx context.Context, vendorID int64) ([]string, error) {
	n, err := s.repo.CountNotificationRecipients(ctx, vendorID)
	if err != nil {
		return nil, fmt.Errorf("counting notification recipients: %w", err)
	}
	if n == 0 {
		return []string{NoNotificationRecipientWarning}, nil
	}
	return []string{}, nil
}

// Create validates and stages a new PIC under vendorID.
func (s *VendorPicAdminService) Create(ctx context.Context, makerID, vendorID int64, req VendorPicUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error) {
	if err := s.validate(ctx, vendorID, &req); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: "vendor_pic", Op: "create",
		Payload: VendorPicPayload{VendorID: vendorID, VendorPicUpdatePayload: req}}, actorIP)
}

// Update validates and stages an update; missing/soft-disabled/foreign target is ErrVendorPicNotFound.
func (s *VendorPicAdminService) Update(ctx context.Context, makerID, vendorID, id int64, req VendorPicUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("loading vendor pic: %w", err)
	}
	if before == nil || before.VendorID != vendorID || before.DeletedAt.Valid {
		return db.MasterDataChangeRequest{}, ErrVendorPicNotFound
	}
	if err := s.validate(ctx, vendorID, &req); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: "vendor_pic", Op: "update", EntityID: &id, Payload: req, Before: before}, actorIP)
}

// Disable stages a soft-disable of a PIC.
func (s *VendorPicAdminService) Disable(ctx context.Context, makerID, vendorID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	return s.toggle(ctx, makerID, vendorID, id, "disable", actorIP)
}

// Enable stages a re-enable of a PIC.
func (s *VendorPicAdminService) Enable(ctx context.Context, makerID, vendorID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	return s.toggle(ctx, makerID, vendorID, id, "enable", actorIP)
}

func (s *VendorPicAdminService) toggle(ctx context.Context, makerID, vendorID, id int64, op, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("loading vendor pic: %w", err)
	}
	if before == nil || before.VendorID != vendorID {
		return db.MasterDataChangeRequest{}, ErrVendorPicNotFound
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: "vendor_pic", Op: op, EntityID: &id, Before: before}, actorIP)
}

// validate normalizes (trims, blank optionals -> nil) and validates the
// payload: name required, email/phone format when present, and an optional
// branch must belong to vendorID.
func (s *VendorPicAdminService) validate(ctx context.Context, vendorID int64, p *VendorPicUpdatePayload) error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return &ValidationError{Field: "name", Message: "wajib diisi"}
	}
	p.Position, p.Phone, p.Email = trimOptional(p.Position), trimOptional(p.Phone), trimOptional(p.Email)

	if p.Email != nil && (!isValidEmail(*p.Email) || strings.ContainsAny(*p.Email, "<> ")) {
		return &ValidationError{Field: "email", Message: "format email tidak valid"}
	}
	if p.Phone != nil && !phoneRe.MatchString(strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(*p.Phone)) {
		return &ValidationError{Field: "phone", Message: "format telepon tidak valid (6-15 digit, boleh diawali +)"}
	}

	if p.VendorBranchID != nil {
		owner, err := s.repo.BranchVendorID(ctx, *p.VendorBranchID)
		if err != nil {
			return fmt.Errorf("checking branch ownership: %w", err)
		}
		if owner == nil {
			return &ValidationError{Field: "vendor_branch_id", Message: "cabang tidak ditemukan"}
		}
		if *owner != vendorID {
			return &ValidationError{Field: "vendor_branch_id", Message: "cabang bukan milik vendor ini"}
		}
	}
	return nil
}

// trimOptional trims s; nil/blank => nil (NULL).
func trimOptional(s *string) *string {
	if s == nil {
		return nil
	}
	return nilIfEmpty(strings.TrimSpace(*s))
}
