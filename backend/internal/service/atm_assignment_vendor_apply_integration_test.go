//go:build integration

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// seedVendorWide inserts an ATM, an FLM vendor with one active cabang and a PT-level
// PAKET 4 tariff matching the ATM (ATM / REGULAR); returns their ids.
func seedVendorWide(t *testing.T, tx pgx.Tx, marker string) (atmID, vendorID, branchID int64) {
	t.Helper()
	ctx := context.Background()
	locID := seedForecastRegionAndLocation(t, tx, marker)
	if err := tx.QueryRow(ctx, `
		INSERT INTO atms (terminal_id, location_id, machine_type, brand, model, operation_hours, deployment_type, priority_class, is_active)
		VALUES ($1, $2, 'ATM', 'TestBrand', 'TestModel', '24_HOURS', 'OFFSITE', 'Non VIP', true) RETURNING id`, "ITEST-"+marker, locID).Scan(&atmID); err != nil {
		t.Fatalf("insert atm: %v", err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO vendors (code, name) VALUES ($1, $2) RETURNING id`, "ITEST-"+marker, "Vendor "+marker).Scan(&vendorID); err != nil {
		t.Fatalf("insert vendor: %v", err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, region) VALUES ($1, $2, 'Cabang', 'Region '||$2) RETURNING id`, vendorID, "ITEST-"+marker).Scan(&branchID); err != nil {
		t.Fatalf("insert branch: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO vendor_package_prices (vendor_id, package, package_code, machine_group, price_class, tier_min, base_price, effective_start_date)
		VALUES ($1, 'PAKET 4', $2, 'ATM', 'REGULAR', 1, 1000, CURRENT_DATE - 30)`, vendorID, "PKG4_"+marker); err != nil {
		t.Fatalf("insert tariff: %v", err)
	}
	return atmID, vendorID, branchID
}

// The forecast count/summary and the vendor-of-terminal resolver (Vendor Request
// create) must see a vendor-wide ATM exactly like a branch-package ATM.
func TestIntegration_VendorWideAssignment_ForecastCountSummaryAndVendorResolver(t *testing.T) {
	q, tx := setupForecastQueryHarness(t)
	ctx := context.Background()
	atmID, vendorID, branchID := seedVendorWide(t, tx, "VW2")
	if _, err := q.CreateATMAssignmentAdmin(ctx, db.CreateATMAssignmentAdminParams{
		AtmID: atmID, VendorID: &vendorID, VendorBranchID: &branchID, Package: sp("PAKET 4"),
		EffectiveStartDate: toPgDate(time.Now().AddDate(0, 0, -1)),
	}); err != nil {
		t.Fatalf("create assignment: %v", err)
	}
	forecastDate := time.Date(2027, 5, 1, 0, 0, 0, 0, time.UTC)
	seedForecastRow(t, tx, "ITEST-VW2", forecastDate, 50000)

	n, err := q.CountForecastForDate(ctx, db.CountForecastForDateParams{ForecastDate: toPgDate(forecastDate), TerminalID: "ITEST-VW2", FlmVendor: "Vendor VW2", FlmVendorRegion: "Region ITEST-VW2"})
	if err != nil || n != 1 {
		t.Errorf("CountForecastForDate filtered by the vendor-wide vendor/region = %d err=%v, want 1", n, err)
	}
	unassigned, err := q.CountForecastForDate(ctx, db.CountForecastForDateParams{ForecastDate: toPgDate(forecastDate), TerminalID: "ITEST-VW2", Unassigned: true})
	if err != nil || unassigned != 0 {
		t.Errorf("a vendor-wide ATM must not count as unassigned: %d err=%v", unassigned, err)
	}

	rows, err := q.SummarizeForecastForDate(ctx, toPgDate(forecastDate))
	if err != nil {
		t.Fatalf("SummarizeForecastForDate: %v", err)
	}
	found := false
	for _, r := range rows {
		if r.VendorID == vendorID {
			found = true
			if r.FlmVendorRegion != "Region ITEST-VW2" || r.AtmCount != 1 || r.AmountReplenish <= 0 || r.UnrequestedAmountReplenish != r.AmountReplenish {
				t.Errorf("summary row wrong: %+v", r)
			}
		}
	}
	if !found {
		t.Errorf("summary has no row for the vendor-wide vendor: %+v", rows)
	}

	got, err := q.GetActiveVendorForTerminal(ctx, db.GetActiveVendorForTerminalParams{AsOfDate: toPgDate(time.Now()), TerminalID: "ITEST-VW2"})
	if err != nil || got.VendorID != vendorID {
		t.Errorf("GetActiveVendorForTerminal = %d err=%v, want %d", got.VendorID, err, vendorID)
	}
}

// The apply-on-approve path against real Postgres: automatic period for a
// vendor-wide package with handover from a running branch period, an in-mode
// label update, and re-validation at apply (tariff gone -> ErrATMAssignmentSourceInvalid).
func TestIntegration_VendorWideAssignment_Applier(t *testing.T) {
	q, tx := setupForecastQueryHarness(t)
	ctx := context.Background()
	atmID, vendorID, branchID := seedVendorWide(t, tx, "VW3")

	change := func(op string, entityID *int64, payload any) db.MasterDataChangeRequest {
		raw, _ := json.Marshal(payload)
		return db.MasterDataChangeRequest{EntityType: "atm_assignment", Op: op, EntityID: entityID, Payload: raw}
	}
	vendorPayload := ATMAssignmentPayload{ATMID: atmID, ATMAssignmentUpdatePayload: ATMAssignmentUpdatePayload{
		Source: AssignmentSourceVendor, VendorID: vendorID, VendorBranchID: branchID, Package: "PAKET 4"}}

	// A running branch-package period is handed over by the automatic vendor-wide period.
	pkgID := seedForecastVendorPackage(t, tx, "VW3B", "Vendor Branch Mode")
	if _, err := q.CreateATMAssignmentAdmin(ctx, db.CreateATMAssignmentAdminParams{
		AtmID: atmID, VendorPackageID: &pkgID, EffectiveStartDate: toPgDate(time.Now().AddDate(0, 0, -10)),
	}); err != nil {
		t.Fatalf("seed running branch period: %v", err)
	}
	id, _, err := ATMAssignmentApplier{}.Apply(ctx, tx, change("create", nil, vendorPayload))
	if err != nil {
		t.Fatalf("apply create vendor-wide: %v", err)
	}
	list, err := q.ListATMAssignmentsAdmin(ctx, db.ListATMAssignmentsAdminParams{AtmID: atmID, Status: "all", PageLimit: 10})
	if err != nil || len(list) != 2 {
		t.Fatalf("want 2 periods after handover, got %d err=%v", len(list), err)
	}
	if list[0].ID != id || list[0].Source != "vendor" || list[0].EffectiveEndDate.Valid {
		t.Errorf("new vendor-wide period must be open-ended: %+v", list[0])
	}
	if list[1].Source != "branch" || !list[1].EffectiveEndDate.Valid {
		t.Errorf("running branch period must be closed by the handover: %+v", list[1])
	}

	// In-mode update: the label may change while the tariff exists.
	if _, err := tx.Exec(ctx, `
		INSERT INTO vendor_package_prices (vendor_id, package, package_code, machine_group, price_class, tier_min, base_price, effective_start_date)
		VALUES ($1, 'PAKET 5', 'PKG5_VW3', 'ATM', 'REGULAR', 1, 1000, CURRENT_DATE - 30)`, vendorID); err != nil {
		t.Fatalf("insert second tariff: %v", err)
	}
	upd := ATMAssignmentUpdatePayload{Source: AssignmentSourceVendor, VendorID: vendorID, VendorBranchID: branchID, Package: "PAKET 5",
		EffectiveStartDate: time.Now().Format(assignmentDateLayout)}
	if _, _, err := (ATMAssignmentApplier{}).Apply(ctx, tx, change("update", &id, upd)); err != nil {
		t.Fatalf("apply update label: %v", err)
	}
	if g, err := q.GetATMAssignmentAdminByID(ctx, id); err != nil || g.PackageCode != "PAKET 5" || g.Source != "vendor" {
		t.Errorf("label not updated: %+v err=%v", g, err)
	}

	// A payload that would switch the row to branch mode is refused at apply, never half-applied.
	modeSwitch := ATMAssignmentUpdatePayload{VendorPackageID: pkgID, EffectiveStartDate: time.Now().Format(assignmentDateLayout)}
	sw, err := tx.Begin(ctx)
	if err != nil {
		t.Fatalf("savepoint: %v", err)
	}
	if _, _, err := (ATMAssignmentApplier{}).Apply(ctx, sw, change("update", &id, modeSwitch)); !errors.Is(err, ErrATMAssignmentSourceInvalid) {
		t.Errorf("want ErrATMAssignmentSourceInvalid for a mode switch at apply, got %v", err)
	}
	sw.Rollback(ctx)

	// Tariff disappears between submit and approve -> apply refuses with the 409 sentinel.
	if _, err := tx.Exec(ctx, `DELETE FROM vendor_package_prices WHERE vendor_id = $1 AND package = 'PAKET 4'`, vendorID); err != nil {
		t.Fatalf("delete tariff: %v", err)
	}
	savepoint, err := tx.Begin(ctx)
	if err != nil {
		t.Fatalf("savepoint: %v", err)
	}
	defer savepoint.Rollback(ctx)
	if _, _, err := (ATMAssignmentApplier{}).Apply(ctx, savepoint, change("create", nil, vendorPayload)); !errors.Is(err, ErrATMAssignmentSourceInvalid) {
		t.Errorf("want ErrATMAssignmentSourceInvalid when the tariff is gone, got %v", err)
	}
}
