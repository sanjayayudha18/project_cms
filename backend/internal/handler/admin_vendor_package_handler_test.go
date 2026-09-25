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

// fakeVendorPackageAdminServicer exercises HTTP wiring only; logic is covered
// by internal/service/vendor_package_admin_test.go.
type fakeVendorPackageAdminServicer struct {
	listResult []service.VendorPackage
	getResult  *service.VendorPackage
	createErr  error
	updateErr  error
}

func (f *fakeVendorPackageAdminServicer) List(context.Context, db.ListVendorPackagesAdminParams) ([]service.VendorPackage, error) {
	return f.listResult, nil
}
func (f *fakeVendorPackageAdminServicer) Count(context.Context, db.CountVendorPackagesAdminParams) (int64, error) {
	return int64(len(f.listResult)), nil
}
func (f *fakeVendorPackageAdminServicer) Get(context.Context, int64, int64) (*service.VendorPackage, error) {
	return f.getResult, nil
}
func (f *fakeVendorPackageAdminServicer) Create(context.Context, int64, int64, service.VendorPackageCreatePayload, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 7, EntityType: "vendor_package", Op: "create", Status: "pending"}, f.createErr
}
func (f *fakeVendorPackageAdminServicer) Update(context.Context, int64, int64, int64, service.VendorPackageContentPayload, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 8, EntityType: "vendor_package", Op: "update", Status: "pending"}, f.updateErr
}
func (f *fakeVendorPackageAdminServicer) Disable(context.Context, int64, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 9, Status: "pending"}, nil
}

func mountAdminVendorPackageHandler(svc VendorPackageAdminServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM"),
	).Mount("/api/v1/admin/vendors/{vendorID}/packages", NewAdminVendorPackageHandler(svc).Routes())
	return r, tokenSvc
}

func TestAdminVendorPackageHandler_List(t *testing.T) {
	branchID := int64(9)
	svc := &fakeVendorPackageAdminServicer{listResult: []service.VendorPackage{{ID: 1, VendorBranchID: &branchID, PackageCode: "PKG1"}}}
	router, tokenSvc := mountAdminVendorPackageHandler(svc)

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/packages", tokenForRole(t, tokenSvc, 1, "ADMIN"), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, `"package_code":"PKG1"`) {
		t.Errorf("expected package row, got: %s", body)
	}
}

func TestAdminVendorPackageHandler_Get_NotFound(t *testing.T) {
	router, tokenSvc := mountAdminVendorPackageHandler(&fakeVendorPackageAdminServicer{})
	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/packages/99", tokenForRole(t, tokenSvc, 1, "ADMIN"), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestAdminVendorPackageHandler_Create_Returns202(t *testing.T) {
	router, tokenSvc := mountAdminVendorPackageHandler(&fakeVendorPackageAdminServicer{})
	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/packages", tokenForRole(t, tokenSvc, 1, "ADMIN"),
		`{"vendor_branch_id":9,"package_code":"PKG1","machine_group":"ATM","price_class":"REGULAR","effective_start_date":"2026-01-01","base_price":"100.00"}`)
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"change_request_id":7`) {
		t.Fatalf("expected 202 with change_request_id, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminVendorPackageHandler_Create_ErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{&service.ValidationError{Field: "base_price", Message: "tidak valid"}, http.StatusUnprocessableEntity},
		{service.ErrVendorPackageOverlap, http.StatusConflict},
		{service.ErrMasterDataChangePending, http.StatusConflict},
	}
	for _, tc := range cases {
		router, tokenSvc := mountAdminVendorPackageHandler(&fakeVendorPackageAdminServicer{createErr: tc.err})
		rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/packages", tokenForRole(t, tokenSvc, 1, "ADMIN"), `{"package_code":"PKG1"}`)
		if rec.Code != tc.want {
			t.Errorf("err %v: expected %d, got %d", tc.err, tc.want, rec.Code)
		}
	}
}

func TestAdminVendorPackageHandler_Update_Returns202(t *testing.T) {
	router, tokenSvc := mountAdminVendorPackageHandler(&fakeVendorPackageAdminServicer{})
	rec := doRequest(router, http.MethodPut, "/api/v1/admin/vendors/3/packages/1", tokenForRole(t, tokenSvc, 1, "ADMIN"),
		`{"base_price":"150.00","effective_end_date":"2027-12-31"}`)
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"change_request_id":8`) {
		t.Fatalf("expected 202 with change_request_id, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminVendorPackageHandler_WrongRole_Forbidden(t *testing.T) {
	router, tokenSvc := mountAdminVendorPackageHandler(&fakeVendorPackageAdminServicer{})
	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/packages", tokenForRole(t, tokenSvc, 1, "ATM-USER"), "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestAdminVendorPackageHandler_Disable_Returns202(t *testing.T) {
	router, tokenSvc := mountAdminVendorPackageHandler(&fakeVendorPackageAdminServicer{})
	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/packages/1/disable", tokenForRole(t, tokenSvc, 1, "ADMIN_PARAM"), "")
	if rec.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", rec.Code)
	}
}
