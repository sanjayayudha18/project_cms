package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
	"github.com/cimb-niaga/cms/pkg/reports"
)

// Property 2 (report-export-migration 3.2): the screen equals the file. The JSON the report
// endpoints send, mapped through pkg/reports, must produce the same CSV as the golden files.

// pagedGoldenDsrLateReader adds real paging to the golden Daily (the plain fake returns no items).
type pagedGoldenDsrLateReader struct{ goldenDsrLateReader }

func (f *pagedGoldenDsrLateReader) Daily(ctx context.Context, a service.Actor, flt service.DsrLateFilter) (*service.DsrLateDailyList, error) {
	all, _ := f.DailyAll(ctx, a, flt)
	page, size := max(flt.Page, 1), flt.PageSize
	lo, hi := min((page-1)*size, len(all)), min(page*size, len(all))
	return &service.DsrLateDailyList{Items: all[lo:hi], Total: int64(len(all)), Page: page, PageSize: size}, nil
}

func screenDailyRow(v service.DsrLateDailyView) reports.DsrLateDailyRow {
	return reports.DsrLateDailyRow{ReportDate: v.ReportDate, VendorCode: v.VendorCode, VendorName: v.VendorName,
		BranchCode: v.BranchCode, BranchName: v.BranchName, RegionCode: v.RegionCode, Deadline: v.Deadline,
		FirstReceivedAt: v.FirstReceivedAt, Status: v.Status, LateMinutes: v.LateMinutes,
		ReceiptChecksum: v.ReceiptChecksum, MatchedLabels: v.MatchedLabels}
}

func screenSummaryRow(v service.DsrLateSummaryView) reports.DsrLateSummaryRow {
	return reports.DsrLateSummaryRow{VendorCode: v.VendorCode, VendorName: v.VendorName, BranchCode: v.BranchCode,
		BranchName: v.BranchName, RegionCode: v.RegionCode, ObligedDays: v.ObligedDays, OkDays: v.OkDays,
		LateDays: v.LateDays, NotSentDays: v.NotSentDays, PendingDays: v.PendingDays, LateMinutesTotal: v.LateMinutesTotal}
}

func screenMismatchRow(v service.ClassificationView) reports.AmountMismatchRow {
	am := make([]reports.Amount, len(v.Amounts))
	for i, a := range v.Amounts {
		am[i] = reports.Amount{Denom: a.Denom, Order: a.Order, Itm: a.Itm, Vendor: a.Vendor, Diff: a.Diff}
	}
	return reports.AmountMismatchRow{TicketNumber: v.TicketNumber, RequestNumber: v.RequestNumber, TerminalID: v.TerminalID,
		VendorName: v.VendorName, BranchCode: v.BranchCode, RegionCode: v.RegionCode, ScheduledDate: v.ScheduledDate,
		RealizedDate: v.RealizedDate, Classification: v.Classification, WorkdayOffset: v.WorkdayOffset,
		Currency: v.Currency, Amounts: am}
}

func screenTripRow(v service.TripRow) reports.TripRow {
	return reports.TripRow{VendorID: v.VendorID, VendorName: v.VendorName, TerminalID: v.TerminalID, Trips: v.Trips,
		FirstDate: v.FirstDate, LastDate: v.LastDate}
}

// assertScreenEqualsGolden writes the lines the way the export does and compares with the golden (after BOM).
func assertScreenEqualsGolden(t *testing.T, report string, lines [][]string, golden string) {
	t.Helper()
	var buf bytes.Buffer
	if err := reports.WriteCSV(&buf, report, lines); err != nil {
		t.Fatal(err)
	}
	got := bytes.TrimPrefix(buf.Bytes(), []byte("\xef\xbb\xbf"))
	want, err := os.ReadFile(filepath.Join(reportGoldenDir, golden+".csv"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("screen != file %s\n--- screen ---\n%s\n--- golden ---\n%s", golden, got, want)
	}
}

func getJSON(t *testing.T, r http.Handler, token, path string, dst any) {
	t.Helper()
	rec := doRequest(r, http.MethodGet, path, token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d: %s", path, rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), dst); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func TestReportScreenEqualsFile(t *testing.T) {
	ts := testTokenService()
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(ts)).Mount("/api/v1/dsr-late-reports", NewDsrLateHandler(&pagedGoldenDsrLateReader{}).Routes())
	r.With(custommw.RequireAuth(ts)).Mount("/api/v1/replenishment", NewReplenishmentHandler(&goldenReplenishmentReader{}).Routes())
	token := tokenForRole(t, ts, 1, "ADMIN")
	const dsr, rep = "/api/v1/dsr-late-reports", "/api/v1/replenishment"
	const rng = "date_from=2026-10-01&date_to=2026-10-31"

	dailyCases := []struct{ name, query, golden string }{
		{"all", "", "dsr_late_daily"},
		{"status_late", "&status=late", "dsr_late_daily_status_late"},
	}
	for _, tc := range dailyCases {
		for _, size := range []int{1, 2, 3, 50} {
			t.Run("daily_"+tc.name+"_page_size_"+strconv.Itoa(size), func(t *testing.T) {
				lines := [][]string{}
				for page, pages := 1, 1; page <= pages; page++ {
					var out service.DsrLateDailyList
					getJSON(t, r, token, dsr+"/daily?month=2026-10&page="+strconv.Itoa(page)+"&page_size="+strconv.Itoa(size)+tc.query, &out)
					pages = int((out.Total + int64(size) - 1) / int64(size))
					for _, v := range out.Items {
						lines = append(lines, reports.DsrLateDailyCells(screenDailyRow(v)))
					}
				}
				assertScreenEqualsGolden(t, reports.DsrLateDaily, lines, tc.golden)
			})
		}
	}

	t.Run("summary", func(t *testing.T) {
		var out service.DsrLateSummary
		getJSON(t, r, token, dsr+"/summary?month=2026-10", &out)
		lines := [][]string{}
		for _, v := range out.Items {
			lines = append(lines, reports.DsrLateSummaryCells(screenSummaryRow(v)))
		}
		assertScreenEqualsGolden(t, reports.DsrLateSummary, lines, "dsr_late_summary")
	})

	for _, tc := range []struct{ name, query, golden string }{
		{"all", "", "amount_mismatch"},
		{"class_late", "&classification=late", "amount_mismatch_class_late"},
	} {
		t.Run("amount_mismatch_"+tc.name, func(t *testing.T) {
			var out struct {
				Items []service.ClassificationView `json:"items"`
			}
			getJSON(t, r, token, rep+"/reports/amount-mismatch?"+rng+tc.query, &out)
			lines := [][]string{}
			for _, v := range out.Items {
				lines = append(lines, reports.AmountMismatchLines(screenMismatchRow(v))...)
			}
			assertScreenEqualsGolden(t, reports.ReplenishmentAmountMismatch, lines, tc.golden)
		})
	}

	t.Run("trips", func(t *testing.T) {
		var out struct {
			Items []service.TripRow `json:"items"`
		}
		getJSON(t, r, token, rep+"/reports/trips?"+rng, &out)
		lines := [][]string{}
		for _, v := range out.Items {
			lines = append(lines, reports.TripCells(screenTripRow(v)))
		}
		assertScreenEqualsGolden(t, reports.ReplenishmentTrips, lines, "trips")
	})
}
