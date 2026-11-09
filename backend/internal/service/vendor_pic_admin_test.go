package service

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

type fakeVendorPicAdminRepo struct {
	getByID        *db.GetVendorPicAdminByIDRow
	recipients     int64
	branchVendorID *int64
	otherDirector  bool
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

func (f *fakeVendorPicAdminRepo) OtherActiveDirectorExists(context.Context, int64, *int64) (bool, error) {
	return f.otherDirector, nil
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
		{"roles normalized", func(p *VendorPicUpdatePayload) { p.Roles = &[]string{" vault", "replenish"} }, ""},
		{"empty roles ok", func(p *VendorPicUpdatePayload) { p.Roles = &[]string{} }, ""},
		{"unknown role", func(p *VendorPicUpdatePayload) { p.Roles = &[]string{"courier"} }, "roles"},
		{"duplicate role", func(p *VendorPicUpdatePayload) { p.Roles = &[]string{"vault", "vault "} }, "roles"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := VendorPicUpdatePayload{Name: "Budi", Email: sp("budi@example.com"), Phone: sp("081234567890")}
			tc.mut(&p)
			err := NewVendorPicAdminService(&fakeVendorPicAdminRepo{}, &fakeVendorBranchSubmitter{}).validate(context.Background(), 3, &p, nil)
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

func TestNormalizePicRoles(t *testing.T) {
	cases := []struct {
		in      []string
		want    []string
		wantErr bool
	}{
		{[]string{}, []string{}, false},
		{[]string{" vault", "cashcount", "replenish "}, []string{"cashcount", "replenish", "vault"}, false},
		{[]string{"vault", "bm"}, []string{"bm", "vault"}, false}, // vendor-cis-limit FR3.1
		{[]string{"Vault"}, nil, true}, // codes are case-sensitive, as in the DB CHECK
		{[]string{""}, nil, true},
		{[]string{"replenish", " replenish"}, nil, true},
	}
	for _, tc := range cases {
		got, err := normalizePicRoles(tc.in)
		if (err != nil) != tc.wantErr || !slices.Equal(got, tc.want) {
			t.Errorf("normalizePicRoles(%q) = %q, %v; want %q, err=%v", tc.in, got, err, tc.want, tc.wantErr)
		}
	}
	// bm is never a default (vendor-cis-limit FR3.1).
	if got := DefaultPicRoles(); !slices.Equal(got, []string{"cashcount", "replenish", "vault"}) {
		t.Errorf("DefaultPicRoles() = %q", got)
	}
}

func bp(b bool) *bool { return &b }

// vendor-pks Req 1.2-1.5, 1.7 (Property 9): director payload rules.
func TestVendorPicAdminService_DirectorRules(t *testing.T) {
	branch := int64(9)
	cases := []struct {
		name      string
		repo      *fakeVendorPicAdminRepo
		before    *db.GetVendorPicAdminByIDRow
		mut       func(*VendorPicUpdatePayload)
		field     string // ValidationError field; "" = valid
		wantErr   error
		wantRoles bool // director => Roles forced to empty non-nil
	}{
		{name: "director valid", repo: &fakeVendorPicAdminRepo{}, mut: func(p *VendorPicUpdatePayload) { p.IsDirector = bp(true) }, wantRoles: true},
		{name: "director with branch", repo: &fakeVendorPicAdminRepo{}, mut: func(p *VendorPicUpdatePayload) { p.IsDirector = bp(true); p.VendorBranchID = &branch }, field: "vendor_branch_id"},
		{name: "director notification recipient", repo: &fakeVendorPicAdminRepo{}, mut: func(p *VendorPicUpdatePayload) { p.IsDirector = bp(true); p.IsNotificationRecipient = true }, field: "is_notification_recipient"},
		{name: "director with roles", repo: &fakeVendorPicAdminRepo{}, mut: func(p *VendorPicUpdatePayload) { p.IsDirector = bp(true); p.Roles = &[]string{"vault"} }, field: "roles"},
		{name: "director with empty roles ok", repo: &fakeVendorPicAdminRepo{}, mut: func(p *VendorPicUpdatePayload) { p.IsDirector = bp(true); p.Roles = &[]string{} }, wantRoles: true},
		{name: "second active director", repo: &fakeVendorPicAdminRepo{otherDirector: true}, mut: func(p *VendorPicUpdatePayload) { p.IsDirector = bp(true) }, wantErr: ErrVendorDirectorExists},
		{name: "non-director ignores director repo", repo: &fakeVendorPicAdminRepo{otherDirector: true}, mut: func(p *VendorPicUpdatePayload) { p.IsDirector = bp(false); p.VendorBranchID = nil }},
		{name: "update inherits director flag (branch rejected)", repo: &fakeVendorPicAdminRepo{},
			before: &db.GetVendorPicAdminByIDRow{ID: 1, VendorID: 3, IsDirector: true},
			mut:    func(p *VendorPicUpdatePayload) { p.VendorBranchID = &branch }, field: "vendor_branch_id"},
		{name: "update inherited director valid", repo: &fakeVendorPicAdminRepo{},
			before: &db.GetVendorPicAdminByIDRow{ID: 1, VendorID: 3, IsDirector: true}, mut: func(p *VendorPicUpdatePayload) {}, wantRoles: true},
		{name: "update releases director flag keeps roles nil", repo: &fakeVendorPicAdminRepo{otherDirector: true},
			before: &db.GetVendorPicAdminByIDRow{ID: 1, VendorID: 3, IsDirector: true},
			mut:    func(p *VendorPicUpdatePayload) { p.IsDirector = bp(false) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := VendorPicUpdatePayload{Name: "Dirut"}
			tc.mut(&p)
			err := NewVendorPicAdminService(tc.repo, &fakeVendorBranchSubmitter{}).validate(context.Background(), 3, &p, tc.before)
			switch {
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("want %v, got %v", tc.wantErr, err)
				}
			case tc.field != "":
				var ve *ValidationError
				if !errors.As(err, &ve) || ve.Field != tc.field {
					t.Fatalf("want ValidationError on %s, got %v", tc.field, err)
				}
			default:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if tc.wantRoles && (p.Roles == nil || len(*p.Roles) != 0) {
					t.Errorf("director roles = %v, want empty non-nil", p.Roles)
				}
				if !tc.wantRoles && p.Roles != nil {
					t.Errorf("non-director roles = %v, want untouched nil", p.Roles)
				}
			}
		})
	}
}

// Req 1.5: re-enabling an inactive director is blocked while another is active.
func TestVendorPicAdminService_EnableDirector(t *testing.T) {
	dir := &db.GetVendorPicAdminByIDRow{ID: 1, VendorID: 3, IsDirector: true, DeletedAt: pgtype.Timestamptz{Valid: true}}
	plain := &db.GetVendorPicAdminByIDRow{ID: 2, VendorID: 3, DeletedAt: pgtype.Timestamptz{Valid: true}}
	cases := []struct {
		name  string
		row   *db.GetVendorPicAdminByIDRow
		other bool
		want  error
	}{
		{"director, other active", dir, true, ErrVendorDirectorExists},
		{"director, none other", dir, false, nil},
		{"non-director never checked", plain, true, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sub := &fakeVendorBranchSubmitter{}
			svc := NewVendorPicAdminService(&fakeVendorPicAdminRepo{getByID: tc.row, otherDirector: tc.other}, sub)
			_, err := svc.Enable(context.Background(), 7, 3, tc.row.ID, "ip")
			if !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
			if (tc.want != nil) == sub.submitCalled {
				t.Errorf("submitCalled = %v with err %v", sub.submitCalled, err)
			}
		})
	}
}

// Create passes the director flag and the forced-empty roles to the applier payload.
func TestVendorPicAdminService_CreateDirectorPayload(t *testing.T) {
	sub := &fakeVendorBranchSubmitter{}
	svc := NewVendorPicAdminService(&fakeVendorPicAdminRepo{}, sub)
	if _, err := svc.Create(context.Background(), 7, 3, VendorPicUpdatePayload{Name: "Dirut", IsDirector: bp(true)}, "ip"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	p := sub.lastRequest.Payload.(VendorPicPayload)
	if p.IsDirector == nil || !*p.IsDirector || p.Roles == nil || len(*p.Roles) != 0 || p.VendorBranchID != nil || p.IsNotificationRecipient {
		t.Errorf("director payload wrong: %+v", p)
	}
}

// vendor-cis-limit FR3.3 / S5: bm only for a PIC bound to a cabang -- also when an update
// moves a bm PIC to vendor level while keeping its roles.
func TestVendorPicAdminService_BmNeedsBranch(t *testing.T) {
	owner := int64(3)
	branch := int64(11)
	cases := []struct {
		name      string
		branchID  *int64
		roles     *[]string
		before    *db.GetVendorPicAdminByIDRow
		wantError bool
	}{
		{"create cabang PIC with bm ok", &branch, &[]string{"bm"}, nil, false},
		{"create vendor-wide PIC with bm", nil, &[]string{"bm", "vault"}, nil, true},
		{"create vendor-wide PIC without bm ok", nil, &[]string{"vault"}, nil, false},
		{"update to vendor level keeps bm roles", nil, nil, &db.GetVendorPicAdminByIDRow{ID: 5, VendorID: 3, VendorBranchID: &branch, Roles: []string{"bm"}}, true},
		{"update to vendor level dropping bm ok", nil, &[]string{"vault"}, &db.GetVendorPicAdminByIDRow{ID: 5, VendorID: 3, VendorBranchID: &branch, Roles: []string{"bm"}}, false},
		{"update cabang PIC keeps bm ok", &branch, nil, &db.GetVendorPicAdminByIDRow{ID: 5, VendorID: 3, VendorBranchID: &branch, Roles: []string{"bm"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := VendorPicUpdatePayload{Name: "Budi", VendorBranchID: tc.branchID, Roles: tc.roles}
			err := NewVendorPicAdminService(&fakeVendorPicAdminRepo{branchVendorID: &owner}, &fakeVendorBranchSubmitter{}).
				validate(context.Background(), 3, &p, tc.before)
			var ve *ValidationError
			if got := errors.As(err, &ve) && ve.Field == "roles"; got != tc.wantError {
				t.Fatalf("err = %v, want roles error = %v", err, tc.wantError)
			}
		})
	}
}
