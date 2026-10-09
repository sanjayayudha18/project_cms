//go:build integration

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// CurrentState feeds the stale check at apply time (maker-checker): it must load
// the same row shape the admin service stored as Before, and surface a missing
// row as pgx.ErrNoRows so the approval is rejected, not applied blindly.
func TestIntegration_AppliersCurrentState(t *testing.T) {
	q, tx := setupForecastQueryHarness(t)
	ctx := context.Background()

	atmID, vendorID, branchID := seedVendorWide(t, tx, "CS1")
	packageID := seedBranchPackage(t, tx, vendorID, "CS2", nil)
	vaultID := seedRow(t, tx, `INSERT INTO vendor_vaults (vendor_branch_id, vault_code, type, category, currency_code)
		VALUES ($1, 'ITEST-CS-V', 'CASH', 'CASH', 'IDR') RETURNING id`, branchID)
	picID := seedRow(t, tx, `INSERT INTO vendor_pics (vendor_id, name, email) VALUES ($1, 'pic cs', 'pic.cs@vendor.test') RETURNING id`, vendorID)
	mapID := seedRow(t, tx, `INSERT INTO dsr_location_vault_maps (vendor_id, dsr_location, vendor_vault_id) VALUES ($1, 'ITEST CS', $2) RETURNING id`, vendorID, vaultID)
	assignment, err := q.CreateATMAssignmentAdmin(ctx, db.CreateATMAssignmentAdminParams{
		AtmID: atmID, VendorID: &vendorID, VendorBranchID: &branchID, Package: sp("PAKET 4"),
		EffectiveStartDate: toPgDate(time.Now().AddDate(0, 0, -1)),
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	cases := []struct {
		name    string
		applier Applier
		id      int64
		check   func(any) bool
	}{
		{"atm assignment", ATMAssignmentApplier{}, assignment.ID, func(v any) bool {
			r, ok := v.(db.GetATMAssignmentAdminByIDRow)
			return ok && r.ID == assignment.ID
		}},
		{"dsr location map", DsrLocationMapApplier{}, mapID, func(v any) bool {
			r, ok := v.(db.GetDsrLocationMapAdminByIDRow)
			return ok && r.ID == mapID
		}},
		{"vendor branch", VendorBranchApplier{}, branchID, func(v any) bool {
			r, ok := v.(db.GetVendorBranchAdminByIDRow)
			return ok && r.ID == branchID
		}},
		{"vendor package", VendorPackageApplier{}, packageID, func(v any) bool {
			r, ok := v.(db.GetVendorPackageAdminByIDRow)
			return ok && r.ID == packageID
		}},
		{"vendor pic", VendorPicApplier{}, picID, func(v any) bool {
			r, ok := v.(db.GetVendorPicAdminByIDRow)
			return ok && r.ID == picID
		}},
		{"vendor vault", VendorVaultApplier{}, vaultID, func(v any) bool {
			r, ok := v.(db.GetVendorVaultAdminByIDRow)
			return ok && r.ID == vaultID
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.applier.CurrentState(ctx, tx, tc.id)
			if err != nil || !tc.check(got) {
				t.Fatalf("CurrentState(%d) = %#v, %v", tc.id, got, err)
			}
			if _, err := tc.applier.CurrentState(ctx, tx, -1); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("CurrentState(missing) err = %v, want pgx.ErrNoRows", err)
			}
		})
	}
}
