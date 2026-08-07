package auth

import (
	"errors"
	"fmt"
)

// Sentinel errors for the auth package.
var (
	ErrInvalidCredentials    = errors.New("Username atau password salah")
	ErrAccountInactive       = errors.New("Akun tidak aktif")
	ErrPortalMismatch        = errors.New("Akun tidak memiliki akses ke portal ini")
	ErrUnsupportedAuthSource = errors.New("Auth source tidak didukung")
	ErrLDAPNotConfigured     = errors.New("LDAP authentication tidak tersedia")
	ErrRateLimited           = errors.New("Terlalu banyak percobaan login")
	ErrServiceUnavailable    = errors.New("Layanan sedang tidak tersedia")
	ErrValidation            = errors.New("Validasi gagal")
	ErrTokenExpired          = errors.New("Sesi telah berakhir")
	ErrTokenInvalid          = errors.New("Token tidak valid")
)

// ValidationError represents a structured field-level validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// RateLimitError carries retry information for rate-limited requests.
type RateLimitError struct {
	RetryAfter int // seconds until the client can retry
}

func (e *RateLimitError) Error() string {
	return ErrRateLimited.Error()
}

// Unwrap allows errors.Is(rateLimitErr, ErrRateLimited) to return true.
func (e *RateLimitError) Unwrap() error {
	return ErrRateLimited
}
