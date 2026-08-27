package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
)

// stubDmaaForecastService implements service.DmaaForecastServicer for handler unit tests.
type stubDmaaForecastService struct {
	result     *service.ListDmaaForecastResult
	err        error
	lastParams service.ListDmaaForecastParams
}

func (s *stubDmaaForecastService) ListDmaaForecast(_ context.Context, params service.ListDmaaForecastParams) (*service.ListDmaaForecastResult, error) {
	s.lastParams = params
	return s.result, s.err
}

func mountDmaaHandler(svc service.DmaaForecastServicer) http.Handler {
	h := NewDmaaForecastHandler(svc)
	r := chi.NewRouter()
	r.Mount("/api/v1/dmaa-forecast", h.Routes())
	return r
}

func dmaaGet(t *testing.T, query string) *httptest.ResponseRecorder {
	t.Helper()
	svc := &stubDmaaForecastService{
		result: &service.ListDmaaForecastResult{
			Data: []service.DmaaForecastRow{{
				TerminalID:      "ATM001",
				DmaaFileID:      42,
				PeriodePred:     time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC),
				Denom:           100000,
				AmountReplenish: 500000000,
				AmountRefund:    0,
				CreatedAt:       time.Date(2026, 8, 25, 10, 30, 0, 0, time.UTC),
			}},
			Total:      1250,
			Page:       1,
			PageSize:   25,
			TotalPages: 50,
		},
	}
	router := mountDmaaHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dmaa-forecast"+query, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestListDmaaForecast_SuccessEnvelope(t *testing.T) {
	rec := dmaaGet(t, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		Data []struct {
			TerminalID      string `json:"terminal_id"`
			PeriodePred     string `json:"periode_pred"`
			AmountReplenish int64  `json:"amount_replenish"`
		} `json:"data"`
		Pagination struct {
			Page       int   `json:"page"`
			PageSize   int   `json:"page_size"`
			TotalRows  int64 `json:"total_rows"`
			TotalPages int   `json:"total_pages"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].TerminalID != "ATM001" {
		t.Errorf("unexpected data: %+v", body.Data)
	}
	if body.Data[0].PeriodePred != "2026-09-15" {
		t.Errorf("periode_pred = %q, want 2026-09-15", body.Data[0].PeriodePred)
	}
	if body.Data[0].AmountReplenish != 500000000 {
		t.Errorf("amount_replenish = %d, want 500000000", body.Data[0].AmountReplenish)
	}
	if body.Pagination.TotalPages != 50 || body.Pagination.TotalRows != 1250 {
		t.Errorf("unexpected pagination: %+v", body.Pagination)
	}
}

func TestListDmaaForecast_DefaultsApplied(t *testing.T) {
	svc := &stubDmaaForecastService{result: &service.ListDmaaForecastResult{}}
	router := mountDmaaHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dmaa-forecast", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if svc.lastParams.Page != 1 {
		t.Errorf("default Page = %d, want 1", svc.lastParams.Page)
	}
	if svc.lastParams.PageSize != 25 {
		t.Errorf("default PageSize = %d, want 25", svc.lastParams.PageSize)
	}
	if svc.lastParams.SortBy != "periode_pred" {
		t.Errorf("default SortBy = %q, want periode_pred", svc.lastParams.SortBy)
	}
	if svc.lastParams.SortOrder != "desc" {
		t.Errorf("default SortOrder = %q, want desc", svc.lastParams.SortOrder)
	}
}

func TestListDmaaForecast_BadRequestCases(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"non-numeric page", "?page=abc"},
		{"non-numeric page_size", "?page_size=xyz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := dmaaGet(t, tt.query)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestListDmaaForecast_ServiceErrors(t *testing.T) {
	validationErr := &service.ValidationError{Field: "sort_by", Message: "invalid"}
	svc := &stubDmaaForecastService{err: validationErr}
	router := mountDmaaHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dmaa-forecast", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("validation error: status = %d, want 400", rec.Code)
	}

	svc = &stubDmaaForecastService{err: errors.New("db down")}
	router = mountDmaaHandler(svc)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/dmaa-forecast", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("db error: status = %d, want 503", rec.Code)
	}
}

func TestParseDmaaForecastParams_PassesFilters(t *testing.T) {
	q := url.Values{}
	q.Set("page", "2")
	q.Set("page_size", "50")
	q.Set("date_from", "2026-08-01")
	q.Set("date_to", "2026-08-31")
	q.Set("terminal_id", "ATM")
	q.Set("sort_by", "denom")
	q.Set("sort_order", "asc")

	params, err := parseDmaaForecastParams(q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if params.Page != 2 || params.PageSize != 50 || params.DateFrom != "2026-08-01" ||
		params.DateTo != "2026-08-31" || params.TerminalID != "ATM" ||
		params.SortBy != "denom" || params.SortOrder != "asc" {
		t.Errorf("params not parsed correctly: %+v", params)
	}
}
