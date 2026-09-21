package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// fakeImportRepo is a tiny in-memory master-data snapshot: vendor V1 with branch
// B1 and package P1, ATMs T1/T2 (location 5), and one active assignment of T1.
type fakeImportRepo struct{}

func page[R any](all []R, after int64, limit int32, id func(R) int64) []R {
	var out []R
	for _, r := range all {
		if id(r) > after && int32(len(out)) < limit {
			out = append(out, r)
		}
	}
	return out
}

func (fakeImportRepo) Vendors(_ context.Context, after int64, _ string, limit int32) ([]db.ExportVendorsBatchRow, error) {
	all := []db.ExportVendorsBatchRow{{ID: 1, Code: "V1", Name: "Vendor Satu", LegalName: "PT Satu", Npwp: "012345678901234", ContactEmail: "a@v1.id", ContactPhone: "+62 812-345", HqAddress: "= Jl. Mawar", IsActive: true}}
	return page(all, after, limit, func(r db.ExportVendorsBatchRow) int64 { return r.ID }), nil
}

func (fakeImportRepo) VendorBranches(_ context.Context, after int64, _ string, limit int32) ([]db.ExportVendorBranchesBatchRow, error) {
	all := []db.ExportVendorBranchesBatchRow{{ID: 1, VendorCode: "V1", BranchCode: "B1", BranchName: "Cabang 1", Region: "Jakarta", LocationID: "5", IsActive: true}}
	return page(all, after, limit, func(r db.ExportVendorBranchesBatchRow) int64 { return r.ID }), nil
}

func (fakeImportRepo) VendorVaults(context.Context, int64, string, int32) ([]db.ExportVendorVaultsBatchRow, error) {
	return nil, nil
}

func (fakeImportRepo) VendorPICs(context.Context, int64, string, int32) ([]db.ExportVendorPicsBatchRow, error) {
	return nil, nil
}

func (fakeImportRepo) ATMs(_ context.Context, after int64, _ string, limit int32) ([]db.ExportATMsBatchRow, error) {
	base := db.ExportATMsBatchRow{LocationID: 5, MachineType: "CRM", Brand: "NCR", Model: "SelfServ", OperationHours: "24H", DeploymentType: "Onsite",
		CapacityAmount: "500000000.00", PriorityClass: "VIP", IsActive: true}
	a, b := base, base
	a.ID, a.TerminalID = 1, "T1"
	b.ID, b.TerminalID, b.PriorityClass = 2, "T2", ""
	return page([]db.ExportATMsBatchRow{a, b}, after, limit, func(r db.ExportATMsBatchRow) int64 { return r.ID }), nil
}

func (fakeImportRepo) ATMAssignments(_ context.Context, after int64, _ string, limit int32) ([]db.ExportATMAssignmentsBatchRow, error) {
	all := []db.ExportATMAssignmentsBatchRow{{ID: 1, TerminalID: "T1", VendorCode: "V1", BranchCode: "B1", PackageCode: "P1",
		EffectiveStartDate: "2026-01-01", EffectiveEndDate: "2026-06-30", IsActive: true}}
	return page(all, after, limit, func(r db.ExportATMAssignmentsBatchRow) int64 { return r.ID }), nil
}

func (fakeImportRepo) PackageKeys(context.Context) ([]db.ImportPackageKeysRow, error) {
	return []db.ImportPackageKeysRow{{ID: 11, VendorCode: "V1", BranchCode: "B1", PackageCode: "P1"}}, nil
}

func (fakeImportRepo) LocationIDs(context.Context) ([]int64, error) { return []int64{5, 6}, nil }

func dryRun(t *testing.T, entity, csvText string) *ImportResult {
	t.Helper()
	res, err := NewMasterDataImporter(fakeImportRepo{}).DryRun(adminCtx(7), entity, strings.NewReader(csvText))
	if err != nil {
		t.Fatalf("DryRun(%s): %v", entity, err)
	}
	return res
}

func hasErr(res *ImportResult, row int, field string) bool {
	for _, e := range res.Preview.Errors {
		if e.Row == row && e.Field == field {
			return true
		}
	}
	return false
}

const vendorHeader = "id,code,name,legal_name,npwp,contact_email,contact_phone,hq_address,is_active\n"

