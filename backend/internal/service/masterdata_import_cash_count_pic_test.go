package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// fakePicImportRepo serves both the replace-all dry run (CashCountPicImportRepo) and
// the stage step (CashCountPicAdminRepo). Users: alice + carol are COORDINATOR,
// bob is EXECUTOR, dave has no CC role. Region JKT in use, area AR1 (id 7) active.
type fakePicImportRepo struct {
	active []db.ListCashCountPicsAdminRow
}

var picTestUsers = map[string]int64{"alice": 11, "bob": 12, "dave": 13, "carol": 14}

func (f *fakePicImportRepo) ActivePics(context.Context) ([]db.ListCashCountPicsAdminRow, error) {
	return f.active, nil
}
func (f *fakePicImportRepo) ActiveAreas(context.Context) ([]db.ListCashCountAreasAdminRow, error) {
	return []db.ListCashCountAreasAdminRow{{ID: 7, Code: "AR1", IsActive: true}}, nil
}
func (f *fakePicImportRepo) UserIDByUsername(_ context.Context, u string) (int64, bool, error) {
	id, ok := picTestUsers[u]
	return id, ok, nil
}
func (f *fakePicImportRepo) UserEligible(_ context.Context, id int64, role string) (bool, error) {
	return ((id == 11 || id == 14) && role == picRoleCoordinator) || (id == 12 && role == picRoleExecutor), nil
}
func (f *fakePicImportRepo) RegionInUse(_ context.Context, r string) (bool, error) {
	return r == "JKT", nil
}
func (f *fakePicImportRepo) List(context.Context, db.ListCashCountPicsAdminParams) ([]db.ListCashCountPicsAdminRow, error) {
	return nil, nil
}
func (f *fakePicImportRepo) GetByID(_ context.Context, id int64) (*db.GetCashCountPicAdminByIDRow, error) {
	for _, a := range f.active {
		if a.ID == id {
			return &db.GetCashCountPicAdminByIDRow{ID: a.ID, PicRole: a.PicRole, UserID: a.UserID, RegionCode: a.RegionCode, AreaID: a.AreaID, IsActive: true}, nil
		}
	}
	return nil, nil
}
func (f *fakePicImportRepo) AreaActive(_ context.Context, id int64) (bool, error) {
	return id == 7, nil
}
func (f *fakePicImportRepo) FindActive(_ context.Context, a db.FindActiveCashCountPicParams) (bool, error) {
	for _, x := range f.active {
		if x.PicRole == a.PicRole && x.UserID == a.UserID && picDeref(x.RegionCode) == a.RegionCode && picDerefInt(x.AreaID) == a.AreaID && x.ID != a.ExcludeID {
			return true, nil
		}
	}
	return false, nil
}

func picStrp(s string) *string { return &s }
func picIntp(n int64) *int64   { return &n }

func newPicImportRepo() *fakePicImportRepo {
	return &fakePicImportRepo{active: []db.ListCashCountPicsAdminRow{
		{ID: 1, PicRole: picRoleCoordinator, UserID: 11, Username: "alice", RegionCode: picStrp("JKT"), IsActive: true},
		{ID: 2, PicRole: picRoleExecutor, UserID: 12, Username: "bob", AreaID: picIntp(7), AreaCode: picStrp("AR1"), IsActive: true},
	}}
}

func picDryRun(t *testing.T, ctx context.Context, csv string) *ImportResult {
	t.Helper()
	res, err := NewMasterDataImporter(nil).WithCashCountPics(newPicImportRepo()).DryRun(ctx, ExportCashCountPICs, strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	return res
}

const ccPicCSVHeader = "pic_role,username,region_code,area_code\n"

// AC-5.1-11: the file is the full set; an active mapping missing from it is disabled.
func TestCashCountPicImport_ReplaceAllDisablesMissing(t *testing.T) {
	res := picDryRun(t, ctxWithRole(1, "CC-CWM"), ccPicCSVHeader+"coordinator,alice,jkt,\n")
	p := res.Preview
	if len(p.Errors) != 0 {
		t.Fatalf("errors: %+v", p.Errors)
	}
	if p.Unchanged != 1 || p.Creates != 0 || p.Disables != 1 || p.Updates != 1 {
		t.Fatalf("preview counts = %+v", p)
	}
	if len(p.Changes) != 1 || p.Changes[0].Op != ImportOpDisable || p.Changes[0].Key != "EXECUTOR / bob / AR1" || p.Changes[0].Row != 0 {
		t.Fatalf("changes = %+v", p.Changes)
	}
}

func TestCashCountPicImport_DuplicateInFile(t *testing.T) {
	res := picDryRun(t, ctxWithRole(1, "CC-CWM"), ccPicCSVHeader+"COORDINATOR,alice,JKT,\nEXECUTOR,bob,,AR1\nEXECUTOR,bob,,ar1\n")
	if errs := res.Preview.Errors; len(errs) != 1 || errs[0].Row != 4 || !strings.Contains(errs[0].Message, "duplikat dengan baris 3") {
		t.Fatalf("errors = %+v", errs)
	}
	if len(res.Preview.Changes) != 0 {
		t.Fatalf("changes must be empty when the file has errors: %+v", res.Preview.Changes)
	}
}

func TestCashCountPicImport_RowValidation(t *testing.T) {
	cases := []struct{ name, line, field string }{
		{"bad role", "BOSS,alice,JKT,", "pic_role"},
		{"coordinator needs region", "COORDINATOR,alice,,", "region_code"},
		{"coordinator area not allowed", "COORDINATOR,alice,JKT,AR1", "area_code"},
		{"executor region not allowed", "EXECUTOR,bob,JKT,AR1", "region_code"},
		{"unknown user", "COORDINATOR,ghost,JKT,", "username"},
		{"user without CC role", "COORDINATOR,dave,JKT,", "username"},
		{"wrong role for user", "EXECUTOR,alice,,AR1", "username"},
		{"region not in use", "COORDINATOR,alice,BDG,", "region_code"},
		{"unknown area", "EXECUTOR,bob,,AR9", "area_code"},
		{"blank username", "COORDINATOR,,JKT,", "username"},
		{"wrong cell count", "COORDINATOR,alice,JKT", "row"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := picDryRun(t, ctxWithRole(1, "CC-CWM"), ccPicCSVHeader+tc.line+"\n").Preview.Errors
			if len(errs) == 0 || errs[0].Field != tc.field || errs[0].Row != 2 {
				t.Fatalf("errors = %+v, want field %s on row 2", errs, tc.field)
			}
		})
	}
}

