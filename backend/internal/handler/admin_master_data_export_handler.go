package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
)

// MasterDataExportServicer is the subset of service.MasterDataExporter the
// handler needs.
type MasterDataExportServicer interface {
	Authorize(ctx context.Context) error
	Write(ctx context.Context, entity, status string, w io.Writer, flush func()) (int, error)
}

// AdminMasterDataExportHandler streams master-data CSV exports (plan.md T5.1).
// Mount at /api/v1/admin/master-data/export behind the shared masterDataAdmin
// group (RequireAuth + RequireRoles("ADMIN","ADMIN_PARAM")) -- see cmd/api/main.go.
type AdminMasterDataExportHandler struct {
	svc MasterDataExportServicer
}

// NewAdminMasterDataExportHandler creates a new AdminMasterDataExportHandler with the given dependency.
func NewAdminMasterDataExportHandler(svc MasterDataExportServicer) *AdminMasterDataExportHandler {
	return &AdminMasterDataExportHandler{svc: svc}
}

// Routes returns a chi.Router with the export endpoint mounted.
func (h *AdminMasterDataExportHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{entity}", h.Export)
	r.Get("/{entity}/template", h.Template)
	return r
}

// Template handles GET /{entity}/template: the blank CSV (BOM + header row,
// no data, no DB access) an admin fills in for import (plan.md T5.2).
func (h *AdminMasterDataExportHandler) Template(w http.ResponseWriter, r *http.Request) {
	entity, err := service.ParseExportEntity(chi.URLParam(r, "entity"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Entitas ekspor tidak dikenal")
		return
	}
	if err := h.svc.Authorize(r.Context()); err != nil {
		writeForbidden(w, "Anda tidak berhak mengunduh template master data")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="master-data-`+entity+`-template.csv"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if err := service.WriteExportTemplate(w, entity); err != nil {
		slog.Error("master-data template failed", "entity", entity, "error", err)
	}
}

// wib is Asia/Jakarta (fixed +07:00, no DST); avoids depending on tzdata being
// present in the container image.
var wib = time.FixedZone("WIB", 7*60*60)

// countingWriter records whether any byte reached the client, which decides if
// a late error can still be answered with a normal JSON error response.
type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// Export handles GET /{entity}?status=active|disabled|all (default active) and
// streams the CSV. If a failure happens after bytes were sent the connection
// is aborted (http.ErrAbortHandler) so the client sees a failed download, never
// a truncated file that looks complete.
func (h *AdminMasterDataExportHandler) Export(w http.ResponseWriter, r *http.Request) {
	entity, err := service.ParseExportEntity(chi.URLParam(r, "entity"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Entitas ekspor tidak dikenal")
		return
	}
	status, err := parseStatusParam(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := h.svc.Authorize(r.Context()); err != nil {
		writeForbidden(w, "Anda tidak berhak mengekspor master data")
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="master-data-`+entity+`-`+time.Now().In(wib).Format("20060102")+`.csv"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	flush := func() {
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	cw := &countingWriter{w: w}
	if _, err := h.svc.Write(r.Context(), entity, status, cw, flush); err != nil {
		slog.Error("master-data export failed", "entity", entity, "status", status, "bytes_sent", cw.n, "error", err)
		if cw.n == 0 {
			// Nothing sent yet: replace the download headers with a normal error.
			w.Header().Del("Content-Disposition")
			w.Header().Set("Content-Type", "application/json")
			if errors.Is(err, service.ErrMasterDataForbidden) {
				writeForbidden(w, "Anda tidak berhak mengekspor master data")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
			return
		}
		panic(http.ErrAbortHandler)
	}
}
