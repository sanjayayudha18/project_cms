package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cimb-niaga/cms/pkg/auth"
	"pgregory.net/rapid"
)

// validRoles are the 9 DB roles defined in the system.
var validRoles = []string{
	"ADMIN",
	"ADMIN_PARAM",
	"ATM-USER",
	"ATM-SPV",
	"BRANCH-USER",
	"BRANCH-SPV",
	"BRANCH-ATM-USER",
	"BRANCH-ATM-SPV",
	"VENDOR-USER",
}

// noopBlacklist implements auth.TokenBlacklist with no blacklisting (always allows).
type noopBlacklist struct{}

func (n *noopBlacklist) Add(_ context.Context, _ string, _ time.Duration) error { return nil }
func (n *noopBlacklist) IsBlacklisted(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// newTestTokenService creates a real TokenService with a known secret for property tests.
func newTestTokenService() *auth.TokenService {
	secret := []byte("test-secret-key-that-is-32-bytes!")
	config := auth.TokenConfig{
		SecretKey:          secret,
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}
	return auth.NewTokenService(config, &noopBlacklist{})
}

// roleGen generates a random role from the 9 valid roles.
func roleGen() *rapid.Generator[string] {
	return rapid.SampledFrom(validRoles)
}

// roleListGen generates a list of 0-9 unique roles (subset of valid roles).
func roleListGen() *rapid.Generator[[]string] {
	return rapid.Custom(func(t *rapid.T) []string {
		count := rapid.IntRange(0, len(validRoles)).Draw(t, "roleListCount")
		if count == 0 {
			return []string{}
		}
		// Shuffle and take first `count` roles to get unique subset
		perm := rapid.Permutation(validRoles).Draw(t, "rolePerm")
		return perm[:count]
	})
}

// identityGen generates a random AuthIdentity with valid role.
func identityGen() *rapid.Generator[*auth.AuthIdentity] {
	return rapid.Custom(func(t *rapid.T) *auth.AuthIdentity {
		role := roleGen().Draw(t, "role")
		userID := rapid.Int64Range(1, 999999).Draw(t, "userID")
		username := rapid.StringMatching(`[a-z]{3,15}\.[a-z]{3,10}`).Draw(t, "username")
		isKaryawan := rapid.Bool().Draw(t, "isKaryawan")

		var vendorID *int64
		if !isKaryawan {
			v := rapid.Int64Range(1, 1000).Draw(t, "vendorID")
			vendorID = &v
		}

		return &auth.AuthIdentity{
			UserID:     userID,
			Username:   username,
			Role:       role,
			IsKaryawan: isKaryawan,
			VendorID:   vendorID,
		}
	})
}

// containsRole checks if a string is in a slice (case-sensitive match).
func containsRole(roles []string, target string) bool {
	for _, r := range roles {
		if r == target {
			return true
		}
	}
	return false
}

// Feature: user-login, Property 6: RBAC Role Authorization
// **Validates: Requirements 5.3, 5.6**
//
// For any valid Access_Token with role R and any endpoint with allowed roles list L,
// the RBAC_Middleware SHALL grant access if R is in L or L is empty, and SHALL return
// 403 Forbidden if R is not in L and L is non-empty.
func TestProperty_RBACRoleAuthorization(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		tokenService := newTestTokenService()
		identity := identityGen().Draw(t, "identity")
		allowedRoles := roleListGen().Draw(t, "allowedRoles")

		// Generate a real access token
		accessToken, _, err := tokenService.GenerateTokenPair(identity)
		if err != nil {
			t.Fatalf("failed to generate token pair: %v", err)
		}

		// Build middleware chain: RequireAuth -> RequireRoles -> handler
		finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := RequireAuth(tokenService)(RequireRoles(allowedRoles...)(finalHandler))

		// Create request with Bearer token
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Determine expected behavior
		shouldAllow := len(allowedRoles) == 0 || containsRole(allowedRoles, identity.Role)

		if shouldAllow {
			if rec.Code != http.StatusOK {
				t.Fatalf("role=%q, allowedRoles=%v: expected 200, got %d", identity.Role, allowedRoles, rec.Code)
			}
		} else {
			if rec.Code != http.StatusForbidden {
				t.Fatalf("role=%q, allowedRoles=%v: expected 403, got %d", identity.Role, allowedRoles, rec.Code)
			}
		}
	})
}

// Feature: user-login, Property 7: Auth Context Injection
// **Validates: Requirements 5.7**
//
// For any valid Access_Token containing claims (id, username, role, is_karyawan, vendor_id),
// after RequireAuth validation the injected AuthContext SHALL contain values identical to
// the token claims.
func TestProperty_AuthContextInjection(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		tokenService := newTestTokenService()
		identity := identityGen().Draw(t, "identity")

		// Generate a real access token
		accessToken, _, err := tokenService.GenerateTokenPair(identity)
		if err != nil {
			t.Fatalf("failed to generate token pair: %v", err)
		}

		// Handler that captures the AuthContext
		var capturedCtx *AuthContext
		captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, ok := GetAuthContext(r.Context())
			if !ok {
				t.Fatal("AuthContext not found in request context")
			}
			capturedCtx = ctx
			w.WriteHeader(http.StatusOK)
		})

		handler := RequireAuth(tokenService)(captureHandler)

		// Create request with Bearer token
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		// Verify injected AuthContext matches original identity
		if capturedCtx == nil {
			t.Fatal("AuthContext was not captured")
		}

		if capturedCtx.UserID != identity.UserID {
			t.Fatalf("UserID mismatch: got %d, want %d", capturedCtx.UserID, identity.UserID)
		}
		if capturedCtx.Username != identity.Username {
			t.Fatalf("Username mismatch: got %q, want %q", capturedCtx.Username, identity.Username)
		}
		if capturedCtx.Role != identity.Role {
			t.Fatalf("Role mismatch: got %q, want %q", capturedCtx.Role, identity.Role)
		}
		if capturedCtx.IsKaryawan != identity.IsKaryawan {
			t.Fatalf("IsKaryawan mismatch: got %v, want %v", capturedCtx.IsKaryawan, identity.IsKaryawan)
		}

		// Compare VendorID (both may be nil)
		if identity.VendorID == nil {
			if capturedCtx.VendorID != nil {
				t.Fatalf("VendorID mismatch: got %v, want nil", *capturedCtx.VendorID)
			}
		} else {
			if capturedCtx.VendorID == nil {
				t.Fatalf("VendorID mismatch: got nil, want %d", *identity.VendorID)
			}
			if *capturedCtx.VendorID != *identity.VendorID {
				t.Fatalf("VendorID mismatch: got %d, want %d", *capturedCtx.VendorID, *identity.VendorID)
			}
		}
	})
}
