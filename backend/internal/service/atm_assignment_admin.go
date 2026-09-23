package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Sentinel errors for ATMAssignmentAdminService / ATMAssignmentApplier.
var (
	ErrATMAssignmentNotFound  = errors.New("atm assignment not found")
	ErrAssignmentATMNotFound  = errors.New("atm not found")
	ErrATMAssignmentOverlap   = errors.New("periode kelolaan tumpang tindih dengan assignment aktif lain untuk ATM ini")
	ErrATMAssignmentDuplicate = errors.New("assignment dengan paket dan tanggal mulai yang sama sudah ada untuk ATM ini (mungkin nonaktif)")
)

const assignmentDateLayout = "2006-01-02"

// ATMAssignmentSubmitter is the narrow MasterDataChangeService surface
// ATMAssignmentAdminService needs (maker-checker-native, D1).
type ATMAssignmentSubmitter interface {
	Submit(ctx context.Context, makerID int64, req SubmitRequest, actorIP string) (db.MasterDataChangeRequest, error)
}

// ATMAssignmentAdminRepo is the read-only repository surface
// ATMAssignmentAdminService needs. *repository.ATMAssignmentAdminRepository satisfies it.
type ATMAssignmentAdminRepo interface {
	List(ctx context.Context, arg db.ListATMAssignmentsAdminParams) ([]db.ListATMAssignmentsAdminRow, error)
	Count(ctx context.Context, arg db.CountATMAssignmentsAdminParams) (int64, error)
	GetByID(ctx context.Context, id int64) (*db.GetATMAssignmentAdminByIDRow, error)
	HasOverlap(ctx context.Context, atmID, excludeID int64, start time.Time, end *time.Time) (bool, error)
	ATMActive(ctx context.Context, atmID int64) (bool, error)
	PackageActive(ctx context.Context, packageID int64) (bool, error)
}

// ATMAssignmentAdminService validates ATM assignment (kelolaan) changes and
// stages each via MasterDataChangeService.Submit; ATMAssignmentApplier writes
// the row once approved. The submit-time overlap check is a friendly early
// answer only -- the atm_vendor_packages_no_overlap exclusion constraint
// (T1.6) is authoritative and is re-enforced at apply time, which covers two
// overlapping requests pending at once.
type ATMAssignmentAdminService struct {
	repo    ATMAssignmentAdminRepo
	changes ATMAssignmentSubmitter
}

// NewATMAssignmentAdminService creates an ATMAssignmentAdminService with the given dependencies.
func NewATMAssignmentAdminService(repo ATMAssignmentAdminRepo, changes ATMAssignmentSubmitter) *ATMAssignmentAdminService {
	return &ATMAssignmentAdminService{repo: repo, changes: changes}
}

// ATMAssignmentUpdatePayload is the editable field set; also the embedded tail
// of ATMAssignmentPayload. atm_id is immutable. Dates are YYYY-MM-DD, both
// inclusive; a nil end date means open-ended.
type ATMAssignmentUpdatePayload struct {
	VendorPackageID    int64   `json:"vendor_package_id"`
	EffectiveStartDate string  `json:"effective_start_date"`
	EffectiveEndDate   *string `json:"effective_end_date"`
}

// ATMAssignmentPayload is the jsonb payload ATMAssignmentApplier unmarshals for op=create.
type ATMAssignmentPayload struct {
	ATMID int64 `json:"atm_id"`
	ATMAssignmentUpdatePayload
}

// ATMAssignment is the read DTO.
type ATMAssignment struct {
	ID                 int64
	ATMID              int64
	VendorPackageID    int64
	PackageCode        string
	EffectiveStartDate string
	EffectiveEndDate   *string
	IsActive           bool
}

func dateToStringPtr(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format(assignmentDateLayout)
	return &s
}

func newATMAssignment(id, atmID, pkgID int64, code string, start, end pgtype.Date, active bool) ATMAssignment {
	a := ATMAssignment{ID: id, ATMID: atmID, VendorPackageID: pkgID, PackageCode: code,
		EffectiveEndDate: dateToStringPtr(end), IsActive: active}
	if s := dateToStringPtr(start); s != nil {
		a.EffectiveStartDate = *s
	}
	return a
}

// List returns a page of an ATM's assignments. Read-only, no audit.
func (s *ATMAssignmentAdminService) List(ctx context.Context, arg db.ListATMAssignmentsAdminParams) ([]ATMAssignment, error) {
	rows, err := s.repo.List(ctx, arg)
	if err != nil {
		return nil, err
	}
	out := make([]ATMAssignment, len(rows))
	for i, r := range rows {
		out[i] = newATMAssignment(r.ID, r.AtmID, r.VendorPackageID, r.PackageCode, r.EffectiveStartDate, r.EffectiveEndDate, r.IsActive)
	}
	return out, nil
}

// Count returns the total matching List's filters. Read-only, no audit.
func (s *ATMAssignmentAdminService) Count(ctx context.Context, arg db.CountATMAssignmentsAdminParams) (int64, error) {
	return s.repo.Count(ctx, arg)
}

