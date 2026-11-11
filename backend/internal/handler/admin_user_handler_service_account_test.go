package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/pkg/userrules"
)

// ext-user-api task 3.5 (Property 4, Req 9.5): a service account is invisible
// to AdminUserAPI -- all five by-id endpoints answer 404 not_found and never
// reach a write service.
func TestAdminUserHandler_ServiceAccount_NotFound(t *testing.T) {
	svcRow := &db.GetUserAdminByIDRow{
		ID: 7, Username: "svc-idm", FullName: "svc", Email: "svc@example.com",
		Role: userrules.ServiceRole, AuthSource: userrules.AuthSourceService, IsActive: true,
	}
	cases := []struct {
		name, method, path, body string
	}{
		{"get", http.MethodGet, "/api/v1/admin/users/7", ""},
		{"update", http.MethodPut, "/api/v1/admin/users/7", `{"full_name":"X","email":"x@example.com","role":"APPACCESS"}`},
		{"disable", http.MethodPost, "/api/v1/admin/users/7/disable", ""},
		{"enable", http.MethodPost, "/api/v1/admin/users/7/enable", ""},
		{"set-initial-password", http.MethodPost, "/api/v1/admin/users/7/set-initial-password", `{"new_password":"InitialPass1"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pwSvc := &fakeSetInitialPasswordService{}
			userSvc := &fakeUserAdminServicer{getResult: svcRow}
			deactSvc := &fakeDeactivateService{}
			router, tokenSvc := mountAdminUserHandler(pwSvc, userSvc, deactSvc)
			token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

			rec := doRequest(router, tc.method, tc.path, token, tc.body)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "not_found") {
				t.Errorf("body should carry code not_found: %s", rec.Body.String())
			}
			if userSvc.updateCalled || deactSvc.deactivateCalled || deactSvc.reactivateCalled || pwSvc.calledTargetID != 0 {
				t.Error("a write service was reached for a service account")
			}
		})
	}
}
