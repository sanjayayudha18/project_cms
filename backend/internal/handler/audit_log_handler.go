package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
)

// wibZone is Asia/Jakarta (WIB, UTC+7). Asia/Jakarta has no DST, so a fixed
// offset is exact and — unlike time.LoadLocation — doesn't depend on the
// container image shipping IANA tzdata (project images are distroless/alpine).
var wibZone = time.FixedZone("WIB", 7*60*60)

// AuditLogHandler handles the read-only Audit Log Viewer HTTP endpoints.
type AuditLogHandler struct {
	service service.AuditLogServicer
}

// NewAuditLogHandler creates a new AuditLogHandler with the given service.
func NewAuditLogHandler(svc service.AuditLogServicer) *AuditLogHandler {
	return &AuditLogHandler{service: svc}
}

// Routes returns a chi.Router with the Audit Log Viewer endpoints mounted.
func (h *AuditLogHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Get("/{id}", h.GetByID)
	return r
}

// List handles GET /api/v1/audit-logs — paginated, filtered audit_logs list.
// The list response omits before/after to keep pages light (design.md).
func (h *AuditLogHandler) List(w http.ResponseWriter, r *http.Request) {
	params, err := parseListAuditLogsParams(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	result, err := h.service.List(r.Context(), params)
	if err != nil {
		h.handleAuditLogServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toAuditLogListResponse(result))
}

// GetByID handles GET /api/v1/audit-logs/{id} — a single entry with the
// full before/after payload.
func (h *AuditLogHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "id harus berupa angka")
		return
	}

	detail, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		h.handleAuditLogServiceError(w, err)
		return
	}
	if detail == nil {
		writeError(w, http.StatusNotFound, "not_found", "Log audit tidak ditemukan")
		return
	}

	writeJSON(w, http.StatusOK, toAuditLogDetailResponse(detail))
}

// parseListAuditLogsParams parses query params into service.ListAuditLogsParams.
// page/page_size are left at 0 when absent — the service clamps/floors them
// (Property 3), unlike the ATM Portal handler's own default-application
// convention, because design.md puts that responsibility in the service here.
func parseListAuditLogsParams(q url.Values) (service.ListAuditLogsParams, error) {
	page, err := parseIntParam(q, "page", 0)
	if err != nil {
		return service.ListAuditLogsParams{}, fmt.Errorf("page harus berupa angka")
	}
	pageSize, err := parseIntParam(q, "page_size", 0)
	if err != nil {
		return service.ListAuditLogsParams{}, fmt.Errorf("page_size harus berupa angka")
	}

	actorID, err := parseOptionalInt64Param(q, "actor_id")
	if err != nil {
		return service.ListAuditLogsParams{}, fmt.Errorf("actor_id harus berupa angka")
	}
	entityID, err := parseOptionalInt64Param(q, "entity_id")
	if err != nil {
		return service.ListAuditLogsParams{}, fmt.Errorf("entity_id harus berupa angka")
	}

	// date_to as a bare YYYY-MM-DD is end-of-day WIB so the inclusive
	// created_at <= date_to filter actually covers that whole calendar day;
	// an explicit RFC3339 timestamp (caller supplied the exact instant) is
	// used as-is.
	dateFrom, err := parseAuditLogDate("date_from", q.Get("date_from"), false)
	if err != nil {
		return service.ListAuditLogsParams{}, err
	}
	dateTo, err := parseAuditLogDate("date_to", q.Get("date_to"), true)
	if err != nil {
		return service.ListAuditLogsParams{}, err
	}

	return service.ListAuditLogsParams{
		ActorID:    actorID,
		Action:     optionalStringParam(q, "action"),
		EntityType: optionalStringParam(q, "entity_type"),
		EntityID:   entityID,
		DateFrom:   dateFrom,
		DateTo:     dateTo,
		Page:       int32(page),
		PageSize:   int32(pageSize),
	}, nil
}

// parseOptionalInt64Param returns nil when the query param is absent/empty,
// otherwise the parsed int64 or an error if it isn't a valid integer.
func parseOptionalInt64Param(q url.Values, key string) (*int64, error) {
	raw := q.Get(key)
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// optionalStringParam returns nil when the query param is absent/empty.
func optionalStringParam(q url.Values, key string) *string {
	raw := q.Get(key)
	if raw == "" {
		return nil
	}
	return &raw
}

// parseAuditLogDate parses a date_from/date_to value as YYYY-MM-DD or
// RFC3339, interpreting a bare YYYY-MM-DD in WIB and converting to UTC.
// When endOfDay is true, a bare date is anchored to 23:59:59.999999999 WIB
// instead of midnight.
func parseAuditLogDate(field, raw string, endOfDay bool) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", raw, wibZone); err == nil {
		if endOfDay {
			t = t.Add(24*time.Hour - time.Nanosecond)
		}
		u := t.UTC()
		return &u, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		u := t.UTC()
		return &u, nil
	}
	return nil, fmt.Errorf("%s harus berformat YYYY-MM-DD atau RFC3339", field)
}

// handleAuditLogServiceError maps service package errors to HTTP responses
// per design.md's error table: ValidationError -> 400, anything else -> 500.
func (h *AuditLogHandler) handleAuditLogServiceError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError
	if errors.As(err, &validationErr) {
		writeError(w, http.StatusBadRequest, "bad_request",
			fmt.Sprintf("%s %s", validationErr.Field, validationErr.Message))
		return
	}

	writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
}

// auditLogListRow is one row in the list response, matching design.md's
// field names exactly. before/after are intentionally absent here.
type auditLogListRow struct {
	ID         int64   `json:"id"`
	ActorID    int64   `json:"actor_id"`
	Action     string  `json:"action"`
	EntityType string  `json:"entity_type"`
	EntityID   int64   `json:"entity_id"`
	IP         *string `json:"ip"`
	CreatedAt  string  `json:"created_at"`
}

// auditLogListResponse is the flat JSON body for GET /api/v1/audit-logs.
type auditLogListResponse struct {
	Data     []auditLogListRow `json:"data"`
	Page     int32             `json:"page"`
	PageSize int32             `json:"page_size"`
	Total    int64             `json:"total"`
}

// auditLogDetailResponse is the flat JSON body for GET /api/v1/audit-logs/{id}.
type auditLogDetailResponse struct {
	auditLogListRow
	Before json.RawMessage `json:"before"`
	After  json.RawMessage `json:"after"`
}

func toAuditLogListRow(item service.AuditLogListItem) auditLogListRow {
	return auditLogListRow{
		ID:         item.ID,
		ActorID:    item.ActorID,
		Action:     item.Action,
		EntityType: item.EntityType,
		EntityID:   item.EntityID,
		IP:         item.IP,
		CreatedAt:  item.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toAuditLogListResponse(result *service.ListAuditLogsResult) auditLogListResponse {
	data := make([]auditLogListRow, len(result.Items))
	for i, item := range result.Items {
		data[i] = toAuditLogListRow(item)
	}
	return auditLogListResponse{
		Data:     data,
		Page:     result.Page,
		PageSize: result.PageSize,
		Total:    result.Total,
	}
}

func toAuditLogDetailResponse(detail *service.AuditLogDetail) auditLogDetailResponse {
	return auditLogDetailResponse{
		auditLogListRow: toAuditLogListRow(detail.AuditLogListItem),
		Before:          detail.Before,
		After:           detail.After,
	}
}
