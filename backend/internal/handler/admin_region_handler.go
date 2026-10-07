package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// RegionAdminServicer is the subset of service.RegionAdminService the
// handler needs: List/Count/Get (read-only) plus
// Create/UpdateName/Disable/Enable (Req 2-4). Unlike ATMAdminServicer,
// mutations apply immediately (no maker-checker, design.md "Documented
// Deviation") and return the region row directly, not a pending change.
type RegionAdminServicer interface {
	List(ctx context.Context, arg db.ListRegionsAdminParams) ([]db.ListRegionsAdminRow, error)
	Count(ctx context.Context, arg db.CountRegionsAdminParams) (int64, error)
	Get(ctx context.Context, id int64) (*db.GetRegionAdminByIDRow, error)
	Create(ctx context.Context, actorID int64, actorRole string, req service.CreateRegionRequest, actorIP string) (db.Region, error)
	UpdateName(ctx context.Context, actorID int64, actorRole string, id int64, req service.UpdateRegionNameRequest, actorIP string) (db.Region, error)
	Disable(ctx context.Context, actorID int64, actorRole string, id int64, actorIP string) (db.Region, error)
	Enable(ctx context.Context, actorID int64, actorRole string, id int64, actorIP string) (db.Region, error)
}

// AdminRegionHandler handles ADMIN/ADMIN_PARAM-only region management
// endpoints: list/get/create/update-name/disable/enable. Mount behind
// RequireAuth + RequireRoles("ADMIN", "ADMIN_PARAM") -- see cmd/api/main.go.
type AdminRegionHandler struct {
	svc RegionAdminServicer
}

// NewAdminRegionHandler creates a new AdminRegionHandler with the given dependency.
func NewAdminRegionHandler(svc RegionAdminServicer) *AdminRegionHandler {
	return &AdminRegionHandler{svc: svc}
}

// Routes returns a chi.Router with all admin region endpoints mounted.
func (h *AdminRegionHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.UpdateName)
	r.Post("/{id}/disable", h.Disable)
	r.Post("/{id}/enable", h.Enable)
	return r
}

// parseRegionPageParams parses page/page_size, defaulting to 1/20 (Req 1.2)
// and capping page_size at 100. Distinct from the shared parsePageParams
// (admin_user_handler.go), which defaults page_size to 25 -- region's own
// requirement (1.2) states 20.
func parseRegionPageParams(q url.Values) (page, pageSize int, err error) {
	page, pageSize = 1, 20
	if v := q.Get("page"); v != "" {
		page, err = strconv.Atoi(v)
		if err != nil || page < 1 {
			return 0, 0, errors.New("page tidak valid")
		}
	}
	if v := q.Get("page_size"); v != "" {
		pageSize, err = strconv.Atoi(v)
		if err != nil || pageSize < 1 {
			return 0, 0, errors.New("page_size tidak valid")
		}
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize, nil
}

// parseRegionStatusParam parses status, defaulting to "active" (consistent
// with the other admin list screens' default). Distinct from the shared
// parseStatusParam, whose vocabulary is active/disabled/all -- regions_admin.sql
// uses active/inactive/all.
func parseRegionStatusParam(q url.Values) (string, error) {
	status := q.Get("status")
	if status == "" {
		return "active", nil
	}
	if status != "active" && status != "inactive" && status != "all" {
		return "", errors.New("status tidak valid")
	}
	return status, nil
}

// List handles GET / -- filterable, paginated region list (Req 1.1-1.8).
func (h *AdminRegionHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	page, pageSize, err := parseRegionPageParams(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	status, err := parseRegionStatusParam(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var qParam *string
	if v := q.Get("q"); v != "" {
		qParam = &v
	}

	regions, err := h.svc.List(r.Context(), db.ListRegionsAdminParams{
		Q: qParam, Status: status,
		PageLimit: int64(pageSize), PageOffset: int64((page - 1) * pageSize),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	total, err := h.svc.Count(r.Context(), db.CountRegionsAdminParams{Q: qParam, Status: status})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}

	items := make([]map[string]any, len(regions))
	for i, reg := range regions {
		items[i] = listRegionRowToResponse(reg)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"regions": items, "page": page, "page_size": pageSize, "total": total,
	})
}

// Get handles GET /{id}.
func (h *AdminRegionHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	region, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.handleRegionAdminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, getRegionRowToResponse(*region))
}

type createRegionRequestBody struct {
	Code   string `json:"code"`
	Region string `json:"region"`
}

// Create handles POST / (Req 2.1-2.7) -- applies immediately (201), no
// maker-checker (design.md "Documented Deviation").
func (h *AdminRegionHandler) Create(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	var body createRegionRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}

	created, err := h.svc.Create(r.Context(), authCtx.UserID, authCtx.Role, service.CreateRegionRequest{
		Code: body.Code, Region: body.Region,
	}, extractClientIP(r))
	if err != nil {
		h.handleRegionAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, regionToResponse(created))
}

type updateRegionRequestBody struct {
	Code   *string `json:"code"`
	Region string  `json:"region"`
}

// UpdateName handles PUT /{id} (Req 3.1-3.6) -- applies immediately (200).
// Code is accepted only so an attempt to change it can be detected and
// rejected by the service (Req 3.2) -- omitting it from the body entirely is
// treated the same as resending the current value.
func (h *AdminRegionHandler) UpdateName(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var body updateRegionRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}

	updated, err := h.svc.UpdateName(r.Context(), authCtx.UserID, authCtx.Role, id, service.UpdateRegionNameRequest{
		Code: body.Code, Region: body.Region,
	}, extractClientIP(r))
	if err != nil {
		h.handleRegionAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, regionToResponse(updated))
}

