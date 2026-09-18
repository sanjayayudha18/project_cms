package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// ATMAdminServicer is the subset of service.ATMAdminService the handler
// needs: List/Count/Get/ListLocations (read-only) plus
// Create/Update/Disable/Enable (Req 3-5).
type ATMAdminServicer interface {
	List(ctx context.Context, arg db.ListATMsAdminParams) ([]service.ATM, error)
	Count(ctx context.Context, arg db.CountATMsAdminParams) (int64, error)
	Get(ctx context.Context, id int64) (service.ATM, error)
	Create(ctx context.Context, actorID int64, req service.CreateATMRequest, actorIP string) (service.ATM, error)
	Update(ctx context.Context, actorID, id int64, req service.UpdateATMRequest, actorIP string) (service.ATM, error)
	Disable(ctx context.Context, actorID, id int64, actorIP string) error
	Enable(ctx context.Context, actorID, id int64, actorIP string) error
	ListLocations(ctx context.Context) ([]service.LocationOption, error)
}

// AdminATMHandler handles ADMIN/ADMIN_PARAM-only ATM management endpoints:
// list/get/create/update/disable/enable, plus a read-only locations lookup
// for the form's Location select. Mirrors AdminVendorHandler structurally.
// Mount behind RequireAuth + RequireRoles("ADMIN", "ADMIN_PARAM") --
// see cmd/api/main.go.
type AdminATMHandler struct {
	svc ATMAdminServicer
}

// NewAdminATMHandler creates a new AdminATMHandler with the given dependency.
func NewAdminATMHandler(svc ATMAdminServicer) *AdminATMHandler {
	return &AdminATMHandler{svc: svc}
}

// Routes returns a chi.Router with all admin ATM endpoints mounted.
// /locations is registered before /{id} so chi resolves the static route
// first (same pattern main.go documents for /users/hierarchy vs
// /users/{id}/hierarchy).
func (h *AdminATMHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/locations", h.ListLocations)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Post("/{id}/disable", h.Disable)
	r.Post("/{id}/enable", h.Enable)
	return r
}

// List handles GET / -- filterable, paginated ATM list (Req 8).
func (h *AdminATMHandler) List(w http.ResponseWriter, r *http.Request) {
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
	var qParam, brand, machineType, deploymentType, priorityClass *string
	if v := q.Get("q"); v != "" {
		qParam = &v
	}
	if v := q.Get("brand"); v != "" {
		brand = &v
	}
	if v := q.Get("machine_type"); v != "" {
		machineType = &v
	}
	if v := q.Get("deployment_type"); v != "" {
		deploymentType = &v
	}
	if v := q.Get("priority_class"); v != "" {
		priorityClass = &v
	}
	var locationID *int64
	if v := q.Get("location_id"); v != "" {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "location_id tidak valid")
			return
		}
		locationID = &parsed
	}

	filters := db.ListATMsAdminParams{
		Q: qParam, Brand: brand, MachineType: machineType, DeploymentType: deploymentType,
		PriorityClass: priorityClass, LocationID: locationID, Status: status,
		PageLimit: int64(pageSize), PageOffset: int64((page - 1) * pageSize),
	}
	atms, err := h.svc.List(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	total, err := h.svc.Count(r.Context(), db.CountATMsAdminParams{
		Q: qParam, Brand: brand, MachineType: machineType, DeploymentType: deploymentType,
		PriorityClass: priorityClass, LocationID: locationID, Status: status,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}

	items := make([]map[string]any, len(atms))
	for i, a := range atms {
		items[i] = atmToResponse(a)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"atms": items, "page": page, "page_size": pageSize, "total": total,
	})
}

// ListLocations handles GET /locations -- read-only Location select data
// (Req 6.1-6.3), never audited.
func (h *AdminATMHandler) ListLocations(w http.ResponseWriter, r *http.Request) {
	opts, err := h.svc.ListLocations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	items := make([]map[string]any, len(opts))
	for i, o := range opts {
		items[i] = map[string]any{"id": o.ID, "name": o.Name, "city_or_regency": o.CityOrRegency, "province": o.Province}
	}
	writeJSON(w, http.StatusOK, map[string]any{"locations": items})
}

// Get handles GET /{id}.
func (h *AdminATMHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	atm, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.handleATMAdminError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, atmToResponse(atm))
}

type createATMAdminRequestBody struct {
	TerminalID              string  `json:"terminal_id"`
	LocationID              int64   `json:"location_id"`
	MachineType             string  `json:"machine_type"`
	Brand                   string  `json:"brand"`
	Model                   string  `json:"model"`
	OperationHours          string  `json:"operation_hours"`
	DeploymentType          string  `json:"deployment_type"`
	CapacityAmount          *string `json:"capacity_amount"`
	LowThresholdAmount      *string `json:"low_threshold_amount"`
	CriticalThresholdAmount *string `json:"critical_threshold_amount"`
	Blacklisted             bool    `json:"blacklisted"`
	EscrowAccount           *string `json:"escrow_account"`
	PriorityClass           *string `json:"priority_class"`
}

