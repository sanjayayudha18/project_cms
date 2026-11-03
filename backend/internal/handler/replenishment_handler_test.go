package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// HTTP wiring only; queries + replica routing are covered by the service integration test.
type fakeReplenishmentReader struct {
	err    error
	filter service.ReplenishmentFilter
	calls  int // report queries reached (AmountMismatch, Trips)
}

func (f *fakeReplenishmentReader) List(_ context.Context, _ service.Actor, flt service.ReplenishmentFilter) (*service.ClassificationList, error) {
	f.filter = flt
	return &service.ClassificationList{Items: []service.ClassificationView{}}, f.err
}
func (f *fakeReplenishmentReader) Summary(_ context.Context, _ service.Actor, from, to time.Time, _ *int64) (*service.ReplenishmentSummary, error) {
	f.filter.From, f.filter.To = from, to
	return &service.ReplenishmentSummary{Currency: "IDR"}, f.err
}
func (f *fakeReplenishmentReader) AmountMismatch(_ context.Context, _ service.Actor, flt service.ReplenishmentFilter) ([]service.ClassificationView, error) {
	f.filter = flt
	f.calls++
	term, num := "ITM1", "REP-X"
	off := int16(1)
	vendor := int64(190000000)
	sched := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	return []service.ClassificationView{{TerminalID: term, RequestNumber: &num, ScheduledDate: &sched, WorkdayOffset: &off,
		Classification: "late", Currency: "IDR",
		Amounts: []service.ClassificationAmount{{Denom: 100000, Order: 200000000, Itm: 150000000, Vendor: &vendor, Diff: -50000000}}}}, f.err
}
func (f *fakeReplenishmentReader) Trips(_ context.Context, _ service.Actor, _, _ time.Time, _ *int64) ([]service.TripRow, error) {
	f.calls++
	name := "=HYPERLINK(1)"
	return []service.TripRow{{TerminalID: "ITM1", Trips: 3, VendorName: &name}}, f.err
}

func TestReplenishmentHandler(t *testing.T) {
	const base = "/api/v1/replenishment"
	rng := "date_from=2026-10-01&date_to=2026-10-14"
	cases := []struct {
		name, path, role string
		svcErr           error
		wantCode         int
		wantBody         string
		noReportCall     bool // csv rejected before the service
	}{
		{"list", base + "/classifications?" + rng + "&classification=late&vendor_id=2&only_diff=true", "ATM-USER", nil, http.StatusOK, `"items":[]`, false},
		{"vendor role denied at route", base + "/classifications?" + rng, "VENDOR-USER", nil, http.StatusForbidden, "", false},
		{"bad date", base + "/classifications?date_from=14-10-2026", "ATM-SPV", nil, http.StatusUnprocessableEntity, "date_from", false},
		{"unknown class", base + "/classifications?" + rng + "&classification=x", "ATM-SPV", nil, http.StatusUnprocessableEntity, "classification", false},
		{"range too long (service)", base + "/classifications?" + rng, "ATM-SPV",
			&service.ValidationError{Field: "date_to", Message: "rentang maksimal 31 hari"}, http.StatusUnprocessableEntity, "date_to", false},
		{"summary", base + "/summary?" + rng, "BRANCH-ATM-SPV", nil, http.StatusOK, `"currency":"IDR"`, false},
		{"mismatch json", base + "/reports/amount-mismatch?" + rng, "ADMIN", nil, http.StatusOK, `"diff":-50000000`, false},
		{"trips json", base + "/reports/trips?" + rng, "ATM-SPV", nil, http.StatusOK, `"trips":3`, false},
		{"mismatch csv rejected", base + "/reports/amount-mismatch?" + rng + "&format=csv", "ADMIN", nil,
			http.StatusBadRequest, `"field":"format"`, true},
		{"trips csv rejected", base + "/reports/trips?" + rng + "&format=csv", "ATM-SPV", nil, http.StatusBadRequest, `Ekspor CSV dipindah ke menu ekspor`, true},
		{"csv rejected before params and service errors", base + "/reports/trips?date_from=bad&format=csv", "ATM-SPV", service.ErrNotAuthorized,
			http.StatusBadRequest, `"field":"format"`, true},
		{"service denies", base + "/summary?" + rng, "ATM-USER", service.ErrNotAuthorized, http.StatusForbidden, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := testTokenService()
			fake := &fakeReplenishmentReader{err: tc.svcErr}
			r := chi.NewRouter()
			r.With(custommw.RequireAuth(ts)).Mount(base, NewReplenishmentHandler(fake).Routes())
			token := tokenForRole(t, ts, 1, tc.role)
			if tc.role == "VENDOR-USER" {
				token = tokenForVendor(t, ts, 9, 42)
			}
			rec := doRequest(r, http.MethodGet, tc.path, token, "")
			if rec.Code != tc.wantCode {
				t.Fatalf("code = %d, want %d: %s", rec.Code, tc.wantCode, rec.Body.String())
			}
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Errorf("body %q does not contain %q", rec.Body.String(), tc.wantBody)
			}
			if strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv") {
				t.Errorf("must never answer text/csv, got %q", rec.Header().Get("Content-Type"))
			}
			if tc.noReportCall && fake.calls != 0 {
				t.Errorf("service reached %d time(s) on a rejected csv request", fake.calls)
			}
			if tc.name == "list" {
				f := fake.filter
				if f.Classification == nil || *f.Classification != "late" || f.VendorID == nil || *f.VendorID != 2 || !f.OnlyDiff ||
					formatDate(f.From) != "2026-10-01" || formatDate(f.To) != "2026-10-14" {
					t.Errorf("filter = %+v", f)
				}
			}
		})
	}
}

func TestVendorOrderHandler_ListRealizations(t *testing.T) {
	ts := testTokenService()
	fake := &fakeVendorOrderServicer{}
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(ts)).Mount("/v", NewVendorOrderHandler(fake).RealizationRoutes())
	// vendor_id / region_code / only_diff are internal-only: ignored for the vendor.
	rec := doRequest(r, http.MethodGet, "/v?date_from=2026-10-01&date_to=2026-10-14&vendor_id=99&only_diff=true", tokenForVendor(t, ts, 9, 42), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d: %s", rec.Code, rec.Body.String())
	}
	if f := fake.realFilter; f.VendorID != nil || f.OnlyDiff || formatDate(f.To) != "2026-10-14" {
		t.Fatalf("filter = %+v", f)
	}
	if rec := doRequest(r, http.MethodGet, "/v", tokenForRole(t, ts, 1, "ATM-SPV"), ""); rec.Code != http.StatusForbidden {
		t.Fatalf("internal role: code = %d, want 403", rec.Code)
	}
}
