package rolemgmt

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// mountVendorChain mounts RequireAuth + mws in front of a handler that records whether it ran.
func mountVendorChain(t *testing.T, mws ...func(http.Handler) http.Handler) (http.Handler, *pkgauth.TokenService, *bool) {
	t.Helper()
	ts := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	called := false
	r := chi.NewRouter()
	r.With(append([]func(http.Handler) http.Handler{custommw.RequireAuth(ts)}, mws...)...).
		Get("/protected", func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		})
	return r, ts, &called
}

// Property 5 / 7 at middleware level (Req 4.4, 4.5, 4.6).
func TestRequireVendorPermission(t *testing.T) {
	tests := []struct {
		name       string
		role       string
		allowed    bool
		err        error
		wantCode   int
		wantCalls  int
		wantCalled bool
	}{
		{"vendor with key -> pass", "VENDOR-USER", true, nil, http.StatusOK, 1, true},
		{"vendor without key -> 403", "VENDOR-VAULT-USER", false, nil, http.StatusForbidden, 1, false},
		{"new vendor role (prefix) without key -> 403", "VENDOR-AUDIT", false, nil, http.StatusForbidden, 1, false},
		{"lowercase vendor prefix still checked", "vendor-user", false, nil, http.StatusForbidden, 1, false},
		{"repo error -> 500, fail closed", "VENDOR-ATM-USER", true, errBoom, http.StatusInternalServerError, 1, false},
		{"internal role passes without query", "ATM-USER", false, nil, http.StatusOK, 0, true},
		{"internal role passes even if repo would error", "ADMIN", false, errBoom, http.StatusOK, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &fakeReplicaReader{allowed: tt.allowed, err: tt.err}
			router, ts, called := mountVendorChain(t, RequireVendorPermission(NewPermissionEvaluator(reader), "vendor-orders"))

			rec := doTestRequest(router, tokenForTestRole(t, ts, tt.role))

			if rec.Code != tt.wantCode {
				t.Fatalf("code = %d, want %d: %s", rec.Code, tt.wantCode, rec.Body.String())
			}
			if reader.calls != tt.wantCalls {
				t.Fatalf("repo calls = %d, want %d", reader.calls, tt.wantCalls)
			}
			if tt.wantCalls > 0 && strings.Join(reader.gotKeys, ",") != "vendor-orders" {
				t.Fatalf("repo keys = %v, want [vendor-orders]", reader.gotKeys)
			}
			if *called != tt.wantCalled {
				t.Fatalf("handler called = %v, want %v", *called, tt.wantCalled)
			}
			if tt.wantCode == http.StatusForbidden && !strings.Contains(rec.Body.String(), `"error":"forbidden"`) {
				t.Fatalf("body = %s, want forbidden", rec.Body.String())
			}
		})
	}
}

// Property 8 (Req 4.7): no cache — a revoked grant is denied on the very next request.
func TestRequireVendorPermission_RevokeTakesEffectNextRequest(t *testing.T) {
	reader := &fakeReplicaReader{allowed: true}
	router, ts, _ := mountVendorChain(t, RequireVendorPermission(NewPermissionEvaluator(reader), "vendor-dsr"))
	token := tokenForTestRole(t, ts, "VENDOR-USER")

	if rec := doTestRequest(router, token); rec.Code != http.StatusOK {
		t.Fatalf("before revoke: code = %d, want 200", rec.Code)
	}
	reader.allowed = false
	if rec := doTestRequest(router, token); rec.Code != http.StatusForbidden {
		t.Fatalf("after revoke: code = %d, want 403", rec.Code)
	}
	if reader.calls != 2 {
		t.Fatalf("repo calls = %d, want 2 (no cache)", reader.calls)
	}
}

func TestRequireVendorPermission_NoToken_Unauthorized(t *testing.T) {
	reader := &fakeReplicaReader{allowed: true}
	router, _, called := mountVendorChain(t, RequireVendorPermission(NewPermissionEvaluator(reader), "vendor-dsr"))
	if rec := doTestRequest(router, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", rec.Code)
	}
	if reader.calls != 0 || *called {
		t.Fatal("no query and no handler call expected without a token")
	}
}

func TestRequireVendorPermission_EmptyKey_Panics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for empty key")
		}
	}()
	RequireVendorPermission(NewPermissionEvaluator(&fakeReplicaReader{}), "")
}

func TestRequireVendorRole(t *testing.T) {
	tests := []struct {
		role     string
		wantCode int
	}{
		{"VENDOR-USER", http.StatusOK},
		{"VENDOR-ATM-USER", http.StatusOK},
		{"VENDOR-NEW-ROLE", http.StatusOK},
		{"ATM-USER", http.StatusForbidden},
		{"ADMIN", http.StatusForbidden},
		{"EXT-CLIENT", http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			router, ts, called := mountVendorChain(t, RequireVendorRole())
			rec := doTestRequest(router, tokenForTestRole(t, ts, tt.role))
			if rec.Code != tt.wantCode {
				t.Fatalf("code = %d, want %d: %s", rec.Code, tt.wantCode, rec.Body.String())
			}
			if *called != (tt.wantCode == http.StatusOK) {
				t.Fatalf("handler called = %v", *called)
			}
		})
	}
}

func TestRequireVendorRole_NoToken_Unauthorized(t *testing.T) {
	router, _, _ := mountVendorChain(t, RequireVendorRole())
	if rec := doTestRequest(router, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", rec.Code)
	}
}

func TestVendorAlwaysOnKeys(t *testing.T) {
	if strings.Join(VendorAlwaysOnKeys, ",") != "vendor-notifications" {
		t.Fatalf("VendorAlwaysOnKeys = %v", VendorAlwaysOnKeys)
	}
}
