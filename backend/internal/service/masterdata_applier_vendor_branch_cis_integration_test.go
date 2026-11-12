//go:build integration

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// R1.5 & R2.1 (migration 040): cis_limit_amount and cis_limit_currency moved from
// vendor_branches to vendor_vaults and go through VendorVaultApplier.
func TestVendorVaultApplier_CisLimit_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()
	q := dbtx(svc)

	vendorID := h.seedVendor("ITCIS-"+tag, "Vendor CIS "+tag)
	branchID := h.seedVendorBranch(vendorID, "ITCISB-"+tag, "Branch CIS "+tag)
	vaultID := h.seedVault(branchID, "ITCISV-"+tag)

	readLimit := func() *string {
		t.Helper()
		var s *string
		var cur string
		if err := q.QueryRow(ctx, `SELECT cis_limit_amount::text, cis_limit_currency FROM vendor_vaults WHERE id = $1`, vaultID).Scan(&s, &cur); err != nil {
			t.Fatal(err)
		}
		if cur != "IDR" {
			t.Errorf("currency = %q, want IDR", cur)
		}
		return s
	}
	update := func(limit *string) error {
		before, err := db.New(q).GetVendorVaultAdminByID(ctx, vaultID) // same shape as CurrentState
		if err != nil {
			t.Fatal(err)
		}
		return approveChange(t, svc, h.insertPendingWithBefore("vendor_vault", "update", &vaultID,
			VendorVaultUpdatePayload{Category: "ATM", CurrencyCode: "IDR", CisLimitAmount: limit}, before))
	}

	if got := readLimit(); got != nil {
		t.Fatalf("new vault limit = %v, want NULL", *got)
	}
	if err := update(sp("2500000000")); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	if got := readLimit(); got == nil || *got != "2500000000.00" {
		t.Fatalf("limit = %v, want 2500000000.00", showStr(got))
	}
	if err := update(nil); err != nil {
		t.Fatalf("clear limit: %v", err)
	}
	if got := readLimit(); got != nil {
		t.Fatalf("cleared limit = %v, want NULL", *got)
	}
	if err := update(sp("750000000.50")); err != nil {
		t.Fatalf("set limit again: %v", err)
	}

	// Stale Before snapshot -> rejected, limit untouched.
	oldBefore := map[string]any{
		"id": vaultID, "vendor_branch_id": branchID, "vault_code": "ITCISV-" + tag,
		"category": "ATM", "currency_code": "IDR", "is_active": true, "deleted_at": nil,
	}
	err := approveChange(t, svc, h.insertPendingWithBefore("vendor_vault", "update", &vaultID,
		VendorVaultUpdatePayload{Category: "ATM", CurrencyCode: "IDR"}, oldBefore))
	if !errors.Is(err, ErrMasterDataChangeStale) {
		t.Fatalf("stale before update: err = %v, want stale", err)
	}
	if got := readLimit(); got == nil || *got != "750000000.50" {
		t.Fatalf("limit after stale change = %v, want 750000000.50", showStr(got))
	}

	// create carries cis_limit_amount too
	if err := approveChange(t, svc, h.insertPending("vendor_vault", "create", nil, VendorVaultPayload{
		VendorBranchID: branchID,
		VaultCode:      "ITCISC-" + tag,
		VendorVaultUpdatePayload: VendorVaultUpdatePayload{
			Category:       "CASH",
			CurrencyCode:   "IDR",
			CisLimitAmount: sp("1000"),
		},
	})); err != nil {
		t.Fatalf("create: %v", err)
	}
	var createdID int64
	var created string
	if err := q.QueryRow(ctx, `SELECT id, cis_limit_amount::text FROM vendor_vaults WHERE vault_code = $1`, "ITCISC-"+tag).Scan(&createdID, &created); err != nil || created != "1000.00" {
		t.Fatalf("created limit = %q (err %v), want 1000.00", created, err)
	}
	h.vendorVaultIDs = append(h.vendorVaultIDs, createdID)

	// DB CHECK is the last net.
	if _, err := q.Exec(ctx, `UPDATE vendor_vaults SET cis_limit_amount = 0 WHERE id = $1`, vaultID); err == nil {
		t.Error("CHECK vendor_vaults_cis_limit_amount_chk: want error for 0")
	}
}
