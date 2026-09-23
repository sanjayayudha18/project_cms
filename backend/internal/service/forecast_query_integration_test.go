//go:build integration

package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// setupForecastQueryHarness opens a real Postgres transaction, always rolled
// back via t.Cleanup, and returns db.Queries scoped to it. Task 1
// (.kiro/specs/update-cit-forecast-browser) extends ListForecastForDate with
// a multi-hop LEFT JOIN LATERAL chain (atms -> locations, atm_vendor_packages
// -> vendor_packages_branch -> vendor_branches -> vendors); these tests exercise
// that join against real Postgres rather than a mock, since a wrong join
// multiplies forecast rows or silently drops recommendations (money-adjacent).
func setupForecastQueryHarness(t *testing.T) (*db.Queries, pgx.Tx) {
	t.Helper()
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

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { tx.Rollback(ctx) })

	return db.New(tx), tx
}

// seedForecastRegionAndLocation creates a minimal region + location pair
// (both tables have no seed data the tests can rely on already existing).
func seedForecastRegionAndLocation(t *testing.T, tx pgx.Tx, marker string) int64 {
	t.Helper()
	ctx := context.Background()

	var regionID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO regions (code, region) VALUES ($1, 'Integration Test Region')
		RETURNING id
	`, "ITEST-"+marker).Scan(&regionID); err != nil {
		t.Fatalf("insert region: %v", err)
	}

	var locationID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO locations (region_id, type, name, address_line1, city_or_regency, province, country_code)
		VALUES ($1, 'ATM_SITE', 'Integration Test Location', 'Jl. Test No. 1', 'Jakarta', 'DKI Jakarta', 'ID')
		RETURNING id
	`, regionID).Scan(&locationID); err != nil {
		t.Fatalf("insert location: %v", err)
	}
	return locationID
}

// seedForecastATM inserts one atms row, real brand/model values so the new
// `brand` column has something to assert on.
func seedForecastATM(t *testing.T, tx pgx.Tx, terminalID string, locationID int64) int64 {
	t.Helper()
	var atmID int64
	err := tx.QueryRow(context.Background(), `
		INSERT INTO atms (terminal_id, location_id, machine_type, brand, model, operation_hours, deployment_type, is_active)
		VALUES ($1, $2, 'ATM', 'TestBrand', 'TestModel', '24_HOURS', 'OFFSITE', true)
		RETURNING id
	`, terminalID, locationID).Scan(&atmID)
	if err != nil {
		t.Fatalf("insert atm %s: %v", terminalID, err)
	}
	return atmID
}

// seedForecastVendorPackage creates a vendor -> vendor_branch -> vendor_package
// chain and returns the vendor_package id, so callers can link one or more
// atm_vendor_packages rows to it.
func seedForecastVendorPackage(t *testing.T, tx pgx.Tx, marker, vendorName string) int64 {
	t.Helper()
	ctx := context.Background()

	var vendorID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO vendors (code, name) VALUES ($1, $2) RETURNING id
	`, "ITEST-"+marker, vendorName).Scan(&vendorID); err != nil {
		t.Fatalf("insert vendor: %v", err)
	}

	var branchID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, region)
		VALUES ($1, $2, $3, 'Integration Test Region')
		RETURNING id
	`, vendorID, "ITEST-"+marker, vendorName+" Branch").Scan(&branchID); err != nil {
		t.Fatalf("insert vendor branch: %v", err)
	}

	var packageID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO vendor_packages_branch (vendor_branch_id, code, priority_class, price)
		VALUES ($1, 'PAKET TEST', 'ALL', 0)
		RETURNING id
	`, branchID).Scan(&packageID); err != nil {
		t.Fatalf("insert vendor package: %v", err)
	}
	return packageID
}

func linkAtmVendorPackage(t *testing.T, tx pgx.Tx, atmID, packageID int64, start, end *time.Time, isActive bool) {
	t.Helper()
	_, err := tx.Exec(context.Background(), `
		INSERT INTO atm_vendor_packages (atm_id, vendor_package_id, effective_start_date, effective_end_date, is_active)
		VALUES ($1, $2, $3, $4, $5)
	`, atmID, packageID, start, end, isActive)
	if err != nil {
		t.Fatalf("insert atm_vendor_packages: %v", err)
	}
}

// seedForecastRow inserts one dmaa_files + dmaa_atm_forecast row for the
// given terminal/date/denom. dmaa_atm_forecast.terminal_id carries a live FK
// to atms.terminal_id (fk_dmaa_atm_forecast_terminal), so the ATM must
// already exist — the spec's Req 3.4 ("forecast row with no atms match still
// returns") describes a state the schema itself forbids; the LEFT JOIN in
// ListForecastForDate stays for defense-in-depth but that exact scenario is
// untestable via a real insert.
func seedForecastRow(t *testing.T, tx pgx.Tx, terminalID string, forecastDate time.Time, denom int32) {
	t.Helper()
	ctx := context.Background()

	var fileID int64
	err := tx.QueryRow(ctx, `
		INSERT INTO dmaa_files (name, status, checksum) VALUES ($1, 'completed', $1)
		RETURNING id
	`, "itest-"+uuid.NewString()+".csv").Scan(&fileID)
	if err != nil {
		t.Fatalf("insert dmaa_files: %v", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO dmaa_atm_forecast (terminal_id, dmaa_file_id, periode_pred, denom, amount_replenish, amount_refund)
		VALUES ($1, $2, $3, $4, 100000000, 0)
	`, terminalID, fileID, forecastDate, denom)
	if err != nil {
		t.Fatalf("insert dmaa_atm_forecast: %v", err)
	}
}

