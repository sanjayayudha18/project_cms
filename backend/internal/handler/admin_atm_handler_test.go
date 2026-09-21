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

	createResult db.MasterDataChangeRequest
	createErr    error
	createCalled bool

	updateResult db.MasterDataChangeRequest
	updateErr    error
	updateCalled bool

	disableResult db.MasterDataChangeRequest
	disableErr    error
	disableCalled bool

	enableResult db.MasterDataChangeRequest
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

func (f *fakeATMAdminServicer) Create(context.Context, int64, service.CreateATMRequest, string) (db.MasterDataChangeRequest, error) {
	f.createCalled = true
	return f.createResult, f.createErr
}

func (f *fakeATMAdminServicer) Update(context.Context, int64, int64, service.UpdateATMRequest, string) (db.MasterDataChangeRequest, error) {
	f.updateCalled = true
	return f.updateResult, f.updateErr
}

func (f *fakeATMAdminServicer) Disable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	f.disableCalled = true
	return f.disableResult, f.disableErr
}

func (f *fakeATMAdminServicer) Enable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	f.enableCalled = true
	return f.enableResult, f.enableErr
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

// T4.2: create no longer returns the ATM row (nothing exists until the change
// is approved) -- it returns 202 with the pending change request.
func TestAdminATMHandler_Create_Accepted202(t *testing.T) {
	svc := &fakeATMAdminServicer{createResult: db.MasterDataChangeRequest{ID: 21, EntityType: "atm", Op: "create", Status: "pending"}}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	body := `{"terminal_id":"TATM001","location_id":10,"machine_type":"ATM","brand":"NCR","model":"SelfServ","operation_hours":"24 Hours","deployment_type":"Onsite"}`
	rec := doRequest(router, http.MethodPost, "/api/v1/admin/atms", token, body)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	for _, want := range []string{`"change_request_id":21`, `"status":"pending"`, `"entity_type":"atm"`, `"op":"create"`} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %s in body, got: %s", want, got)
		}
	}
	if strings.Contains(got, `"terminal_id"`) {
		t.Errorf("202 body must not pretend an ATM row exists yet, got: %s", got)
	}
	if !svc.createCalled {
		t.Error("expected ATMAdminServicer.Create to be called")
	}
}

// T4.2 / T3.6: the service-layer RBAC and pending-change guards reach the
// client as 403 / 409 on every ATM mutation, not a 500.
func TestAdminATMHandler_Mutations_ForbiddenAndPending(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"forbidden", service.ErrMasterDataForbidden, http.StatusForbidden},
		{"pending change exists", service.ErrMasterDataChangePending, http.StatusConflict},
	}
	body := `{"terminal_id":"TATM001","location_id":10,"machine_type":"ATM","brand":"NCR","model":"SelfServ","operation_hours":"24 Hours","deployment_type":"Onsite"}`
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, tokenSvc := mountAdminATMHandler(&fakeATMAdminServicer{createErr: tc.err, updateErr: tc.err, disableErr: tc.err, enableErr: tc.err})
			token := tokenForRole(t, tokenSvc, 1, "ADMIN")
			for _, req := range []struct{ method, path, body string }{
				{http.MethodPost, "/api/v1/admin/atms", body},
				{http.MethodPut, "/api/v1/admin/atms/1", body},
				{http.MethodPost, "/api/v1/admin/atms/1/disable", ""},
				{http.MethodPost, "/api/v1/admin/atms/1/enable", ""},
			} {
				if rec := doRequest(router, req.method, req.path, token, req.body); rec.Code != tc.want {
					t.Errorf("%s %s: expected %d, got %d: %s", req.method, req.path, tc.want, rec.Code, rec.Body.String())
				}
			}
		})
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

func TestAdminATMHandler_Update_Accepted202(t *testing.T) {
	svc := &fakeATMAdminServicer{updateResult: db.MasterDataChangeRequest{ID: 22, EntityType: "atm", Op: "update", Status: "pending"}}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/atms/1", token, `{"location_id":10,"machine_type":"ATM","brand":"Diebold","model":"SelfServ","operation_hours":"24 Hours","deployment_type":"Onsite"}`)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"change_request_id":22`) {
		t.Errorf("expected change_request_id in body, got: %s", rec.Body.String())
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

func TestAdminATMHandler_Disable_Accepted202(t *testing.T) {
	svc := &fakeATMAdminServicer{disableResult: db.MasterDataChangeRequest{ID: 23, EntityType: "atm", Op: "disable", Status: "pending"}}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/atms/1/disable", token, "")

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"change_request_id":23`) {
		t.Errorf("expected change_request_id in body, got: %s", rec.Body.String())
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

func TestAdminATMHandler_Enable_Accepted202(t *testing.T) {
	svc := &fakeATMAdminServicer{enableResult: db.MasterDataChangeRequest{ID: 24, EntityType: "atm", Op: "enable", Status: "pending"}}
	router, tokenSvc := mountAdminATMHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/atms/1/enable", token, "")

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"change_request_id":24`) {
		t.Errorf("expected change_request_id in body, got: %s", rec.Body.String())
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
