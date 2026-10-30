package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

type fakeDsrLateReader struct {
	err    error
	filter service.DsrLateFilter
}

var testWIB = time.FixedZone("WIB", 7*3600)

func (f *fakeDsrLateReader) Daily(_ context.Context, _ service.Actor, flt service.DsrLateFilter) (*service.DsrLateDailyList, error) {
	f.filter = flt
	return &service.DsrLateDailyList{Items: []service.DsrLateDailyView{}, Page: 1, PageSize: 50}, f.err
}
func (f *fakeDsrLateReader) Summary(_ context.Context, _ service.Actor, flt service.DsrLateFilter) (*service.DsrLateSummary, error) {
	f.filter = flt
	return &service.DsrLateSummary{Items: []service.DsrLateSummaryView{
		{VendorCode: "V1", VendorName: "Vendor", BranchCode: "B1", BranchName: "Cab 1", ObligedDays: 3, OkDays: 1, LateDays: 1, NotSentDays: 1, LateMinutesTotal: 2},
	}}, f.err
}
func (f *fakeDsrLateReader) Warnings(_ context.Context, _ service.Actor, flt service.DsrLateFilter) (*service.DsrLateWarnings, error) {
	f.filter = flt
	return &service.DsrLateWarnings{UnmappedVaults: []service.DsrLateUnmappedVault{{VaultCode: "VLT1", BranchExcluded: true}}}, f.err
}

func TestDsrLateHandler(t *testing.T) {
	const base = "/api/v1/dsr-late-reports"
	const csvMsg = "Ekspor CSV dipindah ke menu ekspor"
	cases := []struct {
		name, path, role string
		svcErr           error
		wantCode         int
		wantBody         []string
		wantNoCall       bool // the service must not be reached
	}{
		{"daily json", base + "/daily?month=2026-10&vendor_id=2&vendor_branch_id=3&status=late&page=2&page_size=10", "ATM-USER", nil,
			http.StatusOK, []string{`"items":[]`}, false},
		{"summary json", base + "/summary?month=2026-10", "ATM-SPV", nil, http.StatusOK, []string{`"obliged_days":3`}, false},
		{"warnings json", base + "/warnings?month=2026-10", "BRANCH-ATM-SPV", nil, http.StatusOK, []string{`"branch_excluded":true`}, false},
		{"admin allowed", base + "/summary?month=2026-10", "ADMIN", nil, http.StatusOK, nil, false},
		{"other role denied at route", base + "/daily?month=2026-10", "ACM-USER", nil, http.StatusForbidden, nil, true},
		{"vendor denied at route", base + "/daily?month=2026-10", "VENDOR-USER", nil, http.StatusForbidden, nil, true},
		{"service denies", base + "/summary?month=2026-10", "ATM-USER", service.ErrNotAuthorized, http.StatusForbidden, nil, false},
		{"missing month", base + "/daily", "ATM-USER", nil, http.StatusBadRequest, []string{"month"}, true},
		{"bad month", base + "/daily?month=10-2026", "ATM-USER", nil, http.StatusBadRequest, []string{"month"}, true},
		{"future month (service)", base + "/summary?month=2099-01", "ATM-USER",
			&service.ValidationError{Field: "month", Message: "bulan tidak boleh setelah bulan berjalan"}, http.StatusBadRequest, []string{"month"}, false},
		{"bad status", base + "/daily?month=2026-10&status=x", "ATM-USER", nil, http.StatusBadRequest, []string{"status"}, true},
		{"bad vendor", base + "/daily?month=2026-10&vendor_id=abc", "ATM-USER", nil, http.StatusBadRequest, []string{"vendor_id"}, true},
		{"daily csv rejected", base + "/daily?month=2026-10&format=csv", "ATM-USER", nil,
			http.StatusBadRequest, []string{"validation_error", `"field":"format"`, csvMsg}, true},
		{"summary csv rejected", base + "/summary?month=2026-10&format=csv", "ADMIN", nil,
			http.StatusBadRequest, []string{"validation_error", `"field":"format"`, csvMsg}, true},
		{"csv rejected before params and service errors", base + "/daily?month=bad&format=csv", "ATM-USER", service.ErrNotAuthorized,
			http.StatusBadRequest, []string{`"field":"format"`}, true},
		{"other format stays json", base + "/summary?month=2026-10&format=json", "ATM-SPV", nil, http.StatusOK, []string{`"obliged_days":3`}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := testTokenService()
			fake := &fakeDsrLateReader{err: tc.svcErr}
			r := chi.NewRouter()
			r.With(custommw.RequireAuth(ts)).Mount(base, NewDsrLateHandler(fake).Routes())
			token := tokenForRole(t, ts, 1, tc.role)
			if tc.role == "VENDOR-USER" {
				token = tokenForVendor(t, ts, 9, 42)
			}
			rec := doRequest(r, http.MethodGet, tc.path, token, "")
			if rec.Code != tc.wantCode {
				t.Fatalf("code = %d, want %d: %s", rec.Code, tc.wantCode, rec.Body.String())
			}
			for _, want := range tc.wantBody {
				if !strings.Contains(rec.Body.String(), want) {
					t.Errorf("body %q does not contain %q", rec.Body.String(), want)
				}
			}
			if strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv") {
				t.Errorf("must never answer text/csv, got %q", rec.Header().Get("Content-Type"))
			}
			if tc.wantNoCall && !fake.filter.Month.IsZero() {
				t.Errorf("service reached with filter %+v", fake.filter)
			}
		})
	}
}

func TestDsrLateHandler_FilterParsing(t *testing.T) {
	ts := testTokenService()
	fake := &fakeDsrLateReader{}
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(ts)).Mount("/d", NewDsrLateHandler(fake).Routes())
	rec := doRequest(r, http.MethodGet, "/d/daily?month=2026-10&vendor_id=2&vendor_branch_id=3&status=late&page=2&page_size=10", tokenForRole(t, ts, 1, "ATM-USER"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d: %s", rec.Code, rec.Body.String())
	}
	f := fake.filter
	if f.Month.Format("2006-01") != "2026-10" || f.VendorID == nil || *f.VendorID != 2 || f.VendorBranchID == nil || *f.VendorBranchID != 3 ||
		f.Status == nil || *f.Status != "late" || f.Page != 2 || f.PageSize != 10 {
		t.Fatalf("filter = %+v", f)
	}
}
