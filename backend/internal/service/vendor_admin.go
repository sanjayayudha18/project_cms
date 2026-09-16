package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Sentinel errors for VendorAdminService. Handlers map these to HTTP status
// codes per design.md's error table (Req 7-8): ErrVendorNotFound -> 404,
// ErrVendorCodeConflict -> 409, ErrVendorCodeImmutable -> 400.
var (
	ErrVendorNotFound      = errors.New("vendor not found")
	ErrVendorCodeConflict  = errors.New("vendor code already exists")
	ErrVendorCodeImmutable = errors.New("vendor code cannot be changed")
)

// VendorAdminAuditWriter is the narrow audit-write dependency
// VendorAdminService needs, mirroring auth.AuditWriter -- lets tests fake it
// without a DB.
type VendorAdminAuditWriter interface {
	Write(ctx context.Context, entry audit.Entry) error
}

// VendorAdminRepo is the repository surface VendorAdminService needs.
// *repository.VendorAdminRepository satisfies this automatically. Narrow so
// tests can fake it without a DB (mirrors the SetInitialPasswordService /
// ApprovalOrchestrator narrow-interface pattern, design.md "Service" section).
type VendorAdminRepo interface {
	List(ctx context.Context, arg db.ListVendorsAdminParams) ([]db.ListVendorsAdminRow, error)
	Count(ctx context.Context, arg db.CountVendorsAdminParams) (int64, error)
	GetByID(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error)
	FindByCode(ctx context.Context, code string) (*int64, error)
	Create(ctx context.Context, arg db.CreateVendorAdminParams) (db.CreateVendorAdminRow, error)
	Update(ctx context.Context, arg db.UpdateVendorAdminParams) (db.UpdateVendorAdminRow, error)
	Disable(ctx context.Context, id int64) error
	Enable(ctx context.Context, id int64) error
	CountActiveUsers(ctx context.Context, vendorID int64) (int64, error)
}

// VendorAdminService owns validation, uniqueness resolution, and the
// audit-write guarantee for vendor create/update/disable/enable (Req 7-8).
// Apply-immediately-with-audit, no maker-checker (Resolved Decision 2).
type VendorAdminService struct {
	repo  VendorAdminRepo
	audit VendorAdminAuditWriter
}

// NewVendorAdminService creates a VendorAdminService with the given dependencies.
func NewVendorAdminService(repo VendorAdminRepo, auditWriter VendorAdminAuditWriter) *VendorAdminService {
	return &VendorAdminService{repo: repo, audit: auditWriter}
}

// CreateVendorRequest holds the data for a POST /api/v1/admin/vendors
// request (Req 7.1).
type CreateVendorRequest struct {
	Code         string
	Name         string
	ContactEmail string
	ContactPhone string
	HqAddress    string
}

// UpdateVendorRequest holds the data for a PUT /api/v1/admin/vendors/{id}
// request (Req 7.5). Code is accepted only so an attempt to change it can be
// detected and rejected (Req 7.6) -- nil means the field was not sent at all.
type UpdateVendorRequest struct {
	Code         *string
	Name         string
	ContactEmail string
	ContactPhone string
	HqAddress    string
}

// DisableVendorResult carries the Req 8.5 linked-active-users warning: the
// disable still succeeds even when LinkedUsersWarning > 0.
type DisableVendorResult struct {
	LinkedUsersWarning int64
}

// List returns a page of vendors matching the given filters (Req 6).
// Read-only, never writes an audit entry (Req 9.5).
func (s *VendorAdminService) List(ctx context.Context, arg db.ListVendorsAdminParams) ([]db.ListVendorsAdminRow, error) {
	return s.repo.List(ctx, arg)
}

// Count returns the total number of vendors matching the given filters,
// without pagination (Req 6.6). Read-only, never writes an audit entry.
func (s *VendorAdminService) Count(ctx context.Context, arg db.CountVendorsAdminParams) (int64, error) {
	return s.repo.Count(ctx, arg)
}

// Get returns a vendor by id, including soft-deleted rows. Returns nil, nil
// if no matching vendor is found. Read-only, never writes an audit entry.
func (s *VendorAdminService) Get(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error) {
	return s.repo.GetByID(ctx, id)
}

