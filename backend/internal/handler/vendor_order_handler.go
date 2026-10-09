package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// Phase 2.2b (.claude/sdlc/cit-send-vendor) FR9 / API: VendorPortal replenish
// orders. Mounted under RequireAuth; every route also RequireRoles
// VENDOR-USER, and the service re-checks role + scope (FR5.4). The URL id is
// a party id, so one URL is exactly one scope check.

// VendorOrderServicer is the slice of service.VendorOrderService behind these routes.
type VendorOrderServicer interface {
	List(ctx context.Context, va service.VendorActor, f service.VendorOrderFilter) (*service.VendorOrderList, error)
	Get(ctx context.Context, va service.VendorActor, partyID int64) (*service.VendorOrderDetail, error)
	Accept(ctx context.Context, va service.VendorActor, partyID int64) (*service.VendorOrderDetail, error)
	Reject(ctx context.Context, va service.VendorActor, partyID int64, reason string) (*service.VendorOrderDetail, error)
}

var vendorPartyStatuses = map[string]bool{"": true, "pending": true, "accepted": true, "rejected": true, "withdrawn": true}

type VendorOrderHandler struct {
	svc VendorOrderServicer
}

func NewVendorOrderHandler(svc VendorOrderServicer) *VendorOrderHandler {
	return &VendorOrderHandler{svc: svc}
}

func (h *VendorOrderHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequireRoles("VENDOR-USER"))
	r.Get("/", h.List)
	r.Get("/{id}", h.Get)
	r.Post("/{id}/accept", h.Accept)
	r.Post("/{id}/reject", h.Reject)
	return r
}

func vendorActorFromRequest(r *http.Request) (service.VendorActor, bool) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		return service.VendorActor{}, false
	}
	return service.VendorActor{
		Actor:         service.Actor{UserID: authCtx.UserID, Role: authCtx.Role, IP: extractClientIP(r)},
		ClaimVendorID: authCtx.VendorID,
	}, true
}

// List handles GET /?party_status=&request_status=&from=&to=&page=&page_size=.
func (h *VendorOrderHandler) List(w http.ResponseWriter, r *http.Request) {
	va, ok := vendorActorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	q := r.URL.Query()
	f := service.VendorOrderFilter{PartyStatus: q.Get("party_status"), RequestStatus: q.Get("request_status")}
	if !vendorPartyStatuses[f.PartyStatus] {
		writeValidationError(w, "party_status", "tidak dikenal")
		return
	}
	for field, dst := range map[string]**time.Time{"from": &f.From, "to": &f.To} {
		if raw := q.Get(field); raw != "" {
			d, err := time.Parse("2006-01-02", raw)
			if err != nil {
				writeValidationError(w, field, "format tanggal YYYY-MM-DD")
				return
			}
			*dst = &d
		}
	}
	f.Page, _ = strconv.Atoi(q.Get("page"))
	f.PageSize, _ = strconv.Atoi(q.Get("page_size"))
	out, err := h.svc.List(r.Context(), va, f)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// Get handles GET /{id} (FR3).
func (h *VendorOrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	h.withParty(w, r, func(va service.VendorActor, id int64) (*service.VendorOrderDetail, error) {
		return h.svc.Get(r.Context(), va, id)
	})
}

// Accept handles POST /{id}/accept (FR4.2).
func (h *VendorOrderHandler) Accept(w http.ResponseWriter, r *http.Request) {
	h.withParty(w, r, func(va service.VendorActor, id int64) (*service.VendorOrderDetail, error) {
		return h.svc.Accept(r.Context(), va, id)
	})
}

// Reject handles POST /{id}/reject {reason} (FR4.3/FR4.4).
func (h *VendorOrderHandler) Reject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "body tidak valid")
		return
	}
	h.withParty(w, r, func(va service.VendorActor, id int64) (*service.VendorOrderDetail, error) {
		return h.svc.Reject(r.Context(), va, id, body.Reason)
	})
}

func (h *VendorOrderHandler) withParty(w http.ResponseWriter, r *http.Request,
	fn func(va service.VendorActor, id int64) (*service.VendorOrderDetail, error)) {
	va, ok := vendorActorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "id tidak valid")
		return
	}
	out, err := fn(va, id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *VendorOrderHandler) handleError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeValidationError(w, validationErr.Field, validationErr.Message)
	case errors.Is(err, service.ErrVendorOrderNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Request tidak ditemukan")
	case errors.Is(err, service.ErrInvalidTransition):
		writeError(w, http.StatusConflict, "conflict", "Request ini tidak dapat diterima atau ditolak saat ini")
	case errors.Is(err, service.ErrNotAuthorized):
		writeForbidden(w, "Anda tidak berhak melakukan aksi ini")
	default:
		writeUnexpectedError(w, err)
	}
}
