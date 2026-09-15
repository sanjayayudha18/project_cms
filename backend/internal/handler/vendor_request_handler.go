package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// Defaults from the Vendor Request API contract (Req 3.3, 9.2).
const (
	defaultVendorRequestPageSize = 20
)

var (
	vendorRequestViewerRoles  = []string{"ADMIN", "ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV"}
	vendorRequestMakerRoles   = []string{"ADMIN", "ATM-USER", "BRANCH-ATM-USER"}
	vendorRequestCheckerRoles = []string{"ADMIN", "ATM-SPV", "BRANCH-ATM-SPV"}
	// vendorRequestCancelRoles: cancel is reachable by either a Maker (draft
	// or their own pending request) or a Checker (a pending request not
	// their own) — the actual authorization is enforced by
	// service.checkActor; this route-level gate only needs to admit both
	// role families (Req 2.12 vs Req 10.4/10.8, resolved as a union).
	vendorRequestCancelRoles = []string{"ADMIN", "ATM-USER", "BRANCH-ATM-USER", "ATM-SPV", "BRANCH-ATM-SPV"}
	vendorRequestAuditRoles  = []string{"ADMIN", "ATM-SPV", "BRANCH-ATM-SPV"}
)

// VendorRequestHandler handles Vendor Request HTTP endpoints.
type VendorRequestHandler struct {
	service service.VendorRequestServicer
}

// NewVendorRequestHandler creates a new VendorRequestHandler with the given service.
func NewVendorRequestHandler(svc service.VendorRequestServicer) *VendorRequestHandler {
	return &VendorRequestHandler{service: svc}
}

// Routes returns a chi.Router with all Vendor Request endpoints mounted.
// Each route carries its own RequireRoles on top of the RequireAuth the
// caller mounts this under (main.go), since the write-role subset differs
// per endpoint (design.md "API Contract").
func (h *VendorRequestHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.With(middleware.RequireRoles(vendorRequestViewerRoles...)).Get("/forecast", h.BrowseForecast)
	r.With(middleware.RequireRoles(vendorRequestViewerRoles...)).Get("/vendors", h.ListVendorOptions)
	r.With(middleware.RequireRoles(vendorRequestMakerRoles...)).Post("/", h.Create)
	r.With(middleware.RequireRoles(vendorRequestMakerRoles...)).Put("/{id}/items", h.UpdateItems)
	r.With(middleware.RequireRoles(vendorRequestMakerRoles...)).Post("/{id}/submit", h.Submit)
	r.With(middleware.RequireRoles(vendorRequestCheckerRoles...)).Post("/{id}/approve", h.Approve)
	r.With(middleware.RequireRoles(vendorRequestCheckerRoles...)).Post("/{id}/reject", h.Reject)
	r.With(middleware.RequireRoles(vendorRequestMakerRoles...)).Post("/{id}/revise", h.Revise)
	r.With(middleware.RequireRoles(vendorRequestCancelRoles...)).Post("/{id}/cancel", h.Cancel)
	r.With(middleware.RequireRoles(vendorRequestViewerRoles...)).Get("/", h.List)
	r.With(middleware.RequireRoles(vendorRequestViewerRoles...)).Get("/{id}", h.Get)
	r.With(middleware.RequireRoles(vendorRequestAuditRoles...)).Get("/{id}/audit-log", h.AuditLog)
	return r
}

// --- Handlers -------------------------------------------------------------

// BrowseForecast handles GET /forecast (Req 3).
func (h *VendorRequestHandler) BrowseForecast(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, err := parseIntParam(q, "page", defaultPage)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "page harus berupa angka")
		return
	}
	pageSize, err := parseIntParam(q, "page_size", defaultVendorRequestPageSize)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "page_size harus berupa angka")
		return
	}

	result, err := h.service.BrowseForecast(r.Context(), service.BrowseForecastParams{
		ForecastDate:    q.Get("forecast_date"),
		TerminalID:      q.Get("atm_id"),
		Brand:           q.Get("brand"),
		FLMVendor:       q.Get("flm_vendor"),
		FLMVendorRegion: q.Get("flm_vendor_region"),
		Page:            page,
		PageSize:        pageSize,
	})
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toForecastResponse(result))
}

// ListVendorOptions handles GET /vendors (CIT-2 Req 1.2, 1.3, 3 Q2): the
// active-vendor and distinct-region option lists for the Forecast Browser's
// required FLM Vendor / FLM Vendor Region selects and the manual-request
// vendor select.
func (h *VendorRequestHandler) ListVendorOptions(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListVendorOptions(r.Context())
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toVendorOptionsResponse(result))
}

type createVendorRequestBody struct {
	ForecastDate string `json:"forecast_date"` // required unless is_manual (Q5: manual anchors to today Jakarta instead)
	// CIT-2 additions (Req 2, 3, 4):
	ReplenishDate   string             `json:"replenish_date"`
	RequestCategory string             `json:"request_category"` // required on every create (Req 1): planned | emergency | additional
	IsManual        bool               `json:"is_manual"`
	VendorID        int64              `json:"vendor_id"`
	Notes           string             `json:"notes"`
	Items           []itemInputPayload `json:"items"`
}

