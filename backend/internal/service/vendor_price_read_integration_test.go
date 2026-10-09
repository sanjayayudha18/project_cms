//go:build integration

package service

import (
	"context"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/repository"
)

// Admin price screens read both price tables through the real repositories:
// money comes back as an exact decimal string (never float), an expired row
// keeps its end date, and another vendor's row is invisible (nil, nil).
func TestIntegration_VendorPriceReads(t *testing.T) {
	_, tx := setupForecastQueryHarness(t)
	ctx := context.Background()
	seed := func(sql string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := tx.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
			t.Fatalf("seed %q: %v", sql, err)
		}
		return id
	}
	vendorID := seed(`INSERT INTO vendors (code, name) VALUES ('ITPR', 'Vendor ITPR') RETURNING id`)
	otherVendor := seed(`INSERT INTO vendors (code, name) VALUES ('ITPR2', 'Vendor ITPR2') RETURNING id`)
	branchID := seed(`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name) VALUES ($1, 'ITPR-B', 'B') RETURNING id`, vendorID)
	pkgID := seed(`INSERT INTO vendor_packages_branch (vendor_branch_id, package_code, machine_group, price_class, tier_min, tier_max, base_price, currency, effective_start_date, effective_end_date)
		VALUES ($1, 'PAKET C', 'ATM', 'REGULAR', 1, 5, 1250000.50, 'IDR', '2026-01-01', '2026-06-30') RETURNING id`, branchID)
	priceID := seed(`INSERT INTO vendor_package_prices (vendor_id, package, package_code, machine_group, price_class, tier_min, base_price, effective_start_date)
		VALUES ($1, 'PAKET 3', 'PKG3_ITPR_001', 'ATM', 'REGULAR', 1, 99999999.99, '2026-01-01') RETURNING id`, vendorID)

	t.Run("branch special price", func(t *testing.T) {
		svc := NewVendorPackageAdminService(repository.NewVendorPackageAdminRepository(tx, tx), nil)
		list, err := svc.List(ctx, db.ListVendorPackagesAdminParams{VendorID: vendorID, Status: "all", PageLimit: 10})
		if err != nil || len(list) != 1 {
			t.Fatalf("List = %+v, %v", list, err)
		}
		p := list[0]
		if p.ID != pkgID || p.BasePrice == nil || *p.BasePrice != "1250000.50" || p.TierMax == nil || *p.TierMax != 5 ||
			p.EffectiveStartDate != "2026-01-01" || p.EffectiveEndDate == nil || *p.EffectiveEndDate != "2026-06-30" {
			t.Errorf("package = %+v", p)
		}
		if n, err := svc.Count(ctx, db.CountVendorPackagesAdminParams{VendorID: vendorID, Status: "active"}); err != nil || n != 0 {
			t.Errorf("Count(active) = %d, %v, want 0 (period ended)", n, err)
		}
		if got, err := svc.Get(ctx, vendorID, pkgID); err != nil || got == nil || *got.BasePrice != "1250000.50" {
			t.Errorf("Get = %+v, %v", got, err)
		}
		if got, err := svc.Get(ctx, otherVendor, pkgID); err != nil || got != nil {
			t.Errorf("Get by another vendor = %+v, %v, want nil, nil", got, err)
		}
	})

	t.Run("vendor-wide price", func(t *testing.T) {
		svc := NewVendorPackagePriceAdminService(repository.NewVendorPackagePriceAdminRepository(tx, tx), nil)
		list, err := svc.List(ctx, db.ListVendorPackagePricesAdminParams{VendorID: vendorID, Status: "active", PageLimit: 10})
		if err != nil || len(list) != 1 {
			t.Fatalf("List = %+v, %v", list, err)
		}
		p := list[0]
		if p.ID != priceID || p.PackageCode != "PKG3_ITPR_001" || p.BasePrice == nil || *p.BasePrice != "99999999.99" || p.TierMax != nil || p.EffectiveEndDate != nil {
			t.Errorf("price = %+v", p)
		}
		if n, err := svc.Count(ctx, db.CountVendorPackagePricesAdminParams{VendorID: vendorID, Status: "all"}); err != nil || n != 1 {
			t.Errorf("Count = %d, %v", n, err)
		}
		if got, err := svc.Get(ctx, vendorID, priceID); err != nil || got == nil || got.Package != "PAKET 3" {
			t.Errorf("Get = %+v, %v", got, err)
		}
		if got, err := svc.Get(ctx, otherVendor, priceID); err != nil || got != nil {
			t.Errorf("Get by another vendor = %+v, %v, want nil, nil", got, err)
		}
	})
}