// Disable handles POST /{id}/disable (Req 4.1/4.2, 4.7) -- applies
// immediately (200).
func (h *AdminRegionHandler) Disable(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	updated, err := h.svc.Disable(r.Context(), authCtx.UserID, authCtx.Role, id, extractClientIP(r))
	if err != nil {
		h.handleRegionAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, regionToResponse(updated))
}

// Enable handles POST /{id}/enable (Req 4.3, 4.7) -- applies immediately (200).
func (h *AdminRegionHandler) Enable(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	updated, err := h.svc.Enable(r.Context(), authCtx.UserID, authCtx.Role, id, extractClientIP(r))
	if err != nil {
		h.handleRegionAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, regionToResponse(updated))
}

// handleRegionAdminError maps RegionAdminService errors to HTTP responses
// per design.md's "Error -> HTTP mapping" table.
func (h *AdminRegionHandler) handleRegionAdminError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError

	switch {
	case errors.As(err, &validationErr):
		writeValidationError(w, validationErr.Field, validationErr.Message)
	case errors.Is(err, service.ErrRegionNotFound):
		writeError(w, http.StatusNotFound, "not_found", service.ErrRegionNotFound.Error())
	case errors.Is(err, service.ErrRegionCodeConflict):
		writeError(w, http.StatusConflict, "conflict", service.ErrRegionCodeConflict.Error())
	case errors.Is(err, service.ErrRegionCodeImmutable):
		writeError(w, http.StatusBadRequest, "bad_request", service.ErrRegionCodeImmutable.Error())
	case errors.Is(err, service.ErrRegionHasActiveLocations):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, service.ErrRegionStatusUnchanged):
		writeError(w, http.StatusConflict, "conflict", service.ErrRegionStatusUnchanged.Error())
	case errors.Is(err, service.ErrNotAuthorized):
		writeForbidden(w, "Anda tidak berhak mengubah data region")
	default:
		writeUnexpectedError(w, err)
	}
}

// --- response mapping ------------------------------------------------------

func regionToResponse(reg db.Region) map[string]any {
	return map[string]any{
		"id": reg.ID, "code": reg.Code, "region": reg.Region,
		"is_active": reg.IsActive,
		"created_at": formatTimestamptz(reg.CreatedAt), "updated_at": formatTimestamptz(reg.UpdatedAt),
		"deleted_at": formatTimestamptz(reg.DeletedAt),
	}
}

func listRegionRowToResponse(r db.ListRegionsAdminRow) map[string]any {
	return map[string]any{
		"id": r.ID, "code": r.Code, "region": r.Region,
		"is_active": r.IsActive, "location_count": r.LocationCount,
		"created_at": formatTimestamptz(r.CreatedAt), "updated_at": formatTimestamptz(r.UpdatedAt),
		"deleted_at": formatTimestamptz(r.DeletedAt),
	}
}

func getRegionRowToResponse(r db.GetRegionAdminByIDRow) map[string]any {
	return map[string]any{
		"id": r.ID, "code": r.Code, "region": r.Region,
		"is_active": r.IsActive, "location_count": r.LocationCount,
		"created_at": formatTimestamptz(r.CreatedAt), "updated_at": formatTimestamptz(r.UpdatedAt),
		"deleted_at": formatTimestamptz(r.DeletedAt),
	}
}
