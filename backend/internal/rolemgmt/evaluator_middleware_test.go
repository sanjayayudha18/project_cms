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
}

func (f *fakeReplicaReader) HasPermission(context.Context, string, string) (bool, error) {
	return f.allowed, f.err
}

type noopBlacklist struct{}

func (noopBlacklist) Add(context.Context, string, time.Duration) error { return nil }
func (noopBlacklist) IsBlacklisted(context.Context, string) (bool, error) {
	return false, nil
}

func mountRequirePermission(reader ReplicaReader) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}, noopBlacklist{})

	evaluator := NewPermissionEvaluator(reader)
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		RequirePermission(evaluator, "dashboard"),
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

var errBoom = &boomError{}

type boomError struct{}

func (*boomError) Error() string { return "boom" }
