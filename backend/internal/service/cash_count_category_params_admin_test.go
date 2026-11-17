package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

type fakeCashCountCategoryParamsRepo struct {
	active *db.CashCountCategoryParam
	err    error
}

func (f *fakeCashCountCategoryParamsRepo) List(context.Context) ([]db.CashCountCategoryParam, error) {
	return nil, nil
}

func (f *fakeCashCountCategoryParamsRepo) GetActive(context.Context) (*db.CashCountCategoryParam, error) {
	return f.active, f.err
}

func validCashCountParams() CashCountCategoryParamsPayload {
	return CashCountCategoryParamsPayload{
		MediumMinAmount: "5000000000", MediumMaxAmount: "15000000000.00",
		OnsiteHigh: 3, OnsiteMedium: 2, OnsiteLow: 1,
	}
}

func TestCashCountCategoryParamsAdminService_Create_Validation(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(p *CashCountCategoryParamsPayload)
		wantField string // "" = valid
	}{
		{"valid seed values", func(*CashCountCategoryParamsPayload) {}, ""},
		{"onsite 0 and 6 allowed", func(p *CashCountCategoryParamsPayload) { p.OnsiteHigh, p.OnsiteLow = 6, 0 }, ""},
		{"min missing", func(p *CashCountCategoryParamsPayload) { p.MediumMinAmount = " " }, "medium_min_amount"},
		{"min not a number", func(p *CashCountCategoryParamsPayload) { p.MediumMinAmount = "5jt" }, "medium_min_amount"},
		{"min fraction form", func(p *CashCountCategoryParamsPayload) { p.MediumMinAmount = "1/2" }, "medium_min_amount"},
		{"min zero", func(p *CashCountCategoryParamsPayload) { p.MediumMinAmount = "0" }, "medium_min_amount"},
		{"min negative", func(p *CashCountCategoryParamsPayload) { p.MediumMinAmount = "-1" }, "medium_min_amount"},
		{"min 3 decimals", func(p *CashCountCategoryParamsPayload) { p.MediumMinAmount = "1.001" }, "medium_min_amount"},
		{"max too large for numeric(20,2)", func(p *CashCountCategoryParamsPayload) { p.MediumMaxAmount = "1000000000000000000" }, "medium_max_amount"},
		{"max 18 digits fits", func(p *CashCountCategoryParamsPayload) { p.MediumMaxAmount = "999999999999999999.99" }, ""},
		{"min equals max", func(p *CashCountCategoryParamsPayload) { p.MediumMaxAmount = "5000000000.00" }, "medium_max_amount"},
		{"min above max", func(p *CashCountCategoryParamsPayload) { p.MediumMaxAmount = "4999999999.99" }, "medium_max_amount"},
		{"currency not IDR", func(p *CashCountCategoryParamsPayload) { p.CurrencyCode = "USD" }, "currency_code"},
		{"onsite high 7", func(p *CashCountCategoryParamsPayload) { p.OnsiteHigh = 7 }, "onsite_high"},
		{"onsite medium negative", func(p *CashCountCategoryParamsPayload) { p.OnsiteMedium = -1 }, "onsite_medium"},
		{"onsite low 7", func(p *CashCountCategoryParamsPayload) { p.OnsiteLow = 7 }, "onsite_low"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &fakeVendorBranchSubmitter{}
			svc := NewCashCountCategoryParamsAdminService(&fakeCashCountCategoryParamsRepo{active: &db.CashCountCategoryParam{ID: 1}}, sub)
			p := validCashCountParams()
			tt.mutate(&p)

			_, err := svc.Create(context.Background(), 7, p, "10.0.0.1")

			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Field != tt.wantField {
				t.Fatalf("got %v, want ValidationError on %s", err, tt.wantField)
			}
			if sub.submitCalled {
				t.Fatal("Submit must not be called on invalid input")
			}
		})
	}
}

func TestCashCountCategoryParamsAdminService_Create_StagesNewSet(t *testing.T) {
	active := &db.CashCountCategoryParam{ID: 42}
	sub := &fakeVendorBranchSubmitter{}
	svc := NewCashCountCategoryParamsAdminService(&fakeCashCountCategoryParamsRepo{active: active}, sub)
	p := validCashCountParams()
	p.SupersedesID = 999 // client value must be ignored
	p.CurrencyCode = " idr "

	if _, err := svc.Create(context.Background(), 7, p, "10.0.0.1"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	req := sub.lastRequest
	if req.EntityType != "cash_count_category_params" || req.Op != "create" || req.EntityID != nil {
		t.Fatalf("submit request = %+v, want create of cash_count_category_params without entity id", req)
	}
	if req.Before != active {
		t.Fatalf("Before = %v, want the active set", req.Before)
	}
	got := req.Payload.(CashCountCategoryParamsPayload)
	want := CashCountCategoryParamsPayload{
		MediumMinAmount: "5000000000.00", MediumMaxAmount: "15000000000.00", CurrencyCode: "IDR",
		OnsiteHigh: 3, OnsiteMedium: 2, OnsiteLow: 1, SupersedesID: 42,
	}
	if got != want {
		t.Fatalf("payload = %+v, want %+v", got, want)
	}
}

func TestCashCountCategoryParamsAdminService_Create_NoActiveSet(t *testing.T) {
	sub := &fakeVendorBranchSubmitter{}
	svc := NewCashCountCategoryParamsAdminService(&fakeCashCountCategoryParamsRepo{}, sub)

	_, err := svc.Create(context.Background(), 7, validCashCountParams(), "")

	if !errors.Is(err, ErrCashCountCategoryParamsMissing) || sub.submitCalled {
		t.Fatalf("got err=%v submitCalled=%v, want ErrCashCountCategoryParamsMissing without submit", err, sub.submitCalled)
	}
}

func TestCashCountCategoryParamsAdminService_Create_SubmitErrorPropagates(t *testing.T) {
	sub := &fakeVendorBranchSubmitter{submitFunc: func(context.Context, int64, SubmitRequest, string) (db.MasterDataChangeRequest, error) {
		return db.MasterDataChangeRequest{}, ErrMasterDataForbidden
	}}
	svc := NewCashCountCategoryParamsAdminService(&fakeCashCountCategoryParamsRepo{active: &db.CashCountCategoryParam{ID: 1}}, sub)

	_, err := svc.Create(context.Background(), 7, validCashCountParams(), "")

	if !errors.Is(err, ErrMasterDataForbidden) {
		t.Fatalf("got %v, want ErrMasterDataForbidden", err)
	}
}

func TestCashCountCategoryParamsApplier_RejectsNonCreateOps(t *testing.T) {
	for _, op := range []string{"update", "disable", "enable"} {
		id := int64(1)
		_, _, err := CashCountCategoryParamsApplier{}.Apply(context.Background(), nil, db.MasterDataChangeRequest{Op: op, EntityID: &id})
		if err == nil {
			t.Fatalf("op=%s: want error", op)
		}
	}
}
