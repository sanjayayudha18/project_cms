package handler

// OpenAPI contract test (sdlc/openapi-contract FR3–FR5, T1–T4): every route of
// the pilot handlers is in backend/api/openapi.yaml and vice versa, and every
// response the handlers write validates against it. Fakes stand in for the
// services; only the wire shape is under test.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/api"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

const (
	contractVRBase = "/api/v1/vendor-requests"
	contractVOBase = "/api/v1/vendor/replenish-orders"
	contractDLBase = "/api/v1/dsr-late-reports"
)

// -- spec loading + validation ----------------------------------------------

func loadContract(t *testing.T) (*openapi3.T, routers.Router) {
	t.Helper()
	doc, err := openapi3.NewLoader().LoadFromData(api.Spec)
	if err != nil {
		t.Fatalf("load openapi.yaml: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("openapi.yaml invalid: %v", err)
	}
	router, err := legacy.NewRouter(doc)
	if err != nil {
		t.Fatalf("openapi router: %v", err)
	}
	return doc, router
}

// validateContract checks one recorded response against the operation the
// request maps to; an undocumented status is an error too.
func validateContract(router routers.Router, req *http.Request, status int, header http.Header, body []byte) error {
	route, params, err := router.FindRoute(req)
	if err != nil {
		return err
	}
	opts := &openapi3filter.Options{IncludeResponseStatus: true, AuthenticationFunc: openapi3filter.NoopAuthenticationFunc}
	return openapi3filter.ValidateResponse(context.Background(), &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route, Options: opts},
		Status:                 status,
		Header:                 header,
		Body:                   io.NopCloser(bytes.NewReader(body)),
		Options:                opts,
	})
}

// TestOpenAPIContract_SpecValid is T1.
func TestOpenAPIContract_SpecValid(t *testing.T) {
	loadContract(t)
}

// -- T2: route coverage -------------------------------------------------------

func TestOpenAPIContract_RouteCoverage(t *testing.T) {
	doc, _ := loadContract(t)
	mounted := map[string]chi.Router{
		"/vendor-requests":         contractVRHandler(&fakeVendorRequestServicer{}, &fakeVaultReviewer{}, contractPartyReader{}).Routes(),
		"/vendor/replenish-orders": NewVendorOrderHandler(&contractOrderSvc{}).Routes(),
		"/dsr-late-reports":        NewDsrLateHandler(&contractDsrLate{}).Routes(),
	}
	for prefix, routes := range mounted {
		t.Run(prefix, func(t *testing.T) {
			var code []string
			err := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
				code = append(code, method+" "+strings.TrimSuffix(prefix+route, "/"))
				return nil
			})
			if err != nil {
				t.Fatalf("walk: %v", err)
			}
			var spec []string
			for path, item := range doc.Paths.Map() {
				if path != prefix && !strings.HasPrefix(path, prefix+"/") {
					continue
				}
				for method := range item.Operations() {
					spec = append(spec, method+" "+path)
				}
			}
			sort.Strings(code)
			sort.Strings(spec)
			if strings.Join(code, "\n") != strings.Join(spec, "\n") {
				t.Errorf("routes and spec differ\nroutes:\n%s\n\nspec:\n%s", strings.Join(code, "\n"), strings.Join(spec, "\n"))
			}
		})
	}
}

// -- T3: responses validate -----------------------------------------------------

type contractCase struct {
	name       string
	method     string
	path       string
	body       string
	role       string // "" = no token, "VENDOR" = VENDOR-USER with vendor claim
	vr         *fakeVendorRequestServicer
	vault      *fakeVaultReviewer
	parties    contractPartyReader
	orders     *contractOrderSvc
	dsrLate    *contractDsrLate
	wantStatus int
}

func TestOpenAPIContract_Responses(t *testing.T) {
	doc, router := loadContract(t)
	covered := map[string]bool{}
	for _, tc := range contractCases() {
		t.Run(tc.name, func(t *testing.T) {
			rec, req := serveContractCase(t, tc)
			if route, _, err := router.FindRoute(req); err == nil {
				covered[route.Method+" "+route.Path] = true
			}
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if err := validateContract(router, req, rec.Code, rec.Header(), rec.Body.Bytes()); err != nil {
				t.Errorf("response does not match openapi.yaml: %v\nbody: %s", err, rec.Body.String())
			}
		})
	}
	// Every documented operation needs at least one validated response.
	for path, item := range doc.Paths.Map() {
		for method := range item.Operations() {
			if !covered[method+" "+path] {
				t.Errorf("no response case for %s %s in contractCases", method, path)
			}
		}
	}
}

