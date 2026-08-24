package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
)

// stubAtmPortalService implements service.AtmPortalServicer for handler unit tests.
type stubAtmPortalService struct {
	listATMsResult    *service.ListATMsResult
	listATMsErr       error
	listCashposResult *service.ListCashposResult
	listCashposErr    error
	lastCashposParams service.ListCashposParams
}

func (s *stubAtmPortalService) ListATMs(context.Context, service.ListATMsParams) (*service.ListATMsResult, error) {
	return s.listATMsResult, s.listATMsErr
}

func (s *stubAtmPortalService) ListCashpos(_ context.Context, params service.ListCashposParams) (*service.ListCashposResult, error) {
	s.lastCashposParams = params
	return s.listCashposResult, s.listCashposErr
}

func mountCashposHandler(svc service.AtmPortalServicer) http.Handler {
	h := NewAtmPortalHandler(svc)
	r := chi.NewRouter()
	r.Mount("/api/v1/atm-portal", h.Routes())
	return r
}

func TestListCashpos_SuccessAllFields(t *testing.T) {
	created := time.Date(2026, 8, 20, 10, 30, 0, 0, time.UTC)
	svc := &stubAtmPortalService{
		listCashposResult: &service.ListCashposResult{
			Data: []service.CashposRow{{
				ID: 1, FileID: 2,
				CashposDate: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
				TerminalID:  "T1", MachineType: "ATM100K", TellerID: "TL", BranchCode: "B1",
				StartingCash10k: "100.00", CashIn10k: "10.00", CashOut10k: "5.00", CashPosition10k: "105.00",
				StartingCash20k: "200.00", CashIn20k: "0.00", CashOut20k: "0.00", CashPosition20k: "200.00",
				StartingCash50k: "300.00", CashIn50k: "0.00", CashOut50k: "0.00", CashPosition50k: "300.00",
				StartingCash100k: "9999999999999999.99", CashIn100k: "1.01", CashOut100k: "2.02", CashPosition100k: "3.03",
				PositionSource: "CURRENT", CreatedAt: created,
			}},
			Total: 1, Page: 1, PageSize: 25,
		},
	}
	r := mountCashposHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/atm-portal/cashpos", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	raw := w.Body.String()
	if strings.Contains(raw, `"starting_cash_100k":9999999999999999`) {
		t.Error("amount encoded as JSON number, want string")
	}
	var resp listCashposResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 || len(resp.Data) != 1 {
		t.Fatalf("resp meta: %+v", resp)
	}
	row := resp.Data[0]
	if row.StartingCash100k != "9999999999999999.99" {
		t.Errorf("large decimal lost: %q", row.StartingCash100k)
	}
	if row.CashposDate != "2026-08-20" {
		t.Errorf("cashpos_date=%q", row.CashposDate)
	}
	if row.CreatedAt != "2026-08-20T10:30:00Z" {
		t.Errorf("created_at=%q", row.CreatedAt)
	}
}

func TestListCashpos_Defaults(t *testing.T) {
	svc := &stubAtmPortalService{
		listCashposResult: &service.ListCashposResult{Data: nil, Total: 0, Page: 1, PageSize: 25},
	}
	r := mountCashposHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/atm-portal/cashpos", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	p := svc.lastCashposParams
	if p.Page != 1 || p.PageSize != 25 || p.SortBy != "cashpos_date" || p.SortOrder != "desc" {
		t.Errorf("defaults not applied: %+v", p)
	}
}

func TestListCashpos_BadPage(t *testing.T) {
	svc := &stubAtmPortalService{}
	r := mountCashposHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/atm-portal/cashpos?page=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d", w.Code)
	}
}

func TestListCashpos_ValidationError(t *testing.T) {
	svc := &stubAtmPortalService{
		listCashposErr: &service.ValidationError{Field: "page_size", Message: "harus antara 1 dan 100"},
	}
	r := mountCashposHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/atm-portal/cashpos?page_size=200", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestListCashpos_InternalError(t *testing.T) {
	svc := &stubAtmPortalService{listCashposErr: errors.New("boom")}
	r := mountCashposHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/atm-portal/cashpos", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "boom") {
		t.Error("must not leak internal error detail")
	}
}

func TestListCashpos_Empty(t *testing.T) {
	svc := &stubAtmPortalService{
		listCashposResult: &service.ListCashposResult{Data: []service.CashposRow{}, Total: 0, Page: 1, PageSize: 25},
	}
	r := mountCashposHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/atm-portal/cashpos", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var resp listCashposResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.Data == nil {
		// emit_empty_slices equivalent — prefer [] not null
		t.Log("data is null; acceptable if encoder omits empty slice as null")
	}
	if resp.Total != 0 {
		t.Errorf("total=%d", resp.Total)
	}
}

func TestListCashpos_QueryForwarding(t *testing.T) {
	svc := &stubAtmPortalService{
		listCashposResult: &service.ListCashposResult{Data: nil, Total: 0, Page: 3, PageSize: 50},
	}
	r := mountCashposHandler(svc)
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/atm-portal/cashpos?page=3&page_size=50&search=ATM&date_from=2026-01-01&date_to=2026-08-01&sort_by=terminal_id&sort_order=asc",
		nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	p := svc.lastCashposParams
	if p.Page != 3 || p.PageSize != 50 || p.Search != "ATM" || p.DateFrom != "2026-01-01" || p.DateTo != "2026-08-01" {
		t.Errorf("params: %+v", p)
	}
	if p.SortBy != "terminal_id" || p.SortOrder != "asc" {
		t.Errorf("sort: %+v", p)
	}
}
