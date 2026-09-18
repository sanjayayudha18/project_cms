package handler

import (
	"context"
	"errors"
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

// fakeATMAdminServicer implements ATMAdminServicer for httptest, no real DB
// needed -- the underlying logic is covered by
// backend/internal/service/atm_admin_test.go; this only exercises HTTP wiring.
type fakeATMAdminServicer struct {
	listResult  []service.ATM
	listErr     error
	countResult int64
	countErr    error
	getResult   service.ATM
	getErr      error

	createResult service.ATM
	createErr    error
	createCalled bool

	updateResult service.ATM
	updateErr    error
	updateCalled bool

	disableErr    error
	disableCalled bool

	enableErr    error
	enableCalled bool

	listLocationsResult []service.LocationOption
	listLocationsErr    error
}

func (f *fakeATMAdminServicer) List(context.Context, db.ListATMsAdminParams) ([]service.ATM, error) {
	return f.listResult, f.listErr
}

func (f *fakeATMAdminServicer) Count(context.Context, db.CountATMsAdminParams) (int64, error) {
	return f.countResult, f.countErr
}

func (f *fakeATMAdminServicer) Get(context.Context, int64) (service.ATM, error) {
	return f.getResult, f.getErr
}

func (f *fakeATMAdminServicer) Create(context.Context, int64, service.CreateATMRequest, string) (service.ATM, error) {
	f.createCalled = true
	return f.createResult, f.createErr
}

func (f *fakeATMAdminServicer) Update(context.Context, int64, int64, service.UpdateATMRequest, string) (service.ATM, error) {
	f.updateCalled = true
	return f.updateResult, f.updateErr
}

func (f *fakeATMAdminServicer) Disable(context.Context, int64, int64, string) error {
	f.disableCalled = true
	return f.disableErr
}

func (f *fakeATMAdminServicer) Enable(context.Context, int64, int64, string) error {
	f.enableCalled = true
	return f.enableErr
}

func (f *fakeATMAdminServicer) ListLocations(context.Context) ([]service.LocationOption, error) {
	return f.listLocationsResult, f.listLocationsErr
}

// mountAdminATMHandler mirrors the real mount planned for cmd/api/main.go:
// RequireAuth + RequireRoles("ADMIN", "ADMIN_PARAM").
func mountAdminATMHandler(svc ATMAdminServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}, noopBlacklist{})

	h := NewAdminATMHandler(svc)
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM"),
	).Mount("/api/v1/admin/atms", h.Routes())
	return r, tokenSvc
}

func TestAdminATMHandler_List_HappyPath(t *testing.T) {
	loc := "Jakarta Pusat"
	svc := &fakeATMAdminServicer{
		listResult:  []service.ATM{{ID: 1, TerminalID: "TATM001", LocationName: &loc, IsActive: true}},
		countResult: 1,
	}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/atms?page=1&page_size=10", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"total":1`) || !strings.Contains(body, `"page_size":10`) {
		t.Errorf("expected pagination metadata in body, got: %s", body)
	}
	if !strings.Contains(body, `"terminal_id":"TATM001"`) {
		t.Errorf("expected flat-JSON atm row in body, got: %s", body)
	}
}

