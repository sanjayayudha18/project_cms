package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

type fakeCashCountAreaRepo struct {
	getByID   *db.GetCashCountAreaAdminByIDRow
	codeTaken *int64
	unknown   []string
	conflicts []db.ListCashCountRegionConflictsRow
	executors int64
	lastExcl  int64
}

func (f *fakeCashCountAreaRepo) List(context.Context, string) ([]db.ListCashCountAreasAdminRow, error) {
	return nil, nil
}
func (f *fakeCashCountAreaRepo) GetByID(context.Context, int64) (*db.GetCashCountAreaAdminByIDRow, error) {
	return f.getByID, nil
}
func (f *fakeCashCountAreaRepo) FindByCode(context.Context, string) (*int64, error) {
	return f.codeTaken, nil
}
func (f *fakeCashCountAreaRepo) RegionConflicts(_ context.Context, _ []string, excl int64) ([]db.ListCashCountRegionConflictsRow, error) {
	f.lastExcl = excl
	return f.conflicts, nil
}
func (f *fakeCashCountAreaRepo) UnknownRegionCodes(context.Context, []string) ([]string, error) {
	return f.unknown, nil
}
func (f *fakeCashCountAreaRepo) ActiveExecutorCount(context.Context, int64) (int64, error) {
	return f.executors, nil
}

func TestCashCountAreaAdminService_Create(t *testing.T) {
	taken := int64(1)
	cases := []struct {
		name    string
		repo    fakeCashCountAreaRepo
		req     CashCountAreaPayload
		field   string
		wantErr error
	}{
		{"happy path", fakeCashCountAreaRepo{}, CashCountAreaPayload{Code: " jkt ", Name: " Jakarta ", RegionCodes: []string{"jkt1", "JKT1", "bdg"}}, "", nil},
		{"bad code", fakeCashCountAreaRepo{}, CashCountAreaPayload{Code: "a", Name: "X", RegionCodes: []string{"JKT"}}, "code", nil},
		{"blank name", fakeCashCountAreaRepo{}, CashCountAreaPayload{Code: "JKT", Name: " ", RegionCodes: []string{"JKT"}}, "name", nil},
		{"no regions", fakeCashCountAreaRepo{}, CashCountAreaPayload{Code: "JKT", Name: "X"}, "region_codes", nil},
		{"bad region format", fakeCashCountAreaRepo{}, CashCountAreaPayload{Code: "JKT", Name: "X", RegionCodes: []string{"J"}}, "region_codes", nil},
		{"region unused by active branch", fakeCashCountAreaRepo{unknown: []string{"ZZZ"}}, CashCountAreaPayload{Code: "JKT", Name: "X", RegionCodes: []string{"ZZZ"}}, "region_codes", nil},
		{"region owned by other area", fakeCashCountAreaRepo{conflicts: []db.ListCashCountRegionConflictsRow{{RegionCode: "JKT", AreaID: 2, AreaCode: "OTH"}}}, CashCountAreaPayload{Code: "JKT", Name: "X", RegionCodes: []string{"JKT"}}, "region_codes", nil},
		{"code taken", fakeCashCountAreaRepo{codeTaken: &taken}, CashCountAreaPayload{Code: "JKT", Name: "X", RegionCodes: []string{"JKT"}}, "", ErrCashCountAreaConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sub := &fakeVendorBranchSubmitter{}
			_, err := NewCashCountAreaAdminService(&tc.repo, sub).Create(context.Background(), 7, tc.req, "ip")

			var ve *ValidationError
			switch {
			case tc.field != "":
				if !errors.As(err, &ve) || ve.Field != tc.field || sub.submitCalled {
					t.Fatalf("want %s ValidationError and no submit, got %v", tc.field, err)
				}
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) || sub.submitCalled {
					t.Fatalf("want %v and no submit, got %v", tc.wantErr, err)
				}
			default:
				if err != nil {
					t.Fatalf("Create: %v", err)
				}
				p := sub.lastRequest.Payload.(CashCountAreaPayload)
				if sub.lastRequest.EntityType != "cash_count_area" || sub.lastRequest.Op != "create" ||
					p.Code != "JKT" || p.Name != "Jakarta" || len(p.RegionCodes) != 2 || p.RegionCodes[0] != "BDG" || p.RegionCodes[1] != "JKT1" {
					t.Errorf("unexpected submit: %+v", sub.lastRequest)
				}
				if sub.lastRequest.Before != nil || sub.lastRequest.EntityID != nil {
					t.Error("create must carry neither Before nor EntityID")
				}
			}
		})
	}
}

