package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

type fakeDsrLocationMapRepo struct {
	getByID     *db.GetDsrLocationMapAdminByIDRow
	findActive  *int64
	vaultVendor *int64
}

func (f *fakeDsrLocationMapRepo) List(context.Context, db.ListDsrLocationMapsAdminParams) ([]db.ListDsrLocationMapsAdminRow, error) {
	return nil, nil
}
func (f *fakeDsrLocationMapRepo) ListUnmapped(context.Context, int64) ([]db.ListUnmappedDsrLocationsRow, error) {
	return nil, nil
}
func (f *fakeDsrLocationMapRepo) GetByID(context.Context, int64) (*db.GetDsrLocationMapAdminByIDRow, error) {
	return f.getByID, nil
}
func (f *fakeDsrLocationMapRepo) FindActive(context.Context, int64, string, int64) (*int64, error) {
	return f.findActive, nil
}
func (f *fakeDsrLocationMapRepo) ActiveVaultVendorID(context.Context, int64) (*int64, error) {
	return f.vaultVendor, nil
}

func TestDsrLocationMapAdminService_Create(t *testing.T) {
	own, other, dup := int64(3), int64(4), int64(1)
	cases := []struct {
		name    string
		repo    fakeDsrLocationMapRepo
		req     DsrLocationMapPayload
		field   string // ValidationError field; "" = none
		wantErr error
	}{
		{"happy path", fakeDsrLocationMapRepo{vaultVendor: &own}, DsrLocationMapPayload{DsrLocation: " BINTARO ", VendorVaultID: 9}, "", nil},
		{"blank label", fakeDsrLocationMapRepo{vaultVendor: &own}, DsrLocationMapPayload{DsrLocation: "  ", VendorVaultID: 9}, "dsr_location", nil},
		{"missing vault", fakeDsrLocationMapRepo{vaultVendor: &own}, DsrLocationMapPayload{DsrLocation: "X"}, "vendor_vault_id", nil},
		{"vault disabled/unknown", fakeDsrLocationMapRepo{}, DsrLocationMapPayload{DsrLocation: "X", VendorVaultID: 9}, "vendor_vault_id", nil},
		{"vault of another vendor", fakeDsrLocationMapRepo{vaultVendor: &other}, DsrLocationMapPayload{DsrLocation: "X", VendorVaultID: 9}, "vendor_vault_id", nil},
		{"label already mapped", fakeDsrLocationMapRepo{vaultVendor: &own, findActive: &dup}, DsrLocationMapPayload{DsrLocation: "X", VendorVaultID: 9}, "", ErrDsrLocationMapConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sub := &fakeVendorBranchSubmitter{}
			svc := NewDsrLocationMapAdminService(&tc.repo, sub)
			_, err := svc.Create(context.Background(), 7, own, tc.req, "ip")

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
				p := sub.lastRequest.Payload.(DsrLocationMapPayload)
				if sub.lastRequest.EntityType != "dsr_location_map" || p.DsrLocation != "BINTARO" || p.VendorID != own {
					t.Errorf("unexpected submit: %+v", sub.lastRequest)
				}
			}
		})
	}
}

func TestDsrLocationMapAdminService_OtherVendorIDIsNotFound(t *testing.T) {
	own := int64(3)
	row := &db.GetDsrLocationMapAdminByIDRow{ID: 5, VendorID: 99, DsrLocation: "X", IsActive: true}
	svc := NewDsrLocationMapAdminService(&fakeDsrLocationMapRepo{getByID: row, vaultVendor: &own}, &fakeVendorBranchSubmitter{})
	ctx := context.Background()
	if _, err := svc.Update(ctx, 7, own, 5, DsrLocationMapUpdatePayload{VendorVaultID: 9}, "ip"); !errors.Is(err, ErrDsrLocationMapNotFound) {
		t.Errorf("Update: want not found, got %v", err)
	}
	if _, err := svc.Disable(ctx, 7, own, 5, "ip"); !errors.Is(err, ErrDsrLocationMapNotFound) {
		t.Errorf("Disable: want not found, got %v", err)
	}
	if _, err := svc.Enable(ctx, 7, own, 404, "ip"); !errors.Is(err, ErrDsrLocationMapNotFound) {
		t.Errorf("Enable: want not found, got %v", err)
	}
}

func TestDsrLocationMapAdminService_UpdateDisabledAndEnableConflict(t *testing.T) {
	own, dup := int64(3), int64(8)
	disabled := &db.GetDsrLocationMapAdminByIDRow{ID: 5, VendorID: own, DsrLocation: "X", IsActive: false}
	ctx := context.Background()

	svc := NewDsrLocationMapAdminService(&fakeDsrLocationMapRepo{getByID: disabled, vaultVendor: &own}, &fakeVendorBranchSubmitter{})
	if _, err := svc.Update(ctx, 7, own, 5, DsrLocationMapUpdatePayload{VendorVaultID: 9}, "ip"); !errors.Is(err, ErrDsrLocationMapNotFound) {
		t.Errorf("Update disabled: want not found, got %v", err)
	}

	sub := &fakeVendorBranchSubmitter{}
	svc = NewDsrLocationMapAdminService(&fakeDsrLocationMapRepo{getByID: disabled, findActive: &dup}, sub)
	if _, err := svc.Enable(ctx, 7, own, 5, "ip"); !errors.Is(err, ErrDsrLocationMapConflict) || sub.submitCalled {
		t.Errorf("Enable re-mapped label: want conflict and no submit, got %v", err)
	}
}
