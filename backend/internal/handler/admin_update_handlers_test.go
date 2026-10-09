package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
)

// PUT /{id} on the master-data admin handlers stages an update for approval
// (202); a broken id or body is a 400 before the service is reached.
func TestAdminMasterDataUpdate_Returns202And400(t *testing.T) {
	type mount func() (http.Handler, *pkgauth.TokenService)
	cases := []struct {
		name  string
		mount mount
		base  string
	}{
		{"vault", func() (http.Handler, *pkgauth.TokenService) {
			return mountAdminVendorVaultHandler(&fakeVendorVaultAdminServicer{})
		}, "/api/v1/admin/vendors/3/vaults"},
		{"pic", func() (http.Handler, *pkgauth.TokenService) {
			return mountAdminVendorPicHandler(&fakeVendorPicAdminServicer{})
		}, "/api/v1/admin/vendors/3/pics"},
		{"branch", func() (http.Handler, *pkgauth.TokenService) {
			return mountAdminVendorBranchHandler(&fakeVendorBranchAdminServicer{updateResult: db.MasterDataChangeRequest{ID: 8, Status: "pending"}})
		}, "/api/v1/admin/vendors/3/branches"},
		{"atm assignment", func() (http.Handler, *pkgauth.TokenService) {
			return mountAdminATMAssignmentHandler(&fakeATMAssignmentAdminServicer{})
		}, "/api/v1/admin/atms/4/assignments"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, ts := tc.mount()
			token := tokenForRole(t, ts, 1, "ADMIN_PARAM")
			if rec := doRequest(router, http.MethodPut, tc.base+"/1", token, `{}`); rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"change_request_id"`) {
				t.Errorf("update = %d %s, want 202", rec.Code, rec.Body.String())
			}
			for path, body := range map[string]string{tc.base + "/1": `{bad`, tc.base + "/x": `{}`} {
				if rec := doRequest(router, http.MethodPut, path, token, body); rec.Code != http.StatusBadRequest {
					t.Errorf("PUT %s %s = %d, want 400", path, body, rec.Code)
				}
			}
			if rec := doRequest(router, http.MethodPut, tc.base+"/1", tokenForRole(t, ts, 1, "ATM-USER"), `{}`); rec.Code != http.StatusForbidden {
				t.Errorf("ATM-USER update = %d, want 403", rec.Code)
			}
		})
	}
}

func TestAdminVendorBranchHandler_UpdateAndEnableErrors(t *testing.T) {
	svc := &fakeVendorBranchAdminServicer{updateErr: service.ErrMasterDataChangePending, enableErr: service.ErrMasterDataChangePending}
	router, ts := mountAdminVendorBranchHandler(svc)
	token := tokenForRole(t, ts, 1, "ADMIN")
	if rec := doRequest(router, http.MethodPut, "/api/v1/admin/vendors/3/branches/1", token, `{}`); rec.Code != http.StatusConflict {
		t.Errorf("update pending = %d, want 409", rec.Code)
	}
	if rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/branches/1/enable", token, ""); rec.Code != http.StatusConflict {
		t.Errorf("enable pending = %d, want 409", rec.Code)
	}
	svc.enableErr = nil
	if rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/branches/1/enable", token, ""); rec.Code != http.StatusAccepted {
		t.Errorf("enable = %d, want 202", rec.Code)
	}
}

func TestVendorRequestHandler_UpdateItemsAndRevise(t *testing.T) {
	svc := &fakeVendorRequestServicer{
		updateResult: &service.VendorRequestDetail{ID: 1, Status: "draft"},
		reviseResult: &service.VendorRequestDetail{ID: 1, Status: "draft"},
	}
	router, ts := mountVendorRequestHandler(t, svc)
	maker := vendorRequestTokenFor(t, ts, 5, "ATM-USER")
	items := `{"items":[{"terminal_id":"T001","periode_pred":"2026-10-09","denom":100000,"amount_replenish":1000000}]}`

	if rec := doRequest(router, http.MethodPut, "/api/v1/vendor-requests/1/items", maker, items); rec.Code != http.StatusOK {
		t.Fatalf("update items = %d %s", rec.Code, rec.Body.String())
	}
	for _, tc := range []struct{ name, path, body string }{
		{"bad body", "/api/v1/vendor-requests/1/items", `{bad`},
		{"bad id", "/api/v1/vendor-requests/x/items", items},
		{"periode_pred required", "/api/v1/vendor-requests/1/items", `{"items":[{"terminal_id":"T001","denom":100000,"amount_replenish":1}]}`},
	} {
		if rec := doRequest(router, http.MethodPut, tc.path, maker, tc.body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", tc.name, rec.Code)
		}
	}
	svc.updateErr = service.ErrNotCreator
	if rec := doRequest(router, http.MethodPut, "/api/v1/vendor-requests/1/items", maker, items); rec.Code != http.StatusForbidden {
		t.Errorf("not creator = %d, want 403", rec.Code)
	}
	if rec := doRequest(router, http.MethodPut, "/api/v1/vendor-requests/1/items", vendorRequestTokenFor(t, ts, 5, "ATM-SPV"), items); rec.Code != http.StatusForbidden {
		t.Errorf("checker editing items = %d, want 403 (route RBAC)", rec.Code)
	}

	if rec := doRequest(router, http.MethodPost, "/api/v1/vendor-requests/1/revise", maker, ""); rec.Code != http.StatusOK {
		t.Errorf("revise = %d %s", rec.Code, rec.Body.String())
	}
	svc.reviseErr = service.ErrInvalidTransition
	if rec := doRequest(router, http.MethodPost, "/api/v1/vendor-requests/1/revise", maker, ""); rec.Code != http.StatusConflict {
		t.Errorf("revise wrong state = %d, want 409", rec.Code)
	}
}
