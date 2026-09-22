package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// fakeATMAssignmentAdminServicer exercises HTTP wiring only; logic is covered
// by internal/service/atm_assignment_admin_test.go.
type fakeATMAssignmentAdminServicer struct {
	listResult []service.ATMAssignment
	getResult  *service.ATMAssignment
	createErr  error
}

func (f *fakeATMAssignmentAdminServicer) List(context.Context, db.ListATMAssignmentsAdminParams) ([]service.ATMAssignment, error) {
	return f.listResult, nil
}
func (f *fakeATMAssignmentAdminServicer) Count(context.Context, db.CountATMAssignmentsAdminParams) (int64, error) {
	return int64(len(f.listResult)), nil
}
func (f *fakeATMAssignmentAdminServicer) Get(context.Context, int64, int64) (*service.ATMAssignment, error) {
	return f.getResult, nil
}
func (f *fakeATMAssignmentAdminServicer) Create(context.Context, int64, int64, service.ATMAssignmentUpdatePayload, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 7, EntityType: "atm_assignment", Op: "create", Status: "pending"}, f.createErr
}
func (f *fakeATMAssignmentAdminServicer) Update(context.Context, int64, int64, int64, service.ATMAssignmentUpdatePayload, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 8, Status: "pending"}, nil
}
func (f *fakeATMAssignmentAdminServicer) Disable(context.Context, int64, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 9, Status: "pending"}, nil
}
func (f *fakeATMAssignmentAdminServicer) Enable(context.Context, int64, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 10, Status: "pending"}, nil
}

func assignmentTokenSvc() *pkgauth.TokenService {
	return pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
}

func mountAdminATMAssignmentHandler(svc ATMAssignmentAdminServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := assignmentTokenSvc()
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM"),
	).Mount("/api/v1/admin/atms/{atmID}/assignments", NewAdminATMAssignmentHandler(svc).Routes())
	return r, tokenSvc
}

func TestAdminATMAssignmentHandler_List_DatesAndOpenEnd(t *testing.T) {
	end := "2026-12-31"
	svc := &fakeATMAssignmentAdminServicer{listResult: []service.ATMAssignment{
		{ID: 1, ATMID: 3, VendorPackageID: 5, PackageCode: "PKG1", PriorityClass: "ALL", EffectiveStartDate: "2026-01-01", IsActive: true},
		{ID: 2, ATMID: 3, VendorPackageID: 5, PackageCode: "PKG1", PriorityClass: "ALL", EffectiveStartDate: "2025-01-01", EffectiveEndDate: &end},
	}}
	router, tokenSvc := mountAdminATMAssignmentHandler(svc)

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/atms/3/assignments", tokenForRole(t, tokenSvc, 1, "ADMIN"), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"effective_start_date":"2026-01-01"`) || !strings.Contains(body, `"effective_end_date":null`) || !strings.Contains(body, `"effective_end_date":"2026-12-31"`) {
		t.Errorf("expected ISO dates and null open end, got: %s", body)
	}
}

