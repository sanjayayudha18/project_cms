package handler

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// Vendor-scoping regression tests for the DSR upload handler (Phase 1.3 gap
// T5). These exercise the two pure-Go security controls that keep one vendor
// from touching another vendor's DSR data:
//   1. vendorContext: a non-vendor caller (LDAP karyawan, no VendorID claim)
//      is rejected with 403 before any DSR work happens.
//   2. Confirm's staged-filename guard: a vendor can only commit a staged file
//      whose <vendor_code>__ prefix matches their own resolved vendor code,
//      so a guessed/leaked filename for another vendor is rejected with 403.
//
// The parse/persist pipeline itself lives in backend_python/dsr and is covered
// there; these tests deliberately never reach a working service_dsr_etl.

// scopeFakeDsrRepo is a minimal DsrRepository: GetVendorByID resolves any id to
// a fixed vendor code so the handler's ResolveVendor step succeeds, and every
// other method returns empty/no-rows (never reached by these guard tests).
type scopeFakeDsrRepo struct {
	code string
	name string
}

func (f scopeFakeDsrRepo) GetVendorByID(context.Context, int64) (db.GetVendorByIDRow, error) {
	return db.GetVendorByIDRow{ID: 7, Code: f.code, Name: f.name}, nil
}

func (scopeFakeDsrRepo) GetDsrUploadByChecksum(context.Context, string) (db.GetDsrUploadByChecksumRow, error) {
	return db.GetDsrUploadByChecksumRow{}, pgx.ErrNoRows
}

func (scopeFakeDsrRepo) GetDsrUploadByIDForVendor(context.Context, db.GetDsrUploadByIDForVendorParams) (db.GetDsrUploadByIDForVendorRow, error) {
	return db.GetDsrUploadByIDForVendorRow{}, pgx.ErrNoRows
}

func (scopeFakeDsrRepo) ListDsrDailyRowErrors(context.Context, int64) ([]db.ListDsrDailyRowErrorsRow, error) {
	return nil, nil
}

func (scopeFakeDsrRepo) ListDsrRencanaIsiRowErrors(context.Context, int64) ([]db.ListDsrRencanaIsiRowErrorsRow, error) {
	return nil, nil
}

func (scopeFakeDsrRepo) ListDsrDailyRows(context.Context, int64) ([]db.ListDsrDailyRowsRow, error) {
	return nil, nil
}

func (scopeFakeDsrRepo) ListDsrRencanaIsiRows(context.Context, int64) ([]db.ListDsrRencanaIsiRowsRow, error) {
	return nil, nil
}

func (scopeFakeDsrRepo) ListDsrUploadsByVendor(context.Context, db.ListDsrUploadsByVendorParams) ([]db.ListDsrUploadsByVendorRow, error) {
	return nil, nil
}

func (scopeFakeDsrRepo) CountDsrUploadsByVendor(context.Context, db.CountDsrUploadsByVendorParams) (int64, error) {
	return 0, nil
}

// mountDsrUploadHandler wires the DSR handler exactly as cmd/api/main.go does:
// behind RequireAuth only, with vendor-scoping enforced inside the handler.
// retryBaseURL is intentionally unreachable so no test accidentally depends on
// a live service_dsr_etl; the guard paths return before it is ever called.
func mountDsrUploadHandler(vendorCode string) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})

	svc := service.NewDsrService(
		scopeFakeDsrRepo{code: vendorCode, name: "Vendor " + vendorCode},
		nil, // redis: unused on these paths (no rate-limit call on confirm/get)
		http.DefaultClient,
		"",                   // uploadDir unused
		"http://127.0.0.1:0", // unreachable service_dsr_etl
		"",
	)
	h := NewDsrUploadHandler(svc)
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(tokenSvc)).Mount("/api/v1/dsr", h.Routes())
	return r, tokenSvc
}

// tokenForVendor mints an access token carrying a VendorID claim, which is what
// the DSR handler's vendorContext requires (tokenForRole leaves VendorID nil,
// which would itself be a 403 "khusus vendor" -- exercised separately below).
func tokenForVendor(t *testing.T, ts *pkgauth.TokenService, userID, vendorID int64) string {
	t.Helper()
	vid := vendorID
	access, _, err := ts.GenerateTokenPair(&pkgauth.AuthIdentity{
		UserID:   userID,
		Username: "vendor.user",
		Role:     "VENDOR-USER",
		VendorID: &vid,
	})
	if err != nil {
		t.Fatalf("generate vendor token: %v", err)
	}
	return access
}

// T5a: an authenticated non-vendor caller (no VendorID claim) is forbidden,
// never reaching any DSR data.
func TestDsrUpload_NonVendorCaller_Forbidden(t *testing.T) {
	router, ts := mountDsrUploadHandler("BIJAK")

	// tokenForRole mints a karyawan-style token with VendorID == nil.
	rec := doRequest(router, http.MethodGet, "/api/v1/dsr/uploads/daily/1",
		tokenForRole(t, ts, 1, "ATM-USER"), "")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (non-vendor must be forbidden), body=%s",
			rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

// T5b: a vendor may not commit another vendor's staged file. The staged
// filename embeds <vendor_code>__<user_id>__<name>; confirming BARU__... while
// authenticated as BIJAK must be rejected by the vendor_code guard with 403,
// before any call to service_dsr_etl.
func TestDsrConfirm_OtherVendorStagedFile_Forbidden(t *testing.T) {
	router, ts := mountDsrUploadHandler("BIJAK")

	rec := doRequest(router, http.MethodPost, "/api/v1/dsr/uploads/confirm",
		tokenForVendor(t, ts, 1, 7),
		`{"staged_filename":"BARU__7__laporan.xlsx","checksum":"abc123"}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (cross-vendor commit must be forbidden), body=%s",
			rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

// T5c: the guard must NOT reject a vendor committing their OWN staged file.
// With the vendor_code prefix matching, control passes the guard and reaches
// the service, which then fails trying to call the unreachable service_dsr_etl
// -- surfacing as 502 confirm_failed, NOT 403. The point is purely that the
// scoping guard let the matching-vendor request through.
func TestDsrConfirm_OwnStagedFile_PassesGuard(t *testing.T) {
	router, ts := mountDsrUploadHandler("BIJAK")

	rec := doRequest(router, http.MethodPost, "/api/v1/dsr/uploads/confirm",
		tokenForVendor(t, ts, 1, 7),
		`{"staged_filename":"BIJAK__7__laporan.xlsx","checksum":"abc123"}`)

	if rec.Code == http.StatusForbidden {
		t.Fatalf("own-vendor commit was wrongly forbidden by the scoping guard, body=%s",
			rec.Body.String())
	}
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d (guard passed, then unreachable service_dsr_etl), body=%s",
			rec.Code, http.StatusBadGateway, rec.Body.String())
	}
}
