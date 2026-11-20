package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// PksListServicer is the subset of service.PksListService the handler needs.
type PksListServicer interface {
	List(ctx context.Context, role string, p service.PksListParams) (*service.PksListPage, error)
}

// PksReminderServicer is the subset of service.VendorPksAdminService the reminder endpoint needs.
type PksReminderServicer interface {
	UpdateReminder(ctx context.Context, actorID int64, role string, pksID int64, days int32, ip string) (db.UpdateVendorPksRenewalDaysRow, error)
}

// AdminPksHandler serves Daftar PKS (spec vendor-pks-list Req 2, 4).
// Mount at /api/v1/admin/pks behind RequireAuth + RequireRoles(reports.PksViewerRoles...);
// the reminder PUT is narrowed to ADMIN/ADMIN_PARAM here and re-checked in the service.
type AdminPksHandler struct {
	list     PksListServicer
	reminder PksReminderServicer
}

// NewAdminPksHandler creates an AdminPksHandler.
func NewAdminPksHandler(list PksListServicer, reminder PksReminderServicer) *AdminPksHandler {
	return &AdminPksHandler{list: list, reminder: reminder}
}

// Routes returns the router for the Daftar PKS endpoints.
func (h *AdminPksHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.With(middleware.RequireRoles("ADMIN", "ADMIN_PARAM")).Put("/{pksID}/reminder", h.UpdateReminder)
	return r
}

// List handles GET /?status=&pks_type=&vendor=&q=&page=&page_size= -> {items, total, page, page_size}.
func (h *AdminPksHandler) List(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	q := r.URL.Query()
	page, err := optionalInt(q, "page")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "page tidak valid")
		return
	}
	pageSize, err := optionalInt(q, "page_size")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "page_size tidak valid")
		return
	}
	res, err := h.list.List(r.Context(), authCtx.Role, service.PksListParams{
		Status: q.Get("status"), PksType: q.Get("pks_type"), Vendor: q.Get("vendor"), Q: q.Get("q"),
		Page: page, PageSize: pageSize,
	})
	if err != nil {
		handleVendorPksError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

type pksReminderBody struct {
	RenewalDays int32 `json:"renewal_days"`
}

// UpdateReminder handles PUT /{pksID}/reminder {renewal_days} -> 200 {pks}. Applies
// immediately with audit in the same tx (documented GR#3 deviation, D9), not 202.
func (h *AdminPksHandler) UpdateReminder(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	pksID, err := parsePathID(r, "pksID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var body pksReminderBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	row, err := h.reminder.UpdateReminder(r.Context(), authCtx.UserID, authCtx.Role, pksID, body.RenewalDays, extractClientIP(r))
	if err != nil {
		handleVendorPksError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pks": row})
}

// optionalInt parses an optional integer query param; absent = 0 (service default).
func optionalInt(q url.Values, key string) (int, error) {
	v := q.Get(key)
	if v == "" {
		return 0, nil
	}
	return strconv.Atoi(v)
}