type itemInputPayload struct {
	TerminalID      string `json:"terminal_id"`
	PeriodePred     string `json:"periode_pred"` // required unless is_manual (a manual item has no DMAA periode)
	Denom           int32  `json:"denom"`
	AmountReplenish int64  `json:"amount_replenish"`
	// CIT-2 (Req 3, Q4): operator-entered context for Manual_Request items only.
	Brand     string `json:"brand"`
	LokasiATM string `json:"lokasi_atm"`
}

// Create handles POST / (Req 4).
func (h *VendorRequestHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	var body createVendorRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "body tidak valid")
		return
	}

	var forecastDate time.Time
	if body.IsManual {
		// Q5: a Manual_Request has no DMAA periode, so forecast_date anchors
		// to the request's creation date in Asia/Jakarta instead of a
		// client-supplied value.
		now := time.Now().In(wibZone)
		forecastDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	} else {
		var err error
		forecastDate, err = time.Parse("2006-01-02", body.ForecastDate)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "forecast_date harus berformat YYYY-MM-DD")
			return
		}
	}

	replenishDate, err := time.Parse("2006-01-02", body.ReplenishDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "replenish_date harus berformat YYYY-MM-DD")
		return
	}

	items, err := toItemInputs(body.Items, body.IsManual)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	result, err := h.service.Create(r.Context(), actor, service.CreateVendorRequestInput{
		ForecastDate:    forecastDate,
		ReplenishDate:   replenishDate,
		RequestCategory: body.RequestCategory,
		IsManual:        body.IsManual,
		VendorID:        body.VendorID,
		Notes:           body.Notes,
		Items:           items,
	})
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toDetailResponse(result))
}

type updateItemsBody struct {
	Items []itemInputPayload `json:"items"`
}

// UpdateItems handles PUT /{id}/items (Req 8).
func (h *VendorRequestHandler) UpdateItems(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	id, err := parseVendorRequestID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var body updateItemsBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "body tidak valid")
		return
	}
	// UpdateItems is out of CIT-2 scope (Requirement 3 only extends Create) --
	// always DMAA-backed, so periode_pred stays required (isManual=false).
	items, err := toItemInputs(body.Items, false)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	result, err := h.service.UpdateItems(r.Context(), actor, id, items)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDetailResponse(result))
}

// Submit handles POST /{id}/submit (Req 5).
func (h *VendorRequestHandler) Submit(w http.ResponseWriter, r *http.Request) {
	h.doTransition(w, r, func(actor service.Actor, id int64) (*service.VendorRequestDetail, error) {
		return h.service.Submit(r.Context(), actor, id)
	})
}

// Approve handles POST /{id}/approve (Req 6).
func (h *VendorRequestHandler) Approve(w http.ResponseWriter, r *http.Request) {
	h.doTransition(w, r, func(actor service.Actor, id int64) (*service.VendorRequestDetail, error) {
		return h.service.Approve(r.Context(), actor, id)
	})
}

type rejectBody struct {
	RejectionReason string `json:"rejection_reason"`
}

// Reject handles POST /{id}/reject (Req 6).
func (h *VendorRequestHandler) Reject(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	id, err := parseVendorRequestID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var body rejectBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "body tidak valid")
		return
	}

	result, err := h.service.Reject(r.Context(), actor, id, body.RejectionReason)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDetailResponse(result))
}

// Revise handles POST /{id}/revise (Req 7).
func (h *VendorRequestHandler) Revise(w http.ResponseWriter, r *http.Request) {
	h.doTransition(w, r, func(actor service.Actor, id int64) (*service.VendorRequestDetail, error) {
		return h.service.Revise(r.Context(), actor, id)
	})
}

type cancelBody struct {
	CancellationReason string `json:"cancellation_reason"`
}

// Cancel handles POST /{id}/cancel (Req 10; replenishment-request-enhancements
// Req 3.2/3.3 add the required cancellation_reason body, so this no longer
// goes through doTransition like the other no-body transitions).
func (h *VendorRequestHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	id, err := parseVendorRequestID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var body cancelBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "body tidak valid")
		return
	}

	result, err := h.service.Cancel(r.Context(), actor, id, body.CancellationReason)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDetailResponse(result))
}

// doTransition is the shared parse-actor/parse-id/call/respond flow for the
// three no-body transition endpoints (submit, approve, revise). Cancel used
// to share this too, but now has its own body (cancellation_reason, Req 3.2).
func (h *VendorRequestHandler) doTransition(w http.ResponseWriter, r *http.Request, call func(actor service.Actor, id int64) (*service.VendorRequestDetail, error)) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	id, err := parseVendorRequestID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	result, err := call(actor, id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDetailResponse(result))
}

