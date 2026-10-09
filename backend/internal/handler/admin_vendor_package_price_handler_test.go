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

// fakeVendorPackagePriceServicer exercises HTTP wiring only; validation and
// apply logic are covered in internal/service.
type fakeVendorPackagePriceServicer struct {
	listResult []service.VendorPackagePrice
	getResult  *service.VendorPackagePrice
	err        error // returned by every mutation
	listArg    db.ListVendorPackagePricesAdminParams
}

func (f *fakeVendorPackagePriceServicer) List(_ context.Context, arg db.ListVendorPackagePricesAdminParams) ([]service.VendorPackagePrice, error) {
	f.listArg = arg
	return f.listResult, nil
}
func (f *fakeVendorPackagePriceServicer) Count(context.Context, db.CountVendorPackagePricesAdminParams) (int64, error) {
	return int64(len(f.listResult)), nil
}
func (f *fakeVendorPackagePriceServicer) Get(context.Context, int64, int64) (*service.VendorPackagePrice, error) {
	return f.getResult, nil
}
func (f *fakeVendorPackagePriceServicer) Create(context.Context, int64, int64, service.VendorPackagePriceCreatePayload, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 7, Status: "pending"}, f.err
}
func (f *fakeVendorPackagePriceServicer) Update(context.Context, int64, int64, int64, service.VendorPackagePriceContentPayload, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 8, Status: "pending"}, f.err
}
func (f *fakeVendorPackagePriceServicer) Disable(context.Context, int64, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 9, Status: "pending"}, f.err
}

func mountAdminVendorPackagePriceHandler(svc VendorPackagePriceAdminServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM"),
	).Mount("/api/v1/admin/vendors/{vendorID}/package-prices", NewAdminVendorPackagePriceHandler(svc).Routes())
	return r, tokenSvc
}

const pricesPath = "/api/v1/admin/vendors/3/package-prices"

func TestAdminVendorPackagePriceHandler_ListAndGet(t *testing.T) {
	base := "1250000.50"
	price := service.VendorPackagePrice{ID: 1, VendorID: 3, Package: "PAKET 3", PackageCode: "PKG3_ABC_001", BasePrice: &base}
	svc := &fakeVendorPackagePriceServicer{listResult: []service.VendorPackagePrice{price}, getResult: &price}
	router, ts := mountAdminVendorPackagePriceHandler(svc)
	token := tokenForRole(t, ts, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, pricesPath+"?package=PAKET%203&page=2&page_size=10", token, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"base_price":"1250000.50"`) || !strings.Contains(rec.Body.String(), `"total":1`) {
		t.Fatalf("list = %d %s", rec.Code, rec.Body.String())
	}
	if svc.listArg.VendorID != 3 || svc.listArg.Package == nil || *svc.listArg.Package != "PAKET 3" || svc.listArg.PageOffset != 10 {
		t.Errorf("list arg = %+v", svc.listArg)
	}
	if rec := doRequest(router, http.MethodGet, pricesPath+"/1", token, ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"package_code":"PKG3_ABC_001"`) {
		t.Fatalf("get = %d %s", rec.Code, rec.Body.String())
	}

	svc.getResult = nil
	for path, want := range map[string]int{
		pricesPath + "/1":                        http.StatusNotFound,
		pricesPath + "/abc":                      http.StatusBadRequest,
		pricesPath + "?page=0":                   http.StatusBadRequest,
		pricesPath + "?status=bogus":             http.StatusBadRequest,
		"/api/v1/admin/vendors/x/package-prices": http.StatusBadRequest,
	} {
		if rec := doRequest(router, http.MethodGet, path, token, ""); rec.Code != want {
			t.Errorf("GET %s = %d, want %d", path, rec.Code, want)
		}
	}
}

// Every mutation is staged for approval: 202 + change_request_id, never a direct write.
func TestAdminVendorPackagePriceHandler_MutationsReturn202(t *testing.T) {
	router, ts := mountAdminVendorPackagePriceHandler(&fakeVendorPackagePriceServicer{})
	token := tokenForRole(t, ts, 1, "ADMIN_PARAM")
	for _, tc := range []struct {
		method, path, body, wantID string
	}{
		{http.MethodPost, pricesPath, `{"package":"PAKET 3","machine_group":"ATM","price_class":"REGULAR","tier_min":1,"base_price":"100.00","effective_start_date":"2027-01-01"}`, `"change_request_id":7`},
		{http.MethodPut, pricesPath + "/1", `{"base_price":"150.00"}`, `"change_request_id":8`},
		{http.MethodPost, pricesPath + "/1/disable", ``, `"change_request_id":9`},
	} {
		rec := doRequest(router, tc.method, tc.path, token, tc.body)
		if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), tc.wantID) {
			t.Errorf("%s %s = %d %s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, pricesPath, `{bad`},
		{http.MethodPut, pricesPath + "/1", `{bad`},
		{http.MethodPut, pricesPath + "/abc", `{}`},
		{http.MethodPost, pricesPath + "/abc/disable", ``},
	} {
		if rec := doRequest(router, tc.method, tc.path, token, tc.body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s %s = %d, want 400", tc.method, tc.path, rec.Code)
		}
	}
}

func TestAdminVendorPackagePriceHandler_ErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{&service.ValidationError{Field: "base_price", Message: "tidak valid"}, http.StatusUnprocessableEntity},
		{service.ErrVendorPackagePriceNotFound, http.StatusNotFound},
		{service.ErrVendorPackagePriceNoVendor, http.StatusNotFound},
		{service.ErrVendorPackagePriceInternal, http.StatusConflict},
		{service.ErrVendorPackagePriceOverlap, http.StatusConflict},
		{service.ErrMasterDataChangePending, http.StatusConflict},
		{service.ErrMasterDataForbidden, http.StatusForbidden},
		{context.DeadlineExceeded, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		router, ts := mountAdminVendorPackagePriceHandler(&fakeVendorPackagePriceServicer{err: tc.err})
		token := tokenForRole(t, ts, 1, "ADMIN")
		for _, path := range []string{pricesPath, pricesPath + "/1/disable"} {
			if rec := doRequest(router, http.MethodPost, path, token, `{}`); rec.Code != tc.want {
				t.Errorf("POST %s err %v = %d, want %d", path, tc.err, rec.Code, tc.want)
			}
		}
		if rec := doRequest(router, http.MethodPut, pricesPath+"/1", token, `{}`); rec.Code != tc.want {
			t.Errorf("PUT err %v = %d, want %d", tc.err, rec.Code, tc.want)
		}
	}
}

func TestAdminVendorPackagePriceHandler_RBAC(t *testing.T) {
	router, ts := mountAdminVendorPackagePriceHandler(&fakeVendorPackagePriceServicer{})
	if rec := doRequest(router, http.MethodPost, pricesPath, tokenForRole(t, ts, 1, "ATM-USER"), `{}`); rec.Code != http.StatusForbidden {
		t.Errorf("ATM-USER create = %d, want 403", rec.Code)
	}
	if rec := doRequest(router, http.MethodGet, pricesPath, "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("no token = %d, want 401", rec.Code)
	}
}
