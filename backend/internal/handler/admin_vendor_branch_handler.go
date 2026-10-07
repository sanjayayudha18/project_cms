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

// VendorBranchAdminServicer is the subset of service.VendorBranchAdminService
// the handler needs: List/Get (read-only) plus Create/Update/Disable/Enable
// -- the latter four return a pending db.MasterDataChangeRequest, not the
// entity itself (T3.1 is maker-checker-native, unlike VendorAdminServicer).
type VendorBranchAdminServicer interface {
	List(ctx context.Context, arg db.ListVendorBranchesAdminParams) ([]db.ListVendorBranchesAdminRow, error)
	Count(ctx context.Context, arg db.CountVendorBranchesAdminParams) (int64, error)
	Get(ctx context.Context, id int64) (*db.GetVendorBranchAdminByIDRow, error)
	Create(ctx context.Context, makerID int64, req service.VendorBranchPayload, actorIP string) (db.MasterDataChangeRequest, error)
	Update(ctx context.Context, makerID, id int64, req service.VendorBranchUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error)
	Disable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error)
	Enable(ctx context.Context, makerID, id int64, actorIP string) (db.MasterDataChangeRequest, error)
}

// AdminVendorBranchHandler handles ADMIN/ADMIN_PARAM-only vendor branch
// management: list/get (read-only) plus create/update/disable/enable
// (staged via the maker-checker chain, T3.1). Mount at
// /api/v1/admin/vendors/{vendorID}/branches behind RequireAuth +
// RequireRoles("ADMIN", "ADMIN_PARAM") -- see cmd/api/main.go.
type AdminVendorBranchHandler struct {
	svc VendorBranchAdminServicer
}

// NewAdminVendorBranchHandler creates a new AdminVendorBranchHandler with the given dependency.
func NewAdminVendorBranchHandler(svc VendorBranchAdminServicer) *AdminVendorBranchHandler {
	return &AdminVendorBranchHandler{svc: svc}
}

// Routes returns a chi.Router with all admin vendor branch endpoints mounted.
func (h *AdminVendorBranchHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Post("/{id}/disable", h.Disable)
	r.Post("/{id}/enable", h.Enable)
	return r
}

// List handles GET / -- filterable, paginated branch list for one vendor.
func (h *AdminVendorBranchHandler) List(w http.ResponseWriter, r *http.Request) {
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

	branches, err := h.svc.List(r.Context(), db.ListVendorBranchesAdminParams{
		VendorID: vendorID, Q: qParam, Status: status,
		PageLimit: int64(pageSize), PageOffset: int64((page - 1) * pageSize),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	total, err := h.svc.Count(r.Context(), db.CountVendorBranchesAdminParams{VendorID: vendorID, Q: qParam, Status: status})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}

	items := make([]map[string]any, len(branches))
	for i, b := range branches {
		items[i] = map[string]any{
			"id": b.ID, "vendor_id": b.VendorID, "branch_code": b.BranchCode, "branch_name": b.BranchName,
			"location_id": b.LocationID, "region": b.Region, "category": b.Category, "is_active": b.IsActive,
			"deleted_at": formatTimestamptz(b.DeletedAt),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"branches": items, "page": page, "page_size": pageSize, "total": total,
	})
}

// Get handles GET /{id}.
func (h *AdminVendorBranchHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	branch, err := h.svc.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	if branch == nil {
		writeError(w, http.StatusNotFound, "not_found", service.ErrVendorBranchNotFound.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": branch.ID, "vendor_id": branch.VendorID, "branch_code": branch.BranchCode, "branch_name": branch.BranchName,
		"location_id": branch.LocationID, "region": branch.Region, "category": branch.Category, "is_active": branch.IsActive,
		"deleted_at": formatTimestamptz(branch.DeletedAt),
	})
}

type createVendorBranchRequestBody struct {
	BranchCode string  `json:"branch_code"`
	BranchName string  `json:"branch_name"`
	LocationID *int64  `json:"location_id"`
	Region     *string `json:"region"`
	Category   string  `json:"category"`
}

// Create handles POST / -- stages a new branch for approval (202).
func (h *AdminVendorBranchHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var body createVendorBranchRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}

	change, err := h.svc.Create(r.Context(), authCtx.UserID, service.VendorBranchPayload{
		VendorID: vendorID, BranchCode: body.BranchCode, BranchName: body.BranchName,
		LocationID: body.LocationID, Region: body.Region, Category: body.Category,
	}, extractClientIP(r))
	if err != nil {
		h.handleVendorBranchAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
}

type updateVendorBranchRequestBody struct {
	BranchName string  `json:"branch_name"`
	LocationID *int64  `json:"location_id"`
	Region     *string `json:"region"`
	Category   string  `json:"category"`
}

// Update handles PUT /{id} -- stages a branch update for approval (202).
func (h *AdminVendorBranchHandler) Update(w http.ResponseWriter, r *http.Request) {
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

	var body updateVendorBranchRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}

	change, err := h.svc.Update(r.Context(), authCtx.UserID, id, service.VendorBranchUpdatePayload{
		BranchName: body.BranchName, LocationID: body.LocationID, Region: body.Region, Category: body.Category,
	}, extractClientIP(r))
	if err != nil {
		h.handleVendorBranchAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
}

// Disable handles POST /{id}/disable -- stages a disable for approval (202).
func (h *AdminVendorBranchHandler) Disable(w http.ResponseWriter, r *http.Request) {
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

	change, err := h.svc.Disable(r.Context(), authCtx.UserID, id, extractClientIP(r))
	if err != nil {
		h.handleVendorBranchAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
}

// Enable handles POST /{id}/enable -- stages a re-enable for approval (202).
func (h *AdminVendorBranchHandler) Enable(w http.ResponseWriter, r *http.Request) {
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

	change, err := h.svc.Enable(r.Context(), authCtx.UserID, id, extractClientIP(r))
	if err != nil {
		h.handleVendorBranchAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
}

// handleVendorBranchAdminError maps VendorBranchAdminService errors to HTTP responses.
func (h *AdminVendorBranchHandler) handleVendorBranchAdminError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError

	switch {
	case errors.As(err, &validationErr):
		writeValidationError(w, validationErr.Field, validationErr.Message)
	case errors.Is(err, service.ErrVendorBranchNotFound):
		writeError(w, http.StatusNotFound, "not_found", service.ErrVendorBranchNotFound.Error())
	case errors.Is(err, service.ErrVendorBranchCodeConflict):
		writeError(w, http.StatusConflict, "conflict", service.ErrVendorBranchCodeConflict.Error())
	case errors.Is(err, service.ErrVendorBranchHasActiveChildren):
		writeError(w, http.StatusConflict, "conflict", service.ErrVendorBranchHasActiveChildren.Error())
	case errors.Is(err, service.ErrMasterDataForbidden):
		writeForbidden(w, "Anda tidak berhak mengubah master data")
	case errors.Is(err, service.ErrMasterDataChangePending):
		writeError(w, http.StatusConflict, "conflict", service.ErrMasterDataChangePending.Error())
	default:
		writeUnexpectedError(w, err)
	}
}

// changeRequestAcceptedResponse builds the flat-JSON 202 body every
// maker-checker-native mutation endpoint returns (T3.1 onward): the entity
// itself does not change yet, only a pending change request exists.
func changeRequestAcceptedResponse(c db.MasterDataChangeRequest) map[string]any {
	return map[string]any{
		"change_request_id": c.ID, "status": c.Status, "entity_type": c.EntityType, "op": c.Op,
	}
}
