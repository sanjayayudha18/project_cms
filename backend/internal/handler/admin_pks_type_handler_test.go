package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// Spec vendor-pks-type task 3.4 (handler): HTTP wiring of /admin/pks-types only;
// the rules live in internal/service/pks_type_admin_test.go.

type fakePksTypeServicer struct{ err error }

func (f *fakePksTypeServicer) List(context.Context) ([]db.ListPksTypesRow, error) {
	return []db.ListPksTypesRow{{
		ID: 1, Code: "FLM", Name: "FLM Vendor", IsActive: true, InUse: true,
		CreatedAt: pgtype.Timestamptz{Time: time.Date(2026, 11, 18, 1, 0, 0, 0, time.UTC), Valid: true},
	}}, f.err
}
func (f *fakePksTypeServicer) Create(context.Context, int64, service.PksTypeCreatePayload, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 7, Status: "pending"}, f.err
}
func (f *fakePksTypeServicer) Update(context.Context, int64, int64, service.PksTypeUpdatePayload, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 8, Status: "pending"}, f.err
}
func (f *fakePksTypeServicer) Disable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 9, Status: "pending"}, f.err
}
func (f *fakePksTypeServicer) Enable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 10, Status: "pending"}, f.err
}

func mountPksTypeHandler(svc PksTypeAdminServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(tokenSvc), custommw.RequireRoles("ADMIN", "ADMIN_PARAM")).
		Mount("/api/v1/admin/pks-types", NewAdminPksTypeHandler(svc).Routes())
	return r, tokenSvc
}

func TestAdminPksTypeHandler(t *testing.T) {
	const base = "/api/v1/admin/pks-types"
	cases := []struct {
		name, role, method, path, body string
		svcErr                         error
		wantCode                       int
		wantBody                       string
	}{
		{"list is a flat JSON with in_use", "ADMIN", http.MethodGet, base, "", nil, http.StatusOK, `"in_use":true`},
		{"list as ADMIN_PARAM", "ADMIN_PARAM", http.MethodGet, base, "", nil, http.StatusOK, `"code":"FLM"`},
		{"create 202", "ADMIN", http.MethodPost, base, `{"code":"ATM1","name":"ATM"}`, nil, http.StatusAccepted, ""},
		{"update 202", "ADMIN", http.MethodPut, base + "/5", `{"name":"Baru"}`, nil, http.StatusAccepted, ""},
		{"disable 202", "ADMIN", http.MethodPost, base + "/5/disable", "", nil, http.StatusAccepted, ""},
		{"enable 202", "ADMIN", http.MethodPost, base + "/5/enable", "", nil, http.StatusAccepted, ""},
		{"non-admin role denied at route (list)", "ATM-USER", http.MethodGet, base, "", nil, http.StatusForbidden, ""},
		{"non-admin role denied at route (create)", "VENDOR-USER", http.MethodPost, base, `{}`, nil, http.StatusForbidden, ""},
		{"service forbidden -> 403", "ADMIN", http.MethodPost, base, `{}`, service.ErrMasterDataForbidden, http.StatusForbidden, ""},
		{"duplicate code -> 409", "ADMIN", http.MethodPost, base, `{}`, service.ErrPksTypeConflict, http.StatusConflict, ""},
		{"disable in use -> 409", "ADMIN", http.MethodPost, base + "/5/disable", "", service.ErrPksTypeInUse, http.StatusConflict, ""},
		{"pending change -> 409", "ADMIN", http.MethodPut, base + "/5", `{"name":"x"}`, service.ErrMasterDataChangePending, http.StatusConflict, ""},
		{"unknown id -> 404", "ADMIN", http.MethodPost, base + "/5/enable", "", service.ErrPksTypeNotFound, http.StatusNotFound, ""},
		{"validation error names the field", "ADMIN", http.MethodPost, base, `{}`, &service.ValidationError{Field: "code", Message: "wajib"}, 0, `code`},
		{"malformed body -> 400", "ADMIN", http.MethodPost, base, `{not json`, nil, http.StatusBadRequest, ""},
		{"non numeric id -> 400", "ADMIN", http.MethodPost, base + "/abc/disable", "", nil, http.StatusBadRequest, ""},
		{"unexpected error -> 500", "ADMIN", http.MethodPost, base + "/5/disable", "", context.DeadlineExceeded, http.StatusInternalServerError, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, tokenSvc := mountPksTypeHandler(&fakePksTypeServicer{err: tc.svcErr})
			rec := doRequest(router, tc.method, tc.path, tokenForRole(t, tokenSvc, 1, tc.role), tc.body)
			if tc.wantCode != 0 && rec.Code != tc.wantCode {
				t.Fatalf("code = %d, want %d: %s", rec.Code, tc.wantCode, rec.Body.String())
			}
			if tc.wantCode == 0 && rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("code = %d, want 422: %s", rec.Code, rec.Body.String())
			}
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Errorf("body %s does not contain %s", rec.Body.String(), tc.wantBody)
			}
		})
	}
}

func TestAdminPksTypeHandler_NoToken_401(t *testing.T) {
	router, _ := mountPksTypeHandler(&fakePksTypeServicer{})
	for _, m := range []string{http.MethodGet, http.MethodPost} {
		if rec := doRequest(router, m, "/api/v1/admin/pks-types", "", `{}`); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s without token: code = %d, want 401", m, rec.Code)
		}
	}
}
