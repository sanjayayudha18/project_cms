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

// CashCountAreaServicer is the subset of service.CashCountAreaAdminService the handler needs.
type CashCountAreaServicer interface {
	List(ctx context.Context, status string) ([]db.ListCashCountAreasAdminRow, error)
	Create(ctx context.Context, makerID int64, req service.CashCountAreaPayload, actorIP string) (db.MasterDataChangeRequest, error)
	Update(ctx context.Context, makerID, id int64, req service.CashCountAreaUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error)
	Disable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error)
	Enable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error)
}

// CashCountAreaHandler serves /api/v1/cash-count/areas (ScDD 5.1, AC-5.1-08).
// Route-level RequireRoles (CC-CWM maker, ADMIN read) is applied where it is mounted
// (cmd/api/main.go, task 11.1); the service re-checks the maker role in Submit.
type CashCountAreaHandler struct {
	svc CashCountAreaServicer
}

// NewCashCountAreaHandler creates a CashCountAreaHandler.
func NewCashCountAreaHandler(svc CashCountAreaServicer) *CashCountAreaHandler {
	return &CashCountAreaHandler{svc: svc}
}

// Routes returns the router for the area endpoints.
func (h *CashCountAreaHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Put("/{id}", h.Update)
	r.Post("/{id}/disable", h.Disable)
	r.Post("/{id}/enable", h.Enable)
	return r
}

// List handles GET /?status=active|disabled|all.
func (h *CashCountAreaHandler) List(w http.ResponseWriter, r *http.Request) {
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
	for i, a := range rows {
		regions := a.RegionCodes
		if regions == nil {
			regions = []string{}
		}
		items[i] = map[string]any{
			"id": a.ID, "code": a.Code, "name": a.Name, "region_codes": regions,
			"is_active": a.IsActive, "deleted_at": formatTimestamptz(a.DeletedAt),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"areas": items})
}

// Create handles POST / {code, name, region_codes} -- 202 pending approval.
func (h *CashCountAreaHandler) Create(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	var body service.CashCountAreaPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	change, err := h.svc.Create(r.Context(), authCtx.UserID, body, extractClientIP(r))
	h.respond(w, change, err)
}

// Update handles PUT /{id} {name, region_codes} -- 202 pending approval.
func (h *CashCountAreaHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	var body service.CashCountAreaUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	change, err := h.svc.Update(r.Context(), authCtx.UserID, id, body, extractClientIP(r))
	h.respond(w, change, err)
}

// Disable handles POST /{id}/disable -- 202; 409 while active executors remain.
func (h *CashCountAreaHandler) Disable(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.svc.Disable)
}

// Enable handles POST /{id}/enable -- 202 pending approval.
func (h *CashCountAreaHandler) Enable(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.svc.Enable)
}

func (h *CashCountAreaHandler) toggle(w http.ResponseWriter, r *http.Request, fn func(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error)) {
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

func (h *CashCountAreaHandler) respond(w http.ResponseWriter, change db.MasterDataChangeRequest, err error) {
	var validationErr *service.ValidationError
	switch {
	case err == nil:
		writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
	case errors.As(err, &validationErr):
		writeValidationError(w, validationErr.Field, validationErr.Message)
	case errors.Is(err, service.ErrCashCountAreaNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, service.ErrCashCountAreaConflict),
		errors.Is(err, service.ErrCashCountAreaHasExecutors),
		errors.Is(err, service.ErrMasterDataChangePending):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, service.ErrMasterDataForbidden):
		writeForbidden(w, "Anda tidak berhak mengubah area cash count")
	default:
		writeUnexpectedError(w, err)
	}
}
