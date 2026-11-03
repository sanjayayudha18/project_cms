package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// MasterDataApprovalDetailReader reads the staged change(s) behind a
// master_data approval request. *repository.MasterDataChangeRepository satisfies it.
type MasterDataApprovalDetailReader interface {
	GetByID(ctx context.Context, id int64) (db.MasterDataChangeRequest, error)
	ListByBatch(ctx context.Context, batchID int64) ([]db.MasterDataChangeRequest, error)
}

// WithMasterDataDetail enables GET /{id}/master-data.
func (h *ApprovalHandler) WithMasterDataDetail(r MasterDataApprovalDetailReader) *ApprovalHandler {
	h.mdDetail = r
	return h
}

// MasterDataApprovalBatchReader reads the import batch header.
// *repository.MasterDataImportBatchRepository satisfies it. Optional: without
// it the response simply has no "batch" object.
type MasterDataApprovalBatchReader interface {
	GetBatch(ctx context.Context, id int64) (db.GetMasterDataImportBatchRow, error)
}

// WithMasterDataBatches adds the batch header to GET /{id}/master-data.
func (h *ApprovalHandler) WithMasterDataBatches(r MasterDataApprovalBatchReader) *ApprovalHandler {
	h.mdBatch = r
	return h
}

const (
	// approvalMasterDataDetailLimit is the default and maximum page size for an
	// import batch; the per-op counts always cover the whole batch.
	approvalMasterDataDetailLimit = 200
	// approvalMasterDataDetailMaxPage keeps (page-1)*size far from overflow.
	approvalMasterDataDetailMaxPage = 1_000_000
)

// MasterDataDetail handles GET /{id}/master-data: what an approval request for
// a master-data change actually changes (payload + "before" snapshot, or the
// rows of an import batch). Approver authority comes from the approval
// hierarchy, not from a role, so the master-data admin endpoints (ADMIN/
// ADMIN_PARAM only) cannot serve a checker; this one is authorized per request:
// only the request's maker or an approver with a PENDING step on it may read it.
func (h *ApprovalHandler) MasterDataDetail(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	id, err := parseApprovalRequestID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	req, err := h.reader.GetRequest(r.Context(), id)
	if err != nil || h.mdDetail == nil || req.DocumentType != "master_data" {
		writeError(w, http.StatusNotFound, "not_found", "Approval request master data tidak ditemukan")
		return
	}

	allowed := req.MakerID == authCtx.UserID
	if !allowed {
		items, err := h.reader.ListInboxForApprover(r.Context(), authCtx.UserID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
			return
		}
		for _, item := range items {
			if item.RequestID == id {
				allowed = true
				break
			}
		}
	}
	if !allowed {
		writeForbidden(w, "Anda tidak berhak melihat permintaan ini")
		return
	}

	head, err := h.mdDetail.GetByID(r.Context(), req.DocumentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Perubahan master data tidak ditemukan")
		return
	}
	// Parsed after authorization so paging never changes who may read.
	page, ok := queryInt(w, r.URL.Query().Get("page"), "page", 1, 1, approvalMasterDataDetailMaxPage)
	if !ok {
		return
	}
	pageSize, ok := queryInt(w, r.URL.Query().Get("page_size"), "page_size",
		approvalMasterDataDetailLimit, 1, approvalMasterDataDetailLimit)
	if !ok {
		return
	}

	changes := []db.MasterDataChangeRequest{head}
	var batch map[string]any
	if head.BatchID != nil {
		if changes, err = h.mdDetail.ListByBatch(r.Context(), *head.BatchID); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
			return
		}
		if h.mdBatch != nil {
			b, err := h.mdBatch.GetBatch(r.Context(), *head.BatchID)
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusNotFound, "not_found", "Batch impor tidak ditemukan")
				return
			}
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
				return
			}
			batch = map[string]any{"id": b.ID, "entity": b.Entity, "file_name": b.FileName,
				"maker_name": b.MakerName, "row_count": b.RowCount}
		}
	}

	counts := map[string]int{}
	for _, c := range changes {
		counts[c.Op]++
	}
	total := len(changes)
	shown := changes
	if head.BatchID != nil { // a single change shows its one row on any page
		// Sliced directly rather than exporting service.pageSlice: it is two lines.
		start := min((page-1)*pageSize, total)
		shown = changes[start:min(start+pageSize, total)]
	}
	out := make([]map[string]any, len(shown))
	for i, c := range shown {
		out[i] = masterDataChangeToResponse(c)
	}
	resp := map[string]any{
		"request_id": req.ID, "status": req.Status, "batch_id": head.BatchID,
		"total": total, "counts": counts, "truncated": head.BatchID != nil && total > page*pageSize,
		"changes": out, "page": page, "page_size": pageSize,
	}
	if batch != nil {
		resp["batch"] = batch
	}
	writeJSON(w, http.StatusOK, resp)
}
