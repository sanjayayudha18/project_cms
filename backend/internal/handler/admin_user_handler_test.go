package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/auth"
	"github.com/cimb-niaga/cms/backend/internal/db"
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

// fakeUserAdminServicer implements UserAdminServicer for httptest, no real
// DB needed — the underlying logic is covered by
// backend/internal/auth/user_admin_test.go; this only exercises HTTP wiring.
type fakeUserAdminServicer struct {
	listResult  []db.ListUsersAdminRow
	listErr     error
	countResult int64
	countErr    error
	getResult   *db.GetUserAdminByIDRow
	getErr      error

	createResult db.CreateUserAdminRow
	createErr    error
	createCalled bool
	createReq    auth.CreateUserRequest

	updateResult db.UpdateUserAdminRow
	updateErr    error
	updateCalled bool
	updateReq    auth.UpdateUserRequest
}

func (f *fakeUserAdminServicer) List(context.Context, db.ListUsersAdminParams) ([]db.ListUsersAdminRow, error) {
	return f.listResult, f.listErr
}

func (f *fakeUserAdminServicer) Count(context.Context, db.CountUsersAdminParams) (int64, error) {
	return f.countResult, f.countErr
}

func (f *fakeUserAdminServicer) Get(context.Context, int64) (*db.GetUserAdminByIDRow, error) {
	return f.getResult, f.getErr
}

func (f *fakeUserAdminServicer) Create(_ context.Context, _ int64, req auth.CreateUserRequest, _ string) (db.CreateUserAdminRow, error) {
	f.createCalled = true
	f.createReq = req
	return f.createResult, f.createErr
}

func (f *fakeUserAdminServicer) Update(_ context.Context, _, _ int64, req auth.UpdateUserRequest, _ string) (db.UpdateUserAdminRow, error) {
	f.updateCalled = true
	f.updateReq = req
	return f.updateResult, f.updateErr
}

// fakeDeactivateService implements DeactivateService for httptest.
type fakeDeactivateService struct {
	deactivateErr    error
	reactivateErr    error
	deactivateCalled bool
	reactivateCalled bool
	deactivateTarget int64
	reactivateTarget int64
}

func (f *fakeDeactivateService) Deactivate(_ context.Context, _, targetUserID int64, _ string) error {
	f.deactivateCalled = true
	f.deactivateTarget = targetUserID
	return f.deactivateErr
}

func (f *fakeDeactivateService) Reactivate(_ context.Context, _, targetUserID int64, _ string) error {
	f.reactivateCalled = true
	f.reactivateTarget = targetUserID
	return f.reactivateErr
}

// mountAdminUserHandler mirrors the real mount in cmd/api/main.go:
// RequireAuth + RequireRoles("APPACCESS") — needed to test the role gate
// itself, not just the handler in isolation.
func mountAdminUserHandler(svc SetInitialPasswordService, userAdminSvc UserAdminServicer, deactivateSvc DeactivateService) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}, noopBlacklist{})

	h := NewAdminUserHandler(svc, userAdminSvc, deactivateSvc)
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		custommw.RequireRoles("APPACCESS"),
	).Mount("/api/v1/admin/users", h.Routes())
	return r, tokenSvc
}