func TestAdminATMAssignmentHandler_Get_NotFound(t *testing.T) {
	router, tokenSvc := mountAdminATMAssignmentHandler(&fakeATMAssignmentAdminServicer{})
	rec := doRequest(router, http.MethodGet, "/api/v1/admin/atms/3/assignments/99", tokenForRole(t, tokenSvc, 1, "ADMIN"), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestAdminATMAssignmentHandler_Create_Returns202(t *testing.T) {
	router, tokenSvc := mountAdminATMAssignmentHandler(&fakeATMAssignmentAdminServicer{})
	rec := doRequest(router, http.MethodPost, "/api/v1/admin/atms/3/assignments", tokenForRole(t, tokenSvc, 1, "ADMIN"),
		`{"vendor_package_id":5,"effective_start_date":"2026-03-01","effective_end_date":null}`)
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"change_request_id":7`) {
		t.Fatalf("expected 202 with change_request_id, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminATMAssignmentHandler_Create_ErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{&service.ValidationError{Field: "effective_end_date", Message: "tidak boleh sebelum tanggal mulai"}, http.StatusUnprocessableEntity},
		{service.ErrATMAssignmentOverlap, http.StatusConflict},
		{service.ErrMasterDataChangePending, http.StatusConflict},
		{service.ErrAssignmentATMNotFound, http.StatusNotFound},
		{service.ErrATMAssignmentNotFound, http.StatusNotFound},
	}
	for _, tc := range cases {
		router, tokenSvc := mountAdminATMAssignmentHandler(&fakeATMAssignmentAdminServicer{createErr: tc.err})
		rec := doRequest(router, http.MethodPost, "/api/v1/admin/atms/3/assignments", tokenForRole(t, tokenSvc, 1, "ADMIN"), `{"vendor_package_id":5}`)
		if rec.Code != tc.want {
			t.Errorf("err %v: expected %d, got %d", tc.err, tc.want, rec.Code)
		}
	}
}

func TestAdminATMAssignmentHandler_Overlap_ResponseIsCleanMessageNotDBError(t *testing.T) {
	router, tokenSvc := mountAdminATMAssignmentHandler(&fakeATMAssignmentAdminServicer{createErr: service.ErrATMAssignmentOverlap})
	rec := doRequest(router, http.MethodPost, "/api/v1/admin/atms/3/assignments", tokenForRole(t, tokenSvc, 1, "ADMIN"), `{"vendor_package_id":5}`)
	body := rec.Body.String()
	if !strings.Contains(body, "tumpang tindih") || strings.Contains(body, "atm_vendor_packages_no_overlap") || strings.Contains(body, "23P01") {
		t.Errorf("want clear overlap message with no raw DB detail, got: %s", body)
	}
}

func TestAdminATMAssignmentHandler_WrongRole_Forbidden(t *testing.T) {
	router, tokenSvc := mountAdminATMAssignmentHandler(&fakeATMAssignmentAdminServicer{})
	rec := doRequest(router, http.MethodGet, "/api/v1/admin/atms/3/assignments", tokenForRole(t, tokenSvc, 1, "ATM-USER"), "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestAdminATMAssignmentHandler_Toggle_Returns202(t *testing.T) {
	router, tokenSvc := mountAdminATMAssignmentHandler(&fakeATMAssignmentAdminServicer{})
	for _, path := range []string{"/disable", "/enable"} {
		rec := doRequest(router, http.MethodPost, "/api/v1/admin/atms/3/assignments/1"+path, tokenForRole(t, tokenSvc, 1, "ADMIN_PARAM"), "")
		if rec.Code != http.StatusAccepted {
			t.Errorf("%s: expected 202, got %d", path, rec.Code)
		}
	}
}

// main.go mounts this router under /api/v1/admin/atms/{atmID}/assignments
// while the ATM CRUD router already owns /api/v1/admin/atms/*. This guards the
// wiring: chi must not panic on the second Mount, and each prefix must reach
// its own handler.
func TestAdminATMAssignmentHandler_CoMountedWithATMCrudRouter(t *testing.T) {
	tokenSvc := assignmentTokenSvc()
	guard := func(r chi.Router) chi.Router {
		return r.With(custommw.RequireAuth(tokenSvc), custommw.RequireRoles("ADMIN", "ADMIN_PARAM"))
	}
	atmCrud := chi.NewRouter()
	atmCrud.Get("/{id}", func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, "atm-crud") })

	r := chi.NewRouter()
	guard(r).Mount("/api/v1/admin/atms", atmCrud)
	guard(r).Mount("/api/v1/admin/atms/{atmID}/assignments", NewAdminATMAssignmentHandler(&fakeATMAssignmentAdminServicer{
		listResult: []service.ATMAssignment{{ID: 1, ATMID: 3}},
	}).Routes())

	token := tokenForRole(t, tokenSvc, 1, "ADMIN")
	if rec := doRequest(r, http.MethodGet, "/api/v1/admin/atms/3", token, ""); rec.Body.String() != "atm-crud" {
		t.Errorf("/atms/3 should reach the ATM CRUD router, got %d %q", rec.Code, rec.Body.String())
	}
	if rec := doRequest(r, http.MethodGet, "/api/v1/admin/atms/3/assignments", token, ""); !strings.Contains(rec.Body.String(), `"assignments"`) {
		t.Errorf("/atms/3/assignments should reach the assignment router, got %d %q", rec.Code, rec.Body.String())
	}
}

// T3.6: the service-layer RBAC recheck (ErrMasterDataForbidden from
// MasterDataChangeService.Submit) must reach the client as 403 from every
// master-data handler -- not a 500, and not a validation/conflict code.
func TestMasterDataHandlers_ServiceLayerForbidden_Returns403(t *testing.T) {
	forbidden := service.ErrMasterDataForbidden
	cases := []struct {
		name  string
		mount func() (http.Handler, *pkgauth.TokenService)
		path  string
	}{
		{"branches", func() (http.Handler, *pkgauth.TokenService) {
			return mountAdminVendorBranchHandler(&fakeVendorBranchAdminServicer{createErr: forbidden})
		}, "/api/v1/admin/vendors/3/branches"},
		{"vaults", func() (http.Handler, *pkgauth.TokenService) {
			return mountAdminVendorVaultHandler(&fakeVendorVaultAdminServicer{createErr: forbidden})
		}, "/api/v1/admin/vendors/3/vaults"},
		{"pics", func() (http.Handler, *pkgauth.TokenService) {
			return mountAdminVendorPicHandler(&fakeVendorPicAdminServicer{createErr: forbidden})
		}, "/api/v1/admin/vendors/3/pics"},
		{"packages", func() (http.Handler, *pkgauth.TokenService) {
			return mountAdminVendorPackageHandler(&fakeVendorPackageAdminServicer{createErr: forbidden})
		}, "/api/v1/admin/vendors/3/packages"},
		{"assignments", func() (http.Handler, *pkgauth.TokenService) {
			return mountAdminATMAssignmentHandler(&fakeATMAssignmentAdminServicer{createErr: forbidden})
		}, "/api/v1/admin/atms/3/assignments"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, tokenSvc := tc.mount()
			rec := doRequest(router, http.MethodPost, tc.path, tokenForRole(t, tokenSvc, 1, "ADMIN"), `{"name":"x","code":"x"}`)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// main.go mounts every master-data admin router on ONE shared guarded group
// (masterDataAdmin := r.With(RequireAuth, RequireRoles)). Guards the pattern:
// several Mounts on one inline group must all route correctly and all reject a
// non-admin -- the route-level half of T3.6's "RBAC at middleware AND service".
func TestMasterDataAdminSharedGroup_AllMountsGuardedAndRouted(t *testing.T) {
	tokenSvc := assignmentTokenSvc()
	r := chi.NewRouter()
	group := r.With(custommw.RequireAuth(tokenSvc), custommw.RequireRoles("ADMIN", "ADMIN_PARAM"))
	group.Mount("/api/v1/admin/vendors/{vendorID}/packages", NewAdminVendorPackageHandler(&fakeVendorPackageAdminServicer{}).Routes())
	group.Mount("/api/v1/admin/vendors/{vendorID}/pics", NewAdminVendorPicHandler(&fakeVendorPicAdminServicer{}).Routes())
	group.Mount("/api/v1/admin/atms/{atmID}/assignments", NewAdminATMAssignmentHandler(&fakeATMAssignmentAdminServicer{}).Routes())

	paths := []string{"/api/v1/admin/vendors/3/packages", "/api/v1/admin/vendors/3/pics", "/api/v1/admin/atms/3/assignments"}
	for _, p := range paths {
		if rec := doRequest(r, http.MethodGet, p, tokenForRole(t, tokenSvc, 1, "ADMIN"), ""); rec.Code != http.StatusOK {
			t.Errorf("ADMIN GET %s: expected 200, got %d: %s", p, rec.Code, rec.Body.String())
		}
		if rec := doRequest(r, http.MethodGet, p, tokenForRole(t, tokenSvc, 1, "ADMIN_PARAM"), ""); rec.Code != http.StatusOK {
			t.Errorf("ADMIN_PARAM GET %s: expected 200, got %d", p, rec.Code)
		}
		if rec := doRequest(r, http.MethodGet, p, tokenForRole(t, tokenSvc, 1, "ATM-USER"), ""); rec.Code != http.StatusForbidden {
			t.Errorf("ATM-USER GET %s: expected 403, got %d", p, rec.Code)
		}
		if rec := doRequest(r, http.MethodGet, p, "", ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("anonymous GET %s: expected 401, got %d", p, rec.Code)
		}
	}
}

// Apply-time failures surface through /api/v1/approvals as clean 409s (T3.5),
// not raw DB errors or a bare 500.
func TestApprovalHandler_HandleError_MasterDataApplyConflicts(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{fmt.Errorf("approved but apply failed (change request 1): %w", fmt.Errorf("apply change: %w", fmt.Errorf("create atm assignment: %w", service.ErrATMAssignmentOverlap))), http.StatusConflict},
		{fmt.Errorf("apply change: %w", service.ErrATMAssignmentDuplicate), http.StatusConflict},
		{fmt.Errorf("approved but apply failed (change request 4): %w", fmt.Errorf("apply change: %w", fmt.Errorf("create atm: %w", service.ErrATMTerminalIDConflict))), http.StatusConflict},
		{fmt.Errorf("approved but apply failed (change request 3): %w", fmt.Errorf("apply change: %w", fmt.Errorf("create vendor: %w", service.ErrVendorCodeConflict))), http.StatusConflict},
		{fmt.Errorf("approved but apply failed (change request 2): %w", service.ErrMasterDataChangeStale), http.StatusConflict},
		{errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		(&ApprovalHandler{}).handleError(rec, tc.err)
		if rec.Code != tc.want {
			t.Errorf("err %v: expected %d, got %d", tc.err, tc.want, rec.Code)
		}
	}
}
