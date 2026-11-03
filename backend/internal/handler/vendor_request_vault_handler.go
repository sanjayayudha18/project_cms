package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/cimb-niaga/cms/backend/internal/service"
)

// VendorRequestVaultReviewer is the slice of service.VaultPlanService behind
// the ATM-SPV review routes of a Vendor Request (cit-acm-plan FR5).
type VendorRequestVaultReviewer interface {
	ForRequest(ctx context.Context, requestID int64) (*service.VaultPlanDetail, error)
	ReviewRequest(ctx context.Context, actor service.Actor, requestID int64, approve bool, reason string) error
}

// WithVaultReview mounts /{id}/vault-assignments|vault-approve|vault-reject.
func (h *VendorRequestHandler) WithVaultReview(v VendorRequestVaultReviewer) *VendorRequestHandler {
	h.vault = v
	return h
}

// VaultAssignments handles GET /{id}/vault-assignments (cit-vault-select
// FR9.2): {plan: <same shape as GET /vault-plans/{id}> | null}.
func (h *VendorRequestHandler) VaultAssignments(w http.ResponseWriter, r *http.Request) {
	id, err := parseVendorRequestID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	d, err := h.vault.ForRequest(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	var plan map[string]any
	if d != nil {
		plan = vaultPlanJSON(d)
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan})
}

// VaultApprove handles POST /{id}/vault-approve: vault_review -> ready (FR5.2).
func (h *VendorRequestHandler) VaultApprove(w http.ResponseWriter, r *http.Request) {
	h.vaultReview(w, r, true, "")
}

// VaultReject handles POST /{id}/vault-reject {vault_rejection_reason}: back to vault_assignment (FR5.2).
func (h *VendorRequestHandler) VaultReject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Reason string `json:"vault_rejection_reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "body tidak valid")
		return
	}
	h.vaultReview(w, r, false, body.Reason)
}

func (h *VendorRequestHandler) vaultReview(w http.ResponseWriter, r *http.Request, approve bool, reason string) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	id, err := parseVendorRequestID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := h.vault.ReviewRequest(r.Context(), actor, id, approve, reason); err != nil {
		h.handleError(w, err)
		return
	}
	detail, err := h.service.Get(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDetailResponse(detail))
}