func TestCashCountAreaAdminService_UpdateExcludesOwnRegions(t *testing.T) {
	row := &db.GetCashCountAreaAdminByIDRow{ID: 5, Code: "JKT", Name: "Jakarta", IsActive: true, RegionCodes: []string{"JKT"}}
	repo := &fakeCashCountAreaRepo{getByID: row}
	sub := &fakeVendorBranchSubmitter{}
	_, err := NewCashCountAreaAdminService(repo, sub).Update(context.Background(), 7, 5, CashCountAreaUpdatePayload{Name: "Jakarta Raya", RegionCodes: []string{"jkt"}}, "ip")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if repo.lastExcl != 5 {
		t.Errorf("region conflict check must exclude the area itself, got exclude=%d", repo.lastExcl)
	}
	if sub.lastRequest.Op != "update" || sub.lastRequest.Before != row || *sub.lastRequest.EntityID != 5 {
		t.Errorf("unexpected submit: %+v", sub.lastRequest)
	}
}

func TestCashCountAreaAdminService_NotFoundAndDisabled(t *testing.T) {
	ctx := context.Background()
	missing := NewCashCountAreaAdminService(&fakeCashCountAreaRepo{}, &fakeVendorBranchSubmitter{})
	if _, err := missing.Update(ctx, 7, 5, CashCountAreaUpdatePayload{Name: "X", RegionCodes: []string{"JKT"}}, "ip"); !errors.Is(err, ErrCashCountAreaNotFound) {
		t.Errorf("Update: want not found, got %v", err)
	}
	if _, err := missing.Disable(ctx, 7, 5, "ip"); !errors.Is(err, ErrCashCountAreaNotFound) {
		t.Errorf("Disable: want not found, got %v", err)
	}
	if _, err := missing.Enable(ctx, 7, 5, "ip"); !errors.Is(err, ErrCashCountAreaNotFound) {
		t.Errorf("Enable: want not found, got %v", err)
	}
	disabled := &fakeCashCountAreaRepo{getByID: &db.GetCashCountAreaAdminByIDRow{ID: 5, IsActive: false}}
	if _, err := NewCashCountAreaAdminService(disabled, &fakeVendorBranchSubmitter{}).Update(ctx, 7, 5, CashCountAreaUpdatePayload{Name: "X", RegionCodes: []string{"JKT"}}, "ip"); !errors.Is(err, ErrCashCountAreaNotFound) {
		t.Errorf("Update disabled: want not found, got %v", err)
	}
}

func TestCashCountAreaAdminService_DisableBlockedByExecutors(t *testing.T) {
	row := &db.GetCashCountAreaAdminByIDRow{ID: 5, IsActive: true}
	ctx := context.Background()

	sub := &fakeVendorBranchSubmitter{}
	blocked := &fakeCashCountAreaRepo{getByID: row, executors: 2}
	if _, err := NewCashCountAreaAdminService(blocked, sub).Disable(ctx, 7, 5, "ip"); !errors.Is(err, ErrCashCountAreaHasExecutors) || sub.submitCalled {
		t.Fatalf("want ErrCashCountAreaHasExecutors and no submit, got %v", err)
	}

	free := &fakeCashCountAreaRepo{getByID: row}
	if _, err := NewCashCountAreaAdminService(free, sub).Disable(ctx, 7, 5, "ip"); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if sub.lastRequest.Op != "disable" || sub.lastRequest.Before != row {
		t.Errorf("unexpected submit: %+v", sub.lastRequest)
	}
	if _, err := NewCashCountAreaAdminService(free, sub).Enable(ctx, 7, 5, "ip"); err != nil || sub.lastRequest.Op != "enable" {
		t.Errorf("Enable: err=%v req=%+v", err, sub.lastRequest)
	}
}
