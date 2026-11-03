package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"mime/multipart"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/repository"
	"github.com/cimb-niaga/cms/backend/internal/service"
	"github.com/cimb-niaga/cms/pkg/csvexport"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// MasterDataImportServicer is the subset of service.MasterDataImporter the
// handler needs.
type MasterDataImportServicer interface {
	DryRun(ctx context.Context, entity string, r io.Reader) (*service.ImportResult, error)
	Confirm(ctx context.Context, makerID int64, entity, fileName string, r io.Reader, actorIP string) (*service.ImportConfirmResult, error)
}

// MasterDataImportHistoryServicer is the subset of
// service.MasterDataImportHistoryService the handler needs.
type MasterDataImportHistoryServicer interface {
	List(ctx context.Context, f repository.ImportBatchFilter, page, pageSize int) (service.ImportBatchPage, error)
	Get(ctx context.Context, id int64, page, pageSize int) (service.ImportBatchDetail, error)
}

// AdminMasterDataImportHandler validates uploaded master-data CSVs (plan.md
// T5.3, dry-run: nothing is written) and stages them as one approval batch
// (T5.4, confirm), and serves the batch history. Mount at
// /api/v1/admin/master-data/import behind the shared masterDataAdmin group.
type AdminMasterDataImportHandler struct {
	svc     MasterDataImportServicer
	history MasterDataImportHistoryServicer // nil = /batches answers 404
	limiter *importRateLimiter              // nil = unlimited
}

// NewAdminMasterDataImportHandler creates a new AdminMasterDataImportHandler with the given dependency.
func NewAdminMasterDataImportHandler(svc MasterDataImportServicer) *AdminMasterDataImportHandler {
	return &AdminMasterDataImportHandler{svc: svc}
}

// WithHistory enables GET /batches and GET /batches/{id}.
func (h *AdminMasterDataImportHandler) WithHistory(hs MasterDataImportHistoryServicer) *AdminMasterDataImportHandler {
	h.history = hs
	return h
}

// Routes returns a chi.Router with the import endpoints (rate limited) and the
// history endpoints (not rate limited, Req 10.1) mounted.
func (h *AdminMasterDataImportHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Group(func(g chi.Router) {
		g.Use(h.rateLimit)
		g.Post("/{entity}/dry-run", h.DryRun)
		g.Post("/{entity}/confirm", h.Confirm)
	})
	r.Get("/batches", h.ListBatches)
	r.Get("/batches/{id}", h.GetBatch)
	return r
}

const (
	historyMaxPageSize     = 100
	historyDetailPageSize  = 50
	historyDefaultPageSize = 20
)

var importBatchStatuses = []string{"pending", "approved", "rejected", "applied", "stale"}

// ListBatches handles GET /batches?entity=&status=&mine=true&page=&page_size=.
func (h *AdminMasterDataImportHandler) ListBatches(w http.ResponseWriter, r *http.Request) {
	if h.history == nil {
		http.NotFound(w, r)
		return
	}
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	q := r.URL.Query()
	var f repository.ImportBatchFilter
	if v := q.Get("entity"); v != "" {
		if _, known := csvexport.Lookup(v); !known {
			writeBadFilter(w, "entity", "entitas impor tidak dikenal")
			return
		}
		f.Entity = &v
	}
	if v := q.Get("status"); v != "" {
		if !slices.Contains(importBatchStatuses, v) {
			writeBadFilter(w, "status", "status harus salah satu dari "+strings.Join(importBatchStatuses, ", "))
			return
		}
		f.Status = &v
	}
	if v := q.Get("mine"); v != "" {
		if v != "true" {
			writeBadFilter(w, "mine", "nilai harus 'true'")
			return
		}
		f.MakerID = &authCtx.UserID
	}
	page, ok := queryInt(w, q.Get("page"), "page", 1, 1, math.MaxInt32)
	if !ok {
		return
	}
	pageSize, ok := queryInt(w, q.Get("page_size"), "page_size", historyDefaultPageSize, 1, historyMaxPageSize)
	if !ok {
		return
	}
	res, err := h.history.List(r.Context(), f, page, pageSize)
	if err != nil {
		writeHistoryError(w, err)
		return
	}
	if res.Items == nil {
		res.Items = []service.ImportBatchSummary{}
	}
	writeJSON(w, http.StatusOK, res)
}

// GetBatch handles GET /batches/{id}?page=&page_size= (default size 50).
func (h *AdminMasterDataImportHandler) GetBatch(w http.ResponseWriter, r *http.Request) {
	if h.history == nil {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "id tidak valid")
		return
	}
	q := r.URL.Query()
	page, ok := queryInt(w, q.Get("page"), "page", 1, 1, math.MaxInt32)
	if !ok {
		return
	}
	pageSize, ok := queryInt(w, q.Get("page_size"), "page_size", historyDetailPageSize, 1, service.MasterDataImportMaxRows)
	if !ok {
		return
	}
	res, err := h.history.Get(r.Context(), id, page, pageSize)
	if err != nil {
		writeHistoryError(w, err)
		return
	}
	if res.Changes == nil {
		res.Changes = []service.ImportBatchChange{}
	}
	writeJSON(w, http.StatusOK, res)
}

