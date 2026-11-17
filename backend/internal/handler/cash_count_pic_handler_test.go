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

// fakeCashCountPicServicer exercises HTTP wiring only; logic is covered by
// internal/service/cash_count_pic_admin_test.go.
type fakeCashCountPicServicer struct {
	err      error
	gotRole  string
	gotState string
}

func (f *fakeCashCountPicServicer) List(_ context.Context, role, status string) ([]db.ListCashCountPicsAdminRow, error) {
	f.gotRole, f.gotState = role, status
	region := "JKT"
	return []db.ListCashCountPicsAdminRow{{ID: 1, PicRole: "COORDINATOR", UserID: 4, Username: "budi", RegionCode: &region, IsActive: true}}, nil
}
func (f *fakeCashCountPicServicer) Create(context.Context, int64, service.CashCountPicPayload, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 7, Status: "pending"}, f.err
}
func (f *fakeCashCountPicServicer) Update(context.Context, int64, int64, service.CashCountPicUpdatePayload, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 8, Status: "pending"}, f.err
}
func (f *fakeCashCountPicServicer) Disable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 9, Status: "pending"}, f.err
}
func (f *fakeCashCountPicServicer) Enable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 10, Status: "pending"}, f.err
}

func mountCashCountPicHandler(svc CashCountPicServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(tokenSvc), custommw.RequireRoles("CC-CWM", "ADMIN")).
		Mount("/api/v1/cash-count/pics", NewCashCountPicHandler(svc).Routes())
	return r, tokenSvc
}

func TestCashCountPicHandler(t *testing.T) {
	const base = "/api/v1/cash-count/pics"
	cases := []struct {
		name, role, method, path, body string
		svcErr                         error
		wantCode                       int
		wantBody                       string
	}{
		{"list", "ADMIN", http.MethodGet, base, "", nil, http.StatusOK, `"username":"budi"`},
		{"list bad status", "CC-CWM", http.MethodGet, base + "?status=x", "", nil, http.StatusBadRequest, ""},
		{"list bad pic_role", "CC-CWM", http.MethodGet, base + "?pic_role=X", "", nil, http.StatusBadRequest, ""},
		{"create 202", "CC-CWM", http.MethodPost, base, `{"pic_role":"COORDINATOR","user_id":4,"region_code":"JKT"}`, nil, http.StatusAccepted, ""},
		{"create bad body", "CC-CWM", http.MethodPost, base, `{`, nil, http.StatusBadRequest, ""},
		{"update 202", "CC-CWM", http.MethodPut, base + "/5", `{"user_id":4,"region_code":"BDG"}`, nil, http.StatusAccepted, ""},
		{"disable 202", "CC-CWM", http.MethodPost, base + "/5/disable", "", nil, http.StatusAccepted, ""},
		{"enable 202", "CC-CWM", http.MethodPost, base + "/5/enable", "", nil, http.StatusAccepted, ""},
		{"non-maker role denied at route", "ATM-SPV", http.MethodPost, base, `{}`, nil, http.StatusForbidden, ""},
		{"executor denied at route", "CC-EXECUTOR", http.MethodGet, base, "", nil, http.StatusForbidden, ""},
		{"service forbidden (ADMIN is not maker)", "ADMIN", http.MethodPost, base, `{}`, service.ErrMasterDataForbidden, http.StatusForbidden, ""},
		{"duplicate 409", "CC-CWM", http.MethodPost, base, `{}`, service.ErrCashCountPicConflict, http.StatusConflict, ""},
		{"pending 409", "CC-CWM", http.MethodPost, base + "/5/disable", "", service.ErrMasterDataChangePending, http.StatusConflict, ""},
		{"not found 404", "CC-CWM", http.MethodPost, base + "/5/enable", "", service.ErrCashCountPicNotFound, http.StatusNotFound, ""},
		{"validation error names field", "CC-CWM", http.MethodPost, base, `{}`, &service.ValidationError{Field: "user_id", Message: "wajib diisi"}, 0, `user_id`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, tokenSvc := mountCashCountPicHandler(&fakeCashCountPicServicer{err: tc.svcErr})
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

func TestCashCountPicHandler_ListPassesFilters(t *testing.T) {
	svc := &fakeCashCountPicServicer{}
	router, tokenSvc := mountCashCountPicHandler(svc)
	rec := doRequest(router, http.MethodGet, "/api/v1/cash-count/pics?pic_role=EXECUTOR&status=all", tokenForRole(t, tokenSvc, 1, "CC-CWM"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d: %s", rec.Code, rec.Body.String())
	}
	if svc.gotRole != "EXECUTOR" || svc.gotState != "all" {
		t.Errorf("got role=%q status=%q", svc.gotRole, svc.gotState)
	}
}

func TestCashCountPicHandler_NoToken401(t *testing.T) {
	router, _ := mountCashCountPicHandler(&fakeCashCountPicServicer{})
	rec := doRequest(router, http.MethodGet, "/api/v1/cash-count/pics", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", rec.Code)
	}
}
