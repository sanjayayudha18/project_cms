package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

type fakeVendorPackageAdminRepo struct {
	getByID        *db.GetVendorPackageAdminByIDRow
	branchVendorID *int64
}

func (f *fakeVendorPackageAdminRepo) List(context.Context, db.ListVendorPackagesAdminParams) ([]db.ListVendorPackagesAdminRow, error) {
	return nil, nil
}
func (f *fakeVendorPackageAdminRepo) Count(context.Context, db.CountVendorPackagesAdminParams) (int64, error) {
	return 0, nil
}
func (f *fakeVendorPackageAdminRepo) GetByID(context.Context, int64) (*db.GetVendorPackageAdminByIDRow, error) {
	return f.getByID, nil
}
func (f *fakeVendorPackageAdminRepo) BranchVendorID(context.Context, int64) (*int64, error) {
	return f.branchVendorID, nil
}

func baseVendorPackageCreatePayload(branchID int64) VendorPackageCreatePayload {
	return VendorPackageCreatePayload{
		VendorBranchID: branchID, PackageCode: " PKG1 ", MachineGroup: "ATM", PriceClass: "REGULAR",
		EffectiveStartDate: "2026-01-01",
	}
}

func TestVendorPackageAdminService_Create(t *testing.T) {
	owner, other := int64(3), int64(4)
	branchID := int64(9)

	t.Run("happy path submits vendor_package create with trimmed code", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorPackageAdminService(&fakeVendorPackageAdminRepo{branchVendorID: &owner}, sub)
		if _, err := svc.Create(context.Background(), 7, 3, baseVendorPackageCreatePayload(branchID), "ip"); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if sub.lastRequest.EntityType != "vendor_package" || sub.lastRequest.Op != "create" {
			t.Errorf("unexpected submit: %+v", sub.lastRequest)
		}
		if p := sub.lastRequest.Payload.(VendorPackageCreatePayload); p.PackageCode != "PKG1" {
			t.Errorf("code not trimmed: %q", p.PackageCode)
		}
	})

	t.Run("branch of another vendor rejected", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorPackageAdminService(&fakeVendorPackageAdminRepo{branchVendorID: &other}, sub)
		_, err := svc.Create(context.Background(), 7, 3, baseVendorPackageCreatePayload(branchID), "ip")
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != "vendor_branch_id" || sub.submitCalled {
			t.Fatalf("want vendor_branch_id ValidationError and no submit, got %v", err)
		}
	})

	t.Run("unknown branch rejected", func(t *testing.T) {
		svc := NewVendorPackageAdminService(&fakeVendorPackageAdminRepo{}, &fakeVendorBranchSubmitter{})
		_, err := svc.Create(context.Background(), 7, 3, baseVendorPackageCreatePayload(branchID), "ip")
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != "vendor_branch_id" {
			t.Fatalf("want vendor_branch_id ValidationError, got %v", err)
		}
	})

	t.Run("missing vendor_branch_id rejected", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorPackageAdminService(&fakeVendorPackageAdminRepo{branchVendorID: &owner}, sub)
		_, err := svc.Create(context.Background(), 7, 3, baseVendorPackageCreatePayload(0), "ip")
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != "vendor_branch_id" || sub.submitCalled {
			t.Fatalf("want vendor_branch_id ValidationError and no submit, got %v", err)
		}
	})

	t.Run("bad machine_group rejected", func(t *testing.T) {
		bad := baseVendorPackageCreatePayload(branchID)
		bad.MachineGroup = "CDM"
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorPackageAdminService(&fakeVendorPackageAdminRepo{branchVendorID: &owner}, sub)
		_, err := svc.Create(context.Background(), 7, 3, bad, "ip")
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != "machine_group" || sub.submitCalled {
			t.Fatalf("want machine_group ValidationError and no submit, got %v", err)
		}
	})

	t.Run("tier_max below tier_min rejected", func(t *testing.T) {
		bad := baseVendorPackageCreatePayload(branchID)
		bad.TierMin, bad.TierMax = 10, int64ptr(5)
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorPackageAdminService(&fakeVendorPackageAdminRepo{branchVendorID: &owner}, sub)
		_, err := svc.Create(context.Background(), 7, 3, bad, "ip")
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != "tier_max" || sub.submitCalled {
			t.Fatalf("want tier_max ValidationError and no submit, got %v", err)
		}
	})

	t.Run("invalid base_price rejected", func(t *testing.T) {
		bad := baseVendorPackageCreatePayload(branchID)
		badPrice := "not-a-number"
		bad.BasePrice = &badPrice
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorPackageAdminService(&fakeVendorPackageAdminRepo{branchVendorID: &owner}, sub)
		_, err := svc.Create(context.Background(), 7, 3, bad, "ip")
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != "base_price" || sub.submitCalled {
			t.Fatalf("want base_price ValidationError and no submit, got %v", err)
		}
	})
}