// Get returns atmID's assignment by id incl. disabled; nil, nil if absent or
// owned by another ATM. Read-only, no audit.
func (s *ATMAssignmentAdminService) Get(ctx context.Context, atmID, id int64) (*ATMAssignment, error) {
	r, err := s.repo.GetByID(ctx, id)
	if err != nil || r == nil || r.AtmID != atmID {
		return nil, err
	}
	a := newATMAssignment(r.ID, r.AtmID, r.VendorPackageID, r.PackageCode, r.EffectiveStartDate, r.EffectiveEndDate, r.IsActive)
	return &a, nil
}

// Create validates and stages a new assignment for atmID.
func (s *ATMAssignmentAdminService) Create(ctx context.Context, makerID, atmID int64, req ATMAssignmentUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error) {
	ok, err := s.repo.ATMActive(ctx, atmID)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("checking atm: %w", err)
	}
	if !ok {
		return db.MasterDataChangeRequest{}, ErrAssignmentATMNotFound
	}
	if err := s.validate(ctx, atmID, 0, &req); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: "atm_assignment", Op: "create",
		Payload: ATMAssignmentPayload{ATMID: atmID, ATMAssignmentUpdatePayload: req}}, actorIP)
}

// Update validates and stages an edit of an active assignment; a missing,
// disabled or other-ATM target is ErrATMAssignmentNotFound.
func (s *ATMAssignmentAdminService) Update(ctx context.Context, makerID, atmID, id int64, req ATMAssignmentUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, atmID, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	if !before.IsActive {
		return db.MasterDataChangeRequest{}, ErrATMAssignmentNotFound
	}
	if err := s.validate(ctx, atmID, id, &req); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: "atm_assignment", Op: "update", EntityID: &id, Payload: req, Before: before}, actorIP)
}

// Disable stages a soft-disable (ends the kelolaan, releasing its period).
func (s *ATMAssignmentAdminService) Disable(ctx context.Context, makerID, atmID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, atmID, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: "atm_assignment", Op: "disable", EntityID: &id, Before: before}, actorIP)
}

// Enable stages a re-enable; the assignment's period must not overlap another
// active one (ErrATMAssignmentOverlap).
func (s *ATMAssignmentAdminService) Enable(ctx context.Context, makerID, atmID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.load(ctx, atmID, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	if !before.IsActive {
		start := before.EffectiveStartDate.Time
		var end *time.Time
		if before.EffectiveEndDate.Valid {
			end = &before.EffectiveEndDate.Time
		}
		overlap, err := s.repo.HasOverlap(ctx, atmID, id, start, end)
		if err != nil {
			return db.MasterDataChangeRequest{}, fmt.Errorf("checking overlap: %w", err)
		}
		if overlap {
			return db.MasterDataChangeRequest{}, ErrATMAssignmentOverlap
		}
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: "atm_assignment", Op: "enable", EntityID: &id, Before: before}, actorIP)
}

func (s *ATMAssignmentAdminService) load(ctx context.Context, atmID, id int64) (*db.GetATMAssignmentAdminByIDRow, error) {
	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("loading atm assignment: %w", err)
	}
	if before == nil || before.AtmID != atmID {
		return nil, ErrATMAssignmentNotFound
	}
	return before, nil
}

// validate normalizes (trims) and validates the payload: package required,
// active; dates YYYY-MM-DD with end >= start (an inverted range would make
// Postgres raise a raw daterange error); and no overlap with another active
// assignment of the ATM (excludeID = the row being edited, 0 for create).
func (s *ATMAssignmentAdminService) validate(ctx context.Context, atmID, excludeID int64, p *ATMAssignmentUpdatePayload) error {
	if p.VendorPackageID == 0 {
		return &ValidationError{Field: "vendor_package_id", Message: "wajib diisi"}
	}
	p.EffectiveStartDate = strings.TrimSpace(p.EffectiveStartDate)
	start, err := time.Parse(assignmentDateLayout, p.EffectiveStartDate)
	if err != nil {
		return &ValidationError{Field: "effective_start_date", Message: "wajib format YYYY-MM-DD"}
	}
	var end *time.Time
	if p.EffectiveEndDate != nil {
		if trimmed := strings.TrimSpace(*p.EffectiveEndDate); trimmed == "" {
			p.EffectiveEndDate = nil
		} else {
			e, err := time.Parse(assignmentDateLayout, trimmed)
			if err != nil {
				return &ValidationError{Field: "effective_end_date", Message: "wajib format YYYY-MM-DD"}
			}
			if e.Before(start) {
				return &ValidationError{Field: "effective_end_date", Message: "tidak boleh sebelum tanggal mulai"}
			}
			p.EffectiveEndDate, end = &trimmed, &e
		}
	}

	ok, err := s.repo.PackageActive(ctx, p.VendorPackageID)
	if err != nil {
		return fmt.Errorf("checking package: %w", err)
	}
	if !ok {
		return &ValidationError{Field: "vendor_package_id", Message: "paket tidak ditemukan atau nonaktif"}
	}

	overlap, err := s.repo.HasOverlap(ctx, atmID, excludeID, start, end)
	if err != nil {
		return fmt.Errorf("checking overlap: %w", err)
	}
	if overlap {
		return ErrATMAssignmentOverlap
	}
	return nil
}
