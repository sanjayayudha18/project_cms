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
	err             error
	lastReason      string
	lastUrgent      bool
	lastAllocations []service.AllocationLineInput
	lastBuffer      *string
}

func (f *fakeVaultPlanServicer) detail() (*service.VaultPlanDetail, error) {
	if f.err != nil {
		return nil, f.err
	}
	ticket := "V-21_20261014_001"
	return &service.VaultPlanDetail{Plan: db.GetVaultPlanRow{ID: 1, Status: "draft", RequestNumber: "REP-X"},
		Atms:        []service.VaultPlanAtm{{TerminalID: "T1", Order: map[string]string{"100000": "1000000"}}},
		OrderTotals: map[string]string{"100000": "1000000"},
		Allocations: []service.VaultAllocationView{{VendorVaultID: 21, VaultCode: "V-21", Amounts: map[string]string{"100000": "1000000"}, TicketNumber: &ticket}},
		Tickets:     []db.ListVaultTicketsForRequestRow{{VendorVaultID: 21, VaultCode: "V-21", TicketNumber: ticket, IsActive: true}}}, nil
}
func (f *fakeVaultPlanServicer) List(context.Context, service.Actor, service.VaultPlanListFilter) ([]db.ListVaultPlansRow, error) {
	return []db.ListVaultPlansRow{{ID: 1, RequestNumber: "REP-X", Status: "draft"}}, f.err
}
func (f *fakeVaultPlanServicer) Get(context.Context, service.Actor, int64) (*service.VaultPlanDetail, error) {
	return f.detail()
}
func (f *fakeVaultPlanServicer) Candidates(_ context.Context, _ service.Actor, _ int64, urgent bool) ([]service.VaultBranchOption, error) {
	f.lastUrgent = urgent
	return []service.VaultBranchOption{{VendorBranchID: 9, Tier: 1, Vaults: []service.VaultOption{
		{VendorVaultID: 21, VaultCode: "V-21", Tier: 1, Capacity: map[string]string{"100000": "500000"}}}}}, f.err
}
func (f *fakeVaultPlanServicer) SaveAllocations(_ context.Context, _ service.Actor, _ int64, in []service.AllocationLineInput, buffer *string) (*service.VaultPlanDetail, error) {
	f.lastAllocations, f.lastBuffer = in, buffer
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
		{"detail has allocations + tickets", "ACM-USER", http.MethodGet, base + "/1", "", nil, http.StatusOK, `"ticket_number":"V-21_20261014_001"`},
		{"detail has order totals", "ACM-USER", http.MethodGet, base + "/1", "", nil, http.StatusOK, `"order_totals":{"100000":"1000000"}`},
		{"candidates grouped by branch", "ACM-USER", http.MethodGet, base + "/1/candidates?urgent=true", "", nil, http.StatusOK, `"vaults":[{"vendor_vault_id":21`},
		{"save", "ACM-USER", http.MethodPut, base + "/1/allocations", `{"allocations":[{"vendor_vault_id":21,"amounts":{"100000":"1000000"}}],"buffer_reason":null}`, nil, http.StatusOK, ""},
		{"save bad body", "ACM-USER", http.MethodPut, base + "/1/allocations", `{bad`, nil, http.StatusBadRequest, ""},
		{"old assignments route gone", "ACM-USER", http.MethodPut, base + "/1/assignments", `{}`, nil, http.StatusNotFound, ""},
		{"non-member 404", "ACM-USER", http.MethodGet, base + "/1", "", service.ErrVaultPlanNotFound, http.StatusNotFound, ""},
		{"wrong role 403", "ADMIN", http.MethodPost, base + "/1/submit", "", service.ErrNotAuthorized, http.StatusForbidden, ""},
		{"four-eyes 403", "ACM-SPV", http.MethodPost, base + "/1/approve", "", service.ErrSelfApproval, http.StatusForbidden, ""},
		{"wrong state 409", "ACM-SPV", http.MethodPost, base + "/1/approve", "", service.ErrInvalidTransition, http.StatusConflict, ""},
		{"reject needs reason", "ACM-SPV", http.MethodPost, base + "/1/reject", `{}`, service.ErrRejectReasonEmpty, http.StatusUnprocessableEntity, "rejection_reason"},
		{"validation names field", "ACM-USER", http.MethodPut, base + "/1/allocations", `{"allocations":[]}`, &service.ValidationError{Field: "allocations", Message: "x"}, http.StatusUnprocessableEntity, "allocations"},
		{"audit", "ACM-SPV", http.MethodGet, base + "/1/audit", "", nil, http.StatusOK, ""},
		{"audit non-member 404", "ACM-USER", http.MethodGet, base + "/1/audit", "", service.ErrVaultPlanNotFound, http.StatusNotFound, ""},
		{"audit bad id", "ACM-USER", http.MethodGet, base + "/x/audit", "", nil, http.StatusBadRequest, ""},
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
