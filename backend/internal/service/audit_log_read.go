package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/repository"
)

const (
	defaultAuditLogPageSize int32 = 25
	maxAuditLogPageSize     int32 = 100
)

// ListAuditLogsParams holds the parsed request parameters for listing audit
// logs. Page/PageSize are the raw parsed values (0 when the query param was
// absent or non-positive) — List clamps them; it does not error on them
// (Property 3). DateFrom/DateTo are already converted to UTC by the handler.
type ListAuditLogsParams struct {
	ActorID    *int64
	Action     *string
	EntityType *string
	EntityID   *int64
	DateFrom   *time.Time
	DateTo     *time.Time
	Page       int32
	PageSize   int32
}

// AuditLogListItem is one row of a list page (before/after omitted — see
// AuditLogDetail).
type AuditLogListItem struct {
	ID         int64
	ActorID    int64
	Action     string
	EntityType string
	EntityID   int64
	IP         *string
	CreatedAt  time.Time
}

// AuditLogDetail is a single entry with the full before/after payload.
type AuditLogDetail struct {
	AuditLogListItem
	Before json.RawMessage
	After  json.RawMessage
}

// ListAuditLogsResult is the assembled paginated list result.
type ListAuditLogsResult struct {
	Items    []AuditLogListItem
	Page     int32
	PageSize int32
	Total    int64
}

// AuditLogServicer is the interface AuditLogHandler depends on (Task 4),
// matching the AtmPortalServicer/DmaaForecastServicer convention.
type AuditLogServicer interface {
	List(ctx context.Context, params ListAuditLogsParams) (*ListAuditLogsResult, error)
	GetByID(ctx context.Context, id int64) (*AuditLogDetail, error)
}

// AuditLogRepository is the small interface AuditLogReadService depends on,
// satisfied by *repository.AuditLogRepository (Task 2).
type AuditLogRepository interface {
	List(ctx context.Context, f repository.AuditLogFilter, limit, offset int32) ([]db.AuditLog, error)
	Count(ctx context.Context, f repository.AuditLogFilter) (int64, error)
	GetByID(ctx context.Context, id int64) (*db.AuditLog, error)
}

// AuditLogReadService implements the read-only Audit Log Viewer use cases:
// validation, pagination clamping, and row -> response mapping. audit_logs
// stays append-only — this type defines no write method.
type AuditLogReadService struct {
	repo AuditLogRepository
}

// NewAuditLogReadService creates an AuditLogReadService wrapping the given repository.
func NewAuditLogReadService(repo AuditLogRepository) *AuditLogReadService {
	return &AuditLogReadService{repo: repo}
}

// clampAuditLogPage floors page at 1 (Property 3 covers page_size only, but
// the same floor-not-error policy applies to page per design.md).
func clampAuditLogPage(page int32) int32 {
	if page < 1 {
		return 1
	}
	return page
}

// clampAuditLogPageSize brings pageSize into [1, maxAuditLogPageSize],
// defaulting to defaultAuditLogPageSize when absent (0) or non-positive.
func clampAuditLogPageSize(pageSize int32) int32 {
	if pageSize <= 0 {
		return defaultAuditLogPageSize
	}
	if pageSize > maxAuditLogPageSize {
		return maxAuditLogPageSize
	}
	return pageSize
}

// List validates params, clamps pagination, and assembles a paginated page
// of audit_logs matching the given filters.
func (s *AuditLogReadService) List(ctx context.Context, params ListAuditLogsParams) (*ListAuditLogsResult, error) {
	if params.DateFrom != nil && params.DateTo != nil && params.DateFrom.After(*params.DateTo) {
		return nil, &ValidationError{Field: "date_from", Message: "tidak boleh lebih besar dari date_to"}
	}

	page := clampAuditLogPage(params.Page)
	pageSize := clampAuditLogPageSize(params.PageSize)
	offset := (page - 1) * pageSize

	filter := repository.AuditLogFilter{
		ActorID:    params.ActorID,
		Action:     params.Action,
		EntityType: params.EntityType,
		EntityID:   params.EntityID,
		DateFrom:   params.DateFrom,
		DateTo:     params.DateTo,
	}

	rows, err := s.repo.List(ctx, filter, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("listing audit logs: %w", err)
	}
	total, err := s.repo.Count(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("counting audit logs: %w", err)
	}

	items := make([]AuditLogListItem, len(rows))
	for i, row := range rows {
		items[i] = toAuditLogListItem(row)
	}

	return &ListAuditLogsResult{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

// GetByID retrieves a single audit log entry with its full before/after
// payload. Returns nil, nil when no matching row exists (handler maps that
// to 404).
func (s *AuditLogReadService) GetByID(ctx context.Context, id int64) (*AuditLogDetail, error) {
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting audit log %d: %w", id, err)
	}
	if row == nil {
		return nil, nil
	}
	return toAuditLogDetail(*row), nil
}

func toAuditLogListItem(row db.AuditLog) AuditLogListItem {
	return AuditLogListItem{
		ID:         row.ID,
		ActorID:    row.ActorID,
		Action:     row.Action,
		EntityType: row.EntityType,
		EntityID:   row.EntityID,
		IP:         row.IP,
		CreatedAt:  row.CreatedAt.Time,
	}
}

// toAuditLogDetail maps before/after as raw JSON bytes (json.RawMessage is
// itself just []byte) so the handler's json.Marshal emits them as JSON
// objects/null, never as escaped strings (Property 7).
func toAuditLogDetail(row db.AuditLog) *AuditLogDetail {
	return &AuditLogDetail{
		AuditLogListItem: toAuditLogListItem(row),
		Before:           json.RawMessage(row.Before),
		After:            json.RawMessage(row.After),
	}
}
