package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/repository"
)

// fakeAuditLogRepo implements AuditLogRepository.
type fakeAuditLogRepo struct {
	listRows   []db.AuditLog
	listErr    error
	count      int64
	countErr   error
	getRow     *db.AuditLog
	getErr     error
	lastLimit  int32
	lastOffset int32
	lastFilter repository.AuditLogFilter
}

func (f *fakeAuditLogRepo) List(_ context.Context, filter repository.AuditLogFilter, limit, offset int32) ([]db.AuditLog, error) {
	f.lastFilter = filter
	f.lastLimit = limit
	f.lastOffset = offset
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listRows, nil
}

func (f *fakeAuditLogRepo) Count(_ context.Context, filter repository.AuditLogFilter) (int64, error) {
	f.lastFilter = filter
	if f.countErr != nil {
		return 0, f.countErr
	}
	return f.count, nil
}

func (f *fakeAuditLogRepo) GetByID(_ context.Context, _ int64) (*db.AuditLog, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getRow, nil
}

func sampleAuditLogRow() db.AuditLog {
	ip := "10.20.1.5"
	return db.AuditLog{
		ID:         4211,
		ActorID:    7,
		Action:     "approval.submit",
		EntityType: "approval_request",
		EntityID:   88,
		Before:     nil,
		After:      []byte(`{"status":"pending"}`),
		IP:         &ip,
		CreatedAt:  pgtype.Timestamptz{Time: time.Date(2026, 9, 8, 2, 14, 33, 0, time.UTC), Valid: true},
	}
}