func str(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

// TestIntegration_ListForecastForDate_ResolvesActivePackageOnly seeds an ATM
// with an expired and an active atm_vendor_packages row (Req 3.2): the
// forecast row must appear exactly once (no row multiplication from the
// LATERAL) and resolve the active vendor/region, not the expired one.
func TestIntegration_ListForecastForDate_ResolvesActivePackageOnly(t *testing.T) {
	q, tx := setupForecastQueryHarness(t)
	ctx := context.Background()
	marker := uuid.NewString()[:8]
	terminalID := "ITEST-" + marker

	locationID := seedForecastRegionAndLocation(t, tx, marker)
	atmID := seedForecastATM(t, tx, terminalID, locationID)

	forecastDate := time.Date(2027, 3, 15, 0, 0, 0, 0, time.UTC)

	expiredPkg := seedForecastVendorPackage(t, tx, marker+"-old", "Old Vendor "+marker)
	activePkg := seedForecastVendorPackage(t, tx, marker+"-new", "New Vendor "+marker)

	expiredStart := forecastDate.AddDate(0, -2, 0)
	expiredEnd := forecastDate.AddDate(0, -1, 0)
	linkAtmVendorPackage(t, tx, atmID, expiredPkg, &expiredStart, &expiredEnd, true)

	activeStart := forecastDate.AddDate(0, 0, -10)
	linkAtmVendorPackage(t, tx, atmID, activePkg, &activeStart, nil, true)

	seedForecastRow(t, tx, terminalID, forecastDate, 50000)

	rows, err := q.ListForecastForDate(ctx, db.ListForecastForDateParams{
		ForecastDate: toPgDate(forecastDate),
		TerminalID:   terminalID,
		Page:         1,
		PageSize:     10,
	})
	if err != nil {
		t.Fatalf("ListForecastForDate: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 row (no duplication from the LATERAL), got %d", len(rows))
	}
	row := rows[0]
	if got, want := str(row.FlmVendor), "New Vendor "+marker; got != want {
		t.Errorf("flm_vendor = %q, want %q (must resolve the active package, not the expired one)", got, want)
	}
	if got, want := str(row.FlmVendorRegion), "Integration Test Region"; got != want {
		t.Errorf("flm_vendor_region = %q, want %q", got, want)
	}
	if got, want := str(row.Brand), "TestBrand"; got != want {
		t.Errorf("brand = %q, want %q", got, want)
	}
	if got, want := str(row.LokasiAtm), "Integration Test Location"; got != want {
		t.Errorf("lokasi_atm = %q, want %q", got, want)
	}

	count, err := q.CountForecastForDate(ctx, db.CountForecastForDateParams{
		ForecastDate: toPgDate(forecastDate),
		TerminalID:   terminalID,
	})
	if err != nil {
		t.Fatalf("CountForecastForDate: %v", err)
	}
	if count != 1 {
		t.Errorf("CountForecastForDate = %d, want 1 (must match ListForecastForDate's row count — Req 3.2 design note)", count)
	}
}

// TestIntegration_ListForecastForDate_NoActivePackage_EmptyVendor seeds an
// ATM with zero atm_vendor_packages rows (Req 3.3): the forecast row must
// still be returned, with FLM Vendor / FLM Vendor Region empty rather than
// the row being dropped.
func TestIntegration_ListForecastForDate_NoActivePackage_EmptyVendor(t *testing.T) {
	q, tx := setupForecastQueryHarness(t)
	ctx := context.Background()
	marker := uuid.NewString()[:8]
	terminalID := "ITEST-" + marker

	locationID := seedForecastRegionAndLocation(t, tx, marker)
	_ = seedForecastATM(t, tx, terminalID, locationID)

	forecastDate := time.Date(2027, 3, 15, 0, 0, 0, 0, time.UTC)
	seedForecastRow(t, tx, terminalID, forecastDate, 100000)

	rows, err := q.ListForecastForDate(ctx, db.ListForecastForDateParams{
		ForecastDate: toPgDate(forecastDate),
		TerminalID:   terminalID,
		Page:         1,
		PageSize:     10,
	})
	if err != nil {
		t.Fatalf("ListForecastForDate: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 row (row must not be dropped for lacking a vendor package), got %d", len(rows))
	}
	row := rows[0]
	if row.FlmVendor != nil {
		t.Errorf("flm_vendor = %q, want nil (no active package)", str(row.FlmVendor))
	}
	if row.FlmVendorRegion != nil {
		t.Errorf("flm_vendor_region = %q, want nil (no active package)", str(row.FlmVendorRegion))
	}
	// Brand/Lokasi still resolve — those come from the atms/locations LEFT
	// JOINs, unrelated to the vendor-package LATERAL.
	if got, want := str(row.Brand), "TestBrand"; got != want {
		t.Errorf("brand = %q, want %q (unaffected by missing vendor package)", got, want)
	}
}
