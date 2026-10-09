//go:build integration

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

func applyPriceChange(t *testing.T, tx pgx.Tx, op string, entityID *int64, payload any) (int64, error) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	id, _, err := (VendorPackagePriceApplier{}).Apply(context.Background(), tx, db.MasterDataChangeRequest{Op: op, EntityID: entityID, Payload: raw})
	return id, err
}

func pricePayload(vendorID int64, label, class, base, start string) VendorPackagePricePayload {
	return VendorPackagePricePayload{VendorID: vendorID, VendorPackagePriceCreatePayload: VendorPackagePriceCreatePayload{
		Package: label, MachineGroup: "ATM", PriceClass: class, TierMin: 1, Currency: "IDR", EffectiveStartDate: start,
		VendorPackagePriceContentPayload: VendorPackagePriceContentPayload{BasePrice: &base},
	}}
}

// Apply-on-approve for vendor-wide prices (money, maker-checker): server-generated
// package_code, exact decimal base_price, update/disable, and the overlap guard.
func TestIntegration_VendorPackagePriceApplier(t *testing.T) {
	_, tx := setupForecastQueryHarness(t)
	ctx := context.Background()

	var vendorID int64
	if err := tx.QueryRow(ctx, `INSERT INTO vendors (code, name) VALUES ('ITPX', 'Vendor ITPX') RETURNING id`).Scan(&vendorID); err != nil {
		t.Fatalf("insert vendor: %v", err)
	}
	read := func(id int64) (code, base string, end *string) {
		t.Helper()
		if err := tx.QueryRow(ctx, `SELECT package_code, base_price::text, effective_end_date::text FROM vendor_package_prices WHERE id = $1`, id).Scan(&code, &base, &end); err != nil {
			t.Fatalf("read price %d: %v", id, err)
		}
		return
	}

	// create: package_code is generated on the apply tx, sequence per vendor.
	first, err := applyPriceChange(t, tx, "create", nil, pricePayload(vendorID, "PAKET 3", "REGULAR", "1250000.50", "2027-01-01"))
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	if code, base, end := read(first); code != "PKG3_ITPX_001" || base != "1250000.50" || end != nil {
		t.Errorf("first = (%s, %s, %v), want (PKG3_ITPX_001, 1250000.50, nil)", code, base, end)
	}
	second, err := applyPriceChange(t, tx, "create", nil, pricePayload(vendorID, "PAKET KHUSUS", "VIP_INDUSTRI", "99", "2020-01-01"))
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	if code, _, _ := read(second); code != "PKG0_ITPX_002" {
		t.Errorf("second package_code = %s, want PKG0_ITPX_002", code)
	}

	// update: only content fields; money stays exact.
	newBase, newEnd := "1300000.25", "2027-12-31"
	if _, err := applyPriceChange(t, tx, "update", &first, VendorPackagePriceContentPayload{BasePrice: &newBase, EffectiveEndDate: &newEnd}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if code, base, end := read(first); code != "PKG3_ITPX_001" || base != "1300000.25" || end == nil || *end != newEnd {
		t.Errorf("after update = (%s, %s, %v)", code, base, end)
	}

	// disable closes a running period (end = yesterday) instead of deleting the row.
	// A period starting today or later cannot be disabled yet: CURRENT_DATE-1 <
	// start violates vpp_period_chk -- tracked as backend_QC_fixes.md F17.
	if _, err := applyPriceChange(t, tx, "disable", &second, map[string]any{}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, _, end := read(second); end == nil {
		t.Error("disable must set effective_end_date")
	}

	// CurrentState returns the row used as the maker-checker "before" snapshot.
	state, err := (VendorPackagePriceApplier{}).CurrentState(ctx, tx, first)
	if row, ok := state.(db.GetVendorPackagePriceAdminByIDRow); err != nil || !ok || row.ID != first {
		t.Errorf("CurrentState = %#v, %v", state, err)
	}

	// overlap with the first period (same grain) is refused as ErrVendorPackagePriceOverlap.
	sp, err := tx.Begin(ctx)
	if err != nil {
		t.Fatalf("savepoint: %v", err)
	}
	defer func() { _ = sp.Rollback(ctx) }()
	if _, err := applyPriceChange(t, sp, "create", nil, pricePayload(vendorID, "PAKET 3", "REGULAR", "1", "2027-06-01")); !errors.Is(err, ErrVendorPackagePriceOverlap) {
		t.Errorf("overlapping create: got %v, want ErrVendorPackagePriceOverlap", err)
	}
}
