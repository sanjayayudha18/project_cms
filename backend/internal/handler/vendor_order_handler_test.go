package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// HTTP wiring only; scope/leak/transition logic is covered by
// internal/service/vendor_order_integration_test.go.
type fakeVendorOrderServicer struct {
	err    error
	lastVA service.VendorActor
	filter service.VendorOrderFilter
	reason string
}

func (f *fakeVendorOrderServicer) detail(va service.VendorActor) (*service.VendorOrderDetail, error) {
	f.lastVA = va
	if f.err != nil {
		return nil, f.err
	}
	return &service.VendorOrderDetail{VendorOrderSummary: service.VendorOrderSummary{ID: 7, Role: "vault", Currency: "IDR",
		Totals: []service.DenomAmount{{Denom: 100000, Amount: 1000000}}}}, nil
}
func (f *fakeVendorOrderServicer) List(_ context.Context, va service.VendorActor, flt service.VendorOrderFilter) (*service.VendorOrderList, error) {
	f.lastVA, f.filter = va, flt
	if f.err != nil {
		return nil, f.err
	}
	return &service.VendorOrderList{Items: []service.VendorOrderSummary{}, Page: 1, PageSize: 20}, nil
}
func (f *fakeVendorOrderServicer) Get(_ context.Context, va service.VendorActor, _ int64) (*service.VendorOrderDetail, error) {
	return f.detail(va)
}
func (f *fakeVendorOrderServicer) Accept(_ context.Context, va service.VendorActor, _ int64) (*service.VendorOrderDetail, error) {
	return f.detail(va)
}
func (f *fakeVendorOrderServicer) Reject(_ context.Context, va service.VendorActor, _ int64, reason string) (*service.VendorOrderDetail, error) {
	f.reason = reason
	return f.detail(va)
}

func testTokenService() *pkgauth.TokenService {
	return pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey: []byte("test-secret-minimum-32-bytes-long!!"), AccessTokenExpiry: 15 * time.Minute, SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
}

func TestVendorOrderHandler(t *testing.T) {
	const base = "/api/v1/vendor/replenish-orders"
	cases := []struct {
		name, method, path, body string
		vendor                   bool // VENDOR-USER token with vendor claim; else ATM-SPV
		svcErr                   error
		wantCode                 int
		wantBody                 string
	}{
		{"list", http.MethodGet, base + "?party_status=pending&from=2026-10-01&page=2", "", true, nil, http.StatusOK, `"items":[]`},
		{"internal role denied at route", http.MethodGet, base, "", false, nil, http.StatusForbidden, ""},
		{"unknown party_status", http.MethodGet, base + "?party_status=x", "", true, nil, http.StatusUnprocessableEntity, "party_status"},
		{"bad date", http.MethodGet, base + "?to=09-10-2026", "", true, nil, http.StatusUnprocessableEntity, `"to"`},
		{"get", http.MethodGet, base + "/7", "", true, nil, http.StatusOK, `"currency":"IDR"`},
		{"bad id", http.MethodGet, base + "/abc", "", true, nil, http.StatusBadRequest, ""},
		{"out of scope 404", http.MethodGet, base + "/7", "", true, service.ErrVendorOrderNotFound, http.StatusNotFound, ""},
		{"service not authorized 403", http.MethodGet, base + "/7", "", true, service.ErrNotAuthorized, http.StatusForbidden, ""},
		{"accept", http.MethodPost, base + "/7/accept", "", true, nil, http.StatusOK, `"amount":1000000`},
		{"accept twice 409", http.MethodPost, base + "/7/accept", "", true, service.ErrInvalidTransition, http.StatusConflict, ""},
		{"reject", http.MethodPost, base + "/7/reject", `{"reason":"armada tidak tersedia"}`, true, nil, http.StatusOK, ""},
		{"reject bad body", http.MethodPost, base + "/7/reject", `{`, true, nil, http.StatusBadRequest, ""},
		{"reject short reason", http.MethodPost, base + "/7/reject", `{"reason":"x"}`, true,
			&service.ValidationError{Field: "reason", Message: "alasan 10-500 karakter"}, http.StatusUnprocessableEntity, "reason"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := testTokenService()
			fake := &fakeVendorOrderServicer{err: tc.svcErr}
			r := chi.NewRouter()
			r.With(custommw.RequireAuth(ts)).Mount(base, NewVendorOrderHandler(fake).Routes())
			token := tokenForRole(t, ts, 1, "ATM-SPV")
			if tc.vendor {
				token = tokenForVendor(t, ts, 9, 42)
			}
			rec := doRequest(r, tc.method, tc.path, token, tc.body)
			if rec.Code != tc.wantCode {
				t.Fatalf("code = %d, want %d: %s", rec.Code, tc.wantCode, rec.Body.String())
			}
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Errorf("body %s does not contain %s", rec.Body.String(), tc.wantBody)
			}
			if tc.wantCode == http.StatusOK && (fake.lastVA.UserID != 9 || fake.lastVA.ClaimVendorID == nil || *fake.lastVA.ClaimVendorID != 42) {
				t.Errorf("service got actor %+v, want user 9 with vendor claim 42", fake.lastVA)
			}
		})
	}
}

