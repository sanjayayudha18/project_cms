package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"

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
	ListByVendor(ctx context.Context, vendorID int64) ([]db.ListVendorPksByVendorRow, error)
	Pending(ctx context.Context, vendorID int64) ([]db.GetPendingVendorPksChangesRow, error)
	PksType(ctx context.Context, id int64) (*db.VendorPksType, error)
	PksTypeByCode(ctx context.Context, code string) (*db.VendorPksType, error)
	WodTaken(ctx context.Context, wod string) (bool, error)
	Status(ctx context.Context, arg db.GetVendorPksStatusParams) (string, error)
}

// VendorPksAdminService validates PKS changes and stages them through
// MasterDataChangeService.Submit (which re-checks ADMIN/ADMIN_PARAM).
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
// PksType is a type code (e.g. "FLM"), resolved to an active type.
type SubmitVendorPksRequest struct {
	PksType       string `json:"pks_type"`
	WodNumber     string `json:"wod_number"`
	PksName       string `json:"pks_name"`
	EffectiveDate string `json:"effective_date"`
	ExpiryDate    string `json:"expiry_date"`
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

// VendorPksRecord is one PKS row of the detail. RemainingDays is set only for
// the active PKS; Replaced* only for history.
type VendorPksRecord struct {
	ID                int64      `json:"id"`
	WodNumber         string     `json:"wod_number"`
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

// VendorPksPending is the pending PKS change of one (vendor, type).
type VendorPksPending struct {
	ChangeRequestID int64           `json:"change_request_id"`
	Op              string          `json:"op"`
	Payload         json.RawMessage `json:"payload"`
	MakerID         int64           `json:"maker_id"`
	MakerName       string          `json:"maker_name"`
	CreatedAt       *time.Time      `json:"created_at"`
}

// VendorPksTypeRef names the PKS type of a group.
type VendorPksTypeRef struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// VendorPksGroup is the PKS of one type: active (+ status), history, pending.
type VendorPksGroup struct {
	PksType   VendorPksTypeRef  `json:"pks_type"`
	PksStatus *string           `json:"pks_status"`
	Active    *VendorPksRecord  `json:"active"`
	History   []VendorPksRecord `json:"history"`
	Pending   *VendorPksPending `json:"pending"`

	typeID             int64
	activeExpiry       pgtype.Date
	activeTerminatedAt pgtype.Timestamptz
}

// VendorPksDetail is GET /admin/vendors/{vendorID}/pks: one group per type the
// vendor has a PKS or a pending change for, ordered by type code (Req 3.2).
// Groups is empty for an INTERNAL vendor.
type VendorPksDetail struct {
	VendorID      int64            `json:"vendor_id"`
	Kind          string           `json:"kind"`
	Today         string           `json:"today"`
	RenewalMonths int              `json:"renewal_months"`
	Groups        []VendorPksGroup `json:"groups"`
}

// pendingKey is the part of a pending vendor_pks payload the per-type checks need.
type pendingKey struct {
	PksTypeID int64 `json:"pks_type_id"`
}

// Get returns, per type, the active PKS (status + remaining days), history and
// pending change of a vendor, all from the primary (Req 3.2, 3.3).
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
		RenewalMonths: s.renewalMonths, Groups: []VendorPksGroup{},
	}
	if vendor.Kind == vendorKindInternal {
		return detail, nil
	}

	groups, err := s.recordGroups(ctx, vendorID, today)
	if err != nil {
		return nil, err
	}
	if groups, err = s.withPending(ctx, vendorID, groups); err != nil {
		return nil, err
	}
	slices.SortFunc(groups, func(a, b VendorPksGroup) int { return strings.Compare(a.PksType.Code, b.PksType.Code) })

	for i := range groups {
		g := &groups[i]
		status, err := s.repo.Status(ctx, db.GetVendorPksStatusParams{
			Kind: vendor.Kind, ExpiryDate: g.activeExpiry, TerminatedAt: g.activeTerminatedAt,
			Today: toPgDate(today), RenewalMonths: int32(s.renewalMonths), //nolint:gosec // G115: bounded by reports.ParseVendorPksRenewalMonths
		})
		if err != nil {
			return nil, fmt.Errorf("vendor %d pks status type %s: %w", vendorID, g.PksType.Code, err)
		}
		if status != "" {
			g.PksStatus = &status
		}
	}
	detail.Groups = groups
	return detail, nil
}

