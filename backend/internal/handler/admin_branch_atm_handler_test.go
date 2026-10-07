package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// fakeBranchATMServicer exercises HTTP wiring only; dedup/scope/pagination
// logic is covered by internal/service/branch_atm_test.go.
type fakeBranchATMServicer struct {
	result service.ListManagedATMsResult
	err    error
}

func (f *fakeBranchATMServicer) List(context.Context, int64, int64, int64, int64) (service.ListManagedATMsResult, error) {
	return f.result, f.err
}

func mountAdminBranchATMHandler(svc BranchATMServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM"),
	).Mount("/api/v1/admin/vendors/{vendorID}/branches/{branchID}/atms", NewAdminBranchATMHandler(svc).Routes())
	return r, tokenSvc
}

func TestAdminBranchATMHandler_List_NoToken_Unauthorized(t *testing.T) {
	router, _ := mountAdminBranchATMHandler(&fakeBranchATMServicer{})
	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/branches/9/atms", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminBranchATMHandler_List_WrongRole_Forbidden(t *testing.T) {
	router, tokenSvc := mountAdminBranchATMHandler(&fakeBranchATMServicer{})
	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/branches/9/atms", tokenForRole(t, tokenSvc, 1, "ATM-USER"), "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminBranchATMHandler_List_BadPathID(t *testing.T) {
	router, tokenSvc := mountAdminBranchATMHandler(&fakeBranchATMServicer{})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")
	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/abc/branches/9/atms", token, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad vendorID, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/branches/abc/atms", token, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad branchID, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminBranchATMHandler_List_BadPageSize(t *testing.T) {
	router, tokenSvc := mountAdminBranchATMHandler(&fakeBranchATMServicer{})
	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/branches/9/atms?page_size=abc",
		tokenForRole(t, tokenSvc, 1, "ADMIN"), "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminBranchATMHandler_List_BranchNotFound(t *testing.T) {
	router, tokenSvc := mountAdminBranchATMHandler(&fakeBranchATMServicer{err: service.ErrBranchNotFound})
	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/branches/9/atms",
		tokenForRole(t, tokenSvc, 1, "ADMIN"), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminBranchATMHandler_List_EmptyResult(t *testing.T) {
	router, tokenSvc := mountAdminBranchATMHandler(&fakeBranchATMServicer{
		result: service.ListManagedATMsResult{ATMs: nil, Total: 0},
	})
	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/branches/9/atms",
		tokenForRole(t, tokenSvc, 1, "ADMIN"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"atms":[]`) || !strings.Contains(body, `"total":0`) {
		t.Fatalf("expected empty atms + total 0, got: %s", body)
	}
}

func TestAdminBranchATMHandler_List_WithRows(t *testing.T) {
	locName := "Kantor Cabang Menteng"
	locCity := "Jakarta Pusat"
	priority := "VIP"
	svc := &fakeBranchATMServicer{
		result: service.ListManagedATMsResult{
			ATMs: []service.ManagedATM{
				{
					ATMID: 4021, TerminalID: "ATM00412", LocationName: &locName,
					LocationCityOrRegency: &locCity, PriorityClass: &priority,
					IsActive: true, PackageCode: "PKG-JKT-01",
				},
				{
					ATMID: 4022, TerminalID: "ATM00413", LocationName: nil,
					LocationCityOrRegency: nil, PriorityClass: nil,
					IsActive: false, PackageCode: "PKG-JKT-02",
				},
			},
			Total: 2,
		},
	}
	router, tokenSvc := mountAdminBranchATMHandler(svc)
	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/branches/9/atms?page=1&page_size=200",
		tokenForRole(t, tokenSvc, 1, "ADMIN"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`"atm_id":4021`, `"terminal_id":"ATM00412"`, `"location_name":"Kantor Cabang Menteng"`,
		`"package_code":"PKG-JKT-01"`, `"priority_class":"VIP"`, `"is_active":true`,
		`"atm_id":4022`, `"location_name":null`, `"priority_class":null`, `"is_active":false`,
		`"total":2`, `"page":1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected body to contain %q, got: %s", want, body)
		}
	}
	// page_size clamped to the endpoint maximum (100).
	if !strings.Contains(body, `"page_size":100`) {
		t.Errorf("expected page_size clamped to 100, got: %s", body)
	}
	if strings.Contains(body, `"data":`) || strings.Contains(body, `"success":`) {
		t.Errorf("expected flat JSON (no pkg/response envelope), got: %s", body)
	}
}
