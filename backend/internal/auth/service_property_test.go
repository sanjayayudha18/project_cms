package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"pgregory.net/rapid"

	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
)

// Feature: user-login
// Property 2: Auth Error Uniformity
// Property 14: Portal Type Isolation
// Property 15: Auth pkgauth.Provider Selection
// Property 16: Input Validation Priority
// Property 17: Password Length Boundaries

// --- Generators ---

// nonWSRune returns a generator for non-whitespace printable ASCII runes (0x21–0x7E).
func nonWSRune() *rapid.Generator[rune] {
	return rapid.Map(rapid.Int32Range(0x21, 0x7E), func(i int32) rune {
		return rune(i)
	})
}

// printableRune returns a generator for printable ASCII runes (0x20–0x7E).
func printableRune() *rapid.Generator[rune] {
	return rapid.Map(rapid.Int32Range(0x20, 0x7E), func(i int32) rune {
		return rune(i)
	})
}

// genNonWhitespaceString generates a string with at least one non-whitespace char
// within the specified length range.
func genNonWhitespaceString(minLen, maxLen int) *rapid.Generator[string] {
	return rapid.Custom[string](func(t *rapid.T) string {
		length := rapid.IntRange(minLen, maxLen).Draw(t, "len")
		chars := make([]rune, length)
		// First char is always non-whitespace
		chars[0] = nonWSRune().Draw(t, "firstChar")
		for i := 1; i < length; i++ {
			chars[i] = printableRune().Draw(t, "char")
		}
		return string(chars)
	})
}

// genValidPortalType generates "company" or "vendor".
func genValidPortalType() *rapid.Generator[string] {
	return rapid.SampledFrom([]string{"company", "vendor"})
}

// genWhitespaceString generates a string composed entirely of whitespace characters.
func genWhitespaceString() *rapid.Generator[string] {
	return rapid.Custom[string](func(t *rapid.T) string {
		length := rapid.IntRange(0, 10).Draw(t, "wsLen")
		ws := []rune{' ', '\t', '\n', '\r'}
		chars := make([]rune, length)
		for i := 0; i < length; i++ {
			chars[i] = rapid.SampledFrom(ws).Draw(t, "wsChar")
		}
		return string(chars)
	})
}

// --- Property 2: Auth Error Uniformity ---
// **Validates: Requirements 3.3, 3.4, 3.6**
//
// For any authentication failure — whether caused by a non-existent username
// (nil repo result) or an incorrect password (provider returning pkgauth.ErrInvalidCredentials)
// — the Auth_Service SHALL return pkgauth.ErrInvalidCredentials.
func TestProperty_Service_AuthErrorUniformity(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		username := genNonWhitespaceString(1, 50).Draw(t, "username")
		password := genNonWhitespaceString(1, 50).Draw(t, "password")
		portalType := genValidPortalType().Draw(t, "portalType")

		// Sub-case selection: 0 = user not found, 1 = wrong password
		failureMode := rapid.IntRange(0, 1).Draw(t, "failureMode")

		var repo *stubUserRepo
		var provider *stubProvider
		rl := &stubRateLimiter{}

		switch failureMode {
		case 0:
			// User not found: repo returns nil, nil
			repo = &stubUserRepo{findResult: nil, findErr: nil}
			provider = &stubProvider{}
		case 1:
			// Wrong password: user exists, active, provider returns pkgauth.ErrInvalidCredentials
			isKaryawan := rapid.Bool().Draw(t, "isKaryawan")
			user := &pkgauth.UserRecord{
				ID:           rapid.Int64Range(1, 100000).Draw(t, "userID"),
				Username:     username,
				FullName:     "Test User",
				Email:        "test@example.com",
				PasswordHash: strPtr("$2a$12$hashed"),
				AuthSource:   "local_dev",
				RoleID:       1,
				Role:         "ADMIN",
				IsKaryawan:   isKaryawan,
				IsActive:     true,
				DeletedAt:    nil,
			}
			// Ensure portal type matches user type so we reach credential check
			if isKaryawan {
				portalType = "company"
			} else {
				user.VendorID = int64Ptr(rapid.Int64Range(1, 1000).Draw(t, "vendorID"))
				portalType = "vendor"
			}
			repo = &stubUserRepo{findResult: user}
			provider = &stubProvider{authenticateErr: pkgauth.ErrInvalidCredentials}
		}

		svc := newServiceUnderTest(provider, repo, rl)

		_, _, err := svc.Login(context.Background(), LoginRequest{
			Username:   username,
			Password:   password,
			PortalType: portalType,
			IP:         "10.0.0.1",
		})

		// Both failure modes must produce pkgauth.ErrInvalidCredentials
		if !errors.Is(err, pkgauth.ErrInvalidCredentials) {
			t.Fatalf("failureMode=%d: expected pkgauth.ErrInvalidCredentials, got: %v", failureMode, err)
		}
	})
}