// recordGroups builds one group per type from the vendor's PKS rows (already
// ordered by type code, active first).
func (s *VendorPksAdminService) recordGroups(ctx context.Context, vendorID int64, today time.Time) ([]VendorPksGroup, error) {
	rows, err := s.repo.ListByVendor(ctx, vendorID)
	if err != nil {
		return nil, fmt.Errorf("list vendor %d pks: %w", vendorID, err)
	}
	groups := []VendorPksGroup{}
	for _, r := range rows {
		if len(groups) == 0 || groups[len(groups)-1].typeID != r.PksTypeID {
			groups = append(groups, VendorPksGroup{
				PksType: VendorPksTypeRef{Code: r.PksTypeCode, Name: r.PksTypeName},
				History: []VendorPksRecord{}, typeID: r.PksTypeID,
			})
		}
		g := &groups[len(groups)-1]
		rec := newVendorPksRecord(r)
		if r.ReplacedAt.Valid {
			g.History = append(g.History, rec)
			continue
		}
		days := int(r.ExpiryDate.Time.Sub(today) / (24 * time.Hour))
		rec.RemainingDays = &days
		g.Active = &rec
		g.activeExpiry, g.activeTerminatedAt = r.ExpiryDate, r.TerminatedAt
	}
	return groups, nil
}

// withPending attaches each pending change to its type's group, adding a group
// for a type that has only a pending create.
func (s *VendorPksAdminService) withPending(ctx context.Context, vendorID int64, groups []VendorPksGroup) ([]VendorPksGroup, error) {
	pending, err := s.repo.Pending(ctx, vendorID)
	if err != nil {
		return nil, fmt.Errorf("vendor %d pending pks changes: %w", vendorID, err)
	}
	for _, p := range pending {
		var key pendingKey
		if err := json.Unmarshal(p.Payload, &key); err != nil {
			return nil, fmt.Errorf("decode pending pks change %d: %w", p.ID, err)
		}
		idx := slices.IndexFunc(groups, func(g VendorPksGroup) bool { return g.typeID == key.PksTypeID })
		if idx < 0 {
			t, err := s.repo.PksType(ctx, key.PksTypeID)
			if err != nil {
				return nil, fmt.Errorf("load pks type %d: %w", key.PksTypeID, err)
			}
			if t == nil {
				return nil, fmt.Errorf("pending pks change %d: pks type %d not found", p.ID, key.PksTypeID)
			}
			groups = append(groups, VendorPksGroup{
				PksType: VendorPksTypeRef{Code: t.Code, Name: t.Name}, History: []VendorPksRecord{}, typeID: t.ID,
			})
			idx = len(groups) - 1
		}
		groups[idx].Pending = &VendorPksPending{
			ChangeRequestID: p.ID, Op: p.Op, Payload: json.RawMessage(p.Payload),
			MakerID: p.MakerID, MakerName: p.MakerName, CreatedAt: timestamptzToPtr(p.CreatedAt),
		}
	}
	return groups, nil
}

func newVendorPksRecord(r db.ListVendorPksByVendorRow) VendorPksRecord {
	rec := VendorPksRecord{
		ID: r.ID, WodNumber: r.WodNumber, PksName: r.PksName,
		EffectiveDate: r.EffectiveDate.Time.Format(vendorPksDateLayout), ExpiryDate: r.ExpiryDate.Time.Format(vendorPksDateLayout),
		ValidityMonths: r.ValidityMonths, CreatedAt: timestamptzToPtr(r.CreatedAt),
		TerminatedAt: timestamptzToPtr(r.TerminatedAt), TerminationReason: r.TerminationReason,
	}
	if r.ReplacedAt.Valid {
		rec.ReplacedAt, rec.ReplacedByPksID = timestamptzToPtr(r.ReplacedAt), r.ReplacedByPksID
	}
	return rec
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
		PksName:       strings.TrimSpace(req.PksName),
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
	return p, nil
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
