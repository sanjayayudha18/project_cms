package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/approval"
	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// AdminStore is the subset of approval.AdminStore the handler needs.
type AdminStore interface {
	SetUserHierarchy(ctx context.Context, userID int64, supervisorID *int64, approvalLevel *int32) (db.UpdateUserHierarchyRow, error)
	CreateDelegation(ctx context.Context, fromUserID, toUserID int64, startAt, endAt time.Time, reason *string) (db.ApprovalDelegation, error)
	RevokeDelegation(ctx context.Context, id int64, revokedAt time.Time) (*db.ApprovalDelegation, error)
	CreateLeave(ctx context.Context, userID int64, startAt, endAt time.Time, reason *string) (db.UserLeave, error)
}

// AdminAuditWriter is the one method the admin handler needs from audit.Writer.
type AdminAuditWriter interface {
	Write(ctx context.Context, entry audit.Entry) error
}

// AdminApprovalHandler handles admin-only endpoints for the reporting-line
// hierarchy, delegations, and leaves (RBAC-Setup Task 8). Every action writes
// audit_logs. Mount behind RequireRoles(ADMIN, ADMIN_PARAM).
type AdminApprovalHandler struct {
	store AdminStore
	audit AdminAuditWriter
}

// NewAdminApprovalHandler creates a new AdminApprovalHandler.
func NewAdminApprovalHandler(store AdminStore, auditWriter AdminAuditWriter) *AdminApprovalHandler {
	return &AdminApprovalHandler{store: store, audit: auditWriter}
}

// Routes returns a chi.Router with all admin approval-config endpoints mounted.
func (h *AdminApprovalHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Put("/users/{id}/hierarchy", h.SetHierarchy)
	r.Post("/delegations", h.CreateDelegation)
	r.Delete("/delegations/{id}", h.RevokeDelegation)
	r.Post("/leaves", h.CreateLeave)
	return r
}

type setHierarchyRequest struct {
	SupervisorID  *int64 `json:"supervisor_id"`
	ApprovalLevel *int32 `json:"approval_level"`
}

// SetHierarchy handles PUT /users/{id}/hierarchy.
func (h *AdminApprovalHandler) SetHierarchy(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	userID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var req setHierarchyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	if req.SupervisorID != nil && *req.SupervisorID == userID {
		writeError(w, http.StatusBadRequest, "bad_request", "supervisor_id tidak boleh sama dengan user itu sendiri")
		return
	}

	result, err := h.store.SetUserHierarchy(r.Context(), userID, req.SupervisorID, req.ApprovalLevel)
	if err != nil {
		h.handleError(w, err)
		return
	}

	_ = h.audit.Write(r.Context(), audit.Entry{
		ActorID:    authCtx.UserID,
		Action:     "admin_set_hierarchy",
		EntityType: "user",
		EntityID:   userID,
		After:      req,
		IP:         extractClientIP(r),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"id":             result.ID,
		"supervisor_id":  result.SupervisorID,
		"approval_level": result.ApprovalLevel,
	})
}

type createDelegationRequest struct {
	FromUserID int64   `json:"from_user_id"`
	ToUserID   int64   `json:"to_user_id"`
	StartAt    string  `json:"start_at"` // RFC3339
	EndAt      string  `json:"end_at"`   // RFC3339
	Reason     *string `json:"reason"`
}

// CreateDelegation handles POST /delegations.
func (h *AdminApprovalHandler) CreateDelegation(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	var req createDelegationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FromUserID <= 0 || req.ToUserID <= 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "from_user_id dan to_user_id wajib diisi")
		return
	}
	startAt, endAt, err := parseRange(req.StartAt, req.EndAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	result, err := h.store.CreateDelegation(r.Context(), req.FromUserID, req.ToUserID, startAt, endAt, req.Reason)
	if err != nil {
		h.handleError(w, err)
		return
	}

	_ = h.audit.Write(r.Context(), audit.Entry{
		ActorID:    authCtx.UserID,
		Action:     "admin_create_delegation",
		EntityType: "approval_delegation",
		EntityID:   result.ID,
		After:      req,
		IP:         extractClientIP(r),
	})

	writeJSON(w, http.StatusCreated, toDelegationResponse(result))
}

