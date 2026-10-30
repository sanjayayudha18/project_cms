package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	"github.com/cimb-niaga/cms/pkg/middleware"
	"github.com/cimb-niaga/cms/pkg/reports"
)

// DSR late report (.claude/sdlc/dsr-late-report/spec.md FR4, FR7). Read-only, internal only (#17);
// RequireRoles at the route group and the service re-checks the role (FR4.6).

// DsrLateReader is the slice of service.DsrLateReportService behind these routes.
type DsrLateReader interface {
	Daily(ctx context.Context, actor service.Actor, f service.DsrLateFilter) (*service.DsrLateDailyList, error)
	Summary(ctx context.Context, actor service.Actor, f service.DsrLateFilter) (*service.DsrLateSummary, error)
	Warnings(ctx context.Context, actor service.Actor, f service.DsrLateFilter) (*service.DsrLateWarnings, error)
}

type DsrLateHandler struct {
	svc DsrLateReader
}

func NewDsrLateHandler(svc DsrLateReader) *DsrLateHandler { return &DsrLateHandler{svc: svc} }

func (h *DsrLateHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequireRoles(reports.DsrLateViewerRoles...))
	r.Get("/daily", h.Daily)
	r.Get("/summary", h.Summary)
	r.Get("/warnings", h.Warnings)
	return r
}

// parseDsrLateFilter reads month (required), vendor_id, vendor_branch_id, status, page, page_size.
func parseDsrLateFilter(r *http.Request) (service.DsrLateFilter, *service.ValidationError) {
	q := r.URL.Query()
	var f service.DsrLateFilter
	month, err := service.ParseDsrLateMonth(q.Get("month"))
	if err != nil {
		var ve *service.ValidationError
		errors.As(err, &ve)
		return f, ve
	}
	f.Month = month
	for field, dst := range map[string]**int64{"vendor_id": &f.VendorID, "vendor_branch_id": &f.VendorBranchID} {
		if raw := q.Get(field); raw != "" {
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || id <= 0 {
				return f, &service.ValidationError{Field: field, Message: "tidak valid"}
			}
			*dst = &id
		}
	}
	if s := q.Get("status"); s != "" {
		if !reports.DsrLateStatuses[s] {
			return f, &service.ValidationError{Field: "status", Message: "tidak dikenal"}
		}
		f.Status = &s
	}
	f.Page, _ = strconv.Atoi(q.Get("page"))
	if f.Page > maxPage {
		return f, &service.ValidationError{Field: "page", Message: "maksimal " + strconv.Itoa(maxPage)}
	}
	f.PageSize, _ = strconv.Atoi(q.Get("page_size"))
	return f, nil
}

// writeBadRequest: spec FR4 maps a bad/missing month (and other query params) to 400.
func writeBadRequest(w http.ResponseWriter, field, message string) {
	writeJSON(w, http.StatusBadRequest, ErrorResponse{
		Error: "validation_error", Message: "Validasi gagal", Details: []FieldError{{Field: field, Message: message}},
	})
}

func dsrLateError(w http.ResponseWriter, err error) {
	var ve *service.ValidationError
	switch {
	case errors.As(err, &ve):
		writeBadRequest(w, ve.Field, ve.Message)
	case errors.Is(err, service.ErrNotAuthorized):
		writeForbidden(w, "Anda tidak berhak melihat laporan DSR")
	default:
		writeUnexpectedError(w, err)
	}
}

func (h *DsrLateHandler) withFilter(w http.ResponseWriter, r *http.Request, fn func(service.Actor, service.DsrLateFilter) error) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	f, verr := parseDsrLateFilter(r)
	if verr != nil {
		writeBadRequest(w, verr.Field, verr.Message)
		return
	}
	if err := fn(actor, f); err != nil {
		dsrLateError(w, err)
	}
}

// rejectCSVFormat answers 400 for ?format=csv (report-export-migration 8.1): the CSV export moved to
// backend-exq. Called first, before any filter parsing, service call, audit or data read. Any other
// format value is ignored (JSON, as before). Returns true when it wrote the response.
func rejectCSVFormat(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Query().Get("format") != "csv" {
		return false
	}
	writeBadRequest(w, "format", "Ekspor CSV dipindah ke menu ekspor (POST /api/v1/exports/reports/...)")
	return true
}

// Daily handles GET /daily (FR4.2).
func (h *DsrLateHandler) Daily(w http.ResponseWriter, r *http.Request) {
	if rejectCSVFormat(w, r) {
		return
	}
	h.withFilter(w, r, func(a service.Actor, f service.DsrLateFilter) error {
		out, err := h.svc.Daily(r.Context(), a, f)
		if err == nil {
			writeJSON(w, http.StatusOK, out)
		}
		return err
	})
}

// Summary handles GET /summary (FR4.3).
func (h *DsrLateHandler) Summary(w http.ResponseWriter, r *http.Request) {
	if rejectCSVFormat(w, r) {
		return
	}
	h.withFilter(w, r, func(a service.Actor, f service.DsrLateFilter) error {
		out, err := h.svc.Summary(r.Context(), a, f)
		if err == nil {
			writeJSON(w, http.StatusOK, out)
		}
		return err
	})
}

// Warnings handles GET /warnings (FR3.4).
func (h *DsrLateHandler) Warnings(w http.ResponseWriter, r *http.Request) {
	h.withFilter(w, r, func(a service.Actor, f service.DsrLateFilter) error {
		out, err := h.svc.Warnings(r.Context(), a, f)
		if err == nil {
			writeJSON(w, http.StatusOK, out)
		}
		return err
	})
}
