//go:build integration

package service

import (
	"context"
	"strings"
	"testing"
)

// R1.3 & R1.4 (migration 040): escrow_cash and escrow_atm go through VendorVaultApplier,
// are not unique, and the DB CHECK refuses anything but 12 digits; vault_name is
// auto-generated when omitted on create.
func TestVendorVaultApplier_Escrow_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()
	q := dbtx(svc)

	vendorID := h.seedVendor("ITESC-"+tag, "Vendor Escrow "+tag)
	branchID := h.seedVendorBranch(vendorID, "ITESCB-"+tag, "Branch Escrow "+tag)
	other := h.seedVault(branchID, "ITESC-VO-"+tag)
	if _, err := q.Exec(ctx, `UPDATE vendor_vaults SET escrow_cash = '001234567890', escrow_atm = '998877665544' WHERE id = $1`, other); err != nil {
		t.Fatalf("seed escrow: %v", err)
	}

	fields := VendorVaultUpdatePayload{
		Category:     "ATM_CASH",
		CurrencyCode: "IDR",
		Alamat:       sp("Jl. Sudirman No. 1"),
		EscrowCash:   sp("001234567890"),
		EscrowAtm:    sp("998877665544"),
	}
	if err := approveChange(t, svc, h.insertPending("vendor_vault", "create", nil,
		VendorVaultPayload{VendorBranchID: branchID, VaultCode: "ITESC-V1-" + tag, VendorVaultUpdatePayload: fields})); err != nil {
		t.Fatalf("create with escrow values another vault already uses (not unique): %v", err)
	}
	var vaultID int64
	var vaultName string
	var alamat, escrowCash, escrowAtm *string
	if err := q.QueryRow(ctx, `SELECT id, vault_name, alamat, escrow_cash, escrow_atm FROM vendor_vaults WHERE vault_code = $1`, "ITESC-V1-"+tag).
		Scan(&vaultID, &vaultName, &alamat, &escrowCash, &escrowAtm); err != nil {
		t.Fatal(err)
	}
	h.vendorVaultIDs = append(h.vendorVaultIDs, vaultID)
	if !strings.Contains(vaultName, "_V0") {
		t.Errorf("expected auto-generated vault_name with _V<3-digit>, got %q", vaultName)
	}
	if showStr(alamat) != "Jl. Sudirman No. 1" {
		t.Errorf("alamat = %s, want Jl. Sudirman No. 1", showStr(alamat))
	}
	if showStr(escrowCash) != "001234567890" || showStr(escrowAtm) != "998877665544" {
		t.Fatalf("escrow_cash=%s escrow_atm=%s, want 001234567890 / 998877665544 (leading zeros kept)", showStr(escrowCash), showStr(escrowAtm))
	}

	fields.EscrowCash = nil
	fields.EscrowAtm = nil
	if err := approveChange(t, svc, h.insertPending("vendor_vault", "update", &vaultID, fields)); err != nil {
		t.Fatalf("clear escrow: %v", err)
	}
	if err := q.QueryRow(ctx, `SELECT escrow_cash, escrow_atm FROM vendor_vaults WHERE id = $1`, vaultID).Scan(&escrowCash, &escrowAtm); err != nil || escrowCash != nil || escrowAtm != nil {
		t.Fatalf("cleared escrow_cash=%s escrow_atm=%s (err %v), want NULL", showStr(escrowCash), showStr(escrowAtm), err)
	}

	for _, bad := range []string{"12345678901", "12345678901A", ""} {
		if _, err := q.Exec(ctx, `UPDATE vendor_vaults SET escrow_cash = $2 WHERE id = $1`, vaultID, bad); err == nil {
			t.Errorf("CHECK vendor_vaults_escrow_cash_chk: want error for %q", bad)
		}
		if _, err := q.Exec(ctx, `UPDATE vendor_vaults SET escrow_atm = $2 WHERE id = $1`, vaultID, bad); err == nil {
			t.Errorf("CHECK vendor_vaults_escrow_atm_chk: want error for %q", bad)
		}
	}
}
