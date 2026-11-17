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

// fakeCashCountAreaServicer exercises HTTP wiring only; logic is covered by
// internal/service/cash_count_area_admin_test.go.
type fakeCashCountAreaServicer struct{ err error }

func (f *fakeCashCountAreaServicer) List(context.Context, string) ([]db.ListCashCountAreasAdminRow, error) {
	return []db.ListCashCountAreasAdminRow{{ID: 1, Code: "JKT", Name: "Jakarta", IsActive: true, RegionCodes: []string{"JB"}}, {ID: 2, Code: "SBY", Name: "Surabaya"}}, nil
}
func (f *fakeCashCountAreaServicer) Create(context.Context, int64, service.CashCountAreaPayload, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 7, Status: "pending"}, f.err
}
func (f *fakeCashCountAreaServicer) Update(context.Context, int64, int64, service.CashCountAreaUpdatePayload, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 8, Status: "pending"}, f.err
}
func (f *fakeCashCountAreaServicer) Disable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 9, Status: "pending"}, f.err
}
func (f *fakeCashCountAreaServicer) Enable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 10, Status: "pending"}, f.err
}

func mountCashCountAreaHandler(svc CashCountAreaServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	r := chi.NewRouter()
	// Same guard shape 11.1 mounts: maker CC-CWM + ADMIN (read).
	r.With(custommw.RequireAuth(tokenSvc), custommw.RequireRoles("CC-CWM", "ADMIN")).
		Mount("/api/v1/cash-count/areas", NewCashCountAreaHandler(svc).Routes())
	return r, tokenSvc
}

func TestCashCountAreaHandler(t *testing.T) {
	const base = "/api/v1/cash-count/areas"
	cases := []struct {
		name, role, method, path, body string
		svcErr                         error
		wantCode                       int
		wantBody                       string
	}{
		{"list", "CC-CWM", http.MethodGet, base, "", nil, http.StatusOK, `"region_codes":["JB"]`},
		{"list empty regions is [] not null", "ADMIN", http.MethodGet, base + "?status=all", "", nil, http.StatusOK, `"region_codes":[]`},
		{"list bad status", "ADMIN", http.MethodGet, base + "?status=zzz", "", nil, http.StatusBadRequest, ""},
		{"create 202", "CC-CWM", http.MethodPost, base, `{"code":"JKT","name":"Jakarta","region_codes":["JB"]}`, nil, http.StatusAccepted, ""},
		{"create bad json", "CC-CWM", http.MethodPost, base, `{`, nil, http.StatusBadRequest, ""},
		{"update 202", "CC-CWM", http.MethodPut, base + "/5", `{"name":"X","region_codes":["JB"]}`, nil, http.StatusAccepted, ""},
		{"update bad id", "CC-CWM", http.MethodPut, base + "/abc", `{}`, nil, http.StatusBadRequest, ""},
		{"disable 202", "CC-CWM", http.MethodPost, base + "/5/disable", "", nil, http.StatusAccepted, ""},
		{"enable 202", "CC-CWM", http.MethodPost, base + "/5/enable", "", nil, http.StatusAccepted, ""},
		{"non-maker role denied at route", "ATM-SPV", http.MethodPost, base, `{}`, nil, http.StatusForbidden, ""},
		{"executor role denied at route", "CC-EXECUTOR", http.MethodGet, base, "", nil, http.StatusForbidden, ""},
		{"service forbidden", "CC-CWM", http.MethodPost, base, `{}`, service.ErrMasterDataForbidden, http.StatusForbidden, ""},
		{"duplicate code 409", "CC-CWM", http.MethodPost, base, `{}`, service.ErrCashCountAreaConflict, http.StatusConflict, ""},
		{"pending change 409", "CC-CWM", http.MethodPut, base + "/5", `{}`, service.ErrMasterDataChangePending, http.StatusConflict, ""},
		{"disable with active executors 409", "CC-CWM", http.MethodPost, base + "/5/disable", "", service.ErrCashCountAreaHasExecutors, http.StatusConflict, "Executor aktif"},
		{"unknown id 404", "CC-CWM", http.MethodPost, base + "/5/enable", "", service.ErrCashCountAreaNotFound, http.StatusNotFound, ""},
		{"validation error names field", "CC-CWM", http.MethodPost, base, `{}`, &service.ValidationError{Field: "region_codes", Message: "wajib diisi"}, 0, `region_codes`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, tokenSvc := mountCashCountAreaHandler(&fakeCashCountAreaServicer{err: tc.svcErr})
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

func TestCashCountAreaHandler_NoAuthContext401(t *testing.T) {
	router, _ := mountCashCountAreaHandler(&fakeCashCountAreaServicer{})
	rec := doRequest(router, http.MethodPost, "/api/v1/cash-count/areas", "", `{}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", rec.Code)
	}
}
