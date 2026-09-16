package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// fakeVendorAdminServicer implements VendorAdminServicer for httptest, no
// real DB needed — the underlying logic is covered by
// backend/internal/service/vendor_admin_test.go; this only exercises HTTP wiring.
type fakeVendorAdminServicer struct {
	listResult  []db.ListVendorsAdminRow
	listErr     error
	countResult int64
	countErr    error
	getResult   *db.GetVendorAdminByIDRow
	getErr      error

	createResult db.CreateVendorAdminRow
	createErr    error
	createCalled bool

	updateResult db.UpdateVendorAdminRow
	updateErr    error
	updateCalled bool

	disableResult service.DisableVendorResult
	disableErr    error
	disableCalled bool

	enableErr    error
	enableCalled bool
}

func (f *fakeVendorAdminServicer) List(context.Context, db.ListVendorsAdminParams) ([]db.ListVendorsAdminRow, error) {
	return f.listResult, f.listErr
}

func (f *fakeVendorAdminServicer) Count(context.Context, db.CountVendorsAdminParams) (int64, error) {
	return f.countResult, f.countErr
}

func (f *fakeVendorAdminServicer) Get(context.Context, int64) (*db.GetVendorAdminByIDRow, error) {
	return f.getResult, f.getErr
}

func (f *fakeVendorAdminServicer) Create(context.Context, int64, service.CreateVendorRequest, string) (db.CreateVendorAdminRow, error) {
	f.createCalled = true
	return f.createResult, f.createErr
}

func (f *fakeVendorAdminServicer) Update(context.Context, int64, int64, service.UpdateVendorRequest, string) (db.UpdateVendorAdminRow, error) {
	f.updateCalled = true
	return f.updateResult, f.updateErr
}

func (f *fakeVendorAdminServicer) Disable(context.Context, int64, int64, string) (service.DisableVendorResult, error) {
	f.disableCalled = true
	return f.disableResult, f.disableErr
}

func (f *fakeVendorAdminServicer) Enable(context.Context, int64, int64, string) error {
	f.enableCalled = true
	return f.enableErr
}

// mountAdminVendorHandler mirrors the real mount in cmd/api/main.go:
// RequireAuth + RequireRoles("ADMIN", "ADMIN_PARAM").
func mountAdminVendorHandler(svc VendorAdminServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}, noopBlacklist{})

	h := NewAdminVendorHandler(svc)
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM"),
	).Mount("/api/v1/admin/vendors", h.Routes())
	return r, tokenSvc
}

func TestAdminVendorHandler_List_HappyPath(t *testing.T) {
	svc := &fakeVendorAdminServicer{
		listResult:  []db.ListVendorsAdminRow{{ID: 1, Code: "ACM", Name: "Acme", IsActive: true}},
		countResult: 1,
	}
	router, tokenSvc := mountAdminVendorHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors?page=1&page_size=10", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"total":1`) || !strings.Contains(body, `"page_size":10`) {
		t.Errorf("expected pagination metadata in body, got: %s", body)
	}
}

func TestAdminVendorHandler_List_InvalidStatus_BadRequest(t *testing.T) {
	router, tokenSvc := mountAdminVendorHandler(&fakeVendorAdminServicer{})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN_PARAM")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors?status=bogus", token, "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminVendorHandler_List_NoAuth_Unauthorized(t *testing.T) {
	router, _ := mountAdminVendorHandler(&fakeVendorAdminServicer{})

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors", "", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminVendorHandler_List_WrongRole_Forbidden(t *testing.T) {
	router, tokenSvc := mountAdminVendorHandler(&fakeVendorAdminServicer{})
	token := tokenForRole(t, tokenSvc, 1, "ATM-USER")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors", token, "")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminVendorHandler_Get_HappyPath(t *testing.T) {
	svc := &fakeVendorAdminServicer{getResult: &db.GetVendorAdminByIDRow{ID: 1, Code: "ACM", Name: "Acme", IsActive: true}}
	router, tokenSvc := mountAdminVendorHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/1", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminVendorHandler_Get_NotFound(t *testing.T) {
	router, tokenSvc := mountAdminVendorHandler(&fakeVendorAdminServicer{getResult: nil})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/999", token, "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminVendorHandler_Create_HappyPath(t *testing.T) {
	svc := &fakeVendorAdminServicer{createResult: db.CreateVendorAdminRow{ID: 1, Code: "ACM", Name: "Acme", IsActive: true}}
	router, tokenSvc := mountAdminVendorHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors", token, `{"code":"ACM","name":"Acme"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if !svc.createCalled {
		t.Error("expected VendorAdminServicer.Create to be called")
	}
}

