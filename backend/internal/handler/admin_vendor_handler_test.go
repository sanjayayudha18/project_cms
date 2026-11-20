package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
	"github.com/cimb-niaga/cms/pkg/reports"
)

// fakeVendorAdminServicer implements VendorAdminServicer for httptest, no
// real DB needed — the underlying logic is covered by
// backend/internal/service/vendor_admin_test.go; this only exercises HTTP wiring.
type fakeVendorAdminServicer struct {
	listResult  []db.ListVendorsAdminRow
	listErr     error
	countResult int64
	countErr    error
	listCalled  bool
	lastList    db.ListVendorsAdminParams
	lastCount   db.CountVendorsAdminParams
	getResult   *db.GetVendorAdminByIDRow
	getErr      error

	lastRole string

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

func (f *fakeVendorAdminServicer) List(_ context.Context, role string, p db.ListVendorsAdminParams) ([]db.ListVendorsAdminRow, error) {
	f.listCalled = true
	f.lastRole = role
	f.lastList = p
	return f.listResult, f.listErr
}

func (f *fakeVendorAdminServicer) Count(_ context.Context, _ string, p db.CountVendorsAdminParams) (int64, error) {
	f.lastCount = p
	return f.countResult, f.countErr
}

func (f *fakeVendorAdminServicer) Get(_ context.Context, role string, _ int64) (*db.GetVendorAdminByIDRow, error) {
	return f.getResult, f.getErr
}

func (f *fakeVendorAdminServicer) GetKindAndDirector(context.Context, string, int64) (string, *db.GetVendorDirectorPicRow, error) {
	return "FLM_VENDOR", nil, nil
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
// RequireAuth + RequireRoles(reports.PksViewerRoles...); Routes() narrows writes.
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
		custommw.RequireRoles(reports.PksViewerRoles...),
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

func TestAdminVendorHandler_List_KindFilter(t *testing.T) {
	str := func(s string) *string { return &s }
	tests := []struct {
		name     string
		query    string
		wantCode int
		wantKind *string
		wantCall bool
		wantLim  int64
		wantOff  int64
	}{
		{name: "FLM_VENDOR", query: "kind=FLM_VENDOR", wantCode: 200, wantKind: str("FLM_VENDOR"), wantCall: true, wantLim: 25},
		{name: "INTERNAL", query: "kind=INTERNAL", wantCode: 200, wantKind: str("INTERNAL"), wantCall: true, wantLim: 25},
		{name: "empty kind is nil", query: "kind=", wantCode: 200, wantCall: true, wantLim: 25},
		{name: "absent kind is nil", query: "", wantCode: 200, wantCall: true, wantLim: 25},
		{name: "invalid kind", query: "kind=xyz", wantCode: 400},
		{name: "lowercase kind invalid", query: "kind=internal", wantCode: 400},
		{
			name: "combined with status, paging", query: "kind=INTERNAL&status=active&page=3&page_size=10",
			wantCode: 200, wantKind: str("INTERNAL"), wantCall: true, wantLim: 10, wantOff: 20,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeVendorAdminServicer{}
			router, tokenSvc := mountAdminVendorHandler(svc)
			token := tokenForRole(t, tokenSvc, 1, "ADMIN")

			rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors?"+tt.query, token, "")

			if rec.Code != tt.wantCode {
				t.Fatalf("code = %d, want %d: %s", rec.Code, tt.wantCode, rec.Body.String())
			}
			if svc.listCalled != tt.wantCall {
				t.Fatalf("service called = %v, want %v", svc.listCalled, tt.wantCall)
			}
			if !tt.wantCall {
				return
			}
			for name, got := range map[string]*string{"List": svc.lastList.Kind, "Count": svc.lastCount.Kind} {
				if (got == nil) != (tt.wantKind == nil) || (got != nil && *got != *tt.wantKind) {
					t.Errorf("%s Kind = %v, want %v", name, got, tt.wantKind)
				}
			}
			if svc.lastList.PageLimit != tt.wantLim || svc.lastList.PageOffset != tt.wantOff {
				t.Errorf("paging = %d/%d, want %d/%d", svc.lastList.PageLimit, svc.lastList.PageOffset, tt.wantLim, tt.wantOff)
			}
		})
	}
}

// Each item carries pks_count (never the old pks/pks_status fields) and the director.
func TestAdminVendorHandler_List_ResponseCarriesDirectorAndPksCount(t *testing.T) {
	str := func(s string) *string { return &s }
	i64 := func(n int64) *int64 { return &n }
	svc := &fakeVendorAdminServicer{
		countResult: 2,
		listResult: []db.ListVendorsAdminRow{
			{ID: 1, Code: "A", Name: "A", IsActive: true, Kind: "FLM_VENDOR", PksCount: 3,
				DirectorID: i64(7), DirectorName: str("Budi"), DirectorPhone: str("0812"), DirectorEmail: str("b@x.id")},
			{ID: 2, Code: "B", Name: "B", IsActive: true, Kind: "INTERNAL"},
		},
	}
	router, tokenSvc := mountAdminVendorHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Vendors []map[string]any `json:"vendors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || len(body.Vendors) != 2 {
		t.Fatalf("decode: %v, vendors=%d, body=%s", err, len(body.Vendors), rec.Body.String())
	}
	for i, v := range body.Vendors {
		for _, removed := range []string{"pks", "pks_status"} {
			if _, ok := v[removed]; ok {
				t.Errorf("vendor %d still carries the removed %q field", i, removed)
			}
		}
	}
	v0, v1 := body.Vendors[0], body.Vendors[1]
	dir, _ := v0["director"].(map[string]any)
	if dir["name"] != "Budi" || dir["phone"] != "0812" || dir["email"] != "b@x.id" {
		t.Errorf("vendor 0 director = %v", v0["director"])
	}
	if v0["pks_count"] != float64(3) || v1["pks_count"] != float64(0) {
		t.Errorf("pks_count = %v/%v, want 3/0", v0["pks_count"], v1["pks_count"])
	}
	if v1["director"] != nil || v1["kind"] != "INTERNAL" {
		t.Errorf("vendor 1 director/kind = %v/%v, want nil/INTERNAL", v1["director"], v1["kind"])
	}
}

// ATM-SPV may read vendors (Req 9.2) but every write stays ADMIN/ADMIN_PARAM.
func TestAdminVendorHandler_ATMSPV_ReadOnly(t *testing.T) {
	svc := &fakeVendorAdminServicer{
		getResult:  &db.GetVendorAdminByIDRow{ID: 1, Code: "ACM", Name: "Acme", IsActive: true},
		listResult: []db.ListVendorsAdminRow{{ID: 1, Code: "ACM", Name: "Acme", IsActive: true}},
	}
	router, tokenSvc := mountAdminVendorHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ATM-SPV")

	for _, path := range []string{"/api/v1/admin/vendors", "/api/v1/admin/vendors/1"} {
		if rec := doRequest(router, http.MethodGet, path, token, ""); rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200: %s", path, rec.Code, rec.Body.String())
		}
	}
	if svc.lastRole != "ATM-SPV" {
		t.Errorf("service saw role %q, want ATM-SPV", svc.lastRole)
	}
	writes := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/admin/vendors"},
		{http.MethodPut, "/api/v1/admin/vendors/1"},
		{http.MethodPost, "/api/v1/admin/vendors/1/disable"},
		{http.MethodPost, "/api/v1/admin/vendors/1/enable"},
	}
	for _, w := range writes {
		if rec := doRequest(router, w.method, w.path, token, `{}`); rec.Code != http.StatusForbidden {
			t.Errorf("%s %s = %d, want 403", w.method, w.path, rec.Code)
		}
	}
	if svc.createCalled || svc.updateCalled || svc.disableCalled || svc.enableCalled {
		t.Error("a write reached the service for ATM-SPV")
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
