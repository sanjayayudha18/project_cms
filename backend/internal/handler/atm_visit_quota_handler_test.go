package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// atm-visit-quota (plan "Tests" — handler): route role gates and
// service-error -> HTTP status mapping, with a fake service.

type fakeAtmVisitQuotaServicer struct {
	view       *service.AtmVisitQuotaView
	reset      *service.VendorQuotaResetResult
	err        error
	lastReason string
}

func (f *fakeAtmVisitQuotaServicer) Get(context.Context, string) (*service.AtmVisitQuotaView, error) {
	return f.view, f.err
}
func (f *fakeAtmVisitQuotaServicer) ResetATM(context.Context, service.Actor, string) (*service.AtmVisitQuotaView, error) {
	return f.view, f.err
}
func (f *fakeAtmVisitQuotaServicer) ResetVendor(context.Context, service.Actor, int64) (*service.VendorQuotaResetResult, error) {
	return f.reset, f.err
}
func (f *fakeAtmVisitQuotaServicer) CancelVisit(_ context.Context, _ service.Actor, _ int64, reason string) error {
	f.lastReason = reason
	return f.err
}

func mountAtmVisitQuota(t *testing.T, svc AtmVisitQuotaServicer) (http.Handler, *pkgauth.TokenService) {
	t.Helper()
	ts := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, vendorRequestNoopBlacklist{})
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(ts)).Mount("/api/v1/atm-visit-quotas", NewAtmVisitQuotaHandler(svc).Routes())
	return r, ts
}

func TestAtmVisitQuotaHandler_RoleGates(t *testing.T) {
	sisa := int32(5)
	svc := &fakeAtmVisitQuotaServicer{
		view:  &service.AtmVisitQuotaView{TerminalID: "T1", HasQuota: true, QuotaTotal: 5, Remaining: 4, Sisa: 4, CurrentQuota: &sisa},
		reset: &service.VendorQuotaResetResult{ResetCount: 2, SkippedCount: 1},
	}
	router, ts := mountAtmVisitQuota(t, svc)
	user := vendorRequestTokenFor(t, ts, 1, "ATM-USER")
	spv := vendorRequestTokenFor(t, ts, 2, "ATM-SPV")

	cases := []struct {
		name, method, path, token, body string
		want                            int
	}{
		{"viewer reads", http.MethodGet, "/api/v1/atm-visit-quotas/atms/T1", user, "", http.StatusOK},
		{"ATM-USER cannot reset", http.MethodPost, "/api/v1/atm-visit-quotas/atms/T1/reset", user, "", http.StatusForbidden},
		{"ATM-USER cannot reset vendor", http.MethodPost, "/api/v1/atm-visit-quotas/vendors/7/reset", user, "", http.StatusForbidden},
		{"ATM-USER cannot cancel", http.MethodPost, "/api/v1/atm-visit-quotas/visits/9/cancel", user, `{"reason":"x"}`, http.StatusForbidden},
		{"SPV resets", http.MethodPost, "/api/v1/atm-visit-quotas/atms/T1/reset", spv, "", http.StatusOK},
		{"SPV resets vendor", http.MethodPost, "/api/v1/atm-visit-quotas/vendors/7/reset", spv, "", http.StatusOK},
		{"bad vendor id", http.MethodPost, "/api/v1/atm-visit-quotas/vendors/abc/reset", spv, "", http.StatusBadRequest},
		{"SPV cancels", http.MethodPost, "/api/v1/atm-visit-quotas/visits/9/cancel", spv, `{"reason":"salah"}`, http.StatusOK},
		{"no token", http.MethodGet, "/api/v1/atm-visit-quotas/atms/T1", "", "", http.StatusUnauthorized},
	}
	for _, c := range cases {
		if rec := vendorRequestDoRequest(router, c.method, c.path, c.token, c.body); rec.Code != c.want {
			t.Errorf("%s: status %d, want %d (%s)", c.name, rec.Code, c.want, rec.Body.String())
		}
	}

	rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/atm-visit-quotas/vendors/7/reset", spv, "")
	var body map[string]int
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["reset_count"] != 2 || body["skipped_count"] != 1 {
		t.Errorf("vendor reset body = %v", body)
	}
	if svc.lastReason != "salah" {
		t.Errorf("cancel reason = %q, want salah", svc.lastReason)
	}
}

