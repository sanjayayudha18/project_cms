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

// fakeVaultPlanServicer exercises HTTP wiring only; logic is covered by
// internal/service/vault_plan_integration_test.go.
type fakeVaultPlanServicer struct {
	err        error
	lastReason string
	lastUrgent bool
}

func (f *fakeVaultPlanServicer) detail() (*service.VaultPlanDetail, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &service.VaultPlanDetail{Plan: db.GetVaultPlanRow{ID: 1, Status: "draft", RequestNumber: "REP-X"},
		Atms: []service.VaultPlanAtm{{TerminalID: "T1", Order: map[string]string{"100000": "1000000"}}}}, nil
}
func (f *fakeVaultPlanServicer) List(context.Context, service.Actor, service.VaultPlanListFilter) ([]db.ListVaultPlansRow, error) {
	return []db.ListVaultPlansRow{{ID: 1, RequestNumber: "REP-X", Status: "draft"}}, f.err
}
func (f *fakeVaultPlanServicer) Get(context.Context, service.Actor, int64) (*service.VaultPlanDetail, error) {
	return f.detail()
}
func (f *fakeVaultPlanServicer) Candidates(_ context.Context, _ service.Actor, _ int64, _ string, urgent bool) ([]service.VaultCandidate, error) {
	f.lastUrgent = urgent
	return []service.VaultCandidate{{VendorBranchID: 9, Tier: 1, Capacity: map[string]string{"100000": "500000"}}}, f.err
}
func (f *fakeVaultPlanServicer) SaveAssignments(context.Context, service.Actor, int64, []service.VaultAssignmentInput) (*service.VaultPlanDetail, error) {
	return f.detail()
}
func (f *fakeVaultPlanServicer) Submit(context.Context, service.Actor, int64) (*service.VaultPlanDetail, error) {
	return f.detail()
}
func (f *fakeVaultPlanServicer) Approve(context.Context, service.Actor, int64) (*service.VaultPlanDetail, error) {
	return f.detail()
}
func (f *fakeVaultPlanServicer) Reject(_ context.Context, _ service.Actor, _ int64, reason string) (*service.VaultPlanDetail, error) {
	f.lastReason = reason
	return f.detail()
}
func (f *fakeVaultPlanServicer) Audit(context.Context, service.Actor, int64) ([]service.AuditEntry, error) {
	return nil, f.err
}

func TestVaultPlanHandler(t *testing.T) {
	const base = "/api/v1/vault-plans"
	cases := []struct {
		name, role, method, path, body string
		svcErr                         error
		wantCode                       int
		wantBody                       string
	}{
		{"list", "ACM-USER", http.MethodGet, base + "?status=draft&from=2026-10-01", "", nil, http.StatusOK, `"request_number":"REP-X"`},
		{"bad date", "ACM-USER", http.MethodGet, base + "?from=01-10-2026", "", nil, http.StatusBadRequest, ""},
		{"detail with money as strings", "ACM-SPV", http.MethodGet, base + "/1", "", nil, http.StatusOK, `"100000":"1000000"`},
		{"ADMIN reads", "ADMIN", http.MethodGet, base + "/1", "", nil, http.StatusOK, ""},
		{"ATM-USER denied at route", "ATM-USER", http.MethodGet, base, "", nil, http.StatusForbidden, ""},
		{"candidates need terminal", "ACM-USER", http.MethodGet, base + "/1/candidates", "", nil, http.StatusUnprocessableEntity, "terminal_id"},
		{"candidates", "ACM-USER", http.MethodGet, base + "/1/candidates?terminal_id=T1&urgent=true", "", nil, http.StatusOK, `"tier":1`},
		{"save", "ACM-USER", http.MethodPut, base + "/1/assignments", `{"assignments":[{"terminal_id":"T1","vault_branch_id":9}]}`, nil, http.StatusOK, ""},
		{"non-member 404", "ACM-USER", http.MethodGet, base + "/1", "", service.ErrVaultPlanNotFound, http.StatusNotFound, ""},
		{"wrong role 403", "ADMIN", http.MethodPost, base + "/1/submit", "", service.ErrNotAuthorized, http.StatusForbidden, ""},
		{"four-eyes 403", "ACM-SPV", http.MethodPost, base + "/1/approve", "", service.ErrSelfApproval, http.StatusForbidden, ""},
		{"wrong state 409", "ACM-SPV", http.MethodPost, base + "/1/approve", "", service.ErrInvalidTransition, http.StatusConflict, ""},
		{"reject needs reason", "ACM-SPV", http.MethodPost, base + "/1/reject", `{}`, service.ErrRejectReasonEmpty, http.StatusUnprocessableEntity, "rejection_reason"},
		{"validation names field", "ACM-USER", http.MethodPut, base + "/1/assignments", `{"assignments":[]}`, &service.ValidationError{Field: "assignments", Message: "x"}, http.StatusUnprocessableEntity, "assignments"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
				SecretKey: []byte("test-secret-minimum-32-bytes-long!!"), AccessTokenExpiry: 15 * time.Minute, SessionMaxLifetime: time.Hour,
			}, noopBlacklist{})
			fake := &fakeVaultPlanServicer{err: tc.svcErr}
			r := chi.NewRouter()
			r.With(custommw.RequireAuth(tokenSvc), custommw.RequireRoles("ACM-USER", "ACM-SPV", "ADMIN")).
				Mount(base, NewVaultPlanHandler(fake).Routes())
			rec := doRequest(r, tc.method, tc.path, tokenForRole(t, tokenSvc, 1, tc.role), tc.body)
			if rec.Code != tc.wantCode {
				t.Fatalf("code = %d, want %d: %s", rec.Code, tc.wantCode, rec.Body.String())
			}
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Errorf("body %s does not contain %s", rec.Body.String(), tc.wantBody)
			}
		})
	}
}
