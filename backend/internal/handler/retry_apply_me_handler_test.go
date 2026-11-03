package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/approval"
	"github.com/cimb-niaga/cms/backend/internal/db"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

type fakeApplyRetrier struct {
	err      error
	changeID int64
	actorID  int64
}

func (f *fakeApplyRetrier) RetryApply(_ context.Context, changeID, actorID int64, _ string) error {
	f.changeID, f.actorID = changeID, actorID
	return f.err
}

func mountRetryApply(reader ApprovalReader, retrier MasterDataApplyRetrier) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	h := NewApprovalHandler(&fakeApprovalOrchestrator{}, reader)
	if retrier != nil {
		h = h.WithApplyRetry(retrier)
	}
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(tokenSvc)).Mount("/api/v1/approvals", h.Routes())
	return r, tokenSvc
}

// Approved-but-not-applied master-data change: retry re-runs apply for the
// change behind the approval; anything else is a 409, never a silent re-apply.
func TestApprovalHandler_RetryApply(t *testing.T) {
	approved := db.ApprovalRequest{ID: 3, DocumentType: "master_data", DocumentID: 42, Status: "approved"}
	const path = "/api/v1/approvals/3/retry-apply"

	retrier := &fakeApplyRetrier{}
	router, ts := mountRetryApply(&fakeApprovalReader{getResult: approved}, retrier)
	if rec := doRequest(router, http.MethodPost, path, tokenFor(t, ts, 7), ""); rec.Code != http.StatusOK || retrier.changeID != 42 || retrier.actorID != 7 {
		t.Fatalf("retry = %d %s (change %d actor %d)", rec.Code, rec.Body.String(), retrier.changeID, retrier.actorID)
	}
	if rec := doRequest(router, http.MethodPost, "/api/v1/approvals/x/retry-apply", tokenFor(t, ts, 7), ""); rec.Code != http.StatusBadRequest {
		t.Errorf("bad id = %d, want 400", rec.Code)
	}
	retrier.err = approval.ErrNotAuthorized
	if rec := doRequest(router, http.MethodPost, path, tokenFor(t, ts, 7), ""); rec.Code != http.StatusForbidden {
		t.Errorf("retrier not authorized = %d, want 403", rec.Code)
	}

	for name, reader := range map[string]*fakeApprovalReader{
		"still pending":      {getResult: db.ApprovalRequest{ID: 3, DocumentType: "master_data", DocumentID: 42, Status: "pending"}},
		"not master data":    {getResult: db.ApprovalRequest{ID: 3, DocumentType: "invoice", DocumentID: 42, Status: "approved"}},
		"approval not found": {getErr: errors.New("no rows")},
	} {
		r := &fakeApplyRetrier{}
		router, ts := mountRetryApply(reader, r)
		if rec := doRequest(router, http.MethodPost, path, tokenFor(t, ts, 7), ""); rec.Code != http.StatusConflict || r.changeID != 0 {
			t.Errorf("%s = %d (retrier called: %v), want 409 without retry", name, rec.Code, r.changeID != 0)
		}
	}

	router, ts = mountRetryApply(&fakeApprovalReader{getResult: approved}, nil)
	if rec := doRequest(router, http.MethodPost, path, tokenFor(t, ts, 7), ""); rec.Code != http.StatusNotFound {
		t.Errorf("retry not wired = %d, want 404", rec.Code)
	}
}

type fakeMeRepo struct {
	pkgauth.UserRepository
	user *pkgauth.UserRecord
	err  error
}

func (f fakeMeRepo) GetUserProfile(context.Context, int64) (*pkgauth.UserRecord, error) {
	return f.user, f.err
}

func TestAuthHandler_Me(t *testing.T) {
	ts := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	vendorID := int64(9)
	vendorName, branchID, branchName := "PT Vendor Satu", int64(5), "Cabang Surabaya"
	cases := []struct {
		name     string
		repo     fakeMeRepo
		token    string
		wantCode int
		wantBody string
	}{
		{"profile", fakeMeRepo{user: &pkgauth.UserRecord{ID: 42, Username: "vendor.user", Role: "VENDOR-USER", VendorID: &vendorID}}, tokenFor(t, ts, 42), http.StatusOK, `"vendor_id":9`},
		{"vendor-wide", fakeMeRepo{user: &pkgauth.UserRecord{ID: 42, Role: "VENDOR-USER", VendorID: &vendorID, VendorName: &vendorName}}, tokenFor(t, ts, 42), http.StatusOK,
			`"vendor_name":"PT Vendor Satu","vendor_branch_id":null,"vendor_branch_name":null`},
		{"branch-scoped", fakeMeRepo{user: &pkgauth.UserRecord{ID: 42, Role: "VENDOR-ATM-USER", VendorID: &vendorID, VendorName: &vendorName, VendorBranchID: &branchID, VendorBranchName: &branchName}}, tokenFor(t, ts, 42), http.StatusOK,
			`"vendor_name":"PT Vendor Satu","vendor_branch_id":5,"vendor_branch_name":"Cabang Surabaya"`},
		{"internal", fakeMeRepo{user: &pkgauth.UserRecord{ID: 42, Role: "ADMIN"}}, tokenFor(t, ts, 42), http.StatusOK,
			`"vendor_name":null,"vendor_branch_id":null,"vendor_branch_name":null`},
		{"deleted user", fakeMeRepo{}, tokenFor(t, ts, 42), http.StatusUnauthorized, ""},
		{"db down", fakeMeRepo{err: errors.New("db down")}, tokenFor(t, ts, 42), http.StatusServiceUnavailable, ""},
		{"no token", fakeMeRepo{}, "", http.StatusUnauthorized, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := chi.NewRouter()
			r.Mount("/api/v1/auth", (&AuthHandler{tokenService: ts, userRepo: tc.repo}).Routes())
			rec := doRequest(r, http.MethodGet, "/api/v1/auth/me", tc.token, "")
			if rec.Code != tc.wantCode || !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Fatalf("me = %d %s, want %d containing %q", rec.Code, rec.Body.String(), tc.wantCode, tc.wantBody)
			}
			if tc.wantCode == http.StatusOK && strings.Contains(rec.Body.String(), "password") {
				t.Errorf("profile leaks password fields: %s", rec.Body.String())
			}
		})
	}
}
