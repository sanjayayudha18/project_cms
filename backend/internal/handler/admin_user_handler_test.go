package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/auth"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// fakeSetInitialPasswordService implements SetInitialPasswordService for
// httptest, no real DB needed — the underlying logic is covered by
// backend/internal/auth/set_initial_password_test.go; this only exercises
// HTTP wiring (route, role gate, request decode, error->status mapping).
type fakeSetInitialPasswordService struct {
	err error

	calledActorID  int64
	calledTargetID int64
	calledReq      auth.SetInitialPasswordRequest
}

func (f *fakeSetInitialPasswordService) SetInitialPassword(_ context.Context, actorID, targetUserID int64, req auth.SetInitialPasswordRequest, _ string) error {
	f.calledActorID = actorID
	f.calledTargetID = targetUserID
	f.calledReq = req
	return f.err
}

// mountAdminUserHandler mirrors the real mount in cmd/api/main.go:
// RequireAuth + RequireRoles("APPACCESS") — needed to test the role gate
// itself, not just the handler in isolation.
func mountAdminUserHandler(svc SetInitialPasswordService) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}, noopBlacklist{})

	h := NewAdminUserHandler(svc)
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		custommw.RequireRoles("APPACCESS"),
	).Mount("/api/v1/admin/users", h.Routes())
	return r, tokenSvc
}

func TestAdminUserHandler_SetInitialPassword_HappyPath(t *testing.T) {
	fake := &fakeSetInitialPasswordService{}
	router, tokenSvc := mountAdminUserHandler(fake)
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/7/set-initial-password", token,
		`{"new_password":"InitialPass1"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if fake.calledActorID != 1 {
		t.Errorf("expected actorID=1, got %d", fake.calledActorID)
	}
	if fake.calledTargetID != 7 {
		t.Errorf("expected targetUserID=7, got %d", fake.calledTargetID)
	}
}

func TestAdminUserHandler_SetInitialPassword_NonAPPACCESSRole_Forbidden(t *testing.T) {
	fake := &fakeSetInitialPasswordService{}
	router, tokenSvc := mountAdminUserHandler(fake)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/7/set-initial-password", token,
		`{"new_password":"InitialPass1"}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_SetInitialPassword_TargetLDAP_Forbidden(t *testing.T) {
	fake := &fakeSetInitialPasswordService{err: pkgauth.ErrChangeNotAllowed}
	router, tokenSvc := mountAdminUserHandler(fake)
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/7/set-initial-password", token,
		`{"new_password":"InitialPass1"}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_SetInitialPassword_TargetNotFound(t *testing.T) {
	fake := &fakeSetInitialPasswordService{err: pkgauth.ErrUserNotFound}
	router, tokenSvc := mountAdminUserHandler(fake)
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/999/set-initial-password", token,
		`{"new_password":"InitialPass1"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_SetInitialPassword_InvalidBody(t *testing.T) {
	fake := &fakeSetInitialPasswordService{}
	router, tokenSvc := mountAdminUserHandler(fake)
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/7/set-initial-password", token, `not json`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_SetInitialPassword_InvalidPathID(t *testing.T) {
	fake := &fakeSetInitialPasswordService{}
	router, tokenSvc := mountAdminUserHandler(fake)
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/not-a-number/set-initial-password", token,
		`{"new_password":"InitialPass1"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_SetInitialPassword_WeakPassword_ValidationError(t *testing.T) {
	fake := &fakeSetInitialPasswordService{err: &pkgauth.ValidationError{Field: "new_password", Message: "minimal 8 karakter"}}
	router, tokenSvc := mountAdminUserHandler(fake)
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/7/set-initial-password", token,
		`{"new_password":"short"}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_SetInitialPassword_ServiceUnavailable(t *testing.T) {
	fake := &fakeSetInitialPasswordService{err: pkgauth.ErrServiceUnavailable}
	router, tokenSvc := mountAdminUserHandler(fake)
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/7/set-initial-password", token,
		`{"new_password":"InitialPass1"}`)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_SetInitialPassword_UnknownError_InternalServerError(t *testing.T) {
	fake := &fakeSetInitialPasswordService{err: errors.New("something unexpected")}
	router, tokenSvc := mountAdminUserHandler(fake)
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/7/set-initial-password", token,
		`{"new_password":"InitialPass1"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_SetInitialPassword_NoAuth_Unauthorized(t *testing.T) {
	fake := &fakeSetInitialPasswordService{}
	router, _ := mountAdminUserHandler(fake)

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/7/set-initial-password", "",
		`{"new_password":"InitialPass1"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}
