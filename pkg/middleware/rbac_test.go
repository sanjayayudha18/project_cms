package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cimb-niaga/cms/pkg/auth"
	"github.com/golang-jwt/jwt/v5"
)

// testSecretUnit is the same secret used in newTestTokenService() from rbac_property_test.go.
var testSecretUnit = []byte("test-secret-key-that-is-32-bytes!")

// generateValidToken creates a valid access token for unit testing.
func generateValidToken(t *testing.T, role string) string {
	t.Helper()
	svc := newTestTokenService()
	identity := &auth.AuthIdentity{
		UserID:     1,
		Username:   "testuser",
		Role:       role,
		IsKaryawan: true,
		VendorID:   nil,
	}
	accessToken, _, err := svc.GenerateTokenPair(identity)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	return accessToken
}

// signWithKey signs claims with the given key directly.
func signWithKey(t *testing.T, claims jwt.Claims, key []byte) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

// dummyHandler is a simple handler that writes 200 OK.
func dummyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
}

func TestRBAC_ValidTokenPasses(t *testing.T) {
	svc := newTestTokenService()
	token := generateValidToken(t, "ADMIN")

	handler := RequireAuth(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := GetAuthContext(r.Context())
		if !ok {
			t.Error("expected AuthContext to be present in request context")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if authCtx.UserID != 1 {
			t.Errorf("expected UserID=1, got %d", authCtx.UserID)
		}
		if authCtx.Username != "testuser" {
			t.Errorf("expected Username=testuser, got %s", authCtx.Username)
		}
		if authCtx.Role != "ADMIN" {
			t.Errorf("expected Role=ADMIN, got %s", authCtx.Role)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestRBAC_MissingAuthorizationHeader_401(t *testing.T) {
	svc := newTestTokenService()
	handler := RequireAuth(svc)(dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	// No Authorization header set
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}

	// Verify JSON error response
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("expected JSON response, got decode error: %v", err)
	}
	if body["error"] != "token_invalid" {
		t.Errorf("expected error=token_invalid, got %s", body["error"])
	}
}

func TestRBAC_ExpiredToken_401(t *testing.T) {
	svc := newTestTokenService()

	// Create a token that expired 1 hour ago
	now := time.Now().Add(-2 * time.Hour)
	claims := auth.AccessTokenClaims{
		UserID:     1,
		Username:   "testuser",
		Role:       "ADMIN",
		IsKaryawan: true,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
		},
	}
	expiredToken := signWithKey(t, claims, testSecretUnit)

	handler := RequireAuth(svc)(dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+expiredToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("expected JSON response, got decode error: %v", err)
	}
	if body["error"] != "token_invalid" {
		t.Errorf("expected error=token_invalid, got %s", body["error"])
	}
}

func TestRBAC_InvalidSignature_401(t *testing.T) {
	svc := newTestTokenService()

	// Sign with a different key (simulating invalid signature)
	wrongKey := []byte("wrong-key-that-is-32-bytes-long!")
	claims := auth.AccessTokenClaims{
		UserID:     1,
		Username:   "testuser",
		Role:       "ADMIN",
		IsKaryawan: true,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}
	badToken := signWithKey(t, claims, wrongKey)

	handler := RequireAuth(svc)(dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+badToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("expected JSON response, got decode error: %v", err)
	}
	if body["error"] != "token_invalid" {
		t.Errorf("expected error=token_invalid, got %s", body["error"])
	}
}

func TestRBAC_RoleMismatch_403(t *testing.T) {
	svc := newTestTokenService()
	token := generateValidToken(t, "VENDOR-USER")

	// RequireAuth → RequireRoles(ADMIN only)
	handler := RequireAuth(svc)(RequireRoles("ADMIN")(dummyHandler()))

	req := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("expected JSON response, got decode error: %v", err)
	}
	if body["error"] != "forbidden" {
		t.Errorf("expected error=forbidden, got %s", body["error"])
	}
}

func TestRBAC_EmptyAllowedRoles_AnyAuthenticatedPasses(t *testing.T) {
	svc := newTestTokenService()
	token := generateValidToken(t, "BRANCH-USER")

	// RequireAuth → RequireRoles() with empty roles
	handler := RequireAuth(svc)(RequireRoles()(dummyHandler()))

	req := httptest.NewRequest(http.MethodGet, "/open-to-all-authed", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestRBAC_All9DBRolesRecognized(t *testing.T) {
	svc := newTestTokenService()

	allRoles := []string{
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

	for _, role := range allRoles {
		t.Run(role, func(t *testing.T) {
			token := generateValidToken(t, role)

			// Allow all 9 roles on this endpoint
			handler := RequireAuth(svc)(RequireRoles(allRoles...)(dummyHandler()))

			req := httptest.NewRequest(http.MethodGet, "/multi-role", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("role %s: expected status 200, got %d", role, rec.Code)
			}
		})
	}
}
