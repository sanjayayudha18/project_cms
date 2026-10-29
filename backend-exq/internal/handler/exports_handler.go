package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend-exq/internal/exportjob"
	"github.com/cimb-niaga/cms/pkg/csvexport"
	"github.com/cimb-niaga/cms/pkg/middleware"
	"github.com/cimb-niaga/cms/pkg/response"
)

const (
	chunkWriteDeadline = 60 * time.Second
	abortTimeout       = 10 * time.Second
	defaultListLimit   = 20
	maxListLimit       = 50
	msgTooMany         = "Maksimal 3 ekspor berjalan bersamaan. Tunggu salah satu selesai."
)

// ExportsService is the slice of *exportjob.Service the handler uses.
type ExportsService interface {
	Create(ctx context.Context, a exportjob.Actor, p exportjob.Params) (exportjob.Job, bool, error)
	List(ctx context.Context, a exportjob.Actor, limit int) ([]exportjob.Job, error)
	Get(ctx context.Context, a exportjob.Actor, id int64) (exportjob.Job, error)
	StartDownload(ctx context.Context, a exportjob.Actor, id int64) (exportjob.Job, error)
	OpenChunks(ctx context.Context, jobID int64, chunkCount int64) exportjob.ChunkIterator
	CompleteDownload(ctx context.Context, id int64) error
	AbortDownload(ctx context.Context, id int64) error
}

// ExportsHandler serves /api/v1/exports (design "ExportRequestAPI + TemplateAPI").
type ExportsHandler struct {
	svc ExportsService
}

// NewExportsHandler wires the handler to its service.
func NewExportsHandler(svc ExportsService) *ExportsHandler { return &ExportsHandler{svc: svc} }

// Routes returns every route except the file download (main mounts Download
// with its own timeout). RequireAuth is applied by main; request and template
// routes add the per-kind role guard here, /jobs* rely on service-level checks (D23).
func (h *ExportsHandler) Routes() chi.Router {
	r := chi.NewRouter()
	guard := func(k exportjob.Kind) func(http.Handler) http.Handler {
		return middleware.RequireRoles(exportjob.KindRoles[k]...)
	}

	r.With(guard(exportjob.KindVendor)).Post("/vendors/{vendorID}/{entity}", h.requestVendor)
	r.With(guard(exportjob.KindCabang)).Post("/vendors/{vendorID}/branches/{branchID}/{entity}", h.requestBranch)
	r.With(guard(exportjob.KindHargaPaket)).Post("/vendors/{vendorID}/package-prices", h.requestPrices)
	r.With(guard(exportjob.KindMasterDataGlobal)).Post("/master-data/{entity}", h.requestGlobal)
	r.With(guard(exportjob.KindMasterDataGlobal)).Get("/master-data/{entity}/template", h.template)
	r.Get("/jobs", h.list)
	r.Get("/jobs/{jobID}", h.get)
	return r
}

func (h *ExportsHandler) requestVendor(w http.ResponseWriter, r *http.Request) {
	vid, ok := pathID(w, r, "vendorID")
	if !ok {
		return
	}
	h.create(w, r, exportjob.KindVendor, chi.URLParam(r, "entity"), &vid, nil, "")
}

func (h *ExportsHandler) requestBranch(w http.ResponseWriter, r *http.Request) {
	vid, ok := pathID(w, r, "vendorID")
	if !ok {
		return
	}
	bid, ok := pathID(w, r, "branchID")
	if !ok {
		return
	}
	h.create(w, r, exportjob.KindCabang, chi.URLParam(r, "entity"), &vid, &bid, "")
}

func (h *ExportsHandler) requestPrices(w http.ResponseWriter, r *http.Request) {
	vid, ok := pathID(w, r, "vendorID")
	if !ok {
		return
	}
	var bid *int64
	if raw := r.URL.Query().Get("branch_id"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v <= 0 {
			response.WriteError(w, http.StatusBadRequest, "bad_request", "branch_id harus bilangan bulat positif")
			return
		}
		bid = &v
	}
	h.create(w, r, exportjob.KindHargaPaket, "package-prices", &vid, bid, r.URL.Query().Get("price_level"))
}

