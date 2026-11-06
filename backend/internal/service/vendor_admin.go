package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/pkg/reports"
)

// Sentinel errors for VendorAdminService. Handlers map these to HTTP status
// codes per design.md's error table (Req 7-8): ErrVendorNotFound -> 404,
// ErrVendorCodeConflict -> 409, ErrVendorCodeImmutable -> 400.
var (
	ErrVendorNotFound      = errors.New("vendor not found")
	ErrVendorCodeConflict  = errors.New("vendor code already exists")
	ErrVendorCodeImmutable = errors.New("vendor code cannot be changed")
)

// VendorAdminSubmitter is the narrow MasterDataChangeService surface
// VendorAdminService needs (maker-checker, D1 / plan.md T4.1).
type VendorAdminSubmitter interface {
	Submit(ctx context.Context, makerID int64, req SubmitRequest, actorIP string) (db.MasterDataChangeRequest, error)
}

// VendorAdminRepo is the read-only repository surface VendorAdminService
// needs. *repository.VendorAdminRepository satisfies this automatically.
// Deliberately has no write methods: every vendor mutation goes through
// MasterDataChangeService.Submit and is applied by VendorApplier.
type VendorAdminRepo interface {
	List(ctx context.Context, arg db.ListVendorsAdminParams) ([]db.ListVendorsAdminRow, error)
	Count(ctx context.Context, arg db.CountVendorsAdminParams) (int64, error)
	GetByID(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error)
	FindByCode(ctx context.Context, code string) (*int64, error)
	CountActiveUsers(ctx context.Context, vendorID int64) (int64, error)
	GetKindAndDirector(ctx context.Context, vendorID int64) (string, *db.GetVendorDirectorPicRow, error)
}

// VendorAdminService owns validation and uniqueness pre-checks for vendor
// create/update/disable/enable (Req 7-8) and stages each change via
// MasterDataChangeService.Submit. Since T4.1 nothing is applied here: the
// vendor row is written by VendorApplier once the change is approved, and the
// audit trail is the engine's (submit + apply), not this service's.
type VendorAdminService struct {
	repo    VendorAdminRepo
	changes VendorAdminSubmitter
	// renewalMonths = VENDOR_PKS_RENEWAL_MONTHS (vendor-pks 6.1); now feeds
	// "today" (WIB) into the PKS status filter/list (6.2).
	renewalMonths int
	now           func() time.Time
}

// VendorAdminOption tunes optional VendorAdminService settings, so existing
// two-argument callers (tests, import wiring) stay valid.
type VendorAdminOption func(*VendorAdminService)

// WithPksRenewalMonths sets the "Need Renewal" threshold in months.
func WithPksRenewalMonths(months int) VendorAdminOption {
	return func(s *VendorAdminService) { s.renewalMonths = months }
}

