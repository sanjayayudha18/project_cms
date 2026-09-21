package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/approval"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// ApprovalOrchestrator is the subset of approval.Orchestrator the handler needs.
type ApprovalOrchestrator interface {
	SubmitForApproval(ctx context.Context, makerID int64, documentType string, documentID int64, amount pgtype.Numeric, actorIP string) (db.ApprovalRequest, bool, error)
	Approve(ctx context.Context, requestID, actorID int64, actorIP string) (db.ApprovalRequest, error)
	Reject(ctx context.Context, requestID, actorID int64, actorIP string) (db.ApprovalRequest, error)
}

// ApprovalReader is the read access the handler needs beyond the orchestrator.
type ApprovalReader interface {
	GetRequest(ctx context.Context, id int64) (db.ApprovalRequest, error)
	ListInboxForApprover(ctx context.Context, approverID int64) ([]approval.InboxItem, error)
}

// ApprovalHandler handles the maker-checker approval endpoints.
type ApprovalHandler struct {
	orchestrator ApprovalOrchestrator
	reader       ApprovalReader
	mdDetail     MasterDataApprovalDetailReader // set by WithMasterDataDetail; nil = endpoint answers 404
	retrier      MasterDataApplyRetrier         // set by WithApplyRetry; nil = endpoint answers 404
}

// NewApprovalHandler creates a new ApprovalHandler.
func NewApprovalHandler(orchestrator ApprovalOrchestrator, reader ApprovalReader) *ApprovalHandler {
	return &ApprovalHandler{orchestrator: orchestrator, reader: reader}
}

// Routes returns a chi.Router with all approval endpoints mounted.
func (h *ApprovalHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/submit", h.Submit)
	r.Get("/inbox", h.Inbox)
	r.Post("/{id}/approve", h.Approve)
	r.Post("/{id}/reject", h.Reject)
	r.Post("/{id}/retry-apply", h.RetryApply)
	r.Get("/{id}", h.Get)
	r.Get("/{id}/master-data", h.MasterDataDetail)
	return r
}

type submitRequest struct {
	DocumentType string `json:"document_type"`
	DocumentID   int64  `json:"document_id"`
	Amount       string `json:"amount"`
}

// Submit handles POST /submit. Idempotent per (document_type, document_id):
// a repeat submit for the same document returns 409, not a new request.
func (h *ApprovalHandler) Submit(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	var req submitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DocumentType == "" || req.DocumentID <= 0 || req.Amount == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "document_type, document_id, dan amount wajib diisi")
		return
	}

	var amount pgtype.Numeric
	if err := amount.Scan(req.Amount); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "amount tidak valid")
		return
	}

	result, created, err := h.orchestrator.SubmitForApproval(r.Context(), authCtx.UserID, req.DocumentType, req.DocumentID, amount, extractClientIP(r))
	if err != nil {
		h.handleError(w, err)
		return
	}
	if !created {
		writeError(w, http.StatusConflict, "already_submitted", "Dokumen ini sudah pernah diajukan untuk approval")
		return
	}

	writeJSON(w, http.StatusCreated, toApprovalRequestResponse(result))
}

// Approve handles POST /{id}/approve.
func (h *ApprovalHandler) Approve(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.orchestrator.Approve(r.Context(), id, authCtx.UserID, extractClientIP(r))
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toApprovalRequestResponse(result))
}

// Reject handles POST /{id}/reject.
func (h *ApprovalHandler) Reject(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.orchestrator.Reject(r.Context(), id, authCtx.UserID, extractClientIP(r))
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toApprovalRequestResponse(result))
}

// Get handles GET /{id}.
func (h *ApprovalHandler) Get(w http.ResponseWriter, r *http.Request) {
	if _, ok := middleware.GetAuthContext(r.Context()); !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	id, err := parseApprovalRequestID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	result, err := h.reader.GetRequest(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Approval request tidak ditemukan")
		return
	}
	writeJSON(w, http.StatusOK, toApprovalRequestResponse(result))
}

