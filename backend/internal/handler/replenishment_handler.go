package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	"github.com/cimb-niaga/cms/backend/internal/vendorrole"
	"github.com/cimb-niaga/cms/pkg/middleware"
	"github.com/cimb-niaga/cms/pkg/reports"
)

// Replenishment realisasi vs order (.claude/sdlc/replenishment-realisasi/spec.md FR4, FR5.2, FR6).
// Read-only; RequireRoles at the route group and the service re-checks the role (FR4.3).

// ReplenishmentReader is the slice of service.ReplenishmentReadService behind these routes.
type ReplenishmentReader interface {
	List(ctx context.Context, actor service.Actor, f service.ReplenishmentFilter) (*service.ClassificationList, error)
	Summary(ctx context.Context, actor service.Actor, from, to time.Time, vendorID *int64) (*service.ReplenishmentSummary, error)
	AmountMismatch(ctx context.Context, actor service.Actor, f service.ReplenishmentFilter) ([]service.ClassificationView, error)
	Trips(ctx context.Context, actor service.Actor, from, to time.Time, vendorID *int64) ([]service.TripRow, error)
}

type ReplenishmentHandler struct {
	svc ReplenishmentReader
}

func NewReplenishmentHandler(svc ReplenishmentReader) *ReplenishmentHandler {
	return &ReplenishmentHandler{svc: svc}
}

func (h *ReplenishmentHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequireRoles(reports.ReplenishmentViewerRoles...))
	r.Get("/classifications", h.List)
	r.Get("/summary", h.Summary)
	r.Get("/reports/amount-mismatch", h.AmountMismatch)
	r.Get("/reports/trips", h.Trips)
	return r
}

// parseReplenishmentFilter reads date_from, date_to, classification, page, page_size (shared with the
// VendorPortal page) and, when internal, vendor_id, region_code, only_diff.
func parseReplenishmentFilter(r *http.Request, internal bool) (service.ReplenishmentFilter, *service.ValidationError) {
	q := r.URL.Query()
	var f service.ReplenishmentFilter
	for field, dst := range map[string]*time.Time{"date_from": &f.From, "date_to": &f.To} {
		if raw := q.Get(field); raw != "" {
			d, err := time.Parse("2006-01-02", raw)
			if err != nil {
				return f, &service.ValidationError{Field: field, Message: "format tanggal YYYY-MM-DD"}
			}
			*dst = d
		}
	}
	if c := q.Get("classification"); c != "" {
		if !reports.ReplenishmentClasses[c] {
			return f, &service.ValidationError{Field: "classification", Message: "tidak dikenal"}
		}
		f.Classification = &c
	}
	f.Page, _ = strconv.Atoi(q.Get("page"))
	if f.Page > maxPage {
		return f, &service.ValidationError{Field: "page", Message: "maksimal " + strconv.Itoa(maxPage)}
	}
	f.PageSize, _ = strconv.Atoi(q.Get("page_size"))
	if !internal {
		return f, nil
	}
	if raw := q.Get("vendor_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			return f, &service.ValidationError{Field: "vendor_id", Message: "tidak valid"}
		}
		f.VendorID = &id
	}
	if rc := q.Get("region_code"); rc != "" {
		f.RegionCode = &rc
	}
	f.OnlyDiff = q.Get("only_diff") == "true"
	return f, nil
}

func (h *ReplenishmentHandler) withFilter(w http.ResponseWriter, r *http.Request, fn func(service.Actor, service.ReplenishmentFilter) error) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	f, verr := parseReplenishmentFilter(r, true)
	if verr != nil {
		writeValidationError(w, verr.Field, verr.Message)
		return
	}
	if err := fn(actor, f); err != nil {
		replenishmentError(w, err)
	}
}

func replenishmentError(w http.ResponseWriter, err error) {
	var ve *service.ValidationError
	switch {
	case errors.As(err, &ve):
		writeValidationError(w, ve.Field, ve.Message)
	case errors.Is(err, service.ErrNotAuthorized):
		writeForbidden(w, "Anda tidak berhak melihat data replenishment")
	default:
		writeUnexpectedError(w, err)
	}
}

// List handles GET /classifications (FR4.1).
func (h *ReplenishmentHandler) List(w http.ResponseWriter, r *http.Request) {
	h.withFilter(w, r, func(a service.Actor, f service.ReplenishmentFilter) error {
		out, err := h.svc.List(r.Context(), a, f)
		if err == nil {
			writeJSON(w, http.StatusOK, out)
		}
		return err
	})
}

// Summary handles GET /summary (FR4.2).
func (h *ReplenishmentHandler) Summary(w http.ResponseWriter, r *http.Request) {
	h.withFilter(w, r, func(a service.Actor, f service.ReplenishmentFilter) error {
		out, err := h.svc.Summary(r.Context(), a, f.From, f.To, f.VendorID)
		if err == nil {
			writeJSON(w, http.StatusOK, out)
		}
		return err
	})
}

// AmountMismatch handles GET /reports/amount-mismatch (FR6.1); format=csv is rejected (8.1).
func (h *ReplenishmentHandler) AmountMismatch(w http.ResponseWriter, r *http.Request) {
	if rejectCSVFormat(w, r) {
		return
	}
	h.withFilter(w, r, func(a service.Actor, f service.ReplenishmentFilter) error {
		rows, err := h.svc.AmountMismatch(r.Context(), a, f)
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]any{"items": rows})
		}
		return err
	})
}

// Trips handles GET /reports/trips (FR6.2); format=csv is rejected (8.1).
func (h *ReplenishmentHandler) Trips(w http.ResponseWriter, r *http.Request) {
	if rejectCSVFormat(w, r) {
		return
	}
	h.withFilter(w, r, func(a service.Actor, f service.ReplenishmentFilter) error {
		rows, err := h.svc.Trips(r.Context(), a, f.From, f.To, f.VendorID)
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]any{"items": rows})
		}
		return err
	})
}

// RealizationRoutes: GET / on /api/v1/vendor/replenish-realizations (FR5.2), vendor roles only;
// the service refuses VENDOR-VAULT-USER (vendor-party-scope Req 4.2).
func (h *VendorOrderHandler) RealizationRoutes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequireRoles(vendorrole.All...))
	r.Get("/", h.ListRealizations)
	return r
}

// ListRealizations handles GET /api/v1/vendor/replenish-realizations?date_from&date_to&classification&page.
func (h *VendorOrderHandler) ListRealizations(w http.ResponseWriter, r *http.Request) {
	va, ok := vendorActorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	f, verr := parseReplenishmentFilter(r, false)
	if verr != nil {
		writeValidationError(w, verr.Field, verr.Message)
		return
	}
	out, err := h.svc.ListRealizations(r.Context(), va, f)
	if err != nil {
		replenishmentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
