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
func (f *fakeVendorPackageAdminServicer) Create(context.Context, int64, int64, service.VendorPackagePayload, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 7, EntityType: "vendor_package", Op: "create", Status: "pending"}, f.createErr
}
func (f *fakeVendorPackageAdminServicer) Disable(context.Context, int64, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 9, Status: "pending"}, nil
}
func (f *fakeVendorPackageAdminServicer) Enable(context.Context, int64, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 10, Status: "pending"}, nil
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
	svc := &fakeVendorPackageAdminServicer{listResult: []service.VendorPackage{{ID: 1, VendorBranchID: &branchID, Code: "PKG1", IsActive: true}}}
	router, tokenSvc := mountAdminVendorPackageHandler(svc)

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/packages", tokenForRole(t, tokenSvc, 1, "ADMIN"), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, `"code":"PKG1"`) {
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
		`{"vendor_branch_id":9,"code":"PKG1","priority_class":"ALL","price":"100.00"}`)
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"change_request_id":7`) {
		t.Fatalf("expected 202 with change_request_id, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminVendorPackageHandler_Create_ErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{&service.ValidationError{Field: "price", Message: "tidak valid"}, http.StatusUnprocessableEntity},
		{service.ErrVendorPackageCodeConflict, http.StatusConflict},
		{service.ErrMasterDataChangePending, http.StatusConflict},
	}
	for _, tc := range cases {
		router, tokenSvc := mountAdminVendorPackageHandler(&fakeVendorPackageAdminServicer{createErr: tc.err})
		rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/packages", tokenForRole(t, tokenSvc, 1, "ADMIN"), `{"code":"PKG1"}`)
		if rec.Code != tc.want {
			t.Errorf("err %v: expected %d, got %d", tc.err, tc.want, rec.Code)
		}
	}
}

func TestAdminVendorPackageHandler_WrongRole_Forbidden(t *testing.T) {
	router, tokenSvc := mountAdminVendorPackageHandler(&fakeVendorPackageAdminServicer{})
	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/packages", tokenForRole(t, tokenSvc, 1, "ATM-USER"), "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestAdminVendorPackageHandler_Toggle_Returns202(t *testing.T) {
	router, tokenSvc := mountAdminVendorPackageHandler(&fakeVendorPackageAdminServicer{})
	for _, path := range []string{"/disable", "/enable"} {
		rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/packages/1"+path, tokenForRole(t, tokenSvc, 1, "ADMIN_PARAM"), "")
		if rec.Code != http.StatusAccepted {
			t.Errorf("%s: expected 202, got %d", path, rec.Code)
		}
	}
}