func TestAtmVisitQuotaHandler_ErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{service.ErrAtmNotFound, http.StatusNotFound},
		{service.ErrVisitNotFound, http.StatusNotFound},
		{service.ErrQuotaUnknown, http.StatusUnprocessableEntity},
		{service.ErrVisitNotCancelable, http.StatusConflict},
		{service.ErrNotChecker, http.StatusForbidden},
		{service.ErrCancelVisitReason, http.StatusUnprocessableEntity}, // writeValidationError convention
	}
	for _, c := range cases {
		router, ts := mountAtmVisitQuota(t, &fakeAtmVisitQuotaServicer{err: c.err})
		spv := vendorRequestTokenFor(t, ts, 2, "ATM-SPV")
		if rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/atm-visit-quotas/visits/9/cancel", spv, `{"reason":"x"}`); rec.Code != c.want {
			t.Errorf("%v: status %d, want %d", c.err, rec.Code, c.want)
		}
	}
}

func TestVendorRequestHandler_CompletionRoutes(t *testing.T) {
	detail := &service.VendorRequestDetail{ID: 5, Status: "completed"}
	svc := &fakeVendorRequestServicer{completionResult: detail, overQuota: []string{"T9"}}
	router, ts := mountVendorRequestHandler(t, svc)
	user := vendorRequestTokenFor(t, ts, 1, "ATM-USER")
	spv := vendorRequestTokenFor(t, ts, 2, "ATM-SPV")

	rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/5/complete", user,
		`{"results":[{"terminal_id":"T1","result":"success"},{"terminal_id":"T2","result":"failed"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete: %d %s", rec.Code, rec.Body.String())
	}
	if len(svc.lastCompletionInput) != 2 || svc.lastCompletionInput[1].Result != "failed" {
		t.Errorf("parsed results = %+v", svc.lastCompletionInput)
	}
	if rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/5/complete", spv, `{"results":[]}`); rec.Code != http.StatusForbidden {
		t.Errorf("SPV complete: %d, want 403", rec.Code)
	}
	if rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/5/complete/approve", user, ""); rec.Code != http.StatusForbidden {
		t.Errorf("ATM-USER approve: %d, want 403", rec.Code)
	}

	rec = vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/5/complete/approve", spv, "")
	var body struct {
		Status             string   `json:"status"`
		OverQuotaTerminals []string `json:"over_quota_terminals"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusOK || len(body.OverQuotaTerminals) != 1 || body.OverQuotaTerminals[0] != "T9" {
		t.Errorf("approve: %d %+v", rec.Code, body)
	}
	if rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/5/complete/reject", spv, `{"reason":"kurang"}`); rec.Code != http.StatusOK {
		t.Errorf("reject: %d", rec.Code)
	}

	svc.completionErr = service.ErrSelfApproval
	if rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/5/complete/approve", spv, ""); rec.Code != http.StatusForbidden {
		t.Errorf("self approval: %d, want 403", rec.Code)
	}
	// review N1: empty reject-report reason names the body field "reason".
	svc.completionErr = service.ErrCompletionReasonEmpty
	rec = vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/5/complete/reject", spv, `{"reason":""}`)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"reason"`) {
		t.Errorf("empty completion reason: %d %s, want 422 naming field reason", rec.Code, rec.Body.String())
	}
	svc.completionErr = &service.ValidationError{Field: "results", Message: "x"}
	if rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/5/complete", user, `{"results":[]}`); rec.Code != http.StatusBadRequest {
		t.Errorf("validation: %d, want 400", rec.Code)
	}
}