func TestAdminUserHandler_SetInitialPassword_HappyPath(t *testing.T) {
	fake := &fakeSetInitialPasswordService{}
	router, tokenSvc := mountAdminUserHandler(fake, &fakeUserAdminServicer{}, &fakeDeactivateService{})
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
	router, tokenSvc := mountAdminUserHandler(fake, &fakeUserAdminServicer{}, &fakeDeactivateService{})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/7/set-initial-password", token,
		`{"new_password":"InitialPass1"}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_SetInitialPassword_TargetLDAP_Forbidden(t *testing.T) {
	fake := &fakeSetInitialPasswordService{err: pkgauth.ErrChangeNotAllowed}
	router, tokenSvc := mountAdminUserHandler(fake, &fakeUserAdminServicer{}, &fakeDeactivateService{})
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/7/set-initial-password", token,
		`{"new_password":"InitialPass1"}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_SetInitialPassword_TargetNotFound(t *testing.T) {
	fake := &fakeSetInitialPasswordService{err: pkgauth.ErrUserNotFound}
	router, tokenSvc := mountAdminUserHandler(fake, &fakeUserAdminServicer{}, &fakeDeactivateService{})
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/999/set-initial-password", token,
		`{"new_password":"InitialPass1"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_SetInitialPassword_InvalidBody(t *testing.T) {
	fake := &fakeSetInitialPasswordService{}
	router, tokenSvc := mountAdminUserHandler(fake, &fakeUserAdminServicer{}, &fakeDeactivateService{})
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/7/set-initial-password", token, `not json`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_SetInitialPassword_InvalidPathID(t *testing.T) {
	fake := &fakeSetInitialPasswordService{}
	router, tokenSvc := mountAdminUserHandler(fake, &fakeUserAdminServicer{}, &fakeDeactivateService{})
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/not-a-number/set-initial-password", token,
		`{"new_password":"InitialPass1"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_SetInitialPassword_WeakPassword_ValidationError(t *testing.T) {
	fake := &fakeSetInitialPasswordService{err: &pkgauth.ValidationError{Field: "new_password", Message: "minimal 8 karakter"}}
	router, tokenSvc := mountAdminUserHandler(fake, &fakeUserAdminServicer{}, &fakeDeactivateService{})
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/7/set-initial-password", token,
		`{"new_password":"short"}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_SetInitialPassword_ServiceUnavailable(t *testing.T) {
	fake := &fakeSetInitialPasswordService{err: pkgauth.ErrServiceUnavailable}
	router, tokenSvc := mountAdminUserHandler(fake, &fakeUserAdminServicer{}, &fakeDeactivateService{})
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/7/set-initial-password", token,
		`{"new_password":"InitialPass1"}`)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_SetInitialPassword_UnknownError_InternalServerError(t *testing.T) {
	fake := &fakeSetInitialPasswordService{err: errors.New("something unexpected")}
	router, tokenSvc := mountAdminUserHandler(fake, &fakeUserAdminServicer{}, &fakeDeactivateService{})
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/7/set-initial-password", token,
		`{"new_password":"InitialPass1"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_SetInitialPassword_NoAuth_Unauthorized(t *testing.T) {
	fake := &fakeSetInitialPasswordService{}
	router, _ := mountAdminUserHandler(fake, &fakeUserAdminServicer{}, &fakeDeactivateService{})

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/7/set-initial-password", "",
		`{"new_password":"InitialPass1"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- List / Get / Create / Update / Disable / Enable ----------------------

func TestAdminUserHandler_List_HappyPath_NoPasswordHashInBody(t *testing.T) {
	userSvc := &fakeUserAdminServicer{
		listResult:  []db.ListUsersAdminRow{{ID: 1, Username: "u1", FullName: "User One", Email: "u1@example.com", Role: "APPACCESS", IsActive: true}},
		countResult: 1,
	}
	router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, userSvc, &fakeDeactivateService{})
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/users?page=1&page_size=10", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"total":1`) || !strings.Contains(body, `"page":1`) || !strings.Contains(body, `"page_size":10`) {
		t.Errorf("expected pagination metadata in body, got: %s", body)
	}
	if strings.Contains(body, "password_hash") {
		t.Errorf("response body must never contain password_hash: %s", body)
	}
}

func TestAdminUserHandler_List_InvalidPageParam_BadRequest(t *testing.T) {
	router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, &fakeUserAdminServicer{}, &fakeDeactivateService{})
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/users?page=abc", token, "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_List_NoAuth_Unauthorized(t *testing.T) {
	router, _ := mountAdminUserHandler(&fakeSetInitialPasswordService{}, &fakeUserAdminServicer{}, &fakeDeactivateService{})

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/users", "", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_List_WrongRole_Forbidden(t *testing.T) {
	router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, &fakeUserAdminServicer{}, &fakeDeactivateService{})
	token := tokenForRole(t, tokenSvc, 1, "ATM-USER")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/users", token, "")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_Get_HappyPath(t *testing.T) {
	userSvc := &fakeUserAdminServicer{getResult: &db.GetUserAdminByIDRow{ID: 7, Username: "u7", FullName: "User Seven", Email: "u7@example.com", Role: "APPACCESS", IsActive: true}}
	router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, userSvc, &fakeDeactivateService{})
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/users/7", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "password_hash") {
		t.Errorf("response body must never contain password_hash: %s", rec.Body.String())
	}
}

func TestAdminUserHandler_Get_NotFound(t *testing.T) {
	userSvc := &fakeUserAdminServicer{getResult: nil}
	router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, userSvc, &fakeDeactivateService{})
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/users/999", token, "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_Create_HappyPath(t *testing.T) {
	userSvc := &fakeUserAdminServicer{createResult: db.CreateUserAdminRow{ID: 10, Username: "new.user", FullName: "New User", Email: "new@example.com", IsActive: true}}
	router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, userSvc, &fakeDeactivateService{})
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users", token,
		`{"username":"new.user","full_name":"New User","email":"new@example.com","role":"APPACCESS","auth_source":"ldap"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if !userSvc.createCalled {
		t.Error("expected UserAdminServicer.Create to be called")
	}
	if userSvc.createReq.Username != "new.user" {
		t.Errorf("expected Create called with username=new.user, got %q", userSvc.createReq.Username)
	}
	if strings.Contains(rec.Body.String(), "password_hash") {
		t.Errorf("response body must never contain password_hash: %s", rec.Body.String())
	}
}

func TestAdminUserHandler_Create_ValidationError_422(t *testing.T) {
	userSvc := &fakeUserAdminServicer{createErr: &pkgauth.ValidationError{Field: "email", Message: "wajib diisi"}}
	router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, userSvc, &fakeDeactivateService{})
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users", token,
		`{"username":"new.user","full_name":"New User","role":"APPACCESS","auth_source":"ldap"}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_Create_ConflictError_409(t *testing.T) {
	userSvc := &fakeUserAdminServicer{createErr: &auth.ConflictError{Field: "username", Message: "sudah digunakan"}}
	router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, userSvc, &fakeDeactivateService{})
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users", token,
		`{"username":"new.user","full_name":"New User","email":"new@example.com","role":"APPACCESS","auth_source":"ldap"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_Create_ReferenceError_400(t *testing.T) {
	userSvc := &fakeUserAdminServicer{createErr: &auth.ReferenceError{Field: "vendor_id", Message: "tidak ditemukan"}}
	router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, userSvc, &fakeDeactivateService{})
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users", token,
		`{"username":"new.user","full_name":"New User","email":"new@example.com","role":"APPACCESS","auth_source":"ldap"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_Update_HappyPath(t *testing.T) {
	userSvc := &fakeUserAdminServicer{updateResult: db.UpdateUserAdminRow{ID: 7, FullName: "Renamed", Email: "u7@example.com", IsActive: true}}
	router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, userSvc, &fakeDeactivateService{})
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/users/7", token,
		`{"full_name":"Renamed","email":"u7@example.com","role":"APPACCESS"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !userSvc.updateCalled {
		t.Error("expected UserAdminServicer.Update to be called")
	}
}

func TestAdminUserHandler_Update_NotFound_404(t *testing.T) {
	userSvc := &fakeUserAdminServicer{updateErr: pkgauth.ErrUserNotFound}
	router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, userSvc, &fakeDeactivateService{})
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/users/999", token,
		`{"full_name":"X","email":"x@example.com","role":"APPACCESS"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_Update_InvalidPathID_400(t *testing.T) {
	router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, &fakeUserAdminServicer{}, &fakeDeactivateService{})
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/users/not-a-number", token, `{"full_name":"X"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUserHandler_Disable_HappyPath(t *testing.T) {
	userSvc := &fakeUserAdminServicer{getResult: &db.GetUserAdminByIDRow{ID: 7, Username: "u7"}}
	deactivateSvc := &fakeDeactivateService{}
	router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, userSvc, deactivateSvc)
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/7/disable", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !deactivateSvc.deactivateCalled || deactivateSvc.deactivateTarget != 7 {
		t.Errorf("expected Deactivate called with target=7, got called=%v target=%d", deactivateSvc.deactivateCalled, deactivateSvc.deactivateTarget)
	}
}

