//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Submit-time guards of the master-data admin screens: overlap / active checks
// for ATM assignments, DSR label uniqueness, branch-disable counts, ownership
// lookups and the import reference sets. Real DB, tx rolled back.
func TestAdminGuardQueries(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()
	tag := time.Now().Format("150405.000000")
	seed := func(sql string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := tx.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
			t.Fatalf("seed %q: %v", sql, err)
		}
		return id
	}
	eq := func(name string, got *int64, err error, want int64) {
		t.Helper()
		if err != nil || got == nil || *got != want {
			t.Errorf("%s = %v, %v, want %d", name, got, err, want)
		}
	}
	isNil := func(name string, got *int64, err error) {
		t.Helper()
		if got != nil || err != nil {
			t.Errorf("%s = %v, %v, want nil, nil", name, got, err)
		}
	}

	vendorID := seed(`INSERT INTO vendors (code, name) VALUES ($1, $1) RETURNING id`, "ITG-"+tag)
	branchID := seed(`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name) VALUES ($1, $2, 'B') RETURNING id`, vendorID, "ITGB-"+tag)
	vaultCode := "ITGV-" + tag
	vaultID := seed(`INSERT INTO vendor_vaults (vendor_branch_id, vault_code, type, category, currency_code) VALUES ($1, $2, 'CASH', 'CASH', 'IDR') RETURNING id`, branchID, vaultCode)
	seed(`INSERT INTO vendor_pics (vendor_id, vendor_branch_id, name, email, is_notification_recipient) VALUES ($1, $2, 'pic', 'pic@test.local', true) RETURNING id`, vendorID, branchID)
	pkgID := seed(`INSERT INTO vendor_packages_branch (vendor_branch_id, package_code, machine_group, price_class, tier_min, base_price, currency, effective_start_date)
		VALUES ($1, 'PAKET G', 'ATM', 'REGULAR', 1, 0, 'IDR', CURRENT_DATE - 30) RETURNING id`, branchID)
	endedPkgID := seed(`INSERT INTO vendor_packages_branch (vendor_branch_id, package_code, machine_group, price_class, tier_min, base_price, currency, effective_start_date, effective_end_date)
		VALUES ($1, 'PAKET LAMA', 'ATM', 'REGULAR', 1, 0, 'IDR', CURRENT_DATE - 60, CURRENT_DATE - 31) RETURNING id`, branchID)
	locID := seed(`INSERT INTO locations (region_id, type, name, address_line1, city_or_regency, province, country_code)
		SELECT id, 'branch', $1, 'Jl. Test 1', 'Jakarta', 'DKI Jakarta', 'ID' FROM regions ORDER BY id LIMIT 1 RETURNING id`, "ITG Loc "+tag)
	atmID := seed(`INSERT INTO atms (terminal_id, location_id, machine_type, brand, model, operation_hours, deployment_type, is_active)
		VALUES ($1, $2, 'ATM', 'NCR', 'SelfServ', '24 Hours', 'Onsite', true) RETURNING id`, "ITG-"+tag, locID)
	assignID := seed(`INSERT INTO atm_vendor_packages (atm_id, vendor_package_id, effective_start_date) VALUES ($1, $2, CURRENT_DATE - 10) RETURNING id`, atmID, pkgID)
	mapID := seed(`INSERT INTO dsr_location_vault_maps (vendor_id, dsr_location, vendor_vault_id) VALUES ($1, 'Lenteng Agung', $2) RETURNING id`, vendorID, vaultID)

	t.Run("atm assignment", func(t *testing.T) {
		r := NewATMAssignmentAdminRepository(tx, tx)
		today := time.Now()
		yearAgo, monthAgo := today.AddDate(-1, 0, 0), today.AddDate(0, -1, 0)
		for _, tc := range []struct {
			name    string
			exclude int64
			start   time.Time
			end     *time.Time
			want    bool
		}{
			{"open-ended overlaps", 0, today, nil, true},
			{"own row excluded", assignID, today, nil, false},
			{"period before start", 0, yearAgo, &monthAgo, false},
		} {
			if got, err := r.HasOverlap(ctx, atmID, tc.exclude, tc.start, tc.end); err != nil || got != tc.want {
				t.Errorf("HasOverlap %s = %v, %v, want %v", tc.name, got, err, tc.want)
			}
		}
		if ok, err := r.ATMActive(ctx, atmID); err != nil || !ok {
			t.Errorf("ATMActive = %v, %v", ok, err)
		}
		if ok, err := r.ATMActive(ctx, -1); err != nil || ok {
			t.Errorf("ATMActive(missing) = %v, %v", ok, err)
		}
		for id, want := range map[int64]bool{pkgID: true, endedPkgID: false, -1: false} {
			if ok, err := r.PackageActive(ctx, id); err != nil || ok != want {
				t.Errorf("PackageActive(%d) = %v, %v, want %v", id, ok, err, want)
			}
		}
		if _, err := r.GetATMPriceGroup(ctx, atmID); err != nil {
			t.Errorf("GetATMPriceGroup: %v", err)
		}
		if id, err := r.CheckAssignmentVendorBranch(ctx, db.CheckAssignmentVendorBranchParams{VendorID: vendorID, VendorBranchID: branchID}); err != nil || id != branchID {
			t.Errorf("CheckAssignmentVendorBranch = %d, %v, want %d", id, err, branchID)
		}
		asOf := pgtype.Date{Time: today, Valid: true}
		if ok, err := r.VendorTariffExistsForATM(ctx, db.VendorTariffExistsForATMParams{VendorID: vendorID, AtmID: atmID, Package: "PAKET X", AsOf: asOf}); err != nil || ok {
			t.Errorf("VendorTariffExistsForATM(no tariff) = %v, %v", ok, err)
		}
		if rows, err := r.ListATMPackageOptions(ctx, db.ListATMPackageOptionsParams{AtmID: atmID, VendorID: vendorID, AsOf: asOf}); err != nil || len(rows) != 0 {
			t.Errorf("ListATMPackageOptions(no tariff) = %v, %v", rows, err)
		}
	})

	t.Run("dsr location map", func(t *testing.T) {
		r := NewDsrLocationMapAdminRepository(tx, tx)
		if row, err := r.GetByID(ctx, mapID); err != nil || row == nil || row.VendorVaultID != vaultID {
			t.Errorf("GetByID = %+v, %v", row, err)
		}
		if row, err := r.GetByID(ctx, -1); err != nil || row != nil {
			t.Errorf("GetByID(missing) = %+v, %v", row, err)
		}
		id, err := r.FindActive(ctx, vendorID, "LENTENG agung", 0) // label match is case-insensitive
		eq("FindActive", id, err, mapID)
		id, err = r.FindActive(ctx, vendorID, "Lenteng Agung", mapID)
		isNil("FindActive(excluding self)", id, err)
		id, err = r.ActiveVaultVendorID(ctx, vaultID)
		eq("ActiveVaultVendorID", id, err, vendorID)
		id, err = r.ActiveVaultVendorID(ctx, -1)
		isNil("ActiveVaultVendorID(missing)", id, err)
		if rows, err := r.List(ctx, db.ListDsrLocationMapsAdminParams{VendorID: vendorID, Status: "active"}); err != nil || len(rows) != 1 {
			t.Errorf("List = %v, %v", rows, err)
		}
		if _, err := r.ListUnmapped(ctx, vendorID); err != nil {
			t.Errorf("ListUnmapped: %v", err)
		}
	})

	t.Run("branch children and ownership", func(t *testing.T) {
		vault, pic, pkg := NewVendorVaultAdminRepository(tx, tx), NewVendorPicAdminRepository(tx, tx), NewVendorPackageAdminRepository(tx, tx)
		counts := map[string]func(context.Context, int64) (int64, error){
			"vault": vault.CountActiveByBranch, "pic": pic.CountActiveByBranch, "package": pkg.CountActiveByBranch,
		}
		for name, count := range counts {
			if n, err := count(ctx, branchID); err != nil || n < 1 {
				t.Errorf("%s CountActiveByBranch = %d, %v", name, n, err)
			}
		}
		if n, err := pic.CountNotificationRecipients(ctx, vendorID); err != nil || n != 1 {
			t.Errorf("CountNotificationRecipients = %d, %v", n, err)
		}
		owners := map[string]func(context.Context, int64) (*int64, error){
			"vault": vault.BranchVendorID, "pic": pic.BranchVendorID, "package": pkg.BranchVendorID,
		}
		for name, owner := range owners {
			id, err := owner(ctx, branchID)
			eq(name+" BranchVendorID", id, err, vendorID)
			id, err = owner(ctx, -1)
			isNil(name+" BranchVendorID(missing)", id, err)
		}
		id, err := vault.FindByCode(ctx, vaultCode)
		eq("FindByCode", id, err, vaultID)
		id, err = vault.FindByCode(ctx, "nope-"+vaultCode)
		isNil("FindByCode(missing)", id, err)

		price := NewVendorPackagePriceAdminRepository(tx, tx)
		if k, err := price.VendorKind(ctx, vendorID); err != nil || k == nil || k.Kind != "FLM_VENDOR" || !k.IsActive {
			t.Errorf("VendorKind = %+v, %v", k, err)
		}
		if k, err := price.VendorKind(ctx, -1); err != nil || k != nil {
			t.Errorf("VendorKind(missing) = %+v, %v", k, err)
		}
	})

	t.Run("user employee id", func(t *testing.T) {
		emp := "ITG-EMP-" + tag
		userID := seed(`INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source, employee_id)
			SELECT id, $1, 'IT', $1 || '@test.local', true, 'ldap', $2 FROM roles ORDER BY id LIMIT 1 RETURNING id`, "itg-"+tag, emp)
		r := NewUserAdminRepository(tx)
		id, err := r.FindByEmployeeID(ctx, emp)
		eq("FindByEmployeeID", id, err, userID)
		id, err = r.FindByEmployeeID(ctx, "nope-"+emp)
		isNil("FindByEmployeeID(missing)", id, err)
		id, err = r.FindByEmployeeIDExcludingID(ctx, emp, userID)
		isNil("FindByEmployeeIDExcludingID(self)", id, err)
		id, err = r.FindByEmployeeIDExcludingID(ctx, emp, 0)
		eq("FindByEmployeeIDExcludingID(other)", id, err, userID)
	})

	t.Run("import reference sets", func(t *testing.T) {
		r := NewMasterDataExportRepository(tx)
		if _, err := r.PackageLabels(ctx); err != nil {
			t.Errorf("PackageLabels: %v", err)
		}
		if keys, err := r.PackageKeys(ctx); err != nil || len(keys) == 0 {
			t.Errorf("PackageKeys = %d rows, %v", len(keys), err)
		}
		ids, err := r.LocationIDs(ctx)
		found := false
		for _, id := range ids {
			found = found || id == locID
		}
		if err != nil || !found {
			t.Errorf("LocationIDs missing seeded location %d (err %v)", locID, err)
		}
	})
}