// TestOpenAPIContract_RejectsDrift is T4: the schemas are strict enough to
// catch an undocumented field, money sent as a string, a dropped field and an
// undocumented status.
func TestOpenAPIContract_RejectsDrift(t *testing.T) {
	_, router := loadContract(t)
	rec, req := serveContractCase(t, contractCase{method: http.MethodGet, path: contractVRBase + "/1", role: "ADMIN",
		vr: &fakeVendorRequestServicer{getResult: contractDetailMinimal()}, wantStatus: http.StatusOK})
	good := rec.Body.String()
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"extra field", http.StatusOK, strings.Replace(good, `"id":1,`, `"id":1,"undocumented":true,`, 1)},
		{"money as string", http.StatusOK, strings.Replace(good, `"total_amount":0`, `"total_amount":"0"`, 1)},
		{"missing field", http.StatusOK, strings.Replace(good, `"notes":"",`, "", 1)},
		{"date as timestamp", http.StatusOK, strings.Replace(good, `"forecast_date":"2026-10-19"`, `"forecast_date":"2026-10-19T03:04:05Z"`, 1)},
		{"undocumented status", http.StatusTeapot, good},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.status == http.StatusOK && tc.body == good {
				t.Fatalf("fixture did not change; replace target missing in %s", good)
			}
			if err := validateContract(router, req, tc.status, rec.Header(), []byte(tc.body)); err == nil {
				t.Errorf("validation passed, want failure")
			}
		})
	}
}

// -- harness -------------------------------------------------------------------

func contractVRHandler(svc *fakeVendorRequestServicer, vault *fakeVaultReviewer, parties contractPartyReader) *VendorRequestHandler {
	return NewVendorRequestHandler(svc).WithVaultReview(vault).WithVendorSide(parties, &fakeReturner{})
}

func serveContractCase(t *testing.T, tc contractCase) (*httptest.ResponseRecorder, *http.Request) {
	t.Helper()
	ts := testTokenService()
	vr := tc.vr
	if vr == nil {
		vr = &fakeVendorRequestServicer{getResult: contractDetailMinimal()}
	}
	vault := tc.vault
	if vault == nil {
		vault = &fakeVaultReviewer{}
	}
	orders := tc.orders
	if orders == nil {
		orders = &contractOrderSvc{}
	}
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(ts)).Mount(contractVRBase, contractVRHandler(vr, vault, tc.parties).Routes())
	r.With(custommw.RequireAuth(ts)).Mount(contractVOBase, NewVendorOrderHandler(orders).Routes())
	dsrLate := tc.dsrLate
	if dsrLate == nil {
		dsrLate = &contractDsrLate{}
	}
	r.With(custommw.RequireAuth(ts)).Mount(contractDLBase, NewDsrLateHandler(dsrLate).Routes())

	req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
	req.Header.Set("Content-Type", "application/json")
	switch tc.role {
	case "":
	case "VENDOR":
		req.Header.Set("Authorization", "Bearer "+tokenForVendor(t, ts, 9, 42))
	default:
		req.Header.Set("Authorization", "Bearer "+tokenForRole(t, ts, 5, tc.role))
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	// a fresh request for the validator: the served one has a consumed body
	return rec, httptest.NewRequest(tc.method, tc.path, nil)
}

// contractDsrLate is fakeDsrLateReader with warnings shaped as the real service builds them
// (every list non-nil, a label and a vendor row so their fields are validated too).
type contractDsrLate struct{ fakeDsrLateReader }

func (c *contractDsrLate) Daily(ctx context.Context, a service.Actor, f service.DsrLateFilter) (*service.DsrLateDailyList, error) {
	out, err := c.fakeDsrLateReader.Daily(ctx, a, f)
	if err != nil {
		return nil, err
	}
	region, recv := "JKT", contractTime().In(testWIB)
	deadline := time.Date(2026, 10, 5, 9, 0, 0, 0, testWIB)
	out.Items = []service.DsrLateDailyView{
		{ReportDate: "2026-10-05", VendorID: 2, VendorCode: "V1", VendorName: "PT A", VendorBranchID: 11, BranchCode: "B11", BranchName: "Cabang 11",
			RegionCode: &region, Deadline: deadline, FirstReceivedAt: &recv, ReceiptChecksum: "abc", MatchedLabels: []string{"Gudang X"},
			Status: "late", LateMinutes: 12},
		{ReportDate: "2026-10-06", VendorID: 2, VendorCode: "V1", VendorName: "PT A", VendorBranchID: 11, BranchCode: "B11", BranchName: "Cabang 11",
			Deadline: deadline, MatchedLabels: []string{}, Status: "not_sent"},
	}
	out.Total = 2
	return out, nil
}

func (c *contractDsrLate) Warnings(ctx context.Context, a service.Actor, f service.DsrLateFilter) (*service.DsrLateWarnings, error) {
	out, err := c.fakeDsrLateReader.Warnings(ctx, a, f)
	if err != nil {
		return nil, err
	}
	first, last := "2026-10-01", "2026-10-05"
	out.UnmappedLabels = []service.DsrLateUnmappedLabel{{VendorID: 2, VendorName: "PT A", DsrLocation: "Gudang X", ReceiptCount: 3,
		FirstReportDate: &first, LastReportDate: &last}}
	out.UnresolvedVendors = []service.DsrLateUnresolvedVendor{{VendorLabel: "PT Z", ReceiptCount: 1}}
	return out, nil
}

