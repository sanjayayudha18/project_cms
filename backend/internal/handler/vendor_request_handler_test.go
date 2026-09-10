package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// -- test double ------------------------------------------------------------

// fakeVendorRequestServicer implements service.VendorRequestServicer with
// one configurable result/error per method, mirroring
// fakeApprovalOrchestrator's pattern in approval_handler_test.go.
type fakeVendorRequestServicer struct {
	browseResult  *service.BrowseForecastResult
	browseErr     error
	createResult  *service.VendorRequestDetail
	createErr     error
	updateResult  *service.VendorRequestDetail
	updateErr     error
	submitResult  *service.VendorRequestDetail
	submitErr     error
	approveResult *service.VendorRequestDetail
	approveErr    error
	rejectResult  *service.VendorRequestDetail
	rejectErr     error
	reviseResult  *service.VendorRequestDetail
	reviseErr     error
	cancelResult  *service.VendorRequestDetail
	cancelErr     error
	listResult    *service.ListVendorRequestResult
	listErr       error
	getResult     *service.VendorRequestDetail
	getErr        error
	auditResult   []service.AuditEntry
	auditErr      error
}

func (f *fakeVendorRequestServicer) BrowseForecast(context.Context, service.BrowseForecastParams) (*service.BrowseForecastResult, error) {
	return f.browseResult, f.browseErr
}
func (f *fakeVendorRequestServicer) Create(context.Context, service.Actor, service.CreateVendorRequestInput) (*service.VendorRequestDetail, error) {
	return f.createResult, f.createErr
}
func (f *fakeVendorRequestServicer) UpdateItems(context.Context, service.Actor, int64, []service.ItemInput) (*service.VendorRequestDetail, error) {
	return f.updateResult, f.updateErr
}
func (f *fakeVendorRequestServicer) Submit(context.Context, service.Actor, int64) (*service.VendorRequestDetail, error) {
	return f.submitResult, f.submitErr
}
func (f *fakeVendorRequestServicer) Approve(context.Context, service.Actor, int64) (*service.VendorRequestDetail, error) {
	return f.approveResult, f.approveErr
}
func (f *fakeVendorRequestServicer) Reject(context.Context, service.Actor, int64, string) (*service.VendorRequestDetail, error) {
	return f.rejectResult, f.rejectErr
}
func (f *fakeVendorRequestServicer) Revise(context.Context, service.Actor, int64) (*service.VendorRequestDetail, error) {
	return f.reviseResult, f.reviseErr
}
func (f *fakeVendorRequestServicer) Cancel(context.Context, service.Actor, int64) (*service.VendorRequestDetail, error) {
	return f.cancelResult, f.cancelErr
}
func (f *fakeVendorRequestServicer) List(context.Context, service.ListVendorRequestParams) (*service.ListVendorRequestResult, error) {
	return f.listResult, f.listErr
}
func (f *fakeVendorRequestServicer) Get(context.Context, int64) (*service.VendorRequestDetail, error) {
	return f.getResult, f.getErr
}
func (f *fakeVendorRequestServicer) AuditLog(context.Context, int64) ([]service.AuditEntry, error) {
	return f.auditResult, f.auditErr
}

// -- harness ------------------------------------------------------------

func mountVendorRequestHandler(t *testing.T, svc service.VendorRequestServicer) (http.Handler, *pkgauth.TokenService) {
	t.Helper()
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}, vendorRequestNoopBlacklist{})

	h := NewVendorRequestHandler(svc)
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(tokenSvc)).Mount("/api/v1/vendor-requests", h.Routes())
	return r, tokenSvc
}

type vendorRequestNoopBlacklist struct{}