// NewVendorAdminService creates a VendorAdminService with the given dependencies.
// Without WithPksRenewalMonths the threshold is reports.VendorPksRenewalMonthsDefault.
func NewVendorAdminService(repo VendorAdminRepo, changes VendorAdminSubmitter, opts ...VendorAdminOption) *VendorAdminService {
	s := &VendorAdminService{repo: repo, changes: changes, renewalMonths: reports.VendorPksRenewalMonthsDefault, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// CreateVendorRequest holds the data for a POST /api/v1/admin/vendors
// request (Req 7.1). LegalName and NPWP (T4.3) are optional: blank is stored
// as NULL.
type CreateVendorRequest struct {
	Code         string
	Name         string
	LegalName    string
	NPWP         string
	ContactEmail string
	ContactPhone string
	HqAddress    string
}

// UpdateVendorRequest holds the data for a PUT /api/v1/admin/vendors/{id}
// request (Req 7.5). Code is accepted only so an attempt to change it can be
// detected and rejected (Req 7.6) -- nil means the field was not sent at all.
//
// LegalName and NPWP (T4.3) are tri-state, unlike the other fields (which an
// update always overwrites): nil = not sent, keep the vendor's current value;
// blank = clear it (NULL); otherwise set it. A client that predates these
// fields therefore cannot wipe them by omission.
type UpdateVendorRequest struct {
	Code         *string
	Name         string
	LegalName    *string
	NPWP         *string
	ContactEmail string
	ContactPhone string
	HqAddress    string
}

// npwpRe is the shape vendors_npwp_chk enforces: 15 or 16 ASCII digits.
var npwpRe = regexp.MustCompile(`^[0-9]{15,16}$`)

// normalizeNPWP accepts an NPWP as people write it ("01.234.567.8-901.000"),
// strips the punctuation and spaces, and returns the digits only -- what the
// column stores. Blank returns "" (NULL). Anything that isn't 15 or 16 digits
// after stripping is a ValidationError, so the DB CHECK is never the thing that
// rejects it.
func normalizeNPWP(raw string) (string, error) {
	s := strings.NewReplacer(".", "", "-", "", " ", "").Replace(strings.TrimSpace(raw))
	if s == "" {
		return "", nil
	}
	if !npwpRe.MatchString(s) {
		return "", &ValidationError{Field: "npwp", Message: "harus 15 atau 16 digit (titik, strip, dan spasi diabaikan)"}
	}
	return s, nil
}

func valueOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// DisableVendorResult carries the staged disable change plus the Req 8.5
// linked-active-users warning: staging still succeeds even when
// LinkedUsersWarning > 0. The count is as of submit time.
type DisableVendorResult struct {
	Change             db.MasterDataChangeRequest
	LinkedUsersWarning int64
}

// pksToday is "today" in Asia/Jakarta as a pg date, for vendor_pks_status().
func (s *VendorAdminService) pksToday() pgtype.Date {
	return pgtype.Date{Time: reports.TodayWIB(s.now()), Valid: true}
}

// List returns a page of vendors matching the given filters (Req 6).
// Today/RenewalMonths are filled here (vendor-pks 6.2), overriding the caller.
// Read-only, never writes an audit entry (Req 9.5).
func (s *VendorAdminService) List(ctx context.Context, arg db.ListVendorsAdminParams) ([]db.ListVendorsAdminRow, error) {
	arg.Today, arg.RenewalMonths = s.pksToday(), int32(s.renewalMonths) //nolint:gosec // G115: bounded by reports.ParseVendorPksRenewalMonths
	return s.repo.List(ctx, arg)
}

// Count returns the total number of vendors matching the given filters,
// without pagination (Req 6.6). Read-only, never writes an audit entry.
func (s *VendorAdminService) Count(ctx context.Context, arg db.CountVendorsAdminParams) (int64, error) {
	arg.Today, arg.RenewalMonths = s.pksToday(), int32(s.renewalMonths) //nolint:gosec // G115: bounded by reports.ParseVendorPksRenewalMonths
	return s.repo.Count(ctx, arg)
}

// Get returns a vendor by id, including soft-deleted rows. Returns nil, nil
// if no matching vendor is found. Read-only, never writes an audit entry.
func (s *VendorAdminService) Get(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error) {
	return s.repo.GetByID(ctx, id)
}

// GetKindAndDirector returns the extra detail fields (kind + active
// Director_PIC, nil when none) read from the primary (vendor-pks 6.2).
func (s *VendorAdminService) GetKindAndDirector(ctx context.Context, id int64) (string, *db.GetVendorDirectorPicRow, error) {
	return s.repo.GetKindAndDirector(ctx, id)
}

// Create validates and stages a new vendor (Req 7.1-7.4). The code
// uniqueness check here is an early answer only; the DB unique constraint is
// re-enforced at apply time (a duplicate that was pending twice surfaces then
// as ErrVendorCodeConflict).
func (s *VendorAdminService) Create(ctx context.Context, actorID int64, req CreateVendorRequest, actorIP string) (db.MasterDataChangeRequest, error) {
	code := strings.TrimSpace(req.Code)
	name := strings.TrimSpace(req.Name)
	if code == "" {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "code", Message: "wajib diisi"}
	}
	if name == "" {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "name", Message: "wajib diisi"}
	}
	contactEmail := strings.TrimSpace(req.ContactEmail)
	if contactEmail != "" && !isValidEmail(contactEmail) {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "contact_email", Message: "format email tidak valid"}
	}
	npwp, err := normalizeNPWP(req.NPWP)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}

	existing, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("checking vendor code uniqueness: %w", err)
	}
	if existing != nil {
		return db.MasterDataChangeRequest{}, ErrVendorCodeConflict
	}

	return s.changes.Submit(ctx, actorID, SubmitRequest{
		EntityType: "vendor", Op: "create",
		Payload: vendorCreatePayload{
			Code: code, Name: name, LegalName: strings.TrimSpace(req.LegalName), NPWP: npwp, ContactEmail: contactEmail,
			ContactPhone: strings.TrimSpace(req.ContactPhone), HqAddress: strings.TrimSpace(req.HqAddress),
		},
	}, actorIP)
}