// contractPartyReader returns parties as the real service builds them (non-nil slices).
type contractPartyReader struct{ err error }

func (p contractPartyReader) RequestParties(context.Context, int64) (*service.RequestVendorParties, error) {
	if p.err != nil {
		return nil, p.err
	}
	sent, reason, by := contractTime(), "armada penuh", "Vendor A"
	return &service.RequestVendorParties{
		Parties: []service.RequestVendorParty{
			{ID: 3, Role: "replenish", Status: "rejected", BranchID: 11, BranchCode: "B11", BranchName: "Cabang 11", VendorID: 2, VendorName: "PT A",
				SentAt: &sent, DecidedAt: &sent, DecidedByName: &by, RejectionReason: &reason},
			{ID: 4, Role: "vault", Status: "pending", BranchID: 12, BranchCode: "B12", BranchName: "Cabang 12", VendorID: 2, VendorName: "PT A"},
		},
		Events: []service.RequestVendorPartyEvent{
			{ID: 1, PartyID: 3, Event: "rejected", Reason: &reason, ActorName: by, CreatedAt: &sent},
			{ID: 2, PartyID: 4, Event: "sent", ActorName: "SPV"},
		},
	}, nil
}

// contractOrderSvc is a configurable VendorOrderServicer.
type contractOrderSvc struct {
	detail *service.VendorOrderDetail
	list   *service.VendorOrderList
	err    error
}

func (s *contractOrderSvc) result() (*service.VendorOrderDetail, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.detail != nil {
		return s.detail, nil
	}
	return contractOrderFull(), nil
}
func (s *contractOrderSvc) List(context.Context, service.VendorActor, service.VendorOrderFilter) (*service.VendorOrderList, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.list != nil {
		return s.list, nil
	}
	return &service.VendorOrderList{Items: []service.VendorOrderSummary{contractOrderFull().VendorOrderSummary}, Total: 1, Page: 1, PageSize: 20}, nil
}
func (s *contractOrderSvc) Get(context.Context, service.VendorActor, int64) (*service.VendorOrderDetail, error) {
	return s.result()
}
func (s *contractOrderSvc) Accept(context.Context, service.VendorActor, int64) (*service.VendorOrderDetail, error) {
	return s.result()
}
func (s *contractOrderSvc) Reject(context.Context, service.VendorActor, int64, string) (*service.VendorOrderDetail, error) {
	return s.result()
}
func (s *contractOrderSvc) SubmitRealization(context.Context, service.VendorActor, int64, []service.RealizationRowInput) (*service.VendorOrderDetail, error) {
	return s.result()
}
func (s *contractOrderSvc) ListRealizations(context.Context, service.VendorActor, service.ReplenishmentFilter) (*service.ClassificationList, error) {
	return nil, errors.New("not in the openapi-contract pilot")
}

// -- fixtures: a "full" variant (every nullable set) and a "minimal" one (nulls) --

func contractTime() time.Time { return time.Date(2026, 10, 19, 3, 4, 5, 0, time.UTC) }

func contractStr(s string) *string { return &s }

func contractDetailMinimal() *service.VendorRequestDetail {
	return &service.VendorRequestDetail{ID: 1, RequestNumber: "REP-A-JKT-20261019-001", ForecastDate: contractTime(), Status: "draft",
		CreatedBy: service.UserRef{ID: 5, FullName: "Budi"}, CreatedAt: contractTime(), UpdatedAt: contractTime()}
}

func contractDetailFull() *service.VendorRequestDetail {
	t := contractTime()
	reason := "alasan yang cukup panjang"
	quota, total := int32(3), int32(8)
	user := &service.UserRef{ID: 6, FullName: "Sari"}
	return &service.VendorRequestDetail{
		ID: 1, RequestNumber: "REP-A-JKT-20261019-001", ForecastDate: t, Status: "completed", Notes: "catatan",
		CreatedBy: service.UserRef{ID: 5, FullName: "Budi"}, ApprovedBy: user, RejectedBy: user, RejectionReason: reason,
		CreatedAt: t, UpdatedAt: t, SubmittedAt: &t, ApprovedAt: &t, RejectedAt: &t,
		Items: []service.VendorRequestItemOut{{ID: 9, TerminalID: "S1ABC", PeriodePred: t, Denom: 100000,
			AmountReplenish: 200_000_000, AmountRefund: 0, TicketNumber: "S1ABC_100K_20261020_001"}},
		TotalAmount: 200_000_000, ReplenishDate: &t, RequestCategory: contractStr("planned"), IsManual: true,
		CancellationReason: &reason, RegionCode: contractStr("JKT"),
		CompletionSubmittedBy: user, CompletionSubmittedAt: &t, CompletionApprovedBy: user, CompletionApprovedAt: &t,
		CompletionRejectedBy: user, CompletionRejectedAt: &t, CompletionRejectionReason: &reason,
		Atms: []service.RequestAtmStatus{
			{TerminalID: "S1ABC", TicketNumber: "S1ABC_100K_20261020_001", CompletionResult: contractStr("success"),
				VisitRemaining: &quota, VisitQuotaTotal: &total, IsOverQuota: true, RealizedDate: &t,
				RealizedAmounts: []service.DenomAmount{{Denom: 100000, Amount: 200_000_000}}},
			{TerminalID: "S1XYZ", TicketNumber: "S1XYZ_50K_20261020_001", CompletionResult: contractStr("failed")},
			{TerminalID: "S1QQQ", TicketNumber: "S1QQQ_MIX_20261020_001"},
		},
	}
}

