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
	browseResult        *service.BrowseForecastResult
	browseErr           error
	createResult        *service.VendorRequestDetail
	createErr           error
	updateResult        *service.VendorRequestDetail
	updateErr           error
	submitResult        *service.VendorRequestDetail
	submitErr           error
	approveResult       *service.VendorRequestDetail
	approveErr          error
	rejectResult        *service.VendorRequestDetail
	rejectErr           error
	reviseResult        *service.VendorRequestDetail
	reviseErr           error
	cancelResult        *service.VendorRequestDetail
	cancelErr           error
	lastCancelReason    string
	listResult          *service.ListVendorRequestResult
	listErr             error
	getResult           *service.VendorRequestDetail
	getErr              error
	auditResult         []service.AuditEntry
	auditErr            error
	vendorOptionsResult *service.VendorOptionsResult
	vendorOptionsErr    error

	// CIT-2 (Task 8.3): capture the last received input to verify the
	// handler's request-body/query-param parsing, not just the response shape.
	lastCreateInput service.CreateVendorRequestInput
	lastListParams  service.ListVendorRequestParams
}

func (f *fakeVendorRequestServicer) BrowseForecast(context.Context, service.BrowseForecastParams) (*service.BrowseForecastResult, error) {
	return f.browseResult, f.browseErr
}
func (f *fakeVendorRequestServicer) ListVendorOptions(context.Context) (*service.VendorOptionsResult, error) {
	return f.vendorOptionsResult, f.vendorOptionsErr
}
func (f *fakeVendorRequestServicer) Create(_ context.Context, _ service.Actor, in service.CreateVendorRequestInput) (*service.VendorRequestDetail, error) {
	f.lastCreateInput = in
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
func (f *fakeVendorRequestServicer) Cancel(_ context.Context, _ service.Actor, _ int64, reason string) (*service.VendorRequestDetail, error) {
	f.lastCancelReason = reason
	return f.cancelResult, f.cancelErr
}
func (f *fakeVendorRequestServicer) List(_ context.Context, params service.ListVendorRequestParams) (*service.ListVendorRequestResult, error) {
	f.lastListParams = params
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
		SessionMaxLifetime: time.Hour,
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

// TestVendorRequestHandler_Reject_SelfApprovalForbidden covers Task 9 (Req
// 2.7): four-eyes applies to Reject exactly like Approve — a Checker who
// created the request is denied with ErrSelfApproval -> 403.
func TestVendorRequestHandler_Reject_SelfApprovalForbidden(t *testing.T) {
	svc := &fakeVendorRequestServicer{rejectErr: service.ErrSelfApproval}
	router, ts := mountVendorRequestHandler(t, svc)

	rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/1/reject",
		vendorRequestTokenFor(t, ts, 5, "ATM-SPV"), `{"rejection_reason":"data tidak sesuai"}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body=%s", rec.Code, rec.Body.String())
	}
}

// TestVendorRequestHandler_Reject_WrongRoleForbidden covers Task 9 (Req 2.8,
// 6.9): Reject requires a Checker role at the route level (vendorRequestCheckerRoles,
// gate route POST /{id}/reject) — a Maker-only token is rejected by
// RequireRoles before the handler/service ever runs, mirroring Approve's
// route gate.
func TestVendorRequestHandler_Reject_WrongRoleForbidden(t *testing.T) {
	svc := &fakeVendorRequestServicer{rejectResult: &service.VendorRequestDetail{ID: 1, Status: "rejected"}}
	router, ts := mountVendorRequestHandler(t, svc)

	rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/1/reject",
		vendorRequestTokenFor(t, ts, 5, "ATM-USER"), `{"rejection_reason":"data tidak sesuai"}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body=%s", rec.Code, rec.Body.String())
	}
}

// TestVendorRequestHandler_Reject_InvalidTransitionConflict covers Task 9
// (Req 2.6): rejecting a request outside pending_approval maps
// ErrInvalidTransition -> 409, matching Submit's behavior.
func TestVendorRequestHandler_Reject_InvalidTransitionConflict(t *testing.T) {
	svc := &fakeVendorRequestServicer{rejectErr: service.ErrInvalidTransition}
	router, ts := mountVendorRequestHandler(t, svc)

	rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/1/reject",
		vendorRequestTokenFor(t, ts, 5, "ATM-SPV"), `{"rejection_reason":"data tidak sesuai"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body=%s", rec.Code, rec.Body.String())
	}
}

// TestVendorRequestHandler_Reject_HappyPath covers Task 9 (Req 2.5): a valid
// reason on a rejectable request reaches the service and returns 200 with
// the rejected detail.
func TestVendorRequestHandler_Reject_HappyPath(t *testing.T) {
	svc := &fakeVendorRequestServicer{rejectResult: &service.VendorRequestDetail{ID: 1, Status: "rejected", RejectionReason: "data tidak sesuai"}}
	router, ts := mountVendorRequestHandler(t, svc)

	rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/1/reject",
		vendorRequestTokenFor(t, ts, 5, "ATM-SPV"), `{"rejection_reason":"data tidak sesuai"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Status          string `json:"status"`
		RejectionReason string `json:"rejection_reason"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "rejected" || body.RejectionReason != "data tidak sesuai" {
		t.Errorf("body = %+v, want status=rejected rejection_reason=%q", body, "data tidak sesuai")
	}
}

func TestVendorRequestHandler_Cancel_UnauthorizedForbidden(t *testing.T) {
	svc := &fakeVendorRequestServicer{cancelErr: service.ErrNotAuthorized}
	router, ts := mountVendorRequestHandler(t, svc)

	// Cancel route accepts both maker and checker roles (union rule); this
	// token belongs to a maker who is neither the creator nor a checker for
	// this request, so the service itself rejects it (403), not the route.
	rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/1/cancel",
		vendorRequestTokenFor(t, ts, 5, "ATM-USER"), `{"cancellation_reason":"tidak jadi diproses"}`)

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

// TestVendorRequestHandler_ListVendorOptions_HappyPath covers the new
// GET /vendors endpoint (CIT-2 Req 1.2, 1.3, 3 Q2), added during Task 10
// after finding no existing vendor-listing endpoint for the Forecast
// Browser's required FLM Vendor / FLM Vendor Region selects to call.
func TestVendorRequestHandler_ListVendorOptions_HappyPath(t *testing.T) {
	svc := &fakeVendorRequestServicer{
		vendorOptionsResult: &service.VendorOptionsResult{
			Vendors: []service.VendorOption{{ID: 1, Name: "TAG"}, {ID: 2, Name: "ROH"}},
			Regions: []string{"Jawa Barat", "Jawa Timur"},
		},
	}
	router, ts := mountVendorRequestHandler(t, svc)

	rec := vendorRequestDoRequest(router, http.MethodGet, "/api/v1/vendor-requests/vendors",
		vendorRequestTokenFor(t, ts, 5, "ATM-USER"), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var body vendorOptionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Vendors) != 2 || body.Vendors[0].ID != 1 || body.Vendors[0].Name != "TAG" {
		t.Errorf("vendors = %+v", body.Vendors)
	}
	if len(body.Regions) != 2 || body.Regions[0] != "Jawa Barat" {
		t.Errorf("regions = %+v", body.Regions)
	}
}

func TestVendorRequestHandler_Unauthenticated(t *testing.T) {
	router, _ := mountVendorRequestHandler(t, &fakeVendorRequestServicer{})

	rec := vendorRequestDoRequest(router, http.MethodGet, "/api/v1/vendor-requests/1", "", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body=%s", rec.Code, rec.Body.String())
	}
}

// TestVendorRequestHandler_BrowseForecast_IncludesNewContextFields covers
// Task 3 (.kiro/specs/update-cit-forecast-browser): the four new context
// columns must appear in the flat JSON response additively, alongside the
// existing fields and the unchanged pagination wrapper (Req 4.1).
func TestVendorRequestHandler_BrowseForecast_IncludesNewContextFields(t *testing.T) {
	svc := &fakeVendorRequestServicer{
		browseResult: &service.BrowseForecastResult{
			Data: []service.ForecastRow{
				{
					TerminalID:      "1234",
					PeriodePred:     time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC),
					Denom:           50000,
					AmountReplenish: 100000000,
					AmountRefund:    0,
					DmaaFileID:      1,
					LokasiATM:       "Test Location",
					Brand:           "Hyosung",
					FLMVendor:       "TAG",
					FLMVendorRegion: "TAG Jawa Barat",
				},
			},
			Total: 1, Page: 1, PageSize: 25, TotalPages: 1,
		},
	}
	router, ts := mountVendorRequestHandler(t, svc)

	rec := vendorRequestDoRequest(router, http.MethodGet,
		"/api/v1/vendor-requests/forecast?forecast_date=2027-01-15",
		vendorRequestTokenFor(t, ts, 5, "ATM-USER"), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var body forecastResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data) != 1 {
		t.Fatalf("expected 1 row, got %d", len(body.Data))
	}
	row := body.Data[0]
	if row.TerminalID != "1234" || row.AmountReplenish != 100000000 {
		t.Errorf("existing fields not preserved: %+v", row)
	}
	if row.LokasiATM != "Test Location" || row.Brand != "Hyosung" ||
		row.FLMVendor != "TAG" || row.FLMVendorRegion != "TAG Jawa Barat" {
		t.Errorf("new context fields missing/wrong: %+v", row)
	}
	if body.Pagination.Page != 1 || body.Pagination.PageSize != 25 ||
		body.Pagination.TotalCount != 1 || body.Pagination.TotalPages != 1 {
		t.Errorf("pagination wrapper changed unexpectedly: %+v", body.Pagination)
	}

	// Additive check at the wire level too: the raw JSON must carry both the
	// old and new keys, proving no rename/removal (Req 4.1).
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw: %v", err)
	}
	rawRow := raw["data"].([]any)[0].(map[string]any)
	for _, key := range []string{"terminal_id", "periode_pred", "denom", "amount_replenish", "amount_refund", "dmaa_file_id", "lokasi_atm", "brand", "flm_vendor", "flm_vendor_region"} {
		if _, ok := rawRow[key]; !ok {
			t.Errorf("response JSON missing key %q", key)
		}
	}
}

func TestVendorRequestHandler_BrowseForecast_RoleGate(t *testing.T) {
	router, ts := mountVendorRequestHandler(t, &fakeVendorRequestServicer{
		browseResult: &service.BrowseForecastResult{Data: []service.ForecastRow{}},
	})

	// 401: no token.
	rec := vendorRequestDoRequest(router, http.MethodGet, "/api/v1/vendor-requests/forecast", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: status = %d, want 401, body=%s", rec.Code, rec.Body.String())
	}

	// 403: authenticated but not in vendorRequestViewerRoles.
	rec = vendorRequestDoRequest(router, http.MethodGet, "/api/v1/vendor-requests/forecast",
		vendorRequestTokenFor(t, ts, 5, "VENDOR"), "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("wrong role: status = %d, want 403, body=%s", rec.Code, rec.Body.String())
	}

	// 200: ATM-USER is an allowed viewer role.
	rec = vendorRequestDoRequest(router, http.MethodGet, "/api/v1/vendor-requests/forecast",
		vendorRequestTokenFor(t, ts, 5, "ATM-USER"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("allowed role: status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
}

// TestVendorRequestHandler_Get_CIT2FieldsAreAdditive covers Task 8.3 (Req
// 6.1): replenish_date/request_category/is_canceled/is_manual must appear in
// the flat JSON response additively, alongside every existing field, with no
// rename/removal.
func TestVendorRequestHandler_Get_CIT2FieldsAreAdditive(t *testing.T) {
	category := "emergency"
	replenish := time.Date(2027, 1, 16, 0, 0, 0, 0, time.UTC)
	svc := &fakeVendorRequestServicer{
		getResult: &service.VendorRequestDetail{
			ID: 1, RequestNumber: "REPTAG20270115001", Status: "cancelled",
			CreatedBy:       service.UserRef{ID: 5, FullName: "Budi"},
			ReplenishDate:   &replenish,
			RequestCategory: &category,
			IsCanceled:      true,
			IsManual:        true,
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
	if body.ID != 1 || body.RequestNumber != "REPTAG20270115001" || body.Status != "cancelled" {
		t.Errorf("existing fields not preserved: %+v", body)
	}
	if body.ReplenishDate == nil || *body.ReplenishDate != "2027-01-16" {
		t.Errorf("replenish_date = %v, want \"2027-01-16\"", body.ReplenishDate)
	}
	if body.RequestCategory == nil || *body.RequestCategory != "emergency" {
		t.Errorf("request_category = %v, want \"emergency\"", body.RequestCategory)
	}
	if !body.IsCanceled || !body.IsManual {
		t.Errorf("is_canceled=%v is_manual=%v, want both true", body.IsCanceled, body.IsManual)
	}

	// Wire-level additive check: every old and new key present, nothing renamed/removed.
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw: %v", err)
	}
	for _, key := range []string{
		"id", "request_number", "forecast_date", "status", "notes", "created_by",
		"created_at", "updated_at", "items", "total_amount",
		"replenish_date", "request_category", "is_canceled", "is_manual",
	} {
		if _, ok := raw[key]; !ok {
			t.Errorf("response JSON missing key %q", key)
		}
	}
}

// TestVendorRequestHandler_Get_CIT2FieldsNullForLegacyRow covers Task 8.3:
// a legacy row (nil ReplenishDate/RequestCategory, is_manual=false) must
// serialize those fields as null/false, not omit them.
func TestVendorRequestHandler_Get_CIT2FieldsNullForLegacyRow(t *testing.T) {
	svc := &fakeVendorRequestServicer{
		getResult: &service.VendorRequestDetail{
			ID: 2, RequestNumber: "VR-20260720-0001", Status: "draft",
			CreatedBy: service.UserRef{ID: 5, FullName: "Budi"},
		},
	}
	router, ts := mountVendorRequestHandler(t, svc)

	rec := vendorRequestDoRequest(router, http.MethodGet, "/api/v1/vendor-requests/2",
		vendorRequestTokenFor(t, ts, 5, "ATM-USER"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var body vendorRequestDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ReplenishDate != nil || body.RequestCategory != nil || body.IsCanceled || body.IsManual {
		t.Errorf("legacy row should have null/false CIT-2 fields, got %+v", body)
	}
}

// TestVendorRequestHandler_Cancel_AlreadyCanceled_Returns409 covers Task 8.1
// (Req 5.7): ErrAlreadyCanceled maps to HTTP 409, matching the other
// conflict-class sentinels.
func TestVendorRequestHandler_Cancel_AlreadyCanceled_Returns409(t *testing.T) {
	svc := &fakeVendorRequestServicer{cancelErr: service.ErrAlreadyCanceled}
	router, ts := mountVendorRequestHandler(t, svc)

	rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/1/cancel",
		vendorRequestTokenFor(t, ts, 5, "ATM-USER"), `{"cancellation_reason":"tidak jadi diproses"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body=%s", rec.Code, rec.Body.String())
	}
}

// TestVendorRequestHandler_Cancel_NonCheckerOnApproved_Returns403 covers Task
// 8.7 (Req 3.7, 6.2): a Maker role clears the route's RequireRoles gate
// (vendorRequestCancelRoles allows both makers and checkers, since cancel is
// also reachable by a draft's creator), but the service itself denies
// canceling an approved request to anyone who isn't a Checker.
func TestVendorRequestHandler_Cancel_NonCheckerOnApproved_Returns403(t *testing.T) {
	svc := &fakeVendorRequestServicer{cancelErr: service.ErrNotChecker}
	router, ts := mountVendorRequestHandler(t, svc)

	rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/1/cancel",
		vendorRequestTokenFor(t, ts, 5, "ATM-USER"), `{"cancellation_reason":"tidak jadi diproses"}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body=%s", rec.Code, rec.Body.String())
	}
	if svc.lastCancelReason != "tidak jadi diproses" {
		t.Errorf("lastCancelReason = %q, want the request body's cancellation_reason to reach the service even on a denial", svc.lastCancelReason)
	}
}

// TestVendorRequestHandler_List_IncludeCanceledParam covers Task 8.1 (Req
// 5.5): include_canceled defaults to false and only becomes true when the
// query param is exactly "true".
func TestVendorRequestHandler_List_IncludeCanceledParam(t *testing.T) {
	svc := &fakeVendorRequestServicer{listResult: &service.ListVendorRequestResult{}}
	router, ts := mountVendorRequestHandler(t, svc)
	token := vendorRequestTokenFor(t, ts, 5, "ATM-USER")

	rec := vendorRequestDoRequest(router, http.MethodGet, "/api/v1/vendor-requests/", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if svc.lastListParams.IncludeCanceled {
		t.Error("default IncludeCanceled should be false")
	}

	rec = vendorRequestDoRequest(router, http.MethodGet, "/api/v1/vendor-requests/?include_canceled=true", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if !svc.lastListParams.IncludeCanceled {
		t.Error("include_canceled=true should set IncludeCanceled")
	}
}

// TestVendorRequestHandler_Create_ManualRequestParsing covers Task 8.1 (Req
// 2, 3, 4): a manual create body parses vendor_id/replenish_date/
// request_category/is_manual and per-item brand/lokasi_atm without
// requiring forecast_date or periode_pred (Q5: forecast_date anchors to
// today Jakarta for a manual request; a manual item has no DMAA periode).
func TestVendorRequestHandler_Create_ManualRequestParsing(t *testing.T) {
	svc := &fakeVendorRequestServicer{
		createResult: &service.VendorRequestDetail{ID: 10, RequestNumber: "REPTAG20270115001"},
	}
	router, ts := mountVendorRequestHandler(t, svc)

	body := `{
		"replenish_date": "2027-01-15",
		"request_category": "emergency",
		"is_manual": true,
		"vendor_id": 7,
		"notes": "manual test",
		"items": [{"terminal_id": "T1", "denom": 100000, "amount_replenish": 5000000, "brand": "Wincor", "lokasi_atm": "Plaza Test"}]
	}`
	rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/", vendorRequestTokenFor(t, ts, 5, "ATM-USER"), body)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", rec.Code, rec.Body.String())
	}
	in := svc.lastCreateInput
	if in.VendorID != 7 || !in.IsManual || in.RequestCategory != "emergency" {
		t.Errorf("parsed input = %+v, want VendorID=7 IsManual=true RequestCategory=emergency", in)
	}
	if in.ReplenishDate.Format("2006-01-02") != "2027-01-15" {
		t.Errorf("ReplenishDate = %v, want 2027-01-15", in.ReplenishDate)
	}
	if in.ForecastDate.IsZero() {
		t.Error("ForecastDate should default to today (Q5), not be zero-valued")
	}
	if len(in.Items) != 1 || in.Items[0].Brand != "Wincor" || in.Items[0].LokasiATM != "Plaza Test" {
		t.Errorf("item context fields not parsed: %+v", in.Items)
	}
	if !in.Items[0].PeriodePred.IsZero() {
		t.Errorf("manual item PeriodePred should be left zero-valued at the handler layer, got %v", in.Items[0].PeriodePred)
	}
}

// TestVendorRequestHandler_Create_NonManualRequiresPeriodePred covers Task
// 8.1: a non-manual (standard) create still requires periode_pred per item,
// unchanged from the base spec.
func TestVendorRequestHandler_Create_NonManualRequiresPeriodePred(t *testing.T) {
	router, ts := mountVendorRequestHandler(t, &fakeVendorRequestServicer{})

	body := `{
		"forecast_date": "2027-01-15",
		"replenish_date": "2027-01-16",
		"vendor_id": 7,
		"items": [{"terminal_id": "T1", "denom": 100000, "amount_replenish": 5000000}]
	}`
	rec := vendorRequestDoRequest(router, http.MethodPost, "/api/v1/vendor-requests/", vendorRequestTokenFor(t, ts, 5, "ATM-USER"), body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (missing periode_pred), body=%s", rec.Code, rec.Body.String())
	}
}
