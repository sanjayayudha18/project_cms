package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// VendorAdminServicer is the subset of service.VendorAdminService the
// handler needs: List/Get (read-only, Req 6/9.5) plus
// Create/Update/Disable/Enable (Req 7/8).
type VendorAdminServicer interface {
	List(ctx context.Context, arg db.ListVendorsAdminParams) ([]db.ListVendorsAdminRow, error)
	Count(ctx context.Context, arg db.CountVendorsAdminParams) (int64, error)
	Get(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error)
	// Mutations stage a maker-checker change request (T4.1) and return it
	// pending; the vendor row itself changes only after approval.
	Create(ctx context.Context, actorID int64, req service.CreateVendorRequest, actorIP string) (db.MasterDataChangeRequest, error)
	Update(ctx context.Context, actorID, id int64, req service.UpdateVendorRequest, actorIP string) (db.MasterDataChangeRequest, error)
	Disable(ctx context.Context, actorID, id int64, actorIP string) (service.DisableVendorResult, error)
	Enable(ctx context.Context, actorID, id int64, actorIP string) (db.MasterDataChangeRequest, error)
}

// AdminVendorHandler handles ADMIN/ADMIN_PARAM-only vendor management
// endpoints: list/get/create/update/disable/enable. Mirrors AdminUserHandler
// structurally, but disable/enable go straight through VendorAdminService
// (which already owns the existence pre-check + audit) rather than a
// separate deactivate service. Mount behind RequireAuth +
// RequireRoles("ADMIN", "ADMIN_PARAM") — see cmd/api/main.go.
type AdminVendorHandler struct {
	svc VendorAdminServicer
}

// NewAdminVendorHandler creates a new AdminVendorHandler with the given dependency.
func NewAdminVendorHandler(svc VendorAdminServicer) *AdminVendorHandler {
	return &AdminVendorHandler{svc: svc}
}

// Routes returns a chi.Router with all admin vendor endpoints mounted.
func (h *AdminVendorHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Post("/{id}/disable", h.Disable)
	r.Post("/{id}/enable", h.Enable)
	return r
}

// List handles GET / — filterable, paginated vendor list (Req 6).
func (h *AdminVendorHandler) List(w http.ResponseWriter, r *http.Request) {
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

	listArg := db.ListVendorsAdminParams{
		Q: qParam, Status: status,
		PageLimit: int64(pageSize), PageOffset: int64((page - 1) * pageSize),
	}
	rows, err := h.svc.List(r.Context(), listArg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	total, err := h.svc.Count(r.Context(), db.CountVendorsAdminParams{Q: qParam, Status: status})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}

	items := make([]map[string]any, len(rows))
	for i, row := range rows {
		items[i] = listVendorRowToResponse(row)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"vendors": items, "page": page, "page_size": pageSize, "total": total,
	})
}

// Get handles GET /{id}.
func (h *AdminVendorHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	row, err := h.svc.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	if row == nil {
		writeError(w, http.StatusNotFound, "not_found", "Vendor tidak ditemukan")
		return
	}
	writeJSON(w, http.StatusOK, getVendorRowToResponse(*row))
}

type createVendorAdminRequestBody struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	LegalName    string `json:"legal_name"`
	NPWP         string `json:"npwp"`
	ContactEmail string `json:"contact_email"`
	ContactPhone string `json:"contact_phone"`
	HqAddress    string `json:"hq_address"`
}

// Create handles POST / (Req 7.1-7.4) -- stages the vendor for approval (202).
func (h *AdminVendorHandler) Create(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	var body createVendorAdminRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}

	change, err := h.svc.Create(r.Context(), authCtx.UserID, service.CreateVendorRequest{
		Code: body.Code, Name: body.Name, LegalName: body.LegalName, NPWP: body.NPWP, ContactEmail: body.ContactEmail,
		ContactPhone: body.ContactPhone, HqAddress: body.HqAddress,
	}, extractClientIP(r))
	if err != nil {
		h.handleVendorAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
}

