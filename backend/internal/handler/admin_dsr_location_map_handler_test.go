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

// fakeDsrLocationMapServicer exercises HTTP wiring only; logic is covered by
// internal/service/dsr_location_map_admin_test.go.
type fakeDsrLocationMapServicer struct{ err error }

func (f *fakeDsrLocationMapServicer) List(context.Context, int64, string) ([]db.ListDsrLocationMapsAdminRow, error) {
	return []db.ListDsrLocationMapsAdminRow{{ID: 1, DsrLocation: "BINTARO", VaultCode: "V1", IsActive: true}}, nil
}
func (f *fakeDsrLocationMapServicer) ListUnmapped(context.Context, int64) ([]db.ListUnmappedDsrLocationsRow, error) {
	return []db.ListUnmappedDsrLocationsRow{{DsrLocation: "CIBUBUR"}}, nil
}
func (f *fakeDsrLocationMapServicer) Create(context.Context, int64, int64, service.DsrLocationMapPayload, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 7, Status: "pending"}, f.err
}
func (f *fakeDsrLocationMapServicer) Update(context.Context, int64, int64, int64, service.DsrLocationMapUpdatePayload, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 8, Status: "pending"}, f.err
}
func (f *fakeDsrLocationMapServicer) Disable(context.Context, int64, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 9, Status: "pending"}, f.err
}
func (f *fakeDsrLocationMapServicer) Enable(context.Context, int64, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 10, Status: "pending"}, f.err
}

func mountDsrLocationMapHandler(svc DsrLocationMapAdminServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(tokenSvc), custommw.RequireRoles("ADMIN", "ADMIN_PARAM")).
		Mount("/api/v1/admin/vendors/{vendorID}/dsr-location-maps", NewAdminDsrLocationMapHandler(svc).Routes())
	return r, tokenSvc
}

func TestAdminDsrLocationMapHandler(t *testing.T) {
	const base = "/api/v1/admin/vendors/3/dsr-location-maps"
	cases := []struct {
		name, role, method, path, body string
		svcErr                         error
		wantCode                       int
		wantBody                       string
	}{
		{"list", "ADMIN", http.MethodGet, base, "", nil, http.StatusOK, `"dsr_location":"BINTARO"`},
		{"unmapped", "ADMIN_PARAM", http.MethodGet, base + "/unmapped", "", nil, http.StatusOK, `"dsr_location":"CIBUBUR"`},
		{"create 202", "ADMIN", http.MethodPost, base, `{"dsr_location":"X","vendor_vault_id":9}`, nil, http.StatusAccepted, ""},
		{"update 202", "ADMIN", http.MethodPut, base + "/5", `{"vendor_vault_id":9}`, nil, http.StatusAccepted, ""},
		{"disable 202", "ADMIN", http.MethodPost, base + "/5/disable", "", nil, http.StatusAccepted, ""},
		{"non-admin role denied at route", "ATM-USER", http.MethodPost, base, `{}`, nil, http.StatusForbidden, ""},
		{"service forbidden", "ADMIN", http.MethodPost, base, `{}`, service.ErrMasterDataForbidden, http.StatusForbidden, ""},
		{"duplicate label 409", "ADMIN", http.MethodPost, base, `{}`, service.ErrDsrLocationMapConflict, http.StatusConflict, ""},
		{"other vendor's id 404", "ADMIN", http.MethodPost, base + "/5/enable", "", service.ErrDsrLocationMapNotFound, http.StatusNotFound, ""},
		{"validation error names field", "ADMIN", http.MethodPost, base, `{}`, &service.ValidationError{Field: "vendor_vault_id", Message: "wajib diisi"}, 0, `vendor_vault_id`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, tokenSvc := mountDsrLocationMapHandler(&fakeDsrLocationMapServicer{err: tc.svcErr})
			rec := doRequest(router, tc.method, tc.path, tokenForRole(t, tokenSvc, 1, tc.role), tc.body)
			if tc.wantCode != 0 && rec.Code != tc.wantCode {
				t.Fatalf("code = %d, want %d: %s", rec.Code, tc.wantCode, rec.Body.String())
			}
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Errorf("body %s does not contain %s", rec.Body.String(), tc.wantBody)
			}
		})
	}
}
