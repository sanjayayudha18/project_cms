package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/pkg/auth"
)

// pgUniqueViolation is the Postgres error code for a unique-constraint
// violation, used as the create/update fallback when the pre-check misses a
// race (design.md "Error -> HTTP mapping").
const pgUniqueViolation = "23505"

// Sentinel/structured errors for UserAdminService, distinct from
// pkg/auth.ValidationError (422 field validation) per design.md's error
// table: ConflictError -> 409, ReferenceError -> 400 "invalid_reference",
// the three ErrXxx sentinels -> 400 "bad_request".
var (
	// ErrImmutableField: an update attempted to change username or
	// auth_source (Req 4.2), which are identity/auth-path and permanent.
	ErrImmutableField = errors.New("username dan auth_source tidak dapat diubah")
	// ErrSelfSupervision: supervisor_id equals the target user's own id
	// (Req 4.6, mirrors the users_supervisor_not_self_chk DB CHECK).
	ErrSelfSupervision = errors.New("supervisor_id tidak boleh sama dengan id sendiri")
	// ErrLdapFieldsNotAllowed: vendor_id or temporary_password was supplied
	// for auth_source=ldap, which is vendor/local-only (Req 3.4).
	ErrLdapFieldsNotAllowed = errors.New("vendor_id dan temporary_password tidak boleh diisi untuk auth_source ldap")
)

