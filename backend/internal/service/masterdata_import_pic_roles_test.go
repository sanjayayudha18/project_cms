package service

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

const (
	picHeaderRoles  = "id,vendor_code,branch_code,name,position,phone,email,is_notification_recipient,is_active,roles\n"
	picHeaderLegacy = "id,vendor_code,branch_code,name,position,phone,email,is_notification_recipient,is_active\n"
)

// Req 6.2, 6.3: the dry-run validates the roles cell and normalises it; blank on
// update keeps the current roles. fakeImportRepo has PIC 1 [replenish vault], PIC 2 [].
func TestImport_VendorPICRoles(t *testing.T) {
	csvText := picHeaderRoles +
		",V1,,Cici,,,,,,\n" + // row 2: blank roles on create
		",V1,,Dedi,,,,,,none\n" + // row 3: none = empty set
		",V1,,Eka,,,,,,vault;replenish\n" + // row 4: valid, order normalised
		",V1,,Fajar,,,,,,vault;admin\n" + // row 5: unknown code
		",V1,,Gita,,,,,,vault;vault\n" + // row 6: duplicate code
		",V1,,Hadi,,,,,,Vault\n" + // row 7: codes are lower case
		"1,V1,B1,Ani,,,,false,true,\n" + // row 8: blank roles on update = unchanged
		"2,V1,,Budi,,,,false,true,none\n" + // row 9: none equals the current empty set
		"1,V1,B1,Ani,,,,false,true,cashcount\n" // row 10: repeats id 1

	res := dryRun(t, ExportVendorPICs, csvText)

	for _, row := range []int{5, 6, 7} {
		if !hasErr(res, row, "roles") {
			t.Errorf("missing roles error on row %d in %+v", row, res.Preview.Errors)
		}
	}
	if !hasErr(res, 10, "id") {
		t.Errorf("row 10 repeats id 1 and must be rejected, errors %+v", res.Preview.Errors)
	}
	cells := map[int]string{}
	for _, r := range res.Rows {
		cells[r.Row] = r.Cells["roles"]
	}
	want := map[int]string{2: "", 3: "none", 4: "replenish;vault", 8: "replenish;vault", 9: "none"}
	if !reflect.DeepEqual(cells, want) {
		t.Errorf("roles cells = %v, want %v", cells, want)
	}
	if p := res.Preview; p.Creates != 3 || p.Updates != 0 || p.Unchanged != 2 {
		t.Errorf("want 3 creates and 2 unchanged, got %+v", p)
	}
}

// unsortedRolesRepo stores PIC 1's roles in migration 032's default order
// ({replenish,vault,cashcount}), not the sorted order the importer normalizes to.
type unsortedRolesRepo struct{ fakeImportRepo }

func (unsortedRolesRepo) VendorPICs(_ context.Context, after int64, _ string, limit int32) ([]db.ExportVendorPicsBatchRow, error) {
	all := []db.ExportVendorPicsBatchRow{
		{ID: 1, VendorCode: "V1", BranchCode: "B1", Name: "Ani", Roles: []string{"replenish", "vault", "cashcount"}, IsActive: true},
	}
	return page(all, after, limit, func(r db.ExportVendorPicsBatchRow) int64 { return r.ID }), nil
}

// Regression (bugfixes.md 2026-10-29): an unedited export of a PIC whose roles
// are stored unsorted must dry-run as unchanged, not as an update.
func TestImport_VendorPICUnsortedStoredRolesRoundTripUnchanged(t *testing.T) {
	repo := unsortedRolesRepo{}
	var file bytes.Buffer
	if _, err := NewMasterDataExporter(repo).Write(adminCtx(1), ExportVendorPICs, "all", &file, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(file.String(), "replenish;vault;cashcount") {
		t.Fatalf("export must keep the stored order (golden bytes), got %q", file.String())
	}
	res, err := NewMasterDataImporter(repo).DryRun(adminCtx(1), ExportVendorPICs, &file)
	if err != nil {
		t.Fatal(err)
	}
	if p := res.Preview; len(p.Errors) != 0 || p.Updates != 0 || p.Creates != 0 || p.Unchanged != 1 {
		t.Fatalf("want 1 unchanged, got %+v", p)
	}
}

func TestImport_VendorPICRolesChangeIsAnUpdate(t *testing.T) {
	res := dryRun(t, ExportVendorPICs, picHeaderRoles+"1,V1,B1,Ani,,,,false,true,cashcount\n")

	if p := res.Preview; len(p.Errors) != 0 || p.Updates != 1 {
		t.Fatalf("a changed roles cell must be an update, got %+v", p)
	}
}

// Req 6.5: a file from before the roles column is still accepted; its roles stay.
func TestImport_VendorPICLegacyHeaderKeepsRoles(t *testing.T) {
	res := dryRun(t, ExportVendorPICs, picHeaderLegacy+"1,V1,B1,Ani,,,,false,true\n,V1,,Cici,,,,,\n")

	if p := res.Preview; len(p.Errors) != 0 || p.Unchanged != 1 || p.Creates != 1 {
		t.Fatalf("legacy header: want 1 unchanged + 1 create, got %+v", p)
	}
	if got := res.Rows[0].Cells["roles"]; got != "replenish;vault" {
		t.Errorf("unchanged row roles = %q, want the current value", got)
	}
}

func TestParsePicRoles(t *testing.T) {
	for cell, want := range map[string]*[]string{"": nil, "  ": nil, "none": {}, "vault": {"vault"}, "vault; replenish": {"replenish", "vault"}} {
		got, err := parsePicRoles(cell)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("parsePicRoles(%q) = %v, %v; want %v", cell, got, err, want)
		}
	}
	for _, cell := range []string{"x", "vault;", ";vault", "none;vault", "vault;vault"} {
		if _, err := parsePicRoles(cell); err == nil {
			t.Errorf("parsePicRoles(%q): want error", cell)
		}
	}
}
