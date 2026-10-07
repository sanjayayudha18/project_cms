package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// AtmVisitQuotaServicer is the service surface AtmVisitQuotaHandler needs
// (atm-visit-quota spec FR5), defined where it's used.
type AtmVisitQuotaServicer interface {
	Get(ctx context.Context, terminalID string) (*service.AtmVisitQuotaView, error)
	ResetATM(ctx context.Context, actor service.Actor, terminalID string) (*service.AtmVisitQuotaView, error)
	ResetVendor(ctx context.Context, actor service.Actor, vendorID int64) (*service.VendorQuotaResetResult, error)
	CancelVisit(ctx context.Context, actor service.Actor, visitID int64, reason string) error
}

// AtmVisitQuotaHandler serves /api/v1/atm-visit-quotas. Same role sets as
// Vendor Request: viewers read, checkers (SPV/ADMIN) reset and cancel — the
// service re-checks the checker role (Sec 5: RBAC at route and service).
type AtmVisitQuotaHandler struct {
	service AtmVisitQuotaServicer
}

func NewAtmVisitQuotaHandler(svc AtmVisitQuotaServicer) *AtmVisitQuotaHandler {
	return &AtmVisitQuotaHandler{service: svc}
}

func (h *AtmVisitQuotaHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.With(middleware.RequireRoles(vendorRequestViewerRoles...)).Get("/atms/{terminalId}", h.Get)
	r.With(middleware.RequireRoles(vendorRequestCheckerRoles...)).Post("/atms/{terminalId}/reset", h.ResetATM)
	r.With(middleware.RequireRoles(vendorRequestCheckerRoles...)).Post("/vendors/{vendorId}/reset", h.ResetVendor)
	r.With(middleware.RequireRoles(vendorRequestCheckerRoles...)).Post("/visits/{visitId}/cancel", h.CancelVisit)
	return r
}

type atmVisitResponse struct {
	ID              int64   `json:"id"`
	VendorRequestID int64   `json:"vendor_request_id"`
	RequestNumber   string  `json:"request_number"`
	QuotaKnown      bool    `json:"quota_known"`
	IsOverQuota     bool    `json:"is_over_quota"`
	CreatedAt       string  `json:"created_at"`
	CancelledAt     *string `json:"cancelled_at"`
	CancelledByName *string `json:"cancelled_by_name"`
	CancelReason    *string `json:"cancel_reason"`
}

type atmVisitQuotaResponse struct {
	TerminalID         string                `json:"terminal_id"`
	HasQuota           bool                  `json:"has_quota"`
	PackageCode        string                `json:"package_code"`
	QuotaTotal         int32                 `json:"quota_total"`
	Remaining          int32                 `json:"remaining"`
	Sisa               int32                 `json:"sisa"`
	Kelebihan          int32                 `json:"kelebihan"`
	ResetAt            *string               `json:"reset_at"`
	ResetBy            *vendorRequestUserRef `json:"reset_by"`
	CurrentPackageCode *string               `json:"current_package_code"`
	CurrentQuota       *int32                `json:"current_quota"`
	Visits             []atmVisitResponse    `json:"visits"`
}

func toAtmVisitQuotaResponse(v *service.AtmVisitQuotaView) atmVisitQuotaResponse {
	visits := make([]atmVisitResponse, len(v.Visits))
	for i, x := range v.Visits {
		visits[i] = atmVisitResponse{
			ID: x.ID, VendorRequestID: x.VendorRequestID, RequestNumber: x.RequestNumber,
			QuotaKnown: x.QuotaKnown, IsOverQuota: x.IsOverQuota, CreatedAt: formatTimestamp(x.CreatedAt),
			CancelledAt: formatTimestampPtr(x.CancelledAt), CancelledByName: x.CancelledByName, CancelReason: x.CancelReason,
		}
	}
	return atmVisitQuotaResponse{
		TerminalID: v.TerminalID, HasQuota: v.HasQuota, PackageCode: v.PackageCode,
		QuotaTotal: v.QuotaTotal, Remaining: v.Remaining, Sisa: v.Sisa, Kelebihan: v.Kelebihan,
		ResetAt: formatTimestampPtr(v.ResetAt), ResetBy: toUserRefResponse(v.ResetBy),
		CurrentPackageCode: v.CurrentPackageCode, CurrentQuota: v.CurrentQuota, Visits: visits,
	}
}

func (h *AtmVisitQuotaHandler) Get(w http.ResponseWriter, r *http.Request) {
	v, err := h.service.Get(r.Context(), chi.URLParam(r, "terminalId"))
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAtmVisitQuotaResponse(v))
}

func (h *AtmVisitQuotaHandler) ResetATM(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	v, err := h.service.ResetATM(r.Context(), actor, chi.URLParam(r, "terminalId"))
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAtmVisitQuotaResponse(v))
}

func (h *AtmVisitQuotaHandler) ResetVendor(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	vendorID, err := parsePositiveID(r, "vendorId")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	res, err := h.service.ResetVendor(r.Context(), actor, vendorID)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"reset_count": res.ResetCount, "skipped_count": res.SkippedCount})
}

func (h *AtmVisitQuotaHandler) CancelVisit(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	visitID, err := parsePositiveID(r, "visitId")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var body reasonBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "body tidak valid")
		return
	}
	if err := h.service.CancelVisit(r.Context(), actor, visitID, body.Reason); err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": visitID, "cancelled": true})
}

func parsePositiveID(r *http.Request, param string) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, param), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%s tidak valid", param)
	}
	return id, nil
}

func (h *AtmVisitQuotaHandler) handleError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError
	switch {
	case errors.Is(err, service.ErrAtmNotFound):
		writeError(w, http.StatusNotFound, "not_found", "ATM tidak ditemukan")
	case errors.Is(err, service.ErrVisitNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Kunjungan tidak ditemukan")
	case errors.Is(err, service.ErrQuotaUnknown):
		writeError(w, http.StatusUnprocessableEntity, "quota_unknown", "Paket ATM tidak dikenali, kuota tidak dapat di-reset")
	case errors.Is(err, service.ErrVisitNotCancelable):
		writeError(w, http.StatusConflict, "conflict", "Kunjungan sudah dibatalkan atau berasal dari periode sebelum reset terakhir")
	case errors.Is(err, service.ErrNotChecker):
		writeForbidden(w, "Hanya SPV/ADMIN yang dapat melakukan aksi ini")
	case errors.Is(err, service.ErrCancelVisitReason):
		writeValidationError(w, "reason", "wajib diisi")
	case errors.As(err, &validationErr):
		writeError(w, http.StatusBadRequest, "bad_request", fmt.Sprintf("%s %s", validationErr.Field, validationErr.Message))
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
	}
}