func contractForecast() *service.BrowseForecastResult {
	t, quota := contractTime(), int32(2)
	return &service.BrowseForecastResult{Total: 2, Page: 1, PageSize: 20, TotalPages: 1, Data: []service.ForecastRow{
		{TerminalID: "S1ABC", PeriodePred: t, Denom: 100000, AmountReplenish: 200_000_000, DmaaFileID: 3, LokasiATM: "Mall", Brand: "NCR",
			FLMVendor: "PT A", FLMVendorRegion: "JKT", PriorityClass: "A", Paket: "PAKET 3", Escrow: contractStr("1250000.50"),
			IsRequested: true, VisitRemaining: &quota, VisitQuotaTotal: &quota},
		{TerminalID: "S1XYZ", PeriodePred: t, Denom: 50000},
	}}
}

func contractList() *service.ListVendorRequestResult {
	full, minimal := contractDetailFull(), contractDetailMinimal()
	return &service.ListVendorRequestResult{Total: 2, Page: 1, PageSize: 20, TotalPages: 1, Data: []service.VendorRequestSummary{
		{ID: full.ID, RequestNumber: full.RequestNumber, ForecastDate: full.ForecastDate, Status: full.Status, Notes: full.Notes,
			ItemCount: 1, TotalAmount: full.TotalAmount, CreatedBy: full.CreatedBy, ApprovedBy: full.ApprovedBy, CreatedAt: full.CreatedAt,
			SubmittedAt: full.SubmittedAt, ApprovedAt: full.ApprovedAt, RejectedAt: full.RejectedAt, ReplenishDate: full.ReplenishDate,
			RequestCategory: full.RequestCategory, IsCanceled: true, IsManual: true, CancellationReason: full.CancellationReason},
		{ID: 2, RequestNumber: minimal.RequestNumber, ForecastDate: minimal.ForecastDate, Status: minimal.Status,
			CreatedBy: minimal.CreatedBy, CreatedAt: minimal.CreatedAt},
	}}
}

func contractVaultPlan() *service.VaultPlanDetail {
	ts := pgtype.Timestamptz{Time: contractTime(), Valid: true}
	by, reason := int64(7), "buffer untuk antisipasi"
	branch := service.VaultBranchRef{ID: 11, Code: "B11", Name: "Cabang 11", VendorID: 2, VendorName: "PT A", RegionCode: contractStr("JKT")}
	ticket := "V21_20261020_001"
	return &service.VaultPlanDetail{
		Plan: db.GetVaultPlanRow{ID: 4, VendorRequestID: 1, AcmAreaID: 2, AcmAreaName: "Area Jakarta", Status: "acm_approved",
			SubmittedBy: &by, SubmittedAt: ts, ApprovedBy: &by, ApprovedAt: ts, RejectedBy: &by, RejectedAt: ts, RejectionReason: &reason,
			BufferReason: &reason, RequestNumber: "REP-A-JKT-20261019-001", ReplenishDate: pgtype.Date{Time: contractTime(), Valid: true},
			RegionCode: contractStr("JKT"), RequestStatus: "vault_review", VaultRejectionReason: &reason, RejectedByVendor: true},
		Atms:        []service.VaultPlanAtm{{TerminalID: "S1ABC", ReplenishBranch: branch, Order: map[string]string{"100000": "200000000"}}},
		OrderTotals: map[string]string{"100000": "200000000"},
		Allocations: []service.VaultAllocationView{
			{ID: 1, VendorVaultID: 21, VaultCode: "V21", VaultCategory: "CASH", Branch: branch, Tier: 3, IsUrgent: true, UrgentReason: &reason,
				Amounts: map[string]string{"100000": "200000000"}, Saldo: map[string]string{"100000": "1500000000"},
				Capacity: map[string]string{"100000": "1300000000"}, CapacityWarning: true, TicketNumber: &ticket},
			{ID: 2, VendorVaultID: 22, VaultCode: "V22", VaultCategory: "ATM_CASH", Branch: branch, Tier: 1},
		},
		Tickets: []db.ListVaultTicketsForRequestRow{{ID: 1, VendorVaultID: 21, VaultCode: "V21", TicketNumber: ticket, IsActive: true, CreatedAt: ts}},
	}
}

