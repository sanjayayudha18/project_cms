package reports

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cimb-niaga/cms/pkg/csvexport"
)

const goldenDir = "../../backend/internal/handler/testdata/report_csv"

func ptr[T any](v T) *T { return &v }

func render(t *testing.T, report string, rows [][]string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteCSV(&buf, report, rows); err != nil {
		t.Fatal(err)
	}
	return strings.TrimPrefix(buf.String(), csvexport.BOM)
}

func TestCellsMatchLegacyGolden(t *testing.T) {
	// Timestamps in UTC: wibCell must convert (design: t.In(WIB)).
	utc := func(d, h, m int) time.Time { return time.Date(2026, 10, d, h, m, 0, 0, time.UTC) }
	d := func(day int) *time.Time { return ptr(time.Date(2026, 10, day, 0, 0, 0, 0, time.UTC)) }
	abacus := DsrLateDailyRow{ReportDate: "2026-10-01", VendorCode: "ABACUS", VendorName: "PT Abacus Cash Solution", BranchCode: "ABACUS_004",
		BranchName: "Medan – Jl. Gatot Subroto Ñ", RegionCode: ptr("MDN"), Deadline: utc(1, 2, 0),
		FirstReceivedAt: ptr(utc(1, 1, 41)), Status: "ok", ReceiptChecksum: "a1b2c3", MatchedLabels: []string{"Gudang Medan"}}
	kejar1 := DsrLateDailyRow{ReportDate: "2026-10-01", VendorCode: "KEJAR", VendorName: `=HYPERLINK("x")`, BranchCode: "KJR_01",
		BranchName: "+Cabang Plus", RegionCode: ptr("JKT"), Deadline: utc(1, 2, 0), FirstReceivedAt: ptr(utc(1, 3, 15)),
		Status: "late", LateMinutes: 75, ReceiptChecksum: "d4e5f6", MatchedLabels: []string{"-Vault Minus", "Vault 雅加达"}}
	kejar2 := DsrLateDailyRow{ReportDate: "2026-10-02", VendorCode: "KEJAR", VendorName: `=HYPERLINK("x")`, BranchCode: "KJR_02",
		BranchName: `Cab, "Koma"`, Deadline: utc(2, 2, 0), Status: "not_sent"}
	pending := DsrLateDailyRow{ReportDate: "2026-10-03", VendorCode: "ABACUS", VendorName: "PT Abacus Cash Solution", BranchCode: "ABACUS_004",
		BranchName: "Medan – Jl. Gatot Subroto Ñ", RegionCode: ptr("MDN"), Deadline: utc(3, 2, 0), Status: "pending"}
	mm := []AmountMismatchRow{
		{TicketNumber: ptr("ATM001_100K_20261005_001"), RequestNumber: ptr("REP-ABC-MDN-20261005-001"), TerminalID: "ATM001",
			VendorName: ptr("PT Abacus Cash Solution"), BranchCode: ptr("ABACUS_004"), RegionCode: ptr("MDN"),
			ScheduledDate: d(5), RealizedDate: d(6), WorkdayOffset: ptr(int16(1)), Classification: "late", Currency: "IDR",
			Amounts: []Amount{{50000, 200000000, 150000000, ptr(int64(150000000)), -50000000}, {100000, 300000000, 300000000, nil, 0}}},
		{TerminalID: "-ATM002", VendorName: ptr("Vendor Ü"), RealizedDate: d(7), Classification: "no_order", Currency: "IDR",
			Amounts: []Amount{{100000, 0, 100000000, nil, 100000000}}},
		{TicketNumber: ptr("ATM003_MIX_20261008_002"), RequestNumber: ptr("REP-KJR-JKT-20261008-001"), TerminalID: "ATM003",
			VendorName: ptr(`=HYPERLINK("x")`), BranchCode: ptr("KJR_01"), RegionCode: ptr("JKT"),
			ScheduledDate: d(8), Classification: "void", Currency: "IDR",
			Amounts: []Amount{{50000, 50000000, 0, ptr(int64(0)), -50000000}}},
	}
	var mmAll, mmLate [][]string
	for _, r := range mm {
		mmAll = append(mmAll, AmountMismatchLines(r)...)
		if r.Classification == "late" {
			mmLate = append(mmLate, AmountMismatchLines(r)...)
		}
	}
	daily := func(rs ...DsrLateDailyRow) (out [][]string) {
		for _, r := range rs {
			out = append(out, DsrLateDailyCells(r))
		}
		return out
	}
	month, from, to := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), *d(1), *d(31)
	const mmFile = "replenishment-amount-mismatch-2026-10-01-2026-10-31.csv"
	cases := []struct {
		golden, report, file string
		rows                 [][]string
	}{
		{"dsr_late_daily.csv", DsrLateDaily, "dsr-late-daily-2026-10.csv", daily(abacus, kejar1, kejar2, pending)},
		{"dsr_late_daily_status_late.csv", DsrLateDaily, "dsr-late-daily-2026-10.csv", daily(kejar1)},
		{"dsr_late_summary.csv", DsrLateSummary, "dsr-late-summary-2026-10.csv", [][]string{
			DsrLateSummaryCells(DsrLateSummaryRow{"ABACUS", "PT Abacus Cash Solution", "ABACUS_004", "Medan – Jl. Gatot Subroto Ñ", ptr("MDN"), 3, 1, 0, 0, 1, 0}),
			DsrLateSummaryCells(DsrLateSummaryRow{"KEJAR", `=HYPERLINK("x")`, "KJR_01", "+Cabang Plus", ptr("JKT"), 2, 0, 1, 1, 0, 75})}},
		{"amount_mismatch.csv", ReplenishmentAmountMismatch, mmFile, mmAll},
		{"amount_mismatch_class_late.csv", ReplenishmentAmountMismatch, mmFile, mmLate},
		{"trips.csv", ReplenishmentTrips, "replenishment-trips-2026-10-01-2026-10-31.csv", [][]string{
			TripCells(TripRow{VendorID: ptr(int64(12)), VendorName: ptr("PT Abacus Cash Solution"), TerminalID: "ATM001", Trips: 3, FirstDate: d(1), LastDate: d(20)}),
			TripCells(TripRow{VendorName: ptr("+Vendor Ü"), TerminalID: "ATM002", Trips: 1, FirstDate: d(7), LastDate: d(7)})}},
	}
	for _, tc := range cases {
		t.Run(tc.golden, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join(goldenDir, tc.golden))
			if err != nil {
				t.Fatal(err)
			}
			if got := render(t, tc.report, tc.rows); got != string(want) {
				t.Errorf("csv differs from legacy golden\n got: %q\nwant: %q", got, want)
			}
			if got := FileName(tc.report, month, from, to); got != tc.file {
				t.Errorf("FileName = %q, want %q", got, tc.file)
			}
		})
	}
}
