package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/pkg/reports"
)

// Sentinel errors for VendorPksAdminService (spec vendor-pks, design "Error
// Handling"). Declared here, not in a shared errors file. Handler mapping:
// NotFound → 404; InternalVendor is a *ValidationError (field "vendor") → 422;
// the rest → 409.
var (
	ErrVendorPksNotFound          = errors.New("PKS tidak ditemukan")
	ErrVendorPksInternalVendor    = &ValidationError{Field: "vendor", Message: "Vendor internal tidak memiliki PKS"}
	ErrVendorPksVendorInactive    = errors.New("vendor nonaktif, aktifkan vendor dulu")
	ErrVendorPksNotActive         = errors.New("PKS ini sudah digantikan")
	ErrVendorPksAlreadyTerminated = errors.New("PKS sudah Terminated")
	ErrVendorPksNotTerminated     = errors.New("PKS tidak dalam status Terminated")
)

const (
	vendorPksEntity     = "vendor_pks"
	vendorPksDateLayout = "2006-01-02"
	vendorPksTextMax    = 500
	vendorKindInternal  = "INTERNAL"
)

// VendorPksAdminRepo is the read surface VendorPksAdminService needs (all
// primary). *repository.VendorPksAdminRepository satisfies it.
type VendorPksAdminRepo interface {
	Vendor(ctx context.Context, vendorID int64) (*db.GetVendorPksVendorRow, error)
	Active(ctx context.Context, vendorID int64) (*db.VendorPk, error)
	GetByID(ctx context.Context, id int64) (*db.VendorPk, error)
	ListByVendor(ctx context.Context, vendorID int64) ([]db.ListVendorPksByVendorRow, error)
	Pending(ctx context.Context, vendorID int64) (*db.GetPendingVendorPksChangeRow, error)
	Status(ctx context.Context, arg db.GetVendorPksStatusParams) (string, error)
}

// VendorPksAdminService validates PKS changes and stages them through
// MasterDataChangeService.Submit (which re-checks ADMIN/ADMIN_PARAM, Req 11.1).
// VendorPksApplier writes vendor_pks once a checker approves.
type VendorPksAdminService struct {
	repo          VendorPksAdminRepo
	changes       VendorAdminSubmitter
	renewalMonths int
	now           func() time.Time
}

// NewVendorPksAdminService creates the service; renewalMonths comes from
// VENDOR_PKS_RENEWAL_MONTHS (reports.ParseVendorPksRenewalMonths).
func NewVendorPksAdminService(repo VendorPksAdminRepo, changes VendorAdminSubmitter, renewalMonths int) *VendorPksAdminService {
	return &VendorPksAdminService{repo: repo, changes: changes, renewalMonths: renewalMonths, now: time.Now}
}

// SubmitVendorPksRequest is the body of POST /admin/vendors/{vendorID}/pks.
type SubmitVendorPksRequest struct {
	PksName       string `json:"pks_name"`
	EffectiveDate string `json:"effective_date"`
	ExpiryDate    string `json:"expiry_date"`
}

// VendorPksCreatePayload is the jsonb payload of op=create. ReplacesPksID is
// the active PKS at submit time (null when none); the applier marks the change
// stale when it no longer matches (Req 3.8).
type VendorPksCreatePayload struct {
	VendorID      int64  `json:"vendor_id"`
	PksName       string `json:"pks_name"`
	EffectiveDate string `json:"effective_date"`
	ExpiryDate    string `json:"expiry_date"`
	ReplacesPksID *int64 `json:"replaces_pks_id"`
}

// VendorPksReasonPayload is the jsonb payload of op=disable (terminate) and
// op=enable (batalkan terminate).
type VendorPksReasonPayload struct {
	VendorID int64  `json:"vendor_id"`
	Reason   string `json:"reason"`
}

// VendorPksRecord is one PKS row of the detail. RemainingDays is set only for
// the active PKS (Req 4.5); Replaced* only for history.
type VendorPksRecord struct {
	ID                int64      `json:"id"`
	PksName           string     `json:"pks_name"`
	EffectiveDate     string     `json:"effective_date"`
	ExpiryDate        string     `json:"expiry_date"`
	ValidityMonths    int32      `json:"validity_months"`
	RemainingDays     *int       `json:"remaining_days,omitempty"`
	CreatedAt         *time.Time `json:"created_at"`
	ReplacedAt        *time.Time `json:"replaced_at,omitempty"`
	ReplacedByPksID   *int64     `json:"replaced_by_pks_id,omitempty"`
	TerminatedAt      *time.Time `json:"terminated_at"`
	TerminationReason *string    `json:"termination_reason"`
}

// VendorPksPending is the vendor's pending PKS change (Req 4.4).
type VendorPksPending struct {
	ChangeRequestID int64           `json:"change_request_id"`
	Op              string          `json:"op"`
	Payload         json.RawMessage `json:"payload"`
	MakerID         int64           `json:"maker_id"`
	MakerName       string          `json:"maker_name"`
	CreatedAt       *time.Time      `json:"created_at"`
}

