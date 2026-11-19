package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// PksTypeAdminServicer is the subset of service.PksTypeAdminService the handler needs.
type PksTypeAdminServicer interface {
	List(ctx context.Context) ([]db.ListPksTypesRow, error)
	Create(ctx context.Context, makerID int64, req service.PksTypeCreatePayload, actorIP string) (db.MasterDataChangeRequest, error)
	Update(ctx context.Context, makerID, id int64, req service.PksTypeUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error)
	Disable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error)
	Enable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error)
}

// AdminPksTypeHandler serves the master "Type PKS" endpoints.
// Mount at /api/v1/admin/pks-types behind masterDataAdmin (RequireAuth + ADMIN/ADMIN_PARAM).
type AdminPksTypeHandler struct {
	svc PksTypeAdminServicer
}

// NewAdminPksTypeHandler creates an AdminPksTypeHandler.
func NewAdminPksTypeHandler(svc PksTypeAdminServicer) *AdminPksTypeHandler {
	return &AdminPksTypeHandler{svc: svc}
}

// Routes returns the router for the type endpoints.
func (h *AdminPksTypeHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Put("/{id}", h.Update)
	r.Post("/{id}/disable", h.Disable)
	r.Post("/{id}/enable", h.Enable)
	return r
}

// List handles GET / -- all types incl. disabled, with in_use.
func (h *AdminPksTypeHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.List(r.Context())
	if err != nil {
		writeUnexpectedError(w, err)
		return
	}
	items := make([]map[string]any, len(rows))
	for i, t := range rows {
		items[i] = map[string]any{
			"id": t.ID, "code": t.Code, "name": t.Name, "is_active": t.IsActive, "in_use": t.InUse,
			"created_at": formatTimestamptz(t.CreatedAt), "updated_at": formatTimestamptz(t.UpdatedAt),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"types": items})
}

// Create handles POST / {code, name} -- 202 pending approval.
func (h *AdminPksTypeHandler) Create(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	var body service.PksTypeCreatePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	change, err := h.svc.Create(r.Context(), authCtx.UserID, body, extractClientIP(r))
	h.respond(w, change, err)
}

// Update handles PUT /{id} {name} -- 202 pending approval.
func (h *AdminPksTypeHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	var body service.PksTypeUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	change, err := h.svc.Update(r.Context(), authCtx.UserID, id, body, extractClientIP(r))
	h.respond(w, change, err)
}

// Disable handles POST /{id}/disable -- 202 pending approval.
func (h *AdminPksTypeHandler) Disable(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.svc.Disable)
}

// Enable handles POST /{id}/enable -- 202 pending approval.
func (h *AdminPksTypeHandler) Enable(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.svc.Enable)
}

func (h *AdminPksTypeHandler) toggle(w http.ResponseWriter, r *http.Request, fn func(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error)) {
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
	change, err := fn(r.Context(), authCtx.UserID, id, extractClientIP(r))
	h.respond(w, change, err)
}

func (h *AdminPksTypeHandler) respond(w http.ResponseWriter, change db.MasterDataChangeRequest, err error) {
	var validationErr *service.ValidationError
	switch {
	case err == nil:
		writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
	case errors.As(err, &validationErr):
		writeValidationError(w, validationErr.Field, validationErr.Message)
	case errors.Is(err, service.ErrPksTypeNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, service.ErrPksTypeConflict), errors.Is(err, service.ErrPksTypeInUse), errors.Is(err, service.ErrMasterDataChangePending):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, service.ErrMasterDataForbidden):
		writeForbidden(w, "Anda tidak berhak mengubah master data")
	default:
		writeUnexpectedError(w, err)
	}
}
