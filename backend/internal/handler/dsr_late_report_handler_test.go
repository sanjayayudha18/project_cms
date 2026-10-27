package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

type fakeDsrLateReader struct {
	err       error
	exportErr error
	filter    service.DsrLateFilter
	exported  []string
	exportArg map[string]any
}

var testWIB = time.FixedZone("WIB", 7*3600)

func (f *fakeDsrLateReader) Daily(_ context.Context, _ service.Actor, flt service.DsrLateFilter) (*service.DsrLateDailyList, error) {
	f.filter = flt
	return &service.DsrLateDailyList{Items: []service.DsrLateDailyView{}, Page: 1, PageSize: 50}, f.err
}
func (f *fakeDsrLateReader) DailyAll(_ context.Context, _ service.Actor, flt service.DsrLateFilter) ([]service.DsrLateDailyView, error) {
	f.filter = flt
	recv := time.Date(2026, 10, 5, 9, 2, 0, 0, testWIB)
	region := "JKT"
	return []service.DsrLateDailyView{
		{ReportDate: "2026-10-05", VendorCode: "V1", VendorName: "=EVIL", BranchCode: "B1", BranchName: "Cab 1", RegionCode: &region,
			Deadline: time.Date(2026, 10, 5, 9, 0, 0, 0, testWIB), FirstReceivedAt: &recv, Status: "late", LateMinutes: 2,
				ReceiptChecksum: "abc123", MatchedLabels: []string{"=Gudang A", "Gudang B"}},
		{ReportDate: "2026-10-06", VendorCode: "V1", VendorName: "Vendor", BranchCode: "B1", BranchName: "Cab 1",
			Deadline: time.Date(2026, 10, 6, 9, 0, 0, 0, testWIB), Status: "not_sent"},
	}, f.err
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
func (f *fakeDsrLateReader) RecordExport(_ context.Context, _ service.Actor, report string, filter map[string]any) error {
	if f.exportErr != nil {
		return f.exportErr
	}
	f.exported = append(f.exported, report)
	f.exportArg = filter
	return nil
}

func TestDsrLateHandler(t *testing.T) {
	const base = "/api/v1/dsr-late-reports"
	cases := []struct {
		name, path, role  string
		svcErr, exportErr error
		wantCode          int
		wantBody          []string
		wantExport        string
		wantNoExport      bool
	}{
		{"daily json", base + "/daily?month=2026-10&vendor_id=2&vendor_branch_id=3&status=late&page=2&page_size=10", "ATM-USER", nil, nil,
			http.StatusOK, []string{`"items":[]`}, "", false},
		{"summary json", base + "/summary?month=2026-10", "ATM-SPV", nil, nil, http.StatusOK, []string{`"obliged_days":3`}, "", false},
		{"warnings json", base + "/warnings?month=2026-10", "BRANCH-ATM-SPV", nil, nil, http.StatusOK, []string{`"branch_excluded":true`}, "", false},
		{"admin allowed", base + "/summary?month=2026-10", "ADMIN", nil, nil, http.StatusOK, nil, "", false},
		{"other role denied at route", base + "/daily?month=2026-10", "ACM-USER", nil, nil, http.StatusForbidden, nil, "", true},
		{"vendor denied at route", base + "/daily?month=2026-10", "VENDOR-USER", nil, nil, http.StatusForbidden, nil, "", true},
		{"service denies", base + "/summary?month=2026-10", "ATM-USER", service.ErrNotAuthorized, nil, http.StatusForbidden, nil, "", true},
		{"missing month", base + "/daily", "ATM-USER", nil, nil, http.StatusBadRequest, []string{"month"}, "", true},
		{"bad month", base + "/daily?month=10-2026", "ATM-USER", nil, nil, http.StatusBadRequest, []string{"month"}, "", true},
		{"future month (service)", base + "/summary?month=2099-01", "ATM-USER",
			&service.ValidationError{Field: "month", Message: "bulan tidak boleh setelah bulan berjalan"}, nil, http.StatusBadRequest, []string{"month"}, "", true},
		{"bad status", base + "/daily?month=2026-10&status=x", "ATM-USER", nil, nil, http.StatusBadRequest, []string{"status"}, "", true},
		{"bad vendor", base + "/daily?month=2026-10&vendor_id=abc", "ATM-USER", nil, nil, http.StatusBadRequest, []string{"vendor_id"}, "", true},
		{"daily csv", base + "/daily?month=2026-10&format=csv", "ATM-USER", nil, nil, http.StatusOK,
			[]string{"report_date,vendor_code,vendor,branch_code,branch_name,region_code,deadline_wib,first_received_at_wib,status,late_minutes,receipt_checksum,matched_locations",
				"2026-10-05,V1,'=EVIL,B1,Cab 1,JKT,2026-10-05 09:00:00,2026-10-05 09:02:00,late,2,abc123,'=Gudang A; Gudang B",
				"2026-10-06,V1,Vendor,B1,Cab 1,,2026-10-06 09:00:00,,not_sent,0,,"}, "daily", false},
		{"summary csv", base + "/summary?month=2026-10&format=csv", "ADMIN", nil, nil, http.StatusOK,
			[]string{"obliged_days,ok_days,late_days,not_sent_days,pending_days,late_minutes_total", "V1,Vendor,B1,Cab 1,,3,1,1,1,0,2"}, "summary", false},
		{"csv audit failure -> 500, no csv", base + "/daily?month=2026-10&format=csv", "ATM-USER", nil, errors.New("db down"),
			http.StatusInternalServerError, nil, "", true},
		{"csv denied by service", base + "/daily?month=2026-10&format=csv", "ATM-USER", service.ErrNotAuthorized, nil, http.StatusForbidden, nil, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := testTokenService()
			fake := &fakeDsrLateReader{err: tc.svcErr, exportErr: tc.exportErr}
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
			if tc.wantExport != "" {
				if len(fake.exported) != 1 || fake.exported[0] != tc.wantExport || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv") {
					t.Errorf("export audit = %v, content-type %q", fake.exported, rec.Header().Get("Content-Type"))
				}
				if fake.exportArg["month"] != "2026-10" {
					t.Errorf("audit filter = %v", fake.exportArg)
				}
			}
			if tc.wantNoExport && (len(fake.exported) != 0 || strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv")) {
				t.Errorf("must not export: audit %v, content-type %q", fake.exported, rec.Header().Get("Content-Type"))
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
