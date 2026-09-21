package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

type fakeATMAssignmentAdminRepo struct {
	getByID     *db.GetATMAssignmentAdminByIDRow
	overlap     bool
	atmMissing  bool
	pkgInactive bool
	// lastOverlap records the period HasOverlap was asked about.
	lastExclude int64
	lastStart   time.Time
	lastEnd     *time.Time
}

func (f *fakeATMAssignmentAdminRepo) List(context.Context, db.ListATMAssignmentsAdminParams) ([]db.ListATMAssignmentsAdminRow, error) {
	return nil, nil
}
func (f *fakeATMAssignmentAdminRepo) Count(context.Context, db.CountATMAssignmentsAdminParams) (int64, error) {
	return 0, nil
}
func (f *fakeATMAssignmentAdminRepo) GetByID(context.Context, int64) (*db.GetATMAssignmentAdminByIDRow, error) {
	return f.getByID, nil
}
func (f *fakeATMAssignmentAdminRepo) HasOverlap(_ context.Context, _, excludeID int64, start time.Time, end *time.Time) (bool, error) {
	f.lastExclude, f.lastStart, f.lastEnd = excludeID, start, end
	return f.overlap, nil
}
func (f *fakeATMAssignmentAdminRepo) ATMActive(context.Context, int64) (bool, error) {
	return !f.atmMissing, nil
}
func (f *fakeATMAssignmentAdminRepo) PackageActive(context.Context, int64) (bool, error) {
	return !f.pkgInactive, nil
}

func date(s string) pgtype.Date {
	t, _ := time.Parse(assignmentDateLayout, s)
	return pgtype.Date{Time: t, Valid: true}
}

func validAssignment() ATMAssignmentUpdatePayload {
	return ATMAssignmentUpdatePayload{VendorPackageID: 5, EffectiveStartDate: "2026-03-01", EffectiveEndDate: sp("2026-03-31")}
}

func TestATMAssignmentAdminService_Validate(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(*ATMAssignmentUpdatePayload)
		repo  fakeATMAssignmentAdminRepo
		field string // ValidationError field; "" = no ValidationError
		want  error  // sentinel, when not a ValidationError
	}{
		{name: "valid closed period", mut: func(p *ATMAssignmentUpdatePayload) {}},
		{name: "valid open-ended", mut: func(p *ATMAssignmentUpdatePayload) { p.EffectiveEndDate = nil }},
		{name: "blank end date = open-ended", mut: func(p *ATMAssignmentUpdatePayload) { p.EffectiveEndDate = sp("  ") }},
		{name: "single-day period ok", mut: func(p *ATMAssignmentUpdatePayload) { p.EffectiveEndDate = sp("2026-03-01") }},
		{name: "missing package", mut: func(p *ATMAssignmentUpdatePayload) { p.VendorPackageID = 0 }, field: "vendor_package_id"},
		{name: "inactive package", mut: func(p *ATMAssignmentUpdatePayload) {}, repo: fakeATMAssignmentAdminRepo{pkgInactive: true}, field: "vendor_package_id"},
		{name: "bad start format", mut: func(p *ATMAssignmentUpdatePayload) { p.EffectiveStartDate = "01/03/2026" }, field: "effective_start_date"},
		{name: "impossible date", mut: func(p *ATMAssignmentUpdatePayload) { p.EffectiveStartDate = "2026-02-30" }, field: "effective_start_date"},
		{name: "blank start", mut: func(p *ATMAssignmentUpdatePayload) { p.EffectiveStartDate = "" }, field: "effective_start_date"},
		{name: "bad end format", mut: func(p *ATMAssignmentUpdatePayload) { p.EffectiveEndDate = sp("soon") }, field: "effective_end_date"},
		{name: "end before start", mut: func(p *ATMAssignmentUpdatePayload) { p.EffectiveEndDate = sp("2026-02-28") }, field: "effective_end_date"},
		{name: "overlap with active assignment", mut: func(p *ATMAssignmentUpdatePayload) {}, repo: fakeATMAssignmentAdminRepo{overlap: true}, want: ErrATMAssignmentOverlap},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := validAssignment()
			tc.mut(&p)
			repo := tc.repo
			err := NewATMAssignmentAdminService(&repo, &fakeVendorBranchSubmitter{}).validate(context.Background(), 1, 0, &p)
			switch {
			case tc.field != "":
				var ve *ValidationError
				if !errors.As(err, &ve) || ve.Field != tc.field {
					t.Fatalf("want ValidationError on %s, got %v", tc.field, err)
				}
			case tc.want != nil:
				if !errors.Is(err, tc.want) {
					t.Fatalf("want %v, got %v", tc.want, err)
				}
			case err != nil:
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestATMAssignmentAdminService_Validate_PassesOpenEndAsNil(t *testing.T) {
	repo := &fakeATMAssignmentAdminRepo{}
	p := validAssignment()
	p.EffectiveEndDate = nil
	if err := NewATMAssignmentAdminService(repo, nil).validate(context.Background(), 1, 42, &p); err != nil {
		t.Fatal(err)
	}
	if repo.lastEnd != nil || repo.lastExclude != 42 || repo.lastStart.Format(assignmentDateLayout) != "2026-03-01" {
		t.Errorf("overlap asked with end=%v exclude=%d start=%v", repo.lastEnd, repo.lastExclude, repo.lastStart)
	}
}

func TestATMAssignmentAdminService_Create(t *testing.T) {
	t.Run("happy path submits atm_assignment create with atm_id from URL", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewATMAssignmentAdminService(&fakeATMAssignmentAdminRepo{}, sub)
		if _, err := svc.Create(context.Background(), 7, 3, validAssignment(), "ip"); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if sub.lastRequest.EntityType != "atm_assignment" || sub.lastRequest.Op != "create" {
			t.Errorf("unexpected submit: %+v", sub.lastRequest)
		}
		if p := sub.lastRequest.Payload.(ATMAssignmentPayload); p.ATMID != 3 || p.VendorPackageID != 5 {
			t.Errorf("payload wrong: %+v", p)
		}
	})

	t.Run("unknown atm is not found, nothing submitted", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewATMAssignmentAdminService(&fakeATMAssignmentAdminRepo{atmMissing: true}, sub)
		if _, err := svc.Create(context.Background(), 7, 3, validAssignment(), "ip"); !errors.Is(err, ErrAssignmentATMNotFound) || sub.submitCalled {
			t.Fatalf("want ErrAssignmentATMNotFound and no submit, got %v", err)
		}
	})

	t.Run("overlap is not submitted", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewATMAssignmentAdminService(&fakeATMAssignmentAdminRepo{overlap: true}, sub)
		if _, err := svc.Create(context.Background(), 7, 3, validAssignment(), "ip"); !errors.Is(err, ErrATMAssignmentOverlap) || sub.submitCalled {
			t.Fatalf("want ErrATMAssignmentOverlap and no submit, got %v", err)
		}
	})
}

