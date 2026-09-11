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

// stubAuditLogService implements service.AuditLogServicer for handler unit tests.
type stubAuditLogService struct {
	listResult *service.ListAuditLogsResult
	listErr    error
	lastParams service.ListAuditLogsParams

	detail   *service.AuditLogDetail
	getErr   error
	lastID   int64
	sawGetID bool
}

func (s *stubAuditLogService) List(_ context.Context, params service.ListAuditLogsParams) (*service.ListAuditLogsResult, error) {
	s.lastParams = params
	return s.listResult, s.listErr
}

func (s *stubAuditLogService) GetByID(_ context.Context, id int64) (*service.AuditLogDetail, error) {
	s.lastID = id
	s.sawGetID = true
	return s.detail, s.getErr
}

func mountAuditLogHandler(svc service.AuditLogServicer) http.Handler {
	h := NewAuditLogHandler(svc)
	r := chi.NewRouter()
	r.Mount("/api/v1/audit-logs", h.Routes())
	return r
}

func sampleListItem() service.AuditLogListItem {
	ip := "10.20.1.5"
	return service.AuditLogListItem{
		ID:         4211,
		ActorID:    7,
		Action:     "approval.submit",
		EntityType: "approval_request",
		EntityID:   88,
		IP:         &ip,
		CreatedAt:  time.Date(2026, 9, 8, 2, 14, 33, 0, time.UTC),
	}
}