// VendorPksDetail is GET /admin/vendors/{vendorID}/pks. For an INTERNAL vendor
// PksStatus and Active are nil and History is empty.
type VendorPksDetail struct {
	VendorID      int64             `json:"vendor_id"`
	Kind          string            `json:"kind"`
	Today         string            `json:"today"`
	RenewalMonths int               `json:"renewal_months"`
	PksStatus     *string           `json:"pks_status"`
	Active        *VendorPksRecord  `json:"active"`
	History       []VendorPksRecord `json:"history"`
	Pending       *VendorPksPending `json:"pending"`
}

// Get returns the active PKS (status + remaining days), history and pending
// change of a vendor, all from the primary (Req 4.3–4.5, 6.8).
func (s *VendorPksAdminService) Get(ctx context.Context, vendorID int64) (*VendorPksDetail, error) {
	vendor, err := s.repo.Vendor(ctx, vendorID)
	if err != nil {
		return nil, fmt.Errorf("load vendor %d: %w", vendorID, err)
	}
	if vendor == nil {
		return nil, ErrVendorNotFound
	}
	today := reports.TodayWIB(s.now())
	detail := &VendorPksDetail{
		VendorID: vendorID, Kind: vendor.Kind, Today: today.Format(vendorPksDateLayout),
		RenewalMonths: s.renewalMonths, History: []VendorPksRecord{},
	}
	if vendor.Kind == vendorKindInternal {
		return detail, nil
	}

	rows, err := s.repo.ListByVendor(ctx, vendorID)
	if err != nil {
		return nil, fmt.Errorf("list vendor %d pks: %w", vendorID, err)
	}
	statusArg := db.GetVendorPksStatusParams{Kind: vendor.Kind, Today: toPgDate(today), RenewalMonths: int32(s.renewalMonths)} //nolint:gosec // G115: bounded by reports.ParseVendorPksRenewalMonths
	for _, r := range rows {
		rec := newVendorPksRecord(r)
		if r.ReplacedAt.Valid {
			detail.History = append(detail.History, rec)
			continue
		}
		days := int(r.ExpiryDate.Time.Sub(today) / (24 * time.Hour))
		rec.RemainingDays = &days
		detail.Active = &rec
		statusArg.ExpiryDate, statusArg.TerminatedAt = r.ExpiryDate, r.TerminatedAt
	}
	status, err := s.repo.Status(ctx, statusArg)
	if err != nil {
		return nil, fmt.Errorf("vendor %d pks status: %w", vendorID, err)
	}
	if status != "" {
		detail.PksStatus = &status
	}

	pending, err := s.repo.Pending(ctx, vendorID)
	if err != nil {
		return nil, fmt.Errorf("vendor %d pending pks change: %w", vendorID, err)
	}
	if pending != nil {
		detail.Pending = &VendorPksPending{
			ChangeRequestID: pending.ID, Op: pending.Op, Payload: json.RawMessage(pending.Payload),
			MakerID: pending.MakerID, MakerName: pending.MakerName, CreatedAt: timestamptzToPtr(pending.CreatedAt),
		}
	}
	return detail, nil
}

func newVendorPksRecord(r db.ListVendorPksByVendorRow) VendorPksRecord {
	rec := VendorPksRecord{
		ID: r.ID, PksName: r.PksName,
		EffectiveDate: r.EffectiveDate.Time.Format(vendorPksDateLayout), ExpiryDate: r.ExpiryDate.Time.Format(vendorPksDateLayout),
		ValidityMonths: r.ValidityMonths, CreatedAt: timestamptzToPtr(r.CreatedAt),
		TerminatedAt: timestamptzToPtr(r.TerminatedAt), TerminationReason: r.TerminationReason,
	}
	if r.ReplacedAt.Valid {
		rec.ReplacedAt, rec.ReplacedByPksID = timestamptzToPtr(r.ReplacedAt), r.ReplacedByPksID
	}
	return rec
}

// SubmitNew stages a new PKS (op=create). Order per design: vendor → fields →
// pending → Submit. Dates are not compared with earlier PKS (Req 3.4, D23); a
// terminated active PKS may be replaced (Req 3.10).
func (s *VendorPksAdminService) SubmitNew(ctx context.Context, makerID, vendorID int64, req SubmitVendorPksRequest, ip string) (db.MasterDataChangeRequest, error) {
	if err := s.checkVendor(ctx, vendorID); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	payload, err := validateVendorPksCreate(vendorID, req)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	if err := s.checkNoPending(ctx, vendorID); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	active, err := s.repo.Active(ctx, vendorID)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("load active pks of vendor %d: %w", vendorID, err)
	}
	if active != nil {
		payload.ReplacesPksID = &active.ID
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: vendorPksEntity, Op: "create", Payload: payload}, ip)
}

