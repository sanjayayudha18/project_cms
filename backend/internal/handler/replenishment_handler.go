package handler

import (
	"context"
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// Replenishment realisasi vs order (.claude/sdlc/replenishment-realisasi/spec.md FR4, FR5.2, FR6).
// Read-only; RequireRoles at the route group and the service re-checks the role (FR4.3).

// ReplenishmentReader is the slice of service.ReplenishmentReadService behind these routes.
type ReplenishmentReader interface {
	List(ctx context.Context, actor service.Actor, f service.ReplenishmentFilter) (*service.ClassificationList, error)
	Summary(ctx context.Context, actor service.Actor, from, to time.Time, vendorID *int64) (*service.ReplenishmentSummary, error)
	AmountMismatch(ctx context.Context, actor service.Actor, f service.ReplenishmentFilter) ([]service.ClassificationView, error)
	Trips(ctx context.Context, actor service.Actor, from, to time.Time, vendorID *int64) ([]service.TripRow, error)
	RecordExport(ctx context.Context, actor service.Actor, report string, filter map[string]any) error
}

type ReplenishmentHandler struct {
	svc ReplenishmentReader
}

func NewReplenishmentHandler(svc ReplenishmentReader) *ReplenishmentHandler {
	return &ReplenishmentHandler{svc: svc}
}

func (h *ReplenishmentHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequireRoles(vendorRequestViewerRoles...))
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
		if !service.ReplenishmentClasses[c] {
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

func exportFilter(f service.ReplenishmentFilter) map[string]any {
	return map[string]any{"date_from": formatDate(f.From), "date_to": formatDate(f.To), "vendor_id": f.VendorID, "region_code": f.RegionCode}
}

// AmountMismatch handles GET /reports/amount-mismatch[?format=csv] (FR6.1, FR6.3).
func (h *ReplenishmentHandler) AmountMismatch(w http.ResponseWriter, r *http.Request) {
	h.withFilter(w, r, func(a service.Actor, f service.ReplenishmentFilter) error {
		rows, err := h.svc.AmountMismatch(r.Context(), a, f)
		if err != nil {
			return err
		}
		if r.URL.Query().Get("format") != "csv" {
			writeJSON(w, http.StatusOK, map[string]any{"items": rows})
			return nil
		}
		if err := h.svc.RecordExport(r.Context(), a, "amount_mismatch", exportFilter(f)); err != nil {
			return err
		}
		lines := [][]string{{"report_date", "ticket_number", "request_number", "terminal_id", "vendor", "branch_code", "region_code",
			"scheduled_date", "realized_date", "classification", "workday_offset", "denom", "order_amount", "itm_amount",
			"vendor_amount", "diff", "currency"}}
		for _, v := range rows {
			for _, am := range v.Amounts {
				lines = append(lines, []string{dateCell(v.ScheduledDate, v.RealizedDate), str(v.TicketNumber), str(v.RequestNumber), v.TerminalID,
					str(v.VendorName), str(v.BranchCode), str(v.RegionCode), dateCell(v.ScheduledDate, nil), dateCell(v.RealizedDate, nil),
					v.Classification, intCell(v.WorkdayOffset), strconv.Itoa(int(am.Denom)), strconv.FormatInt(am.Order, 10),
					strconv.FormatInt(am.Itm, 10), int64Cell(am.Vendor), strconv.FormatInt(am.Diff, 10), v.Currency})
			}
		}
		writeCSV(w, "replenishment-amount-mismatch-"+formatDate(f.From)+"-"+formatDate(f.To), lines)
		return nil
	})
}

// Trips handles GET /reports/trips[?format=csv] (FR6.2, FR6.3).
func (h *ReplenishmentHandler) Trips(w http.ResponseWriter, r *http.Request) {
	h.withFilter(w, r, func(a service.Actor, f service.ReplenishmentFilter) error {
		rows, err := h.svc.Trips(r.Context(), a, f.From, f.To, f.VendorID)
		if err != nil {
			return err
		}
		if r.URL.Query().Get("format") != "csv" {
			writeJSON(w, http.StatusOK, map[string]any{"items": rows})
			return nil
		}
		if err := h.svc.RecordExport(r.Context(), a, "trips", exportFilter(f)); err != nil {
			return err
		}
		lines := [][]string{{"vendor_id", "vendor", "terminal_id", "trips", "first_date", "last_date"}}
		for _, t := range rows {
			lines = append(lines, []string{int64Cell(t.VendorID), str(t.VendorName), t.TerminalID, strconv.FormatInt(t.Trips, 10),
				dateCell(t.FirstDate, nil), dateCell(t.LastDate, nil)})
		}
		writeCSV(w, "replenishment-trips-"+formatDate(f.From)+"-"+formatDate(f.To), lines)
		return nil
	})
}

// writeCSV sends a small, fully built report (<= 31 days) as a download; amounts are plain integers.
func writeCSV(w http.ResponseWriter, name string, lines [][]string) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`.csv"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	for _, line := range lines {
		for i, cell := range line {
			line[i] = service.CSVSafe(cell) // vendor/branch names are typed by admins (review R5)
		}
	}
	cw := csv.NewWriter(w)
	_ = cw.WriteAll(lines) // headers are out: a write error can only abort the stream
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func int64Cell(p *int64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatInt(*p, 10)
}

func intCell(p *int16) string {
	if p == nil {
		return ""
	}
	return strconv.Itoa(int(*p))
}

// dateCell formats the first non-nil date.
func dateCell(a, b *time.Time) string {
	for _, t := range []*time.Time{a, b} {
		if t != nil {
			return formatDate(*t)
		}
	}
	return ""
}

// RealizationRoutes: GET / on /api/v1/vendor/replenish-realizations (FR5.2), VENDOR-USER only.
func (h *VendorOrderHandler) RealizationRoutes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequireRoles("VENDOR-USER"))
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
