//go:build integration

package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// TestIntegration_SummarizeForecastForDate covers forecast-browser-summary
// (.claude/sdlc/forecast-browser-summary/plan.md B4): the summary recap must
// agree with ListForecastForDate/CountForecastForDate row for row, count an
// ATM as requested only when all its denoms are, and treat cancelled/rejected
// requests as not requested. Uses a far-future date so dev data never leaks
// into the per-date aggregate.
func TestIntegration_SummarizeForecastForDate(t *testing.T) {
	q, tx := setupForecastQueryHarness(t)
	ctx := context.Background()
	marker := uuid.NewString()[:8]
	forecastDate := time.Date(2031, 3, 17, 0, 0, 0, 0, time.UTC)
	start := forecastDate.AddDate(0, 0, -10)

	locationID := seedForecastRegionAndLocation(t, tx, marker)
	vendorA := "SUMA Vendor " + marker
	vendorB := "SUMB Vendor " + marker
	pkgA := seedForecastVendorPackage(t, tx, marker+"-a", vendorA)
	pkgB := seedForecastVendorPackage(t, tx, marker+"-b", vendorB)

	seedATM := func(suffix string, pkg int64, denoms ...int32) string {
		terminal := "ISUM-" + suffix + "-" + marker
		atmID := seedForecastATM(t, tx, terminal, locationID)
		if pkg != 0 {
			linkAtmVendorPackage(t, tx, atmID, pkg, &start, nil, true)
		}
		for _, d := range denoms {
			seedForecastRow(t, tx, terminal, forecastDate, d)
		}
		return terminal
	}
	a1 := seedATM("A1", pkgA, 50000)         // requested (draft)
	a2 := seedATM("A2", pkgA, 50000, 100000) // only one denom requested -> belum
	a3 := seedATM("A3", pkgA, 100000)        // requested but cancelled -> belum
	b1 := seedATM("B1", pkgB, 50000)         // requested (approved)
	b2 := seedATM("B2", pkgB, 50000)         // requested but rejected -> belum
	u1 := seedATM("U1", 0, 50000)            // no active vendor package

	seedRequest(t, tx, "draft", false, forecastDate, item{a1, 50000}, item{a2, 50000})
	seedRequest(t, tx, "cancelled", true, forecastDate, item{a3, 100000})
	seedRequest(t, tx, "approved", false, forecastDate, item{b1, 50000})
	seedRequest(t, tx, "rejected", false, forecastDate, item{b2, 50000})

	groups, err := q.SummarizeForecastForDate(ctx, toPgDate(forecastDate))
	if err != nil {
		t.Fatalf("SummarizeForecastForDate: %v", err)
	}
	byKey := map[string]db.SummarizeForecastForDateRow{}
	var totalATMs int64
	for _, g := range groups {
		byKey[g.FlmVendor] = g
		totalATMs += g.AtmCount
	}

	t.Run("(c)(d) requested = all denoms requested, cancelled/rejected count as belum", func(t *testing.T) {
		ga := byKey[vendorA]
		if ga.AtmCount != 3 || ga.RequestedAtmCount != 1 {
			t.Errorf("vendor A atm/requested = %d/%d, want 3/1", ga.AtmCount, ga.RequestedAtmCount)
		}
		// 4 rows x 100_000_000; unrequested = a2@100000 + a3 = 2 rows.
		if ga.AmountReplenish != 400_000_000 || ga.UnrequestedAmountReplenish != 200_000_000 {
			t.Errorf("vendor A amounts = %d/%d, want 400000000/200000000", ga.AmountReplenish, ga.UnrequestedAmountReplenish)
		}
		gb := byKey[vendorB]
		if gb.AtmCount != 2 || gb.RequestedAtmCount != 1 {
			t.Errorf("vendor B atm/requested = %d/%d, want 2/1", gb.AtmCount, gb.RequestedAtmCount)
		}
	})

	t.Run("(e) no-active-vendor group has empty vendor/region", func(t *testing.T) {
		gu, ok := byKey[""]
		if !ok || gu.FlmVendorRegion != "" || gu.AtmCount != 1 || gu.RequestedAtmCount != 0 {
			t.Errorf("unassigned group = %+v (present=%v), want 1 ATM, 0 requested", gu, ok)
		}
		rows := listForecast(t, q, forecastDate, "", "", true)
		if len(rows) != 1 || rows[0].TerminalID != u1 {
			t.Errorf("unassigned=true rows = %v, want only %s", terminalIDs(rows), u1)
		}
	})

	t.Run("(a)(b)(f) groups agree with ListForecastForDate/CountForecastForDate", func(t *testing.T) {
		all := listForecast(t, q, forecastDate, "", "", false)
		if int64(len(distinct(all))) != totalATMs {
			t.Errorf("Σ group atm_count = %d, distinct terminals in list = %d", totalATMs, len(distinct(all)))
		}
		for _, vendor := range []string{vendorA, vendorB} {
			rows := listForecast(t, q, forecastDate, vendor, "Integration Test Region", false)
			if got := int64(len(distinct(rows))); got != byKey[vendor].AtmCount {
				t.Errorf("%s: list distinct = %d, summary atm_count = %d", vendor, got, byKey[vendor].AtmCount)
			}
			count, err := q.CountForecastForDate(ctx, db.CountForecastForDateParams{
				ForecastDate: toPgDate(forecastDate), FlmVendor: vendor, FlmVendorRegion: "Integration Test Region",
			})
			if err != nil {
				t.Fatalf("CountForecastForDate: %v", err)
			}
			if count != int64(len(rows)) {
				t.Errorf("%s: count = %d, list rows = %d", vendor, count, len(rows))
			}
		}
	})

	t.Run("is_requested on list rows", func(t *testing.T) {
		want := map[string]bool{a1 + "|50000": true, a2 + "|50000": true, a2 + "|100000": false,
			a3 + "|100000": false, b1 + "|50000": true, b2 + "|50000": false, u1 + "|50000": false}
		for _, r := range listForecast(t, q, forecastDate, "", "", false) {
			key := r.TerminalID + "|" + strconv.Itoa(int(r.Denom))
			if w, ok := want[key]; ok && r.IsRequested != w {
				t.Errorf("%s is_requested = %v, want %v", key, r.IsRequested, w)
			}
		}
	})
}