func TestAdminATMHandler_List_InvalidStatus_BadRequest(t *testing.T) {
	router, tokenSvc := mountAdminATMHandler(&fakeATMAdminServicer{})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN_PARAM")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/atms?status=bogus", token, "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_List_InvalidLocationID_BadRequest(t *testing.T) {
	router, tokenSvc := mountAdminATMHandler(&fakeATMAdminServicer{})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/atms?location_id=not-a-number", token, "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_List_NoAuth_Unauthorized(t *testing.T) {
	router, _ := mountAdminATMHandler(&fakeATMAdminServicer{})

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/atms", "", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_List_WrongRole_Forbidden(t *testing.T) {
	router, tokenSvc := mountAdminATMHandler(&fakeATMAdminServicer{})
	token := tokenForRole(t, tokenSvc, 1, "ATM-USER")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/atms", token, "")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_Locations_HappyPath_StaticRouteBeforeParam(t *testing.T) {
	svc := &fakeATMAdminServicer{
		listLocationsResult: []service.LocationOption{{ID: 10, Name: "Jakarta Pusat", CityOrRegency: "Jakarta Pusat", Province: "DKI Jakarta"}},
	}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	// /locations must resolve to ListLocations, not fall into the /{id} route.
	rec := doRequest(router, http.MethodGet, "/api/v1/admin/atms/locations", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"name":"Jakarta Pusat"`) {
		t.Errorf("expected location options in body, got: %s", rec.Body.String())
	}
}

func TestAdminATMHandler_Get_HappyPath(t *testing.T) {
	svc := &fakeATMAdminServicer{getResult: service.ATM{ID: 1, TerminalID: "TATM001", IsActive: true}}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/atms/1", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_Get_NotFound(t *testing.T) {
	svc := &fakeATMAdminServicer{getErr: service.ErrATMNotFound}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/atms/999", token, "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_Create_HappyPath(t *testing.T) {
	svc := &fakeATMAdminServicer{createResult: service.ATM{ID: 1, TerminalID: "TATM001", IsActive: true}}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	body := `{"terminal_id":"TATM001","location_id":10,"machine_type":"ATM","brand":"NCR","model":"SelfServ","operation_hours":"24 Hours","deployment_type":"Onsite"}`
	rec := doRequest(router, http.MethodPost, "/api/v1/admin/atms", token, body)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if !svc.createCalled {
		t.Error("expected ATMAdminServicer.Create to be called")
	}
}

func TestAdminATMHandler_Create_ValidationError_422(t *testing.T) {
	svc := &fakeATMAdminServicer{createErr: &service.ValidationError{Field: "terminal_id", Message: "wajib diisi"}}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/atms", token, `{"location_id":10}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_Create_TerminalIDConflict_409(t *testing.T) {
	svc := &fakeATMAdminServicer{createErr: service.ErrATMTerminalIDConflict}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/atms", token, `{"terminal_id":"TATM001","location_id":10,"machine_type":"ATM","brand":"NCR","model":"SelfServ","operation_hours":"24 Hours","deployment_type":"Onsite"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_Create_InvalidReference_400(t *testing.T) {
	svc := &fakeATMAdminServicer{createErr: service.ErrATMInvalidReference}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/atms", token, `{"terminal_id":"TATM001","location_id":999,"machine_type":"ATM","brand":"NCR","model":"SelfServ","operation_hours":"24 Hours","deployment_type":"Onsite"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid_reference") {
		t.Errorf("expected invalid_reference error code in body, got: %s", rec.Body.String())
	}
}

func TestAdminATMHandler_Update_HappyPath(t *testing.T) {
	svc := &fakeATMAdminServicer{updateResult: service.ATM{ID: 1, TerminalID: "TATM001", Brand: "Diebold", IsActive: true}}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/atms/1", token, `{"location_id":10,"machine_type":"ATM","brand":"Diebold","model":"SelfServ","operation_hours":"24 Hours","deployment_type":"Onsite"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !svc.updateCalled {
		t.Error("expected ATMAdminServicer.Update to be called")
	}
}

func TestAdminATMHandler_Update_ImmutableTerminalID_400(t *testing.T) {
	svc := &fakeATMAdminServicer{updateErr: service.ErrATMTerminalIDImmutable}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/atms/1", token, `{"terminal_id":"DIFFERENT","location_id":10,"machine_type":"ATM","brand":"NCR","model":"SelfServ","operation_hours":"24 Hours","deployment_type":"Onsite"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_Update_NotFound_404(t *testing.T) {
	svc := &fakeATMAdminServicer{updateErr: service.ErrATMNotFound}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/atms/999", token, `{"location_id":10,"machine_type":"ATM","brand":"NCR","model":"SelfServ","operation_hours":"24 Hours","deployment_type":"Onsite"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_Disable_HappyPath(t *testing.T) {
	svc := &fakeATMAdminServicer{}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/atms/1/disable", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !svc.disableCalled {
		t.Error("expected ATMAdminServicer.Disable to be called")
	}
}