// queryInt parses an optional integer query parameter within [lo, hi]; on a bad
// value it has already answered 400.
func queryInt(w http.ResponseWriter, raw, field string, def, lo, hi int) (int, bool) {
	if raw == "" {
		return def, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < lo || n > hi {
		writeBadFilter(w, field, "harus bilangan bulat "+strconv.Itoa(lo)+" sampai "+strconv.Itoa(hi))
		return 0, false
	}
	return n, true
}

func writeBadFilter(w http.ResponseWriter, field, message string) {
	writeJSON(w, http.StatusBadRequest, ErrorResponse{
		Error: "validation_error", Message: "Parameter tidak valid",
		Details: []FieldError{{Field: field, Message: message}},
	})
}

func writeHistoryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrMasterDataForbidden):
		writeForbidden(w, "Anda tidak berhak melihat riwayat impor")
	case errors.Is(err, service.ErrImportBatchNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Batch impor tidak ditemukan")
	default:
		slog.Error("master-data import history failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
	}
}

// multipartOverhead is headroom over the file limit for the multipart framing.
const multipartOverhead = 1 << 20

// DryRun handles POST /{entity}/dry-run with a multipart `file` field and
// answers 200 with the preview {valid_rows, errors[{row, field, message}], ...}.
// Problems in the DATA are part of that 200 answer; only an unreadable upload
// (no file, too big) or a server failure is an error status.
func (h *AdminMasterDataImportHandler) DryRun(w http.ResponseWriter, r *http.Request) {
	entity, file, _, ok := uploadedCSV(w, r)
	if !ok {
		return
	}
	defer func() { _ = file.Close() }()

	res, err := h.svc.DryRun(r.Context(), entity, file)
	if err != nil {
		writeImportError(w, entity, err)
		return
	}
	writeJSON(w, http.StatusOK, res.Preview)
}

// uploadedCSV validates the {entity} segment and extracts the multipart `file`
// under the size cap; on failure it has already answered. The header carries
// the client's file name (untrusted; the service sanitizes it).
func uploadedCSV(w http.ResponseWriter, r *http.Request) (entity string, file multipart.File, header *multipart.FileHeader, ok bool) {
	entity = chi.URLParam(r, "entity")
	if _, known := csvexport.Lookup(entity); !known {
		writeError(w, http.StatusNotFound, "not_found", "Entitas impor tidak dikenal")
		return "", nil, nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, service.MasterDataImportMaxBytes+multipartOverhead)
	file, header, err := r.FormFile("file")
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "file_too_large", "Ukuran file melebihi batas")
			return "", nil, nil, false
		}
		writeValidationError(w, "file", "wajib mengunggah file CSV pada field 'file'")
		return "", nil, nil, false
	}
	return entity, file, header, true
}

// Confirm handles POST /{entity}/confirm (plan.md T5.4): the same upload as the
// dry-run, staged as ONE batch under a single approval. 202 for a new batch;
// 200 when this exact file had already been staged (the existing batch is
// returned, nothing new is created). A file with row errors is 422 with the
// dry-run's error list and stages nothing.
func (h *AdminMasterDataImportHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	entity, file, header, ok := uploadedCSV(w, r)
	if !ok {
		return
	}
	defer func() { _ = file.Close() }()

	res, err := h.svc.Confirm(r.Context(), authCtx.UserID, entity, header.Filename, file, extractClientIP(r))
	if err != nil {
		writeImportError(w, entity, err)
		return
	}
	status := http.StatusAccepted
	if res.Existing {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{
		"batch_id": res.BatchID, "rows": res.Rows, "approval_request_id": res.ApprovalRequestID,
		"existing": res.Existing, "entity": entity,
	})
}

func writeImportError(w http.ResponseWriter, entity string, err error) {
	var ve *service.ValidationError
	var invalid *service.ImportInvalidError
	switch {
	case errors.Is(err, service.ErrMasterDataForbidden):
		writeForbidden(w, "Anda tidak berhak mengimpor master data")
	case errors.As(err, &invalid):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "validation_failed", "message": "File impor berisi kesalahan", "errors": invalid.Errors})
	case errors.Is(err, service.ErrImportNothingToDo):
		writeError(w, http.StatusUnprocessableEntity, "nothing_to_import", "Tidak ada perubahan untuk diimpor")
	case errors.Is(err, service.ErrMasterDataChangePending):
		writeError(w, http.StatusConflict, "change_pending", err.Error())
	case errors.As(err, &ve):
		writeValidationError(w, ve.Field, ve.Message)
	default:
		slog.Error("master-data import failed", "entity", entity, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
	}
}
