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
	findByCode     *int64
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
func (f *fakeVendorPackageAdminRepo) FindByBranchCode(context.Context, int64, string) (*int64, error) {
	return f.findByCode, nil
}
func (f *fakeVendorPackageAdminRepo) BranchVendorID(context.Context, int64) (*int64, error) {
	return f.branchVendorID, nil
}

func TestValidatePackageFields(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(*VendorPackageUpdatePayload)
		field string // "" = valid
	}{
		{"valid", func(p *VendorPackageUpdatePayload) {}, ""},
		{"zero price ok", func(p *VendorPackageUpdatePayload) { p.Price = "0" }, ""},
		{"integer price ok", func(p *VendorPackageUpdatePayload) { p.Price = "150000" }, ""},
		{"blank class", func(p *VendorPackageUpdatePayload) { p.PriorityClass = " " }, "priority_class"},
		{"blank price", func(p *VendorPackageUpdatePayload) { p.Price = "" }, "price"},
		{"negative", func(p *VendorPackageUpdatePayload) { p.Price = "-1" }, "price"},
		{"three decimals rejected, not rounded", func(p *VendorPackageUpdatePayload) { p.Price = "1.234" }, "price"},
		{"non-numeric", func(p *VendorPackageUpdatePayload) { p.Price = "abc" }, "price"},
		{"fraction syntax", func(p *VendorPackageUpdatePayload) { p.Price = "1/2" }, "price"},
		{"exponent syntax", func(p *VendorPackageUpdatePayload) { p.Price = "1e3" }, "price"},
		{"19 integer digits too big", func(p *VendorPackageUpdatePayload) { p.Price = "1000000000000000000" }, "price"},
		{"18 integer digits ok", func(p *VendorPackageUpdatePayload) { p.Price = "999999999999999999.99" }, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := VendorPackageUpdatePayload{PriorityClass: "ALL", Price: "100000.00"}
			tc.mut(&p)
			err := validatePackageFields(&p)
			if tc.field == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Field != tc.field {
				t.Fatalf("want ValidationError on %s, got %v", tc.field, err)
			}
		})
	}

	t.Run("inputs are trimmed", func(t *testing.T) {
		p := VendorPackageUpdatePayload{PriorityClass: " ALL ", Price: " 10.5 "}
		if err := validatePackageFields(&p); err != nil || p.PriorityClass != "ALL" || p.Price != "10.5" {
			t.Errorf("want trimmed ALL/10.5 with no error, got %+v err=%v", p, err)
		}
	})
}

func TestVendorPackageAdminService_Create(t *testing.T) {
	owner, other := int64(3), int64(4)
	req := VendorPackagePayload{VendorBranchID: 9, Code: " PKG1 ", VendorPackageUpdatePayload: VendorPackageUpdatePayload{PriorityClass: "ALL", Price: "100.00"}}

	t.Run("happy path submits vendor_package create with trimmed code", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorPackageAdminService(&fakeVendorPackageAdminRepo{branchVendorID: &owner}, sub)
		if _, err := svc.Create(context.Background(), 7, 3, req, "ip"); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if sub.lastRequest.EntityType != "vendor_package" || sub.lastRequest.Op != "create" {
			t.Errorf("unexpected submit: %+v", sub.lastRequest)
		}
		if p := sub.lastRequest.Payload.(VendorPackagePayload); p.Code != "PKG1" {
			t.Errorf("code not trimmed: %q", p.Code)
		}
	})

	t.Run("branch of another vendor rejected", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorPackageAdminService(&fakeVendorPackageAdminRepo{branchVendorID: &other}, sub)
		_, err := svc.Create(context.Background(), 7, 3, req, "ip")
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != "vendor_branch_id" || sub.submitCalled {
			t.Fatalf("want vendor_branch_id ValidationError and no submit, got %v", err)
		}
	})

	t.Run("unknown branch rejected", func(t *testing.T) {
		svc := NewVendorPackageAdminService(&fakeVendorPackageAdminRepo{}, &fakeVendorBranchSubmitter{})
		_, err := svc.Create(context.Background(), 7, 3, req, "ip")
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != "vendor_branch_id" {
			t.Fatalf("want vendor_branch_id ValidationError, got %v", err)
		}
	})

	t.Run("duplicate code in branch conflicts", func(t *testing.T) {
		existing := int64(1)
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorPackageAdminService(&fakeVendorPackageAdminRepo{branchVendorID: &owner, findByCode: &existing}, sub)
		_, err := svc.Create(context.Background(), 7, 3, req, "ip")
		if !errors.Is(err, ErrVendorPackageCodeConflict) || sub.submitCalled {
			t.Fatalf("want ErrVendorPackageCodeConflict and no submit, got %v", err)
		}
	})
}

func TestVendorPackageAdminService_UpdateAndToggle_Scoping(t *testing.T) {
	req := VendorPackageUpdatePayload{PriorityClass: "ALL", Price: "1.00"}
	mine := &db.GetVendorPackageAdminByIDRow{ID: 1, VendorID: 3}
	foreign := &db.GetVendorPackageAdminByIDRow{ID: 1, VendorID: 4}
	disabled := &db.GetVendorPackageAdminByIDRow{ID: 1, VendorID: 3, DeletedAt: pgtype.Timestamptz{Valid: true}}
	ctx := context.Background()

	update := func(s *VendorPackageAdminService) error { _, e := s.Update(ctx, 7, 3, 1, req, "ip"); return e }
	disable := func(s *VendorPackageAdminService) error { _, e := s.Disable(ctx, 7, 3, 1, "ip"); return e }
	enable := func(s *VendorPackageAdminService) error { _, e := s.Enable(ctx, 7, 3, 1, "ip"); return e }

	cases := []struct {
		name string
		row  *db.GetVendorPackageAdminByIDRow
		call func(*VendorPackageAdminService) error
		want error
	}{
		{"update missing", nil, update, ErrVendorPackageNotFound},
		{"update foreign vendor", foreign, update, ErrVendorPackageNotFound},
		{"update soft-disabled", disabled, update, ErrVendorPackageNotFound},
		{"disable foreign vendor", foreign, disable, ErrVendorPackageNotFound},
		{"enable soft-disabled ok", disabled, enable, nil},
		{"update own ok", mine, update, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewVendorPackageAdminService(&fakeVendorPackageAdminRepo{getByID: tc.row}, &fakeVendorBranchSubmitter{})
			if err := tc.call(svc); !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}
