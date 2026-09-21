package handler

import (
	"context"
	"net/http"

	"github.com/cimb-niaga/cms/pkg/middleware"
)

// MasterDataApplyRetrier re-runs the apply step of an approved master-data change.
type MasterDataApplyRetrier interface {
	RetryApply(ctx context.Context, changeID, actorID int64, actorIP string) error
}

// WithApplyRetry enables POST /{id}/retry-apply.
func (h *ApprovalHandler) WithApplyRetry(r MasterDataApplyRetrier) *ApprovalHandler {
	h.retrier = r
	return h
}

// RetryApply handles POST /{id}/retry-apply: re-applies a master-data change that
// was approved but failed to apply. 409 when the change is not awaiting apply.
func (h *ApprovalHandler) RetryApply(w http.ResponseWriter, r *http.Request) {
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
	if h.retrier == nil {
		writeError(w, http.StatusNotFound, "not_found", "Tidak tersedia")
		return
	}
	req, err := h.reader.GetRequest(r.Context(), id)
	if err != nil || req.DocumentType != "master_data" || req.Status != "approved" {
		writeError(w, http.StatusConflict, "conflict", "Perubahan tidak menunggu penerapan")
		return
	}
	if err := h.retrier.RetryApply(r.Context(), req.DocumentID, authCtx.UserID, extractClientIP(r)); err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toApprovalRequestResponse(req))
}