func TestVendorOrderHandler_ListParsesFilter(t *testing.T) {
	ts := testTokenService()
	fake := &fakeVendorOrderServicer{}
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(ts)).Mount("/o", NewVendorOrderHandler(fake).Routes())
	rec := doRequest(r, http.MethodGet, "/o?party_status=accepted&request_status=sent_to_vendor&from=2026-10-01&to=2026-10-09&page=3&page_size=50",
		tokenForVendor(t, ts, 9, 42), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d: %s", rec.Code, rec.Body.String())
	}
	f := fake.filter
	if f.PartyStatus != "accepted" || f.RequestStatus != "sent_to_vendor" || f.Page != 3 || f.PageSize != 50 ||
		f.From == nil || f.From.Format("2006-01-02") != "2026-10-01" || f.To == nil || f.To.Format("2006-01-02") != "2026-10-09" {
		t.Fatalf("filter = %+v", f)
	}
}

type fakePartyReader struct{ err error }

func (f fakePartyReader) RequestParties(context.Context, int64) (*service.RequestVendorParties, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &service.RequestVendorParties{Parties: []service.RequestVendorParty{{ID: 3, Role: "replenish", Status: "pending"}}}, nil
}

type fakeReturner struct {
	err    error
	reason string
}

func (f *fakeReturner) VendorReturn(_ context.Context, _ service.Actor, id int64, reason string) (*service.VendorRequestDetail, error) {
	f.reason = reason
	if f.err != nil {
		return nil, f.err
	}
	return &service.VendorRequestDetail{ID: id, Status: "rejected"}, nil
}

// FR6.1/FR6.6 routes on the Vendor Request handler: viewer roles read the
// party panel, checker roles return; VENDOR-USER reaches neither.
func TestVendorRequestHandler_VendorSide(t *testing.T) {
	const base = "/api/v1/vendor-requests/5"
	cases := []struct {
		name, role, method, path, body string
		retErr                         error
		wantCode                       int
		wantBody                       string
	}{
		{"parties for viewer", "ATM-USER", http.MethodGet, base + "/vendor-parties", "", nil, http.StatusOK, `"role":"replenish"`},
		{"parties denied to vendor", "VENDOR-USER", http.MethodGet, base + "/vendor-parties", "", nil, http.StatusForbidden, ""},
		{"return by checker", "ATM-SPV", http.MethodPost, base + "/vendor-return", `{"rejection_reason":"kurangi ATM"}`, nil, http.StatusOK, `"rejected"`},
		{"return denied to maker role", "ATM-USER", http.MethodPost, base + "/vendor-return", `{"rejection_reason":"x"}`, nil, http.StatusForbidden, ""},
		{"return wrong status", "ATM-SPV", http.MethodPost, base + "/vendor-return", `{"rejection_reason":"x"}`, service.ErrInvalidTransition, http.StatusConflict, ""},
		{"return empty reason", "ATM-SPV", http.MethodPost, base + "/vendor-return", `{}`, service.ErrRejectReasonEmpty, http.StatusUnprocessableEntity, "rejection_reason"},
		{"return self", "ATM-SPV", http.MethodPost, base + "/vendor-return", `{"rejection_reason":"x"}`, service.ErrSelfApproval, http.StatusForbidden, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := testTokenService()
			ret := &fakeReturner{err: tc.retErr}
			h := NewVendorRequestHandler(&fakeVendorRequestServicer{}).WithVendorSide(fakePartyReader{}, ret)
			r := chi.NewRouter()
			r.With(custommw.RequireAuth(ts)).Mount("/api/v1/vendor-requests", h.Routes())
			rec := doRequest(r, tc.method, tc.path, tokenForRole(t, ts, 1, tc.role), tc.body)
			if rec.Code != tc.wantCode {
				t.Fatalf("code = %d, want %d: %s", rec.Code, tc.wantCode, rec.Body.String())
			}
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Errorf("body %s does not contain %s", rec.Body.String(), tc.wantBody)
			}
		})
	}
}
