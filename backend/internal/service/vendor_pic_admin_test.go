package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

type fakeVendorPicAdminRepo struct {
	getByID        *db.GetVendorPicAdminByIDRow
	recipients     int64
	branchVendorID *int64
}

func (f *fakeVendorPicAdminRepo) List(context.Context, db.ListVendorPicsAdminParams) ([]db.ListVendorPicsAdminRow, error) {
	return nil, nil
}
func (f *fakeVendorPicAdminRepo) Count(context.Context, db.CountVendorPicsAdminParams) (int64, error) {
	return 0, nil
}
func (f *fakeVendorPicAdminRepo) GetByID(context.Context, int64) (*db.GetVendorPicAdminByIDRow, error) {
	return f.getByID, nil
}
func (f *fakeVendorPicAdminRepo) CountNotificationRecipients(context.Context, int64) (int64, error) {
	return f.recipients, nil
}
func (f *fakeVendorPicAdminRepo) BranchVendorID(context.Context, int64) (*int64, error) {
	return f.branchVendorID, nil
}

func TestVendorPicAdminService_Validate(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(*VendorPicUpdatePayload)
		field string // "" = valid
	}{
		{"valid", func(p *VendorPicUpdatePayload) {}, ""},
		{"blank name", func(p *VendorPicUpdatePayload) { p.Name = "  " }, "name"},
		{"bad email", func(p *VendorPicUpdatePayload) { p.Email = sp("not-an-email") }, "email"},
		{"display-name email rejected", func(p *VendorPicUpdatePayload) { p.Email = sp("Bob <bob@example.com>") }, "email"},
		{"blank email becomes NULL", func(p *VendorPicUpdatePayload) { p.Email = sp("  ") }, ""},
		{"phone with separators ok", func(p *VendorPicUpdatePayload) { p.Phone = sp("+62 (21) 555-1234") }, ""},
		{"phone letters", func(p *VendorPicUpdatePayload) { p.Phone = sp("08123abc") }, "phone"},
		{"phone too short", func(p *VendorPicUpdatePayload) { p.Phone = sp("12345") }, "phone"},
		{"phone too long", func(p *VendorPicUpdatePayload) { p.Phone = sp("1234567890123456") }, "phone"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := VendorPicUpdatePayload{Name: "Budi", Email: sp("budi@example.com"), Phone: sp("081234567890")}
			tc.mut(&p)
			err := NewVendorPicAdminService(&fakeVendorPicAdminRepo{}, &fakeVendorBranchSubmitter{}).validate(context.Background(), 3, &p)
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
}

func TestVendorPicAdminService_Create(t *testing.T) {
	owner, other, branch := int64(3), int64(4), int64(9)

	t.Run("happy path submits vendor_pic create with vendor_id from URL", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorPicAdminService(&fakeVendorPicAdminRepo{branchVendorID: &owner}, sub)
		req := VendorPicUpdatePayload{VendorBranchID: &branch, Name: " Budi "}
		if _, err := svc.Create(context.Background(), 7, 3, req, "ip"); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if sub.lastRequest.EntityType != "vendor_pic" || sub.lastRequest.Op != "create" {
			t.Errorf("unexpected submit: %+v", sub.lastRequest)
		}
		if p := sub.lastRequest.Payload.(VendorPicPayload); p.VendorID != 3 || p.Name != "Budi" {
			t.Errorf("payload not normalized: %+v", p)
		}
	})

	t.Run("branch of another vendor rejected", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorPicAdminService(&fakeVendorPicAdminRepo{branchVendorID: &other}, sub)
		_, err := svc.Create(context.Background(), 7, 3, VendorPicUpdatePayload{VendorBranchID: &branch, Name: "Budi"}, "ip")
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != "vendor_branch_id" || sub.submitCalled {
			t.Fatalf("want vendor_branch_id ValidationError and no submit, got %v", err)
		}
	})

	t.Run("unknown branch rejected", func(t *testing.T) {
		svc := NewVendorPicAdminService(&fakeVendorPicAdminRepo{}, &fakeVendorBranchSubmitter{})
		_, err := svc.Create(context.Background(), 7, 3, VendorPicUpdatePayload{VendorBranchID: &branch, Name: "Budi"}, "ip")
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != "vendor_branch_id" {
			t.Fatalf("want vendor_branch_id ValidationError, got %v", err)
		}
	})
}

func TestVendorPicAdminService_UpdateAndToggle_Scoping(t *testing.T) {
	req := VendorPicUpdatePayload{Name: "Budi"}
	mine := &db.GetVendorPicAdminByIDRow{ID: 1, VendorID: 3}
	foreign := &db.GetVendorPicAdminByIDRow{ID: 1, VendorID: 4}
	disabled := &db.GetVendorPicAdminByIDRow{ID: 1, VendorID: 3, DeletedAt: pgtype.Timestamptz{Valid: true}}

	cases := []struct {
		name string
		row  *db.GetVendorPicAdminByIDRow
		call func(*VendorPicAdminService) error
		want error
	}{
		{"update missing", nil, func(s *VendorPicAdminService) error {
			_, e := s.Update(context.Background(), 7, 3, 1, req, "ip")
			return e
		}, ErrVendorPicNotFound},
		{"update foreign vendor", foreign, func(s *VendorPicAdminService) error {
			_, e := s.Update(context.Background(), 7, 3, 1, req, "ip")
			return e
		}, ErrVendorPicNotFound},
		{"update soft-disabled", disabled, func(s *VendorPicAdminService) error {
			_, e := s.Update(context.Background(), 7, 3, 1, req, "ip")
			return e
		}, ErrVendorPicNotFound},
		{"disable foreign vendor", foreign, func(s *VendorPicAdminService) error { _, e := s.Disable(context.Background(), 7, 3, 1, "ip"); return e }, ErrVendorPicNotFound},
		{"enable soft-disabled ok", disabled, func(s *VendorPicAdminService) error { _, e := s.Enable(context.Background(), 7, 3, 1, "ip"); return e }, nil},
		{"update own ok", mine, func(s *VendorPicAdminService) error {
			_, e := s.Update(context.Background(), 7, 3, 1, req, "ip")
			return e
		}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewVendorPicAdminService(&fakeVendorPicAdminRepo{getByID: tc.row}, &fakeVendorBranchSubmitter{})
			if err := tc.call(svc); !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}

func TestVendorPicAdminService_Warnings(t *testing.T) {
	none, _ := NewVendorPicAdminService(&fakeVendorPicAdminRepo{recipients: 0}, nil).Warnings(context.Background(), 3)
	if len(none) != 1 || none[0] != NoNotificationRecipientWarning {
		t.Errorf("want no-recipient warning, got %v", none)
	}
	some, _ := NewVendorPicAdminService(&fakeVendorPicAdminRepo{recipients: 2}, nil).Warnings(context.Background(), 3)
	if len(some) != 0 {
		t.Errorf("want no warnings, got %v", some)
	}
}
