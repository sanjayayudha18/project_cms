//go:build integration

package service

import (
	"context"
	"testing"
)

// vendor-cis-limit FR2.1–2.2: escrow goes through the vendor_vault Applier, is not unique,
// and the DB CHECK refuses anything but 12 digits.
func TestVendorVaultApplier_Escrow_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()
	q := dbtx(svc)

	vendorID := h.seedVendor("ITESC-"+tag, "Vendor Escrow "+tag)
	branchID := h.seedVendorBranch(vendorID, "ITESCB-"+tag, "Branch Escrow "+tag)
	other := h.seedVault(branchID, "ITESC-VO-"+tag)
	if _, err := q.Exec(ctx, `UPDATE vendor_vaults SET escrow_account = '001234567890' WHERE id = $1`, other); err != nil {
		t.Fatalf("seed escrow: %v", err)
	}

	fields := VendorVaultUpdatePayload{Category: "ATM", CurrencyCode: "IDR", EscrowAccount: sp("001234567890")}
	if err := approveChange(t, svc, h.insertPending("vendor_vault", "create", nil,
		VendorVaultPayload{VendorBranchID: branchID, VaultCode: "ITESC-V1-" + tag, VendorVaultUpdatePayload: fields})); err != nil {
		t.Fatalf("create with an escrow another vault already uses (not unique): %v", err)
	}
	var vaultID int64
	var escrow *string
	if err := q.QueryRow(ctx, `SELECT id, escrow_account FROM vendor_vaults WHERE vault_code = $1`, "ITESC-V1-"+tag).Scan(&vaultID, &escrow); err != nil {
		t.Fatal(err)
	}
	h.vendorVaultIDs = append(h.vendorVaultIDs, vaultID)
	if showStr(escrow) != "001234567890" {
		t.Fatalf("escrow = %s, want 001234567890 (leading zeros kept)", showStr(escrow))
	}

	fields.EscrowAccount = nil
	if err := approveChange(t, svc, h.insertPending("vendor_vault", "update", &vaultID, fields)); err != nil {
		t.Fatalf("clear escrow: %v", err)
	}
	if err := q.QueryRow(ctx, `SELECT escrow_account FROM vendor_vaults WHERE id = $1`, vaultID).Scan(&escrow); err != nil || escrow != nil {
		t.Fatalf("cleared escrow = %s (err %v), want NULL", showStr(escrow), err)
	}

	for _, bad := range []string{"12345678901", "12345678901A", ""} {
		if _, err := q.Exec(ctx, `UPDATE vendor_vaults SET escrow_account = $2 WHERE id = $1`, vaultID, bad); err == nil {
			t.Errorf("CHECK vendor_vaults_escrow_account_chk: want error for %q", bad)
		}
	}
}
