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

	createResult db.MasterDataChangeRequest
	createErr    error
	createCalled bool
	lastCreate   service.CreateVendorRequest

	updateResult db.MasterDataChangeRequest
	updateErr    error
	updateCalled bool
	lastUpdate   service.UpdateVendorRequest

	disableResult service.DisableVendorResult
	disableErr    error
	disableCalled bool

	enableResult db.MasterDataChangeRequest
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

func (f *fakeVendorAdminServicer) Create(_ context.Context, _ int64, req service.CreateVendorRequest, _ string) (db.MasterDataChangeRequest, error) {
	f.createCalled = true
	f.lastCreate = req
	return f.createResult, f.createErr
}

func (f *fakeVendorAdminServicer) Update(_ context.Context, _ int64, _ int64, req service.UpdateVendorRequest, _ string) (db.MasterDataChangeRequest, error) {
	f.updateCalled = true
	f.lastUpdate = req
	return f.updateResult, f.updateErr
}

func (f *fakeVendorAdminServicer) Disable(context.Context, int64, int64, string) (service.DisableVendorResult, error) {
	f.disableCalled = true
	return f.disableResult, f.disableErr
}

func (f *fakeVendorAdminServicer) Enable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	f.enableCalled = true
	return f.enableResult, f.enableErr
}

// mountAdminVendorHandler mirrors the real mount in cmd/api/main.go:
// RequireAuth + RequireRoles("ADMIN", "ADMIN_PARAM").
func mountAdminVendorHandler(svc VendorAdminServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
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

// T4.1: create no longer returns the vendor row (nothing exists until the
// change is approved) -- it returns 202 with the pending change request.
func TestAdminVendorHandler_Create_Accepted202(t *testing.T) {
	svc := &fakeVendorAdminServicer{createResult: db.MasterDataChangeRequest{ID: 11, EntityType: "vendor", Op: "create", Status: "pending"}}
	router, tokenSvc := mountAdminVendorHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors", token, `{"code":"ACM","name":"Acme"}`)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"change_request_id":11`, `"status":"pending"`, `"entity_type":"vendor"`, `"op":"create"`} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %s in body, got: %s", want, body)
		}
	}
	if strings.Contains(body, `"code":"ACM"`) {
		t.Errorf("202 body must not pretend a vendor row exists yet, got: %s", body)
	}
	if !svc.createCalled {
		t.Error("expected VendorAdminServicer.Create to be called")
	}
}

// T4.3: legal_name and npwp are accepted on create and appear in list/get.
func TestAdminVendorHandler_LegalNameAndNPWP_RequestAndResponse(t *testing.T) {
	legal, npwp := "PT Acme Sejahtera", "012345678901000"
	svc := &fakeVendorAdminServicer{
		createResult: db.MasterDataChangeRequest{ID: 30, Status: "pending"},
		getResult:    &db.GetVendorAdminByIDRow{ID: 1, Code: "ACM", Name: "Acme", LegalName: &legal, Npwp: &npwp, IsActive: true},
		listResult:   []db.ListVendorsAdminRow{{ID: 1, Code: "ACM", Name: "Acme", LegalName: &legal, Npwp: &npwp, IsActive: true}},
		countResult:  1,
	}
	router, tokenSvc := mountAdminVendorHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors", token, `{"code":"ACM","name":"Acme","legal_name":"PT Acme Sejahtera","npwp":"01.234.567.8-901.000"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create: expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.lastCreate.LegalName != "PT Acme Sejahtera" || svc.lastCreate.NPWP != "01.234.567.8-901.000" {
		t.Errorf("create request = %+v, want legal_name/npwp passed through untouched (service normalizes)", svc.lastCreate)
	}

	for _, path := range []string{"/api/v1/admin/vendors/1", "/api/v1/admin/vendors"} {
		body := doRequest(router, http.MethodGet, path, token, "").Body.String()
		if !strings.Contains(body, `"legal_name":"PT Acme Sejahtera"`) || !strings.Contains(body, `"npwp":"012345678901000"`) {
			t.Errorf("GET %s should include legal_name and npwp, got: %s", path, body)
		}
	}
}

// T4.3: a vendor without these values serializes them as null (not omitted, not "").
func TestAdminVendorHandler_Get_LegalNameNPWPNullWhenUnset(t *testing.T) {
	svc := &fakeVendorAdminServicer{getResult: &db.GetVendorAdminByIDRow{ID: 1, Code: "ACM", Name: "Acme", IsActive: true}}
	router, tokenSvc := mountAdminVendorHandler(svc)

	body := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/1", tokenForRole(t, tokenSvc, 1, "ADMIN"), "").Body.String()

	if !strings.Contains(body, `"legal_name":null`) || !strings.Contains(body, `"npwp":null`) {
		t.Errorf("expected explicit nulls, got: %s", body)
	}
}

