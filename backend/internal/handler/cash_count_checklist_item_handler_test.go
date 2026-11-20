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

// fakeChecklistItemServicer exercises HTTP wiring only; rules are covered by
// internal/service/cash_count_checklist_item_admin_test.go.
type fakeChecklistItemServicer struct{ err error }

func (f *fakeChecklistItemServicer) List(context.Context, string, string) ([]db.CashCountChecklistItem, error) {
	day := "DAY1"
	return []db.CashCountChecklistItem{{ID: 1, Category: "SELECTED_MACHINE", Day: &day, PointNo: 2, ItemText: "Cek kaset", IsActive: true}}, nil
}
func (f *fakeChecklistItemServicer) Create(context.Context, int64, service.CashCountChecklistItemPayload, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 7, Status: "pending"}, f.err
}
func (f *fakeChecklistItemServicer) Update(context.Context, int64, int64, service.CashCountChecklistItemUpdatePayload, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 8, Status: "pending"}, f.err
}
func (f *fakeChecklistItemServicer) Disable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 9, Status: "pending"}, f.err
}
func (f *fakeChecklistItemServicer) Enable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 10, Status: "pending"}, f.err
}

func mountChecklistItemHandler(svc CashCountChecklistItemServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(tokenSvc), custommw.RequireRoles("CC-CCR", "ADMIN")).
		Mount("/api/v1/cash-count/checklist-items", NewCashCountChecklistItemHandler(svc).Routes())
	return r, tokenSvc
}

func TestCashCountChecklistItemHandler(t *testing.T) {
	const base = "/api/v1/cash-count/checklist-items"
	cases := []struct {
		name, role, method, path, body string
		svcErr                         error
		wantCode                       int
		wantBody                       string
	}{
		{"list", "CC-CCR", http.MethodGet, base, "", nil, http.StatusOK, `"item_text":"Cek kaset"`},
		{"list by admin", "ADMIN", http.MethodGet, base + "?status=all&category=NON_MACHINE", "", nil, http.StatusOK, `"items"`},
		{"list bad status", "CC-CCR", http.MethodGet, base + "?status=x", "", nil, http.StatusBadRequest, ""},
		{"list bad category", "CC-CCR", http.MethodGet, base + "?category=X", "", nil, http.StatusBadRequest, ""},
		{"create 202", "CC-CCR", http.MethodPost, base, `{"category":"NON_MACHINE","point_no":1,"item_text":"x"}`, nil, http.StatusAccepted, `"status":"pending"`},
		{"update 202", "CC-CCR", http.MethodPut, base + "/5", `{"point_no":1,"item_text":"x"}`, nil, http.StatusAccepted, ""},
		{"disable 202", "CC-CCR", http.MethodPost, base + "/5/disable", "", nil, http.StatusAccepted, ""},
		{"enable 202", "CC-CCR", http.MethodPost, base + "/5/enable", "", nil, http.StatusAccepted, ""},
		{"bad body 400", "CC-CCR", http.MethodPost, base, `{`, nil, http.StatusBadRequest, ""},
		{"bad id 400", "CC-CCR", http.MethodPost, base + "/abc/disable", "", nil, http.StatusBadRequest, ""},
		{"non-maker role denied at route", "ATM-SPV", http.MethodPost, base, `{}`, nil, http.StatusForbidden, ""},
		{"executor denied at route", "CC-EXECUTOR", http.MethodGet, base, "", nil, http.StatusForbidden, ""},
		{"service forbidden (ADMIN is not a maker)", "ADMIN", http.MethodPost, base, `{}`, service.ErrMasterDataForbidden, http.StatusForbidden, ""},
		{"duplicate point 409", "CC-CCR", http.MethodPost, base, `{}`, service.ErrCashCountChecklistItemConflict, http.StatusConflict, ""},
		{"pending change 409", "CC-CCR", http.MethodPost, base + "/5/disable", "", service.ErrMasterDataChangePending, http.StatusConflict, ""},
		{"not found 404", "CC-CCR", http.MethodPost, base + "/5/enable", "", service.ErrCashCountChecklistItemNotFound, http.StatusNotFound, ""},
		{"validation error names field", "CC-CCR", http.MethodPost, base, `{}`, &service.ValidationError{Field: "day", Message: "wajib diisi"}, 0, `day`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, tokenSvc := mountChecklistItemHandler(&fakeChecklistItemServicer{err: tc.svcErr})
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

func TestCashCountChecklistItemHandler_NoAuth401(t *testing.T) {
	router, _ := mountChecklistItemHandler(&fakeChecklistItemServicer{})
	rec := doRequest(router, http.MethodGet, "/api/v1/cash-count/checklist-items", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", rec.Code)
	}
}