func (h *ExportsHandler) requestGlobal(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, exportjob.KindMasterDataGlobal, chi.URLParam(r, "entity"), nil, nil, "")
}

func (h *ExportsHandler) create(w http.ResponseWriter, r *http.Request, kind exportjob.Kind, entity string, vendorID, branchID *int64, priceLevel string) {
	p, err := exportjob.ParseParams(kind, entity, vendorID, branchID, r.URL.Query().Get("status"), priceLevel)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	job, created, err := h.svc.Create(r.Context(), actorFrom(r), p)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusAccepted
	}
	writeJSON(w, status, response.Envelope{Success: true, Data: toJobJSON(job)})
}

// template serves the blank import template: synchronous, no DB, no audit (Req 14.2, 8.11).
func (h *ExportsHandler) template(w http.ResponseWriter, r *http.Request) {
	entity := chi.URLParam(r, "entity")
	if _, ok := csvexport.Lookup(entity); !ok {
		response.WriteError(w, http.StatusNotFound, "not_found", "Entitas ekspor tidak dikenal")
		return
	}
	setCSVHeaders(w, "master-data-"+entity+"-template.csv")
	if err := csvexport.WriteTemplate(w, entity); err != nil {
		slog.Error("export template failed", "entity", entity, "error", err)
	}
}

func (h *ExportsHandler) list(w http.ResponseWriter, r *http.Request) {
	limit := defaultListLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 || v > maxListLimit {
			response.WriteError(w, http.StatusBadRequest, "bad_request", "limit harus 1-50")
			return
		}
		limit = v
	}
	jobs, err := h.svc.List(r.Context(), actorFrom(r), limit)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	out := make([]jobJSON, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, toJobJSON(j))
	}
	response.WriteSuccess(w, out)
}

func (h *ExportsHandler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "jobID")
	if !ok {
		return
	}
	job, err := h.svc.Get(r.Context(), actorFrom(r), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.WriteSuccess(w, toJobJSON(job))
}

// Download streams the job file once (GET /jobs/{jobID}/file). Errors before the
// first byte are JSON; a failure mid-stream aborts the connection so the client
// sees a failed download, never a truncated file that looks complete (Req 9.13, 9.14).
func (h *ExportsHandler) Download(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "jobID")
	if !ok {
		return
	}
	job, err := h.svc.StartDownload(r.Context(), actorFrom(r), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if job.ByteSize == nil || job.ChunkCount == nil {
		slog.Error("export download: job without file metadata", "job_id", id)
		h.abort(id)
		response.WriteError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	setCSVHeaders(w, job.FileName)
	w.Header().Set("Content-Length", strconv.FormatInt(*job.ByteSize, 10))

	rc := http.NewResponseController(w)
	for data, err := range h.svc.OpenChunks(r.Context(), id, *job.ChunkCount) {
		if err == nil {
			// unsupported on test recorders; fine
			_ = rc.SetWriteDeadline(time.Now().Add(chunkWriteDeadline))
			if _, err = w.Write(data); err == nil { //nolint:gosec // G705: CSV attachment (text/csv, nosniff, Content-Disposition attachment), cells defused by csvexport.CSVSafe; never rendered as HTML
				err = rc.Flush()
			}
		}
		if err != nil {
			slog.Error("export download aborted", "job_id", id, "error", err)
			h.abort(id)
			panic(http.ErrAbortHandler)
		}
	}
	if err := h.svc.CompleteDownload(context.WithoutCancel(r.Context()), id); err != nil {
		slog.Error("export download complete failed", "job_id", id, "error", err)
	}
}

// abort returns the job to succeeded using a fresh context (the request ctx is usually dead).
func (h *ExportsHandler) abort(id int64) {
	ctx, cancel := context.WithTimeout(context.Background(), abortTimeout)
	defer cancel()
	if err := h.svc.AbortDownload(ctx, id); err != nil {
		slog.Error("export abort failed", "job_id", id, "error", err)
	}
}

func setCSVHeaders(w http.ResponseWriter, fileName string) {
	hd := w.Header()
	hd.Set("Content-Type", "text/csv; charset=utf-8")
	hd.Set("Content-Disposition", "attachment; filename="+strconv.Quote(fileName))
	hd.Set("Cache-Control", "no-store")
	hd.Set("X-Content-Type-Options", "nosniff")
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	v, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil || v <= 0 {
		response.WriteError(w, http.StatusBadRequest, "bad_request", name+" harus bilangan bulat positif")
		return 0, false
	}
	return v, true
}

// actorFrom builds the caller from the auth context; IP is r.RemoteAddr (set by
// TrustedRealIP), never a header. No auth context -> zero UserID -> service 401.
func actorFrom(r *http.Request) exportjob.Actor {
	a := exportjob.Actor{IP: remoteIP(r.RemoteAddr)}
	if ac, ok := middleware.GetAuthContext(r.Context()); ok {
		a.UserID, a.Username, a.Role = ac.UserID, ac.Username, ac.Role
	}
	return a
}

func remoteIP(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, exportjob.ErrValidation):
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
	case errors.Is(err, exportjob.ErrUnauthenticated):
		response.WriteError(w, http.StatusUnauthorized, "unauthorized", "Sesi tidak valid")
	case errors.Is(err, exportjob.ErrForbidden):
		response.WriteError(w, http.StatusForbidden, "forbidden", "Anda tidak memiliki akses ke resource ini")
	case errors.Is(err, exportjob.ErrUnknownEntity), errors.Is(err, exportjob.ErrNotFound):
		response.WriteError(w, http.StatusNotFound, "not_found", "Data tidak ditemukan")
	case errors.Is(err, exportjob.ErrNotReady):
		response.WriteError(w, http.StatusConflict, "export_not_ready", "Ekspor belum siap diunduh (status belum succeeded)")
	case errors.Is(err, exportjob.ErrGone):
		response.WriteError(w, http.StatusGone, "export_gone", "Ekspor sudah diunduh atau kedaluwarsa")
	case errors.Is(err, exportjob.ErrTooManyActive):
		response.WriteError(w, http.StatusTooManyRequests, "too_many_exports", msgTooMany)
	default:
		slog.Error("export request failed", "error", err)
		response.WriteError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
	}
}

