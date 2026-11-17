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

// fakeCashCountAnomalyServicer exercises HTTP wiring only; logic is covered by
// internal/service/cash_count_anomaly_category_admin_test.go.
type fakeCashCountAnomalyServicer struct{ err error }

func (f *fakeCashCountAnomalyServicer) List(context.Context, string) ([]db.CashCountAnomalyCategory, error) {
	return []db.CashCountAnomalyCategory{{ID: 1, Code: "JAM", Name: "Kartu macet", IsActive: true}}, f.err
}
func (f *fakeCashCountAnomalyServicer) Create(context.Context, int64, service.CashCountAnomalyCategoryPayload, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 7, Status: "pending"}, f.err
}
func (f *fakeCashCountAnomalyServicer) Update(context.Context, int64, int64, service.CashCountAnomalyCategoryUpdatePayload, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 8, Status: "pending"}, f.err
}
func (f *fakeCashCountAnomalyServicer) Disable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 9, Status: "pending"}, f.err
}
func (f *fakeCashCountAnomalyServicer) Enable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 10, Status: "pending"}, f.err
}

func mountCashCountAnomalyHandler(svc CashCountAnomalyCategoryServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(tokenSvc), custommw.RequireRoles("CC-CCR", "ADMIN")).
		Mount("/api/v1/cash-count/anomaly-categories", NewCashCountAnomalyCategoryHandler(svc).Routes())
	return r, tokenSvc
}

func TestCashCountAnomalyCategoryHandler(t *testing.T) {
	const base = "/api/v1/cash-count/anomaly-categories"
	cases := []struct {
		name, role, method, path, body string
		svcErr                         error
		wantCode                       int
		wantBody                       string
	}{
		{"list", "ADMIN", http.MethodGet, base + "?status=all", "", nil, http.StatusOK, `"code":"JAM"`},
		{"list bad status", "CC-CCR", http.MethodGet, base + "?status=zzz", "", nil, http.StatusBadRequest, ""},
		{"create 202", "CC-CCR", http.MethodPost, base, `{"code":"JAM","name":"x"}`, nil, http.StatusAccepted, ""},
		{"create bad body", "CC-CCR", http.MethodPost, base, `{`, nil, http.StatusBadRequest, ""},
		{"update 202", "CC-CCR", http.MethodPut, base + "/5", `{"name":"y"}`, nil, http.StatusAccepted, ""},
		{"update bad id", "CC-CCR", http.MethodPut, base + "/abc", `{"name":"y"}`, nil, http.StatusBadRequest, ""},
		{"disable 202", "CC-CCR", http.MethodPost, base + "/5/disable", "", nil, http.StatusAccepted, ""},
		{"enable 202", "CC-CCR", http.MethodPost, base + "/5/enable", "", nil, http.StatusAccepted, ""},
		{"non-CCR role denied at route", "ATM-SPV", http.MethodPost, base, `{}`, nil, http.StatusForbidden, ""},
		{"executor denied at route", "CC-EXECUTOR", http.MethodGet, base, "", nil, http.StatusForbidden, ""},
		{"service forbidden (ADMIN is not maker)", "ADMIN", http.MethodPost, base, `{}`, service.ErrMasterDataForbidden, http.StatusForbidden, ""},
		{"duplicate code 409", "CC-CCR", http.MethodPost, base, `{}`, service.ErrCashCountAnomalyCategoryConflict, http.StatusConflict, ""},
		{"pending change 409", "CC-CCR", http.MethodPost, base + "/5/disable", "", service.ErrMasterDataChangePending, http.StatusConflict, ""},
		{"unknown id 404", "CC-CCR", http.MethodPost, base + "/5/enable", "", service.ErrCashCountAnomalyCategoryNotFound, http.StatusNotFound, ""},
		{"validation error names field", "CC-CCR", http.MethodPost, base, `{}`, &service.ValidationError{Field: "code", Message: "wajib"}, 0, `code`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, tokenSvc := mountCashCountAnomalyHandler(&fakeCashCountAnomalyServicer{err: tc.svcErr})
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

func TestCashCountAnomalyCategoryHandler_NoToken401(t *testing.T) {
	router, _ := mountCashCountAnomalyHandler(&fakeCashCountAnomalyServicer{})
	rec := doRequest(router, http.MethodPost, "/api/v1/cash-count/anomaly-categories", "", `{}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", rec.Code)
	}
}