func TestATMAssignmentAdminService_UpdateAndToggle(t *testing.T) {
	ctx := context.Background()
	active := &db.GetATMAssignmentAdminByIDRow{ID: 1, AtmID: 3, IsActive: true, EffectiveStartDate: date("2026-03-01")}
	disabled := &db.GetATMAssignmentAdminByIDRow{ID: 1, AtmID: 3, IsActive: false, EffectiveStartDate: date("2026-03-01"), EffectiveEndDate: date("2026-03-31")}
	foreign := &db.GetATMAssignmentAdminByIDRow{ID: 1, AtmID: 4, IsActive: true}

	update := func(s *ATMAssignmentAdminService) error {
		_, e := s.Update(ctx, 7, 3, 1, validAssignment(), "ip")
		return e
	}
	disable := func(s *ATMAssignmentAdminService) error { _, e := s.Disable(ctx, 7, 3, 1, "ip"); return e }
	enable := func(s *ATMAssignmentAdminService) error { _, e := s.Enable(ctx, 7, 3, 1, "ip"); return e }

	cases := []struct {
		name string
		repo fakeATMAssignmentAdminRepo
		call func(*ATMAssignmentAdminService) error
		want error
	}{
		{"update missing", fakeATMAssignmentAdminRepo{}, update, ErrATMAssignmentNotFound},
		{"update other atm's assignment", fakeATMAssignmentAdminRepo{getByID: foreign}, update, ErrATMAssignmentNotFound},
		{"update disabled assignment", fakeATMAssignmentAdminRepo{getByID: disabled}, update, ErrATMAssignmentNotFound},
		{"update overlapping", fakeATMAssignmentAdminRepo{getByID: active, overlap: true}, update, ErrATMAssignmentOverlap},
		{"update ok", fakeATMAssignmentAdminRepo{getByID: active}, update, nil},
		{"disable other atm's assignment", fakeATMAssignmentAdminRepo{getByID: foreign}, disable, ErrATMAssignmentNotFound},
		{"disable ok", fakeATMAssignmentAdminRepo{getByID: active}, disable, nil},
		{"enable overlapping is rejected up front", fakeATMAssignmentAdminRepo{getByID: disabled, overlap: true}, enable, ErrATMAssignmentOverlap},
		{"enable ok", fakeATMAssignmentAdminRepo{getByID: disabled}, enable, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := tc.repo
			svc := NewATMAssignmentAdminService(&repo, &fakeVendorBranchSubmitter{})
			if err := tc.call(svc); !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}

func TestATMAssignmentAdminService_Update_ExcludesOwnRowFromOverlap(t *testing.T) {
	repo := &fakeATMAssignmentAdminRepo{getByID: &db.GetATMAssignmentAdminByIDRow{ID: 9, AtmID: 3, IsActive: true}}
	if _, err := NewATMAssignmentAdminService(repo, &fakeVendorBranchSubmitter{}).Update(context.Background(), 7, 3, 9, validAssignment(), "ip"); err != nil {
		t.Fatal(err)
	}
	if repo.lastExclude != 9 {
		t.Errorf("update must exclude its own row from the overlap check, got exclude=%d", repo.lastExclude)
	}
}