// SubmitTerminate stages a terminate (op=disable) of the vendor's active PKS (Req 5.1, 5.2, 5.6, 5.7).
func (s *VendorPksAdminService) SubmitTerminate(ctx context.Context, makerID, vendorID, pksID int64, reason, ip string) (db.MasterDataChangeRequest, error) {
	return s.submitReason(ctx, makerID, vendorID, pksID, reason, ip, true)
}

// SubmitUnterminate stages a batalkan terminate (op=enable) of the vendor's active, terminated PKS (Req 5.4, 5.6, 5.7).
func (s *VendorPksAdminService) SubmitUnterminate(ctx context.Context, makerID, vendorID, pksID int64, reason, ip string) (db.MasterDataChangeRequest, error) {
	return s.submitReason(ctx, makerID, vendorID, pksID, reason, ip, false)
}

func (s *VendorPksAdminService) submitReason(ctx context.Context, makerID, vendorID, pksID int64, reason, ip string, terminate bool) (db.MasterDataChangeRequest, error) {
	if err := s.checkVendor(ctx, vendorID); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	pks, err := s.repo.GetByID(ctx, pksID)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("load pks %d: %w", pksID, err)
	}
	if pks == nil || pks.VendorID != vendorID {
		return db.MasterDataChangeRequest{}, ErrVendorPksNotFound
	}
	if pks.ReplacedAt.Valid {
		return db.MasterDataChangeRequest{}, ErrVendorPksNotActive
	}
	op := "disable"
	if terminate && pks.TerminatedAt.Valid {
		return db.MasterDataChangeRequest{}, ErrVendorPksAlreadyTerminated
	}
	if !terminate {
		if !pks.TerminatedAt.Valid {
			return db.MasterDataChangeRequest{}, ErrVendorPksNotTerminated
		}
		op = "enable"
	}
	reason = strings.TrimSpace(reason)
	if err := validateVendorPksText("reason", "Alasan", reason); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	if err := s.checkNoPending(ctx, vendorID); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{
		EntityType: vendorPksEntity, Op: op, EntityID: &pksID, Before: *pks,
		Payload: VendorPksReasonPayload{VendorID: vendorID, Reason: reason},
	}, ip)
}

// checkVendor: missing → 404, INTERNAL → 422, disabled → 409 (Req 3.5).
func (s *VendorPksAdminService) checkVendor(ctx context.Context, vendorID int64) error {
	v, err := s.repo.Vendor(ctx, vendorID)
	if err != nil {
		return fmt.Errorf("load vendor %d: %w", vendorID, err)
	}
	switch {
	case v == nil:
		return ErrVendorNotFound
	case v.Kind == vendorKindInternal:
		return ErrVendorPksInternalVendor
	case v.DeletedAt.Valid:
		return ErrVendorPksVendorInactive
	}
	return nil
}

// checkNoPending: one pending vendor_pks change per vendor (Req 3.6). The
// partial unique index on payload->>'vendor_id' backs this against races.
func (s *VendorPksAdminService) checkNoPending(ctx context.Context, vendorID int64) error {
	p, err := s.repo.Pending(ctx, vendorID)
	if err != nil {
		return fmt.Errorf("vendor %d pending pks change: %w", vendorID, err)
	}
	if p != nil {
		return ErrMasterDataChangePending
	}
	return nil
}

func validateVendorPksCreate(vendorID int64, req SubmitVendorPksRequest) (VendorPksCreatePayload, error) {
	p := VendorPksCreatePayload{
		VendorID: vendorID, PksName: strings.TrimSpace(req.PksName),
		EffectiveDate: strings.TrimSpace(req.EffectiveDate), ExpiryDate: strings.TrimSpace(req.ExpiryDate),
	}
	if err := validateVendorPksText("pks_name", "Nama PKS", p.PksName); err != nil {
		return p, err
	}
	eff, err := parseVendorPksDate("effective_date", "Tanggal berlaku", p.EffectiveDate)
	if err != nil {
		return p, err
	}
	exp, err := parseVendorPksDate("expiry_date", "Tanggal berakhir", p.ExpiryDate)
	if err != nil {
		return p, err
	}
	if exp.Before(eff) {
		return p, &ValidationError{Field: "expiry_date", Message: "Tanggal berakhir tidak boleh sebelum tanggal berlaku"}
	}
	return p, nil
}

// validateVendorPksText: already trimmed, 1–500 characters.
func validateVendorPksText(field, label, v string) error {
	if v == "" {
		return &ValidationError{Field: field, Message: label + " wajib diisi"}
	}
	if utf8.RuneCountInString(v) > vendorPksTextMax {
		return &ValidationError{Field: field, Message: fmt.Sprintf("%s maksimal %d karakter", label, vendorPksTextMax)}
	}
	return nil
}

func parseVendorPksDate(field, label, v string) (time.Time, error) {
	if v == "" {
		return time.Time{}, &ValidationError{Field: field, Message: label + " wajib diisi"}
	}
	t, err := time.Parse(vendorPksDateLayout, v)
	if err != nil {
		return time.Time{}, &ValidationError{Field: field, Message: label + " harus berformat YYYY-MM-DD"}
	}
	return t, nil
}
