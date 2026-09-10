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
)

// fakeChangePasswordService implements ChangePasswordService for httptest,
// no real DB or bcrypt needed — the underlying logic is covered by
// backend/internal/auth/change_password_test.go; this only exercises HTTP
// wiring (route, auth gate, request decode, error->status mapping).
type fakeChangePasswordService struct {
	err error

	calledUserID int64
	calledReq    auth.ChangePasswordRequest
}

func (f *fakeChangePasswordService) ChangePassword(_ context.Context, userID int64, req auth.ChangePasswordRequest, _ string) error {
	f.calledUserID = userID
	f.calledReq = req
	return f.err
}

// mountAuthHandlerForChangePassword mounts only what /change-password needs
// (tokenService for RequireAuth + the fake service) — the other AuthHandler
// fields stay nil, which is fine since these tests never hit /login etc.
func mountAuthHandlerForChangePassword(svc ChangePasswordService) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}, noopBlacklist{})

	h := &AuthHandler{tokenService: tokenSvc, changePasswordSvc: svc}
	r := chi.NewRouter()
	r.Mount("/api/v1/auth", h.Routes())
	return r, tokenSvc
}

func TestAuthHandler_ChangePassword_HappyPath(t *testing.T) {
	fake := &fakeChangePasswordService{}
	router, tokenSvc := mountAuthHandlerForChangePassword(fake)
	token := tokenFor(t, tokenSvc, 42)

	rec := doRequest(router, http.MethodPost, "/api/v1/auth/change-password", token,
		`{"old_password":"OldPassword1","new_password":"NewPassword2"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if fake.calledUserID != 42 {
		t.Errorf("expected ChangePassword called with userID=42, got %d", fake.calledUserID)
	}
	if fake.calledReq.OldPassword != "OldPassword1" || fake.calledReq.NewPassword != "NewPassword2" {
		t.Errorf("unexpected request forwarded to service: %+v", fake.calledReq)
	}
}

func TestAuthHandler_ChangePassword_NoAuth_Unauthorized(t *testing.T) {
	fake := &fakeChangePasswordService{}
	router, _ := mountAuthHandlerForChangePassword(fake)

	rec := doRequest(router, http.MethodPost, "/api/v1/auth/change-password", "",
		`{"old_password":"OldPassword1","new_password":"NewPassword2"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAuthHandler_ChangePassword_WrongOldPassword(t *testing.T) {
	fake := &fakeChangePasswordService{err: pkgauth.ErrInvalidCredentials}
	router, tokenSvc := mountAuthHandlerForChangePassword(fake)
	token := tokenFor(t, tokenSvc, 42)

	rec := doRequest(router, http.MethodPost, "/api/v1/auth/change-password", token,
		`{"old_password":"WrongPassword","new_password":"NewPassword2"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAuthHandler_ChangePassword_LDAPAccount_Forbidden(t *testing.T) {
	fake := &fakeChangePasswordService{err: pkgauth.ErrChangeNotAllowed}
	router, tokenSvc := mountAuthHandlerForChangePassword(fake)
	token := tokenFor(t, tokenSvc, 42)

	rec := doRequest(router, http.MethodPost, "/api/v1/auth/change-password", token,
		`{"old_password":"OldPassword1","new_password":"NewPassword2"}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAuthHandler_ChangePassword_NewEqualsOld_BadRequest(t *testing.T) {
	fake := &fakeChangePasswordService{err: pkgauth.ErrPasswordUnchanged}
	router, tokenSvc := mountAuthHandlerForChangePassword(fake)
	token := tokenFor(t, tokenSvc, 42)

	rec := doRequest(router, http.MethodPost, "/api/v1/auth/change-password", token,
		`{"old_password":"SamePassword1","new_password":"SamePassword1"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAuthHandler_ChangePassword_InvalidBody(t *testing.T) {
	fake := &fakeChangePasswordService{}
	router, tokenSvc := mountAuthHandlerForChangePassword(fake)
	token := tokenFor(t, tokenSvc, 42)

	rec := doRequest(router, http.MethodPost, "/api/v1/auth/change-password", token, `not json`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAuthHandler_ChangePassword_ServiceUnavailable(t *testing.T) {
	fake := &fakeChangePasswordService{err: pkgauth.ErrServiceUnavailable}
	router, tokenSvc := mountAuthHandlerForChangePassword(fake)
	token := tokenFor(t, tokenSvc, 42)

	rec := doRequest(router, http.MethodPost, "/api/v1/auth/change-password", token,
		`{"old_password":"OldPassword1","new_password":"NewPassword2"}`)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAuthHandler_ChangePassword_UnknownError_InternalServerError(t *testing.T) {
	fake := &fakeChangePasswordService{err: errors.New("something unexpected")}
	router, tokenSvc := mountAuthHandlerForChangePassword(fake)
	token := tokenFor(t, tokenSvc, 42)

	rec := doRequest(router, http.MethodPost, "/api/v1/auth/change-password", token,
		`{"old_password":"OldPassword1","new_password":"NewPassword2"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAuthHandler_ChangePassword_WeakPassword_ValidationError(t *testing.T) {
	fake := &fakeChangePasswordService{err: &pkgauth.ValidationError{Field: "new_password", Message: "minimal 8 karakter"}}
	router, tokenSvc := mountAuthHandlerForChangePassword(fake)
	token := tokenFor(t, tokenSvc, 42)

	rec := doRequest(router, http.MethodPost, "/api/v1/auth/change-password", token,
		`{"old_password":"OldPassword1","new_password":"short"}`)

	// writeValidationError uses 422, matching every other ValidationError
	// response in this handler (e.g. login's field-level validation).
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}
