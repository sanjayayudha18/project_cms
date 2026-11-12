package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// fakeVendorVaultAdminServicer exercises HTTP wiring only; logic is covered
// by internal/service/vendor_vault_admin_test.go.
type fakeVendorVaultAdminServicer struct {
	listResult []service.VendorVault
	getResult  *service.VendorVault
	createErr  error
	lastCreate service.VendorVaultPayload
	lastUpdate service.VendorVaultUpdatePayload
}

func (f *fakeVendorVaultAdminServicer) List(context.Context, db.ListVendorVaultsAdminParams) ([]service.VendorVault, error) {
	return f.listResult, nil
}
func (f *fakeVendorVaultAdminServicer) Count(context.Context, db.CountVendorVaultsAdminParams) (int64, error) {
	return int64(len(f.listResult)), nil
}
func (f *fakeVendorVaultAdminServicer) Get(context.Context, int64) (*service.VendorVault, error) {
	return f.getResult, nil
}
func (f *fakeVendorVaultAdminServicer) Create(_ context.Context, _, _ int64, p service.VendorVaultPayload, _ string) (db.MasterDataChangeRequest, error) {
	f.lastCreate = p
	return db.MasterDataChangeRequest{ID: 7, EntityType: "vendor_vault", Op: "create", Status: "pending"}, f.createErr
}
func (f *fakeVendorVaultAdminServicer) Update(_ context.Context, _, _ int64, p service.VendorVaultUpdatePayload, _ string) (db.MasterDataChangeRequest, error) {
	f.lastUpdate = p
	return db.MasterDataChangeRequest{ID: 8, Status: "pending"}, nil
}
func (f *fakeVendorVaultAdminServicer) Disable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 9, Status: "pending"}, nil
}
func (f *fakeVendorVaultAdminServicer) Enable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 10, Status: "pending"}, nil
}

func mountAdminVendorVaultHandler(svc VendorVaultAdminServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM"),
	).Mount("/api/v1/admin/vendors/{vendorID}/vaults", NewAdminVendorVaultHandler(svc).Routes())
	return r, tokenSvc
}

func TestAdminVendorVaultHandler_List_DecimalStringsInResponse(t *testing.T) {
	minB, maxB, cisLimit := "100000000.00", "9000000000.00", "2500000000.00"
	escrowCash, escrowAtm := "001234567890", "998877665544"
	region, alamat := "Bali", "Jl. Raya Kuta No. 10"
	svc := &fakeVendorVaultAdminServicer{
		listResult: []service.VendorVault{{
			ID:               1,
			VendorBranchID:   9,
			BranchName:       "Abacus Bali",
			Region:           &region,
			VaultCode:        "V1",
			VaultName:        "ABA_Bali_V001",
			Category:         "ATM_CASH",
			CurrencyCode:     "IDR",
			Alamat:           &alamat,
			MinBalance:       &minB,
			MaxBalance:       &maxB,
			CisLimitAmount:   &cisLimit,
			CisLimitCurrency: "IDR",
			EscrowCash:       &escrowCash,
			EscrowAtm:        &escrowAtm,
			IsActive:         true,
		}},
	}
	router, tokenSvc := mountAdminVendorVaultHandler(svc)

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/vaults", tokenForRole(t, tokenSvc, 1, "ADMIN"), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`"vault_code":"V1"`,
		`"vault_name":"ABA_Bali_V001"`,
		`"region":"Bali"`,
		`"alamat":"Jl. Raya Kuta No. 10"`,
		`"min_balance":"100000000.00"`,
		`"max_balance":"9000000000.00"`,
		`"cis_limit_amount":"2500000000.00"`,
		`"cis_limit_currency":"IDR"`,
		`"escrow_cash":"001234567890"`,
		`"escrow_atm":"998877665544"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %s in response body, got: %s", want, body)
		}
	}
}

func TestAdminVendorVaultHandler_Get_NotFound(t *testing.T) {
	router, tokenSvc := mountAdminVendorVaultHandler(&fakeVendorVaultAdminServicer{})
	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/vaults/99", tokenForRole(t, tokenSvc, 1, "ADMIN"), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestAdminVendorVaultHandler_Create_Returns202(t *testing.T) {
	svc := &fakeVendorVaultAdminServicer{}
	router, tokenSvc := mountAdminVendorVaultHandler(svc)
	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/vaults", tokenForRole(t, tokenSvc, 1, "ADMIN"),
		`{"vendor_branch_id":9,"vault_code":"V1","vault_name":"ABA_Bali_V001","category":"ATM","currency_code":"IDR","alamat":"Jl. Kuta","min_balance":"100.00","max_balance":"500.00","cis_limit_amount":"1000.00","escrow_cash":"001234567890","escrow_atm":"998877665544"}`)
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"change_request_id":7`) {
		t.Fatalf("expected 202 with change_request_id, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.lastCreate.VaultName == nil || *svc.lastCreate.VaultName != "ABA_Bali_V001" ||
		svc.lastCreate.MaxBalance == nil || *svc.lastCreate.MaxBalance != "500.00" ||
		svc.lastCreate.CisLimitAmount == nil || *svc.lastCreate.CisLimitAmount != "1000.00" ||
		svc.lastCreate.EscrowCash == nil || *svc.lastCreate.EscrowCash != "001234567890" ||
		svc.lastCreate.EscrowAtm == nil || *svc.lastCreate.EscrowAtm != "998877665544" {
		t.Errorf("unexpected create payload: %+v", svc.lastCreate)
	}
}

func TestAdminVendorVaultHandler_Create_ErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{&service.ValidationError{Field: "category", Message: "harus ATM atau CASH"}, http.StatusUnprocessableEntity},
		{service.ErrVendorVaultCodeConflict, http.StatusConflict},
		{service.ErrVendorVaultNameConflict, http.StatusConflict},
		{service.ErrMasterDataChangePending, http.StatusConflict},
	}
	for _, tc := range cases {
		router, tokenSvc := mountAdminVendorVaultHandler(&fakeVendorVaultAdminServicer{createErr: tc.err})
		rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/vaults", tokenForRole(t, tokenSvc, 1, "ADMIN"), `{"vault_code":"V1"}`)
		if rec.Code != tc.want {
			t.Errorf("err %v: expected %d, got %d", tc.err, tc.want, rec.Code)
		}
	}
}

func TestAdminVendorVaultHandler_WrongRole_Forbidden(t *testing.T) {
	router, tokenSvc := mountAdminVendorVaultHandler(&fakeVendorVaultAdminServicer{})
	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/vaults", tokenForRole(t, tokenSvc, 1, "ATM-USER"), "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestAdminVendorVaultHandler_Toggle_Returns202(t *testing.T) {
	router, tokenSvc := mountAdminVendorVaultHandler(&fakeVendorVaultAdminServicer{})
	for _, path := range []string{"/disable", "/enable"} {
		rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/vaults/1"+path, tokenForRole(t, tokenSvc, 1, "ADMIN_PARAM"), "")
		if rec.Code != http.StatusAccepted {
			t.Errorf("%s: expected 202, got %d", path, rec.Code)
		}
	}
}