// Update validates and stages an overwrite of a vendor's editable fields,
// rejecting any attempt to change code (Req 7.5-7.8). A missing or
// soft-disabled target id is a 404 (ErrVendorNotFound) -- a disabled vendor
// must be enabled before it can be edited (requirements.md Resolved Decision
// 5). The vendor row as loaded here is the change's "before" snapshot, so an
// edit that lands after someone else changed the vendor is marked stale
// rather than silently overwriting (T2.5).
func (s *VendorAdminService) Update(ctx context.Context, actorID, id int64, req UpdateVendorRequest, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("loading vendor: %w", err)
	}
	if before == nil || before.DeletedAt.Valid {
		return db.MasterDataChangeRequest{}, ErrVendorNotFound
	}

	if req.Code != nil && strings.TrimSpace(*req.Code) != before.Code {
		return db.MasterDataChangeRequest{}, ErrVendorCodeImmutable
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "name", Message: "wajib diisi"}
	}
	contactEmail := strings.TrimSpace(req.ContactEmail)
	if contactEmail != "" && !isValidEmail(contactEmail) {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "contact_email", Message: "format email tidak valid"}
	}

	// Tri-state fields (see UpdateVendorRequest): absent keeps the loaded value.
	legalName, npwp := valueOrEmpty(before.LegalName), valueOrEmpty(before.Npwp)
	if req.LegalName != nil {
		legalName = strings.TrimSpace(*req.LegalName)
	}
	if req.NPWP != nil {
		if npwp, err = normalizeNPWP(*req.NPWP); err != nil {
			return db.MasterDataChangeRequest{}, err
		}
	}

	return s.changes.Submit(ctx, actorID, SubmitRequest{
		EntityType: "vendor", Op: "update", EntityID: &id, Before: before,
		Payload: vendorUpdatePayload{
			Name: name, LegalName: legalName, NPWP: npwp, ContactEmail: contactEmail,
			ContactPhone: strings.TrimSpace(req.ContactPhone), HqAddress: strings.TrimSpace(req.HqAddress),
		},
	}, actorIP)
}

// Disable stages a soft-disable of a vendor (Req 8.1, 8.4-8.6). Staging still
// succeeds when the vendor has active linked users -- the caller gets back
// DisableVendorResult.LinkedUsersWarning so the handler/UI can surface the
// warning (Req 8.5).
func (s *VendorAdminService) Disable(ctx context.Context, actorID, id int64, actorIP string) (DisableVendorResult, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return DisableVendorResult{}, fmt.Errorf("loading vendor: %w", err)
	}
	if existing == nil {
		return DisableVendorResult{}, ErrVendorNotFound
	}

	linked, err := s.repo.CountActiveUsers(ctx, id)
	if err != nil {
		return DisableVendorResult{}, fmt.Errorf("counting linked active users: %w", err)
	}

	change, err := s.changes.Submit(ctx, actorID, SubmitRequest{EntityType: "vendor", Op: "disable", EntityID: &id, Before: existing}, actorIP)
	if err != nil {
		return DisableVendorResult{}, err
	}
	return DisableVendorResult{Change: change, LinkedUsersWarning: linked}, nil
}

// Enable stages the reverse of Disable (Req 8.2, 8.4, 8.6). A non-existent id
// is a 404 and stages nothing.
func (s *VendorAdminService) Enable(ctx context.Context, actorID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("loading vendor: %w", err)
	}
	if existing == nil {
		return db.MasterDataChangeRequest{}, ErrVendorNotFound
	}
	return s.changes.Submit(ctx, actorID, SubmitRequest{EntityType: "vendor", Op: "enable", EntityID: &id, Before: existing}, actorIP)
}

// nilIfEmpty converts an already-trimmed string into a *string for optional
// nullable columns (contact_email, contact_phone, hq_address), nil meaning
// "not provided" rather than an empty string.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// isValidEmail is a basic format check via the stdlib RFC 5322 parser (Req
// 7.2). Good enough for a format guard; it is not a deliverability check.
func isValidEmail(s string) bool {
	_, err := mail.ParseAddress(s)
	return err == nil
}
