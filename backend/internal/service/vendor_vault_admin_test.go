package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

type fakeVendorVaultAdminRepo struct {
	getByID        *db.GetVendorVaultAdminByIDRow
	findByCode     *int64
	branchVendorID *int64
}

func (f *fakeVendorVaultAdminRepo) List(context.Context, db.ListVendorVaultsAdminParams) ([]db.ListVendorVaultsAdminRow, error) {
	return nil, nil
}
func (f *fakeVendorVaultAdminRepo) Count(context.Context, db.CountVendorVaultsAdminParams) (int64, error) {
	return 0, nil
}
func (f *fakeVendorVaultAdminRepo) GetByID(context.Context, int64) (*db.GetVendorVaultAdminByIDRow, error) {
	return f.getByID, nil
}
func (f *fakeVendorVaultAdminRepo) FindByCode(context.Context, string) (*int64, error) {
	return f.findByCode, nil
}
func (f *fakeVendorVaultAdminRepo) BranchVendorID(context.Context, int64) (*int64, error) {
	return f.branchVendorID, nil
}

func sp(s string) *string { return &s }

func validVaultUpdate() VendorVaultUpdatePayload {
	return VendorVaultUpdatePayload{Category: "ATM", CurrencyCode: "idr", MinCapacityAmount: sp("100.00"), MaxCapacityAmount: sp("500.00")}
}

func TestValidateVaultFields(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(*VendorVaultUpdatePayload)
		field string // "" = valid
	}{
		{"valid, currency upper-cased", func(p *VendorVaultUpdatePayload) {}, ""},
		{"bad category", func(p *VendorVaultUpdatePayload) { p.Category = "ATM_CASH" }, "category"},
		{"missing currency", func(p *VendorVaultUpdatePayload) { p.CurrencyCode = "" }, "currency_code"},
		{"min > max", func(p *VendorVaultUpdatePayload) { p.MinCapacityAmount = sp("600") }, "min_capacity_amount"},
		{"min == max ok", func(p *VendorVaultUpdatePayload) { p.MinCapacityAmount = sp("500") }, ""},
		{"negative max", func(p *VendorVaultUpdatePayload) { p.MaxCapacityAmount = sp("-1") }, "max_capacity_amount"},
		{"non-numeric", func(p *VendorVaultUpdatePayload) { p.MaxCapacityAmount = sp("abc") }, "max_capacity_amount"},
		{"fraction syntax rejected", func(p *VendorVaultUpdatePayload) { p.MaxCapacityAmount = sp("1/2") }, "max_capacity_amount"},
		{"lat out of range", func(p *VendorVaultUpdatePayload) { p.Latitude = sp("90.5") }, "latitude"},
		{"lon out of range", func(p *VendorVaultUpdatePayload) { p.Longitude = sp("-180.1") }, "longitude"},
		{"coords at bounds ok", func(p *VendorVaultUpdatePayload) { p.Latitude = sp("-90"); p.Longitude = sp("180") }, ""},
		{"nil capacities ok", func(p *VendorVaultUpdatePayload) { p.MinCapacityAmount, p.MaxCapacityAmount = nil, nil }, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := validVaultUpdate()
			tc.mut(&p)
			err := validateVaultFields(&p)
			if tc.field == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if p.CurrencyCode != "IDR" {
					t.Errorf("currency not normalized: %q", p.CurrencyCode)
				}
				return
			}
			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Field != tc.field {
				t.Fatalf("want ValidationError on %s, got %v", tc.field, err)
			}
		})
	}
}

func TestVendorVaultAdminService_Create(t *testing.T) {
	owner := int64(3)
	other := int64(4)
	req := VendorVaultPayload{VendorBranchID: 9, VaultCode: " V1 ", VendorVaultUpdatePayload: validVaultUpdate()}

	t.Run("happy path submits vendor_vault create with trimmed code", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorVaultAdminService(&fakeVendorVaultAdminRepo{branchVendorID: &owner}, sub)
		if _, err := svc.Create(context.Background(), 7, 3, req, "ip"); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if sub.lastRequest.EntityType != "vendor_vault" || sub.lastRequest.Op != "create" {
			t.Errorf("unexpected submit: %+v", sub.lastRequest)
		}
		if p := sub.lastRequest.Payload.(VendorVaultPayload); p.VaultCode != "V1" {
			t.Errorf("code not trimmed: %q", p.VaultCode)
		}
	})

	t.Run("branch of another vendor rejected", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorVaultAdminService(&fakeVendorVaultAdminRepo{branchVendorID: &other}, sub)
		_, err := svc.Create(context.Background(), 7, 3, req, "ip")
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != "vendor_branch_id" || sub.submitCalled {
			t.Fatalf("want vendor_branch_id ValidationError and no submit, got %v", err)
		}
	})

	t.Run("unknown branch rejected", func(t *testing.T) {
		svc := NewVendorVaultAdminService(&fakeVendorVaultAdminRepo{}, &fakeVendorBranchSubmitter{})
		_, err := svc.Create(context.Background(), 7, 3, req, "ip")
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != "vendor_branch_id" {
			t.Fatalf("want vendor_branch_id ValidationError, got %v", err)
		}
	})

	t.Run("duplicate code conflicts", func(t *testing.T) {
		existing := int64(1)
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorVaultAdminService(&fakeVendorVaultAdminRepo{branchVendorID: &owner, findByCode: &existing}, sub)
		_, err := svc.Create(context.Background(), 7, 3, req, "ip")
		if !errors.Is(err, ErrVendorVaultCodeConflict) || sub.submitCalled {
			t.Fatalf("want ErrVendorVaultCodeConflict and no submit, got %v", err)
		}
	})
}

func TestVendorVaultAdminService_UpdateAndToggle_NotFound(t *testing.T) {
	svc := NewVendorVaultAdminService(&fakeVendorVaultAdminRepo{}, &fakeVendorBranchSubmitter{})
	if _, err := svc.Update(context.Background(), 7, 1, validVaultUpdate(), "ip"); !errors.Is(err, ErrVendorVaultNotFound) {
		t.Errorf("Update: want ErrVendorVaultNotFound, got %v", err)
	}
	if _, err := svc.Disable(context.Background(), 7, 1, "ip"); !errors.Is(err, ErrVendorVaultNotFound) {
		t.Errorf("Disable: want ErrVendorVaultNotFound, got %v", err)
	}
}