// TestClampAuditLogPageSize covers Property 3: page_size clamp (0, negative,
// 101 → clamped) and default-when-absent.
func TestClampAuditLogPageSize(t *testing.T) {
	tests := []struct {
		name  string
		input int32
		want  int32
	}{
		{"absent (zero)", 0, defaultAuditLogPageSize},
		{"negative", -5, defaultAuditLogPageSize},
		{"within range", 10, 10},
		{"at max", 100, 100},
		{"over max", 101, 100},
		{"far over max", 1000, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampAuditLogPageSize(tt.input); got != tt.want {
				t.Errorf("clampAuditLogPageSize(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// TestClampAuditLogPage covers "page floor at 1".
func TestClampAuditLogPage(t *testing.T) {
	tests := []struct {
		name  string
		input int32
		want  int32
	}{
		{"absent (zero)", 0, 1},
		{"negative", -3, 1},
		{"already valid", 1, 1},
		{"beyond first page", 7, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampAuditLogPage(tt.input); got != tt.want {
				t.Errorf("clampAuditLogPage(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// TestAuditLogReadService_List_DateRangeValidation covers "date_from >
// date_to → ValidationError".
func TestAuditLogReadService_List_DateRangeValidation(t *testing.T) {
	repo := &fakeAuditLogRepo{}
	svc := NewAuditLogReadService(repo)

	from := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	_, err := svc.List(context.Background(), ListAuditLogsParams{
		Page: 1, PageSize: 25, DateFrom: &from, DateTo: &to,
	})
	if err == nil {
		t.Fatal("List() error = nil, want ValidationError")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("List() error = %v (%T), want *ValidationError", err, err)
	}
	if ve.Field != "date_from" {
		t.Errorf("ValidationError.Field = %q, want date_from", ve.Field)
	}
}

// TestAuditLogReadService_List_EqualDatesAllowed: date_from == date_to is not
// "strictly after", so it must not be rejected (boundary is inclusive).
func TestAuditLogReadService_List_EqualDatesAllowed(t *testing.T) {
	repo := &fakeAuditLogRepo{count: 0}
	svc := NewAuditLogReadService(repo)

	same := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	_, err := svc.List(context.Background(), ListAuditLogsParams{
		Page: 1, PageSize: 25, DateFrom: &same, DateTo: &same,
	})
	if err != nil {
		t.Fatalf("List() error = %v, want nil for equal date_from/date_to", err)
	}
}

// TestAuditLogReadService_List_ClampsAndAssembles verifies clamped
// page/page_size flow into the repository call and the result.
func TestAuditLogReadService_List_ClampsAndAssembles(t *testing.T) {
	repo := &fakeAuditLogRepo{listRows: []db.AuditLog{sampleAuditLogRow()}, count: 120}
	svc := NewAuditLogReadService(repo)

	result, err := svc.List(context.Background(), ListAuditLogsParams{Page: 3, PageSize: 200})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if result.Page != 3 {
		t.Errorf("result.Page = %d, want 3", result.Page)
	}
	if result.PageSize != maxAuditLogPageSize {
		t.Errorf("result.PageSize = %d, want %d (clamped)", result.PageSize, maxAuditLogPageSize)
	}
	if result.Total != 120 {
		t.Errorf("result.Total = %d, want 120", result.Total)
	}
	wantOffset := int32(2) * maxAuditLogPageSize // (page-1) * pageSize
	if repo.lastOffset != wantOffset {
		t.Errorf("repo received offset = %d, want %d", repo.lastOffset, wantOffset)
	}
	if repo.lastLimit != maxAuditLogPageSize {
		t.Errorf("repo received limit = %d, want %d", repo.lastLimit, maxAuditLogPageSize)
	}
	if len(result.Items) != 1 || result.Items[0].ID != 4211 {
		t.Errorf("result.Items = %+v, want one item with ID 4211", result.Items)
	}
}

// TestAuditLogReadService_List_RepoError propagates repository failures.
func TestAuditLogReadService_List_RepoError(t *testing.T) {
	repo := &fakeAuditLogRepo{listErr: errors.New("db down")}
	svc := NewAuditLogReadService(repo)

	_, err := svc.List(context.Background(), ListAuditLogsParams{Page: 1, PageSize: 25})
	if err == nil {
		t.Fatal("List() error = nil, want propagated repo error")
	}
}

// TestAuditLogReadService_GetByID_BeforeAfterPreservedAsJSON covers
// Property 7: before/after are valid JSON, not re-encoded/escaped strings.
func TestAuditLogReadService_GetByID_BeforeAfterPreservedAsJSON(t *testing.T) {
	row := sampleAuditLogRow()
	repo := &fakeAuditLogRepo{getRow: &row}
	svc := NewAuditLogReadService(repo)

	detail, err := svc.GetByID(context.Background(), 4211)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if detail == nil {
		t.Fatal("GetByID() = nil, want detail")
	}
	if detail.Before != nil {
		t.Errorf("Before = %s, want nil (no before recorded)", detail.Before)
	}

	// Marshal the whole detail and confirm "after" renders as a JSON object,
	// not a quoted/escaped string.
	b, err := json.Marshal(struct {
		After json.RawMessage `json:"after"`
	}{After: detail.After})
	if err != nil {
		t.Fatalf("marshal detail: %v", err)
	}
	var roundTrip struct {
		After map[string]any `json:"after"`
	}
	if err := json.Unmarshal(b, &roundTrip); err != nil {
		t.Fatalf("after did not round-trip as a JSON object (got %s): %v", b, err)
	}
	if roundTrip.After["status"] != "pending" {
		t.Errorf("after.status = %v, want pending", roundTrip.After["status"])
	}
}

// TestAuditLogReadService_GetByID_NotFound covers "detail id not found".
func TestAuditLogReadService_GetByID_NotFound(t *testing.T) {
	repo := &fakeAuditLogRepo{getRow: nil}
	svc := NewAuditLogReadService(repo)

	detail, err := svc.GetByID(context.Background(), 999)
	if err != nil {
		t.Fatalf("GetByID() error = %v, want nil", err)
	}
	if detail != nil {
		t.Errorf("GetByID() = %+v, want nil", detail)
	}
}