func TestAdminUserHandler_Disable_SelfDisable_BadRequest(t *testing.T) {
	userSvc := &fakeUserAdminServicer{getResult: &db.GetUserAdminByIDRow{ID: 1, Username: "self"}}
	deactivateSvc := &fakeDeactivateService{}
	router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, userSvc, deactivateSvc)
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS") // actor id == target id

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/1/disable", token, "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if deactivateSvc.deactivateCalled {
		t.Error("expected Deactivate NOT to be called for a self-disable attempt")
	}
}

func TestAdminUserHandler_Disable_NotFound_404_NoDeactivateCall(t *testing.T) {
	userSvc := &fakeUserAdminServicer{getResult: nil}
	deactivateSvc := &fakeDeactivateService{}
	router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, userSvc, deactivateSvc)
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/999/disable", token, "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	if deactivateSvc.deactivateCalled {
		t.Error("expected Deactivate NOT to be called for a non-existent target")
	}
}

func TestAdminUserHandler_Enable_HappyPath(t *testing.T) {
	userSvc := &fakeUserAdminServicer{getResult: &db.GetUserAdminByIDRow{ID: 7, Username: "u7"}}
	deactivateSvc := &fakeDeactivateService{}
	router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, userSvc, deactivateSvc)
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/7/enable", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !deactivateSvc.reactivateCalled || deactivateSvc.reactivateTarget != 7 {
		t.Errorf("expected Reactivate called with target=7, got called=%v target=%d", deactivateSvc.reactivateCalled, deactivateSvc.reactivateTarget)
	}
}

// Enable on an unknown id must be a 404 with Reactivate never called --
// DeactivateUserService.Reactivate itself silently no-ops on an unknown id
// but would still leave an audit entry if reached, so the handler's
// existence pre-check must short-circuit before it.
func TestAdminUserHandler_Enable_UnknownID_404_ReactivateNotCalled(t *testing.T) {
	userSvc := &fakeUserAdminServicer{getResult: nil}
	deactivateSvc := &fakeDeactivateService{}
	router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, userSvc, deactivateSvc)
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users/999/enable", token, "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	if deactivateSvc.reactivateCalled {
		t.Error("expected Reactivate NOT to be called for a non-existent target (no audit for a non-existent id)")
	}
}
