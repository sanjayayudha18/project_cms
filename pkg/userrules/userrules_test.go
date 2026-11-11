package userrules

import (
	"errors"
	"testing"

	"github.com/cimb-niaga/cms/pkg/auth"
)

func TestIsPrivileged(t *testing.T) {
	for role, want := range map[string]bool{
		"ADMIN": true, "appaccess": true, " Admin_Param ": true, "EXT-CLIENT": true,
		"ATM-USER": false, "": false, "ADMINS": false,
	} {
		if got := IsPrivileged(role); got != want {
			t.Errorf("IsPrivileged(%q) = %v; want %v", role, got, want)
		}
	}
}

func TestValidateProfile(t *testing.T) {
	tests := []struct {
		name, fullName, email, role string
		wantField, wantMsg          string
	}{
		{"ok", "Budi", "b@x.co", "ATM-USER", "", ""},
		{"empty name wins", "", "", "", "full_name", "wajib diisi"},
		{"empty email", "Budi", "", "", "email", "wajib diisi"},
		{"bad email", "Budi", "nope", "", "email", "format email tidak valid"},
		{"empty role", "Budi", "b@x.co", "", "role", "wajib diisi"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProfile(tt.fullName, tt.email, tt.role)
			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var ve *auth.ValidationError
			if !errors.As(err, &ve) || ve.Field != tt.wantField || ve.Message != tt.wantMsg {
				t.Fatalf("got %v; want %s/%s", err, tt.wantField, tt.wantMsg)
			}
		})
	}
}

func TestIsValidEmail(t *testing.T) {
	for s, want := range map[string]bool{"a@b.co": true, "Nama <a@b.co>": true, "a": false, "": false, "@b.co": false} {
		if got := IsValidEmail(s); got != want {
			t.Errorf("IsValidEmail(%q) = %v; want %v", s, got, want)
		}
	}
}

func TestUniqueness(t *testing.T) {
	id := int64(7)
	boom := errors.New("db down")

	if err := Uniqueness("email", nil, nil); err != nil {
		t.Errorf("free value: %v", err)
	}
	var ce *ConflictError
	if err := Uniqueness("email", &id, nil); !errors.As(err, &ce) || ce.Field != "email" || ce.Message != "sudah digunakan" {
		t.Errorf("taken value: %v", err)
	}
	if err := Uniqueness("email", &id, boom); !errors.Is(err, boom) {
		t.Errorf("lookup error must be wrapped, got %v", err)
	}
}

func TestConflictFieldFromConstraint(t *testing.T) {
	for name, want := range map[string]string{
		"users_username_key": "username", "users_email_key": "email",
		"users_employee_id_key": "employee_id", "other": "unknown", "": "unknown",
	} {
		if got := ConflictFieldFromConstraint(name); got != want {
			t.Errorf("ConflictFieldFromConstraint(%q) = %q; want %q", name, got, want)
		}
	}
}

func TestNilIfEmpty(t *testing.T) {
	if NilIfEmpty("") != nil {
		t.Error("empty must be nil")
	}
	if p := NilIfEmpty("x"); p == nil || *p != "x" {
		t.Error("non-empty must point to value")
	}
}

func TestErrorStrings(t *testing.T) {
	if got := (&ConflictError{Field: "email", Message: "sudah digunakan"}).Error(); got != "email: sudah digunakan" {
		t.Errorf("ConflictError = %q", got)
	}
	if got := (&ReferenceError{Field: "role", Message: "tidak ditemukan"}).Error(); got != "role: tidak ditemukan" {
		t.Errorf("ReferenceError = %q", got)
	}
}
