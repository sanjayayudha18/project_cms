package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/pkg/reports"
)

// Sentinel errors for VendorPksAdminService (spec vendor-pks, design "Error
// Handling"; per type: vendor-pks-type). Declared here, not in a shared errors
// file. Handler mapping: NotFound → 404; InternalVendor is a *ValidationError
// (field "vendor") → 422; WodTaken → 409 on field "wod_number"; the rest → 409.
var (
	ErrVendorPksNotFound          = errors.New("PKS tidak ditemukan")
	ErrVendorPksInternalVendor    = &ValidationError{Field: "vendor", Message: "Vendor internal tidak memiliki PKS"}
	ErrVendorPksVendorInactive    = errors.New("vendor nonaktif, aktifkan vendor dulu")
	ErrVendorPksNotActive         = errors.New("PKS ini sudah digantikan")
	ErrVendorPksAlreadyTerminated = errors.New("PKS sudah Terminated")
	ErrVendorPksNotTerminated     = errors.New("PKS tidak dalam status Terminated")
	// ErrVendorPksWodTaken: the No. WOD is held by a PKS (history included) or
	// a pending create (vendor-pks-type Req 2.4). The applier (task 4.2) returns
	// it too on a vendor_pks_wod_uq violation.
	ErrVendorPksWodTaken = errors.New("No. WOD sudah dipakai") //nolint:staticcheck // ST1005: Indonesian proper noun abbreviation "No. WOD"
)

const (
	vendorPksEntity     = "vendor_pks"
	vendorPksDateLayout = "2006-01-02"
	vendorPksTextMax    = 500
	vendorPksWodMax     = 100
	vendorKindInternal  = "INTERNAL"
)

// VendorPksAdminRepo is the read surface VendorPksAdminService needs (all
// primary). *repository.VendorPksAdminRepository satisfies it.
type VendorPksAdminRepo interface {
	Vendor(ctx context.Context, vendorID int64) (*db.GetVendorPksVendorRow, error)
	Active(ctx context.Context, vendorID, pksTypeID int64) (*db.GetActiveVendorPksRow, error)
	GetByID(ctx context.Context, id int64) (*db.GetVendorPksByIDRow, error)
	Pending(ctx context.Context, vendorID int64) ([]db.GetPendingVendorPksChangesRow, error)
	PksTypeByCode(ctx context.Context, code string) (*db.VendorPksType, error)
	WodTaken(ctx context.Context, wod string) (bool, error)
}

// VendorPksAdminService validates PKS changes and stages them through
// MasterDataChangeService.Submit (which re-checks ADMIN/ADMIN_PARAM).
// VendorPksApplier writes vendor_pks once a checker approves.
type VendorPksAdminService struct {
	repo    VendorPksAdminRepo
	changes VendorAdminSubmitter
	pool    VendorPicRolePool // primary; UpdateReminder tx
}

// NewVendorPksAdminService creates the service. pool is the primary pool.
func NewVendorPksAdminService(repo VendorPksAdminRepo, changes VendorAdminSubmitter, pool VendorPicRolePool) *VendorPksAdminService {
	return &VendorPksAdminService{repo: repo, changes: changes, pool: pool}
}

