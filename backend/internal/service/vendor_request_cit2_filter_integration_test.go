//go:build integration

package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// TestIntegration_ListForecastForDate_CIT2Filters covers Task 4.2 (Req 1):
// seeds forecast rows across two brands/vendors/regions and asserts the new
// brand/flm_vendor/flm_vendor_region filters combine with logical AND
// (Req 1.10), match case-insensitively (Req 1.6-1.8), and that an
// empty-string Brand is a no-op filter (Req 1.9, 1.16) — reusing the same
// seeding harness as forecast_query_integration_test.go (Task 1 of the
// sibling update-cit-forecast-browser spec).
func TestIntegration_ListForecastForDate_CIT2Filters(t *testing.T) {
	q, tx := setupForecastQueryHarness(t)
	ctx := context.Background()
	marker := uuid.NewString()[:8]
	forecastDate := time.Date(2027, 4, 1, 0, 0, 0, 0, time.UTC)

	locationID := seedForecastRegionAndLocation(t, tx, marker)

	// Terminal A: TAG vendor / Integration Test Region / brand TestBrand
	// (seedForecastATM hardcodes brand="TestBrand").
	terminalA := "ITEST-A-" + marker
	atmA := seedForecastATM(t, tx, terminalA, locationID)
	pkgA := seedForecastVendorPackage(t, tx, marker+"-a", "TAG Vendor "+marker)
	startA := forecastDate.AddDate(0, 0, -10)
	linkAtmVendorPackage(t, tx, atmA, pkgA, &startA, nil, true)
	seedForecastRow(t, tx, terminalA, forecastDate, 50000)

	// Terminal B: different vendor, same brand/region (seedForecastVendorPackage
	// always seeds region 'Integration Test Region').
	terminalB := "ITEST-B-" + marker
	atmB := seedForecastATM(t, tx, terminalB, locationID)
	pkgB := seedForecastVendorPackage(t, tx, marker+"-b", "ROH Vendor "+marker)
	startB := forecastDate.AddDate(0, 0, -10)
	linkAtmVendorPackage(t, tx, atmB, pkgB, &startB, nil, true)
	seedForecastRow(t, tx, terminalB, forecastDate, 50000)

	vendorAName := "TAG Vendor " + marker

	t.Run("combined brand+flm_vendor+flm_vendor_region AND filter isolates one row (Req 1.10)", func(t *testing.T) {
		rows, err := q.ListForecastForDate(ctx, db.ListForecastForDateParams{
			ForecastDate:    toPgDate(forecastDate),
			Brand:           "TestBrand",
			FlmVendor:       vendorAName,
			FlmVendorRegion: "Integration Test Region",
			Page:            1,
			PageSize:        10,
		})
		if err != nil {
			t.Fatalf("ListForecastForDate: %v", err)
		}
		var got []string
		for _, r := range rows {
			got = append(got, r.TerminalID)
		}
		if len(rows) != 1 || rows[0].TerminalID != terminalA {
			t.Fatalf("expected exactly [%s], got %v", terminalA, got)
		}
	})

	t.Run("case-insensitive match on brand and flm_vendor (Req 1.6-1.8)", func(t *testing.T) {
		rows, err := q.ListForecastForDate(ctx, db.ListForecastForDateParams{
			ForecastDate:    toPgDate(forecastDate),
			Brand:           "testbrand",
			FlmVendor:       "tag vendor " + marker,
			FlmVendorRegion: "integration test region",
			Page:            1,
			PageSize:        10,
		})
		if err != nil {
			t.Fatalf("ListForecastForDate: %v", err)
		}
		if len(rows) != 1 || rows[0].TerminalID != terminalA {
			t.Fatalf("case-insensitive filter should still match terminal %s, got %d rows", terminalA, len(rows))
		}
	})

	t.Run("empty Brand sentinel is a no-op filter (Req 1.9, 1.16)", func(t *testing.T) {
		rows, err := q.ListForecastForDate(ctx, db.ListForecastForDateParams{
			ForecastDate:    toPgDate(forecastDate),
			Brand:           "",
			FlmVendor:       vendorAName,
			FlmVendorRegion: "Integration Test Region",
			Page:            1,
			PageSize:        10,
		})
		if err != nil {
			t.Fatalf("ListForecastForDate: %v", err)
		}
		if len(rows) != 1 || rows[0].TerminalID != terminalA {
			t.Fatalf("empty brand should behave identically to the brand-set case, got %d rows", len(rows))
		}
	})

	t.Run("Count matches List under the same filters", func(t *testing.T) {
		params := db.CountForecastForDateParams{
			ForecastDate:    toPgDate(forecastDate),
			Brand:           "TestBrand",
			FlmVendor:       vendorAName,
			FlmVendorRegion: "Integration Test Region",
		}
		count, err := q.CountForecastForDate(ctx, params)
		if err != nil {
			t.Fatalf("CountForecastForDate: %v", err)
		}
		if count != 1 {
			t.Errorf("CountForecastForDate = %d, want 1 (must match ListForecastForDate's filtered row count)", count)
		}
	})

	t.Run("filtering by the other vendor returns only that terminal", func(t *testing.T) {
		rows, err := q.ListForecastForDate(ctx, db.ListForecastForDateParams{
			ForecastDate:    toPgDate(forecastDate),
			FlmVendor:       "ROH Vendor " + marker,
			FlmVendorRegion: "Integration Test Region",
			Page:            1,
			PageSize:        10,
		})
		if err != nil {
			t.Fatalf("ListForecastForDate: %v", err)
		}
		if len(rows) != 1 || rows[0].TerminalID != terminalB {
			t.Fatalf("expected exactly [%s], got %d rows", terminalB, len(rows))
		}
	})
}