// T4.3: on update, absent/null legal_name+npwp mean "keep" (nil pointer) while ""
// means "clear" (non-nil blank) -- the wire semantics ATM/vendor clients rely on
// so a PUT from a client that predates the fields cannot wipe them.
func TestAdminVendorHandler_Update_LegalNameNPWP_TriStateWire(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantLegal *string
		wantNPWP  *string
	}{
		{"omitted", `{"name":"Acme"}`, nil, nil},
		{"explicit null", `{"name":"Acme","legal_name":null,"npwp":null}`, nil, nil},
		{"empty string clears", `{"name":"Acme","legal_name":"","npwp":""}`, strPtr(""), strPtr("")},
		{"values set", `{"name":"Acme","legal_name":"PT Baru","npwp":"0123456789012345"}`, strPtr("PT Baru"), strPtr("0123456789012345")},
		{"only npwp", `{"name":"Acme","npwp":"0123456789012345"}`, nil, strPtr("0123456789012345")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeVendorAdminServicer{updateResult: db.MasterDataChangeRequest{ID: 31, Status: "pending"}}
			router, tokenSvc := mountAdminVendorHandler(svc)

			rec := doRequest(router, http.MethodPut, "/api/v1/admin/vendors/1", tokenForRole(t, tokenSvc, 1, "ADMIN"), tc.body)

			if rec.Code != http.StatusAccepted {
				t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
			}
			if !equalStrPtr(svc.lastUpdate.LegalName, tc.wantLegal) || !equalStrPtr(svc.lastUpdate.NPWP, tc.wantNPWP) {
				t.Errorf("update request legal_name=%v npwp=%v, want %v / %v", deref(svc.lastUpdate.LegalName), deref(svc.lastUpdate.NPWP), deref(tc.wantLegal), deref(tc.wantNPWP))
			}
		})
	}
}

func TestAdminVendorHandler_Create_InvalidNPWP_422(t *testing.T) {
	svc := &fakeVendorAdminServicer{createErr: &service.ValidationError{Field: "npwp", Message: "harus 15 atau 16 digit"}}
	router, tokenSvc := mountAdminVendorHandler(svc)

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors", tokenForRole(t, tokenSvc, 1, "ADMIN"), `{"code":"ACM","name":"Acme","npwp":"123"}`)

	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "npwp") {
		t.Fatalf("expected 422 naming the npwp field, got %d: %s", rec.Code, rec.Body.String())
	}
}

func strPtr(s string) *string { return &s }

func equalStrPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func deref(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return "'" + *p + "'"
}

// T4.1 / T3.6: the service-layer RBAC and pending-change guards reach the
// client as 403 / 409 on every vendor mutation, not a 500.
func TestAdminVendorHandler_Mutations_ForbiddenAndPending(t *testing.T) {
	cases := []struct {
		name string
		svc  *fakeVendorAdminServicer
		want int
	}{
		{"forbidden", &fakeVendorAdminServicer{createErr: service.ErrMasterDataForbidden, updateErr: service.ErrMasterDataForbidden, disableErr: service.ErrMasterDataForbidden, enableErr: service.ErrMasterDataForbidden}, http.StatusForbidden},
		{"pending change exists", &fakeVendorAdminServicer{createErr: service.ErrMasterDataChangePending, updateErr: service.ErrMasterDataChangePending, disableErr: service.ErrMasterDataChangePending, enableErr: service.ErrMasterDataChangePending}, http.StatusConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, tokenSvc := mountAdminVendorHandler(tc.svc)
			token := tokenForRole(t, tokenSvc, 1, "ADMIN")
			for _, req := range []struct{ method, path, body string }{
				{http.MethodPost, "/api/v1/admin/vendors", `{"code":"ACM","name":"Acme"}`},
				{http.MethodPut, "/api/v1/admin/vendors/1", `{"name":"Acme"}`},
				{http.MethodPost, "/api/v1/admin/vendors/1/disable", ""},
				{http.MethodPost, "/api/v1/admin/vendors/1/enable", ""},
			} {
				if rec := doRequest(router, req.method, req.path, token, req.body); rec.Code != tc.want {
					t.Errorf("%s %s: expected %d, got %d: %s", req.method, req.path, tc.want, rec.Code, rec.Body.String())
				}
			}
		})
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

func TestAdminVendorHandler_Update_Accepted202(t *testing.T) {
	svc := &fakeVendorAdminServicer{updateResult: db.MasterDataChangeRequest{ID: 12, EntityType: "vendor", Op: "update", Status: "pending"}}
	router, tokenSvc := mountAdminVendorHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/vendors/1", token, `{"name":"Renamed"}`)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"change_request_id":12`) {
		t.Errorf("expected change_request_id in body, got: %s", rec.Body.String())
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

func TestAdminVendorHandler_Disable_Accepted202_NoWarning(t *testing.T) {
	svc := &fakeVendorAdminServicer{disableResult: service.DisableVendorResult{
		Change: db.MasterDataChangeRequest{ID: 13, EntityType: "vendor", Op: "disable", Status: "pending"},
	}}
	router, tokenSvc := mountAdminVendorHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/1/disable", token, "")

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"change_request_id":13`) {
		t.Errorf("expected change_request_id in body, got: %s", rec.Body.String())
	}
	if !svc.disableCalled {
		t.Error("expected VendorAdminServicer.Disable to be called")
	}
	if strings.Contains(rec.Body.String(), "linked_active_users") {
		t.Errorf("expected no linked_active_users warning when count is 0, got: %s", rec.Body.String())
	}
}

func TestAdminVendorHandler_Disable_SurfacesLinkedUsersWarning(t *testing.T) {
	svc := &fakeVendorAdminServicer{disableResult: service.DisableVendorResult{
		Change:             db.MasterDataChangeRequest{ID: 14, EntityType: "vendor", Op: "disable", Status: "pending"},
		LinkedUsersWarning: 3,
	}}
	router, tokenSvc := mountAdminVendorHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/1/disable", token, "")

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"linked_active_users":3`) || !strings.Contains(body, `"change_request_id":14`) {
		t.Errorf("expected linked_active_users=3 warning alongside the change request, got: %s", body)
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

func TestAdminVendorHandler_Enable_Accepted202(t *testing.T) {
	svc := &fakeVendorAdminServicer{enableResult: db.MasterDataChangeRequest{ID: 15, EntityType: "vendor", Op: "enable", Status: "pending"}}
	router, tokenSvc := mountAdminVendorHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/1/enable", token, "")

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"change_request_id":15`) {
		t.Errorf("expected change_request_id in body, got: %s", rec.Body.String())
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