// UpdateReminder sets renewal_days of any PKS row (replaced and terminated
// included) immediately — documented GR#3 deviation (spec vendor-pks-list D9,
// same pattern as VendorPicRoleService.SetRoles): no maker-checker, the
// update and its audit_logs row share one primary tx, so a failed audit
// rolls the update back. Role ∉ ADMIN/ADMIN_PARAM → ErrMasterDataForbidden;
// missing row → ErrVendorPksNotFound; invalid days → 422 on "renewal_days".
// An unchanged value returns the row without update or audit.
func (s *VendorPksAdminService) UpdateReminder(ctx context.Context, actorID int64, role string, pksID int64, days int32, ip string) (db.UpdateVendorPksRenewalDaysRow, error) {
	var zero db.UpdateVendorPksRenewalDaysRow
	if !masterDataMakerRoles[strings.ToUpper(strings.TrimSpace(role))] {
		return zero, ErrMasterDataForbidden
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return zero, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)
	row, err := q.GetVendorPksForUpdate(ctx, pksID)
	if errors.Is(err, pgx.ErrNoRows) {
		return zero, ErrVendorPksNotFound
	}
	if err != nil {
		return zero, fmt.Errorf("load pks %d for update: %w", pksID, err)
	}
	if err := validateRenewalDays(days, row.EffectiveDate.Time, row.ExpiryDate.Time); err != nil {
		return zero, err
	}
	if row.RenewalDays == days {
		return db.UpdateVendorPksRenewalDaysRow(row), nil
	}

	updated, err := q.UpdateVendorPksRenewalDays(ctx, db.UpdateVendorPksRenewalDaysParams{RenewalDays: days, ID: pksID})
	if err != nil {
		return zero, fmt.Errorf("update pks %d renewal days: %w", pksID, err)
	}
	if err := audit.NewWriter(tx).Write(ctx, audit.Entry{
		ActorID:    actorID,
		Action:     "vendor_pks_reminder_updated",
		EntityType: vendorPksEntity,
		EntityID:   pksID,
		Before:     map[string]any{"renewal_days": row.RenewalDays},
		After:      map[string]any{"renewal_days": days},
		IP:         ip,
	}); err != nil {
		return zero, fmt.Errorf("write audit log: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return zero, fmt.Errorf("commit pks %d reminder: %w", pksID, err)
	}
	return updated, nil
}

// SubmitVendorPksRequest is the body of POST /admin/vendors/{vendorID}/pks.
// PksType is a type code (e.g. "FLM"), resolved to an active type.
type SubmitVendorPksRequest struct {
	PksType       string `json:"pks_type"`
	WodNumber     string `json:"wod_number"`
	PksName       string `json:"pks_name"`
	EffectiveDate string `json:"effective_date"`
	ExpiryDate    string `json:"expiry_date"`
	RenewalDays   int32  `json:"renewal_days"`
}

// VendorPksCreatePayload is the jsonb payload of op=create (design "Payload
// change"). ReplacesPksID is the active PKS of the same (vendor, type) at
// submit time (null when none); the applier marks the change stale when it no
// longer matches.
type VendorPksCreatePayload struct {
	VendorID      int64  `json:"vendor_id"`
	PksTypeID     int64  `json:"pks_type_id"`
	PksName       string `json:"pks_name"`
	EffectiveDate string `json:"effective_date"`
	ExpiryDate    string `json:"expiry_date"`
	WodNumber     string `json:"wod_number"`
	RenewalDays   int32  `json:"renewal_days"` // absent in pre-044 payloads → applier uses reports.RenewalDaysDefault
	ReplacesPksID *int64 `json:"replaced_pks_id"`
}

// VendorPksReasonPayload is the jsonb payload of op=disable (terminate) and
// op=enable (batalkan terminate). PksTypeID is taken from the target PKS row:
// the pending index is per (vendor_id, pks_type_id).
type VendorPksReasonPayload struct {
	VendorID  int64  `json:"vendor_id"`
	PksTypeID int64  `json:"pks_type_id"`
	Reason    string `json:"reason"`
}

// pendingKey is the part of a pending vendor_pks payload the per-type checks need.
type pendingKey struct {
	PksTypeID int64 `json:"pks_type_id"`
}

// SubmitNew stages a new PKS (op=create) for one type. Order: vendor → fields
// (type, WOD, name, dates; 422) → WOD taken (409) → pending for the same
// (vendor, type) (409) → Submit. A vendor may hold PKS for any subset of types
// (Req 2.10); dates are not compared with earlier PKS; a terminated active PKS
// may be replaced.
func (s *VendorPksAdminService) SubmitNew(ctx context.Context, makerID, vendorID int64, req SubmitVendorPksRequest, ip string) (db.MasterDataChangeRequest, error) {
	if err := s.checkVendor(ctx, vendorID); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	typeID, err := s.resolveActiveType(ctx, req.PksType)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	payload, err := validateVendorPksCreate(vendorID, typeID, req)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	if err := s.checkWodFree(ctx, payload.WodNumber); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	if err := s.checkNoPending(ctx, vendorID, typeID); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	active, err := s.repo.Active(ctx, vendorID, typeID)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("load active pks of vendor %d type %d: %w", vendorID, typeID, err)
	}
	if active != nil {
		payload.ReplacesPksID = &active.ID
	}
	change, err := s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: vendorPksEntity, Op: "create", Payload: payload}, ip)
	if errors.Is(err, ErrMasterDataChangePending) {
		// A unique violation lost a race: either the (vendor, type) pending
		// index or the pending-WOD index. Re-check WOD to report the right field.
		if wodErr := s.checkWodFree(ctx, payload.WodNumber); wodErr != nil {
			return db.MasterDataChangeRequest{}, wodErr
		}
	}
	return change, err
}

// SubmitTerminate stages a terminate (op=disable) of an active PKS of the vendor.
func (s *VendorPksAdminService) SubmitTerminate(ctx context.Context, makerID, vendorID, pksID int64, reason, ip string) (db.MasterDataChangeRequest, error) {
	return s.submitReason(ctx, makerID, vendorID, pksID, reason, ip, true)
}

// SubmitUnterminate stages a batalkan terminate (op=enable) of an active, terminated PKS of the vendor.
func (s *VendorPksAdminService) SubmitUnterminate(ctx context.Context, makerID, vendorID, pksID int64, reason, ip string) (db.MasterDataChangeRequest, error) {
	return s.submitReason(ctx, makerID, vendorID, pksID, reason, ip, false)
}

