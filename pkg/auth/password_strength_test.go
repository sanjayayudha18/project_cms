package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		wantValid bool
	}{
		{name: "too short (7 chars)", password: "Abcdef1", wantValid: false},
		{name: "exactly 8 chars, letter+digit: valid", password: "Abcdef12", wantValid: true},
		{name: "letters only, no digit", password: "Abcdefgh", wantValid: false},
		{name: "digits only, no letter", password: "12345678", wantValid: false},
		{name: "valid mixed", password: "Password123", wantValid: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePasswordStrength(tc.password)
			if tc.wantValid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
			if !tc.wantValid && err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}

func TestValidatePasswordStrength_LengthBoundaries(t *testing.T) {
	exactly255 := strings.Repeat("a", 253) + "A1" // 255 chars, letter+digit present
	if len(exactly255) != 255 {
		t.Fatalf("test setup bug: exactly255 has len %d, want 255", len(exactly255))
	}
	if err := ValidatePasswordStrength(exactly255); err != nil {
		t.Errorf("expected 255-char password to be valid, got: %v", err)
	}

	over255 := exactly255 + "1"
	err := ValidatePasswordStrength(over255)
	if err == nil {
		t.Fatal("expected 256-char password to be rejected")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected *ValidationError, got: %T", err)
	}
	if validationErr.Field != "new_password" {
		t.Errorf("expected Field=new_password, got %s", validationErr.Field)
	}
}
