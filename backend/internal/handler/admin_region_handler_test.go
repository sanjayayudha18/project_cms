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

// fakeRegionAdminServicer implements RegionAdminServicer for httptest, no
// real DB needed -- the underlying logic is covered by
// backend/internal/service/region_admin_integration_test.go; this only
// exercises HTTP wiring.
type fakeRegionAdminServicer struct {
	listResult  []db.ListRegionsAdminRow
	listErr     error
	countResult int64
	countErr    error
	getResult   *db.GetRegionAdminByIDRow
	getErr      error

	createResult db.Region
	createErr    error
	createCalled bool

	updateResult db.Region
	updateErr    error
	updateCalled bool

	disableResult db.Region
	disableErr    error
	disableCalled bool

	enableResult db.Region
	enableErr    error
	enableCalled bool
}

func (f *fakeRegionAdminServicer) List(context.Context, db.ListRegionsAdminParams) ([]db.ListRegionsAdminRow, error) {
	return f.listResult, f.listErr
}

func (f *fakeRegionAdminServicer) Count(context.Context, db.CountRegionsAdminParams) (int64, error) {
	return f.countResult, f.countErr
}

func (f *fakeRegionAdminServicer) Get(context.Context, int64) (*db.GetRegionAdminByIDRow, error) {
	return f.getResult, f.getErr
}

func (f *fakeRegionAdminServicer) Create(context.Context, int64, string, service.CreateRegionRequest, string) (db.Region, error) {
	f.createCalled = true
	return f.createResult, f.createErr
}

func (f *fakeRegionAdminServicer) UpdateName(context.Context, int64, string, int64, service.UpdateRegionNameRequest, string) (db.Region, error) {
	f.updateCalled = true
	return f.updateResult, f.updateErr
}

func (f *fakeRegionAdminServicer) Disable(context.Context, int64, string, int64, string) (db.Region, error) {
	f.disableCalled = true
	return f.disableResult, f.disableErr
}

func (f *fakeRegionAdminServicer) Enable(context.Context, int64, string, int64, string) (db.Region, error) {
	f.enableCalled = true
	return f.enableResult, f.enableErr
}

// mountAdminRegionHandler mirrors the real mount in cmd/api/main.go:
// RequireAuth + RequireRoles("ADMIN", "ADMIN_PARAM").
func mountAdminRegionHandler(svc RegionAdminServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})

	h := NewAdminRegionHandler(svc)
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM"),
	).Mount("/api/v1/admin/regions", h.Routes())
	return r, tokenSvc
}

func regionName(s string) *string { return &s }

func TestAdminRegionHandler_List_HappyPath(t *testing.T) {
	svc := &fakeRegionAdminServicer{
		listResult:  []db.ListRegionsAdminRow{{ID: 1, Code: "JKT_CENTRAL", Region: regionName("JKT-Central"), IsActive: true, LocationCount: 12}},
		countResult: 1,
	}
	router, tokenSvc := mountAdminRegionHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/regions?page=1&page_size=10", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"total":1`, `"page_size":10`, `"code":"JKT_CENTRAL"`, `"is_active":true`, `"location_count":12`} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %s in body, got: %s", want, body)
		}
	}
}

func TestAdminRegionHandler_List_DefaultPageSizeIs20(t *testing.T) {
	svc := &fakeRegionAdminServicer{}
	router, tokenSvc := mountAdminRegionHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/regions", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"page_size":20`) {
		t.Errorf("expected default page_size=20 (Req 1.2), got: %s", rec.Body.String())
	}
}

func TestAdminRegionHandler_List_InvalidStatus_BadRequest(t *testing.T) {
	router, tokenSvc := mountAdminRegionHandler(&fakeRegionAdminServicer{})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/regions?status=bogus", token, "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRegionHandler_List_NoAuth_Unauthorized(t *testing.T) {
	router, _ := mountAdminRegionHandler(&fakeRegionAdminServicer{})

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/regions", "", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRegionHandler_List_WrongRole_Forbidden(t *testing.T) {
	router, tokenSvc := mountAdminRegionHandler(&fakeRegionAdminServicer{})
	token := tokenForRole(t, tokenSvc, 1, "ATM-USER")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/regions", token, "")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRegionHandler_List_RepoError_500(t *testing.T) {
	svc := &fakeRegionAdminServicer{listErr: errors.New("db down")}
	router, tokenSvc := mountAdminRegionHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/regions", token, "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRegionHandler_Get_HappyPath(t *testing.T) {
	svc := &fakeRegionAdminServicer{getResult: &db.GetRegionAdminByIDRow{ID: 1, Code: "JKT_CENTRAL", Region: regionName("JKT-Central"), IsActive: true, LocationCount: 3}}
	router, tokenSvc := mountAdminRegionHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/regions/1", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"location_count":3`) {
		t.Errorf("expected location_count in body, got: %s", rec.Body.String())
	}
}

func TestAdminRegionHandler_Get_NotFound(t *testing.T) {
	svc := &fakeRegionAdminServicer{getErr: service.ErrRegionNotFound}
	router, tokenSvc := mountAdminRegionHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/regions/999", token, "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRegionHandler_Get_InvalidID_BadRequest(t *testing.T) {
	router, tokenSvc := mountAdminRegionHandler(&fakeRegionAdminServicer{})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/regions/not-a-number", token, "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRegionHandler_Create_Created201(t *testing.T) {
	svc := &fakeRegionAdminServicer{createResult: db.Region{ID: 1, Code: "JKT_CENTRAL", Region: regionName("JKT-Central"), IsActive: true}}
	router, tokenSvc := mountAdminRegionHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/regions", token, `{"code":"JKT_CENTRAL","region":"JKT-Central"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"code":"JKT_CENTRAL"`, `"is_active":true`} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %s in body, got: %s", want, body)
		}
	}
	if !svc.createCalled {
		t.Error("expected RegionAdminServicer.Create to be called")
	}
}