// submitReason touches only the target PKS row; other types are unaffected (Req 3.1).
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
	if err := s.checkNoPending(ctx, vendorID, pks.PksTypeID); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	return s.changes.Submit(ctx, makerID, SubmitRequest{
		EntityType: vendorPksEntity, Op: op, EntityID: &pksID, Before: *pks,
		Payload: VendorPksReasonPayload{VendorID: vendorID, PksTypeID: pks.PksTypeID, Reason: reason},
	}, ip)
}

// checkVendor: missing → 404, INTERNAL → 422, disabled → 409.
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

// resolveActiveType maps a type code to the id of an active type; missing,
// unknown or inactive → 422 on "pks_type" (Req 2.2).
func (s *VendorPksAdminService) resolveActiveType(ctx context.Context, code string) (int64, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return 0, &ValidationError{Field: "pks_type", Message: "Type PKS wajib diisi"}
	}
	t, err := s.repo.PksTypeByCode(ctx, code)
	if err != nil {
		return 0, fmt.Errorf("load pks type %q: %w", code, err)
	}
	if t == nil || !t.IsActive {
		return 0, &ValidationError{Field: "pks_type", Message: "Type PKS tidak dikenal atau nonaktif"}
	}
	return t.ID, nil
}

// checkWodFree: a WOD already on any PKS row or pending create → 409 (Req 2.4).
func (s *VendorPksAdminService) checkWodFree(ctx context.Context, wod string) error {
	taken, err := s.repo.WodTaken(ctx, wod)
	if err != nil {
		return fmt.Errorf("check wod number: %w", err)
	}
	if taken {
		return ErrVendorPksWodTaken
	}
	return nil
}

// checkNoPending: one pending vendor_pks change per (vendor, type) (Req 2.8).
// The partial unique index on (payload vendor_id, pks_type_id) backs this
// against races (Submit maps its 23505 to ErrMasterDataChangePending).
func (s *VendorPksAdminService) checkNoPending(ctx context.Context, vendorID, pksTypeID int64) error {
	pending, err := s.repo.Pending(ctx, vendorID)
	if err != nil {
		return fmt.Errorf("vendor %d pending pks changes: %w", vendorID, err)
	}
	for _, p := range pending {
		var key pendingKey
		if err := json.Unmarshal(p.Payload, &key); err != nil {
			return fmt.Errorf("decode pending pks change %d: %w", p.ID, err)
		}
		if key.PksTypeID == pksTypeID {
			return ErrMasterDataChangePending
		}
	}
	return nil
}

func validateVendorPksCreate(vendorID, pksTypeID int64, req SubmitVendorPksRequest) (VendorPksCreatePayload, error) {
	p := VendorPksCreatePayload{
		VendorID: vendorID, PksTypeID: pksTypeID, WodNumber: strings.TrimSpace(req.WodNumber),
		PksName: strings.TrimSpace(req.PksName), RenewalDays: req.RenewalDays,
		EffectiveDate: strings.TrimSpace(req.EffectiveDate), ExpiryDate: strings.TrimSpace(req.ExpiryDate),
	}
	if err := validateVendorPksWod(p.WodNumber); err != nil {
		return p, err
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
	if err := validateRenewalDays(p.RenewalDays, eff, exp); err != nil {
		return p, err
	}
	return p, nil
}

// validateRenewalDays: required, RenewalDaysMin–RenewalDaysMax and not longer
// than the PKS term (expiry − effective, in days) → 422 on "renewal_days".
// Shared with UpdateReminder (task 4.2).
func validateRenewalDays(days int32, eff, exp time.Time) error {
	if days == 0 {
		return &ValidationError{Field: "renewal_days", Message: "Hari reminder wajib diisi"}
	}
	if days < reports.RenewalDaysMin || days > reports.RenewalDaysMax {
		return &ValidationError{Field: "renewal_days", Message: fmt.Sprintf("Hari reminder harus antara %d dan %d", reports.RenewalDaysMin, reports.RenewalDaysMax)}
	}
	if term := int(exp.Sub(eff).Hours() / 24); int(days) > term {
		return &ValidationError{Field: "renewal_days", Message: fmt.Sprintf("Hari reminder tidak boleh melebihi masa PKS (%d hari)", term)}
	}
	return nil
}

// validateVendorPksWod: already trimmed, 1–100 characters, no format pattern (Req 2.3).
func validateVendorPksWod(v string) error {
	if v == "" {
		return &ValidationError{Field: "wod_number", Message: "No. WOD wajib diisi"}
	}
	if utf8.RuneCountInString(v) > vendorPksWodMax {
		return &ValidationError{Field: "wod_number", Message: fmt.Sprintf("No. WOD maksimal %d karakter", vendorPksWodMax)}
	}
	return nil
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
