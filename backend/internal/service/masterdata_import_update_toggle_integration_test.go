//go:build integration

package service

import (
	"strconv"
	"testing"
)

// be-tech-debt C4.4: a CSV import of existing vendors stages an update and a
// disable in one batch; nothing changes before the batch's single approval,
// and both land (with their audit rows) when it is approved.
func TestMasterDataImportConfirm_UpdateAndToggleRows_EffectOnlyAfterApproval_Integration(t *testing.T) {
	f := newImportFixture(t)
	codeA, codeB := "IT-"+f.tag+"UA", "IT-"+f.tag+"UB"
	idA := f.h.seedVendor(codeA, "Vendor UA")
	idB := f.h.seedVendor(codeB, "Vendor UB")

	csv := vendorHeader +
		strconv.FormatInt(idA, 10) + "," + codeA + ",Renamed UA,,,,,,true\n" + // data change -> update
		strconv.FormatInt(idB, 10) + "," + codeB + ",Vendor UB,,,,,,false\n" // only is_active -> disable
	res := f.confirm(csv)
	if res.ApprovalRequestID == nil {
		t.Fatalf("confirm = %+v, want one approval for the batch", res)
	}
	if n := f.count(`SELECT count(*) FROM master_data_change_requests WHERE batch_id = $1`, res.BatchID); n != 2 {
		t.Fatalf("staged rows = %d, want 2", n)
	}
	if n := f.count(`SELECT count(*) FROM master_data_change_requests WHERE batch_id = $1 AND op = 'update' AND entity_id = $2`, res.BatchID, idA); n != 1 {
		t.Errorf("vendor UA staged as update rows = %d, want 1", n)
	}
	if n := f.count(`SELECT count(*) FROM master_data_change_requests WHERE batch_id = $1 AND op = 'disable' AND entity_id = $2`, res.BatchID, idB); n != 1 {
		t.Errorf("vendor UB staged as disable rows = %d, want 1", n)
	}

	// Maker-checker: nothing applied before approval.
	if n := f.count(`SELECT count(*) FROM vendors WHERE id = $1 AND name = 'Vendor UA'`, idA); n != 1 {
		t.Fatal("vendor UA changed before approval")
	}
	if n := f.count(`SELECT count(*) FROM vendors WHERE id = $1 AND is_active AND deleted_at IS NULL`, idB); n != 1 {
		t.Fatal("vendor UB disabled before approval")
	}

	if err := f.approve(res); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if n := f.count(`SELECT count(*) FROM vendors WHERE id = $1 AND name = 'Renamed UA' AND is_active`, idA); n != 1 {
		t.Error("vendor UA not renamed after approval")
	}
	// Soft-disable only (never a hard delete).
	if n := f.count(`SELECT count(*) FROM vendors WHERE id = $1 AND NOT is_active AND deleted_at IS NOT NULL`, idB); n != 1 {
		t.Error("vendor UB not soft-disabled after approval")
	}
	if n := f.count(`SELECT count(*) FROM master_data_change_requests WHERE batch_id = $1 AND status = 'applied'`, res.BatchID); n != 2 {
		t.Errorf("applied rows = %d, want 2", n)
	}
	if n := f.count(`SELECT count(*) FROM audit_logs WHERE action = 'master_data_applied' AND entity_type = 'vendor' AND entity_id = ANY($1)`, []int64{idA, idB}); n != 2 {
		t.Errorf("master_data_applied audit rows = %d, want 2", n)
	}
}
