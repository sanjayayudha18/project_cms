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
	ForRequest(ctx context.Context, requestID int64) (*service.RequestVaultReview, error)
	ReviewRequest(ctx context.Context, actor service.Actor, requestID int64, approve bool, reason string) error
}

// WithVaultReview mounts /{id}/vault-assignments|vault-approve|vault-reject.
func (h *VendorRequestHandler) WithVaultReview(v VendorRequestVaultReviewer) *VendorRequestHandler {
	h.vault = v
	return h
}

// VaultAssignments handles GET /{id}/vault-assignments (FR5.1).
func (h *VendorRequestHandler) VaultAssignments(w http.ResponseWriter, r *http.Request) {
	id, err := parseVendorRequestID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	review, err := h.vault.ForRequest(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	asg := make([]map[string]any, len(review.Assignments))
	for i, a := range review.Assignments {
		asg[i] = map[string]any{
			"vault_plan_id": a.VaultPlanID, "terminal_id": a.TerminalID,
			"replenish_branch": map[string]any{"id": a.ReplenishBranchID, "code": a.ReplenishBranchCode},
			"vault_branch":     map[string]any{"id": a.VaultBranchID, "code": a.VaultBranchCode, "name": a.VaultBranchName, "vendor_name": a.VaultVendorName},
			"tier":             a.Tier, "is_urgent": a.IsUrgent, "urgent_reason": a.UrgentReason,
			"saldo_snapshot": json.RawMessage(orNull(a.SaldoSnapshot)), "capacity_snapshot": json.RawMessage(orNull(a.CapacitySnapshot)),
			"capacity_warning": a.CapacityWarning,
		}
	}
	plans := make([]map[string]any, len(review.Plans))
	for i, p := range review.Plans {
		plans[i] = map[string]any{"id": p.ID, "acm_area_id": p.AcmAreaID, "acm_area_name": p.AcmAreaName, "status": p.Status, "rejection_reason": p.RejectionReason}
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": plans, "assignments": asg})
}

func orNull(b []byte) []byte {
	if len(b) == 0 {
		return []byte("null")
	}
	return b
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
