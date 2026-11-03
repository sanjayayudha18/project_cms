//go:build integration

package service

import (
	"context"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/repository"
)

// Read paths of the master-data screens through the real repositories: the
// pending-change list/detail (maker-checker inbox), the branch list/detail and
// the DSR location map list (incl. labels the vendor sent but nobody mapped).
func TestIntegration_AdminReads(t *testing.T) {
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
	vendorID := seed(`INSERT INTO vendors (code, name) VALUES ('ITRD', 'Vendor ITRD') RETURNING id`)
	branchID := seed(`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name) VALUES ($1, 'ITRD-B', 'Cabang ITRD') RETURNING id`, vendorID)
	vaultID := seed(`INSERT INTO vendor_vaults (vendor_branch_id, vault_code, type, category, currency_code) VALUES ($1, 'ITRD-V', 'CASH', 'CASH', 'IDR') RETURNING id`, branchID)
	seed(`INSERT INTO dsr_location_vault_maps (vendor_id, dsr_location, vendor_vault_id) VALUES ($1, 'MAPPED', $2) RETURNING id`, vendorID, vaultID)
	upload := seed(`INSERT INTO dsr_uploads (filename, vendor, report_date, daily_status) VALUES ('itrd.xlsx', 'Vendor ITRD', CURRENT_DATE, 'completed') RETURNING id`)
	seed(`INSERT INTO dsr_daily_rows (upload_id, row_no, location, section, flow, line_label, line_total_idr)
		VALUES ($1, 1, 'NOT MAPPED', 'd0', 'saldo_akhir', 'SALDO AKHIR', 1) RETURNING row_no`, upload)
	var makerID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM users ORDER BY id LIMIT 1`).Scan(&makerID); err != nil {
		t.Fatal(err)
	}
	changeID := seed(`INSERT INTO master_data_change_requests (entity_type, entity_id, op, payload, maker_id)
		VALUES ('itest_read', $1, 'update', '{}', $2) RETURNING id`, branchID, makerID)

	t.Run("change requests", func(t *testing.T) {
		svc := NewMasterDataChangeService(repository.NewMasterDataChangeRepository(tx), nil, nil)
		if c, err := svc.Get(ctx, changeID); err != nil || c.Status != "pending" || c.MakerID != makerID {
			t.Errorf("Get = %+v, %v", c, err)
		}
		et := "itest_read"
		if rows, err := svc.List(ctx, db.ListMasterDataChangeRequestsParams{EntityType: &et, PageLimit: 10}); err != nil || len(rows) != 1 {
			t.Errorf("List = %v, %v", rows, err)
		}
		if n, err := svc.Count(ctx, db.CountMasterDataChangeRequestsParams{EntityType: &et}); err != nil || n != 1 {
			t.Errorf("Count = %d, %v", n, err)
		}
	})

	t.Run("vendor branches", func(t *testing.T) {
		svc := NewVendorBranchAdminService(repository.NewVendorBranchAdminRepository(tx, tx), nil, nil)
		if rows, err := svc.List(ctx, db.ListVendorBranchesAdminParams{VendorID: vendorID, Status: "all", PageLimit: 10}); err != nil || len(rows) != 1 || rows[0].ID != branchID {
			t.Errorf("List = %+v, %v", rows, err)
		}
		if n, err := svc.Count(ctx, db.CountVendorBranchesAdminParams{VendorID: vendorID, Status: "all"}); err != nil || n != 1 {
			t.Errorf("Count = %d, %v", n, err)
		}
		if b, err := svc.Get(ctx, branchID); err != nil || b == nil || b.BranchName != "Cabang ITRD" {
			t.Errorf("Get = %+v, %v", b, err)
		}
		if b, err := svc.Get(ctx, -1); err != nil || b != nil {
			t.Errorf("Get(missing) = %+v, %v, want nil, nil", b, err)
		}
	})

	t.Run("dsr location maps", func(t *testing.T) {
		svc := NewDsrLocationMapAdminService(repository.NewDsrLocationMapAdminRepository(tx, tx), nil)
		if rows, err := svc.List(ctx, vendorID, "active"); err != nil || len(rows) != 1 || rows[0].DsrLocation != "MAPPED" {
			t.Errorf("List = %+v, %v", rows, err)
		}
		rows, err := svc.ListUnmapped(ctx, vendorID)
		if err != nil || len(rows) != 1 || rows[0].DsrLocation != "NOT MAPPED" {
			t.Errorf("ListUnmapped = %+v, %v, want only the unmapped label", rows, err)
		}
	})
}