// Round trip (plan Validate Fase 5): what the exporter writes, the importer
// accepts as "no change" -- through BOM, csvSafe apostrophes and quoting.
func TestImport_RoundTripOfExportIsAllUnchanged(t *testing.T) {
	for _, entity := range []string{ExportVendors, ExportVendorBranches, ExportATMs, ExportATMAssignments} {
		var buf bytes.Buffer
		if _, err := newTestExporter(fakeImportRepo{}, 1).Write(adminCtx(7), entity, "all", &buf, nil); err != nil {
			t.Fatalf("export %s: %v", entity, err)
		}

		res := dryRun(t, entity, buf.String())

		if p := res.Preview; len(p.Errors) != 0 || p.Creates != 0 || p.Updates != 0 || p.Unchanged != p.TotalRows || p.TotalRows == 0 {
			t.Errorf("%s: round trip should be all unchanged, got %+v", entity, p)
		}
	}
}

func TestImport_ParserAcceptsBOMSemicolonAndEmptyRows(t *testing.T) {
	csvText := utf8BOM + strings.ReplaceAll(vendorHeader, ",", ";") +
		";;;;;;;;\n" + // fully empty row: skipped, not an error
		";V2;Vendor Dua;;;;;;\n"

	res := dryRun(t, ExportVendors, csvText)

	if p := res.Preview; len(p.Errors) != 0 || p.TotalRows != 1 || p.Creates != 1 {
		t.Fatalf("want 1 create and no errors, got %+v", p)
	}
	if res.Rows[0].Row != 3 {
		t.Errorf("row number = %d, want the file line (3)", res.Rows[0].Row)
	}
}

func TestImport_FileLevelErrors(t *testing.T) {
	cases := map[string]string{
		"wrong header":       "id,code,nama\n",
		"header wrong order": "code,id,name,legal_name,npwp,contact_email,contact_phone,hq_address,is_active\n",
		"empty file":         "",
		"unbalanced quote":   vendorHeader + ",V2,\"Vendor,,,,,,\n",
	}
	for name, csvText := range cases {
		t.Run(name, func(t *testing.T) {
			res := dryRun(t, ExportVendors, csvText)
			if len(res.Preview.Errors) != 1 || res.Preview.ValidRows != 0 || res.Preview.TotalRows != 0 {
				t.Errorf("want one file-level error and no rows processed, got %+v", res.Preview)
			}
		})
	}
}

func TestImport_SizeAndRowLimits(t *testing.T) {
	imp := NewMasterDataImporter(fakeImportRepo{})

	_, err := imp.DryRun(adminCtx(7), ExportVendors, strings.NewReader(vendorHeader+strings.Repeat("x", MasterDataImportMaxBytes)))
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "file" {
		t.Errorf("oversize: want ValidationError(file), got %v", err)
	}

	var sb strings.Builder
	sb.WriteString(vendorHeader)
	for i := 0; i <= MasterDataImportMaxRows; i++ {
		fmt.Fprintf(&sb, ",V%d,N,,,,,,\n", i+10)
	}
	if _, err := imp.DryRun(adminCtx(7), ExportVendors, strings.NewReader(sb.String())); !errors.As(err, &ve) {
		t.Errorf("too many rows: want ValidationError, got %v", err)
	}
}

func TestImport_VendorRowRules(t *testing.T) {
	csvText := vendorHeader +
		",V2,Vendor Dua,,01.234.567.8-901.000,x@y.id,,,\n" + // row 2: valid; NPWP punctuation normalized
		",,,,,,,,\n" + // row 3: empty, skipped
		",V3,,,,not-an-email,,,\n" + // row 4: name missing + bad email
		",V2,Duplikat,,,,,,\n" + // row 5: duplicate of row 2 in the file
		",V1,Sudah Ada,,,,,,\n" + // row 6: exists already, no id
		"99,V9,Tak Ada,,,,,,true\n" + // row 7: unknown id
		"1,V1X,Vendor Satu,PT Satu,012345678901234,a@v1.id,+62 812-345,= Jl. Mawar,true\n" + // row 8: code is immutable
		",V4,NPWP Salah,,123,,,,\n" // row 9: bad NPWP

	res := dryRun(t, ExportVendors, csvText)

	for _, want := range []struct {
		row   int
		field string
	}{{4, "name"}, {4, "contact_email"}, {5, "code"}, {6, "code"}, {7, "id"}, {8, "code"}, {9, "npwp"}} {
		if !hasErr(res, want.row, want.field) {
			t.Errorf("missing error row %d field %s in %+v", want.row, want.field, res.Preview.Errors)
		}
	}
	if p := res.Preview; p.ValidRows != 1 || p.Creates != 1 {
		t.Errorf("want only the row-2 create valid, got %+v", p)
	}
	if got := res.Rows[0].Cells["npwp"]; got != "012345678901000" {
		t.Errorf("npwp stored as %q, want digits only", got)
	}
}

