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

// VendorVaultAdminServicer is the subset of service.VendorVaultAdminService
// the handler needs. Mutations return a pending db.MasterDataChangeRequest.
type VendorVaultAdminServicer interface {
	List(ctx context.Context, arg db.ListVendorVaultsAdminParams) ([]service.VendorVault, error)
	Count(ctx context.Context, arg db.CountVendorVaultsAdminParams) (int64, error)
	Get(ctx context.Context, id int64) (*service.VendorVault, error)
	Create(ctx context.Context, makerID, vendorID int64, req service.VendorVaultPayload, actorIP string) (db.MasterDataChangeRequest, error)
	Update(ctx context.Context, makerID, id int64, req service.VendorVaultUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error)
	Disable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error)
	Enable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error)
}

// AdminVendorVaultHandler handles ADMIN/ADMIN_PARAM-only vendor vault
// management (T3.2). Mount at /api/v1/admin/vendors/{vendorID}/vaults behind
// RequireAuth + RequireRoles("ADMIN", "ADMIN_PARAM") -- see cmd/api/main.go.
type AdminVendorVaultHandler struct {
	svc VendorVaultAdminServicer
}

// NewAdminVendorVaultHandler creates a new AdminVendorVaultHandler with the given dependency.
func NewAdminVendorVaultHandler(svc VendorVaultAdminServicer) *AdminVendorVaultHandler {
	return &AdminVendorVaultHandler{svc: svc}
}

// Routes returns a chi.Router with all admin vendor vault endpoints mounted.
func (h *AdminVendorVaultHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Post("/{id}/disable", h.Disable)
	r.Post("/{id}/enable", h.Enable)
	return r
}

func vaultToResponse(v service.VendorVault) map[string]any {
	return map[string]any{
		"id": v.ID, "vendor_branch_id": v.VendorBranchID, "vault_code": v.VaultCode, "category": v.Category,
		"currency_code": v.CurrencyCode, "min_capacity_amount": v.MinCapacityAmount, "max_capacity_amount": v.MaxCapacityAmount,
		"latitude": v.Latitude, "longitude": v.Longitude, "operating_hours": v.OperatingHours, "location_id": v.LocationID,
		"is_active": v.IsActive, "deleted_at": formatTimePtr(v.DeletedAt),
	}
}

// List handles GET / -- filterable, paginated vault list for one vendor.
func (h *AdminVendorVaultHandler) List(w http.ResponseWriter, r *http.Request) {
	vendorID, err := parsePathID(r, "vendorID")
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
	var qParam *string
	if v := q.Get("q"); v != "" {
		qParam = &v
	}
	branchID, err := parseOptionalIDParam(q, "branch_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	vaults, err := h.svc.List(r.Context(), db.ListVendorVaultsAdminParams{
		VendorID: vendorID, VendorBranchID: branchID, Q: qParam, Status: status,
		PageLimit: int64(pageSize), PageOffset: int64((page - 1) * pageSize),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	total, err := h.svc.Count(r.Context(), db.CountVendorVaultsAdminParams{VendorID: vendorID, VendorBranchID: branchID, Q: qParam, Status: status})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}

	items := make([]map[string]any, len(vaults))
	for i, v := range vaults {
		items[i] = vaultToResponse(v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"vaults": items, "page": page, "page_size": pageSize, "total": total})
}

// Get handles GET /{id}.
func (h *AdminVendorVaultHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	v, err := h.svc.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	if v == nil {
		writeError(w, http.StatusNotFound, "not_found", service.ErrVendorVaultNotFound.Error())
		return
	}
	writeJSON(w, http.StatusOK, vaultToResponse(*v))
}

type createVendorVaultRequestBody struct {
	VendorBranchID int64  `json:"vendor_branch_id"`
	VaultCode      string `json:"vault_code"`
	service.VendorVaultUpdatePayload
}

// Create handles POST / -- stages a new vault for approval (202).
func (h *AdminVendorVaultHandler) Create(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	vendorID, err := parsePathID(r, "vendorID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var body createVendorVaultRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}

	change, err := h.svc.Create(r.Context(), authCtx.UserID, vendorID, service.VendorVaultPayload{
		VendorBranchID: body.VendorBranchID, VaultCode: body.VaultCode, VendorVaultUpdatePayload: body.VendorVaultUpdatePayload,
	}, extractClientIP(r))
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
}

// Update handles PUT /{id} -- stages an update for approval (202).
func (h *AdminVendorVaultHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	var body service.VendorVaultUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}

	change, err := h.svc.Update(r.Context(), authCtx.UserID, id, body, extractClientIP(r))
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
}

// Disable handles POST /{id}/disable -- stages a disable for approval (202).
func (h *AdminVendorVaultHandler) Disable(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.svc.Disable)
}

// Enable handles POST /{id}/enable -- stages a re-enable for approval (202).
func (h *AdminVendorVaultHandler) Enable(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.svc.Enable)
}

func (h *AdminVendorVaultHandler) toggle(w http.ResponseWriter, r *http.Request, fn func(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error)) {
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
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
}

func (h *AdminVendorVaultHandler) handleError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeValidationError(w, validationErr.Field, validationErr.Message)
	case errors.Is(err, service.ErrVendorVaultNotFound):
		writeError(w, http.StatusNotFound, "not_found", service.ErrVendorVaultNotFound.Error())
	case errors.Is(err, service.ErrVendorVaultCodeConflict):
		writeError(w, http.StatusConflict, "conflict", service.ErrVendorVaultCodeConflict.Error())
	case errors.Is(err, service.ErrMasterDataForbidden):
		writeForbidden(w, "Anda tidak berhak mengubah master data")
	case errors.Is(err, service.ErrMasterDataChangePending):
		writeError(w, http.StatusConflict, "conflict", service.ErrMasterDataChangePending.Error())
	default:
		writeUnexpectedError(w, err)
	}
}