func TestAdminRegionHandler_Create_ValidationError_422(t *testing.T) {
	svc := &fakeRegionAdminServicer{createErr: &service.ValidationError{Field: "code", Message: "kode wajib diisi"}}
	router, tokenSvc := mountAdminRegionHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/regions", token, `{"code":"","region":"X"}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRegionHandler_Create_CodeConflict_409(t *testing.T) {
	svc := &fakeRegionAdminServicer{createErr: service.ErrRegionCodeConflict}
	router, tokenSvc := mountAdminRegionHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/regions", token, `{"code":"JKT_CENTRAL","region":"X"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRegionHandler_Create_Forbidden_403(t *testing.T) {
	svc := &fakeRegionAdminServicer{createErr: service.ErrNotAuthorized}
	router, tokenSvc := mountAdminRegionHandler(svc)
	// Route guard lets ADMIN_PARAM through; service-layer re-check still
	// rejects (Req 5.2), exercised via the fake returning ErrNotAuthorized.
	token := tokenForRole(t, tokenSvc, 1, "ADMIN_PARAM")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/regions", token, `{"code":"JKT_CENTRAL","region":"X"}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRegionHandler_Create_MalformedBody_400(t *testing.T) {
	router, tokenSvc := mountAdminRegionHandler(&fakeRegionAdminServicer{})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/regions", token, `{not-json`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRegionHandler_Create_UnexpectedError_500(t *testing.T) {
	svc := &fakeRegionAdminServicer{createErr: errors.New("db down")}
	router, tokenSvc := mountAdminRegionHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/regions", token, `{"code":"JKT_CENTRAL","region":"X"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRegionHandler_UpdateName_OK200(t *testing.T) {
	svc := &fakeRegionAdminServicer{updateResult: db.Region{ID: 1, Code: "JKT_CENTRAL", Region: regionName("Jakarta Pusat"), IsActive: true}}
	router, tokenSvc := mountAdminRegionHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/regions/1", token, `{"region":"Jakarta Pusat"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"region":"Jakarta Pusat"`) {
		t.Errorf("expected updated region in body, got: %s", rec.Body.String())
	}
	if !svc.updateCalled {
		t.Error("expected RegionAdminServicer.UpdateName to be called")
	}
}

func TestAdminRegionHandler_UpdateName_CodeImmutable_400(t *testing.T) {
	svc := &fakeRegionAdminServicer{updateErr: service.ErrRegionCodeImmutable}
	router, tokenSvc := mountAdminRegionHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/regions/1", token, `{"code":"DIFFERENT","region":"X"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRegionHandler_UpdateName_NotFound_404(t *testing.T) {
	svc := &fakeRegionAdminServicer{updateErr: service.ErrRegionNotFound}
	router, tokenSvc := mountAdminRegionHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/regions/999", token, `{"region":"X"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRegionHandler_UpdateName_InvalidID_BadRequest(t *testing.T) {
	router, tokenSvc := mountAdminRegionHandler(&fakeRegionAdminServicer{})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/regions/not-a-number", token, `{}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRegionHandler_Disable_OK200(t *testing.T) {
	svc := &fakeRegionAdminServicer{disableResult: db.Region{ID: 1, Code: "JKT_CENTRAL", Region: regionName("JKT-Central"), IsActive: false}}
	router, tokenSvc := mountAdminRegionHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/regions/1/disable", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"is_active":false`) {
		t.Errorf("expected is_active:false in body, got: %s", rec.Body.String())
	}
	if !svc.disableCalled {
		t.Error("expected RegionAdminServicer.Disable to be called")
	}
}

func TestAdminRegionHandler_Disable_HasActiveLocations_409(t *testing.T) {
	svc := &fakeRegionAdminServicer{disableErr: errors.Join(service.ErrRegionHasActiveLocations, errors.New("2"))}
	router, tokenSvc := mountAdminRegionHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/regions/1/disable", token, "")

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRegionHandler_Disable_StatusUnchanged_409(t *testing.T) {
	svc := &fakeRegionAdminServicer{disableErr: service.ErrRegionStatusUnchanged}
	router, tokenSvc := mountAdminRegionHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/regions/1/disable", token, "")

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRegionHandler_Disable_NotFound_404(t *testing.T) {
	svc := &fakeRegionAdminServicer{disableErr: service.ErrRegionNotFound}
	router, tokenSvc := mountAdminRegionHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/regions/999/disable", token, "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRegionHandler_Enable_OK200(t *testing.T) {
	svc := &fakeRegionAdminServicer{enableResult: db.Region{ID: 1, Code: "JKT_CENTRAL", Region: regionName("JKT-Central"), IsActive: true}}
	router, tokenSvc := mountAdminRegionHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN_PARAM")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/regions/1/enable", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !svc.enableCalled {
		t.Error("expected RegionAdminServicer.Enable to be called")
	}
}

func TestAdminRegionHandler_Enable_StatusUnchanged_409(t *testing.T) {
	svc := &fakeRegionAdminServicer{enableErr: service.ErrRegionStatusUnchanged}
	router, tokenSvc := mountAdminRegionHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/regions/1/enable", token, "")

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRegionHandler_Enable_InvalidID_BadRequest(t *testing.T) {
	router, tokenSvc := mountAdminRegionHandler(&fakeRegionAdminServicer{})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/regions/not-a-number/enable", token, "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
