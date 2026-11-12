package service

import (
	"context"
	"strings"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// escrowVaultRepo adds one vault (V1/B1/VLT1, escrow set) to fakeImportRepo.
type escrowVaultRepo struct{ fakeImportRepo }

func (escrowVaultRepo) VendorVaults(_ context.Context, after int64, _ string, limit int32) ([]db.ExportVendorVaultsBatchRow, error) {
	all := []db.ExportVendorVaultsBatchRow{{
		ID: 1, VendorCode: "V1", BranchCode: "B1", BranchName: "Cabang 1",
		VaultCode: "VLT1", VaultName: "VLT1", Region: "Jakarta", Category: "ATM",
		OperatingHours: "24h", CurrencyCode: "IDR", Alamat: "Jl. Sudirman No. 1",
		MinBalance: "1000000.00", MaxBalance: "5000000000.00", CisLimitAmount: "2500000000.00",
		LocationID: "5", EscrowCash: "001234567890", EscrowAtm: "001234567891", IsActive: true,
	}}
	return page(all, after, limit, func(r db.ExportVendorVaultsBatchRow) int64 { return r.ID }), nil
}

func dryRunEntity(t *testing.T, repo MasterDataImportRepo, entity, csvText string) *ImportResult {
	t.Helper()
	res, err := NewMasterDataImporter(repo).DryRun(adminCtx(7), entity, strings.NewReader(csvText))
	if err != nil {
		t.Fatalf("DryRun(%s): %v", entity, err)
	}
	return res
}

// vendor-vaults-export-revamp Req 5.3: 9-column vendor-branches header without cis_limit_amount,
// while legacy 10-column header with cis_limit_amount is still accepted with cis_limit_amount ignored.
func TestImport_VendorBranchCisLimit(t *testing.T) {
	newHeader := "id,vendor_code,branch_code,branch_name,region,region_code,category,location_id,is_active\n"
	if res := dryRun(t, ExportVendorBranches, newHeader+"1,V1,B1,Cabang 1,Jakarta,JKT,ATM,5,true\n"); len(res.Preview.Errors) != 0 || res.Preview.Unchanged != 1 {
		t.Errorf("9-column vendor-branches header must round-trip with 0 changes, got %+v", res.Preview)
	}

	legacy10 := "id,vendor_code,branch_code,branch_name,region,region_code,category,location_id,cis_limit_amount,is_active\n"
	if res := dryRun(t, ExportVendorBranches, legacy10+"1,V1,B1,Cabang 1,Jakarta,JKT,ATM,5,2500000000.00,true\n"); len(res.Preview.Errors) != 0 || res.Preview.Unchanged != 1 {
		t.Errorf("legacy 10-col export round trip must ignore cis_limit_amount (no-op), got %+v", res.Preview)
	}
	if res := dryRun(t, ExportVendorBranches, legacy10+"1,V1,B1,Cabang 1,Jakarta,JKT,ATM,5,9999999999.00,true\n"); len(res.Preview.Errors) != 0 || res.Preview.Unchanged != 1 {
		t.Errorf("changed cis_limit_amount in legacy 10-col branch header must be ignored, got %+v", res.Preview)
	}
}

// vendor-vaults-export-revamp Req 6.1-6.4: revamped 14-column vendor-vaults CSV import
// (no and region ignored, matched by branch_name + vault_name, validates min_balance,
// max_balance, cis_limit_amount, escrow_cash, escrow_atm, alamat) and 7-column vendors import.
func TestImport_VendorVaultRevamped14Columns(t *testing.T) {
	repo := escrowVaultRepo{}
	h := "no,branch_name,vault_name,category,operating_hours,currency_code,region,alamat,min_balance,max_balance,cis_limit_amount,escrow_cash,escrow_atm,is_active\n"

	// Round-trip of exported row (even with modified 'no' and 'region') is unchanged.
	if res := dryRunEntity(t, repo, ExportVendorVaults, h+"99,Cabang 1,VLT1,ATM,24h,IDR,Region Diabaikan,Jl. Sudirman No. 1,1000000.00,5000000000.00,2500000000.00,001234567890,001234567891,true\n"); len(res.Preview.Errors) != 0 || res.Preview.Unchanged != 1 {
		t.Errorf("14-column round trip with changed no/region must be unchanged, got %+v", res.Preview)
	}

	// Existing vault matched by branch_name + vault_name -> update.
	res := dryRunEntity(t, repo, ExportVendorVaults, h+"1,Cabang 1,VLT1,ATM_CASH,24h,IDR,Jakarta,Jl. Baru No. 2,2000000.00,6000000000.00, 3000000000 ,001234567890,001234567899,true\n")
	if len(res.Preview.Errors) != 0 || res.Preview.Updates != 1 || res.Rows[0].ID != 1 || res.Rows[0].Cells["cis_limit_amount"] != "3000000000" {
		t.Errorf("existing vault should resolve to update on ID 1, got %+v", res.Preview)
	}

	// New vault on existing branch -> create.
	res = dryRunEntity(t, repo, ExportVendorVaults, h+"2,Cabang 1,VLT_BARU,CASH,08:00-17:00,IDR,Jakarta,Jl. Kas 3,500000.00,10000000.00,1500000000.00,111222333444,,true\n")
	if len(res.Preview.Errors) != 0 || res.Preview.Creates != 1 || res.Rows[0].ID != 0 {
		t.Errorf("new vault should resolve to create, got %+v", res.Preview)
	}

	// Validation errors on unknown branch, min > max balance, invalid cis_limit_amount, invalid escrow_cash/escrow_atm.
	badCSV := h +
		"1,Cabang Tidak Ada,VLT_X,ATM,24h,IDR,Jakarta,,100,200,,,,true\n" +
		"2,Cabang 1,VLT_A,ATM,24h,IDR,Jakarta,,500,100,,,,true\n" +
		"3,Cabang 1,VLT_B,ATM,24h,IDR,Jakarta,,,,0,,,true\n" +
		"4,Cabang 1,VLT_C,ATM,24h,IDR,Jakarta,,,,,12345,,true\n" +
		"5,Cabang 1,VLT_D,ATM,24h,IDR,Jakarta,,,,,,12345678901A,true\n"
	badRes := dryRunEntity(t, repo, ExportVendorVaults, badCSV)
	for _, want := range []struct {
		row   int
		field string
	}{
		{2, "branch_name"},
		{3, "min_balance"},
		{4, "cis_limit_amount"},
		{5, "escrow_cash"},
		{6, "escrow_atm"},
	} {
		if !hasErr(badRes, want.row, want.field) {
			t.Errorf("missing error row %d field %s in %+v", want.row, want.field, badRes.Preview.Errors)
		}
	}

	// 7-column vendors import: match by name for update, new name for create.
	vh := "name,legal_name,npwp,contact_email,contact_phone,hq_address,is_active\n"
	vRes := dryRun(t, ExportVendors, vh+
		"Vendor Satu,PT Satu Baru,012345678901234,a@v1.id,+62 812-345,'= Jl. Mawar,true\n"+
		"Vendor Baru,PT Baru,,b@v2.id,,,true\n")
	if len(vRes.Preview.Errors) != 0 || vRes.Preview.Updates != 1 || vRes.Preview.Creates != 1 {
		t.Errorf("7-col vendors import should have 1 update + 1 create, got %+v", vRes.Preview)
	}
}

// vendor-cis-limit FR2.4: legacy 14/13-col escrow_account column; a file without it keeps the current escrow.
func TestImport_VendorVaultEscrow(t *testing.T) {
	repo := escrowVaultRepo{}
	old := "id,vendor_code,branch_code,vault_code,category,currency_code,min_capacity_amount,max_capacity_amount,latitude,longitude,operating_hours,location_id,is_active\n"
	if res := dryRunEntity(t, repo, ExportVendorVaults, old+"1,V1,B1,VLT1,ATM,IDR,1000000.00,5000000000.00,,,24h,5,true\n"); len(res.Preview.Errors) != 0 || res.Preview.Unchanged != 1 {
		t.Errorf("file without escrow_account must keep the escrow (no-op), got %+v", res.Preview)
	}

	h := "id,vendor_code,branch_code,vault_code,category,currency_code,min_capacity_amount,max_capacity_amount,latitude,longitude,operating_hours,location_id,escrow_account,is_active\n"
	if res := dryRunEntity(t, repo, ExportVendorVaults, h+"1,V1,B1,VLT1,ATM,IDR,1000000.00,5000000000.00,,,24h,5,001234567890,true\n"); len(res.Preview.Errors) != 0 || res.Preview.Unchanged != 1 {
		t.Errorf("export round trip (leading zeros) must be a no-op, got %+v", res.Preview)
	}
	if res := dryRunEntity(t, repo, ExportVendorVaults, h+"1,V1,B1,VLT1,ATM,IDR,1000000.00,5000000000.00,,,24h,5,,true\n"); len(res.Preview.Errors) != 0 || res.Preview.Updates != 1 {
		t.Errorf("blank escrow in the new header clears it (update), got %+v", res.Preview)
	}
	for _, bad := range []string{"12345678901", "12345678901A"} {
		if res := dryRunEntity(t, repo, ExportVendorVaults, h+"1,V1,B1,VLT1,ATM,IDR,1000000.00,5000000000.00,,,24h,5,"+bad+",true\n"); !hasErr(res, 2, "escrow_account") {
			t.Errorf("escrow %q must be a row error, got %+v", bad, res.Preview.Errors)
		}
	}
}