// updateVendorAdminRequestBody: legal_name/npwp are pointers on purpose --
// absent (null/omitted) keeps the current value, "" clears it (T4.3), so a
// client that predates the fields cannot wipe them with a PUT.
type updateVendorAdminRequestBody struct {
	Code         *string `json:"code"`
	Name         string  `json:"name"`
	LegalName    *string `json:"legal_name"`
	NPWP         *string `json:"npwp"`
	ContactEmail string  `json:"contact_email"`
	ContactPhone string  `json:"contact_phone"`
	HqAddress    string  `json:"hq_address"`
}

// Update handles PUT /{id} (Req 7.5-7.8) -- stages the edit for approval (202).
func (h *AdminVendorHandler) Update(w http.ResponseWriter, r *http.Request) {
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

	var body updateVendorAdminRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}

	change, err := h.svc.Update(r.Context(), authCtx.UserID, id, service.UpdateVendorRequest{
		Code: body.Code, Name: body.Name, LegalName: body.LegalName, NPWP: body.NPWP, ContactEmail: body.ContactEmail,
		ContactPhone: body.ContactPhone, HqAddress: body.HqAddress,
	}, extractClientIP(r))
	if err != nil {
		h.handleVendorAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
}

// Disable handles POST /{id}/disable (Req 8.1, 8.4-8.6) -- stages the disable
// for approval (202). VendorAdminService owns the existence pre-check; the
// handler surfaces the Req 8.5 linked-active-users warning (as of submit
// time) alongside the change-request fields.
func (h *AdminVendorHandler) Disable(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.svc.Disable(r.Context(), authCtx.UserID, id, extractClientIP(r))
	if err != nil {
		h.handleVendorAdminError(w, err)
		return
	}

	resp := changeRequestAcceptedResponse(result.Change)
	if result.LinkedUsersWarning > 0 {
		resp["warning"] = fmt.Sprintf("Vendor masih memiliki %d pengguna aktif yang terhubung", result.LinkedUsersWarning)
		resp["linked_active_users"] = result.LinkedUsersWarning
	}
	writeJSON(w, http.StatusAccepted, resp)
}

// Enable handles POST /{id}/enable (Req 8.2, 8.4, 8.6) -- stages the re-enable for approval (202).
func (h *AdminVendorHandler) Enable(w http.ResponseWriter, r *http.Request) {
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
		h.handleVendorAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
}

// handleVendorAdminError maps VendorAdminService errors to HTTP responses
// per design.md's "Error -> HTTP mapping" table.
func (h *AdminVendorHandler) handleVendorAdminError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError

	switch {
	case errors.As(err, &validationErr):
		writeValidationError(w, validationErr.Field, validationErr.Message)
	case errors.Is(err, service.ErrVendorNotFound):
		writeError(w, http.StatusNotFound, "not_found", service.ErrVendorNotFound.Error())
	case errors.Is(err, service.ErrVendorCodeConflict):
		writeError(w, http.StatusConflict, "conflict", service.ErrVendorCodeConflict.Error())
	case errors.Is(err, service.ErrVendorCodeImmutable):
		writeError(w, http.StatusBadRequest, "bad_request", service.ErrVendorCodeImmutable.Error())
	case errors.Is(err, service.ErrMasterDataForbidden):
		writeForbidden(w, "Anda tidak berhak mengubah master data")
	case errors.Is(err, service.ErrMasterDataChangePending):
		writeError(w, http.StatusConflict, "conflict", service.ErrMasterDataChangePending.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
	}
}

// --- response mapping ------------------------------------------------------

func listVendorRowToResponse(r db.ListVendorsAdminRow) map[string]any {
	return map[string]any{
		"id": r.ID, "code": r.Code, "name": r.Name, "legal_name": r.LegalName, "npwp": r.Npwp, "contact_email": r.ContactEmail,
		"contact_phone": r.ContactPhone, "hq_address": r.HqAddress, "is_active": r.IsActive,
		"deleted_at": formatTimestamptz(r.DeletedAt),
	}
}

func getVendorRowToResponse(r db.GetVendorAdminByIDRow) map[string]any {
	return map[string]any{
		"id": r.ID, "code": r.Code, "name": r.Name, "legal_name": r.LegalName, "npwp": r.Npwp, "contact_email": r.ContactEmail,
		"contact_phone": r.ContactPhone, "hq_address": r.HqAddress, "is_active": r.IsActive,
		"deleted_at": formatTimestamptz(r.DeletedAt),
	}
}
