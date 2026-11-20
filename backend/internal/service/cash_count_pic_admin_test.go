package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

type fakePicRepo struct {
	row      *db.GetCashCountPicAdminByIDRow
	eligible bool
	region   bool
	area     bool
	dup      bool
}

func (f *fakePicRepo) List(context.Context, db.ListCashCountPicsAdminParams) ([]db.ListCashCountPicsAdminRow, error) {
	return nil, nil
}
func (f *fakePicRepo) GetByID(context.Context, int64) (*db.GetCashCountPicAdminByIDRow, error) {
	return f.row, nil
}
func (f *fakePicRepo) UserEligible(context.Context, int64, string) (bool, error) {
	return f.eligible, nil
}
func (f *fakePicRepo) RegionInUse(context.Context, string) (bool, error) { return f.region, nil }
func (f *fakePicRepo) AreaActive(context.Context, int64) (bool, error)   { return f.area, nil }
func (f *fakePicRepo) FindActive(context.Context, db.FindActiveCashCountPicParams) (bool, error) {
	return f.dup, nil
}

type fakePicSubmitter struct{ got *SubmitRequest }

func (f *fakePicSubmitter) Submit(_ context.Context, _ int64, req SubmitRequest, _ string) (db.MasterDataChangeRequest, error) {
	f.got = &req
	return db.MasterDataChangeRequest{}, nil
}

func picStr(s string) *string { return &s }
func picInt(n int64) *int64   { return &n }

func TestCashCountPicAdminService_Create(t *testing.T) {
	ok := func() *fakePicRepo { return &fakePicRepo{eligible: true, region: true, area: true} }
	tests := []struct {
		name    string
		repo    *fakePicRepo
		req     CashCountPicPayload
		wantErr string // "" = submitted; "validation" or "conflict"
	}{
		{"coordinator ok", ok(), CashCountPicPayload{PicRole: "COORDINATOR", UserID: 1, RegionCode: picStr(" jkt ")}, ""},
		{"executor ok", ok(), CashCountPicPayload{PicRole: "EXECUTOR", UserID: 1, AreaID: picInt(3)}, ""},
		{"bad role", ok(), CashCountPicPayload{PicRole: "BOSS", UserID: 1}, "validation"},
		{"no user", ok(), CashCountPicPayload{PicRole: "EXECUTOR", AreaID: picInt(3)}, "validation"},
		{"user not eligible", &fakePicRepo{region: true, area: true}, CashCountPicPayload{PicRole: "EXECUTOR", UserID: 1, AreaID: picInt(3)}, "validation"},
		{"coordinator without region", ok(), CashCountPicPayload{PicRole: "COORDINATOR", UserID: 1}, "validation"},
		{"region unused", &fakePicRepo{eligible: true}, CashCountPicPayload{PicRole: "COORDINATOR", UserID: 1, RegionCode: picStr("JKT")}, "validation"},
		{"executor without area", ok(), CashCountPicPayload{PicRole: "EXECUTOR", UserID: 1}, "validation"},
		{"area inactive", &fakePicRepo{eligible: true}, CashCountPicPayload{PicRole: "EXECUTOR", UserID: 1, AreaID: picInt(3)}, "validation"},
		{"duplicate active", &fakePicRepo{eligible: true, area: true, dup: true}, CashCountPicPayload{PicRole: "EXECUTOR", UserID: 1, AreaID: picInt(3)}, "conflict"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &fakePicSubmitter{}
			_, err := NewCashCountPicAdminService(tt.repo, sub).Create(context.Background(), 9, tt.req, "")
			var ve *ValidationError
			switch tt.wantErr {
			case "":
				if err != nil || sub.got == nil || sub.got.Op != "create" || sub.got.EntityID != nil || sub.got.Before != nil {
					t.Fatalf("want create submitted, err=%v got=%+v", err, sub.got)
				}
			case "validation":
				if !errors.As(err, &ve) || sub.got != nil {
					t.Fatalf("want ValidationError, err=%v", err)
				}
			case "conflict":
				if !errors.Is(err, ErrCashCountPicConflict) || sub.got != nil {
					t.Fatalf("want conflict, err=%v", err)
				}
			}
		})
	}
}

func TestCashCountPicAdminService_CreateNormalisesTarget(t *testing.T) {
	sub := &fakePicSubmitter{}
	repo := &fakePicRepo{eligible: true, region: true}
	// stray area_id on a COORDINATOR must be dropped; region trimmed + upper-cased.
	_, err := NewCashCountPicAdminService(repo, sub).Create(context.Background(), 9,
		CashCountPicPayload{PicRole: "COORDINATOR", UserID: 1, RegionCode: picStr(" jkt "), AreaID: picInt(7)}, "")
	if err != nil {
		t.Fatal(err)
	}
	p := sub.got.Payload.(CashCountPicPayload)
	if p.AreaID != nil || p.RegionCode == nil || *p.RegionCode != "JKT" {
		t.Fatalf("payload not normalised: %+v", p)
	}
}

func TestCashCountPicAdminService_UpdateDisableEnable(t *testing.T) {
	active := &db.GetCashCountPicAdminByIDRow{ID: 5, PicRole: "EXECUTOR", UserID: 1, AreaID: picInt(3), IsActive: true}
	disabled := &db.GetCashCountPicAdminByIDRow{ID: 5, PicRole: "EXECUTOR", UserID: 1, AreaID: picInt(3)}
	ctx := context.Background()

	t.Run("update passes before", func(t *testing.T) {
		sub := &fakePicSubmitter{}
		_, err := NewCashCountPicAdminService(&fakePicRepo{row: active, eligible: true, area: true}, sub).
			Update(ctx, 9, 5, CashCountPicUpdatePayload{UserID: 2, AreaID: picInt(4)}, "")
		if err != nil || sub.got.Before != active || sub.got.Op != "update" {
			t.Fatalf("err=%v got=%+v", err, sub.got)
		}
	})
	t.Run("update of disabled row is not found", func(t *testing.T) {
		_, err := NewCashCountPicAdminService(&fakePicRepo{row: disabled}, &fakePicSubmitter{}).
			Update(ctx, 9, 5, CashCountPicUpdatePayload{UserID: 2, AreaID: picInt(4)}, "")
		if !errors.Is(err, ErrCashCountPicNotFound) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("missing id is not found", func(t *testing.T) {
		_, err := NewCashCountPicAdminService(&fakePicRepo{}, &fakePicSubmitter{}).Disable(ctx, 9, 5, "")
		if !errors.Is(err, ErrCashCountPicNotFound) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("disable passes before", func(t *testing.T) {
		sub := &fakePicSubmitter{}
		if _, err := NewCashCountPicAdminService(&fakePicRepo{row: active}, sub).Disable(ctx, 9, 5, ""); err != nil || sub.got.Before != active {
			t.Fatalf("err=%v got=%+v", err, sub.got)
		}
	})
	t.Run("enable refused when duplicate became active", func(t *testing.T) {
		_, err := NewCashCountPicAdminService(&fakePicRepo{row: disabled, dup: true}, &fakePicSubmitter{}).Enable(ctx, 9, 5, "")
		if !errors.Is(err, ErrCashCountPicConflict) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("enable ok", func(t *testing.T) {
		sub := &fakePicSubmitter{}
		if _, err := NewCashCountPicAdminService(&fakePicRepo{row: disabled}, sub).Enable(ctx, 9, 5, ""); err != nil || sub.got.Op != "enable" {
			t.Fatalf("err=%v got=%+v", err, sub.got)
		}
	})
}
