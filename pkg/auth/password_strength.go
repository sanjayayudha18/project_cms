package auth

import (
	"fmt"
	"unicode"
)

// Local-password strength policy (auth_source=local|local_dev only; used by
// self-service change-password and APPACCESS set-initial-password).
// See .kiro/specs/Auth-Local-Lifecycle/task.md Task 5.
const (
	MinPasswordLength = 8
	MaxPasswordLength = 255 // matches the existing login password field limit
)

// ValidatePasswordStrength checks a candidate new password against the local
// policy: length bounds, at least one letter and one digit. Returns a
// *ValidationError (field "new_password") on failure, nil if acceptable.
func ValidatePasswordStrength(password string) error {
	if len(password) < MinPasswordLength {
		return &ValidationError{Field: "new_password", Message: fmt.Sprintf("minimal %d karakter", MinPasswordLength)}
	}
	if len(password) > MaxPasswordLength {
		return &ValidationError{Field: "new_password", Message: fmt.Sprintf("maksimal %d karakter", MaxPasswordLength)}
	}

	hasLetter := false
	hasDigit := false
	for _, r := range password {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return &ValidationError{Field: "new_password", Message: "harus mengandung huruf dan angka"}
	}

	return nil
}
