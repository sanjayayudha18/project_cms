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
	if got := res.Rows[0].Cells["is_director"]; got != "false" {
		t.Errorf("legacy header must keep the current is_director, got %q", got)
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

const picHeaderDirector = "id,vendor_code,branch_code,name,position,phone,email,is_notification_recipient,is_active,roles,is_director\n"

// directorImportRepo adds to fakeImportRepo's PICs an active Direktur Utama (3)
// and an old inactive one (4), both vendor-level with no roles.
type directorImportRepo struct{ fakeImportRepo }

func (directorImportRepo) VendorPICs(ctx context.Context, after int64, status string, limit int32) ([]db.ExportVendorPicsBatchRow, error) {
	base, _ := fakeImportRepo{}.VendorPICs(ctx, 0, status, 100)
	base = append(base,
		db.ExportVendorPicsBatchRow{ID: 3, VendorCode: "V1", Name: "Dirut Aktif", Roles: []string{}, IsActive: true, IsDirector: true},
		db.ExportVendorPicsBatchRow{ID: 4, VendorCode: "V1", Name: "Dirut Lama", Roles: []string{}, IsActive: false, IsDirector: true},
	)
	return page(base, after, limit, func(r db.ExportVendorPicsBatchRow) int64 { return r.ID }), nil
}

func dryRunWith(t *testing.T, repo MasterDataImportRepo, csvText string) *ImportResult {
	t.Helper()
	res, err := NewMasterDataImporter(repo).DryRun(adminCtx(7), ExportVendorPICs, strings.NewReader(csvText))
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	return res
}

// vendor-pks Req 2.1-2.4 (Property 11): director rows follow the same rules as
// VendorPicAdminService.validate, inside one file and against the DB.
func TestImport_VendorPICDirectorRules(t *testing.T) {
	csvText := picHeaderDirector +
		",V1,,Dirut Baru,,,,false,true,,true\n" + // row 2: first director in file, roles blank -> none
		",V1,,Dirut Kedua,,,,false,true,,true\n" + // row 3: second director for V1 in the same file
		",V1,B1,Dirut Cabang,,,,false,true,,true\n" + // row 4: director with a branch
		",V1,,Dirut Notif,,,,true,true,,true\n" + // row 5: director + notification recipient
		",V1,,Dirut Peran,,,,false,true,vault,true\n" + // row 6: director with roles
		"2,V1,,Budi,,,,false,true,none,false\n" // row 7: unchanged non-director

	res := dryRunWith(t, fakeImportRepo{}, csvText)

	for _, want := range []struct {
		row   int
		field string
	}{{3, "is_director"}, {4, "branch_code"}, {5, "is_notification_recipient"}, {6, "roles"}} {
		if !hasErr(res, want.row, want.field) {
			t.Errorf("missing error row %d field %s in %+v", want.row, want.field, res.Preview.Errors)
		}
	}
	if hasErr(res, 2, "is_director") || hasErr(res, 2, "roles") {
		t.Errorf("row 2 is a valid director, got %+v", res.Preview.Errors)
	}
	cells := map[int]string{}
	for _, r := range res.Rows {
		cells[r.Row] = r.Cells["roles"]
	}
	if cells[2] != "none" {
		t.Errorf("director with blank roles must resolve to none, got %q", cells[2])
	}
	if p := res.Preview; p.Creates != 1 || p.Unchanged != 1 {
		t.Errorf("want 1 valid create and 1 unchanged, got %+v", p)
	}
}

func TestImport_VendorPICDirectorAgainstDB(t *testing.T) {
	csvText := picHeaderDirector +
		",V1,,Dirut Baru,,,,false,true,,true\n" + // row 2: V1 already has an active director (id 3)
		"2,V1,,Budi,,,,false,true,none,true\n" + // row 3: promoting Budi collides too
		"3,V1,,Dirut Aktif,,,,false,true,none,true\n" + // row 4: the director itself, unchanged
		"4,V1,,Dirut Lama,,,,false,false,none,true\n" + // row 5: old inactive director, unchanged
		"4,V1,,Dirut Lama,,,,false,true,none,true\n" // row 6: repeats id 4 (row-level id error)

	res := dryRunWith(t, directorImportRepo{}, csvText)

	for _, row := range []int{2, 3} {
		if !hasErr(res, row, "is_director") {
			t.Errorf("row %d must collide with the active director, errors %+v", row, res.Preview.Errors)
		}
	}
	for _, row := range []int{4, 5} {
		if hasErr(res, row, "is_director") {
			t.Errorf("row %d (unchanged director) must not be flagged, errors %+v", row, res.Preview.Errors)
		}
	}
}

// Req 2.2: a file written before is_director existed keeps the flag of every row it
// updates, and creates non-directors.
func TestImport_VendorPICLegacyHeaderKeepsDirectorFlag(t *testing.T) {
	res := dryRunWith(t, directorImportRepo{}, picHeaderRoles+"3,V1,,Dirut Aktif,,,,false,true,none\n,V1,,Baru,,,,,,\n")

	if p := res.Preview; len(p.Errors) != 0 || p.Unchanged != 1 || p.Creates != 1 {
		t.Fatalf("legacy header: want 1 unchanged + 1 create, got %+v", p)
	}
	if got := res.Rows[0].Cells["is_director"]; got != "true" {
		t.Errorf("updated row is_director = %q, want kept true", got)
	}
	if got := res.Rows[1].Cells["is_director"]; got != "false" {
		t.Errorf("created row is_director = %q, want false", got)
	}
}

// Req 2.3 (Property 11): export -> import of PICs that include directors is no change.
func TestImport_VendorPICDirectorRoundTripUnchanged(t *testing.T) {
	repo := directorImportRepo{}
	var file bytes.Buffer
	if _, err := NewMasterDataExporter(repo).Write(adminCtx(1), ExportVendorPICs, "all", &file, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(file.String(), ",is_director\n") && !strings.Contains(file.String(), "is_director") {
		t.Fatalf("export lacks is_director column: %q", file.String())
	}
	res, err := NewMasterDataImporter(repo).DryRun(adminCtx(1), ExportVendorPICs, &file)
	if err != nil {
		t.Fatal(err)
	}
	if p := res.Preview; len(p.Errors) != 0 || p.Creates != 0 || p.Updates != 0 || p.Unchanged != 4 {
		t.Fatalf("want 4 unchanged, got %+v", p)
	}
}

// vendor-cis-limit FR3.3 / S5: the importer accepts bm only on a cabang PIC.
func TestImport_VendorPICRoleBm(t *testing.T) {
	csvText := picHeaderRoles +
		",V1,B1,Indra,,,,,,bm;vault\n" + // row 2: cabang PIC with bm
		",V1,,Joko,,,,,,bm\n" // row 3: vendor-wide PIC with bm

	res := dryRun(t, ExportVendorPICs, csvText)

	if hasErr(res, 2, "roles") {
		t.Errorf("row 2 (cabang PIC) must accept bm, errors %+v", res.Preview.Errors)
	}
	if !hasErr(res, 3, "roles") {
		t.Errorf("row 3 (vendor-wide PIC) must reject bm, errors %+v", res.Preview.Errors)
	}
}