func contractOrderFull() *service.VendorOrderDetail {
	t, reason := contractTime(), "armada penuh hari ini"
	da := []service.DenomAmount{{Denom: 100000, Amount: 200_000_000}}
	self := service.PartyBranch{ID: 11, Code: "B11", Name: "Cabang 11", VendorID: 2, VendorName: "PT A", Address: "Jl. Sudirman"}
	return &service.VendorOrderDetail{
		VendorOrderSummary: service.VendorOrderSummary{ID: 7, Role: "replenish", Status: "accepted", RequestNumber: "REP-A-JKT-20261019-001",
			ReplenishDate: &t, RequestStatus: "vendor_accepted", BranchID: 11, BranchCode: "B11", BranchName: "Cabang 11",
			AtmCount: 1, VaultCount: 1, Totals: da, Currency: "IDR", SentAt: &t, DecidedAt: &t},
		RejectionReason: &reason,
		Content: service.PartyContent{Role: "replenish", Branch: self,
			Atms:              []service.PartyAtm{{TerminalID: "S1ABC", Lokasi: "Mall", TicketNumber: "S1ABC_100K_20261020_001", Denoms: da}},
			Vaults:            []service.PartyVault{{TicketNumber: "V21_20261020_001", Vault: service.PartyVaultRef{ID: 21, Code: "V21"}, Branch: self, Denoms: da}},
			ReplenishBranches: []service.PartyBranch{{ID: 11, Code: "B11", Name: "Cabang 11", VendorID: 2, VendorName: "PT A"}},
			Totals:            da, Currency: "IDR"},
		Realizations: []service.RealizationView{
			{TerminalID: "S1ABC", TicketNumber: "S1ABC_100K_20261020_001", Result: contractStr("success"), RealizedDate: &t,
				Amounts: da, SubmittedAt: &t, SubmittedByName: "Vendor A"},
		},
		CanReport: true, CompletionRejectionReason: &reason,
	}
}

