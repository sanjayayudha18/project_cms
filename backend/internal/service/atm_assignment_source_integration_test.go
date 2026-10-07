//go:build integration

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Vendor-wide (migration 023) kelolaan against real Postgres, in a rolled-back
// transaction: the DB constraints (avp_source_chk, no-overlap across modes) and
// every reader that must understand both package sources.
func TestIntegration_VendorWideAssignment(t *testing.T) {
	q, tx := setupForecastQueryHarness(t)
	ctx := context.Background()

	locID := seedForecastRegionAndLocation(t, tx, "VW1")
	var atmID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO atms (terminal_id, location_id, machine_type, brand, model, operation_hours, deployment_type, priority_class, is_active)
		VALUES ('ITEST-VW1', $1, 'ATM', 'TestBrand', 'TestModel', '24_HOURS', 'OFFSITE', 'Non VIP', true) RETURNING id`, locID).Scan(&atmID); err != nil {
		t.Fatalf("insert atm: %v", err)
	}
	var vendorID, branchID int64
	if err := tx.QueryRow(ctx, `INSERT INTO vendors (code, name) VALUES ('ITEST-VW1', 'Vendor Wide Test') RETURNING id`).Scan(&vendorID); err != nil {
		t.Fatalf("insert vendor: %v", err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, region) VALUES ($1, 'ITEST-VW1', 'Cabang VW', 'Region VW') RETURNING id`, vendorID).Scan(&branchID); err != nil {
		t.Fatalf("insert branch: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO vendor_package_prices (vendor_id, package, package_code, machine_group, price_class, tier_min, base_price, effective_start_date)
		VALUES ($1, 'PAKET 4', 'PKG4_ITESTVW1_001', 'ATM', 'REGULAR', 1, 1000, CURRENT_DATE - 30)`, vendorID); err != nil {
		t.Fatalf("insert tariff: %v", err)
	}

	asOf := toPgDate(time.Now())
	exists, err := q.VendorTariffExistsForATM(ctx, db.VendorTariffExistsForATMParams{VendorID: vendorID, Package: "PAKET 4", AtmID: atmID, AsOf: asOf})
	if err != nil || !exists {
		t.Fatalf("tariff should exist for PAKET 4: exists=%v err=%v", exists, err)
	}
	if exists, _ = q.VendorTariffExistsForATM(ctx, db.VendorTariffExistsForATMParams{VendorID: vendorID, Package: "PAKET 9", AtmID: atmID, AsOf: asOf}); exists {
		t.Error("PAKET 9 has no tariff, must not exist")
	}
	if opts, err := q.ListATMPackageOptions(ctx, db.ListATMPackageOptionsParams{VendorID: vendorID, AtmID: atmID, AsOf: asOf}); err != nil || len(opts) != 1 || opts[0].Package != "PAKET 4" || opts[0].PackageCode == "" {
		t.Fatalf("package options = %v err=%v, want [PAKET 4]", opts, err)
	}
	if _, err := q.CheckAssignmentVendorBranch(ctx, db.CheckAssignmentVendorBranchParams{VendorID: vendorID, VendorBranchID: branchID}); err != nil {
		t.Fatalf("branch of vendor should be valid: %v", err)
	}

	// avp_source_chk: both modes at once, and neither, are refused by the DB.
	pkgID := seedForecastVendorPackage(t, tx, "VW1B", "Vendor Branch Mode")
	start := toPgDate(time.Now().AddDate(0, 0, -1))
	for name, arg := range map[string]db.CreateATMAssignmentAdminParams{
		"both modes": {AtmID: atmID, VendorPackageID: &pkgID, VendorID: &vendorID, VendorBranchID: &branchID, Package: sp("PAKET 4"), EffectiveStartDate: start},
		"neither":    {AtmID: atmID, EffectiveStartDate: start},
	} {
		savepoint, err := tx.Begin(ctx)
		if err != nil {
			t.Fatalf("savepoint: %v", err)
		}
		_, err = db.New(savepoint).CreateATMAssignmentAdmin(ctx, arg)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
			t.Errorf("%s: want check violation 23514, got %v", name, err)
		}
		savepoint.Rollback(ctx)
	}

	created, err := q.CreateATMAssignmentAdmin(ctx, db.CreateATMAssignmentAdminParams{
		AtmID: atmID, VendorID: &vendorID, VendorBranchID: &branchID, Package: sp("PAKET 4"), EffectiveStartDate: start,
	})
	if err != nil {
		t.Fatalf("create vendor-wide assignment: %v", err)
	}

	// no-overlap holds across modes: a branch-mode period over the same dates is refused.
	savepoint, err := tx.Begin(ctx)
	if err != nil {
		t.Fatalf("savepoint: %v", err)
	}
	_, err = db.New(savepoint).CreateATMAssignmentAdmin(ctx, db.CreateATMAssignmentAdminParams{AtmID: atmID, VendorPackageID: &pkgID, EffectiveStartDate: start})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23P01" {
		t.Errorf("branch row over a vendor-wide period: want exclusion violation 23P01, got %v", err)
	}
	savepoint.Rollback(ctx)

	// admin list/get: one shape for both sources.
	list, err := q.ListATMAssignmentsAdmin(ctx, db.ListATMAssignmentsAdminParams{AtmID: atmID, Status: "all", PageLimit: 10})
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %d rows err=%v", len(list), err)
	}
	if r := list[0]; r.Source != "vendor" || r.PackageCode != "PAKET 4" || r.VendorPackageID != nil ||
		r.VendorBranchID == nil || *r.VendorBranchID != branchID || r.VendorID == nil || *r.VendorID != vendorID {
		t.Errorf("list row wrong: %+v", r)
	}
	if g, err := q.GetATMAssignmentAdminByID(ctx, created.ID); err != nil || g.Source != "vendor" || g.PackageCode != "PAKET 4" {
		t.Errorf("get row wrong: %+v err=%v", g, err)
	}

	// admin ATM list: current kelolaan.
	if atms, err := q.ListATMsAdmin(ctx, db.ListATMsAdminParams{Q: sp("ITEST-VW1"), Status: "all", PageLimit: 10}); err != nil || len(atms) != 1 ||
		atms[0].CurrentPackageCode != "PAKET 4" || atms[0].CurrentBranchCode != "ITEST-VW1" || atms[0].CurrentVendorCode != "ITEST-VW1" {
		t.Errorf("ATM list current kelolaan wrong: %+v err=%v", atms, err)
	}

	// kuota: label comes from the vendor-wide column, branch code is NULL.
	qa, err := q.ResolveAtmQuota(ctx, db.ResolveAtmQuotaParams{AsOfDate: asOf, TerminalID: "ITEST-VW1"})
	if err != nil || qa.PackageCode != nil || qa.VendorPackageLabel == nil || *qa.VendorPackageLabel != "PAKET 4" {
		t.Errorf("ResolveAtmQuota label wrong: %+v err=%v", qa, err)
	}
	if got := packageLabel(qa.PackageCode, qa.VendorPackageLabel); got == nil || *got != "PAKET 4" {
		t.Errorf("packageLabel = %v, want PAKET 4", got)
	}

	// vendor branch ATM tab.
	if rows, err := q.ListBranchATMs(ctx, db.ListBranchATMsParams{VendorBranchID: &branchID, PageLimit: 10}); err != nil || len(rows) != 1 || rows[0].PackageCode != "PAKET 4" {
		t.Errorf("ListBranchATMs wrong: %+v err=%v", rows, err)
	}
	if n, err := q.CountBranchATMs(ctx, &branchID); err != nil || n != 1 {
		t.Errorf("CountBranchATMs = %d err=%v, want 1", n, err)
	}

	// forecast browser: vendor/branch resolve and the paket label survives.
	forecastDate := time.Date(2027, 4, 1, 0, 0, 0, 0, time.UTC)
	seedForecastRow(t, tx, "ITEST-VW1", forecastDate, 50000)
	fr, err := q.ListForecastForDate(ctx, db.ListForecastForDateParams{ForecastDate: toPgDate(forecastDate), TerminalID: "ITEST-VW1", Page: 1, PageSize: 10})
	if err != nil || len(fr) != 1 {
		t.Fatalf("ListForecastForDate rows=%d err=%v", len(fr), err)
	}
	if r := fr[0]; r.FlmVendor == nil || *r.FlmVendor != "Vendor Wide Test" || r.FlmVendorRegion == nil || *r.FlmVendorRegion != "Region VW" ||
		r.Paket != nil || packageLabel(r.Paket, r.PaketVendor) == nil || *packageLabel(r.Paket, r.PaketVendor) != "PAKET 4" {
		t.Errorf("forecast row wrong: %+v", r)
	}

	// CSV export carries the managing branch + label + source.
	ex, err := q.ExportATMAssignmentsBatch(ctx, db.ExportATMAssignmentsBatchParams{AfterID: created.ID - 1, Status: "all", BatchSize: 1})
	if err != nil || len(ex) != 1 || ex[0].PackageSource != "vendor" || ex[0].PackageCode != "PAKET 4" || ex[0].BranchCode != "ITEST-VW1" || ex[0].VendorCode != "ITEST-VW1" {
		t.Errorf("export row wrong: %+v err=%v", ex, err)
	}
}
