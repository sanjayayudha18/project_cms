package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

type fakeChecklistRepo struct {
	getByID    *db.GetCashCountChecklistItemAdminByIDRow
	findActive *int64
}

func (f *fakeChecklistRepo) List(context.Context, db.ListCashCountChecklistItemsAdminParams) ([]db.CashCountChecklistItem, error) {
	return nil, nil
}
func (f *fakeChecklistRepo) GetByID(context.Context, int64) (*db.GetCashCountChecklistItemAdminByIDRow, error) {
	return f.getByID, nil
}
func (f *fakeChecklistRepo) FindActivePoint(context.Context, string, string, int16, int64) (*int64, error) {
	return f.findActive, nil
}

func TestCashCountChecklistItemAdminService_Create(t *testing.T) {
	dup := int64(1)
	cases := []struct {
		name    string
		repo    fakeChecklistRepo
		req     CashCountChecklistItemPayload
		field   string
		wantErr error
	}{
		{"machine day1 ok", fakeChecklistRepo{}, CashCountChecklistItemPayload{Category: "SELECTED_MACHINE", Day: strp("DAY1"), PointNo: 1, ItemText: " Cek kaset "}, "", nil},
		{"machine day2 ok", fakeChecklistRepo{}, CashCountChecklistItemPayload{Category: "SELECTED_MACHINE", Day: strp("DAY2"), PointNo: 1, ItemText: "Cek vault"}, "", nil},
		{"non machine ok", fakeChecklistRepo{}, CashCountChecklistItemPayload{Category: "NON_MACHINE", PointNo: 2, ItemText: "Umum"}, "", nil},
		{"bad category", fakeChecklistRepo{}, CashCountChecklistItemPayload{Category: "X", PointNo: 1, ItemText: "a"}, "category", nil},
		{"machine without day", fakeChecklistRepo{}, CashCountChecklistItemPayload{Category: "SELECTED_MACHINE", PointNo: 1, ItemText: "a"}, "day", nil},
		{"machine bad day", fakeChecklistRepo{}, CashCountChecklistItemPayload{Category: "SELECTED_MACHINE", Day: strp("DAY3"), PointNo: 1, ItemText: "a"}, "day", nil},
		{"non machine with day", fakeChecklistRepo{}, CashCountChecklistItemPayload{Category: "NON_MACHINE", Day: strp("DAY1"), PointNo: 1, ItemText: "a"}, "day", nil},
		{"blank text", fakeChecklistRepo{}, CashCountChecklistItemPayload{Category: "NON_MACHINE", PointNo: 1, ItemText: "  "}, "item_text", nil},
		{"text too long", fakeChecklistRepo{}, CashCountChecklistItemPayload{Category: "NON_MACHINE", PointNo: 1, ItemText: strings.Repeat("x", 501)}, "item_text", nil},
		{"point zero", fakeChecklistRepo{}, CashCountChecklistItemPayload{Category: "NON_MACHINE", PointNo: 0, ItemText: "a"}, "point_no", nil},
		{"point reused", fakeChecklistRepo{findActive: &dup}, CashCountChecklistItemPayload{Category: "NON_MACHINE", PointNo: 1, ItemText: "a"}, "", ErrCashCountChecklistItemConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sub := &fakeVendorBranchSubmitter{}
			svc := NewCashCountChecklistItemAdminService(&tc.repo, sub)
			_, err := svc.Create(context.Background(), 7, tc.req, "ip")

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
				p := sub.lastRequest.Payload.(CashCountChecklistItemPayload)
				if sub.lastRequest.EntityType != "cash_count_checklist_item" || sub.lastRequest.Op != "create" ||
					sub.lastRequest.EntityID != nil || sub.lastRequest.Before != nil || p.ItemText != strings.TrimSpace(tc.req.ItemText) {
					t.Errorf("unexpected submit: %+v", sub.lastRequest)
				}
			}
		})
	}
}

