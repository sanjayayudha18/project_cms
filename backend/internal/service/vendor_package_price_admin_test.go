package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// fakePriceAdminRepo: vendor 3 is an FLM vendor, 4 is INTERNAL (ROH); price 1
// is vendor 3's running row, price 2 vendor 3's ended row.
type fakePriceAdminRepo struct{ kindErr error }

func (fakePriceAdminRepo) List(context.Context, db.ListVendorPackagePricesAdminParams) ([]db.ListVendorPackagePricesAdminRow, error) {
	return nil, nil
}
func (fakePriceAdminRepo) Count(context.Context, db.CountVendorPackagePricesAdminParams) (int64, error) {
	return 0, nil
}
func (fakePriceAdminRepo) GetByID(_ context.Context, id int64) (*db.GetVendorPackagePriceAdminByIDRow, error) {
	start := pgtype.Date{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}
	switch id {
	case 1:
		return &db.GetVendorPackagePriceAdminByIDRow{ID: 1, VendorID: 3, EffectiveStartDate: start}, nil
	case 2:
		return &db.GetVendorPackagePriceAdminByIDRow{ID: 2, VendorID: 3, EffectiveStartDate: start,
			EffectiveEndDate: pgtype.Date{Time: time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC), Valid: true}}, nil
	}
	return nil, nil
}
func (f fakePriceAdminRepo) VendorKind(_ context.Context, vendorID int64) (*db.GetVendorKindForPackagePriceRow, error) {
	if f.kindErr != nil {
		return nil, f.kindErr
	}
	switch vendorID {
	case 3:
		return &db.GetVendorKindForPackagePriceRow{Kind: "FLM_VENDOR", IsActive: true}, nil
	case 4:
		return &db.GetVendorKindForPackagePriceRow{Kind: "INTERNAL", IsActive: true}, nil
	}
	return nil, nil
}

func validPriceCreate() VendorPackagePriceCreatePayload {
	base := "1250000.50"
	return VendorPackagePriceCreatePayload{Package: " PAKET 3 ", MachineGroup: "ATM", PriceClass: "REGULAR", TierMin: 1,
		EffectiveStartDate: "2027-01-01", VendorPackagePriceContentPayload: VendorPackagePriceContentPayload{BasePrice: &base}}
}

// Vendor-wide prices are money + maker-checker: every change is staged via
// Submit (never written directly), and an INTERNAL vendor (ROH) never gets one.
func TestVendorPackagePriceAdminService_Create(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name     string
		repo     fakePriceAdminRepo
		vendorID int64
		mutate   func(*VendorPackagePriceCreatePayload)
		wantErr  error // nil + field "" = staged
		field    string
	}{
		{"staged", fakePriceAdminRepo{}, 3, nil, nil, ""},
		{"unknown vendor", fakePriceAdminRepo{}, 99, nil, ErrVendorPackagePriceNoVendor, ""},
		{"internal vendor", fakePriceAdminRepo{}, 4, nil, ErrVendorPackagePriceInternal, ""},
		{"missing label", fakePriceAdminRepo{}, 3, func(p *VendorPackagePriceCreatePayload) { p.Package = " " }, nil, "package"},
		{"bad price", fakePriceAdminRepo{}, 3, func(p *VendorPackagePriceCreatePayload) { bad := "12,5"; p.BasePrice = &bad }, nil, "base_price"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sub := &fakeVendorBranchSubmitter{}
			req := validPriceCreate()
			if tc.mutate != nil {
				tc.mutate(&req)
			}
			_, err := NewVendorPackagePriceAdminService(tc.repo, sub).Create(ctx, 7, tc.vendorID, req, "ip")
			var ve *ValidationError
			switch {
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
			case tc.field != "":
				if !errors.As(err, &ve) || ve.Field != tc.field {
					t.Fatalf("err = %v, want %s ValidationError", err, tc.field)
				}
			default:
				p, ok := sub.lastRequest.Payload.(VendorPackagePricePayload)
				if err != nil || sub.lastRequest.EntityType != "vendor_package_price" || sub.lastRequest.Op != "create" ||
					!ok || p.VendorID != 3 || p.Package != "PAKET 3" || *p.BasePrice != "1250000.50" {
					t.Fatalf("staged %+v, %v", sub.lastRequest, err)
				}
				return
			}
			if sub.submitCalled {
				t.Error("a rejected price must not be staged")
			}
		})
	}

	sub := &fakeVendorBranchSubmitter{}
	if _, err := NewVendorPackagePriceAdminService(fakePriceAdminRepo{kindErr: errors.New("db down")}, sub).Create(ctx, 7, 3, validPriceCreate(), "ip"); err == nil || sub.submitCalled {
		t.Errorf("vendor lookup failure = %v (submitted %v), want error and no submit", err, sub.submitCalled)
	}
}

