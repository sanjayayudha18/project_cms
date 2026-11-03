//go:build integration

package service

import (
	"context"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

func (h *testHelpers) seedVault(branchID int64, code string) int64 {
	h.t.Helper()
	var id int64
	if err := h.pool.QueryRow(h.ctx, `INSERT INTO vendor_vaults (vendor_branch_id, vault_code, type, category, currency_code)
		VALUES ($1, $2, 'CASH', 'CASH', 'IDR') RETURNING id`, branchID, code).Scan(&id); err != nil {
		h.t.Fatalf("seed vault: %v", err)
	}
	h.vendorVaultIDs = append(h.vendorVaultIDs, id)
	return id
}

// approveChange runs one staged change through the real apply-on-approve path.
func approveChange(t *testing.T, svc *MasterDataApprovalService, changeID int64) error {
	t.Helper()
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "approved"}}
	_, err := svc.Approve(context.Background(), 999, 1, "127.0.0.1")
	return err
}

func TestDsrLocationMapApplier_Lifecycle_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()
	q := dbtx(svc)

	vendorID := h.seedVendor("ITDM-"+tag, "Vendor DSR Map "+tag)
	branchID := h.seedVendorBranch(vendorID, "ITDMB-"+tag, "Branch "+tag)
	vault1 := h.seedVault(branchID, "ITDM-V1-"+tag)
	vault2 := h.seedVault(branchID, "ITDM-V2-"+tag)
	otherVendor := h.seedVendor("ITDMO-"+tag, "Other Vendor "+tag)
	otherVault := h.seedVault(h.seedVendorBranch(otherVendor, "ITDMOB-"+tag, "Other "+tag), "ITDM-VO-"+tag)

	// create
	if err := approveChange(t, svc, h.insertPending("dsr_location_map", "create", nil,
		DsrLocationMapPayload{VendorID: vendorID, DsrLocation: "LENTENG AGUNG", VendorVaultID: vault1})); err != nil {
		t.Fatalf("create: %v", err)
	}
	var mapID, gotVault int64
	if err := q.QueryRow(ctx, `SELECT id, vendor_vault_id FROM dsr_location_vault_maps WHERE vendor_id = $1 AND is_active`, vendorID).Scan(&mapID, &gotVault); err != nil || gotVault != vault1 {
		t.Fatalf("created map: vault=%d err=%v, want %d", gotVault, err, vault1)
	}

	// same label, other case -> unique index refuses at apply; nothing inserted
	if err := approveChange(t, svc, h.insertPending("dsr_location_map", "create", nil,
		DsrLocationMapPayload{VendorID: vendorID, DsrLocation: "lenteng agung", VendorVaultID: vault2})); err == nil {
		t.Error("duplicate label (case-insensitive): want apply error")
	}

	// another vendor's vault -> guarded insert matches no row
	if err := approveChange(t, svc, h.insertPending("dsr_location_map", "create", nil,
		DsrLocationMapPayload{VendorID: vendorID, DsrLocation: "BINTARO", VendorVaultID: otherVault})); err == nil {
		t.Error("foreign vault: want apply error")
	}
	var n int
	if err := q.QueryRow(ctx, `SELECT count(*) FROM dsr_location_vault_maps WHERE vendor_id = $1`, vendorID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("maps for vendor = %d (err %v), want 1", n, err)
	}

	// update -> re-point to vault2
	if err := approveChange(t, svc, h.insertPending("dsr_location_map", "update", &mapID, DsrLocationMapUpdatePayload{VendorVaultID: vault2})); err != nil {
		t.Fatalf("update: %v", err)
	}
	// disable
	if err := approveChange(t, svc, h.insertPending("dsr_location_map", "disable", &mapID, nil)); err != nil {
		t.Fatalf("disable: %v", err)
	}
	var active bool
	if err := q.QueryRow(ctx, `SELECT vendor_vault_id, is_active FROM dsr_location_vault_maps WHERE id = $1`, mapID).Scan(&gotVault, &active); err != nil {
		t.Fatal(err)
	}
	if gotVault != vault2 || active {
		t.Errorf("after update+disable: vault=%d active=%v, want %d/false", gotVault, active, vault2)
	}
}
