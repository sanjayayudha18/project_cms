package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// DSR late report (.claude/sdlc/dsr-late-report/spec.md FR4, FR7). Read-only, internal only (#17);
// RequireRoles at the route group and the service re-checks the role (FR4.6).

var dsrLateViewerRoles = []string{"ATM-USER", "ATM-SPV", "BRANCH-ATM-SPV", "ADMIN"}

// DsrLateReader is the slice of service.DsrLateReportService behind these routes.
type DsrLateReader interface {
	Daily(ctx context.Context, actor service.Actor, f service.DsrLateFilter) (*service.DsrLateDailyList, error)
	DailyAll(ctx context.Context, actor service.Actor, f service.DsrLateFilter) ([]service.DsrLateDailyView, error)
	Summary(ctx context.Context, actor service.Actor, f service.DsrLateFilter) (*service.DsrLateSummary, error)
	Warnings(ctx context.Context, actor service.Actor, f service.DsrLateFilter) (*service.DsrLateWarnings, error)
	RecordExport(ctx context.Context, actor service.Actor, report string, filter map[string]any) error
}

type DsrLateHandler struct {
	svc DsrLateReader
}

func NewDsrLateHandler(svc DsrLateReader) *DsrLateHandler { return &DsrLateHandler{svc: svc} }

func (h *DsrLateHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequireRoles(dsrLateViewerRoles...))
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
		if !service.DsrLateStatuses[s] {
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

func dsrLateExportFilter(f service.DsrLateFilter, withStatus bool) map[string]any {
	m := map[string]any{"month": f.Month.Format("2006-01"), "vendor_id": f.VendorID, "vendor_branch_id": f.VendorBranchID}
	if withStatus {
		m["status"] = f.Status
	}
	return m
}

// Daily handles GET /daily[?format=csv] (FR4.2, FR4.4).
func (h *DsrLateHandler) Daily(w http.ResponseWriter, r *http.Request) {
	h.withFilter(w, r, func(a service.Actor, f service.DsrLateFilter) error {
		if r.URL.Query().Get("format") != "csv" {
			out, err := h.svc.Daily(r.Context(), a, f)
			if err == nil {
				writeJSON(w, http.StatusOK, out)
			}
			return err
		}
		rows, err := h.svc.DailyAll(r.Context(), a, f)
		if err != nil {
			return err
		}
		if err := h.svc.RecordExport(r.Context(), a, "daily", dsrLateExportFilter(f, true)); err != nil {
			return err
		}
		lines := [][]string{{"report_date", "vendor_code", "vendor", "branch_code", "branch_name", "region_code",
			"deadline_wib", "first_received_at_wib", "status", "late_minutes", "receipt_checksum", "matched_locations"}}
		for _, v := range rows {
			lines = append(lines, []string{v.ReportDate, v.VendorCode, v.VendorName, v.BranchCode, v.BranchName, str(v.RegionCode),
				wibCell(&v.Deadline), wibCell(v.FirstReceivedAt), v.Status, strconv.Itoa(int(v.LateMinutes)),
					v.ReceiptChecksum, strings.Join(v.MatchedLabels, "; ")}) // R3: what decided the status, for later disputes
		}
		writeCSV(w, "dsr-late-daily-"+f.Month.Format("2006-01"), lines)
		return nil
	})
}

// Summary handles GET /summary[?format=csv] (FR4.3, FR4.4).
func (h *DsrLateHandler) Summary(w http.ResponseWriter, r *http.Request) {
	h.withFilter(w, r, func(a service.Actor, f service.DsrLateFilter) error {
		out, err := h.svc.Summary(r.Context(), a, f)
		if err != nil {
			return err
		}
		if r.URL.Query().Get("format") != "csv" {
			writeJSON(w, http.StatusOK, out)
			return nil
		}
		if err := h.svc.RecordExport(r.Context(), a, "summary", dsrLateExportFilter(f, false)); err != nil {
			return err
		}
		lines := [][]string{{"vendor_code", "vendor", "branch_code", "branch_name", "region_code",
			"obliged_days", "ok_days", "late_days", "not_sent_days", "pending_days", "late_minutes_total"}}
		for _, v := range out.Items {
			lines = append(lines, []string{v.VendorCode, v.VendorName, v.BranchCode, v.BranchName, str(v.RegionCode),
				itoa(v.ObligedDays), itoa(v.OkDays), itoa(v.LateDays), itoa(v.NotSentDays), itoa(v.PendingDays), itoa(v.LateMinutesTotal)})
		}
		writeCSV(w, "dsr-late-summary-"+f.Month.Format("2006-01"), lines)
		return nil
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

func itoa(n int32) string { return strconv.Itoa(int(n)) }

// wibCell formats a timestamp in WIB; nil = empty cell.
func wibCell(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}