// Create validates and inserts a new vendor, then writes a vendor_created
// audit entry (Req 7.1-7.4, 9.1-9.4). Never creates a vendor and skips the
// audit write, or vice versa outside of the pre-checked path -- a failed
// audit write after a successful insert surfaces as an error (Req 9.4)
// rather than silently swallowing it, matching DeactivateUserService's
// convention.
func (s *VendorAdminService) Create(ctx context.Context, actorID int64, req CreateVendorRequest, actorIP string) (db.CreateVendorAdminRow, error) {
	code := strings.TrimSpace(req.Code)
	name := strings.TrimSpace(req.Name)
	if code == "" {
		return db.CreateVendorAdminRow{}, &ValidationError{Field: "code", Message: "wajib diisi"}
	}
	if name == "" {
		return db.CreateVendorAdminRow{}, &ValidationError{Field: "name", Message: "wajib diisi"}
	}
	contactEmail := strings.TrimSpace(req.ContactEmail)
	if contactEmail != "" && !isValidEmail(contactEmail) {
		return db.CreateVendorAdminRow{}, &ValidationError{Field: "contact_email", Message: "format email tidak valid"}
	}

	existing, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		return db.CreateVendorAdminRow{}, fmt.Errorf("checking vendor code uniqueness: %w", err)
	}
	if existing != nil {
		return db.CreateVendorAdminRow{}, ErrVendorCodeConflict
	}

	created, err := s.repo.Create(ctx, db.CreateVendorAdminParams{
		Code:         code,
		Name:         name,
		ContactEmail: nilIfEmpty(contactEmail),
		ContactPhone: nilIfEmpty(strings.TrimSpace(req.ContactPhone)),
		HqAddress:    nilIfEmpty(strings.TrimSpace(req.HqAddress)),
	})
	if err != nil {
		if isUniqueViolation(err) {
			// Pre-check above missed a race; the DB constraint is the source of truth.
			return db.CreateVendorAdminRow{}, ErrVendorCodeConflict
		}
		return db.CreateVendorAdminRow{}, fmt.Errorf("creating vendor: %w", err)
	}

	if err := s.audit.Write(ctx, audit.Entry{
		ActorID:    actorID,
		Action:     "vendor_created",
		EntityType: "vendor",
		EntityID:   created.ID,
		After:      created,
		IP:         actorIP,
	}); err != nil {
		return db.CreateVendorAdminRow{}, fmt.Errorf("write audit log: %w", err)
	}

	return created, nil
}

// Update validates and overwrites a vendor's editable fields, rejecting any
// attempt to change code (Req 7.5-7.8, 9.1-9.4). A missing or soft-disabled
// target id is a 404 (ErrVendorNotFound) -- a disabled vendor must be
// enabled before it can be edited, consistent with the user-admin Update
// convention (requirements.md Resolved Decision 5).
func (s *VendorAdminService) Update(ctx context.Context, actorID, id int64, req UpdateVendorRequest, actorIP string) (db.UpdateVendorAdminRow, error) {
	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return db.UpdateVendorAdminRow{}, fmt.Errorf("loading vendor: %w", err)
	}
	if before == nil || before.DeletedAt.Valid {
		return db.UpdateVendorAdminRow{}, ErrVendorNotFound
	}

	if req.Code != nil && strings.TrimSpace(*req.Code) != before.Code {
		return db.UpdateVendorAdminRow{}, ErrVendorCodeImmutable
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return db.UpdateVendorAdminRow{}, &ValidationError{Field: "name", Message: "wajib diisi"}
	}
	contactEmail := strings.TrimSpace(req.ContactEmail)
	if contactEmail != "" && !isValidEmail(contactEmail) {
		return db.UpdateVendorAdminRow{}, &ValidationError{Field: "contact_email", Message: "format email tidak valid"}
	}

	updated, err := s.repo.Update(ctx, db.UpdateVendorAdminParams{
		ID:           id,
		Name:         name,
		ContactEmail: nilIfEmpty(contactEmail),
		ContactPhone: nilIfEmpty(strings.TrimSpace(req.ContactPhone)),
		HqAddress:    nilIfEmpty(strings.TrimSpace(req.HqAddress)),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// The query filters deleted_at IS NULL -- a race where the vendor
			// was disabled between the pre-check and this UPDATE lands here too.
			return db.UpdateVendorAdminRow{}, ErrVendorNotFound
		}
		return db.UpdateVendorAdminRow{}, fmt.Errorf("updating vendor: %w", err)
	}

	if err := s.audit.Write(ctx, audit.Entry{
		ActorID:    actorID,
		Action:     "vendor_updated",
		EntityType: "vendor",
		EntityID:   id,
		Before:     before,
		After:      updated,
		IP:         actorIP,
	}); err != nil {
		return db.UpdateVendorAdminRow{}, fmt.Errorf("write audit log: %w", err)
	}

	return updated, nil
}

// Disable soft-disables a vendor and writes a vendor_deactivated audit entry
// (Req 8.1, 8.4-8.6, 9.1-9.4). The disable still succeeds even when the
// vendor has active linked users -- the caller gets back
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

	if err := s.repo.Disable(ctx, id); err != nil {
		return DisableVendorResult{}, fmt.Errorf("disabling vendor: %w", err)
	}

	if err := s.audit.Write(ctx, audit.Entry{
		ActorID:    actorID,
		Action:     "vendor_deactivated",
		EntityType: "vendor",
		EntityID:   id,
		IP:         actorIP,
	}); err != nil {
		return DisableVendorResult{}, fmt.Errorf("write audit log: %w", err)
	}

	return DisableVendorResult{LinkedUsersWarning: linked}, nil
}

// Enable reverses Disable and writes a vendor_reactivated audit entry (Req
// 8.2, 8.4, 8.6, 9.1-9.4). A non-existent id is a 404 with NO audit entry
// written -- an explicit existence pre-check, since the underlying
// EnableVendor UPDATE silently no-ops on 0 rows affected and would
// otherwise let this method report success for an id that was never real.
func (s *VendorAdminService) Enable(ctx context.Context, actorID, id int64, actorIP string) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("loading vendor: %w", err)
	}
	if existing == nil {
		return ErrVendorNotFound
	}

	if err := s.repo.Enable(ctx, id); err != nil {
		return fmt.Errorf("enabling vendor: %w", err)
	}

	if err := s.audit.Write(ctx, audit.Entry{
		ActorID:    actorID,
		Action:     "vendor_reactivated",
		EntityType: "vendor",
		EntityID:   id,
		IP:         actorIP,
	}); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}

	return nil
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
