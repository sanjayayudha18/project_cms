// Package userrules holds the user-validation rules shared by the admin API
// (backend) and the external user API (backend-extapi), so both reject the
// same input with the same field and message. Pure code: no pgx, no sqlc.
package userrules

import (
	"fmt"
	"net/mail"
	"strings"

	"github.com/cimb-niaga/cms/pkg/auth"
)

const (
	AuthSourceLDAP    = "ldap"
	AuthSourceService = "service"
	ServiceRole       = "EXT-CLIENT"
	// PgUniqueViolation is the Postgres error code for a unique-constraint violation.
	PgUniqueViolation = "23505"
)

// PrivilegedRoles are never assignable or touchable through the external API.
// Migration 039 mirrors this list in a CHECK on api_clients.assignable_roles.
var PrivilegedRoles = []string{"ADMIN", "APPACCESS", "ADMIN_PARAM", ServiceRole}

// ConflictError is a 409 uniqueness conflict on one field.
type ConflictError struct {
	Field   string
	Message string
}

func (e *ConflictError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }

// ReferenceError is a 400 "invalid_reference": a role/vendor/supervisor that
// does not resolve to an acceptable record.
type ReferenceError struct {
	Field   string
	Message string
}

func (e *ReferenceError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }

// IsPrivileged reports whether role is a privileged role (trim + case-insensitive).
func IsPrivileged(role string) bool {
	role = strings.TrimSpace(role)
	for _, p := range PrivilegedRoles {
		if strings.EqualFold(role, p) {
			return true
		}
	}
	return false
}

// ValidateProfile checks the fields create and update share, in the order the
// form shows them (first failing field wins). Inputs must already be trimmed.
func ValidateProfile(fullName, email, role string) error {
	switch {
	case fullName == "":
		return &auth.ValidationError{Field: "full_name", Message: "wajib diisi"}
	case email == "":
		return &auth.ValidationError{Field: "email", Message: "wajib diisi"}
	case !IsValidEmail(email):
		return &auth.ValidationError{Field: "email", Message: "format email tidak valid"}
	case role == "":
		return &auth.ValidationError{Field: "role", Message: "wajib diisi"}
	}
	return nil
}

// IsValidEmail is a format check via the stdlib RFC 5322 parser, not a deliverability check.
func IsValidEmail(s string) bool {
	_, err := mail.ParseAddress(s)
	return err == nil
}

// Uniqueness turns a FindByXxx lookup result into a ConflictError when another
// user already holds the value.
func Uniqueness(field string, existingID *int64, err error) error {
	if err != nil {
		return fmt.Errorf("checking %s uniqueness: %w", field, err)
	}
	if existingID != nil {
		return &ConflictError{Field: field, Message: "sudah digunakan"}
	}
	return nil
}

// ConflictFieldFromConstraint maps a users unique-constraint name to its field;
// any other constraint maps to "unknown".
func ConflictFieldFromConstraint(name string) string {
	switch name {
	case "users_username_key":
		return "username"
	case "users_email_key":
		return "email"
	case "users_employee_id_key":
		return "employee_id"
	}
	return "unknown"
}

// NilIfEmpty converts an already-trimmed string to nil when empty (nullable column).
func NilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
