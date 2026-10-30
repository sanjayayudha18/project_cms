//go:build integration

package service

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/pkg/reports"
)

// Property 2 (report-export-migration 3.2) on real Postgres, READ-ONLY: the paged screen query
// (Daily) must give the same lines through pkg/reports whatever the page size, and the
// other reports must be well formed. Nothing is inserted, updated or deleted.

func dailyLines(items []DsrLateDailyView) [][]string {
	out := [][]string{}
	for _, v := range items {
		out = append(out, reports.DsrLateDailyCells(reports.DsrLateDailyRow{ReportDate: v.ReportDate, VendorCode: v.VendorCode,
			VendorName: v.VendorName, BranchCode: v.BranchCode, BranchName: v.BranchName, RegionCode: v.RegionCode,
			Deadline: v.Deadline, FirstReceivedAt: v.FirstReceivedAt, Status: v.Status, LateMinutes: v.LateMinutes,
			ReceiptChecksum: v.ReceiptChecksum, MatchedLabels: v.MatchedLabels}))
	}
	return out
}

func mismatchLines(items []ClassificationView) [][]string {
	out := [][]string{}
	for _, v := range items {
		am := make([]reports.Amount, len(v.Amounts))
		for i, a := range v.Amounts {
			am[i] = reports.Amount{Denom: a.Denom, Order: a.Order, Itm: a.Itm, Vendor: a.Vendor, Diff: a.Diff}
		}
		out = append(out, reports.AmountMismatchLines(reports.AmountMismatchRow{TicketNumber: v.TicketNumber,
			RequestNumber: v.RequestNumber, TerminalID: v.TerminalID, VendorName: v.VendorName, BranchCode: v.BranchCode,
			RegionCode: v.RegionCode, ScheduledDate: v.ScheduledDate, RealizedDate: v.RealizedDate,
			Classification: v.Classification, WorkdayOffset: v.WorkdayOffset, Currency: v.Currency, Amounts: am})...)
	}
	return out
}

func tripLines(items []TripRow) [][]string {
	out := [][]string{}
	for _, v := range items {
		out = append(out, reports.TripCells(reports.TripRow{VendorID: v.VendorID, VendorName: v.VendorName,
			TerminalID: v.TerminalID, Trips: v.Trips, FirstDate: v.FirstDate, LastDate: v.LastDate}))
	}
	return out
}

func assertWidth(t *testing.T, name, report string, lines [][]string) {
	t.Helper()
	w := len(reports.Header(report))
	for i, l := range lines {
		if len(l) != w {
			t.Fatalf("%s line %d has %d cells, header has %d", name, i, len(l), w)
		}
	}
	if len(lines) == 0 {
		t.Logf("%s: dev DB has no rows for this window (empty case asserted)", name)
	}
}

func TestIntegration_ScreenFile_Reports(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}
	t.Cleanup(pool.Close)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	dsr := NewDsrLateReportService(pool, &start)
	rep := NewReplenishmentReadService(pool)
	actor := Actor{UserID: 1, Role: "ATM-SPV"}

	now := time.Now().In(reports.WIB)
	thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	months := []time.Time{thisMonth, thisMonth.AddDate(0, -1, 0)}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	windows := [][2]time.Time{
		{thisMonth, thisMonth.AddDate(0, 1, -1)},
		{months[1], thisMonth.AddDate(0, 0, -1)},
		{today.AddDate(0, 0, -30), today},
	}

	for _, m := range months {
		t.Run("daily_"+m.Format("2006-01"), func(t *testing.T) {
			pageAll := func(size int) []DsrLateDailyView {
				rows := []DsrLateDailyView{}
				for page := 1; ; page++ {
					out, err := dsr.Daily(ctx, actor, DsrLateFilter{Month: m, Page: page, PageSize: size})
					if err != nil {
						t.Fatal(err)
					}
					rows = append(rows, out.Items...)
					if len(out.Items) == 0 || int64(len(rows)) >= out.Total {
						if int64(len(rows)) != out.Total {
							t.Fatalf("paged %d rows, total says %d", len(rows), out.Total)
						}
						return rows
					}
				}
			}
			small, large := pageAll(7), pageAll(dsrLateMaxPageSize)
			if got, want := dailyLines(small), dailyLines(large); !reflect.DeepEqual(got, want) {
				t.Fatalf("page size 7 (%d lines) != page size %d (%d lines)", len(got), dsrLateMaxPageSize, len(want))
			}
			assertWidth(t, "daily", reports.DsrLateDaily, dailyLines(large))
		})
	}

	for _, w := range windows {
		name := w[0].Format("2006-01-02") + "_" + w[1].Format("2006-01-02")
		t.Run("amount_mismatch_"+name, func(t *testing.T) {
			rows, err := rep.AmountMismatch(ctx, actor, ReplenishmentFilter{From: w[0], To: w[1]})
			if err != nil {
				t.Fatal(err)
			}
			assertWidth(t, "amount_mismatch "+name, reports.ReplenishmentAmountMismatch, mismatchLines(rows))
		})
		t.Run("trips_"+name, func(t *testing.T) {
			rows, err := rep.Trips(ctx, actor, w[0], w[1], nil)
			if err != nil {
				t.Fatal(err)
			}
			assertWidth(t, "trips "+name, reports.ReplenishmentTrips, tripLines(rows))
		})
	}
}
