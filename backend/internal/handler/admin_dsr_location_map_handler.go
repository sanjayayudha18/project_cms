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

// DsrLocationMapAdminServicer is the subset of service.DsrLocationMapAdminService the handler needs.
type DsrLocationMapAdminServicer interface {
	List(ctx context.Context, vendorID int64, status string) ([]db.ListDsrLocationMapsAdminRow, error)
	ListUnmapped(ctx context.Context, vendorID int64) ([]db.ListUnmappedDsrLocationsRow, error)
	Create(ctx context.Context, makerID, vendorID int64, req service.DsrLocationMapPayload, actorIP string) (db.MasterDataChangeRequest, error)
	Update(ctx context.Context, makerID, vendorID, id int64, req service.DsrLocationMapUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error)
	Disable(ctx context.Context, makerID, vendorID, id int64, actorIP string) (db.MasterDataChangeRequest, error)
	Enable(ctx context.Context, makerID, vendorID, id int64, actorIP string) (db.MasterDataChangeRequest, error)
}

// AdminDsrLocationMapHandler serves the "Mapping DSR" tab (cit-acm-plan FR2).
// Mount at /api/v1/admin/vendors/{vendorID}/dsr-location-maps behind the
// masterDataAdmin group (RequireAuth + ADMIN/ADMIN_PARAM) -- see cmd/api/main.go.
type AdminDsrLocationMapHandler struct {
	svc DsrLocationMapAdminServicer
}

// NewAdminDsrLocationMapHandler creates an AdminDsrLocationMapHandler.
func NewAdminDsrLocationMapHandler(svc DsrLocationMapAdminServicer) *AdminDsrLocationMapHandler {
	return &AdminDsrLocationMapHandler{svc: svc}
}

// Routes returns the router for the mapping endpoints.
func (h *AdminDsrLocationMapHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/unmapped", h.Unmapped)
	r.Put("/{id}", h.Update)
	r.Post("/{id}/disable", h.Disable)
	r.Post("/{id}/enable", h.Enable)
	return r
}

// List handles GET /?status=active|disabled|all.
func (h *AdminDsrLocationMapHandler) List(w http.ResponseWriter, r *http.Request) {
	vendorID, err := parsePathID(r, "vendorID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	status, err := parseStatusParam(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	rows, err := h.svc.List(r.Context(), vendorID, status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	items := make([]map[string]any, len(rows))
	for i, m := range rows {
		items[i] = map[string]any{
			"id": m.ID, "vendor_id": m.VendorID, "dsr_location": m.DsrLocation, "vendor_vault_id": m.VendorVaultID,
			"vault_code": m.VaultCode, "vendor_branch_id": m.VendorBranchID, "branch_name": m.BranchName,
			"is_active": m.IsActive, "deleted_at": formatTimestamptz(m.DeletedAt),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"maps": items})
}

// Unmapped handles GET /unmapped -- DSR block labels without an active mapping.
func (h *AdminDsrLocationMapHandler) Unmapped(w http.ResponseWriter, r *http.Request) {
	vendorID, err := parsePathID(r, "vendorID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	rows, err := h.svc.ListUnmapped(r.Context(), vendorID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	items := make([]map[string]any, len(rows))
	for i, u := range rows {
		var last any
		if u.LastReportDate.Valid {
			last = u.LastReportDate.Time.Format("2006-01-02")
		}
		items[i] = map[string]any{"dsr_location": u.DsrLocation, "last_report_date": last}
	}
	writeJSON(w, http.StatusOK, map[string]any{"locations": items})
}

// Create handles POST / {dsr_location, vendor_vault_id} -- 202 pending approval.
func (h *AdminDsrLocationMapHandler) Create(w http.ResponseWriter, r *http.Request) {
	authCtx, vendorID, ok := h.authAndVendor(w, r)
	if !ok {
		return
	}
	var body service.DsrLocationMapPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	change, err := h.svc.Create(r.Context(), authCtx.UserID, vendorID, body, extractClientIP(r))
	h.respond(w, change, err)
}

// Update handles PUT /{id} {vendor_vault_id} -- 202 pending approval.
func (h *AdminDsrLocationMapHandler) Update(w http.ResponseWriter, r *http.Request) {
	authCtx, vendorID, ok := h.authAndVendor(w, r)
	if !ok {
		return
	}
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var body service.DsrLocationMapUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	change, err := h.svc.Update(r.Context(), authCtx.UserID, vendorID, id, body, extractClientIP(r))
	h.respond(w, change, err)
}

// Disable handles POST /{id}/disable -- 202 pending approval.
func (h *AdminDsrLocationMapHandler) Disable(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.svc.Disable)
}

// Enable handles POST /{id}/enable -- 202 pending approval.
func (h *AdminDsrLocationMapHandler) Enable(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.svc.Enable)
}

func (h *AdminDsrLocationMapHandler) toggle(w http.ResponseWriter, r *http.Request, fn func(context.Context, int64, int64, int64, string) (db.MasterDataChangeRequest, error)) {
	authCtx, vendorID, ok := h.authAndVendor(w, r)
	if !ok {
		return
	}
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	change, err := fn(r.Context(), authCtx.UserID, vendorID, id, extractClientIP(r))
	h.respond(w, change, err)
}

func (h *AdminDsrLocationMapHandler) authAndVendor(w http.ResponseWriter, r *http.Request) (*middleware.AuthContext, int64, bool) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return nil, 0, false
	}
	vendorID, err := parsePathID(r, "vendorID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return nil, 0, false
	}
	return authCtx, vendorID, true
}

func (h *AdminDsrLocationMapHandler) respond(w http.ResponseWriter, change db.MasterDataChangeRequest, err error) {
	var validationErr *service.ValidationError
	switch {
	case err == nil:
		writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
	case errors.As(err, &validationErr):
		writeValidationError(w, validationErr.Field, validationErr.Message)
	case errors.Is(err, service.ErrDsrLocationMapNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, service.ErrDsrLocationMapConflict), errors.Is(err, service.ErrMasterDataChangePending):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, service.ErrMasterDataForbidden):
		writeForbidden(w, "Anda tidak berhak mengubah master data")
	default:
		writeUnexpectedError(w, err)
	}
}
