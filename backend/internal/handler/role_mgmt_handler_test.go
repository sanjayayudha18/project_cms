package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/rolemgmt"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// fakeRoleMgmtServicer implements RoleMgmtServicer for httptest, no real DB
// needed -- the underlying logic is covered by
// backend/internal/rolemgmt/service_integration_test.go; this only exercises
// HTTP wiring and error-code mapping.
type fakeRoleMgmtServicer struct {
	listRolesResult []db.ListRolesWithPermissionsRow
	listRolesErr    error

	listCatalogResult []db.MenuFeature
	listCatalogErr    error

	createResult db.Role
	createErr    error

	updateResult []int64
	updateErr    error
}

func (f *fakeRoleMgmtServicer) ListRoles(context.Context) ([]db.ListRolesWithPermissionsRow, error) {
	return f.listRolesResult, f.listRolesErr
}

func (f *fakeRoleMgmtServicer) ListCatalog(context.Context) ([]db.MenuFeature, error) {
	return f.listCatalogResult, f.listCatalogErr
}

func (f *fakeRoleMgmtServicer) CreateRole(context.Context, int64, string, rolemgmt.CreateRoleRequest, string) (db.Role, error) {
	return f.createResult, f.createErr
}

func (f *fakeRoleMgmtServicer) UpdateRolePermissions(context.Context, int64, string, int64, []int64, string) ([]int64, error) {
	return f.updateResult, f.updateErr
}

// mountRoleMgmtHandler mirrors the real mount in cmd/api/main.go: RequireAuth
// + RequireRoles("APPACCESS", "ADMIN").
func mountRoleMgmtHandler(svc RoleMgmtServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})

	h := NewRoleMgmtHandler(svc)
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		custommw.RequireRoles("APPACCESS", "ADMIN"),
	).Mount("/api/v1/admin/roles", h.Routes())
	return r, tokenSvc
}

func TestRoleMgmtHandler_ListRoles_GroupsPermissionsByRole(t *testing.T) {
	key := "dashboard"
	label := "Dashboard"
	kind := "menu"
	menuFeatureID := int64(10)

	svc := &fakeRoleMgmtServicer{
		listRolesResult: []db.ListRolesWithPermissionsRow{
			{RoleID: 1, Role: "ADMIN", MenuFeatureID: &menuFeatureID, MenuFeatureKey: &key, MenuFeatureLabel: &label, MenuFeatureKind: &kind},
			{RoleID: 2, Role: "AUDITOR", MenuFeatureID: nil, MenuFeatureKey: nil, MenuFeatureLabel: nil, MenuFeatureKind: nil},
		},
	}
	router, tokenSvc := mountRoleMgmtHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/roles", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"role":"ADMIN"`) || !strings.Contains(body, `"key":"dashboard"`) {
		t.Errorf("expected ADMIN's granted entry in body, got: %s", body)
	}
	if !strings.Contains(body, `"role":"AUDITOR"`) || !strings.Contains(body, `"permissions":[]`) {
		t.Errorf("expected AUDITOR with empty permissions array (not a null-entry array), got: %s", body)
	}
}

func TestRoleMgmtHandler_ListCatalog_HappyPath(t *testing.T) {
	svc := &fakeRoleMgmtServicer{
		listCatalogResult: []db.MenuFeature{{ID: 1, Key: "settings", Label: "Pengaturan", Kind: "menu"}},
	}
	router, tokenSvc := mountRoleMgmtHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/roles/catalog", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"key":"settings"`) {
		t.Errorf("expected catalog entry in body, got: %s", rec.Body.String())
	}
}

func TestRoleMgmtHandler_CreateRole_HappyPath(t *testing.T) {
	desc := "test"
	svc := &fakeRoleMgmtServicer{createResult: db.Role{ID: 5, Role: "AUDITOR", Description: &desc}}
	router, tokenSvc := mountRoleMgmtHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/roles", token, `{"role":"AUDITOR","description":"test"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"role":"AUDITOR"`) {
		t.Errorf("expected created role in body, got: %s", rec.Body.String())
	}
}

