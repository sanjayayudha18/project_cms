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

// fakeVendorBranchAdminServicer implements VendorBranchAdminServicer for
// httptest, no real DB needed -- the underlying logic is covered by
// backend/internal/service/vendor_branch_admin_test.go; this only exercises
// HTTP wiring.
type fakeVendorBranchAdminServicer struct {
	listResult  []db.ListVendorBranchesAdminRow
	listErr     error
	countResult int64
	countErr    error
	getResult   *db.GetVendorBranchAdminByIDRow
	getErr      error

	createResult db.MasterDataChangeRequest
	createErr    error
	updateResult db.MasterDataChangeRequest
	updateErr    error
	disableErr   error
	enableErr    error
}

func (f *fakeVendorBranchAdminServicer) List(context.Context, db.ListVendorBranchesAdminParams) ([]db.ListVendorBranchesAdminRow, error) {
	return f.listResult, f.listErr
}

func (f *fakeVendorBranchAdminServicer) Count(context.Context, db.CountVendorBranchesAdminParams) (int64, error) {
	return f.countResult, f.countErr
}

func (f *fakeVendorBranchAdminServicer) Get(context.Context, int64) (*db.GetVendorBranchAdminByIDRow, error) {
	return f.getResult, f.getErr
}

func (f *fakeVendorBranchAdminServicer) Create(context.Context, int64, service.VendorBranchPayload, string) (db.MasterDataChangeRequest, error) {
	return f.createResult, f.createErr
}

func (f *fakeVendorBranchAdminServicer) Update(context.Context, int64, int64, service.VendorBranchUpdatePayload, string) (db.MasterDataChangeRequest, error) {
	return f.updateResult, f.updateErr
}

func (f *fakeVendorBranchAdminServicer) Disable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 1, Status: "pending"}, f.disableErr
}

func (f *fakeVendorBranchAdminServicer) Enable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 1, Status: "pending"}, f.enableErr
}

func mountAdminVendorBranchHandler(svc VendorBranchAdminServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})

	h := NewAdminVendorBranchHandler(svc)
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM"),
	).Mount("/api/v1/admin/vendors/{vendorID}/branches", h.Routes())
	return r, tokenSvc
}

func TestAdminVendorBranchHandler_List_HappyPath(t *testing.T) {
	svc := &fakeVendorBranchAdminServicer{
		listResult:  []db.ListVendorBranchesAdminRow{{ID: 1, VendorID: 3, BranchCode: "BR1", BranchName: "Branch One", IsActive: true}},
		countResult: 1,
	}
	router, tokenSvc := mountAdminVendorBranchHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/branches?page=1&page_size=10", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"total":1`) || !strings.Contains(body, `"branch_code":"BR1"`) {
		t.Errorf("expected branch list in body, got: %s", body)
	}
}

func TestAdminVendorBranchHandler_List_InvalidVendorID_BadRequest(t *testing.T) {
	router, tokenSvc := mountAdminVendorBranchHandler(&fakeVendorBranchAdminServicer{})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/not-a-number/branches", token, "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminVendorBranchHandler_List_NoAuth_Unauthorized(t *testing.T) {
	router, _ := mountAdminVendorBranchHandler(&fakeVendorBranchAdminServicer{})

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/branches", "", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminVendorBranchHandler_List_WrongRole_Forbidden(t *testing.T) {
	router, tokenSvc := mountAdminVendorBranchHandler(&fakeVendorBranchAdminServicer{})
	token := tokenForRole(t, tokenSvc, 1, "ATM-USER")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/branches", token, "")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminVendorBranchHandler_Get_NotFound(t *testing.T) {
	svc := &fakeVendorBranchAdminServicer{getResult: nil}
	router, tokenSvc := mountAdminVendorBranchHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/branches/999", token, "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminVendorBranchHandler_Create_HappyPath_Returns202WithChangeRequest(t *testing.T) {
	svc := &fakeVendorBranchAdminServicer{
		createResult: db.MasterDataChangeRequest{ID: 42, EntityType: "vendor_branch", Op: "create", Status: "pending"},
	}
	router, tokenSvc := mountAdminVendorBranchHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/branches", token, `{"branch_code":"BR1","branch_name":"Branch One"}`)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"change_request_id":42`) || !strings.Contains(body, `"status":"pending"`) {
		t.Errorf("expected pending change request body, got: %s", body)
	}
}

func TestAdminVendorBranchHandler_Create_ServiceConflict_MapsTo409(t *testing.T) {
	svc := &fakeVendorBranchAdminServicer{createErr: service.ErrVendorBranchCodeConflict}
	router, tokenSvc := mountAdminVendorBranchHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/branches", token, `{"branch_code":"BR1","branch_name":"Branch One"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminVendorBranchHandler_Create_ValidationError_MapsTo400(t *testing.T) {
	svc := &fakeVendorBranchAdminServicer{createErr: &service.ValidationError{Field: "branch_code", Message: "wajib diisi"}}
	router, tokenSvc := mountAdminVendorBranchHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/branches", token, `{"branch_name":"Branch One"}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminVendorBranchHandler_Disable_HappyPath_Returns202(t *testing.T) {
	router, tokenSvc := mountAdminVendorBranchHandler(&fakeVendorBranchAdminServicer{})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN_PARAM")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/branches/1/disable", token, "")

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminVendorBranchHandler_Disable_PendingConflict_MapsTo409(t *testing.T) {
	svc := &fakeVendorBranchAdminServicer{disableErr: service.ErrMasterDataChangePending}
	router, tokenSvc := mountAdminVendorBranchHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/branches/1/disable", token, "")

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}
