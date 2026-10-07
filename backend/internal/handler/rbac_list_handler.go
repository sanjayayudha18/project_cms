package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// RbacReader is the read access the RBAC settings menu's list endpoints need.
type RbacReader interface {
	ListUserHierarchy(ctx context.Context) ([]db.ListUserHierarchyRow, error)
	ListDelegations(ctx context.Context) ([]db.ApprovalDelegation, error)
	ListLeaves(ctx context.Context) ([]db.UserLeave, error)
	ListApprovalPolicies(ctx context.Context) ([]db.ApprovalPolicy, error)
}

// ApprovalPolicyWriter is the write access the policy create/edit endpoints need.
type ApprovalPolicyWriter interface {
	Create(ctx context.Context, arg db.CreateApprovalPolicyParams) (db.ApprovalPolicy, error)
	Get(ctx context.Context, id int64) (*db.ApprovalPolicy, error)
	Update(ctx context.Context, arg db.UpdateApprovalPolicyParams) (db.ApprovalPolicy, error)
}

// RbacListHandler serves the RBAC settings menu's list reads (replica-backed)
// and approval-policy create/edit (primary-backed, audited). Mount behind
// RequireRoles(ADMIN, ADMIN_PARAM, APPACCESS) alongside AdminApprovalHandler.
type RbacListHandler struct {
	reader      RbacReader
	policyStore ApprovalPolicyWriter
	audit       AdminAuditWriter
}

// NewRbacListHandler creates a new RbacListHandler.
func NewRbacListHandler(reader RbacReader, policyStore ApprovalPolicyWriter, auditWriter AdminAuditWriter) *RbacListHandler {
	return &RbacListHandler{reader: reader, policyStore: policyStore, audit: auditWriter}
}

// Routes returns a chi.Router with all RBAC list + policy write endpoints mounted.
func (h *RbacListHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/users/hierarchy", h.ListUserHierarchy)
	r.Get("/delegations", h.ListDelegations)
	r.Get("/leaves", h.ListLeaves)
	r.Get("/policies", h.ListPolicies)
	r.Post("/policies", h.CreatePolicy)
	r.Put("/policies/{id}", h.UpdatePolicy)
	return r
}

type userHierarchyResponse struct {
	ID            int64   `json:"id"`
	Username      string  `json:"username"`
	FullName      string  `json:"full_name"`
	SupervisorID  *int64  `json:"supervisor_id"`
	ApprovalLevel *int32  `json:"approval_level"`
	Role          string  `json:"role"`
	AuthSource    string  `json:"auth_source"`
	VendorID      *int64  `json:"vendor_id"`
	VendorName    *string `json:"vendor_name"`
}