// ConflictError represents a 409 uniqueness conflict on a specific field
// (username/email/employee_id, Req 3.5, 4.5).
type ConflictError struct {
	Field   string
	Message string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ReferenceError represents a 400 "invalid_reference" error: role/vendor_id/
// supervisor_id was supplied but does not resolve to an existing record
// (Req 3.6, 3.7, 4.7).
type ReferenceError struct {
	Field   string
	Message string
}

func (e *ReferenceError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// UserAdminRepo is the repository surface UserAdminService needs.
// *repository.UserAdminRepository satisfies this automatically. Narrow so
// tests can fake it without a DB (mirrors the ChangePasswordService /
// ApprovalOrchestrator narrow-interface pattern).
type UserAdminRepo interface {
	List(ctx context.Context, arg db.ListUsersAdminParams) ([]db.ListUsersAdminRow, error)
	Count(ctx context.Context, arg db.CountUsersAdminParams) (int64, error)
	GetByID(ctx context.Context, id int64) (*db.GetUserAdminByIDRow, error)
	Create(ctx context.Context, arg db.CreateUserAdminParams) (db.CreateUserAdminRow, error)
	Update(ctx context.Context, arg db.UpdateUserAdminParams) (db.UpdateUserAdminRow, error)
	FindByUsername(ctx context.Context, username string) (*int64, error)
	FindByEmail(ctx context.Context, email string) (*int64, error)
	FindByEmailExcludingID(ctx context.Context, email string, excludeID int64) (*int64, error)
	FindByEmployeeID(ctx context.Context, employeeID string) (*int64, error)
	FindByEmployeeIDExcludingID(ctx context.Context, employeeID string, excludeID int64) (*int64, error)
	GetRoleByName(ctx context.Context, role string) (*db.GetRoleByNameRow, error)
	VendorExists(ctx context.Context, vendorID int64) (bool, error)
	SupervisorExists(ctx context.Context, userID int64) (bool, error)
}

// UserAdminService owns validation, uniqueness/reference resolution, and
// the audit-write guarantee for user create/update (Req 3-4). Disable/
// enable are NOT implemented here -- the handler calls the existing
// DeactivateUserService.Deactivate/Reactivate directly (design.md).
type UserAdminService struct {
	repo  UserAdminRepo
	audit AuditWriter
}

// NewUserAdminService creates a UserAdminService with the given dependencies.
func NewUserAdminService(repo UserAdminRepo, auditWriter AuditWriter) *UserAdminService {
	return &UserAdminService{repo: repo, audit: auditWriter}
}

// CreateUserRequest holds the data for a POST /api/v1/admin/users request
// (Req 3.1). Role is the role text (e.g. "APPACCESS"), resolved to role_id.
type CreateUserRequest struct {
	Username          string
	FullName          string
	Email             string
	Role              string
	IsKaryawan        bool
	AuthSource        string // "ldap" | "local"
	TemporaryPassword string // required when AuthSource == "local"
	EmployeeID        string
	VendorID          *int64
	SupervisorID      *int64
	ApprovalLevel     *int32
}

// UpdateUserRequest holds the data for a PUT /api/v1/admin/users/{id}
// request (Req 4.1). Username/AuthSource are accepted only so an attempt to
// change them can be detected and rejected (Req 4.2) -- nil means the field
// was not sent at all.
type UpdateUserRequest struct {
	Username      *string
	AuthSource    *string
	FullName      string
	Email         string
	Role          string
	IsKaryawan    bool
	EmployeeID    string
	VendorID      *int64
	SupervisorID  *int64
	ApprovalLevel *int32
}

// List returns a page of users matching the given filters (Req 2). Read-
// only, never writes an audit entry (Req 9.5).
func (s *UserAdminService) List(ctx context.Context, arg db.ListUsersAdminParams) ([]db.ListUsersAdminRow, error) {
	return s.repo.List(ctx, arg)
}

// Count returns the total number of users matching the given filters,
// without pagination (Req 2.8). Read-only, never writes an audit entry.
func (s *UserAdminService) Count(ctx context.Context, arg db.CountUsersAdminParams) (int64, error) {
	return s.repo.Count(ctx, arg)
}

// Get returns a user by id, including soft-deleted rows. Returns nil, nil
// if no matching user is found. Read-only, never writes an audit entry.
func (s *UserAdminService) Get(ctx context.Context, id int64) (*db.GetUserAdminByIDRow, error) {
	return s.repo.GetByID(ctx, id)
}

// Create validates the request, enforces the auth_source rules (Resolved
// Decision 5: local sets password_hash + must_change_password=true in the
// SAME insert, no separate SetInitialPassword call), resolves references,
// inserts the user, and writes a user_created audit entry (Req 3, 9.1-9.4).
// The temporary password is NEVER placed in the audit payload -- the
// created row (db.CreateUserAdminRow) has no password_hash field at all.
func (s *UserAdminService) Create(ctx context.Context, actorID int64, req CreateUserRequest, actorIP string) (db.CreateUserAdminRow, error) {
	username := strings.TrimSpace(req.Username)
	fullName := strings.TrimSpace(req.FullName)
	email := strings.TrimSpace(req.Email)
	role := strings.TrimSpace(req.Role)
	authSource := strings.TrimSpace(req.AuthSource)

	if username == "" {
		return db.CreateUserAdminRow{}, &auth.ValidationError{Field: "username", Message: "wajib diisi"}
	}
	if fullName == "" {
		return db.CreateUserAdminRow{}, &auth.ValidationError{Field: "full_name", Message: "wajib diisi"}
	}
	if email == "" {
		return db.CreateUserAdminRow{}, &auth.ValidationError{Field: "email", Message: "wajib diisi"}
	}
	if !isValidEmail(email) {
		return db.CreateUserAdminRow{}, &auth.ValidationError{Field: "email", Message: "format email tidak valid"}
	}
	if role == "" {
		return db.CreateUserAdminRow{}, &auth.ValidationError{Field: "role", Message: "wajib diisi"}
	}
	if authSource != "ldap" && authSource != "local" {
		return db.CreateUserAdminRow{}, &auth.ValidationError{Field: "auth_source", Message: "harus ldap atau local"}
	}

	var passwordHash *string
	mustChangePassword := false

	switch authSource {
	case "local":
		if req.VendorID == nil {
			return db.CreateUserAdminRow{}, &auth.ValidationError{Field: "vendor_id", Message: "wajib diisi untuk auth_source local"}
		}
		if req.TemporaryPassword == "" {
			return db.CreateUserAdminRow{}, &auth.ValidationError{Field: "temporary_password", Message: "wajib diisi untuk auth_source local"}
		}
		if err := auth.ValidatePasswordStrength(req.TemporaryPassword); err != nil {
			return db.CreateUserAdminRow{}, remapPasswordFieldError(err)
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.TemporaryPassword), BcryptCost)
		if err != nil {
			return db.CreateUserAdminRow{}, auth.ErrServiceUnavailable
		}
		h := string(hash)
		passwordHash = &h
		mustChangePassword = true
	case "ldap":
		if req.VendorID != nil || req.TemporaryPassword != "" {
			return db.CreateUserAdminRow{}, ErrLdapFieldsNotAllowed
		}
	}

	if existingID, err := s.repo.FindByUsername(ctx, username); err != nil {
		return db.CreateUserAdminRow{}, fmt.Errorf("checking username uniqueness: %w", err)
	} else if existingID != nil {
		return db.CreateUserAdminRow{}, &ConflictError{Field: "username", Message: "sudah digunakan"}
	}
	if existingID, err := s.repo.FindByEmail(ctx, email); err != nil {
		return db.CreateUserAdminRow{}, fmt.Errorf("checking email uniqueness: %w", err)
	} else if existingID != nil {
		return db.CreateUserAdminRow{}, &ConflictError{Field: "email", Message: "sudah digunakan"}
	}
	employeeID := strings.TrimSpace(req.EmployeeID)
	if employeeID != "" {
		if existingID, err := s.repo.FindByEmployeeID(ctx, employeeID); err != nil {
			return db.CreateUserAdminRow{}, fmt.Errorf("checking employee_id uniqueness: %w", err)
		} else if existingID != nil {
			return db.CreateUserAdminRow{}, &ConflictError{Field: "employee_id", Message: "sudah digunakan"}
		}
	}

	roleRow, err := s.repo.GetRoleByName(ctx, role)
	if err != nil {
		return db.CreateUserAdminRow{}, fmt.Errorf("resolving role: %w", err)
	}
	if roleRow == nil {
		return db.CreateUserAdminRow{}, &ReferenceError{Field: "role", Message: "tidak ditemukan"}
	}
	if req.VendorID != nil {
		ok, err := s.repo.VendorExists(ctx, *req.VendorID)
		if err != nil {
			return db.CreateUserAdminRow{}, fmt.Errorf("checking vendor reference: %w", err)
		}
		if !ok {
			return db.CreateUserAdminRow{}, &ReferenceError{Field: "vendor_id", Message: "tidak ditemukan"}
		}
	}
	if req.SupervisorID != nil {
		ok, err := s.repo.SupervisorExists(ctx, *req.SupervisorID)
		if err != nil {
			return db.CreateUserAdminRow{}, fmt.Errorf("checking supervisor reference: %w", err)
		}
		if !ok {
			return db.CreateUserAdminRow{}, &ReferenceError{Field: "supervisor_id", Message: "tidak ditemukan"}
		}
	}

	created, err := s.repo.Create(ctx, db.CreateUserAdminParams{
		RoleID:             roleRow.ID,
		EmployeeID:         nilIfEmpty(employeeID),
		Username:           username,
		FullName:           fullName,
		Email:              email,
		IsKaryawan:         req.IsKaryawan,
		AuthSource:         authSource,
		PasswordHash:       passwordHash,
		VendorID:           req.VendorID,
		SupervisorID:       req.SupervisorID,
		ApprovalLevel:      req.ApprovalLevel,
		MustChangePassword: mustChangePassword,
	})
	if err != nil {
		if field, ok := conflictFieldFromPgError(err); ok {
			return db.CreateUserAdminRow{}, &ConflictError{Field: field, Message: "sudah digunakan"}
		}
		return db.CreateUserAdminRow{}, fmt.Errorf("creating user: %w", err)
	}

	// Never include password/hash values in the audit trail -- created has
	// no PasswordHash field (CreateUserAdmin's RETURNING excludes it).
	if err := s.audit.Write(ctx, audit.Entry{
		ActorID:    actorID,
		Action:     "user_created",
		EntityType: "user",
		EntityID:   created.ID,
		After:      created,
		IP:         actorIP,
	}); err != nil {
		return db.CreateUserAdminRow{}, fmt.Errorf("write audit log: %w", err)
	}

	return created, nil
}

// Update validates and overwrites a user's editable fields, rejecting any
// attempt to change username/auth_source, self-supervision, and unresolved
// references (Req 4, 9.1-9.4). A missing or soft-disabled target id is a
// 404 (auth.ErrUserNotFound) -- a disabled user must be enabled before it
// can be edited.
func (s *UserAdminService) Update(ctx context.Context, actorID, id int64, req UpdateUserRequest, actorIP string) (db.UpdateUserAdminRow, error) {
	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return db.UpdateUserAdminRow{}, fmt.Errorf("loading user: %w", err)
	}
	if before == nil || before.DeletedAt.Valid {
		return db.UpdateUserAdminRow{}, auth.ErrUserNotFound
	}

	if req.Username != nil && strings.TrimSpace(*req.Username) != before.Username {
		return db.UpdateUserAdminRow{}, ErrImmutableField
	}
	if req.AuthSource != nil && strings.TrimSpace(*req.AuthSource) != before.AuthSource {
		return db.UpdateUserAdminRow{}, ErrImmutableField
	}

	fullName := strings.TrimSpace(req.FullName)
	email := strings.TrimSpace(req.Email)
	role := strings.TrimSpace(req.Role)
	if fullName == "" {
		return db.UpdateUserAdminRow{}, &auth.ValidationError{Field: "full_name", Message: "wajib diisi"}
	}
	if email == "" {
		return db.UpdateUserAdminRow{}, &auth.ValidationError{Field: "email", Message: "wajib diisi"}
	}
	if !isValidEmail(email) {
		return db.UpdateUserAdminRow{}, &auth.ValidationError{Field: "email", Message: "format email tidak valid"}
	}
	if role == "" {
		return db.UpdateUserAdminRow{}, &auth.ValidationError{Field: "role", Message: "wajib diisi"}
	}

	if req.SupervisorID != nil && *req.SupervisorID == id {
		return db.UpdateUserAdminRow{}, ErrSelfSupervision
	}

	if email != before.Email {
		if existingID, err := s.repo.FindByEmailExcludingID(ctx, email, id); err != nil {
			return db.UpdateUserAdminRow{}, fmt.Errorf("checking email uniqueness: %w", err)
		} else if existingID != nil {
			return db.UpdateUserAdminRow{}, &ConflictError{Field: "email", Message: "sudah digunakan"}
		}
	}
	employeeID := strings.TrimSpace(req.EmployeeID)
	beforeEmployeeID := ""
	if before.EmployeeID != nil {
		beforeEmployeeID = *before.EmployeeID
	}
	if employeeID != "" && employeeID != beforeEmployeeID {
		if existingID, err := s.repo.FindByEmployeeIDExcludingID(ctx, employeeID, id); err != nil {
			return db.UpdateUserAdminRow{}, fmt.Errorf("checking employee_id uniqueness: %w", err)
		} else if existingID != nil {
			return db.UpdateUserAdminRow{}, &ConflictError{Field: "employee_id", Message: "sudah digunakan"}
		}
	}

	roleRow, err := s.repo.GetRoleByName(ctx, role)
	if err != nil {
		return db.UpdateUserAdminRow{}, fmt.Errorf("resolving role: %w", err)
	}
	if roleRow == nil {
		return db.UpdateUserAdminRow{}, &ReferenceError{Field: "role", Message: "tidak ditemukan"}
	}
	if req.VendorID != nil {
		ok, err := s.repo.VendorExists(ctx, *req.VendorID)
		if err != nil {
			return db.UpdateUserAdminRow{}, fmt.Errorf("checking vendor reference: %w", err)
		}
		if !ok {
			return db.UpdateUserAdminRow{}, &ReferenceError{Field: "vendor_id", Message: "tidak ditemukan"}
		}
	}
	if req.SupervisorID != nil {
		ok, err := s.repo.SupervisorExists(ctx, *req.SupervisorID)
		if err != nil {
			return db.UpdateUserAdminRow{}, fmt.Errorf("checking supervisor reference: %w", err)
		}
		if !ok {
			return db.UpdateUserAdminRow{}, &ReferenceError{Field: "supervisor_id", Message: "tidak ditemukan"}
		}
	}

	updated, err := s.repo.Update(ctx, db.UpdateUserAdminParams{
		ID:            id,
		FullName:      fullName,
		Email:         email,
		RoleID:        roleRow.ID,
		IsKaryawan:    req.IsKaryawan,
		EmployeeID:    nilIfEmpty(employeeID),
		VendorID:      req.VendorID,
		SupervisorID:  req.SupervisorID,
		ApprovalLevel: req.ApprovalLevel,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// The query filters deleted_at IS NULL -- a race where the user
			// was disabled between the pre-check and this UPDATE lands here too.
			return db.UpdateUserAdminRow{}, auth.ErrUserNotFound
		}
		if field, ok := conflictFieldFromPgError(err); ok {
			return db.UpdateUserAdminRow{}, &ConflictError{Field: field, Message: "sudah digunakan"}
		}
		return db.UpdateUserAdminRow{}, fmt.Errorf("updating user: %w", err)
	}

	if err := s.audit.Write(ctx, audit.Entry{
		ActorID:    actorID,
		Action:     "user_updated",
		EntityType: "user",
		EntityID:   id,
		Before:     before,
		After:      updated,
		IP:         actorIP,
	}); err != nil {
		return db.UpdateUserAdminRow{}, fmt.Errorf("write audit log: %w", err)
	}

	return updated, nil
}

