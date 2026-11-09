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
	all := []db.ExportVendorVaultsBatchRow{{ID: 1, VendorCode: "V1", BranchCode: "B1", VaultCode: "VLT1", Category: "ATM",
		CurrencyCode: "IDR", LocationID: "5", EscrowAccount: "001234567890", IsActive: true}}
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

// vendor-cis-limit FR1.5: cis_limit_amount column; a file without it keeps the current limit.
// fakeImportRepo branch 1 has limit 2500000000.00.
func TestImport_VendorBranchCisLimit(t *testing.T) {
	old := "id,vendor_code,branch_code,branch_name,region,region_code,category,location_id,is_active\n"
	if res := dryRun(t, ExportVendorBranches, old+"1,V1,B1,Cabang 1,Jakarta,JKT,ATM,5,true\n"); len(res.Preview.Errors) != 0 || res.Preview.Unchanged != 1 {
		t.Errorf("file without cis_limit_amount must keep the limit (no-op), got %+v", res.Preview)
	}

	h := "id,vendor_code,branch_code,branch_name,region,region_code,category,location_id,cis_limit_amount,is_active\n"
	if res := dryRun(t, ExportVendorBranches, h+"1,V1,B1,Cabang 1,Jakarta,JKT,ATM,5,2500000000.00,true\n"); len(res.Preview.Errors) != 0 || res.Preview.Unchanged != 1 {
		t.Errorf("export round trip must be a no-op, got %+v", res.Preview)
	}
	res := dryRun(t, ExportVendorBranches, h+"1,V1,B1,Cabang 1,Jakarta,JKT,ATM,5, 3000000000 ,true\n")
	if len(res.Preview.Errors) != 0 || res.Preview.Updates != 1 || res.Rows[0].Cells["cis_limit_amount"] != "3000000000" {
		t.Errorf("new limit should be a trimmed update, got %+v", res.Preview)
	}
	res = dryRun(t, ExportVendorBranches, h+"1,V1,B1,Cabang 1,Jakarta,JKT,ATM,5,,true\n")
	if len(res.Preview.Errors) != 0 || res.Preview.Updates != 1 {
		t.Errorf("blank limit in the new header clears it (update), got %+v", res.Preview)
	}
	for _, bad := range []string{"0", "1.234", "1e5", "abc"} {
		if res := dryRun(t, ExportVendorBranches, h+"1,V1,B1,Cabang 1,Jakarta,JKT,ATM,5,"+bad+",true\n"); !hasErr(res, 2, "cis_limit_amount") {
			t.Errorf("limit %q must be a row error, got %+v", bad, res.Preview.Errors)
		}
	}
}

// vendor-cis-limit FR2.4: escrow_account column; a file without it keeps the current escrow.
func TestImport_VendorVaultEscrow(t *testing.T) {
	repo := escrowVaultRepo{}
	old := "id,vendor_code,branch_code,vault_code,category,currency_code,min_capacity_amount,max_capacity_amount,latitude,longitude,operating_hours,location_id,is_active\n"
	if res := dryRunEntity(t, repo, ExportVendorVaults, old+"1,V1,B1,VLT1,ATM,IDR,,,,,,5,true\n"); len(res.Preview.Errors) != 0 || res.Preview.Unchanged != 1 {
		t.Errorf("file without escrow_account must keep the escrow (no-op), got %+v", res.Preview)
	}

	h := "id,vendor_code,branch_code,vault_code,category,currency_code,min_capacity_amount,max_capacity_amount,latitude,longitude,operating_hours,location_id,escrow_account,is_active\n"
	if res := dryRunEntity(t, repo, ExportVendorVaults, h+"1,V1,B1,VLT1,ATM,IDR,,,,,,5,001234567890,true\n"); len(res.Preview.Errors) != 0 || res.Preview.Unchanged != 1 {
		t.Errorf("export round trip (leading zeros) must be a no-op, got %+v", res.Preview)
	}
	if res := dryRunEntity(t, repo, ExportVendorVaults, h+"1,V1,B1,VLT1,ATM,IDR,,,,,,5,,true\n"); len(res.Preview.Errors) != 0 || res.Preview.Updates != 1 {
		t.Errorf("blank escrow in the new header clears it (update), got %+v", res.Preview)
	}
	for _, bad := range []string{"12345678901", "12345678901A"} {
		if res := dryRunEntity(t, repo, ExportVendorVaults, h+"1,V1,B1,VLT1,ATM,IDR,,,,,,5,"+bad+",true\n"); !hasErr(res, 2, "escrow_account") {
			t.Errorf("escrow %q must be a row error, got %+v", bad, res.Preview.Errors)
		}
	}
}