// Inbox handles GET /inbox: the caller's own pending steps.
func (h *ApprovalHandler) Inbox(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	items, err := h.reader.ListInboxForApprover(r.Context(), authCtx.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	writeJSON(w, http.StatusOK, toInboxResponse(items))
}

func parseApprovalRequestID(r *http.Request) (int64, error) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("id tidak valid")
	}
	return id, nil
}

// handleError maps approval package sentinel errors to HTTP responses.
func (h *ApprovalHandler) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, approval.ErrNotAuthorized):
		writeForbidden(w, "Anda tidak berhak melakukan aksi ini")
	case errors.Is(err, approval.ErrRequestNotPending):
		writeError(w, http.StatusConflict, "conflict", "Approval request tidak dalam status pending")
	case errors.Is(err, approval.ErrPolicyNotFound):
		writeError(w, http.StatusBadRequest, "bad_request", "Tidak ada kebijakan approval untuk jenis dokumen ini")
	// Master-data apply-on-approve failures a checker can act on (T3.5): the
	// approval decision is already recorded; the change itself could not land.
	case errors.Is(err, service.ErrATMAssignmentOverlap):
		writeError(w, http.StatusConflict, "conflict", service.ErrATMAssignmentOverlap.Error())
	case errors.Is(err, service.ErrVendorCodeConflict):
		writeError(w, http.StatusConflict, "conflict", service.ErrVendorCodeConflict.Error())
	case errors.Is(err, service.ErrATMTerminalIDConflict):
		writeError(w, http.StatusConflict, "conflict", service.ErrATMTerminalIDConflict.Error())
	case errors.Is(err, service.ErrATMAssignmentDuplicate):
		writeError(w, http.StatusConflict, "conflict", service.ErrATMAssignmentDuplicate.Error())
	case errors.Is(err, service.ErrVendorVaultCodeConflict):
		writeError(w, http.StatusConflict, "conflict", service.ErrVendorVaultCodeConflict.Error())
	case errors.Is(err, service.ErrATMNotFound), errors.Is(err, service.ErrATMInvalidReference):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, service.ErrMasterDataForbidden):
		writeForbidden(w, "Anda tidak berhak melakukan aksi ini")
	case errors.Is(err, service.ErrMasterDataNotRetryable):
		writeError(w, http.StatusConflict, "conflict", "Perubahan tidak menunggu penerapan")
	case errors.Is(err, service.ErrMasterDataChangeStale):
		writeError(w, http.StatusConflict, "conflict", service.ErrMasterDataChangeStale.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
	}
}

// -- JSON response shapes --------------------------------------------------

type approvalRequestResponse struct {
	ID            int64  `json:"id"`
	MakerID       int64  `json:"maker_id"`
	DocumentType  string `json:"document_type"`
	DocumentID    int64  `json:"document_id"`
	Amount        string `json:"amount"`
	RequiredLevel int32  `json:"required_level"`
	Status        string `json:"status"`
}

func toApprovalRequestResponse(r db.ApprovalRequest) approvalRequestResponse {
	amount, _ := r.Amount.Value()
	return approvalRequestResponse{
		ID:            r.ID,
		MakerID:       r.MakerID,
		DocumentType:  r.DocumentType,
		DocumentID:    r.DocumentID,
		Amount:        fmt.Sprintf("%v", amount),
		RequiredLevel: r.RequiredLevel,
		Status:        r.Status,
	}
}

type inboxItemResponse struct {
	StepID       int64  `json:"step_id"`
	RequestID    int64  `json:"request_id"`
	StepLevel    int32  `json:"step_level"`
	DocumentType string `json:"document_type"`
	DocumentID   int64  `json:"document_id"`
	Amount       string `json:"amount"`
	MakerID      int64  `json:"maker_id"`
}

func toInboxResponse(items []approval.InboxItem) []inboxItemResponse {
	out := make([]inboxItemResponse, len(items))
	for i, item := range items {
		amount, _ := item.Amount.Value()
		out[i] = inboxItemResponse{
			StepID:       item.StepID,
			RequestID:    item.RequestID,
			StepLevel:    item.StepLevel,
			DocumentType: item.DocumentType,
			DocumentID:   item.DocumentID,
			Amount:       fmt.Sprintf("%v", amount),
			MakerID:      item.MakerID,
		}
	}
	return out
}
