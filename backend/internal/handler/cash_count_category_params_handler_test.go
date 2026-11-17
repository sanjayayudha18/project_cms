package handler

import (
	"context"
	"errors"
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

// fakeCashCountCategoryParamsServicer exercises HTTP wiring only; validation is
// covered by internal/service/cash_count_category_params_admin_test.go.
type fakeCashCountCategoryParamsServicer struct {
	rows    []db.CashCountCategoryParam
	listErr error
	err     error
	got     service.CashCountCategoryParamsPayload
	maker   int64
}

func (f *fakeCashCountCategoryParamsServicer) List(context.Context) ([]db.CashCountCategoryParam, error) {
	return f.rows, f.listErr
}

func (f *fakeCashCountCategoryParamsServicer) Create(_ context.Context, makerID int64, req service.CashCountCategoryParamsPayload, _ string) (db.MasterDataChangeRequest, error) {
	f.got, f.maker = req, makerID
	return db.MasterDataChangeRequest{ID: 7, Status: "pending"}, f.err
}

func ccNumeric(t *testing.T, s string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		t.Fatalf("scan %s: %v", s, err)
	}
	return n
}

func mountCashCountCategoryParamsHandler(svc CashCountCategoryParamsServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(tokenSvc), custommw.RequireRoles("ADMIN", "CC-CWM")).
		Mount("/api/v1/cash-count/category-params", NewCashCountCategoryParamsHandler(svc).Routes())
	return r, tokenSvc
}

const ccParamsBase = "/api/v1/cash-count/category-params"

func TestCashCountCategoryParamsHandler_List(t *testing.T) {
	now := pgtype.Timestamptz{Time: time.Date(2026, 11, 13, 1, 0, 0, 0, time.UTC), Valid: true}
	svc := &fakeCashCountCategoryParamsServicer{rows: []db.CashCountCategoryParam{
		{ID: 2, MediumMinAmount: ccNumeric(t, "5000000000.00"), MediumMaxAmount: ccNumeric(t, "15000000000.01"), CurrencyCode: "IDR", OnsiteHigh: 3, OnsiteMedium: 2, OnsiteLow: 1, PeriodMonths: 6, EffectiveFrom: now},
		{ID: 1, MediumMinAmount: ccNumeric(t, "4000000000.00"), MediumMaxAmount: ccNumeric(t, "9000000000.00"), CurrencyCode: "IDR", PeriodMonths: 6, EffectiveFrom: now, SupersededAt: now},
	}}
	router, tokenSvc := mountCashCountCategoryParamsHandler(svc)
	rec := doRequest(router, http.MethodGet, ccParamsBase, tokenForRole(t, tokenSvc, 1, "ADMIN"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`"active":{`, `"medium_max_amount":"15000000000.01"`, // exact decimal, no float rounding
		`"history":[{`, `"medium_min_amount":"4000000000.00"`, `"superseded_at":"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body %s does not contain %s", body, want)
		}
	}
}

func TestCashCountCategoryParamsHandler_ListEmptyAndError(t *testing.T) {
	router, tokenSvc := mountCashCountCategoryParamsHandler(&fakeCashCountCategoryParamsServicer{})
	rec := doRequest(router, http.MethodGet, ccParamsBase, tokenForRole(t, tokenSvc, 1, "CC-CWM"), "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"active":null`) || !strings.Contains(rec.Body.String(), `"history":[]`) {
		t.Fatalf("empty list: %d %s", rec.Code, rec.Body.String())
	}
	router, tokenSvc = mountCashCountCategoryParamsHandler(&fakeCashCountCategoryParamsServicer{listErr: errors.New("db down")})
	rec = doRequest(router, http.MethodGet, ccParamsBase, tokenForRole(t, tokenSvc, 1, "CC-CWM"), "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("list error code = %d", rec.Code)
	}
}

func TestCashCountCategoryParamsHandler_Create(t *testing.T) {
	const valid = `{"medium_min_amount":"5000000000.00","medium_max_amount":"15000000000.00","currency_code":"IDR","onsite_high":3,"onsite_medium":2,"onsite_low":1}`
	cases := []struct {
		name, role, body string
		svcErr           error
		wantCode         int
		wantBody         string
	}{
		{"maker 202", "CC-CWM", valid, nil, http.StatusAccepted, `"status":"pending"`},
		{"non-maker role denied at route", "ATM-SPV", valid, nil, http.StatusForbidden, ""},
		{"service forbidden (ADMIN is not maker)", "ADMIN", valid, service.ErrMasterDataForbidden, http.StatusForbidden, ""},
		{"validation 422 names field", "CC-CWM", valid, &service.ValidationError{Field: "onsite_high", Message: "jumlah kunjungan onsite harus 0–6 per 6 bulan"}, http.StatusUnprocessableEntity, "onsite_high"},
		{"no active set 409", "CC-CWM", valid, service.ErrCashCountCategoryParamsMissing, http.StatusConflict, ""},
		{"pending change 409", "CC-CWM", valid, service.ErrMasterDataChangePending, http.StatusConflict, ""},
		{"bad json 400", "CC-CWM", `{`, nil, http.StatusBadRequest, ""},
		{"unexpected 500", "CC-CWM", valid, errors.New("boom"), http.StatusInternalServerError, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeCashCountCategoryParamsServicer{err: tc.svcErr}
			router, tokenSvc := mountCashCountCategoryParamsHandler(svc)
			rec := doRequest(router, http.MethodPost, ccParamsBase, tokenForRole(t, tokenSvc, 42, tc.role), tc.body)
			if rec.Code != tc.wantCode {
				t.Fatalf("code = %d, want %d: %s", rec.Code, tc.wantCode, rec.Body.String())
			}
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Errorf("body %s does not contain %s", rec.Body.String(), tc.wantBody)
			}
			if tc.wantCode == http.StatusAccepted && (svc.maker != 42 || svc.got.MediumMinAmount != "5000000000.00" || svc.got.OnsiteHigh != 3) {
				t.Errorf("service got maker=%d payload=%+v", svc.maker, svc.got)
			}
		})
	}
}

func TestCashCountCategoryParamsHandler_NoToken401(t *testing.T) {
	router, _ := mountCashCountCategoryParamsHandler(&fakeCashCountCategoryParamsServicer{})
	rec := doRequest(router, http.MethodPost, ccParamsBase, "", `{}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d", rec.Code)
	}
}
