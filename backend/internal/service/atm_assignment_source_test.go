package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

func vendorAssignment() ATMAssignmentUpdatePayload {
	return ATMAssignmentUpdatePayload{Source: AssignmentSourceVendor, VendorID: 2, VendorBranchID: 9, Package: "PAKET 4",
		EffectiveStartDate: "2026-03-01"}
}

func TestNormalizeSource(t *testing.T) {
	cases := []struct {
		name  string
		p     ATMAssignmentUpdatePayload
		field string // "" = valid
		want  string // expected canonical Source when valid
	}{
		{"empty source = branch (legacy payload)", ATMAssignmentUpdatePayload{VendorPackageID: 5}, "", AssignmentSourceBranch},
		{"branch needs package", ATMAssignmentUpdatePayload{Source: "branch"}, "vendor_package_id", ""},
		{"branch rejects vendor fields", ATMAssignmentUpdatePayload{Source: "branch", VendorPackageID: 5, Package: "PAKET 4"}, "source", ""},
		{"vendor ok, label trimmed", ATMAssignmentUpdatePayload{Source: "vendor", VendorID: 2, VendorBranchID: 9, Package: "  PAKET 4 "}, "", AssignmentSourceVendor},
		{"vendor rejects vendor_package_id", ATMAssignmentUpdatePayload{Source: "vendor", VendorPackageID: 5, VendorID: 2, VendorBranchID: 9, Package: "P"}, "source", ""},
		{"vendor needs vendor", ATMAssignmentUpdatePayload{Source: "vendor", VendorBranchID: 9, Package: "P"}, "vendor_id", ""},
		{"vendor needs branch", ATMAssignmentUpdatePayload{Source: "vendor", VendorID: 2, Package: "P"}, "vendor_branch_id", ""},
		{"vendor needs label", ATMAssignmentUpdatePayload{Source: "vendor", VendorID: 2, VendorBranchID: 9, Package: "   "}, "package", ""},
		{"unknown source", ATMAssignmentUpdatePayload{Source: "other", VendorPackageID: 5}, "source", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.p
			err := normalizeSource(&p)
			if tc.field != "" {
				var ve *ValidationError
				if !errors.As(err, &ve) || ve.Field != tc.field {
					t.Fatalf("want ValidationError on %s, got %v", tc.field, err)
				}
				return
			}
			if err != nil || p.Source != tc.want {
				t.Fatalf("got source=%q err=%v, want %q", p.Source, err, tc.want)
			}
		})
	}
}