// RevokeDelegation handles DELETE /delegations/{id} ("cabut").
func (h *AdminApprovalHandler) RevokeDelegation(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	result, err := h.store.RevokeDelegation(r.Context(), id, time.Now())
	if err != nil {
		h.handleError(w, err)
		return
	}
	if result == nil {
		writeError(w, http.StatusNotFound, "not_found", "Delegasi tidak ditemukan atau sudah berakhir")
		return
	}

	_ = h.audit.Write(r.Context(), audit.Entry{
		ActorID:    authCtx.UserID,
		Action:     "admin_revoke_delegation",
		EntityType: "approval_delegation",
		EntityID:   id,
		After:      toDelegationResponse(*result),
		IP:         extractClientIP(r),
	})

	writeJSON(w, http.StatusOK, toDelegationResponse(*result))
}

type createLeaveRequest struct {
	UserID  int64   `json:"user_id"`
	StartAt string  `json:"start_at"` // RFC3339
	EndAt   string  `json:"end_at"`   // RFC3339
	Reason  *string `json:"reason"`
}

// CreateLeave handles POST /leaves.
func (h *AdminApprovalHandler) CreateLeave(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	var req createLeaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID <= 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "user_id wajib diisi")
		return
	}
	startAt, endAt, err := parseRange(req.StartAt, req.EndAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	result, err := h.store.CreateLeave(r.Context(), req.UserID, startAt, endAt, req.Reason)
	if err != nil {
		h.handleError(w, err)
		return
	}

	_ = h.audit.Write(r.Context(), audit.Entry{
		ActorID:    authCtx.UserID,
		Action:     "admin_create_leave",
		EntityType: "user_leave",
		EntityID:   result.ID,
		After:      req,
		IP:         extractClientIP(r),
	})

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":       result.ID,
		"user_id":  result.UserID,
		"start_at": req.StartAt,
		"end_at":   req.EndAt,
	})
}

func parsePathID(r *http.Request, param string) (int64, error) {
	raw := chi.URLParam(r, param)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%s tidak valid", param)
	}
	return id, nil
}

// parseOptionalIDParam reads an optional positive-integer query param
// (e.g. branch_id) -- nil, nil if absent or blank.
func parseOptionalIDParam(q url.Values, name string) (*int64, error) {
	raw := q.Get(name)
	if raw == "" {
		return nil, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("%s tidak valid", name)
	}
	return &id, nil
}

func parseRange(startRaw, endRaw string) (time.Time, time.Time, error) {
	start, err := time.Parse(time.RFC3339, startRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("start_at harus format RFC3339")
	}
	end, err := time.Parse(time.RFC3339, endRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("end_at harus format RFC3339")
	}
	if !start.Before(end) {
		return time.Time{}, time.Time{}, fmt.Errorf("start_at harus sebelum end_at")
	}
	return start, end, nil
}

func (h *AdminApprovalHandler) handleError(w http.ResponseWriter, err error) {
	if errors.Is(err, approval.ErrDelegationOverlap) {
		writeError(w, http.StatusConflict, "conflict", "Rentang delegasi tumpang tindih dengan delegasi lain untuk user ini")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
}

type delegationResponse struct {
	ID         int64   `json:"id"`
	FromUserID int64   `json:"from_user_id"`
	ToUserID   int64   `json:"to_user_id"`
	StartAt    string  `json:"start_at"`
	EndAt      string  `json:"end_at"`
	Reason     *string `json:"reason"`
}

func toDelegationResponse(d db.ApprovalDelegation) delegationResponse {
	return delegationResponse{
		ID:         d.ID,
		FromUserID: d.FromUserID,
		ToUserID:   d.ToUserID,
		StartAt:    d.StartAt.Time.Format(time.RFC3339),
		EndAt:      d.EndAt.Time.Format(time.RFC3339),
		Reason:     d.Reason,
	}
}