// ListUserHierarchy handles GET /users/hierarchy.
func (h *RbacListHandler) ListUserHierarchy(w http.ResponseWriter, r *http.Request) {
	rows, err := h.reader.ListUserHierarchy(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	out := make([]userHierarchyResponse, len(rows))
	for i, row := range rows {
		out[i] = userHierarchyResponse{
			ID:            row.ID,
			Username:      row.Username,
			FullName:      row.FullName,
			SupervisorID:  row.SupervisorID,
			ApprovalLevel: row.ApprovalLevel,
			Role:          row.Role,
			AuthSource:    row.AuthSource,
			VendorID:      row.VendorID,
			VendorName:    row.VendorName,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

// ListDelegations handles GET /delegations.
func (h *RbacListHandler) ListDelegations(w http.ResponseWriter, r *http.Request) {
	rows, err := h.reader.ListDelegations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	out := make([]delegationResponse, len(rows))
	for i, row := range rows {
		out[i] = toDelegationResponse(row)
	}
	writeJSON(w, http.StatusOK, map[string]any{"delegations": out})
}

type leaveResponse struct {
	ID      int64   `json:"id"`
	UserID  int64   `json:"user_id"`
	StartAt string  `json:"start_at"`
	EndAt   string  `json:"end_at"`
	Reason  *string `json:"reason"`
}

func toLeaveResponse(l db.UserLeave) leaveResponse {
	return leaveResponse{
		ID:      l.ID,
		UserID:  l.UserID,
		StartAt: l.StartAt.Time.Format(time.RFC3339),
		EndAt:   l.EndAt.Time.Format(time.RFC3339),
		Reason:  l.Reason,
	}
}

// ListLeaves handles GET /leaves.
func (h *RbacListHandler) ListLeaves(w http.ResponseWriter, r *http.Request) {
	rows, err := h.reader.ListLeaves(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	out := make([]leaveResponse, len(rows))
	for i, row := range rows {
		out[i] = toLeaveResponse(row)
	}
	writeJSON(w, http.StatusOK, map[string]any{"leaves": out})
}

type policyResponse struct {
	ID            int64  `json:"id"`
	DocumentType  string `json:"document_type"`
	MinAmount     string `json:"min_amount"`
	MaxAmount     string `json:"max_amount"`
	RequiredLevel int32  `json:"required_level"`
}

func toPolicyResponse(p db.ApprovalPolicy) policyResponse {
	minAmount, _ := p.MinAmount.Value()
	maxAmount, _ := p.MaxAmount.Value()
	return policyResponse{
		ID:            p.ID,
		DocumentType:  p.DocumentType,
		MinAmount:     fmt.Sprintf("%v", minAmount),
		MaxAmount:     fmt.Sprintf("%v", maxAmount),
		RequiredLevel: p.RequiredLevel,
	}
}

// ListPolicies handles GET /policies.
func (h *RbacListHandler) ListPolicies(w http.ResponseWriter, r *http.Request) {
	rows, err := h.reader.ListApprovalPolicies(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	out := make([]policyResponse, len(rows))
	for i, row := range rows {
		out[i] = toPolicyResponse(row)
	}
	writeJSON(w, http.StatusOK, map[string]any{"policies": out})
}

type policyRequest struct {
	DocumentType  string `json:"document_type"`
	MinAmount     string `json:"min_amount"`
	MaxAmount     string `json:"max_amount"`
	RequiredLevel int32  `json:"required_level"`
}

// validatePolicyRequest checks the fields shared by create and update:
// document_type non-empty, min_amount < max_amount (matches the
// approval_policies_amount_range_chk CHECK constraint, migration 022 — not
// "<=", an equal range is rejected by the DB), and a positive required_level.
// Returns the parsed amounts on success, or a Bahasa Indonesia message on failure.
func validatePolicyRequest(req policyRequest) (minAmount, maxAmount pgtype.Numeric, errMsg string) {
	if strings.TrimSpace(req.DocumentType) == "" {
		return minAmount, maxAmount, "document_type wajib diisi"
	}
	if req.RequiredLevel <= 0 {
		return minAmount, maxAmount, "required_level harus lebih besar dari 0"
	}
	minRat, ok := new(big.Rat).SetString(req.MinAmount)
	if !ok {
		return minAmount, maxAmount, "min_amount tidak valid"
	}
	maxRat, ok := new(big.Rat).SetString(req.MaxAmount)
	if !ok {
		return minAmount, maxAmount, "max_amount tidak valid"
	}
	if minRat.Cmp(maxRat) >= 0 {
		return minAmount, maxAmount, "min_amount harus lebih kecil dari max_amount"
	}
	if err := minAmount.Scan(req.MinAmount); err != nil {
		return minAmount, maxAmount, "min_amount tidak valid"
	}
	if err := maxAmount.Scan(req.MaxAmount); err != nil {
		return minAmount, maxAmount, "max_amount tidak valid"
	}
	return minAmount, maxAmount, ""
}

// CreatePolicy handles POST /policies.
func (h *RbacListHandler) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	var req policyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	minAmount, maxAmount, errMsg := validatePolicyRequest(req)
	if errMsg != "" {
		writeError(w, http.StatusBadRequest, "bad_request", errMsg)
		return
	}

	result, err := h.policyStore.Create(r.Context(), db.CreateApprovalPolicyParams{
		DocumentType:  req.DocumentType,
		MinAmount:     minAmount,
		MaxAmount:     maxAmount,
		RequiredLevel: req.RequiredLevel,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}

	_ = h.audit.Write(r.Context(), audit.Entry{
		ActorID:    authCtx.UserID,
		Action:     "admin_create_policy",
		EntityType: "approval_policy",
		EntityID:   result.ID,
		After:      toPolicyResponse(result),
		IP:         extractClientIP(r),
	})

	writeJSON(w, http.StatusCreated, toPolicyResponse(result))
}

// UpdatePolicy handles PUT /policies/{id}.
func (h *RbacListHandler) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
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

	var req policyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	minAmount, maxAmount, errMsg := validatePolicyRequest(req)
	if errMsg != "" {
		writeError(w, http.StatusBadRequest, "bad_request", errMsg)
		return
	}

	before, err := h.policyStore.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	if before == nil {
		writeError(w, http.StatusNotFound, "not_found", "Kebijakan approval tidak ditemukan")
		return
	}

	result, err := h.policyStore.Update(r.Context(), db.UpdateApprovalPolicyParams{
		ID:            id,
		DocumentType:  req.DocumentType,
		MinAmount:     minAmount,
		MaxAmount:     maxAmount,
		RequiredLevel: req.RequiredLevel,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}

	_ = h.audit.Write(r.Context(), audit.Entry{
		ActorID:    authCtx.UserID,
		Action:     "admin_update_policy",
		EntityType: "approval_policy",
		EntityID:   id,
		Before:     toPolicyResponse(*before),
		After:      toPolicyResponse(result),
		IP:         extractClientIP(r),
	})

	writeJSON(w, http.StatusOK, toPolicyResponse(result))
}
