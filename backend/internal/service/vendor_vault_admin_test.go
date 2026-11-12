package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

type fakeVendorVaultAdminRepo struct {
	getByID             *db.GetVendorVaultAdminByIDRow
	findByCode          *int64
	findByBranchAndName *int64
	nextVaultName       string
	noAutoVaultName     bool
	branchVendorID      *int64
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
func (f *fakeVendorVaultAdminRepo) FindByBranchAndName(context.Context, int64, string) (*int64, error) {
	return f.findByBranchAndName, nil
}
func (f *fakeVendorVaultAdminRepo) GenerateNextVaultName(context.Context, int64) (string, error) {
	if f.noAutoVaultName {
		return "", nil
	}
	if f.nextVaultName != "" {
		return f.nextVaultName, nil
	}
	return "ABA_Bali_V001", nil
}
func (f *fakeVendorVaultAdminRepo) BranchVendorID(context.Context, int64) (*int64, error) {
	return f.branchVendorID, nil
}

func sp(s string) *string { return &s }

func validVaultUpdate() VendorVaultUpdatePayload {
	return VendorVaultUpdatePayload{
		Category:     "ATM",
		CurrencyCode: "idr",
		MinBalance:   sp("100.00"),
		MaxBalance:   sp("500.00"),
	}
}

func TestValidateVaultFields(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(*VendorVaultUpdatePayload)
		field string // "" = valid
	}{
		{"valid, currency upper-cased", func(p *VendorVaultUpdatePayload) {}, ""},
		{"category ATM_CASH ok", func(p *VendorVaultUpdatePayload) { p.Category = "ATM_CASH" }, ""},
		{"category CASH ok", func(p *VendorVaultUpdatePayload) { p.Category = "CASH" }, ""},
		{"bad category", func(p *VendorVaultUpdatePayload) { p.Category = "VAULT" }, "category"},
		{"missing currency", func(p *VendorVaultUpdatePayload) { p.CurrencyCode = "" }, "currency_code"},
		{"min > max", func(p *VendorVaultUpdatePayload) { p.MinBalance = sp("600") }, "min_balance"},
		{"min == max ok", func(p *VendorVaultUpdatePayload) { p.MinBalance = sp("500") }, ""},
		{"negative min", func(p *VendorVaultUpdatePayload) { p.MinBalance = sp("-1") }, "min_balance"},
		{"negative max", func(p *VendorVaultUpdatePayload) { p.MaxBalance = sp("-1") }, "max_balance"},
		{"non-numeric", func(p *VendorVaultUpdatePayload) { p.MaxBalance = sp("abc") }, "max_balance"},
		{"fraction syntax rejected", func(p *VendorVaultUpdatePayload) { p.MaxBalance = sp("1/2") }, "max_balance"},
		{"lat out of range", func(p *VendorVaultUpdatePayload) { p.Latitude = sp("90.5") }, "latitude"},
		{"lon out of range", func(p *VendorVaultUpdatePayload) { p.Longitude = sp("-180.1") }, "longitude"},
		{"coords at bounds ok", func(p *VendorVaultUpdatePayload) { p.Latitude = sp("-90"); p.Longitude = sp("180") }, ""},
		{"nil balances ok", func(p *VendorVaultUpdatePayload) { p.MinBalance, p.MaxBalance = nil, nil }, ""},
		{"cis_limit_amount valid", func(p *VendorVaultUpdatePayload) { p.CisLimitAmount = sp("2500000000.00") }, ""},
		{"cis_limit_amount zero rejected", func(p *VendorVaultUpdatePayload) { p.CisLimitAmount = sp("0") }, "cis_limit_amount"},
		{"cis_limit_amount 3 decimals rejected", func(p *VendorVaultUpdatePayload) { p.CisLimitAmount = sp("100.123") }, "cis_limit_amount"},
		// R1.4: escrow_cash & escrow_atm optional, each 12 digits, not unique.
		{"escrow_cash 12 digits ok", func(p *VendorVaultUpdatePayload) { p.EscrowCash = sp("001234567890") }, ""},
		{"escrow_atm 12 digits ok", func(p *VendorVaultUpdatePayload) { p.EscrowAtm = sp("998877665544") }, ""},
		{"escrow_cash blank ok", func(p *VendorVaultUpdatePayload) { p.EscrowCash = sp("  ") }, ""},
		{"escrow_atm blank ok", func(p *VendorVaultUpdatePayload) { p.EscrowAtm = sp("  ") }, ""},
		{"escrow_cash 11 digits", func(p *VendorVaultUpdatePayload) { p.EscrowCash = sp("12345678901") }, "escrow_cash"},
		{"escrow_cash 13 digits", func(p *VendorVaultUpdatePayload) { p.EscrowCash = sp("1234567890123") }, "escrow_cash"},
		{"escrow_atm letters", func(p *VendorVaultUpdatePayload) { p.EscrowAtm = sp("12345678901A") }, "escrow_atm"},
		{"escrow_atm inner spaces", func(p *VendorVaultUpdatePayload) { p.EscrowAtm = sp("1234 5678 90") }, "escrow_atm"},
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

	t.Run("happy path submits vendor_vault create with trimmed code and auto-generated vault_name", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorVaultAdminService(&fakeVendorVaultAdminRepo{branchVendorID: &owner, nextVaultName: "ABA_Bali_V001"}, sub)
		if _, err := svc.Create(context.Background(), 7, 3, req, "ip"); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if sub.lastRequest.EntityType != "vendor_vault" || sub.lastRequest.Op != "create" {
			t.Errorf("unexpected submit: %+v", sub.lastRequest)
		}
		p := sub.lastRequest.Payload.(VendorVaultPayload)
		if p.VaultCode != "V1" {
			t.Errorf("code not trimmed: %q", p.VaultCode)
		}
		if p.VaultName == nil || *p.VaultName != "ABA_Bali_V001" {
			t.Errorf("expected auto-generated vault_name ABA_Bali_V001, got %v", showStr(p.VaultName))
		}
	})

	t.Run("empty vault_code defaults to auto-generated vault_name", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorVaultAdminService(&fakeVendorVaultAdminRepo{branchVendorID: &owner, nextVaultName: "ABA_Bali_V002"}, sub)
		noCodeReq := VendorVaultPayload{VendorBranchID: 9, VendorVaultUpdatePayload: validVaultUpdate()}
		if _, err := svc.Create(context.Background(), 7, 3, noCodeReq, "ip"); err != nil {
			t.Fatalf("Create without vault_code: %v", err)
		}
		p := sub.lastRequest.Payload.(VendorVaultPayload)
		if p.VaultCode != "ABA_Bali_V002" || p.VaultName == nil || *p.VaultName != "ABA_Bali_V002" {
			t.Errorf("expected vault_code and vault_name ABA_Bali_V002, got code=%q name=%v", p.VaultCode, showStr(p.VaultName))
		}
	})

	t.Run("explicit vault_name is trimmed and preserved", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorVaultAdminService(&fakeVendorVaultAdminRepo{branchVendorID: &owner}, sub)
		namedReq := req
		namedReq.VaultName = sp("  ABA_Bali_V009  ")
		if _, err := svc.Create(context.Background(), 7, 3, namedReq, "ip"); err != nil {
			t.Fatalf("Create: %v", err)
		}
		p := sub.lastRequest.Payload.(VendorVaultPayload)
		if p.VaultName == nil || *p.VaultName != "ABA_Bali_V009" {
			t.Errorf("expected trimmed vault_name ABA_Bali_V009, got %v", showStr(p.VaultName))
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

	t.Run("duplicate vault_name in same branch conflicts", func(t *testing.T) {
		existing := int64(2)
		sub := &fakeVendorBranchSubmitter{}
		svc := NewVendorVaultAdminService(&fakeVendorVaultAdminRepo{branchVendorID: &owner, findByBranchAndName: &existing}, sub)
		namedReq := req
		namedReq.VaultName = sp("ABA_Bali_V001")
		_, err := svc.Create(context.Background(), 7, 3, namedReq, "ip")
		if !errors.Is(err, ErrVendorVaultCodeConflict) || !errors.Is(err, ErrVendorVaultNameConflict) || sub.submitCalled {
			t.Fatalf("want ErrVendorVaultNameConflict and no submit, got %v", err)
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

func TestVendorVaultAdminService_Update_VaultNameConflict(t *testing.T) {
	otherID := int64(22)
	repo := &fakeVendorVaultAdminRepo{
		getByID:             &db.GetVendorVaultAdminByIDRow{ID: 1, VendorBranchID: 9, VaultName: "ABA_Bali_V001"},
		findByBranchAndName: &otherID,
	}
	sub := &fakeVendorBranchSubmitter{}
	svc := NewVendorVaultAdminService(repo, sub)

	upd := validVaultUpdate()
	upd.VaultName = sp("ABA_Bali_V002")
	if _, err := svc.Update(context.Background(), 7, 1, upd, "ip"); !errors.Is(err, ErrVendorVaultNameConflict) || sub.submitCalled {
		t.Fatalf("expected ErrVendorVaultNameConflict, got %v", err)
	}
}

func TestValidateVaultFields_EscrowAndAlamatNormalized(t *testing.T) {
	for in, want := range map[string]string{" 001234567890 ": "001234567890", "   ": "<nil>", "": "<nil>"} {
		p := validVaultUpdate()
		p.EscrowCash = sp(in)
		p.EscrowAtm = sp(in)
		if err := validateVaultFields(&p); err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if got := showStr(p.EscrowCash); got != want {
			t.Errorf("%q: escrow_cash = %s, want %s", in, got, want)
		}
		if got := showStr(p.EscrowAtm); got != want {
			t.Errorf("%q: escrow_atm = %s, want %s", in, got, want)
		}
	}
	p := validVaultUpdate()
	p.Alamat = sp("  Jl. Raya Kuta No. 10  ")
	if err := validateVaultFields(&p); err != nil {
		t.Fatalf("alamat: %v", err)
	}
	if showStr(p.Alamat) != "Jl. Raya Kuta No. 10" {
		t.Errorf("alamat = %s, want trimmed", showStr(p.Alamat))
	}
}
