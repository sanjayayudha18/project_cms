package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

func showStr(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

// R1.5 (moved from vendor_branches to vendor_vaults in migration 040):
// Limit CIS optional, > 0, at most 2 decimals, fits numeric(20,2).
func TestNormalizeCisLimit(t *testing.T) {
	tests := []struct {
		name    string
		in      *string
		want    *string
		wantErr bool
	}{
		{"nil stays nil", nil, nil, false},
		{"empty = NULL", sp(""), nil, false},
		{"blank = NULL", sp("   "), nil, false},
		{"integer", sp("1000"), sp("1000"), false},
		{"trimmed, 2 decimals", sp(" 1500000.50 "), sp("1500000.50"), false},
		{"1 decimal", sp("7.5"), sp("7.5"), false},
		{"18 integer digits", sp("999999999999999999.99"), sp("999999999999999999.99"), false},
		{"zero", sp("0"), nil, true},
		{"zero with decimals", sp("0.00"), nil, true},
		{"negative", sp("-5"), nil, true},
		{"3 decimals", sp("1.234"), nil, true},
		{"exponent", sp("1e5"), nil, true},
		{"thousand separator", sp("1,000"), nil, true},
		{"letters", sp("abc"), nil, true},
		{"19 integer digits", sp("1000000000000000000"), nil, true},
		{"trailing dot", sp("10."), nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeCisLimit(tt.in)
			if tt.wantErr {
				var ve *ValidationError
				if !errors.As(err, &ve) || ve.Field != "cis_limit_amount" {
					t.Fatalf("err = %v, want ValidationError on cis_limit_amount", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
				t.Errorf("got %v, want %v", showStr(got), showStr(tt.want))
			}
		})
	}
}

func TestVendorVaultAdminService_Create_CisLimitNormalized(t *testing.T) {
	owner := int64(3)
	sub := &fakeVendorBranchSubmitter{}
	svc := NewVendorVaultAdminService(&fakeVendorVaultAdminRepo{branchVendorID: &owner}, sub)

	if _, err := svc.Create(context.Background(), 7, 3, VendorVaultPayload{
		VendorBranchID: 9,
		VaultCode:      "V1",
		VendorVaultUpdatePayload: VendorVaultUpdatePayload{
			Category:       "ATM",
			CurrencyCode:   "IDR",
			CisLimitAmount: sp(" 2500000000 "),
		},
	}, "127.0.0.1"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	p := sub.lastRequest.Payload.(VendorVaultPayload)
	if p.CisLimitAmount == nil || *p.CisLimitAmount != "2500000000" {
		t.Errorf("payload limit = %v, want 2500000000", showStr(p.CisLimitAmount))
	}
}

func TestVendorVaultAdminService_CisLimitInvalid_Rejected(t *testing.T) {
	owner := int64(3)
	repo := &fakeVendorVaultAdminRepo{
		branchVendorID: &owner,
		getByID:        &db.GetVendorVaultAdminByIDRow{ID: 1, VendorBranchID: 9},
	}
	sub := &fakeVendorBranchSubmitter{}
	svc := NewVendorVaultAdminService(repo, sub)
	ctx := context.Background()

	_, createErr := svc.Create(ctx, 7, 3, VendorVaultPayload{
		VendorBranchID: 9,
		VaultCode:      "V1",
		VendorVaultUpdatePayload: VendorVaultUpdatePayload{
			Category:       "ATM",
			CurrencyCode:   "IDR",
			CisLimitAmount: sp("0"),
		},
	}, "127.0.0.1")
	_, updateErr := svc.Update(ctx, 7, 1, VendorVaultUpdatePayload{
		Category:       "ATM",
		CurrencyCode:   "IDR",
		CisLimitAmount: sp("12.345"),
	}, "127.0.0.1")
	for name, err := range map[string]error{"create": createErr, "update": updateErr} {
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != "cis_limit_amount" {
			t.Errorf("%s: err = %v, want ValidationError on cis_limit_amount", name, err)
		}
	}
	if sub.submitCalled {
		t.Error("Submit must not be called for an invalid limit")
	}
}
