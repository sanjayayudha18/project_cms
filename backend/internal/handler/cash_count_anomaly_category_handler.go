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

// CashCountAnomalyCategoryServicer is the subset of
// service.CashCountAnomalyCategoryAdminService the handler needs.
type CashCountAnomalyCategoryServicer interface {
	List(ctx context.Context, status string) ([]db.CashCountAnomalyCategory, error)
	Create(ctx context.Context, makerID int64, req service.CashCountAnomalyCategoryPayload, actorIP string) (db.MasterDataChangeRequest, error)
	Update(ctx context.Context, makerID, id int64, req service.CashCountAnomalyCategoryUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error)
	Disable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error)
	Enable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error)
}

// CashCountAnomalyCategoryHandler serves "Kategori ATM Anomaly" (ScDD 5.1, AC-5.1-05).
// Mount at /api/v1/cash-count/anomaly-categories behind RequireAuth + RequireRoles
// (CC-CCR maker, ADMIN read) in cmd/api/main.go (task 11.1); the maker role is
// re-checked inside MasterDataChangeService.Submit.
type CashCountAnomalyCategoryHandler struct {
	svc CashCountAnomalyCategoryServicer
}

// NewCashCountAnomalyCategoryHandler creates the handler.
func NewCashCountAnomalyCategoryHandler(svc CashCountAnomalyCategoryServicer) *CashCountAnomalyCategoryHandler {
	return &CashCountAnomalyCategoryHandler{svc: svc}
}

// Routes returns the router for the endpoints.
func (h *CashCountAnomalyCategoryHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Put("/{id}", h.Update)
	r.Post("/{id}/disable", h.Disable)
	r.Post("/{id}/enable", h.Enable)
	return r
}

// List handles GET /?status=active|disabled|all.
func (h *CashCountAnomalyCategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	status, err := parseStatusParam(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	rows, err := h.svc.List(r.Context(), status)
	if err != nil {
		writeUnexpectedError(w, err)
		return
	}
	items := make([]map[string]any, len(rows))
	for i, c := range rows {
		items[i] = map[string]any{
			"id": c.ID, "code": c.Code, "name": c.Name, "description": c.Description,
			"is_active": c.IsActive, "deleted_at": formatTimestamptz(c.DeletedAt),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": items})
}

// Create handles POST / {code, name, description?} -- 202 pending approval.
func (h *CashCountAnomalyCategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	var body service.CashCountAnomalyCategoryPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	change, err := h.svc.Create(r.Context(), authCtx.UserID, body, extractClientIP(r))
	h.respond(w, change, err)
}

// Update handles PUT /{id} {name, description?} -- 202 pending approval.
func (h *CashCountAnomalyCategoryHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	var body service.CashCountAnomalyCategoryUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	change, err := h.svc.Update(r.Context(), authCtx.UserID, id, body, extractClientIP(r))
	h.respond(w, change, err)
}

// Disable handles POST /{id}/disable -- 202 pending approval.
func (h *CashCountAnomalyCategoryHandler) Disable(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.svc.Disable)
}

// Enable handles POST /{id}/enable -- 202 pending approval.
func (h *CashCountAnomalyCategoryHandler) Enable(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.svc.Enable)
}

func (h *CashCountAnomalyCategoryHandler) toggle(w http.ResponseWriter, r *http.Request, fn func(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error)) {
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

func (h *CashCountAnomalyCategoryHandler) respond(w http.ResponseWriter, change db.MasterDataChangeRequest, err error) {
	var validationErr *service.ValidationError
	switch {
	case err == nil:
		writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
	case errors.As(err, &validationErr):
		writeValidationError(w, validationErr.Field, validationErr.Message)
	case errors.Is(err, service.ErrCashCountAnomalyCategoryNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, service.ErrCashCountAnomalyCategoryConflict), errors.Is(err, service.ErrMasterDataChangePending):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, service.ErrMasterDataForbidden):
		writeForbidden(w, "Anda tidak berhak mengubah kategori ATM Anomaly")
	default:
		writeUnexpectedError(w, err)
	}
}
