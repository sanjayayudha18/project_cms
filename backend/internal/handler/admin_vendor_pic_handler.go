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

// VendorPicAdminServicer is the subset of service.VendorPicAdminService the
// handler needs. Mutations return a pending db.MasterDataChangeRequest.
type VendorPicAdminServicer interface {
	List(ctx context.Context, arg db.ListVendorPicsAdminParams) ([]service.VendorPic, error)
	Count(ctx context.Context, arg db.CountVendorPicsAdminParams) (int64, error)
	Get(ctx context.Context, vendorID, id int64) (*service.VendorPic, error)
	Warnings(ctx context.Context, vendorID int64) ([]string, error)
	Create(ctx context.Context, makerID, vendorID int64, req service.VendorPicUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error)
	Update(ctx context.Context, makerID, vendorID, id int64, req service.VendorPicUpdatePayload, actorIP string) (db.MasterDataChangeRequest, error)
	Disable(ctx context.Context, makerID, vendorID, id int64, actorIP string) (db.MasterDataChangeRequest, error)
	Enable(ctx context.Context, makerID, vendorID, id int64, actorIP string) (db.MasterDataChangeRequest, error)
}

// AdminVendorPicHandler handles ADMIN/ADMIN_PARAM-only vendor PIC management
// (T3.3). Mount at /api/v1/admin/vendors/{vendorID}/pics behind RequireAuth +
// RequireRoles("ADMIN", "ADMIN_PARAM") -- see cmd/api/main.go.
type AdminVendorPicHandler struct {
	svc VendorPicAdminServicer
}

// NewAdminVendorPicHandler creates a new AdminVendorPicHandler with the given dependency.
func NewAdminVendorPicHandler(svc VendorPicAdminServicer) *AdminVendorPicHandler {
	return &AdminVendorPicHandler{svc: svc}
}

// Routes returns a chi.Router with all admin vendor PIC endpoints mounted.
func (h *AdminVendorPicHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Post("/{id}/disable", h.Disable)
	r.Post("/{id}/enable", h.Enable)
	return r
}

func picToResponse(p service.VendorPic) map[string]any {
	return map[string]any{
		"id": p.ID, "vendor_id": p.VendorID, "vendor_branch_id": p.VendorBranchID, "name": p.Name,
		"position": p.Position, "phone": p.Phone, "email": p.Email,
		"is_notification_recipient": p.IsNotificationRecipient, "is_active": p.IsActive,
		"deleted_at": formatTimePtr(p.DeletedAt),
	}
}

// List handles GET / -- filterable, paginated PIC list for one vendor, plus
// non-blocking "warnings" (e.g. no active notification recipient).
func (h *AdminVendorPicHandler) List(w http.ResponseWriter, r *http.Request) {
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

	pics, err := h.svc.List(r.Context(), db.ListVendorPicsAdminParams{
		VendorID: vendorID, Q: qParam, Status: status,
		PageLimit: int64(pageSize), PageOffset: int64((page - 1) * pageSize),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	total, err := h.svc.Count(r.Context(), db.CountVendorPicsAdminParams{VendorID: vendorID, Q: qParam, Status: status})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	warnings, err := h.svc.Warnings(r.Context(), vendorID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}

	items := make([]map[string]any, len(pics))
	for i, p := range pics {
		items[i] = picToResponse(p)
	}
	writeJSON(w, http.StatusOK, map[string]any{"pics": items, "page": page, "page_size": pageSize, "total": total, "warnings": warnings})
}

// Get handles GET /{id}.
func (h *AdminVendorPicHandler) Get(w http.ResponseWriter, r *http.Request) {
	vendorID, err := parsePathID(r, "vendorID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	p, err := h.svc.Get(r.Context(), vendorID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	if p == nil {
		writeError(w, http.StatusNotFound, "not_found", service.ErrVendorPicNotFound.Error())
		return
	}
	writeJSON(w, http.StatusOK, picToResponse(*p))
}

// Create handles POST / -- stages a new PIC for approval (202).
func (h *AdminVendorPicHandler) Create(w http.ResponseWriter, r *http.Request) {
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
	var body service.VendorPicUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}

	change, err := h.svc.Create(r.Context(), authCtx.UserID, vendorID, body, extractClientIP(r))
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
}

// Update handles PUT /{id} -- stages an update for approval (202).
func (h *AdminVendorPicHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var body service.VendorPicUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}

	change, err := h.svc.Update(r.Context(), authCtx.UserID, vendorID, id, body, extractClientIP(r))
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
}

// Disable handles POST /{id}/disable -- stages a disable for approval (202).
func (h *AdminVendorPicHandler) Disable(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.svc.Disable)
}

// Enable handles POST /{id}/enable -- stages a re-enable for approval (202).
func (h *AdminVendorPicHandler) Enable(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.svc.Enable)
}

func (h *AdminVendorPicHandler) toggle(w http.ResponseWriter, r *http.Request, fn func(context.Context, int64, int64, int64, string) (db.MasterDataChangeRequest, error)) {
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
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	change, err := fn(r.Context(), authCtx.UserID, vendorID, id, extractClientIP(r))
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
}

func (h *AdminVendorPicHandler) handleError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeValidationError(w, validationErr.Field, validationErr.Message)
	case errors.Is(err, service.ErrVendorPicNotFound):
		writeError(w, http.StatusNotFound, "not_found", service.ErrVendorPicNotFound.Error())
	case errors.Is(err, service.ErrMasterDataForbidden):
		writeForbidden(w, "Anda tidak berhak mengubah master data")
	case errors.Is(err, service.ErrMasterDataChangePending):
		writeError(w, http.StatusConflict, "conflict", service.ErrMasterDataChangePending.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
	}
}