func TestImport_VendorUpdateResolvesToUpdateOrUnchanged(t *testing.T) {
	res := dryRun(t, ExportVendors, vendorHeader+"1,V1,Vendor Satu Baru,PT Satu,012345678901234,a@v1.id,+62 812-345,'= Jl. Mawar,true\n")

	if p := res.Preview; len(p.Errors) != 0 || p.Updates != 1 || p.Unchanged != 0 {
		t.Fatalf("a changed name must be an update, got %+v", p)
	}
	// The export's apostrophe guard was undone: the address compares equal to the stored one.
	if got := res.Rows[0].Cells["hq_address"]; got != "= Jl. Mawar" {
		t.Errorf("hq_address = %q, want the apostrophe stripped", got)
	}
}

func TestImport_ATMRules(t *testing.T) {
	h := "id,terminal_id,location_id,machine_type,brand,model,operation_hours,deployment_type,capacity_amount,low_threshold_amount,critical_threshold_amount,blacklisted,escrow_account,priority_class,is_active\n"
	csvText := h +
		",T9,5,CRM,NCR,S,24H,Onsite,1000.50,,,,,VIP,\n" + // row 2: valid create
		",T10,77,CRM,NCR,S,24H,Onsite,,,,,,,\n" + // row 3: unknown location
		",T11,5,CRM,NCR,S,24H,Onsite,-5,,,,,,\n" + // row 4: negative money
		",T12,5,CRM,NCR,S,24H,Onsite,abc,,,,,Gold,\n" + // row 5: bad money
		",T13,5,CRM,NCR,S,24H,Onsite,1e3,,,,,,\n" // row 6: exponent notation is not a decimal

	res := dryRun(t, ExportATMs, csvText)

	for _, want := range []struct {
		row   int
		field string
	}{{3, "location_id"}, {4, "capacity_amount"}, {5, "priority_class"}, {6, "capacity_amount"}} {
		if !hasErr(res, want.row, want.field) {
			t.Errorf("missing error row %d field %s in %+v", want.row, want.field, res.Preview.Errors)
		}
	}
	if res.Preview.Creates != 1 {
		t.Errorf("want exactly the row-2 create valid, got %+v", res.Preview)
	}
}

func TestImport_AssignmentOverlapAndReferences(t *testing.T) {
	h := "id,terminal_id,vendor_code,branch_code,package_code,effective_start_date,effective_end_date,is_active\n"
	csvText := h +
		",T1,V1,B1,P1,2026-07-01,2026-12-31,\n" + // row 2: adjacent to the existing 01-01..06-30 period: fine
		",T1,V1,B1,P1,2026-12-31,,\n" + // row 3: overlaps row 2 (both inclusive) within the file
		",T1,V1,B1,P1,2026-03-01,2026-03-31,\n" + // row 4: overlaps the database row
		",T2,V1,B1,P1,2026-01-01,,\n" + // row 5: other ATM, open-ended: fine
		",T2,V1,B1,P1,2025-01-01,2025-12-31,\n" + // row 6: fine, before the open-ended period
		",T2,V1,B1,P1,2026-02-01,2026-01-01,\n" + // row 7: end before start
		",T3,V1,B1,P1,2026-01-01,,\n" + // row 8: unknown ATM
		",T2,V1,B1,PX,2026-01-01,,\n" + // row 9: unknown package
		",T2,V1,B1,P1,01/02/2026,,\n" // row 10: not YYYY-MM-DD

	res := dryRun(t, ExportATMAssignments, csvText)

	for _, want := range []struct {
		row   int
		field string
	}{{3, "effective_start_date"}, {4, "effective_start_date"}, {7, "effective_end_date"}, {8, "terminal_id"}, {9, "package_code"}, {10, "effective_start_date"}} {
		if !hasErr(res, want.row, want.field) {
			t.Errorf("missing error row %d field %s in %+v", want.row, want.field, res.Preview.Errors)
		}
	}
	if hasErr(res, 2, "effective_start_date") || hasErr(res, 5, "effective_start_date") || hasErr(res, 6, "effective_start_date") {
		t.Errorf("adjacent/non-overlapping periods must be accepted: %+v", res.Preview.Errors)
	}
	if res.Preview.Creates != 3 {
		t.Errorf("want rows 2, 5, 6 valid, got %+v", res.Preview)
	}
}