func TestAdminVendorHandler_Create_ValidationError_422(t *testing.T) {
	svc := &fakeVendorAdminServicer{createErr: &service.ValidationError{Field: "code", Message: "wajib diisi"}}
	router, tokenSvc := mountAdminVendorHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors", token, `{"name":"Acme"}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminVendorHandler_Create_CodeConflict_409(t *testing.T) {
	svc := &fakeVendorAdminServicer{createErr: service.ErrVendorCodeConflict}
	router, tokenSvc := mountAdminVendorHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors", token, `{"code":"ACM","name":"Acme"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminVendorHandler_Update_HappyPath(t *testing.T) {
	svc := &fakeVendorAdminServicer{updateResult: db.UpdateVendorAdminRow{ID: 1, Code: "ACM", Name: "Renamed", IsActive: true}}
	router, tokenSvc := mountAdminVendorHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/vendors/1", token, `{"name":"Renamed"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !svc.updateCalled {
		t.Error("expected VendorAdminServicer.Update to be called")
	}
}

func TestAdminVendorHandler_Update_ImmutableCode_400(t *testing.T) {
	svc := &fakeVendorAdminServicer{updateErr: service.ErrVendorCodeImmutable}
	router, tokenSvc := mountAdminVendorHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/vendors/1", token, `{"code":"DIFFERENT","name":"Acme"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminVendorHandler_Update_NotFound_404(t *testing.T) {
	svc := &fakeVendorAdminServicer{updateErr: service.ErrVendorNotFound}
	router, tokenSvc := mountAdminVendorHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/vendors/999", token, `{"name":"Acme"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminVendorHandler_Disable_HappyPath_NoWarning(t *testing.T) {
	svc := &fakeVendorAdminServicer{disableResult: service.DisableVendorResult{LinkedUsersWarning: 0}}
	router, tokenSvc := mountAdminVendorHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/1/disable", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !svc.disableCalled {
		t.Error("expected VendorAdminServicer.Disable to be called")
	}
	if strings.Contains(rec.Body.String(), "linked_active_users") {
		t.Errorf("expected no linked_active_users warning when count is 0, got: %s", rec.Body.String())
	}
}

func TestAdminVendorHandler_Disable_SurfacesLinkedUsersWarning(t *testing.T) {
	svc := &fakeVendorAdminServicer{disableResult: service.DisableVendorResult{LinkedUsersWarning: 3}}
	router, tokenSvc := mountAdminVendorHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/1/disable", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"linked_active_users":3`) {
		t.Errorf("expected linked_active_users=3 warning in body, got: %s", rec.Body.String())
	}
}

func TestAdminVendorHandler_Disable_NotFound_404(t *testing.T) {
	svc := &fakeVendorAdminServicer{disableErr: service.ErrVendorNotFound}
	router, tokenSvc := mountAdminVendorHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/999/disable", token, "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminVendorHandler_Enable_HappyPath(t *testing.T) {
	svc := &fakeVendorAdminServicer{}
	router, tokenSvc := mountAdminVendorHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/1/enable", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !svc.enableCalled {
		t.Error("expected VendorAdminServicer.Enable to be called")
	}
}

func TestAdminVendorHandler_Enable_NotFound_404(t *testing.T) {
	svc := &fakeVendorAdminServicer{enableErr: service.ErrVendorNotFound}
	router, tokenSvc := mountAdminVendorHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN_PARAM")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/999/enable", token, "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