func int64ptr(v int64) *int64 { return &v }

func TestVendorPackageAdminService_Update(t *testing.T) {
	owned := &db.GetVendorPackageAdminByIDRow{
		ID: 1, VendorID: 3,
		EffectiveStartDate: pgtype.Date{Time: mustParseDate(t, "2026-01-01"), Valid: true},
	}
	foreign := &db.GetVendorPackageAdminByIDRow{ID: 1, VendorID: 4}
	ended := &db.GetVendorPackageAdminByIDRow{
		ID: 1, VendorID: 3,
		EffectiveStartDate: pgtype.Date{Time: mustParseDate(t, "2026-01-01"), Valid: true},
		EffectiveEndDate:   pgtype.Date{Time: mustParseDate(t, "2026-02-01"), Valid: true},
	}

	t.Run("happy path submits vendor_package update", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorPackageAdminService(&fakeVendorPackageAdminRepo{getByID: owned}, sub)
		end := "2027-12-31"
		_, err := svc.Update(context.Background(), 7, 3, 1, VendorPackageContentPayload{EffectiveEndDate: &end}, "ip")
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if sub.lastRequest.EntityType != "vendor_package" || sub.lastRequest.Op != "update" {
			t.Errorf("unexpected submit: %+v", sub.lastRequest)
		}
	})

	t.Run("foreign vendor rejected", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorPackageAdminService(&fakeVendorPackageAdminRepo{getByID: foreign}, sub)
		_, err := svc.Update(context.Background(), 7, 3, 1, VendorPackageContentPayload{}, "ip")
		if !errors.Is(err, ErrVendorPackageNotFound) || sub.submitCalled {
			t.Fatalf("want ErrVendorPackageNotFound and no submit, got %v", err)
		}
	})

	t.Run("already-ended row rejected", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorPackageAdminService(&fakeVendorPackageAdminRepo{getByID: ended}, sub)
		_, err := svc.Update(context.Background(), 7, 3, 1, VendorPackageContentPayload{}, "ip")
		if !errors.Is(err, ErrVendorPackageNotFound) || sub.submitCalled {
			t.Fatalf("want ErrVendorPackageNotFound and no submit, got %v", err)
		}
	})

	t.Run("end date before start rejected", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorPackageAdminService(&fakeVendorPackageAdminRepo{getByID: owned}, sub)
		end := "2025-01-01"
		_, err := svc.Update(context.Background(), 7, 3, 1, VendorPackageContentPayload{EffectiveEndDate: &end}, "ip")
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != "effective_end_date" || sub.submitCalled {
			t.Fatalf("want effective_end_date ValidationError and no submit, got %v", err)
		}
	})
}

func TestVendorPackageAdminService_Disable_Scoping(t *testing.T) {
	foreign := &db.GetVendorPackageAdminByIDRow{ID: 1, VendorID: 4}
	ctx := context.Background()

	cases := []struct {
		name string
		row  *db.GetVendorPackageAdminByIDRow
		want error
	}{
		{"missing", nil, ErrVendorPackageNotFound},
		{"foreign vendor", foreign, ErrVendorPackageNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewVendorPackageAdminService(&fakeVendorPackageAdminRepo{getByID: tc.row}, &fakeVendorBranchSubmitter{})
			_, err := svc.Disable(ctx, 7, 3, 1, "ip")
			if !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}