func TestVendorPackagePriceAdminService_UpdateAndDisable(t *testing.T) {
	ctx := context.Background()
	newBase, early := "1300000.25", "2025-12-31"
	for _, tc := range []struct {
		name     string
		vendorID int64
		id       int64
		req      VendorPackagePriceContentPayload
		wantErr  error
		field    string
	}{
		{"staged", 3, 1, VendorPackagePriceContentPayload{BasePrice: &newBase}, nil, ""},
		{"another vendor's row", 5, 1, VendorPackagePriceContentPayload{BasePrice: &newBase}, ErrVendorPackagePriceNotFound, ""},
		{"missing row", 3, 9, VendorPackagePriceContentPayload{BasePrice: &newBase}, ErrVendorPackagePriceNotFound, ""},
		{"ended row is read-only", 3, 2, VendorPackagePriceContentPayload{BasePrice: &newBase}, ErrVendorPackagePriceNotFound, ""},
		{"end before start", 3, 1, VendorPackagePriceContentPayload{BasePrice: &newBase, EffectiveEndDate: &early}, nil, "effective_end_date"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sub := &fakeVendorBranchSubmitter{}
			_, err := NewVendorPackagePriceAdminService(fakePriceAdminRepo{}, sub).Update(ctx, 7, tc.vendorID, tc.id, tc.req, "ip")
			var ve *ValidationError
			switch {
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) || sub.submitCalled {
					t.Fatalf("err = %v (submitted %v), want %v", err, sub.submitCalled, tc.wantErr)
				}
			case tc.field != "":
				if !errors.As(err, &ve) || ve.Field != tc.field || sub.submitCalled {
					t.Fatalf("err = %v (submitted %v), want %s ValidationError", err, sub.submitCalled, tc.field)
				}
			default:
				if err != nil || sub.lastRequest.Op != "update" || sub.lastRequest.EntityID == nil || *sub.lastRequest.EntityID != 1 || sub.lastRequest.Before == nil {
					t.Fatalf("staged %+v, %v (update needs id + before snapshot)", sub.lastRequest, err)
				}
			}
		})
	}

	sub := &fakeVendorBranchSubmitter{}
	svc := NewVendorPackagePriceAdminService(fakePriceAdminRepo{}, sub)
	if _, err := svc.Disable(ctx, 7, 3, 1, "ip"); err != nil || sub.lastRequest.Op != "disable" || sub.lastRequest.Before == nil {
		t.Fatalf("disable staged %+v, %v", sub.lastRequest, err)
	}
	if _, err := svc.Disable(ctx, 7, 5, 1, "ip"); !errors.Is(err, ErrVendorPackagePriceNotFound) {
		t.Fatalf("disable another vendor's row err = %v", err)
	}

	// QC F17: a price starting today or later cannot be ended "as of
	// yesterday" (period CHECK end >= start) -- refused at submit, not at approve.
	sub = &fakeVendorBranchSubmitter{}
	future := NewVendorPackagePriceAdminService(futurePriceRepo{}, sub)
	var ve *ValidationError
	if _, err := future.Disable(ctx, 7, 3, 1, "ip"); !errors.As(err, &ve) || ve.Field != "effective_start_date" || sub.submitCalled {
		t.Fatalf("disable not-yet-effective price = %v (submitted %v), want effective_start_date ValidationError", err, sub.submitCalled)
	}
}

// futurePriceRepo serves vendor 3's price 1 starting today (WIB).
type futurePriceRepo struct{ fakePriceAdminRepo }

func (futurePriceRepo) GetByID(context.Context, int64) (*db.GetVendorPackagePriceAdminByIDRow, error) {
	return &db.GetVendorPackagePriceAdminByIDRow{ID: 1, VendorID: 3,
		EffectiveStartDate: pgtype.Date{Time: jakartaCalendarDate(0), Valid: true}}, nil
}