type item struct {
	terminal string
	denom    int32
}

func seedRequest(t *testing.T, tx pgx.Tx, status string, isCanceled bool, date time.Time, items ...item) {
	t.Helper()
	ctx := context.Background()
	var createdBy, requestID int64
	if err := tx.QueryRow(ctx, "SELECT id FROM users ORDER BY id LIMIT 1").Scan(&createdBy); err != nil {
		t.Skipf("no users row to own the request: %v", err)
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO vendor_requests (request_number, forecast_date, created_by, status, is_canceled)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		"ISUM-"+uuid.NewString()[:12], date, createdBy, status, isCanceled).Scan(&requestID); err != nil {
		t.Fatalf("insert vendor_requests: %v", err)
	}
	for _, it := range items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO vendor_request_items (vendor_request_id, terminal_id, periode_pred, denom, amount_replenish)
			VALUES ($1, $2, $3, $4, 100000000)`, requestID, it.terminal, date, it.denom); err != nil {
			t.Fatalf("insert vendor_request_items: %v", err)
		}
	}
}

func listForecast(t *testing.T, q *db.Queries, date time.Time, vendor, region string, unassigned bool) []db.ListForecastForDateRow {
	t.Helper()
	rows, err := q.ListForecastForDate(context.Background(), db.ListForecastForDateParams{
		ForecastDate: toPgDate(date), FlmVendor: vendor, FlmVendorRegion: region,
		Unassigned: unassigned, Page: 1, PageSize: 100,
	})
	if err != nil {
		t.Fatalf("ListForecastForDate: %v", err)
	}
	return rows
}

func distinct(rows []db.ListForecastForDateRow) map[string]bool {
	m := map[string]bool{}
	for _, r := range rows {
		m[r.TerminalID] = true
	}
	return m
}

func terminalIDs(rows []db.ListForecastForDateRow) []string {
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.TerminalID
	}
	return ids
}