func (vendorRequestNoopBlacklist) Add(_ context.Context, _ string, _ time.Duration) error { return nil }
func (vendorRequestNoopBlacklist) IsBlacklisted(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func vendorRequestTokenFor(t *testing.T, ts *pkgauth.TokenService, userID int64, role string) string {
	t.Helper()
	access, _, err := ts.GenerateTokenPair(&pkgauth.AuthIdentity{
		UserID:     userID,
		Username:   "user",
		Role:       role,
		IsKaryawan: true,
	})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return access
}

func vendorRequestDoRequest(router http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// -- tests ------------------------------------------------------------

func TestVendorRequestHandler_Get_HappyPath(t *testing.T) {
	svc := &fakeVendorRequestServicer{
		getResult: &service.VendorRequestDetail{
			ID: 1, RequestNumber: "VR-20260912-0001", Status: "draft",
			CreatedBy: service.UserRef{ID: 5, FullName: "Budi"},
		},
	}
	router, ts := mountVendorRequestHandler(t, svc)

	rec := vendorRequestDoRequest(router, http.MethodGet, "/api/v1/vendor-requests/1",
		vendorRequestTokenFor(t, ts, 5, "ATM-USER"), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var body vendorRequestDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ID != 1 || body.RequestNumber != "VR-20260912-0001" {
		t.Errorf("unexpected body: %+v", body)
	}
}

func TestVendorRequestHandler_Get_NotFound(t *testing.T) {
	svc := &fakeVendorRequestServicer{getErr: service.ErrNotFound}
	router, ts := mountVendorRequestHandler(t, svc)

	rec := vendorRequestDoRequest(router, http.MethodGet, "/api/v1/vendor-requests/999",
		vendorRequestTokenFor(t, ts, 5, "ATM-USER"), "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", rec.Code, rec.Body.String())
	}
}

func TestVendorRequestHandler_Get_InvalidID(t *testing.T) {
	router, ts := mountVendorRequestHandler(t, &fakeVendorRequestServicer{})

	rec := vendorRequestDoRequest(router, http.MethodGet, "/api/v1/vendor-requests/not-a-number",
		vendorRequestTokenFor(t, ts, 5, "ATM-USER"), "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
}

func TestVendorRequestHandler_Submit_InvalidTransitionConflict(t *testing.T) {
	svc := &fakeVendorRequestServicer{submitErr: service.ErrInvalidTransition}
	router, ts := mountVendorRequestHandler(t, svc)

	rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/1/submit",
		vendorRequestTokenFor(t, ts, 5, "ATM-USER"), "")

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body=%s", rec.Code, rec.Body.String())
	}
}

func TestVendorRequestHandler_Approve_SelfApprovalForbidden(t *testing.T) {
	svc := &fakeVendorRequestServicer{approveErr: service.ErrSelfApproval}
	router, ts := mountVendorRequestHandler(t, svc)

	rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/1/approve",
		vendorRequestTokenFor(t, ts, 5, "ATM-SPV"), "")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body=%s", rec.Code, rec.Body.String())
	}
}

// Approve requires a Checker role (ATM-SPV/BRANCH-ATM-SPV/ADMIN) at the
// route level (Req 6.9) — a Maker-only token must be rejected by
// RequireRoles before the handler/service ever runs.
func TestVendorRequestHandler_Approve_WrongRoleForbidden(t *testing.T) {
	svc := &fakeVendorRequestServicer{approveResult: &service.VendorRequestDetail{ID: 1, Status: "approved"}}
	router, ts := mountVendorRequestHandler(t, svc)

	rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/1/approve",
		vendorRequestTokenFor(t, ts, 5, "ATM-USER"), "")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body=%s", rec.Code, rec.Body.String())
	}
}

func TestVendorRequestHandler_Reject_EmptyReasonValidationError(t *testing.T) {
	svc := &fakeVendorRequestServicer{rejectErr: service.ErrRejectReasonEmpty}
	router, ts := mountVendorRequestHandler(t, svc)

	rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/1/reject",
		vendorRequestTokenFor(t, ts, 5, "ATM-SPV"), `{"rejection_reason":""}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422, body=%s", rec.Code, rec.Body.String())
	}
}

func TestVendorRequestHandler_Cancel_UnauthorizedForbidden(t *testing.T) {
	svc := &fakeVendorRequestServicer{cancelErr: service.ErrNotAuthorized}
	router, ts := mountVendorRequestHandler(t, svc)

	// Cancel route accepts both maker and checker roles (union rule); this
	// token belongs to a maker who is neither the creator nor a checker for
	// this request, so the service itself rejects it (403), not the route.
	rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/1/cancel",
		vendorRequestTokenFor(t, ts, 5, "ATM-USER"), "")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body=%s", rec.Code, rec.Body.String())
	}
}

func TestVendorRequestHandler_List_BadPageParam(t *testing.T) {
	router, ts := mountVendorRequestHandler(t, &fakeVendorRequestServicer{})

	rec := vendorRequestDoRequest(router, http.MethodGet, "/api/v1/vendor-requests?page=abc",
		vendorRequestTokenFor(t, ts, 5, "ATM-USER"), "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
}

func TestVendorRequestHandler_AuditLog_RequiresCheckerOrAdmin(t *testing.T) {
	svc := &fakeVendorRequestServicer{auditResult: []service.AuditEntry{{ID: 1, Action: "create"}}}
	router, ts := mountVendorRequestHandler(t, svc)

	// Viewer-only role (ATM-USER, a maker) must be rejected per Req 16.5.
	rec := vendorRequestDoRequest(router, http.MethodGet, "/api/v1/vendor-requests/1/audit-log",
		vendorRequestTokenFor(t, ts, 5, "ATM-USER"), "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body=%s", rec.Code, rec.Body.String())
	}

	rec = vendorRequestDoRequest(router, http.MethodGet, "/api/v1/vendor-requests/1/audit-log",
		vendorRequestTokenFor(t, ts, 5, "ATM-SPV"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
}

func TestVendorRequestHandler_Unauthenticated(t *testing.T) {
	router, _ := mountVendorRequestHandler(t, &fakeVendorRequestServicer{})

	rec := vendorRequestDoRequest(router, http.MethodGet, "/api/v1/vendor-requests/1", "", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body=%s", rec.Code, rec.Body.String())
	}
}