func TestImport_MovingAnAssignmentFreesItsOldPeriod(t *testing.T) {
	h := "id,terminal_id,vendor_code,branch_code,package_code,effective_start_date,effective_end_date,is_active\n"
	// Row 2 edits db assignment 1 away from Jan-Jun; row 3 then takes Feb-May for a new period.
	res := dryRun(t, ExportATMAssignments, h+
		"1,T1,V1,B1,P1,2027-01-01,2027-06-30,true\n"+
		",T1,V1,B1,P1,2026-02-01,2026-05-31,\n")

	if len(res.Preview.Errors) != 0 || res.Preview.Updates != 1 || res.Preview.Creates != 1 {
		t.Errorf("want update + create with no overlap error, got %+v", res.Preview)
	}
}

func TestImport_RequiresMasterDataAdminRole(t *testing.T) {
	_, err := NewMasterDataImporter(fakeImportRepo{}).DryRun(context.Background(), ExportVendors, strings.NewReader(vendorHeader))
	if !errors.Is(err, ErrMasterDataForbidden) {
		t.Fatalf("want ErrMasterDataForbidden without auth context, got %v", err)
	}
}

func TestImport_UnknownEntity(t *testing.T) {
	_, err := NewMasterDataImporter(fakeImportRepo{}).DryRun(adminCtx(7), "users", strings.NewReader(""))
	if !errors.Is(err, ErrExportUnknownEntity) {
		t.Fatalf("want ErrExportUnknownEntity, got %v", err)
	}
}

func TestImport_ToggleAndDataChangeCannotShareARow(t *testing.T) {
	// Vendor 1 is active. Disabling it AND renaming it in one row would need two change requests for the same entity.
	res := dryRun(t, ExportVendors, vendorHeader+"1,V1,Renamed,PT Satu,012345678901234,a@v1.id,+62 812-345,'= Jl. Mawar,false\n")
	if !hasErr(res, 2, "is_active") {
		t.Errorf("want an is_active error for toggle + data change, got %+v", res.Preview.Errors)
	}

	// Disable alone is a valid toggle (an update op flagged Toggle).
	res = dryRun(t, ExportVendors, vendorHeader+"1,V1,Vendor Satu,PT Satu,012345678901234,a@v1.id,+62 812-345,'= Jl. Mawar,false\n")
	if len(res.Preview.Errors) != 0 || res.Preview.Updates != 1 || !res.Rows[0].Toggle {
		t.Errorf("disable-only should be a valid toggle, got %+v", res.Preview)
	}
}

// T5.5: the limits are inclusive at the boundary and named constants.
func TestImport_LimitBoundaries(t *testing.T) {
	imp := NewMasterDataImporter(fakeImportRepo{})

	// Exactly MasterDataImportMaxBytes is accepted (one over is rejected: TestImport_SizeAndRowLimits).
	row := ",V9,"
	tail := ",,,,,,\n"
	pad := MasterDataImportMaxBytes - len(vendorHeader) - len(row) - len(tail)
	if _, err := imp.DryRun(adminCtx(7), ExportVendors, strings.NewReader(vendorHeader+row+strings.Repeat("x", pad)+tail)); err != nil {
		t.Errorf("a file of exactly %d bytes must be accepted: %v", MasterDataImportMaxBytes, err)
	}

	// Exactly MasterDataImportMaxRows data rows is accepted.
	var sb strings.Builder
	sb.WriteString(vendorHeader)
	for i := 0; i < MasterDataImportMaxRows; i++ {
		fmt.Fprintf(&sb, ",V%d,N,,,,,,\n", i+10)
	}
	res, err := imp.DryRun(adminCtx(7), ExportVendors, strings.NewReader(sb.String()))
	if err != nil || res.Preview.TotalRows != MasterDataImportMaxRows {
		t.Errorf("exactly %d rows must be accepted, got total=%v err=%v", MasterDataImportMaxRows, res, err)
	}
}

func TestImport_ErrorListIsCappedButCounted(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(vendorHeader)
	for i := 0; i < masterDataImportMaxErrs+50; i++ {
		fmt.Fprintf(&sb, ",V%d,,,,,,,\n", i+10) // name missing on every row
	}

	res := dryRun(t, ExportVendors, sb.String())

	if p := res.Preview; len(p.Errors) != masterDataImportMaxErrs || !p.ErrorsTruncated || p.ValidRows != 0 {
		t.Errorf("want %d errors listed + truncated flag, got %d truncated=%v valid=%d", masterDataImportMaxErrs, len(p.Errors), p.ErrorsTruncated, p.ValidRows)
	}
}
