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

// VendorPksAdminServicer is the subset of service.VendorPksAdminService the handler needs.
type VendorPksAdminServicer interface {
	Get(ctx context.Context, vendorID int64) (*service.VendorPksDetail, error)
	SubmitNew(ctx context.Context, makerID, vendorID int64, req service.SubmitVendorPksRequest, ip string) (db.MasterDataChangeRequest, error)
	SubmitTerminate(ctx context.Context, makerID, vendorID, pksID int64, reason, ip string) (db.MasterDataChangeRequest, error)
	SubmitUnterminate(ctx context.Context, makerID, vendorID, pksID int64, reason, ip string) (db.MasterDataChangeRequest, error)
}

// vendorPksReasonBody is the body of the terminate / unterminate endpoints.
type vendorPksReasonBody struct {
	Reason string `json:"reason"`
}

// AdminVendorPksHandler serves the PKS tab of a vendor (spec vendor-pks, Req 3-5).
// Mount at /api/v1/admin/vendors/{vendorID}/pks behind the masterDataAdmin group
// (RequireAuth + ADMIN/ADMIN_PARAM); Submit re-checks the role (Req 11.1).
type AdminVendorPksHandler struct {
	svc VendorPksAdminServicer
}

// NewAdminVendorPksHandler creates an AdminVendorPksHandler.
func NewAdminVendorPksHandler(svc VendorPksAdminServicer) *AdminVendorPksHandler {
	return &AdminVendorPksHandler{svc: svc}
}

// Routes returns the router for the PKS endpoints.
func (h *AdminVendorPksHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.Get)
	r.Post("/", h.Create)
	r.Post("/{pksID}/terminate", h.Terminate)
	r.Post("/{pksID}/unterminate", h.Unterminate)
	return r
}

// Get handles GET / -- active PKS, history and pending change.
func (h *AdminVendorPksHandler) Get(w http.ResponseWriter, r *http.Request) {
	vendorID, err := parsePathID(r, "vendorID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	detail, err := h.svc.Get(r.Context(), vendorID)
	if err != nil {
		handleVendorPksError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// Create handles POST / {pks_name, effective_date, expiry_date} -- 202 pending approval.
func (h *AdminVendorPksHandler) Create(w http.ResponseWriter, r *http.Request) {
	authCtx, vendorID, ok := h.authAndVendor(w, r)
	if !ok {
		return
	}
	var body service.SubmitVendorPksRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	change, err := h.svc.SubmitNew(r.Context(), authCtx.UserID, vendorID, body, extractClientIP(r))
	respondVendorPks(w, change, err)
}

// Terminate handles POST /{pksID}/terminate {reason} -- 202 pending approval.
func (h *AdminVendorPksHandler) Terminate(w http.ResponseWriter, r *http.Request) {
	h.reasonAction(w, r, h.svc.SubmitTerminate)
}

// Unterminate handles POST /{pksID}/unterminate {reason} -- 202 pending approval.
func (h *AdminVendorPksHandler) Unterminate(w http.ResponseWriter, r *http.Request) {
	h.reasonAction(w, r, h.svc.SubmitUnterminate)
}

func (h *AdminVendorPksHandler) reasonAction(w http.ResponseWriter, r *http.Request, fn func(context.Context, int64, int64, int64, string, string) (db.MasterDataChangeRequest, error)) {
	authCtx, vendorID, ok := h.authAndVendor(w, r)
	if !ok {
		return
	}
	pksID, err := parsePathID(r, "pksID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var body vendorPksReasonBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	change, err := fn(r.Context(), authCtx.UserID, vendorID, pksID, body.Reason, extractClientIP(r))
	respondVendorPks(w, change, err)
}

func (h *AdminVendorPksHandler) authAndVendor(w http.ResponseWriter, r *http.Request) (*middleware.AuthContext, int64, bool) {
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

func respondVendorPks(w http.ResponseWriter, change db.MasterDataChangeRequest, err error) {
	if err != nil {
		handleVendorPksError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
}

// handleVendorPksError maps service errors per design.md "Error Handling".
// Anything unlisted (incl. approval.ErrChainIncomplete -> 422) goes through
// writeUnexpectedError, which logs details and returns a generic 500.
func handleVendorPksError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeValidationError(w, validationErr.Field, validationErr.Message)
	case errors.Is(err, service.ErrVendorNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Vendor tidak ditemukan")
	case errors.Is(err, service.ErrVendorPksNotFound):
		writeError(w, http.StatusNotFound, "not_found", "PKS tidak ditemukan")
	case errors.Is(err, service.ErrMasterDataChangePending):
		writeError(w, http.StatusConflict, "conflict", "Masih ada perubahan PKS yang menunggu persetujuan")
	case errors.Is(err, service.ErrVendorPksVendorInactive):
		writeError(w, http.StatusConflict, "conflict", "Vendor nonaktif. Aktifkan vendor dulu")
	case errors.Is(err, service.ErrVendorPksNotActive):
		writeError(w, http.StatusConflict, "conflict", "PKS ini sudah digantikan")
	case errors.Is(err, service.ErrVendorPksAlreadyTerminated):
		writeError(w, http.StatusConflict, "conflict", "PKS sudah Terminated")
	case errors.Is(err, service.ErrVendorPksNotTerminated):
		writeError(w, http.StatusConflict, "conflict", "PKS tidak dalam status Terminated")
	case errors.Is(err, service.ErrMasterDataForbidden):
		writeForbidden(w, "Anda tidak berhak mengubah master data")
	default:
		writeUnexpectedError(w, err)
	}
}