// Create handles POST / (Req 3.1-3.9).
func (h *AdminATMHandler) Create(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	var body createATMAdminRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}

	created, err := h.svc.Create(r.Context(), authCtx.UserID, service.CreateATMRequest{
		TerminalID: body.TerminalID, LocationID: body.LocationID, MachineType: body.MachineType,
		Brand: body.Brand, Model: body.Model, OperationHours: body.OperationHours,
		DeploymentType: body.DeploymentType, CapacityAmount: body.CapacityAmount,
		LowThresholdAmount: body.LowThresholdAmount, CriticalThresholdAmount: body.CriticalThresholdAmount,
		Blacklisted: body.Blacklisted, EscrowAccount: body.EscrowAccount, PriorityClass: body.PriorityClass,
	}, extractClientIP(r))
	if err != nil {
		h.handleATMAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, atmToResponse(created))
}

type updateATMAdminRequestBody struct {
	TerminalID              *string `json:"terminal_id"`
	LocationID              int64   `json:"location_id"`
	MachineType             string  `json:"machine_type"`
	Brand                   string  `json:"brand"`
	Model                   string  `json:"model"`
	OperationHours          string  `json:"operation_hours"`
	DeploymentType          string  `json:"deployment_type"`
	CapacityAmount          *string `json:"capacity_amount"`
	LowThresholdAmount      *string `json:"low_threshold_amount"`
	CriticalThresholdAmount *string `json:"critical_threshold_amount"`
	Blacklisted             bool    `json:"blacklisted"`
	EscrowAccount           *string `json:"escrow_account"`
	PriorityClass           *string `json:"priority_class"`
}

// Update handles PUT /{id} (Req 4.1-4.7). TerminalID is accepted only so an
// attempt to change it can be detected and rejected by the service --
// omitting it from the body entirely is treated the same as resending the
// current value.
func (h *AdminATMHandler) Update(w http.ResponseWriter, r *http.Request) {
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

	var body updateATMAdminRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}

	updated, err := h.svc.Update(r.Context(), authCtx.UserID, id, service.UpdateATMRequest{
		TerminalID: body.TerminalID, LocationID: body.LocationID, MachineType: body.MachineType,
		Brand: body.Brand, Model: body.Model, OperationHours: body.OperationHours,
		DeploymentType: body.DeploymentType, CapacityAmount: body.CapacityAmount,
		LowThresholdAmount: body.LowThresholdAmount, CriticalThresholdAmount: body.CriticalThresholdAmount,
		Blacklisted: body.Blacklisted, EscrowAccount: body.EscrowAccount, PriorityClass: body.PriorityClass,
	}, extractClientIP(r))
	if err != nil {
		h.handleATMAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, atmToResponse(updated))
}

// Disable handles POST /{id}/disable (Req 5.1, 5.4-5.5).
func (h *AdminATMHandler) Disable(w http.ResponseWriter, r *http.Request) {
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

	if err := h.svc.Disable(r.Context(), authCtx.UserID, id, extractClientIP(r)); err != nil {
		h.handleATMAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "ATM berhasil dinonaktifkan"})
}

// Enable handles POST /{id}/enable (Req 5.2, 5.6).
func (h *AdminATMHandler) Enable(w http.ResponseWriter, r *http.Request) {
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

	if err := h.svc.Enable(r.Context(), authCtx.UserID, id, extractClientIP(r)); err != nil {
		h.handleATMAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "ATM berhasil diaktifkan kembali"})
}

// handleATMAdminError maps ATMAdminService errors to HTTP responses per
// design.md's "Error -> HTTP mapping" table.
func (h *AdminATMHandler) handleATMAdminError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError

	switch {
	case errors.As(err, &validationErr):
		writeValidationError(w, validationErr.Field, validationErr.Message)
	case errors.Is(err, service.ErrATMNotFound):
		writeError(w, http.StatusNotFound, "not_found", service.ErrATMNotFound.Error())
	case errors.Is(err, service.ErrATMTerminalIDConflict):
		writeError(w, http.StatusConflict, "conflict", service.ErrATMTerminalIDConflict.Error())
	case errors.Is(err, service.ErrATMTerminalIDImmutable):
		writeError(w, http.StatusBadRequest, "bad_request", service.ErrATMTerminalIDImmutable.Error())
	case errors.Is(err, service.ErrATMInvalidReference):
		writeError(w, http.StatusBadRequest, "invalid_reference", service.ErrATMInvalidReference.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
	}
}

// --- response mapping ------------------------------------------------------

// atmToResponse builds the flat-JSON ATM DTO from service.ATM: amounts as
// *string decimal (already converted by the service), timestamps as RFC3339
// pointers via formatTimePtr (atm_portal_handler.go).
func atmToResponse(a service.ATM) map[string]any {
	return map[string]any{
		"id": a.ID, "terminal_id": a.TerminalID, "location_id": a.LocationID, "location_name": a.LocationName,
		"machine_type": a.MachineType, "brand": a.Brand, "model": a.Model, "operation_hours": a.OperationHours,
		"deployment_type": a.DeploymentType, "capacity_amount": a.CapacityAmount,
		"low_threshold_amount": a.LowThresholdAmount, "critical_threshold_amount": a.CriticalThresholdAmount,
		"blacklisted": a.Blacklisted, "escrow_account": a.EscrowAccount, "priority_class": a.PriorityClass,
		"is_active": a.IsActive, "created_at": formatTimePtr(a.CreatedAt), "updated_at": formatTimePtr(a.UpdatedAt),
		"deleted_at": formatTimePtr(a.DeletedAt),
	}
}
