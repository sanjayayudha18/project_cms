package handler

import (
	"context"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// Golden CSV of the legacy report exports (report-export-migration task 1.1). Recorded from the
// handlers as they are before pkg/reports / backend-exq take over; regenerate only on purpose:
//
//	go test ./internal/handler -run TestReportCSVGolden -update
//
// Data comes from a fake reader (same choice as the master-data golden), so the files lock the
// cells, header, CSVSafe and file name; the SQL behind the views is shared and unchanged.
var updateReportGolden = flag.Bool("update", false, "rewrite report CSV golden files")

const reportGoldenDir = "testdata/report_csv"

func ptr[T any](v T) *T { return &v }

func wibAt(d, h, m int) time.Time { return time.Date(2026, 10, d, h, m, 0, 0, testWIB) }

// goldenDsrLateReader returns fixed views; status is filtered here the way the SQL does.
type goldenDsrLateReader struct{ fakeDsrLateReader }

func goldenDailyRows() []service.DsrLateDailyView {
	return []service.DsrLateDailyView{
		{ReportDate: "2026-10-01", VendorCode: "ABACUS", VendorName: "PT Abacus Cash Solution", BranchCode: "ABACUS_004",
			BranchName: "Medan – Jl. Gatot Subroto Ñ", RegionCode: ptr("MDN"), Deadline: wibAt(1, 9, 0),
			FirstReceivedAt: ptr(wibAt(1, 8, 41)), Status: "ok", ReceiptChecksum: "a1b2c3", MatchedLabels: []string{"Gudang Medan"}},
		{ReportDate: "2026-10-01", VendorCode: "KEJAR", VendorName: "=HYPERLINK(\"x\")", BranchCode: "KJR_01",
			BranchName: "+Cabang Plus", RegionCode: ptr("JKT"), Deadline: wibAt(1, 9, 0),
			FirstReceivedAt: ptr(wibAt(1, 10, 15)), Status: "late", LateMinutes: 75, ReceiptChecksum: "d4e5f6",
			MatchedLabels: []string{"-Vault Minus", "Vault 雅加达"}},
		{ReportDate: "2026-10-02", VendorCode: "KEJAR", VendorName: "=HYPERLINK(\"x\")", BranchCode: "KJR_02",
			BranchName: "Cab, \"Koma\"", Deadline: wibAt(2, 9, 0), Status: "not_sent"},
		{ReportDate: "2026-10-03", VendorCode: "ABACUS", VendorName: "PT Abacus Cash Solution", BranchCode: "ABACUS_004",
			BranchName: "Medan – Jl. Gatot Subroto Ñ", RegionCode: ptr("MDN"), Deadline: wibAt(3, 9, 0), Status: "pending"},
	}
}

func (f *goldenDsrLateReader) DailyAll(_ context.Context, _ service.Actor, flt service.DsrLateFilter) ([]service.DsrLateDailyView, error) {
	var out []service.DsrLateDailyView
	for _, v := range goldenDailyRows() {
		if flt.Status == nil || *flt.Status == v.Status {
			out = append(out, v)
		}
	}
	return out, nil
}

func (f *goldenDsrLateReader) Summary(_ context.Context, _ service.Actor, _ service.DsrLateFilter) (*service.DsrLateSummary, error) {
	return &service.DsrLateSummary{Items: []service.DsrLateSummaryView{
		{VendorCode: "ABACUS", VendorName: "PT Abacus Cash Solution", BranchCode: "ABACUS_004", BranchName: "Medan – Jl. Gatot Subroto Ñ",
			RegionCode: ptr("MDN"), ObligedDays: 3, OkDays: 1, PendingDays: 1, LateMinutesTotal: 0},
		{VendorCode: "KEJAR", VendorName: "=HYPERLINK(\"x\")", BranchCode: "KJR_01", BranchName: "+Cabang Plus",
			RegionCode: ptr("JKT"), ObligedDays: 2, LateDays: 1, NotSentDays: 1, LateMinutesTotal: 75},
	}}, nil
}

// goldenReplenishmentReader returns fixed views; classification is filtered here the way the SQL does.
type goldenReplenishmentReader struct{ fakeReplenishmentReader }

func goldenMismatchRows() []service.ClassificationView {
	d := func(day int) *time.Time { return ptr(time.Date(2026, 10, day, 0, 0, 0, 0, time.UTC)) }
	return []service.ClassificationView{
		{TicketNumber: ptr("ATM001_100K_20261005_001"), RequestNumber: ptr("REP-ABC-MDN-20261005-001"), TerminalID: "ATM001",
			VendorName: ptr("PT Abacus Cash Solution"), BranchCode: ptr("ABACUS_004"), RegionCode: ptr("MDN"),
			ScheduledDate: d(5), RealizedDate: d(6), WorkdayOffset: ptr(int16(1)), Classification: "late", Currency: "IDR",
			Amounts: []service.ClassificationAmount{
				{Denom: 50000, Order: 200000000, Itm: 150000000, Vendor: ptr(int64(150000000)), Diff: -50000000},
				{Denom: 100000, Order: 300000000, Itm: 300000000, Vendor: nil, Diff: 0},
			}},
		{TerminalID: "-ATM002", VendorName: ptr("Vendor Ü"), RealizedDate: d(7), Classification: "no_order", Currency: "IDR",
			Amounts: []service.ClassificationAmount{{Denom: 100000, Order: 0, Itm: 100000000, Diff: 100000000}}},
		{TicketNumber: ptr("ATM003_MIX_20261008_002"), RequestNumber: ptr("REP-KJR-JKT-20261008-001"), TerminalID: "ATM003",
			VendorName: ptr("=HYPERLINK(\"x\")"), BranchCode: ptr("KJR_01"), RegionCode: ptr("JKT"),
			ScheduledDate: d(8), Classification: "void", Currency: "IDR",
			Amounts: []service.ClassificationAmount{{Denom: 50000, Order: 50000000, Itm: 0, Vendor: ptr(int64(0)), Diff: -50000000}}},
	}
}

func (f *goldenReplenishmentReader) AmountMismatch(_ context.Context, _ service.Actor, flt service.ReplenishmentFilter) ([]service.ClassificationView, error) {
	var out []service.ClassificationView
	for _, v := range goldenMismatchRows() {
		if flt.Classification == nil || *flt.Classification == v.Classification {
			out = append(out, v)
		}
	}
	return out, nil
}

func (f *goldenReplenishmentReader) Trips(_ context.Context, _ service.Actor, _, _ time.Time, _ *int64) ([]service.TripRow, error) {
	d := func(day int) *time.Time { return ptr(time.Date(2026, 10, day, 0, 0, 0, 0, time.UTC)) }
	return []service.TripRow{
		{VendorID: ptr(int64(12)), VendorName: ptr("PT Abacus Cash Solution"), TerminalID: "ATM001", Trips: 3, FirstDate: d(1), LastDate: d(20)},
		{VendorName: ptr("+Vendor Ü"), TerminalID: "ATM002", Trips: 1, FirstDate: d(7), LastDate: d(7)},
	}, nil
}

func TestReportCSVGolden(t *testing.T) {
	ts := testTokenService()
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(ts)).Mount("/api/v1/dsr-late-reports", NewDsrLateHandler(&goldenDsrLateReader{}).Routes())
	r.With(custommw.RequireAuth(ts)).Mount("/api/v1/replenishment", NewReplenishmentHandler(&goldenReplenishmentReader{}).Routes())
	token := tokenForRole(t, ts, 1, "ADMIN")

	const rng = "date_from=2026-10-01&date_to=2026-10-31"
	cases := []struct{ name, path, wantFile string }{
		{"dsr_late_daily", "/api/v1/dsr-late-reports/daily?month=2026-10&format=csv", "dsr-late-daily-2026-10.csv"},
		{"dsr_late_daily_status_late", "/api/v1/dsr-late-reports/daily?month=2026-10&status=late&format=csv", "dsr-late-daily-2026-10.csv"},
		{"dsr_late_summary", "/api/v1/dsr-late-reports/summary?month=2026-10&format=csv", "dsr-late-summary-2026-10.csv"},
		{"amount_mismatch", "/api/v1/replenishment/reports/amount-mismatch?" + rng + "&format=csv",
			"replenishment-amount-mismatch-2026-10-01-2026-10-31.csv"},
		{"amount_mismatch_class_late", "/api/v1/replenishment/reports/amount-mismatch?" + rng + "&classification=late&format=csv",
			"replenishment-amount-mismatch-2026-10-01-2026-10-31.csv"},
		{"trips", "/api/v1/replenishment/reports/trips?" + rng + "&format=csv", "replenishment-trips-2026-10-01-2026-10-31.csv"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(r, http.MethodGet, tc.path, token, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("code = %d: %s", rec.Code, rec.Body.String())
			}
			if cd, want := rec.Header().Get("Content-Disposition"), `attachment; filename="`+tc.wantFile+`"`; cd != want {
				t.Errorf("Content-Disposition = %q, want %q", cd, want)
			}
			path := filepath.Join(reportGoldenDir, tc.name+".csv")
			if *updateReportGolden {
				if err := os.MkdirAll(reportGoldenDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, rec.Body.Bytes(), 0o600); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden (run with -update to record): %v", err)
			}
			if got := rec.Body.String(); got != string(want) {
				t.Errorf("CSV differs from %s\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
			}
			if strings.HasPrefix(rec.Body.String(), "\xef\xbb\xbf") {
				t.Error("legacy report CSV has no BOM; golden must not either")
			}
		})
	}
}