func TestCashCountPicImport_EmptyFileRefused(t *testing.T) {
	res := picDryRun(t, ctxWithRole(1, "CC-CWM"), ccPicCSVHeader)
	if len(res.Preview.Errors) != 1 || res.Preview.Errors[0].Field != "file" || len(res.Rows) != 0 {
		t.Fatalf("empty file must not disable everything: %+v", res.Preview)
	}
}

// AC-5.1-14: only CC-CWM imports mapping PIC; CC-CWM cannot import other entities.
func TestCashCountPicImport_RBAC(t *testing.T) {
	for _, role := range []string{"ADMIN", "ADMIN_PARAM", "ATM-SPV", "CC-EXECUTOR"} {
		imp := NewMasterDataImporter(nil).WithCashCountPics(newPicImportRepo()).WithConfirm(ImportConfirmDeps{})
		if _, err := imp.DryRun(ctxWithRole(1, role), ExportCashCountPICs, strings.NewReader(ccPicCSVHeader)); !errors.Is(err, ErrMasterDataForbidden) {
			t.Errorf("%s dry-run: want forbidden, got %v", role, err)
		}
		if _, err := imp.Confirm(ctxWithRole(1, role), 1, ExportCashCountPICs, "x.csv", strings.NewReader(ccPicCSVHeader), ""); !errors.Is(err, ErrMasterDataForbidden) {
			t.Errorf("%s confirm: want forbidden, got %v", role, err)
		}
	}
	if _, err := NewMasterDataImporter(nil).DryRun(ctxWithRole(1, "CC-CWM"), ExportVendors, strings.NewReader("")); !errors.Is(err, ErrMasterDataForbidden) {
		t.Errorf("CC-CWM importing vendors: want forbidden, got %v", err)
	}
}

// One file = one create (carol) + one disable (bob, missing), staged through the
// same service the single-record endpoint uses.
func TestCashCountPicImport_StageCreateAndDisable(t *testing.T) {
	ctx := ctxWithRole(1, "CC-CWM")
	res := picDryRun(t, ctx, ccPicCSVHeader+"COORDINATOR,alice,JKT,\nCOORDINATOR,carol,JKT,\n")
	if len(res.Preview.Errors) != 0 {
		t.Fatalf("errors = %+v", res.Preview.Errors)
	}
	collector := &importCollector{}
	svcs := ImportServices{CashCountPICs: NewCashCountPicAdminService(newPicImportRepo(), collector)}
	for _, row := range res.Rows {
		if row.Op == ImportOpUnchanged {
			continue
		}
		if err := stageRow(ctx, 1, ExportCashCountPICs, row, nil, svcs, ""); err != nil {
			t.Fatalf("stage row %d: %v", row.Row, err)
		}
	}
	if len(collector.reqs) != 2 {
		t.Fatalf("want 2 staged changes, got %+v", collector.reqs)
	}
	create, disable := collector.reqs[0].req, collector.reqs[1].req
	p, _ := create.Payload.(CashCountPicPayload)
	if create.Op != "create" || create.EntityType != cashCountPicEntity || p.UserID != 14 || picDeref(p.RegionCode) != "JKT" || p.AreaID != nil {
		t.Fatalf("create = %+v", create)
	}
	if disable.Op != "disable" || disable.EntityID == nil || *disable.EntityID != 2 || disable.Before == nil {
		t.Fatalf("disable = %+v", disable)
	}
	if err := stageRow(ctx, 1, ExportCashCountPICs, res.Rows[1], nil, ImportServices{}, ""); err == nil {
		t.Fatal("stage without the PIC service must fail")
	}
}
