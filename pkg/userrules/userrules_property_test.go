package userrules

import (
	"net/mail"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/pkg/auth"
)

// Golden copies of the functions as they were in backend/internal/auth/user_admin.go
// before the move to pkg/userrules. Do not "fix" them: they pin existing behaviour.

func oldValidateProfile(fullName, email, role string) error {
	switch {
	case fullName == "":
		return &auth.ValidationError{Field: "full_name", Message: "wajib diisi"}
	case email == "":
		return &auth.ValidationError{Field: "email", Message: "wajib diisi"}
	case !oldIsValidEmail(email):
		return &auth.ValidationError{Field: "email", Message: "format email tidak valid"}
	case role == "":
		return &auth.ValidationError{Field: "role", Message: "wajib diisi"}
	}
	return nil
}

func oldIsValidEmail(s string) bool {
	_, err := mail.ParseAddress(s)
	return err == nil
}

func oldConflictField(name string) string {
	switch name {
	case "users_username_key":
		return "username"
	case "users_email_key":
		return "email"
	case "users_employee_id_key":
		return "employee_id"
	default:
		return "unknown"
	}
}

func sameErr(a, b error) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Error() == b.Error()
}

// Property 11: shared rules give the same result as the pre-refactor admin code.
func TestProperty11SameAsOldAdminRules(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.OneOf(rapid.Just(""), rapid.String()).Draw(t, "name")
		email := rapid.OneOf(rapid.Just(""), rapid.Just("a@b.co"), rapid.String()).Draw(t, "email")
		role := rapid.OneOf(rapid.Just(""), rapid.String()).Draw(t, "role")
		if got, want := ValidateProfile(name, email, role), oldValidateProfile(name, email, role); !sameErr(got, want) {
			t.Fatalf("ValidateProfile(%q,%q,%q) = %v; old = %v", name, email, role, got, want)
		}

		constraint := rapid.OneOf(
			rapid.SampledFrom([]string{"users_username_key", "users_email_key", "users_employee_id_key"}),
			rapid.String(),
		).Draw(t, "constraint")
		if got, want := ConflictFieldFromConstraint(constraint), oldConflictField(constraint); got != want {
			t.Fatalf("ConflictFieldFromConstraint(%q) = %q; old = %q", constraint, got, want)
		}
	})
}

// Privileged check ignores case and surrounding space for any variation of a listed role.
func TestIsPrivilegedCaseAndSpaceInsensitive(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		role := rapid.SampledFrom(PrivilegedRoles).Draw(t, "role")
		pad := rapid.StringMatching(`[ \t]{0,3}`).Draw(t, "pad")
		if !IsPrivileged(pad + strings.ToLower(role) + pad) {
			t.Fatalf("%q must be privileged", role)
		}
	})
}