func TestAdminATMHandler_Disable_NotFound_404(t *testing.T) {
	svc := &fakeATMAdminServicer{disableErr: service.ErrATMNotFound}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/atms/999/disable", token, "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_Enable_HappyPath(t *testing.T) {
	svc := &fakeATMAdminServicer{}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/atms/1/enable", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !svc.enableCalled {
		t.Error("expected ATMAdminServicer.Enable to be called")
	}
}

func TestAdminATMHandler_Enable_NotFound_404(t *testing.T) {
	svc := &fakeATMAdminServicer{enableErr: service.ErrATMNotFound}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN_PARAM")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/atms/999/enable", token, "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- error/edge paths (500s, malformed bodies, unexpected repo errors) ----

func TestAdminATMHandler_List_RepoError_500(t *testing.T) {
	svc := &fakeATMAdminServicer{listErr: errors.New("db down")}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/atms", token, "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_List_CountError_500(t *testing.T) {
	svc := &fakeATMAdminServicer{countErr: errors.New("db down")}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/atms", token, "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_Locations_RepoError_500(t *testing.T) {
	svc := &fakeATMAdminServicer{listLocationsErr: errors.New("db down")}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/atms/locations", token, "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_Get_InvalidID_BadRequest(t *testing.T) {
	router, tokenSvc := mountAdminATMHandler(&fakeATMAdminServicer{})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/atms/not-a-number", token, "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_Get_UnexpectedError_500(t *testing.T) {
	svc := &fakeATMAdminServicer{getErr: errors.New("db down")}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/atms/1", token, "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_Create_MalformedBody_400(t *testing.T) {
	router, tokenSvc := mountAdminATMHandler(&fakeATMAdminServicer{})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/atms", token, `{not-json`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_Create_UnexpectedError_500(t *testing.T) {
	svc := &fakeATMAdminServicer{createErr: errors.New("db down")}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/atms", token, `{"terminal_id":"TATM001","location_id":10,"machine_type":"ATM","brand":"NCR","model":"SelfServ","operation_hours":"24 Hours","deployment_type":"Onsite"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_Update_InvalidID_BadRequest(t *testing.T) {
	router, tokenSvc := mountAdminATMHandler(&fakeATMAdminServicer{})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/atms/not-a-number", token, `{}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_Update_MalformedBody_400(t *testing.T) {
	router, tokenSvc := mountAdminATMHandler(&fakeATMAdminServicer{})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/atms/1", token, `{not-json`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_Update_UnexpectedError_500(t *testing.T) {
	svc := &fakeATMAdminServicer{updateErr: errors.New("db down")}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/atms/1", token, `{"location_id":10,"machine_type":"ATM","brand":"NCR","model":"SelfServ","operation_hours":"24 Hours","deployment_type":"Onsite"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_Disable_InvalidID_BadRequest(t *testing.T) {
	router, tokenSvc := mountAdminATMHandler(&fakeATMAdminServicer{})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/atms/not-a-number/disable", token, "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_Disable_UnexpectedError_500(t *testing.T) {
	svc := &fakeATMAdminServicer{disableErr: errors.New("db down")}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/atms/1/disable", token, "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_Enable_InvalidID_BadRequest(t *testing.T) {
	router, tokenSvc := mountAdminATMHandler(&fakeATMAdminServicer{})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/atms/not-a-number/enable", token, "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMHandler_Enable_UnexpectedError_500(t *testing.T) {
	svc := &fakeATMAdminServicer{enableErr: errors.New("db down")}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/atms/1/enable", token, "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}