func contractCases() []contractCase {
	vr, vo, dl := contractVRBase, contractVOBase, contractDLBase
	full, minimal := contractDetailFull(), contractDetailMinimal()
	detail := func(d *service.VendorRequestDetail) *fakeVendorRequestServicer {
		return &fakeVendorRequestServicer{createResult: d, updateResult: d, submitResult: d, approveResult: d, rejectResult: d,
			reviseResult: d, cancelResult: d, getResult: d, completionResult: d, overQuota: []string{"S1ABC"}}
	}
	createBody := `{"forecast_date":"2026-10-19","replenish_date":"2026-10-20","request_category":"planned",
		"items":[{"terminal_id":"S1ABC","periode_pred":"2026-10-20","denom":100000,"amount_replenish":200000000}]}`
	itemsBody := `{"items":[{"terminal_id":"S1ABC","periode_pred":"2026-10-20","denom":100000,"amount_replenish":200000000}]}`
	realization := `{"realizations":[{"terminal_id":"S1ABC","result":"success","realized_date":"2026-10-20","amounts":{"100000":200000000}}]}`

	cases := []contractCase{
		// forecast + options
		{name: "forecast", method: http.MethodGet, path: vr + "/forecast?forecast_date=2026-10-19", role: "ADMIN",
			vr: &fakeVendorRequestServicer{browseResult: contractForecast()}, wantStatus: 200},
		{name: "forecast bad flag", method: http.MethodGet, path: vr + "/forecast?unassigned=yes", role: "ADMIN", wantStatus: 400},
		{name: "forecast summary", method: http.MethodGet, path: vr + "/forecast/summary?forecast_date=2026-10-19", role: "ADMIN",
			vr: &fakeVendorRequestServicer{summaryResult: &service.ForecastSummaryResult{ForecastDate: contractTime(),
				Totals: service.ForecastSummaryTotals{ATMCount: 2, AmountReplenish: 1},
				Groups: []service.ForecastSummaryGroup{{VendorID: 2, FLMVendor: "PT A", FLMVendorRegion: "JKT", ATMCount: 2}}}}, wantStatus: 200},
		{name: "forecast summary error", method: http.MethodGet, path: vr + "/forecast/summary", role: "ADMIN",
			vr: &fakeVendorRequestServicer{summaryErr: &service.ValidationError{Field: "forecast_date", Message: "wajib"}}, wantStatus: 400},
		{name: "vendor options", method: http.MethodGet, path: vr + "/vendors", role: "ADMIN",
			vr: &fakeVendorRequestServicer{vendorOptionsResult: &service.VendorOptionsResult{
				Vendors: []service.VendorOption{{ID: 2, Name: "PT A"}}, Regions: []string{"JKT"}}}, wantStatus: 200},
		{name: "vendor options error", method: http.MethodGet, path: vr + "/vendors", role: "ADMIN",
			vr: &fakeVendorRequestServicer{vendorOptionsErr: errors.New("db down")}, wantStatus: 500},

		// list / get / audit
		{name: "list", method: http.MethodGet, path: vr + "?status=draft,pending&page=1", role: "ATM-USER",
			vr: &fakeVendorRequestServicer{listResult: contractList()}, wantStatus: 200},
		{name: "list bad page", method: http.MethodGet, path: vr + "?page=x", role: "ATM-USER", wantStatus: 400},
		{name: "get full", method: http.MethodGet, path: vr + "/1", role: "ATM-USER", vr: detail(full), wantStatus: 200},
		{name: "get minimal", method: http.MethodGet, path: vr + "/1", role: "ATM-USER", vr: detail(minimal), wantStatus: 200},
		{name: "get not found", method: http.MethodGet, path: vr + "/1", role: "ATM-USER",
			vr: &fakeVendorRequestServicer{getErr: service.ErrNotFound}, wantStatus: 404},
		{name: "get bad id", method: http.MethodGet, path: vr + "/0", role: "ATM-USER", wantStatus: 400},
		{name: "audit log", method: http.MethodGet, path: vr + "/1/audit-log", role: "ATM-SPV",
			vr: &fakeVendorRequestServicer{auditResult: []service.AuditEntry{
				{ID: 1, EntityType: "vendor_request", EntityID: 1, Action: "submit", PerformedBy: 5, PerformedAt: contractTime(),
					PreviousState: contractStr("draft"), NewState: contractStr("pending"), Metadata: map[string]any{"ip": "10.0.0.1", "n": 2}},
				{ID: 2, EntityType: "vendor_request", EntityID: 1, Action: "create", PerformedBy: 5, PerformedAt: contractTime()},
			}}, wantStatus: 200},
		{name: "audit log empty", method: http.MethodGet, path: vr + "/1/audit-log", role: "ATM-SPV",
			vr: &fakeVendorRequestServicer{auditResult: []service.AuditEntry{}}, wantStatus: 200},

		// create / items / transitions
		{name: "create", method: http.MethodPost, path: vr, body: createBody, role: "ATM-USER", vr: detail(full), wantStatus: 201},
		{name: "create bad date", method: http.MethodPost, path: vr, body: `{"forecast_date":"19-10-2026"}`, role: "ATM-USER", wantStatus: 400},
		{name: "create conflict", method: http.MethodPost, path: vr, body: createBody, role: "ATM-USER",
			vr: &fakeVendorRequestServicer{createErr: service.ErrNumberExhausted}, wantStatus: 409},
		{name: "update items", method: http.MethodPut, path: vr + "/1/items", body: itemsBody, role: "ATM-USER", vr: detail(minimal), wantStatus: 200},
		{name: "update items not creator", method: http.MethodPut, path: vr + "/1/items", body: itemsBody, role: "ATM-USER",
			vr: &fakeVendorRequestServicer{updateErr: service.ErrNotCreator}, wantStatus: 403},
		{name: "submit", method: http.MethodPost, path: vr + "/1/submit", role: "ATM-USER", vr: detail(minimal), wantStatus: 200},
		{name: "approve", method: http.MethodPost, path: vr + "/1/approve", role: "ATM-SPV", vr: detail(full), wantStatus: 200},
		{name: "approve self", method: http.MethodPost, path: vr + "/1/approve", role: "ATM-SPV",
			vr: &fakeVendorRequestServicer{approveErr: service.ErrSelfApproval}, wantStatus: 403},
		{name: "reject", method: http.MethodPost, path: vr + "/1/reject", body: `{"rejection_reason":"tidak sesuai forecast"}`, role: "ATM-SPV",
			vr: detail(full), wantStatus: 200},
		{name: "reject empty reason", method: http.MethodPost, path: vr + "/1/reject", body: `{}`, role: "ATM-SPV",
			vr: &fakeVendorRequestServicer{rejectErr: service.ErrRejectReasonEmpty}, wantStatus: 422},
		{name: "revise", method: http.MethodPost, path: vr + "/1/revise", role: "ATM-USER", vr: detail(minimal), wantStatus: 200},
		{name: "cancel", method: http.MethodPost, path: vr + "/1/cancel", body: `{"cancellation_reason":"tidak jadi"}`, role: "ATM-SPV",
			vr: detail(full), wantStatus: 200},
		{name: "cancel twice", method: http.MethodPost, path: vr + "/1/cancel", body: `{"cancellation_reason":"tidak jadi"}`, role: "ATM-SPV",
			vr: &fakeVendorRequestServicer{cancelErr: service.ErrAlreadyCanceled}, wantStatus: 409},

		// laporan selesai (legacy)
		{name: "complete", method: http.MethodPost, path: vr + "/1/complete", body: `{"results":[{"terminal_id":"S1ABC","result":"success"}]}`,
			role: "ATM-USER", vr: detail(full), wantStatus: 200},
		{name: "complete by vendor", method: http.MethodPost, path: vr + "/1/complete", body: `{"results":[]}`, role: "ATM-USER",
			vr: &fakeVendorRequestServicer{completionErr: service.ErrCompletionByVendor}, wantStatus: 409},
		{name: "complete approve over quota", method: http.MethodPost, path: vr + "/1/complete/approve", role: "ATM-SPV", vr: detail(full), wantStatus: 200},
		{name: "complete approve no over quota", method: http.MethodPost, path: vr + "/1/complete/approve", role: "ATM-SPV",
			vr: &fakeVendorRequestServicer{completionResult: minimal}, wantStatus: 200},
		{name: "complete reject", method: http.MethodPost, path: vr + "/1/complete/reject", body: `{"reason":"data salah"}`, role: "ATM-SPV",
			vr: detail(minimal), wantStatus: 200},
		{name: "complete reject empty", method: http.MethodPost, path: vr + "/1/complete/reject", body: `{}`, role: "ATM-SPV",
			vr: &fakeVendorRequestServicer{completionErr: service.ErrCompletionReasonEmpty}, wantStatus: 422},

		// vault review
		{name: "vault assignments", method: http.MethodGet, path: vr + "/1/vault-assignments", role: "ATM-USER",
			vault: &fakeVaultReviewer{review: contractVaultPlan()}, wantStatus: 200},
		{name: "vault assignments minimal plan", method: http.MethodGet, path: vr + "/1/vault-assignments", role: "ATM-USER",
			vault: &fakeVaultReviewer{review: &service.VaultPlanDetail{Plan: db.GetVaultPlanRow{ID: 4}, Allocations: []service.VaultAllocationView{}}},
			wantStatus: 200},
		{name: "vault assignments no plan", method: http.MethodGet, path: vr + "/1/vault-assignments", role: "ATM-USER", wantStatus: 200},
		{name: "vault approve", method: http.MethodPost, path: vr + "/1/vault-approve", role: "ATM-SPV", vr: detail(full), wantStatus: 200},
		{name: "vault approve conflict", method: http.MethodPost, path: vr + "/1/vault-approve", role: "ATM-SPV",
			vault: &fakeVaultReviewer{err: service.ErrInvalidTransition}, wantStatus: 409},
		{name: "vault reject", method: http.MethodPost, path: vr + "/1/vault-reject", body: `{"vault_rejection_reason":"vault penuh"}`,
			role: "ATM-SPV", vr: detail(minimal), wantStatus: 200},

		// vendor side
		{name: "vendor parties", method: http.MethodGet, path: vr + "/1/vendor-parties", role: "ATM-USER", wantStatus: 200},
		{name: "vendor parties error", method: http.MethodGet, path: vr + "/1/vendor-parties", role: "ATM-USER",
			parties: contractPartyReader{err: service.ErrNotFound}, wantStatus: 404},
		{name: "vendor return", method: http.MethodPost, path: vr + "/1/vendor-return", body: `{"rejection_reason":"revisi order"}`,
			role: "ATM-SPV", wantStatus: 200},

		// auth on the vendor-requests prefix
		{name: "vr no token", method: http.MethodGet, path: vr + "/1", wantStatus: 401},
		{name: "vr vendor role denied", method: http.MethodGet, path: vr + "/1", role: "VENDOR", wantStatus: 403},

		// vendor orders
		{name: "orders list", method: http.MethodGet, path: vo + "?party_status=pending&from=2026-10-01", role: "VENDOR", wantStatus: 200},
		{name: "orders list empty", method: http.MethodGet, path: vo, role: "VENDOR",
			orders: &contractOrderSvc{list: &service.VendorOrderList{Items: []service.VendorOrderSummary{}, Page: 1, PageSize: 20}}, wantStatus: 200},
		{name: "orders list bad status", method: http.MethodGet, path: vo + "?party_status=x", role: "VENDOR", wantStatus: 422},
		{name: "order get full", method: http.MethodGet, path: vo + "/7", role: "VENDOR", wantStatus: 200},
		{name: "order get minimal", method: http.MethodGet, path: vo + "/7", role: "VENDOR",
			orders: &contractOrderSvc{detail: &service.VendorOrderDetail{VendorOrderSummary: service.VendorOrderSummary{ID: 7, Role: "vault", Status: "pending", Currency: "IDR"}}},
			wantStatus: 200},
		{name: "order get not found", method: http.MethodGet, path: vo + "/7", role: "VENDOR",
			orders: &contractOrderSvc{err: service.ErrVendorOrderNotFound}, wantStatus: 404},
		{name: "order get bad id", method: http.MethodGet, path: vo + "/abc", role: "VENDOR", wantStatus: 400},
		{name: "order accept", method: http.MethodPost, path: vo + "/7/accept", role: "VENDOR", wantStatus: 200},
		{name: "order accept conflict", method: http.MethodPost, path: vo + "/7/accept", role: "VENDOR",
			orders: &contractOrderSvc{err: service.ErrInvalidTransition}, wantStatus: 409},
		{name: "order reject", method: http.MethodPost, path: vo + "/7/reject", body: `{"reason":"armada tidak tersedia"}`, role: "VENDOR", wantStatus: 200},
		{name: "order reject short reason", method: http.MethodPost, path: vo + "/7/reject", body: `{"reason":"x"}`, role: "VENDOR",
			orders: &contractOrderSvc{err: &service.ValidationError{Field: "reason", Message: "alasan 10-500 karakter"}}, wantStatus: 422},
		{name: "order realization", method: http.MethodPost, path: vo + "/7/realization", body: realization, role: "VENDOR", wantStatus: 200},
		{name: "order realization bad denom", method: http.MethodPost, path: vo + "/7/realization",
			body: `{"realizations":[{"terminal_id":"S1ABC","result":"success","amounts":{"x":1}}]}`, role: "VENDOR", wantStatus: 422},
		{name: "order service forbidden", method: http.MethodPost, path: vo + "/7/accept", role: "VENDOR",
			orders: &contractOrderSvc{err: service.ErrNotAuthorized}, wantStatus: 403},
		{name: "order internal error", method: http.MethodPost, path: vo + "/7/accept", role: "VENDOR",
			orders: &contractOrderSvc{err: errors.New("boom")}, wantStatus: 500},
		{name: "orders internal role denied", method: http.MethodGet, path: vo, role: "ATM-SPV", wantStatus: 403},
		{name: "orders no token", method: http.MethodGet, path: vo, wantStatus: 401},

		// dsr late report
		{name: "dsr late daily", method: http.MethodGet, path: dl + "/daily?month=2026-10&status=late&page=1", role: "ATM-USER", wantStatus: 200},
		{name: "dsr late daily csv", method: http.MethodGet, path: dl + "/daily?month=2026-10&format=csv", role: "ATM-SPV", wantStatus: 200},
		{name: "dsr late daily no month", method: http.MethodGet, path: dl + "/daily", role: "ATM-USER", wantStatus: 400},
		{name: "dsr late daily bad status", method: http.MethodGet, path: dl + "/daily?month=2026-10&status=x", role: "ATM-USER", wantStatus: 400},
		{name: "dsr late daily forbidden", method: http.MethodGet, path: dl + "/daily?month=2026-10", role: "ATM-USER",
			dsrLate: &contractDsrLate{fakeDsrLateReader{err: service.ErrNotAuthorized}}, wantStatus: 403},
		{name: "dsr late daily error", method: http.MethodGet, path: dl + "/daily?month=2026-10", role: "ATM-USER",
			dsrLate: &contractDsrLate{fakeDsrLateReader{err: errors.New("boom")}}, wantStatus: 500},
		{name: "dsr late summary", method: http.MethodGet, path: dl + "/summary?month=2026-10&vendor_id=2", role: "BRANCH-ATM-SPV", wantStatus: 200},
		{name: "dsr late summary csv", method: http.MethodGet, path: dl + "/summary?month=2026-10&format=csv", role: "ADMIN", wantStatus: 200},
		{name: "dsr late summary bad vendor", method: http.MethodGet, path: dl + "/summary?month=2026-10&vendor_id=0", role: "ADMIN", wantStatus: 400},
		{name: "dsr late warnings", method: http.MethodGet, path: dl + "/warnings?month=2026-10", role: "ADMIN", wantStatus: 200},
		{name: "dsr late warnings bad month", method: http.MethodGet, path: dl + "/warnings?month=10-2026", role: "ADMIN", wantStatus: 400},
		{name: "dsr late no token", method: http.MethodGet, path: dl + "/daily?month=2026-10", wantStatus: 401},
		{name: "dsr late vendor role denied", method: http.MethodGet, path: dl + "/daily?month=2026-10", role: "VENDOR", wantStatus: 403},
	}
	// every handleError class once, on a route that reaches it
	for _, e := range []struct {
		name   string
		err    error
		status int
	}{
		{"not found", service.ErrNotFound, 404}, {"invalid transition", service.ErrInvalidTransition, 409},
		{"empty items", service.ErrEmptyItems, 409}, {"not checker", service.ErrNotChecker, 403},
		{"not authorized", service.ErrNotAuthorized, 403}, {"invalid items", &service.InvalidItemsError{}, 400},
		{"duplicate items", service.ErrDuplicateItems, 400}, {"cancel reason empty", service.ErrCancelReasonEmpty, 422},
		{"number generation", service.ErrNumberGeneration, 500}, {"unexpected", errors.New("boom"), 500},
	} {
		cases = append(cases, contractCase{name: "submit error " + e.name, method: http.MethodPost, path: vr + "/1/submit",
			role: "ATM-USER", vr: &fakeVendorRequestServicer{submitErr: e.err}, wantStatus: e.status})
	}
	return cases
}
