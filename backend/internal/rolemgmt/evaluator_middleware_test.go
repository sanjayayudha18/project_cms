package rolemgmt

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// fakeReplicaReader implements ReplicaReader for httptest, no real DB
// needed. HasPermission's own decision logic is covered by
// evaluator_integration_test.go (Property 7); this only exercises the
// RequirePermission middleware's HTTP wiring (401/403/500/pass-through).
type fakeReplicaReader struct {
	allowed bool
	err     error
	calls   int
	gotKeys []string
}

func (f *fakeReplicaReader) HasAnyPermission(_ context.Context, _ string, keys []string) (bool, error) {
	f.calls++
	f.gotKeys = keys
	return f.allowed, f.err
}

type noopBlacklist struct{}

func (noopBlacklist) Add(context.Context, string, time.Duration) error { return nil }
func (noopBlacklist) IsBlacklisted(context.Context, string) (bool, error) {
	return false, nil
}

func mountRequirePermission(reader ReplicaReader) (http.Handler, *pkgauth.TokenService) {
	return mountRequirePermissionKeys(reader, "monitoring.audit-logs")
}

func mountRequirePermissionKeys(reader ReplicaReader, keys ...string) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})

	evaluator := NewPermissionEvaluator(reader)
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		RequirePermission(evaluator, keys...),
	).Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return r, tokenSvc
}

func tokenForTestRole(t *testing.T, ts *pkgauth.TokenService, role string) string {
	t.Helper()
	access, _, err := ts.GenerateTokenPair(&pkgauth.AuthIdentity{
		UserID:   1,
		Username: "user",
		Role:     role,
	})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return access
}

func doTestRequest(router http.Handler, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestRequirePermission_NoToken_Unauthorized(t *testing.T) {
	router, _ := mountRequirePermission(&fakeReplicaReader{allowed: true})
	rec := doTestRequest(router, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequirePermission_NotMapped_Forbidden(t *testing.T) {
	router, ts := mountRequirePermission(&fakeReplicaReader{allowed: false})
	token := tokenForTestRole(t, ts, "ATM-USER")

	rec := doTestRequest(router, token)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), `"error":"forbidden"`) {
		t.Errorf("expected forbidden error code in body, got: %s", body)
	}
}

func TestRequirePermission_EvaluatorError_InternalError(t *testing.T) {
	router, ts := mountRequirePermission(&fakeReplicaReader{err: errBoom})
	token := tokenForTestRole(t, ts, "ADMIN")

	rec := doTestRequest(router, token)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRequirePermission_Mapped_PassesThrough(t *testing.T) {
	router, ts := mountRequirePermission(&fakeReplicaReader{allowed: true})
	token := tokenForTestRole(t, ts, "ADMIN")

	rec := doTestRequest(router, token)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("expected passthrough handler body, got: %s", rec.Body.String())
	}
}

// Basic any-of / always-on / non-internal wiring (task 2.3). The full
// Property 4/5 tables are task 2.5.
func TestRequirePermission_Rules(t *testing.T) {
	tests := []struct {
		name      string
		keys      []string
		role      string
		allowed   bool
		wantCode  int
		wantCalls int
	}{
		{"any-of passes all keys to one query", []string{"settings.vendors", "settings.pks"}, "ATM-SPV", true, http.StatusOK, 1},
		{"any-of no match -> 403", []string{"settings.vendors", "settings.pks"}, "ATM-SPV", false, http.StatusForbidden, 1},
		{"always-on key, internal role -> allowed without query", []string{"approvals"}, "ATM-USER", false, http.StatusOK, 0},
		{"always-on key among others -> allowed without query", []string{"settings.pks", "dashboard"}, "CC-CWM", false, http.StatusOK, 0},
		{"vendor role -> 403 without query even if granted", []string{"monitoring.audit-logs"}, "VENDOR-USER", true, http.StatusForbidden, 0},
		{"vendor role on always-on key -> 403", []string{"dashboard"}, "VENDOR-ATM-USER", true, http.StatusForbidden, 0},
		{"EXT-CLIENT -> 403 without query", []string{"dashboard"}, "EXT-CLIENT", true, http.StatusForbidden, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &fakeReplicaReader{allowed: tt.allowed}
			router, ts := mountRequirePermissionKeys(reader, tt.keys...)

			rec := doTestRequest(router, tokenForTestRole(t, ts, tt.role))

			if rec.Code != tt.wantCode {
				t.Fatalf("code = %d, want %d: %s", rec.Code, tt.wantCode, rec.Body.String())
			}
			if reader.calls != tt.wantCalls {
				t.Fatalf("repo calls = %d, want %d", reader.calls, tt.wantCalls)
			}
			if tt.wantCalls > 0 && strings.Join(reader.gotKeys, ",") != strings.Join(tt.keys, ",") {
				t.Fatalf("repo keys = %v, want %v", reader.gotKeys, tt.keys)
			}
		})
	}
}

func TestRequirePermission_NoKeys_Panics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for RequirePermission with no keys")
		}
	}()
	RequirePermission(NewPermissionEvaluator(&fakeReplicaReader{}))
}

var errBoom = &boomError{}

type boomError struct{}

func (*boomError) Error() string { return "boom" }