func TestAuditLogList_SuccessEnvelope(t *testing.T) {
	svc := &stubAuditLogService{listResult: &service.ListAuditLogsResult{
		Items:    []service.AuditLogListItem{sampleListItem()},
		Page:     1,
		PageSize: 25,
		Total:    3120,
	}}
	router := mountAuditLogHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body)
	}

	var body struct {
		Data []struct {
			ID         int64   `json:"id"`
			ActorID    int64   `json:"actor_id"`
			Action     string  `json:"action"`
			EntityType string  `json:"entity_type"`
			EntityID   int64   `json:"entity_id"`
			IP         *string `json:"ip"`
			CreatedAt  string  `json:"created_at"`
			Before     *string `json:"before"`
		} `json:"data"`
		Page     int32 `json:"page"`
		PageSize int32 `json:"page_size"`
		Total    int64 `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].ID != 4211 {
		t.Fatalf("unexpected data: %+v", body.Data)
	}
	if body.Data[0].Before != nil {
		t.Errorf("list row should omit before/after, got before=%v", body.Data[0].Before)
	}
	if body.Page != 1 || body.PageSize != 25 || body.Total != 3120 {
		t.Errorf("unexpected pagination: page=%d page_size=%d total=%d", body.Page, body.PageSize, body.Total)
	}
}

func TestAuditLogList_FiltersPassThrough(t *testing.T) {
	svc := &stubAuditLogService{listResult: &service.ListAuditLogsResult{}}
	router := mountAuditLogHandler(svc)

	q := url.Values{}
	q.Set("actor_id", "7")
	q.Set("action", "approval.submit")
	q.Set("entity_type", "approval_request")
	q.Set("entity_id", "88")
	q.Set("date_from", "2026-09-01")
	q.Set("date_to", "2026-09-08")
	q.Set("page", "2")
	q.Set("page_size", "50")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body)
	}
	p := svc.lastParams
	if p.ActorID == nil || *p.ActorID != 7 {
		t.Errorf("ActorID = %v, want 7", p.ActorID)
	}
	if p.Action == nil || *p.Action != "approval.submit" {
		t.Errorf("Action = %v, want approval.submit", p.Action)
	}
	if p.EntityType == nil || *p.EntityType != "approval_request" {
		t.Errorf("EntityType = %v, want approval_request", p.EntityType)
	}
	if p.EntityID == nil || *p.EntityID != 88 {
		t.Errorf("EntityID = %v, want 88", p.EntityID)
	}
	if p.Page != 2 || p.PageSize != 50 {
		t.Errorf("Page/PageSize = %d/%d, want 2/50", p.Page, p.PageSize)
	}
	if p.DateFrom == nil || p.DateTo == nil {
		t.Fatalf("DateFrom/DateTo not parsed: %+v", p)
	}
	// date_from is WIB midnight -> previous-day 17:00 UTC.
	wantFrom := time.Date(2026, 8, 31, 17, 0, 0, 0, time.UTC)
	if !p.DateFrom.Equal(wantFrom) {
		t.Errorf("DateFrom = %v, want %v", p.DateFrom, wantFrom)
	}
	// date_to is WIB end-of-day (23:59:59.999999999) -> UTC.
	wantTo := time.Date(2026, 9, 8, 16, 59, 59, 999999999, time.UTC)
	if !p.DateTo.Equal(wantTo) {
		t.Errorf("DateTo = %v, want %v", p.DateTo, wantTo)
	}
}

func TestAuditLogList_DefaultsWhenAbsent(t *testing.T) {
	svc := &stubAuditLogService{listResult: &service.ListAuditLogsResult{}}
	router := mountAuditLogHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	p := svc.lastParams
	if p.ActorID != nil || p.Action != nil || p.EntityType != nil || p.EntityID != nil || p.DateFrom != nil || p.DateTo != nil {
		t.Errorf("expected all filters nil when absent, got %+v", p)
	}
	if p.Page != 0 || p.PageSize != 0 {
		t.Errorf("Page/PageSize = %d/%d, want 0/0 (service clamps)", p.Page, p.PageSize)
	}
}

func TestAuditLogList_BadRequestCases(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"non-numeric page", "?page=abc"},
		{"non-numeric page_size", "?page_size=xyz"},
		{"non-numeric actor_id", "?actor_id=abc"},
		{"non-numeric entity_id", "?entity_id=abc"},
		{"bad date_from format", "?date_from=01-09-2026"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubAuditLogService{listResult: &service.ListAuditLogsResult{}}
			router := mountAuditLogHandler(svc)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs"+tt.query, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400, body=%s", rec.Code, rec.Body)
			}
		})
	}
}

func TestAuditLogList_DateFromAfterDateTo_ValidationErrorMapsTo400(t *testing.T) {
	svc := &stubAuditLogService{listErr: &service.ValidationError{Field: "date_from", Message: "tidak boleh lebih besar dari date_to"}}
	router := mountAuditLogHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs?date_from=2026-09-10&date_to=2026-09-01", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400, body=%s", rec.Code, rec.Body)
	}
}

func TestAuditLogList_InternalErrorMapsTo500(t *testing.T) {
	svc := &stubAuditLogService{listErr: errors.New("db down")}
	router := mountAuditLogHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestAuditLogGetByID_SuccessIncludesBeforeAfter(t *testing.T) {
	svc := &stubAuditLogService{detail: &service.AuditLogDetail{
		AuditLogListItem: sampleListItem(),
		Before:           nil,
		After:            json.RawMessage(`{"status":"pending"}`),
	}}
	router := mountAuditLogHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs/4211", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body)
	}
	if !svc.sawGetID || svc.lastID != 4211 {
		t.Errorf("service.GetByID called with id=%d, want 4211", svc.lastID)
	}

	var body struct {
		ID     int64          `json:"id"`
		Before *string        `json:"before"`
		After  map[string]any `json:"after"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != 4211 {
		t.Errorf("id = %d, want 4211", body.ID)
	}
	if body.Before != nil {
		t.Errorf("before = %v, want null", body.Before)
	}
	if body.After["status"] != "pending" {
		t.Errorf("after.status = %v, want pending (JSON object, not a string)", body.After["status"])
	}
}

func TestAuditLogGetByID_NonIntegerID(t *testing.T) {
	svc := &stubAuditLogService{}
	router := mountAuditLogHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs/abc", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if svc.sawGetID {
		t.Error("service.GetByID should not be called for a non-integer id")
	}
}

func TestAuditLogGetByID_NotFound(t *testing.T) {
	svc := &stubAuditLogService{detail: nil}
	router := mountAuditLogHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs/999", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