// remapPasswordFieldError rewrites a *pkg/auth.ValidationError from
// ValidatePasswordStrength (which hardcodes Field "new_password") to
// "temporary_password", so the User_Form_Dialog can surface the message
// against the right field (Req 11.3a).
func remapPasswordFieldError(err error) error {
	var valErr *auth.ValidationError
	if errors.As(err, &valErr) {
		return &auth.ValidationError{Field: "temporary_password", Message: valErr.Message}
	}
	return err
}

// conflictFieldFromPgError maps a Postgres unique-violation error to the
// offending field via its constraint name (design.md "409 detection"
// fallback for a race the pre-check missed). ok=false means err is not a
// unique violation at all.
func conflictFieldFromPgError(err error) (field string, ok bool) {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != pgUniqueViolation {
		return "", false
	}
	switch pgErr.ConstraintName {
	case "users_username_key":
		return "username", true
	case "users_email_key":
		return "email", true
	case "users_employee_id_key":
		return "employee_id", true
	default:
		return "unknown", true
	}
}

// nilIfEmpty converts an already-trimmed string into a *string for the
// nullable employee_id column, nil meaning "not provided" rather than an
// empty string.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// isValidEmail is a basic format check via the stdlib RFC 5322 parser (Req
// 3.2, 4.1). Good enough for a format guard; it is not a deliverability check.
func isValidEmail(s string) bool {
	_, err := mail.ParseAddress(s)
	return err == nil
}