// List handles GET / (Req 9.1-9.5).
func (h *VendorRequestHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, err := parseIntParam(q, "page", defaultPage)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "page harus berupa angka")
		return
	}
	pageSize, err := parseIntParam(q, "page_size", defaultVendorRequestPageSize)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "page_size harus berupa angka")
		return
	}
	createdBy, err := parseIntParam(q, "created_by", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "created_by harus berupa angka")
		return
	}

	result, err := h.service.List(r.Context(), service.ListVendorRequestParams{
		Status:          splitNonEmpty(q.Get("status")),
		ForecastDate:    q.Get("forecast_date"),
		CreatedBy:       int64(createdBy),
		RequestNumber:   q.Get("request_number"),
		IncludeCanceled: q.Get("include_canceled") == "true",
		Page:            page,
		PageSize:        pageSize,
	})
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toListResponse(result))
}

// Get handles GET /{id} (Req 9.6).
func (h *VendorRequestHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseVendorRequestID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	result, err := h.service.Get(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDetailResponse(result))
}

// AuditLog handles GET /{id}/audit-log (Req 16.4-16.7).
func (h *VendorRequestHandler) AuditLog(w http.ResponseWriter, r *http.Request) {
	id, err := parseVendorRequestID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	entries, err := h.service.AuditLog(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAuditLogResponse(entries))
}

// --- parsing helpers --------------------------------------------------

func actorFromRequest(r *http.Request) (service.Actor, bool) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		return service.Actor{}, false
	}
	return service.Actor{UserID: authCtx.UserID, Role: authCtx.Role, IP: extractClientIP(r)}, true
}

func parseVendorRequestID(r *http.Request) (int64, error) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("id tidak valid")
	}
	return id, nil
}

// toItemInputs converts item payloads to service.ItemInput. periode_pred is
// required for a DMAA-backed item (matched against dmaa_atm_forecast), but a
// Manual_Request item (isManual) has no DMAA periode -- the service anchors
// it to replenish_date instead (Req 3, Q5), so an empty periode_pred is
// accepted and left zero-valued here.
func toItemInputs(items []itemInputPayload, isManual bool) ([]service.ItemInput, error) {
	out := make([]service.ItemInput, len(items))
	for i, it := range items {
		var periodePred time.Time
		switch {
		case it.PeriodePred != "":
			var err error
			periodePred, err = time.Parse("2006-01-02", it.PeriodePred)
			if err != nil {
				return nil, fmt.Errorf("items[%d].periode_pred harus berformat YYYY-MM-DD", i)
			}
		case !isManual:
			return nil, fmt.Errorf("items[%d].periode_pred wajib diisi", i)
		}
		out[i] = service.ItemInput{
			TerminalID:      it.TerminalID,
			PeriodePred:     periodePred,
			Denom:           it.Denom,
			AmountReplenish: it.AmountReplenish,
			Brand:           it.Brand,
			LokasiATM:       it.LokasiATM,
		}
	}
	return out, nil
}

func splitNonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// --- error mapping ------------------------------------------------------

// handleError maps service package errors to HTTP responses per
// design.md's "Error responses" table.
func (h *VendorRequestHandler) handleError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError
	var invalidItemsErr *service.InvalidItemsError

	switch {
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Vendor request tidak ditemukan")
	case errors.Is(err, service.ErrInvalidTransition):
		writeError(w, http.StatusConflict, "conflict", "Status saat ini tidak mengizinkan aksi ini")
	case errors.Is(err, service.ErrEmptyItems):
		writeError(w, http.StatusConflict, "conflict", "Request harus memiliki minimal 1 item")
	case errors.Is(err, service.ErrNumberExhausted):
		writeError(w, http.StatusConflict, "conflict", "Nomor request untuk tanggal ini sudah mencapai batas maksimum")
	case errors.Is(err, service.ErrAlreadyCanceled):
		writeError(w, http.StatusConflict, "conflict", "Vendor request sudah dibatalkan")
	case errors.Is(err, service.ErrNotCreator):
		writeForbidden(w, "Hanya pembuat request yang dapat melakukan aksi ini")
	case errors.Is(err, service.ErrNotChecker):
		writeForbidden(w, "Hanya checker yang dapat melakukan aksi ini")
	case errors.Is(err, service.ErrSelfApproval):
		writeForbidden(w, "Checker tidak boleh sama dengan pembuat request (four-eyes)")
	case errors.Is(err, service.ErrNotAuthorized):
		writeForbidden(w, "Anda tidak berhak melakukan aksi ini")
	case errors.As(err, &invalidItemsErr):
		writeError(w, http.StatusBadRequest, "bad_request", invalidItemsErr.Error())
	case errors.Is(err, service.ErrDuplicateItems):
		writeError(w, http.StatusBadRequest, "bad_request", "Terdapat item duplikat dalam request")
	case errors.Is(err, service.ErrRejectReasonEmpty):
		writeValidationError(w, "rejection_reason", "wajib diisi")
	case errors.Is(err, service.ErrCancelReasonEmpty):
		writeValidationError(w, "cancellation_reason", "wajib diisi")
	case errors.Is(err, service.ErrNumberGeneration):
		writeError(w, http.StatusInternalServerError, "internal_error", "Gagal membuat nomor request")
	case errors.As(err, &validationErr):
		writeError(w, http.StatusBadRequest, "bad_request", fmt.Sprintf("%s %s", validationErr.Field, validationErr.Message))
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
	}
}