func TestRoleMgmtHandler_CreateRole_ErrorMapping(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"validation", &rolemgmt.ValidationError{Field: "role", Message: "wajib diisi"}, http.StatusUnprocessableEntity, "validation_error"},
		{"name conflict", rolemgmt.ErrRoleNameConflict, http.StatusConflict, "conflict"},
		{"not authorized", rolemgmt.ErrNotAuthorized, http.StatusForbidden, "forbidden"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeRoleMgmtServicer{createErr: tc.err}
			router, tokenSvc := mountRoleMgmtHandler(svc)
			token := tokenForRole(t, tokenSvc, 1, "ADMIN")

			rec := doRequest(router, http.MethodPost, "/api/v1/admin/roles", token, `{"role":"X"}`)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tc.wantStatus, rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `"error":"`+tc.wantCode+`"`) {
				t.Errorf("expected error code %q in body, got: %s", tc.wantCode, rec.Body.String())
			}
		})
	}
}

func TestRoleMgmtHandler_UpdateRolePermissions_HappyPath(t *testing.T) {
	svc := &fakeRoleMgmtServicer{updateResult: []int64{1, 2}}
	router, tokenSvc := mountRoleMgmtHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/roles/7/permissions", token, `{"menu_feature_ids":[1,2]}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"role_id":7`) || !strings.Contains(body, `"permissions":[1,2]`) {
		t.Errorf("expected role_id and permissions in body, got: %s", body)
	}
}

func TestRoleMgmtHandler_UpdateRolePermissions_ErrorMapping(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"invalid catalog ref", rolemgmt.ErrCatalogEntryNotFound, http.StatusBadRequest, "invalid_reference"},
		{"role not found", rolemgmt.ErrRoleNotFound, http.StatusNotFound, "not_found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeRoleMgmtServicer{updateErr: tc.err}
			router, tokenSvc := mountRoleMgmtHandler(svc)
			token := tokenForRole(t, tokenSvc, 1, "ADMIN")

			rec := doRequest(router, http.MethodPut, "/api/v1/admin/roles/999/permissions", token, `{"menu_feature_ids":[1]}`)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tc.wantStatus, rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `"error":"`+tc.wantCode+`"`) {
				t.Errorf("expected error code %q in body, got: %s", tc.wantCode, rec.Body.String())
			}
		})
	}
}

// TestRoleMgmtHandler_RBAC is the RBAC gate test (pattern of
// admin_approval_handler_test.go / audit_log_handler_rbac_test.go): only
// APPACCESS/ADMIN reach the handler; every other role gets 403 from
// RequireRoles before the servicer is ever called.
func TestRoleMgmtHandler_RBAC(t *testing.T) {
	cases := []struct {
		role    string
		allowed bool
	}{
		{"APPACCESS", true},
		{"ADMIN", true},
		{"ADMIN_PARAM", false},
		{"ATM-USER", false},
		{"VENDOR-USER", false},
	}
	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			svc := &fakeRoleMgmtServicer{}
			router, tokenSvc := mountRoleMgmtHandler(svc)
			token := tokenForRole(t, tokenSvc, 1, tc.role)

			rec := doRequest(router, http.MethodGet, "/api/v1/admin/roles", token, "")

			if tc.allowed {
				if rec.Code != http.StatusOK {
					t.Fatalf("role %q: expected 200, got %d: %s", tc.role, rec.Code, rec.Body.String())
				}
				return
			}
			if rec.Code != http.StatusForbidden {
				t.Fatalf("role %q: expected 403, got %d: %s", tc.role, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRoleMgmtHandler_NoToken_Unauthorized(t *testing.T) {
	router, _ := mountRoleMgmtHandler(&fakeRoleMgmtServicer{})
	rec := doRequest(router, http.MethodGet, "/api/v1/admin/roles", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}
