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

// CashCountPicServicer is the subset of service.CashCountPicAdminService the handler needs.
type CashCountPicServicer interface {
	List(ctx context.Context, picRole, status string) ([]db.ListCashCountPicsAdminRow, error)
	Create(ctx context.Context, makerID int64, req service.CashCountPicPayload, actorIP string) (db.MasterDataChangeRequest, error)
	Update(ctx context.Context, makerID, id int64, req service.CashCountPicUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error)
	Disable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error)
	Enable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error)
}

// CashCountPicHandler serves "Mapping PIC" cash count (ScDD 5.1, AC-5.1-09/10).
// Mount at /api/v1/cash-count/pics behind RequireAuth + RequireRoles(CC-CWM, ADMIN) -- see
// cmd/api/main.go (task 11.1). Writes are re-checked (CC-CWM only) in the service's Submit.
type CashCountPicHandler struct {
	svc CashCountPicServicer
}

// NewCashCountPicHandler creates a CashCountPicHandler.
func NewCashCountPicHandler(svc CashCountPicServicer) *CashCountPicHandler {
	return &CashCountPicHandler{svc: svc}
}

// Routes returns the router for the PIC mapping endpoints.
func (h *CashCountPicHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Put("/{id}", h.Update)
	r.Post("/{id}/disable", h.Disable)
	r.Post("/{id}/enable", h.Enable)
	return r
}

// List handles GET /?status=active|disabled|all&pic_role=COORDINATOR|EXECUTOR.
func (h *CashCountPicHandler) List(w http.ResponseWriter, r *http.Request) {
	status, err := parseStatusParam(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	picRole := r.URL.Query().Get("pic_role")
	if picRole != "" && picRole != "COORDINATOR" && picRole != "EXECUTOR" {
		writeError(w, http.StatusBadRequest, "bad_request", "pic_role tidak valid")
		return
	}
	rows, err := h.svc.List(r.Context(), picRole, status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	items := make([]map[string]any, len(rows))
	for i, p := range rows {
		items[i] = map[string]any{
			"id": p.ID, "pic_role": p.PicRole, "user_id": p.UserID, "username": p.Username, "full_name": p.FullName,
			"region_code": p.RegionCode, "area_id": p.AreaID, "area_code": p.AreaCode, "area_name": p.AreaName,
			"is_active": p.IsActive, "deleted_at": formatTimestamptz(p.DeletedAt),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"pics": items})
}

// Create handles POST / {pic_role, user_id, region_code|area_id} -- 202 pending approval.
func (h *CashCountPicHandler) Create(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	var body service.CashCountPicPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	change, err := h.svc.Create(r.Context(), authCtx.UserID, body, extractClientIP(r))
	h.respond(w, change, err)
}

// Update handles PUT /{id} {user_id, region_code|area_id} -- 202 pending approval.
func (h *CashCountPicHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	var body service.CashCountPicUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	change, err := h.svc.Update(r.Context(), authCtx.UserID, id, body, extractClientIP(r))
	h.respond(w, change, err)
}

// Disable handles POST /{id}/disable -- 202 pending approval.
func (h *CashCountPicHandler) Disable(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.svc.Disable)
}

// Enable handles POST /{id}/enable -- 202 pending approval.
func (h *CashCountPicHandler) Enable(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.svc.Enable)
}

func (h *CashCountPicHandler) toggle(w http.ResponseWriter, r *http.Request, fn func(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error)) {
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

func (h *CashCountPicHandler) respond(w http.ResponseWriter, change db.MasterDataChangeRequest, err error) {
	var validationErr *service.ValidationError
	switch {
	case err == nil:
		writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
	case errors.As(err, &validationErr):
		writeValidationError(w, validationErr.Field, validationErr.Message)
	case errors.Is(err, service.ErrCashCountPicNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, service.ErrCashCountPicConflict), errors.Is(err, service.ErrMasterDataChangePending):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, service.ErrMasterDataForbidden):
		writeForbidden(w, "Anda tidak berhak mengubah mapping PIC cash count")
	default:
		writeUnexpectedError(w, err)
	}
}