// --- Property 14: Portal Type Isolation ---
// **Validates: Requirements 12.4, 12.5, 12.10**
//
// For any user where is_karyawan=true + portal_type='vendor', or
// is_karyawan=false + portal_type='company', returns pkgauth.ErrPortalMismatch
// (but only after credential verification succeeds).
func TestProperty_Service_PortalTypeIsolation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Pick a mismatch scenario
		isKaryawan := rapid.Bool().Draw(t, "isKaryawan")
		var portalType string
		if isKaryawan {
			portalType = "vendor" // mismatch: karyawan + vendor portal
		} else {
			portalType = "company" // mismatch: non-karyawan + company portal
		}

		username := genNonWhitespaceString(1, 50).Draw(t, "username")
		password := genNonWhitespaceString(1, 50).Draw(t, "password")

		user := &pkgauth.UserRecord{
			ID:           rapid.Int64Range(1, 100000).Draw(t, "userID"),
			Username:     username,
			FullName:     "Test User",
			Email:        "test@example.com",
			PasswordHash: strPtr("$2a$12$hashed"),
			AuthSource:   "local_dev",
			RoleID:       1,
			Role:         "ADMIN",
			IsKaryawan:   isKaryawan,
			IsActive:     true,
			DeletedAt:    nil,
		}
		if !isKaryawan {
			user.VendorID = int64Ptr(rapid.Int64Range(1, 1000).Draw(t, "vendorID"))
		}

		// pkgauth.Provider succeeds (credentials are valid)
		provider := &stubProvider{
			authenticateID: &pkgauth.AuthIdentity{
				UserID:     user.ID,
				Username:   user.Username,
				Role:       user.Role,
				IsKaryawan: user.IsKaryawan,
				VendorID:   user.VendorID,
			},
		}
		repo := &stubUserRepo{findResult: user}
		rl := &stubRateLimiter{}

		svc := newServiceUnderTest(provider, repo, rl)

		_, _, err := svc.Login(context.Background(), LoginRequest{
			Username:   username,
			Password:   password,
			PortalType: portalType,
			IP:         "10.0.0.1",
		})

		if !errors.Is(err, pkgauth.ErrPortalMismatch) {
			t.Fatalf("isKaryawan=%v, portalType=%s: expected pkgauth.ErrPortalMismatch, got: %v",
				isKaryawan, portalType, err)
		}
	})
}

// --- Property 15: Auth pkgauth.Provider Selection ---
// **Validates: Requirements 2.3, 2.4, 2.6**
//
// auth_source 'local'/'local_dev' → LocalProvider selected.
// 'ldap' without LDAP provider → pkgauth.ErrLDAPNotConfigured.
// Unknown → pkgauth.ErrUnsupportedAuthSource.
func TestProperty_Service_AuthProviderSelection(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Choose auth_source scenario: 0=local, 1=local_dev, 2=ldap, 3=unknown
		scenario := rapid.IntRange(0, 3).Draw(t, "scenario")

		var authSource string
		switch scenario {
		case 0:
			authSource = "local"
		case 1:
			authSource = "local_dev"
		case 2:
			authSource = "ldap"
		case 3:
			// Generate a random unrecognized auth_source
			authSource = rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "unknownSource")
			// Ensure it's not one of the known values
			for authSource == "local" || authSource == "local_dev" || authSource == "ldap" {
				authSource = authSource + "x"
			}
		}

		username := genNonWhitespaceString(1, 50).Draw(t, "username")
		password := genNonWhitespaceString(1, 50).Draw(t, "password")

		user := &pkgauth.UserRecord{
			ID:           rapid.Int64Range(1, 100000).Draw(t, "userID"),
			Username:     username,
			FullName:     "Test User",
			Email:        "test@example.com",
			PasswordHash: strPtr("$2a$12$hashed"),
			AuthSource:   authSource,
			RoleID:       1,
			Role:         "ADMIN",
			IsKaryawan:   true,
			IsActive:     true,
			DeletedAt:    nil,
		}

		// LocalProvider only supports "local" and "local_dev"
		provider := &stubProvider{
			supportsFunc: func(source string) bool {
				return source == "local" || source == "local_dev"
			},
			authenticateID: &pkgauth.AuthIdentity{
				UserID:     user.ID,
				Username:   user.Username,
				Role:       user.Role,
				IsKaryawan: user.IsKaryawan,
				VendorID:   user.VendorID,
			},
		}
		repo := &stubUserRepo{findResult: user}
		rl := &stubRateLimiter{}

		svc := newServiceUnderTest(provider, repo, rl)

		_, _, err := svc.Login(context.Background(), LoginRequest{
			Username:   username,
			Password:   password,
			PortalType: "company",
			IP:         "10.0.0.1",
		})

		switch scenario {
		case 0, 1:
			// local/local_dev: provider selected successfully, no provider-selection error
			if errors.Is(err, pkgauth.ErrLDAPNotConfigured) {
				t.Fatalf("auth_source=%q: should not get pkgauth.ErrLDAPNotConfigured", authSource)
			}
			if errors.Is(err, pkgauth.ErrUnsupportedAuthSource) {
				t.Fatalf("auth_source=%q: should not get pkgauth.ErrUnsupportedAuthSource", authSource)
			}
		case 2:
			// ldap without LDAP provider → pkgauth.ErrLDAPNotConfigured
			if !errors.Is(err, pkgauth.ErrLDAPNotConfigured) {
				t.Fatalf("auth_source=ldap: expected pkgauth.ErrLDAPNotConfigured, got: %v", err)
			}
		case 3:
			// unknown → pkgauth.ErrUnsupportedAuthSource
			if !errors.Is(err, pkgauth.ErrUnsupportedAuthSource) {
				t.Fatalf("auth_source=%q: expected pkgauth.ErrUnsupportedAuthSource, got: %v", authSource, err)
			}
		}
	})
}