// writeJSON writes an envelope with an arbitrary status (pkg/response has no 202 helper).
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// jobJSON is the public Job schema (design). Times are RFC 3339 UTC.
type jobJSON struct {
	ID           int64   `json:"id"`
	Kind         string  `json:"kind"`
	Entity       string  `json:"entity"`
	VendorID     *int64  `json:"vendor_id"`
	VendorCode   *string `json:"vendor_code"`
	BranchID     *int64  `json:"branch_id"`
	BranchCode   *string `json:"branch_code"`
	StatusFilter string  `json:"status_filter"`
	PriceLevel   *string `json:"price_level"`
	FileName     string  `json:"file_name"`
	Status       string  `json:"status"`
	RowCount     int64   `json:"row_count"`
	ByteSize     *int64  `json:"byte_size"`
	ErrorMessage *string `json:"error_message"`
	CreatedAt    string  `json:"created_at"`
	StartedAt    *string `json:"started_at"`
	FinishedAt   *string `json:"finished_at"`
	ExpiresAt    *string `json:"expires_at"`
	DownloadedAt *string `json:"downloaded_at"`
}

func toJobJSON(j exportjob.Job) jobJSON {
	return jobJSON{
		ID: j.ID, Kind: string(j.Params.Kind), Entity: j.Params.Entity,
		VendorID: j.Params.VendorID, VendorCode: nilIfEmpty(j.VendorCode),
		BranchID: j.Params.BranchID, BranchCode: nilIfEmpty(j.BranchCode),
		StatusFilter: j.Params.StatusFilter, PriceLevel: j.Params.PriceLevel,
		FileName: j.FileName, Status: string(j.Status), RowCount: j.RowCount,
		ByteSize: j.ByteSize, ErrorMessage: j.ErrorMessage,
		CreatedAt: j.CreatedAt.UTC().Format(time.RFC3339), StartedAt: ts(j.StartedAt),
		FinishedAt: ts(j.FinishedAt), ExpiresAt: ts(j.ExpiresAt), DownloadedAt: ts(j.DownloadedAt),
	}
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func ts(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}