func TestATMAssignmentAdminService_VendorSource_Validate(t *testing.T) {
	cases := []struct {
		name  string
		repo  fakeATMAssignmentAdminRepo
		field string
	}{
		{name: "valid"},
		{name: "ATM price group unmapped", repo: fakeATMAssignmentAdminRepo{noPriceGroup: true}, field: "package"},
		{name: "branch not of an active FLM vendor", repo: fakeATMAssignmentAdminRepo{branchInvalid: true}, field: "vendor_branch_id"},
		{name: "no tariff for label", repo: fakeATMAssignmentAdminRepo{tariffMissing: true}, field: "package"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := vendorAssignment()
			repo := tc.repo
			err := NewATMAssignmentAdminService(&repo, &fakeVendorBranchSubmitter{}).validate(context.Background(), 1, 0, &p)
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

func TestATMAssignmentAdminService_VendorSource_OverlapAcrossModes(t *testing.T) {
	p := vendorAssignment()
	err := NewATMAssignmentAdminService(&fakeATMAssignmentAdminRepo{overlap: true}, nil).validate(context.Background(), 1, 0, &p)
	if !errors.Is(err, ErrATMAssignmentOverlap) {
		t.Fatalf("want ErrATMAssignmentOverlap, got %v", err)
	}
}

func TestATMAssignmentAdminService_VendorSource_Create(t *testing.T) {
	t.Run("automatic period submits vendor payload, no overlap check", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewATMAssignmentAdminService(&fakeATMAssignmentAdminRepo{overlap: true}, sub)
		req := vendorAssignment()
		req.EffectiveStartDate = ""
		if _, err := svc.Create(context.Background(), 7, 3, req, "ip"); err != nil {
			t.Fatalf("Create: %v", err)
		}
		p := sub.lastRequest.Payload.(ATMAssignmentPayload)
		if p.ATMID != 3 || p.Source != AssignmentSourceVendor || p.VendorID != 2 || p.VendorBranchID != 9 || p.Package != "PAKET 4" || p.VendorPackageID != 0 {
			t.Errorf("payload wrong: %+v", p)
		}
	})

	t.Run("missing tariff blocks submit", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewATMAssignmentAdminService(&fakeATMAssignmentAdminRepo{tariffMissing: true}, sub)
		req := vendorAssignment()
		req.EffectiveStartDate = ""
		if _, err := svc.Create(context.Background(), 7, 3, req, "ip"); err == nil || sub.submitCalled {
			t.Fatalf("want validation error and no submit, got %v", err)
		}
	})
}

func TestATMAssignmentAdminService_Update_RejectsModeSwitch(t *testing.T) {
	before := &db.GetATMAssignmentAdminByIDRow{ID: 4, AtmID: 3, IsActive: true, Source: AssignmentSourceBranch}
	svc := NewATMAssignmentAdminService(&fakeATMAssignmentAdminRepo{getByID: before}, &fakeVendorBranchSubmitter{})
	_, err := svc.Update(context.Background(), 7, 3, 4, vendorAssignment(), "ip")
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "source" {
		t.Fatalf("want ValidationError on source, got %v", err)
	}
}

func TestATMAssignmentAdminService_Update_VendorLabelWithinMode(t *testing.T) {
	before := &db.GetATMAssignmentAdminByIDRow{ID: 4, AtmID: 3, IsActive: true, Source: AssignmentSourceVendor}
	sub := &fakeVendorBranchSubmitter{}
	svc := NewATMAssignmentAdminService(&fakeATMAssignmentAdminRepo{getByID: before}, sub)
	req := vendorAssignment()
	req.Package = "PAKET 5"
	if _, err := svc.Update(context.Background(), 7, 3, 4, req, "ip"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if p := sub.lastRequest.Payload.(ATMAssignmentUpdatePayload); p.Package != "PAKET 5" {
		t.Errorf("payload wrong: %+v", p)
	}
}

func TestATMAssignmentAdminService_PackageOptions(t *testing.T) {
	t.Run("returns labels", func(t *testing.T) {
		svc := NewATMAssignmentAdminService(&fakeATMAssignmentAdminRepo{options: []string{"PAKET 3", "PAKET 4"}}, nil)
		got, err := svc.PackageOptions(context.Background(), 3, 2)
		if err != nil || len(got) != 2 {
			t.Fatalf("got %v err=%v", got, err)
		}
	})
	t.Run("unknown ATM", func(t *testing.T) {
		svc := NewATMAssignmentAdminService(&fakeATMAssignmentAdminRepo{atmMissing: true}, nil)
		if _, err := svc.PackageOptions(context.Background(), 3, 2); !errors.Is(err, ErrAssignmentATMNotFound) {
			t.Fatalf("want ErrAssignmentATMNotFound, got %v", err)
		}
	})
	t.Run("vendor required", func(t *testing.T) {
		svc := NewATMAssignmentAdminService(&fakeATMAssignmentAdminRepo{}, nil)
		var ve *ValidationError
		if _, err := svc.PackageOptions(context.Background(), 3, 0); !errors.As(err, &ve) || ve.Field != "vendor_id" {
			t.Fatalf("want ValidationError on vendor_id, got %v", err)
		}
	})
}

func TestValidateSourceAtApply(t *testing.T) {
	now := time.Date(2026, 3, 1, 3, 0, 0, 0, time.UTC)
	t.Run("branch mode is not re-checked", func(t *testing.T) {
		repo := &fakeATMAssignmentAdminRepo{tariffMissing: true}
		if err := validateSourceAtApply(context.Background(), repo, 3, ATMAssignmentUpdatePayload{VendorPackageID: 5}, now); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("vendor mode still valid", func(t *testing.T) {
		if err := validateSourceAtApply(context.Background(), &fakeATMAssignmentAdminRepo{}, 3, vendorAssignment(), now); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("tariff vanished between submit and apply -> source invalid", func(t *testing.T) {
		err := validateSourceAtApply(context.Background(), &fakeATMAssignmentAdminRepo{tariffMissing: true}, 3, vendorAssignment(), now)
		if !errors.Is(err, ErrATMAssignmentSourceInvalid) {
			t.Fatalf("want ErrATMAssignmentSourceInvalid, got %v", err)
		}
	})
}

func TestAssignmentAsOf(t *testing.T) {
	// 2026-03-01 18:00 UTC is already 2026-03-02 in WIB.
	now := time.Date(2026, 3, 1, 18, 0, 0, 0, time.UTC)
	if got := assignmentAsOf(ATMAssignmentUpdatePayload{}, now).Format(assignmentDateLayout); got != "2026-03-02" {
		t.Errorf("automatic period asOf = %s, want 2026-03-02 (WIB)", got)
	}
	if got := assignmentAsOf(ATMAssignmentUpdatePayload{EffectiveStartDate: "2026-05-10"}, now).Format(assignmentDateLayout); got != "2026-05-10" {
		t.Errorf("explicit asOf = %s, want 2026-05-10", got)
	}
}

// listFakeRepo returns canned rows from List/GetByID on top of the shared fake.
type listFakeRepo struct {
	fakeATMAssignmentAdminRepo
	rows []db.ListATMAssignmentsAdminRow
}

func (f *listFakeRepo) List(context.Context, db.ListATMAssignmentsAdminParams) ([]db.ListATMAssignmentsAdminRow, error) {
	return f.rows, nil
}

func TestATMAssignmentAdminService_ListAndGet_MapBothSources(t *testing.T) {
	pkg, vendor, branch := int64(8), int64(2), int64(5)
	repo := &listFakeRepo{rows: []db.ListATMAssignmentsAdminRow{
		{ID: 1, AtmID: 3, VendorPackageID: &pkg, PackageCode: "PKG-A", Source: AssignmentSourceBranch, EffectiveStartDate: date("2026-01-01"), IsActive: true, VendorID: &vendor, VendorBranchID: &branch},
		{ID: 2, AtmID: 3, PackageCode: "PAKET 4", Source: AssignmentSourceVendor, EffectiveStartDate: date("2026-07-01"), IsActive: true, VendorID: &vendor, VendorBranchID: &branch},
	}}
	repo.getByID = &db.GetATMAssignmentAdminByIDRow{ID: 2, AtmID: 3, PackageCode: "PAKET 4", Source: AssignmentSourceVendor,
		EffectiveStartDate: date("2026-07-01"), IsActive: true, VendorID: &vendor, VendorBranchID: &branch}
	svc := NewATMAssignmentAdminService(repo, nil)

	got, err := svc.List(context.Background(), db.ListATMAssignmentsAdminParams{AtmID: 3})
	if err != nil || len(got) != 2 {
		t.Fatalf("List = %v err=%v", got, err)
	}
	if got[0].Source != AssignmentSourceBranch || got[0].VendorPackageID == nil || *got[0].VendorPackageID != 8 {
		t.Errorf("branch row wrong: %+v", got[0])
	}
	if v := got[1]; v.Source != AssignmentSourceVendor || v.VendorPackageID != nil || v.PackageCode != "PAKET 4" ||
		v.VendorID == nil || *v.VendorID != 2 || v.VendorBranchID == nil || *v.VendorBranchID != 5 {
		t.Errorf("vendor-wide row wrong: %+v", v)
	}

	one, err := svc.Get(context.Background(), 3, 2)
	if err != nil || one == nil || one.Source != AssignmentSourceVendor || one.VendorBranchID == nil || *one.VendorBranchID != 5 {
		t.Errorf("Get wrong: %+v err=%v", one, err)
	}
	if other, err := svc.Get(context.Background(), 99, 2); err != nil || other != nil {
		t.Errorf("Get for another ATM must be nil, got %+v err=%v", other, err)
	}
}
