package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// MasterDataImportServicer is the subset of service.MasterDataImporter the
// handler needs.
type MasterDataImportServicer interface {
	DryRun(ctx context.Context, entity string, r io.Reader) (*service.ImportResult, error)
	Confirm(ctx context.Context, makerID int64, entity string, r io.Reader, actorIP string) (*service.ImportConfirmResult, error)
}

// AdminMasterDataImportHandler validates uploaded master-data CSVs (plan.md
// T5.3, dry-run: nothing is written) and stages them as one approval batch
// (T5.4, confirm). Mount at
// /api/v1/admin/master-data/import behind the shared masterDataAdmin group.
type AdminMasterDataImportHandler struct {
	svc     MasterDataImportServicer
	limiter *importRateLimiter // nil = unlimited
}

// NewAdminMasterDataImportHandler creates a new AdminMasterDataImportHandler with the given dependency.
func NewAdminMasterDataImportHandler(svc MasterDataImportServicer) *AdminMasterDataImportHandler {
	return &AdminMasterDataImportHandler{svc: svc}
}

// Routes returns a chi.Router with the dry-run endpoint mounted.
func (h *AdminMasterDataImportHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(h.rateLimit)
	r.Post("/{entity}/dry-run", h.DryRun)
	r.Post("/{entity}/confirm", h.Confirm)
	return r
}

// multipartOverhead is headroom over the file limit for the multipart framing.
const multipartOverhead = 1 << 20

// DryRun handles POST /{entity}/dry-run with a multipart `file` field and
// answers 200 with the preview {valid_rows, errors[{row, field, message}], ...}.
// Problems in the DATA are part of that 200 answer; only an unreadable upload
// (no file, too big) or a server failure is an error status.
func (h *AdminMasterDataImportHandler) DryRun(w http.ResponseWriter, r *http.Request) {
	entity, file, ok := uploadedCSV(w, r)
	if !ok {
		return
	}
	defer file.Close()

	res, err := h.svc.DryRun(r.Context(), entity, file)
	if err != nil {
		writeImportError(w, entity, err)
		return
	}
	writeJSON(w, http.StatusOK, res.Preview)
}

// uploadedCSV validates the {entity} segment and extracts the multipart `file`
// under the size cap; on failure it has already answered.
func uploadedCSV(w http.ResponseWriter, r *http.Request) (entity string, file multipart.File, ok bool) {
	entity, err := service.ParseExportEntity(chi.URLParam(r, "entity"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Entitas impor tidak dikenal")
		return "", nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, service.MasterDataImportMaxBytes+multipartOverhead)
	file, _, err = r.FormFile("file")
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "file_too_large", "Ukuran file melebihi batas")
			return "", nil, false
		}
		writeValidationError(w, "file", "wajib mengunggah file CSV pada field 'file'")
		return "", nil, false
	}
	return entity, file, true
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
	entity, file, ok := uploadedCSV(w, r)
	if !ok {
		return
	}
	defer file.Close()

	res, err := h.svc.Confirm(r.Context(), authCtx.UserID, entity, file, extractClientIP(r))
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