func TestCashCountChecklistItemAdminService_UpdateDisableEnable(t *testing.T) {
	day := "DAY1"
	active := &db.GetCashCountChecklistItemAdminByIDRow{ID: 5, Category: "SELECTED_MACHINE", Day: &day, PointNo: 1, ItemText: "a", IsActive: true}
	off := &db.GetCashCountChecklistItemAdminByIDRow{ID: 5, Category: "SELECTED_MACHINE", Day: &day, PointNo: 1, ItemText: "a", IsActive: false}
	dup := int64(9)
	ctx := context.Background()

	t.Run("update passes Before and keeps category rules", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewCashCountChecklistItemAdminService(&fakeChecklistRepo{getByID: active}, sub)
		if _, err := svc.Update(ctx, 7, 5, CashCountChecklistItemUpdatePayload{Day: strp("DAY2"), PointNo: 3, ItemText: "b"}, "ip"); err != nil {
			t.Fatalf("Update: %v", err)
		}
		if sub.lastRequest.Op != "update" || sub.lastRequest.Before != active || sub.lastRequest.EntityID == nil || *sub.lastRequest.EntityID != 5 {
			t.Errorf("unexpected submit: %+v", sub.lastRequest)
		}
		// SELECTED_MACHINE item cannot lose its day.
		var ve *ValidationError
		if _, err := svc.Update(ctx, 7, 5, CashCountChecklistItemUpdatePayload{PointNo: 3, ItemText: "b"}, "ip"); !errors.As(err, &ve) || ve.Field != "day" {
			t.Errorf("want day ValidationError, got %v", err)
		}
	})
	t.Run("update disabled or missing is not found", func(t *testing.T) {
		for _, r := range []*db.GetCashCountChecklistItemAdminByIDRow{off, nil} {
			sub := &fakeVendorBranchSubmitter{}
			svc := NewCashCountChecklistItemAdminService(&fakeChecklistRepo{getByID: r}, sub)
			_, err := svc.Update(ctx, 7, 5, CashCountChecklistItemUpdatePayload{Day: strp("DAY1"), PointNo: 1, ItemText: "a"}, "ip")
			if !errors.Is(err, ErrCashCountChecklistItemNotFound) || sub.submitCalled {
				t.Errorf("want not found, got %v", err)
			}
		}
	})
	t.Run("update point reused", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewCashCountChecklistItemAdminService(&fakeChecklistRepo{getByID: active, findActive: &dup}, sub)
		_, err := svc.Update(ctx, 7, 5, CashCountChecklistItemUpdatePayload{Day: strp("DAY1"), PointNo: 2, ItemText: "a"}, "ip")
		if !errors.Is(err, ErrCashCountChecklistItemConflict) || sub.submitCalled {
			t.Errorf("want conflict, got %v", err)
		}
	})
	t.Run("disable passes Before", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewCashCountChecklistItemAdminService(&fakeChecklistRepo{getByID: active}, sub)
		if _, err := svc.Disable(ctx, 7, 5, "ip"); err != nil || sub.lastRequest.Op != "disable" || sub.lastRequest.Before != active {
			t.Errorf("Disable: %v %+v", err, sub.lastRequest)
		}
	})
	t.Run("enable refused when point reused", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewCashCountChecklistItemAdminService(&fakeChecklistRepo{getByID: off, findActive: &dup}, sub)
		if _, err := svc.Enable(ctx, 7, 5, "ip"); !errors.Is(err, ErrCashCountChecklistItemConflict) || sub.submitCalled {
			t.Errorf("want conflict, got %v", err)
		}
		svc = NewCashCountChecklistItemAdminService(&fakeChecklistRepo{getByID: off}, sub)
		if _, err := svc.Enable(ctx, 7, 5, "ip"); err != nil || sub.lastRequest.Op != "enable" {
			t.Errorf("Enable: %v %+v", err, sub.lastRequest)
		}
	})
	t.Run("disable missing is not found", func(t *testing.T) {
		svc := NewCashCountChecklistItemAdminService(&fakeChecklistRepo{}, &fakeVendorBranchSubmitter{})
		if _, err := svc.Disable(ctx, 7, 5, "ip"); !errors.Is(err, ErrCashCountChecklistItemNotFound) {
			t.Errorf("want not found, got %v", err)
		}
	})
}