// --- Property 16: Input Validation Priority ---
// **Validates: Requirements 3.7, 3.8**
//
// When username or password is whitespace-only AND credentials would be invalid,
// validation error is returned first.
func TestProperty_Service_InputValidationPriority(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Pick which field to make invalid: 0=username whitespace, 1=password whitespace
		invalidField := rapid.IntRange(0, 1).Draw(t, "invalidField")

		var username, password string
		switch invalidField {
		case 0:
			username = genWhitespaceString().Draw(t, "wsUsername")
			password = genNonWhitespaceString(1, 50).Draw(t, "password")
		case 1:
			username = genNonWhitespaceString(1, 50).Draw(t, "username")
			password = genWhitespaceString().Draw(t, "wsPassword")
		}

		portalType := genValidPortalType().Draw(t, "portalType")

		// Set up stubs that would fail auth if reached (proves validation runs first)
		provider := &stubProvider{
			authenticateErr: pkgauth.ErrInvalidCredentials,
		}
		repo := &stubUserRepo{findResult: nil, findErr: nil}
		rl := &stubRateLimiter{}

		svc := newServiceUnderTest(provider, repo, rl)

		_, _, err := svc.Login(context.Background(), LoginRequest{
			Username:   username,
			Password:   password,
			PortalType: portalType,
			IP:         "10.0.0.1",
		})

		// Must get a pkgauth.ValidationError, not pkgauth.ErrInvalidCredentials
		var validationErr *pkgauth.ValidationError
		if !errors.As(err, &validationErr) {
			t.Fatalf("invalidField=%d: expected *pkgauth.ValidationError, got: %T (%v)", invalidField, err, err)
		}

		switch invalidField {
		case 0:
			if validationErr.Field != "username" {
				t.Fatalf("expected Field=username, got %s", validationErr.Field)
			}
		case 1:
			if validationErr.Field != "password" {
				t.Fatalf("expected Field=password, got %s", validationErr.Field)
			}
		}
	})
}

// --- Property 17: Password Length Boundaries ---
// **Validates: Requirements 6.5**
//
// The service validates max 255 chars for password field.
// Passwords exceeding 255 chars are rejected with validation error.
func TestProperty_Service_PasswordLengthBoundaries(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate password just over 255 chars
		extraLen := rapid.IntRange(1, 100).Draw(t, "extraLen")
		password := strings.Repeat("A", 255+extraLen)

		username := genNonWhitespaceString(1, 50).Draw(t, "username")
		portalType := genValidPortalType().Draw(t, "portalType")

		// Stubs that would fail auth if reached
		provider := &stubProvider{authenticateErr: pkgauth.ErrInvalidCredentials}
		repo := &stubUserRepo{findResult: nil}
		rl := &stubRateLimiter{}

		svc := newServiceUnderTest(provider, repo, rl)

		_, _, err := svc.Login(context.Background(), LoginRequest{
			Username:   username,
			Password:   password,
			PortalType: portalType,
			IP:         "10.0.0.1",
		})

		// Must get a pkgauth.ValidationError for password length
		var validationErr *pkgauth.ValidationError
		if !errors.As(err, &validationErr) {
			t.Fatalf("password len=%d: expected *pkgauth.ValidationError, got: %T (%v)", len(password), err, err)
		}
		if validationErr.Field != "password" {
			t.Fatalf("expected Field=password, got %s", validationErr.Field)
		}
	})
}

// --- Helper ---

func int64Ptr(v int64) *int64 { return &v }
