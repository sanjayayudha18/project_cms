package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// mountAuditLogHandlerWithAuth mounts the Audit Log Viewer handler behind
// the real RequireAuth + RequireRoles("ADMIN","ADMIN_PARAM") middleware
// stack, matching cmd/api/main.go's wiring — used to test 401/403, which
// mountAuditLogHandler (no middleware) can't exercise.
func mountAuditLogHandlerWithAuth(svc service.AuditLogServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})

	h := NewAuditLogHandler(svc)
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM"),
	).Mount("/api/v1/audit-logs", h.Routes())
	return r, tokenSvc
}

func TestAuditLogHandler_NoToken_Unauthorized(t *testing.T) {
	router, _ := mountAuditLogHandlerWithAuth(&stubAuditLogService{listResult: &service.ListAuditLogsResult{}})

	rec := doRequest(router, http.MethodGet, "/api/v1/audit-logs", "", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestAuditLogHandler_NonAdminRole_Forbidden(t *testing.T) {
	router, ts := mountAuditLogHandlerWithAuth(&stubAuditLogService{listResult: &service.ListAuditLogsResult{}})

	access, _, err := ts.GenerateTokenPair(&pkgauth.AuthIdentity{UserID: 1, Username: "user", Role: "ATM-USER"})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	rec := doRequest(router, http.MethodGet, "/api/v1/audit-logs", access, "")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestAuditLogHandler_AdminRole_Allowed(t *testing.T) {
	router, ts := mountAuditLogHandlerWithAuth(&stubAuditLogService{listResult: &service.ListAuditLogsResult{}})

	access, _, err := ts.GenerateTokenPair(&pkgauth.AuthIdentity{UserID: 1, Username: "admin", Role: "ADMIN_PARAM"})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	rec := doRequest(router, http.MethodGet, "/api/v1/audit-logs", access, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
}
