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

// fakeVaultReviewer stands in for service.VaultPlanService on the ATM-SPV review routes.
type fakeVaultReviewer struct {
	review     *service.RequestVaultReview
	err        error
	gotApprove bool
	gotReason  string
	gotActor   service.Actor
}

func (f *fakeVaultReviewer) ForRequest(context.Context, int64) (*service.RequestVaultReview, error) {
	return f.review, f.err
}

func (f *fakeVaultReviewer) ReviewRequest(_ context.Context, actor service.Actor, _ int64, approve bool, reason string) error {
	f.gotActor, f.gotApprove, f.gotReason = actor, approve, reason
	return f.err
}

func mountVaultReviewHandler(t *testing.T, vault *fakeVaultReviewer) (http.Handler, *pkgauth.TokenService) {
	t.Helper()
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, vendorRequestNoopBlacklist{})
	svc := &fakeVendorRequestServicer{getResult: &service.VendorRequestDetail{ID: 1, Status: "sent_to_vendor"}}
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(tokenSvc)).Mount("/api/v1/vendor-requests", NewVendorRequestHandler(svc).WithVaultReview(vault).Routes())
	return r, tokenSvc
}

func TestVendorRequestVault_Assignments(t *testing.T) {
	vault := &fakeVaultReviewer{review: &service.RequestVaultReview{
		Plans: []db.ListVaultPlansForRequestRow{{ID: 4, AcmAreaID: 2, AcmAreaName: "Area Jakarta", Status: "acm_approved"}},
		Assignments: []db.ListVaultAssignmentsForRequestRow{
			{VaultPlanID: 4, TerminalID: "T001", Tier: 1, SaldoSnapshot: []byte(`{"100000":"1500000"}`)},
			{VaultPlanID: 4, TerminalID: "T002", Tier: 3, IsUrgent: true}, // no snapshot -> null
		},
	}}
	router, ts := mountVaultReviewHandler(t, vault)
	viewer := vendorRequestTokenFor(t, ts, 9, "ATM-USER")

	rec := doRequest(router, http.MethodGet, "/api/v1/vendor-requests/1/vault-assignments", viewer, "")
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"acm_area_name":"Area Jakarta"`) ||
		!strings.Contains(body, `"saldo_snapshot":{"100000":"1500000"}`) || !strings.Contains(body, `"saldo_snapshot":null`) {
		t.Fatalf("assignments = %d %s", rec.Code, body)
	}
	if rec := doRequest(router, http.MethodGet, "/api/v1/vendor-requests/x/vault-assignments", viewer, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("bad id = %d, want 400", rec.Code)
	}
	vault.err = service.ErrNotFound
	if rec := doRequest(router, http.MethodGet, "/api/v1/vendor-requests/1/vault-assignments", viewer, ""); rec.Code != http.StatusNotFound {
		t.Errorf("missing request = %d, want 404", rec.Code)
	}
}

func TestVendorRequestVault_ApproveAndReject(t *testing.T) {
	vault := &fakeVaultReviewer{}
	router, ts := mountVaultReviewHandler(t, vault)
	spv := vendorRequestTokenFor(t, ts, 9, "ATM-SPV")

	rec := doRequest(router, http.MethodPost, "/api/v1/vendor-requests/1/vault-approve", spv, "")
	if rec.Code != http.StatusOK || !vault.gotApprove || vault.gotActor.UserID != 9 || !strings.Contains(rec.Body.String(), `"sent_to_vendor"`) {
		t.Fatalf("approve = %d %s (approve=%v actor=%+v)", rec.Code, rec.Body.String(), vault.gotApprove, vault.gotActor)
	}
	rec = doRequest(router, http.MethodPost, "/api/v1/vendor-requests/1/vault-reject", spv, `{"vault_rejection_reason":"kapasitas vault kurang"}`)
	if rec.Code != http.StatusOK || vault.gotApprove || vault.gotReason != "kapasitas vault kurang" {
		t.Fatalf("reject = %d (approve=%v reason=%q)", rec.Code, vault.gotApprove, vault.gotReason)
	}
	if rec := doRequest(router, http.MethodPost, "/api/v1/vendor-requests/1/vault-reject", spv, `{bad`); rec.Code != http.StatusBadRequest {
		t.Errorf("bad body = %d, want 400", rec.Code)
	}
	if rec := doRequest(router, http.MethodPost, "/api/v1/vendor-requests/x/vault-approve", spv, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("bad id = %d, want 400", rec.Code)
	}

	// Makers cannot review (route RBAC); service-level four-eyes / state errors map to 403 / 409 / 422.
	if rec := doRequest(router, http.MethodPost, "/api/v1/vendor-requests/1/vault-approve", vendorRequestTokenFor(t, ts, 9, "ATM-USER"), ""); rec.Code != http.StatusForbidden {
		t.Errorf("ATM-USER approve = %d, want 403", rec.Code)
	}
	for err, want := range map[error]int{
		service.ErrSelfApproval:      http.StatusForbidden,
		service.ErrNotChecker:        http.StatusForbidden,
		service.ErrInvalidTransition: http.StatusConflict,
		service.ErrRejectReasonEmpty: http.StatusUnprocessableEntity,
	} {
		vault.err = err
		if rec := doRequest(router, http.MethodPost, "/api/v1/vendor-requests/1/vault-reject", spv, `{"vault_rejection_reason":""}`); rec.Code != want {
			t.Errorf("err %v = %d, want %d", err, rec.Code, want)
		}
	}
}
