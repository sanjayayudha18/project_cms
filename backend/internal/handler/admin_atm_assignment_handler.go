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

// ATMAssignmentAdminServicer is the subset of service.ATMAssignmentAdminService
// the handler needs. Mutations return a pending db.MasterDataChangeRequest.
type ATMAssignmentAdminServicer interface {
	List(ctx context.Context, arg db.ListATMAssignmentsAdminParams) ([]service.ATMAssignment, error)
	Count(ctx context.Context, arg db.CountATMAssignmentsAdminParams) (int64, error)
	Get(ctx context.Context, atmID, id int64) (*service.ATMAssignment, error)
	Create(ctx context.Context, makerID, atmID int64, req service.ATMAssignmentUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error)
	Update(ctx context.Context, makerID, atmID, id int64, req service.ATMAssignmentUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error)
	Disable(ctx context.Context, makerID, atmID, id int64, actorIP string) (db.MasterDataChangeRequest, error)
	Enable(ctx context.Context, makerID, atmID, id int64, actorIP string) (db.MasterDataChangeRequest, error)
}

// AdminATMAssignmentHandler handles ADMIN/ADMIN_PARAM-only ATM assignment
// (kelolaan) management (T3.5). Mount at /api/v1/admin/atms/{atmID}/assignments
// behind RequireAuth + RequireRoles("ADMIN", "ADMIN_PARAM") -- see cmd/api/main.go.
type AdminATMAssignmentHandler struct {
	svc ATMAssignmentAdminServicer
}

// NewAdminATMAssignmentHandler creates a new AdminATMAssignmentHandler with the given dependency.
func NewAdminATMAssignmentHandler(svc ATMAssignmentAdminServicer) *AdminATMAssignmentHandler {
	return &AdminATMAssignmentHandler{svc: svc}
}

// Routes returns a chi.Router with all admin ATM assignment endpoints mounted.
func (h *AdminATMAssignmentHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Post("/{id}/disable", h.Disable)
	r.Post("/{id}/enable", h.Enable)
	return r
}

func assignmentToResponse(a service.ATMAssignment) map[string]any {
	return map[string]any{
		"id": a.ID, "atm_id": a.ATMID, "vendor_package_id": a.VendorPackageID, "package_code": a.PackageCode,
		"priority_class": a.PriorityClass, "effective_start_date": a.EffectiveStartDate,
		"effective_end_date": a.EffectiveEndDate, "is_active": a.IsActive,
	}
}

// List handles GET / -- paginated assignment history of one ATM (newest first).
func (h *AdminATMAssignmentHandler) List(w http.ResponseWriter, r *http.Request) {
	atmID, err := parsePathID(r, "atmID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	q := r.URL.Query()
	page, pageSize, err := parsePageParams(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	status, err := parseStatusParam(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	items, err := h.svc.List(r.Context(), db.ListATMAssignmentsAdminParams{
		AtmID: atmID, Status: status, PageLimit: int64(pageSize), PageOffset: int64((page - 1) * pageSize),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	total, err := h.svc.Count(r.Context(), db.CountATMAssignmentsAdminParams{AtmID: atmID, Status: status})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}

	out := make([]map[string]any, len(items))
	for i, a := range items {
		out[i] = assignmentToResponse(a)
	}
	writeJSON(w, http.StatusOK, map[string]any{"assignments": out, "page": page, "page_size": pageSize, "total": total})
}

// Get handles GET /{id}.
func (h *AdminATMAssignmentHandler) Get(w http.ResponseWriter, r *http.Request) {
	atmID, err := parsePathID(r, "atmID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	a, err := h.svc.Get(r.Context(), atmID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	if a == nil {
		writeError(w, http.StatusNotFound, "not_found", service.ErrATMAssignmentNotFound.Error())
		return
	}
	writeJSON(w, http.StatusOK, assignmentToResponse(*a))
}

// Create handles POST / -- stages a new assignment for approval (202).
func (h *AdminATMAssignmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	atmID, err := parsePathID(r, "atmID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var body service.ATMAssignmentUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}

	change, err := h.svc.Create(r.Context(), authCtx.UserID, atmID, body, extractClientIP(r))
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
}

// Update handles PUT /{id} -- stages an edit for approval (202).
func (h *AdminATMAssignmentHandler) Update(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	atmID, err := parsePathID(r, "atmID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var body service.ATMAssignmentUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}

	change, err := h.svc.Update(r.Context(), authCtx.UserID, atmID, id, body, extractClientIP(r))
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
}

// Disable handles POST /{id}/disable -- stages a disable for approval (202).
func (h *AdminATMAssignmentHandler) Disable(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.svc.Disable)
}

// Enable handles POST /{id}/enable -- stages a re-enable for approval (202).
func (h *AdminATMAssignmentHandler) Enable(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.svc.Enable)
}

func (h *AdminATMAssignmentHandler) toggle(w http.ResponseWriter, r *http.Request, fn func(context.Context, int64, int64, int64, string) (db.MasterDataChangeRequest, error)) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	atmID, err := parsePathID(r, "atmID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	change, err := fn(r.Context(), authCtx.UserID, atmID, id, extractClientIP(r))
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
}

func (h *AdminATMAssignmentHandler) handleError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeValidationError(w, validationErr.Field, validationErr.Message)
	case errors.Is(err, service.ErrATMAssignmentNotFound):
		writeError(w, http.StatusNotFound, "not_found", service.ErrATMAssignmentNotFound.Error())
	case errors.Is(err, service.ErrAssignmentATMNotFound):
		writeError(w, http.StatusNotFound, "not_found", service.ErrAssignmentATMNotFound.Error())
	case errors.Is(err, service.ErrATMAssignmentOverlap):
		writeError(w, http.StatusConflict, "conflict", service.ErrATMAssignmentOverlap.Error())
	case errors.Is(err, service.ErrMasterDataForbidden):
		writeForbidden(w, "Anda tidak berhak mengubah master data")
	case errors.Is(err, service.ErrMasterDataChangePending):
		writeError(w, http.StatusConflict, "conflict", service.ErrMasterDataChangePending.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
	}
}
