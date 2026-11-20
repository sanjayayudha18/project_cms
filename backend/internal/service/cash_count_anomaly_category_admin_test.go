package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

type fakeAnomalyRepo struct {
	getByID *db.GetCashCountAnomalyCategoryAdminByIDRow
	byCode  *int64
}

func (f *fakeAnomalyRepo) List(context.Context, string) ([]db.CashCountAnomalyCategory, error) {
	return nil, nil
}
func (f *fakeAnomalyRepo) GetByID(context.Context, int64) (*db.GetCashCountAnomalyCategoryAdminByIDRow, error) {
	return f.getByID, nil
}
func (f *fakeAnomalyRepo) FindByCode(context.Context, string) (*int64, error) { return f.byCode, nil }

func TestCashCountAnomalyCategoryAdminService_Create(t *testing.T) {
	dup := int64(1)
	long := strings.Repeat("x", 501)
	cases := []struct {
		name    string
		repo    fakeAnomalyRepo
		req     CashCountAnomalyCategoryPayload
		field   string
		wantErr error
	}{
		{"happy path", fakeAnomalyRepo{}, CashCountAnomalyCategoryPayload{Code: " SKIMMING ", Name: " Skimming "}, "", nil},
		{"lowercase code", fakeAnomalyRepo{}, CashCountAnomalyCategoryPayload{Code: "skim", Name: "x"}, "code", nil},
		{"code too short", fakeAnomalyRepo{}, CashCountAnomalyCategoryPayload{Code: "A", Name: "x"}, "code", nil},
		{"blank name", fakeAnomalyRepo{}, CashCountAnomalyCategoryPayload{Code: "AB", Name: " "}, "name", nil},
		{"description too long", fakeAnomalyRepo{}, CashCountAnomalyCategoryPayload{Code: "AB", Name: "x", Description: &long}, "description", nil},
		{"code taken (even disabled)", fakeAnomalyRepo{byCode: &dup}, CashCountAnomalyCategoryPayload{Code: "AB", Name: "x"}, "", ErrCashCountAnomalyCategoryConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sub := &fakeVendorBranchSubmitter{}
			svc := NewCashCountAnomalyCategoryAdminService(&tc.repo, sub)
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
				p := sub.lastRequest.Payload.(CashCountAnomalyCategoryPayload)
				if sub.lastRequest.EntityType != "cash_count_anomaly_category" || sub.lastRequest.Op != "create" ||
					sub.lastRequest.EntityID != nil || sub.lastRequest.Before != nil || p.Code != "SKIMMING" || p.Name != "Skimming" {
					t.Errorf("unexpected submit: %+v", sub.lastRequest)
				}
			}
		})
	}
}

func TestCashCountAnomalyCategoryAdminService_UpdateDisableEnable(t *testing.T) {
	active := &db.GetCashCountAnomalyCategoryAdminByIDRow{ID: 5, Code: "AB", Name: "Old", IsActive: true}
	disabled := &db.GetCashCountAnomalyCategoryAdminByIDRow{ID: 5, Code: "AB", Name: "Old", IsActive: false}
	ctx := context.Background()

	t.Run("update passes Before", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewCashCountAnomalyCategoryAdminService(&fakeAnomalyRepo{getByID: active}, sub)
		if _, err := svc.Update(ctx, 7, 5, CashCountAnomalyCategoryUpdatePayload{Name: "New"}, "ip"); err != nil {
			t.Fatalf("Update: %v", err)
		}
		if sub.lastRequest.Op != "update" || sub.lastRequest.Before != active || *sub.lastRequest.EntityID != 5 {
			t.Errorf("unexpected submit: %+v", sub.lastRequest)
		}
	})
	t.Run("update disabled is not found", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewCashCountAnomalyCategoryAdminService(&fakeAnomalyRepo{getByID: disabled}, sub)
		if _, err := svc.Update(ctx, 7, 5, CashCountAnomalyCategoryUpdatePayload{Name: "New"}, "ip"); !errors.Is(err, ErrCashCountAnomalyCategoryNotFound) || sub.submitCalled {
			t.Fatalf("want not found, got %v", err)
		}
	})
	t.Run("unknown id is not found", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewCashCountAnomalyCategoryAdminService(&fakeAnomalyRepo{}, sub)
		if _, err := svc.Disable(ctx, 7, 5, "ip"); !errors.Is(err, ErrCashCountAnomalyCategoryNotFound) || sub.submitCalled {
			t.Fatalf("want not found, got %v", err)
		}
	})
	t.Run("disable and enable pass Before", func(t *testing.T) {
		for _, op := range []string{"disable", "enable"} {
			sub := &fakeVendorBranchSubmitter{}
			svc := NewCashCountAnomalyCategoryAdminService(&fakeAnomalyRepo{getByID: disabled}, sub)
			var err error
			if op == "disable" {
				_, err = svc.Disable(ctx, 7, 5, "ip")
			} else {
				_, err = svc.Enable(ctx, 7, 5, "ip")
			}
			if err != nil || sub.lastRequest.Op != op || sub.lastRequest.Before != disabled {
				t.Errorf("%s: err=%v req=%+v", op, err, sub.lastRequest)
			}
		}
	})
}
