package service

import (
	"context"
	"fmt"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/notification"
)

// createVaultPlans creates the missing draft vault plans (one per request, held
// by the area of the request's region -- cit-vault-select FR2.1/FR1.3), audits
// each, and notifies the ACM-USER members of the plan's area (FR8.3) -- all
// inside the caller's tx. requestID nil = every request currently in
// vault_assignment. A request whose region has no area (or no region at all)
// gets no plan; it surfaces in the ADMIN warning list (FR1.4) until a region
// mapping calls this again.
func createVaultPlans(ctx context.Context, q *db.Queries, aw *audit.Writer, notifier Notifier, actor Actor, requestID *int64) error {
	created, err := q.CreateMissingVaultPlansByRegion(ctx, db.CreateMissingVaultPlansByRegionParams{RequestID: requestID})
	if err != nil {
		return fmt.Errorf("create vault plans: %w", err)
	}
	for _, p := range created {
		if err := aw.Write(ctx, audit.Entry{
			ActorID: actor.UserID, Action: "vault_plan_created", EntityType: "vendor_request_vault_plan", EntityID: p.ID,
			After: map[string]any{"vendor_request_id": p.VendorRequestID, "acm_area_id": p.AcmAreaID, "status": "draft"},
			IP:    actor.IP,
		}); err != nil {
			return fmt.Errorf("write audit log: %w", err)
		}
		if notifier == nil {
			continue
		}
		users, err := q.ListAcmAreaMemberIDsByRole(ctx, db.ListAcmAreaMemberIDsByRoleParams{AreaIds: []int64{p.AcmAreaID}, Role: "ACM-USER"})
		if err != nil {
			return fmt.Errorf("list acm area members: %w", err)
		}
		if len(users) == 0 {
			continue
		}
		planID := p.ID
		msg := notification.Message{
			Type:       "vault_plan.assignment_needed",
			Title:      "Penetapan vault diperlukan",
			Body:       "Ada Vendor Request yang menunggu pembagian nominal ke vault di area Anda.",
			Link:       fmt.Sprintf("/cit/vault-plans/%d", planID),
			EntityType: "vendor_request_vault_plan",
			EntityID:   &planID,
			Email:      true,
		}
		if err := notifier.Send(ctx, q, msg, notification.Recipients{UserIDs: users}); err != nil {
			return fmt.Errorf("notify vault plan %d: %w", planID, err)
		}
	}
	return nil
}

// resetVaultPlansForReapproval: a request approved again after vendor-return
// -> revise -> edit (cit-send-vendor FR6.2) still has its live plan. It
// reopens as draft (allocations kept as ACM's starting point; the total is
// re-checked at submit) and its ACM-USERs are told to review it. No-op on a
// first approval.
func resetVaultPlansForReapproval(ctx context.Context, q *db.Queries, aw *audit.Writer, notifier Notifier, actor Actor, req db.VendorRequest) error {
	plans, err := q.ListVaultPlansForRequest(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("list plans of request %d: %w", req.ID, err)
	}
	if err := q.ResetVaultPlansToDraft(ctx, req.ID); err != nil {
		return fmt.Errorf("reopen vault plans of request %d: %w", req.ID, err)
	}
	for _, p := range plans {
		if p.Status != planDraft {
			if err := aw.Write(ctx, audit.Entry{
				ActorID: actor.UserID, Action: "vault_plan_reset", EntityType: "vendor_request_vault_plan", EntityID: p.ID,
				Before: map[string]any{"status": p.Status}, After: map[string]any{"status": planDraft, "reason": "request re-approved after edit"},
				IP: actor.IP,
			}); err != nil {
				return fmt.Errorf("write audit log: %w", err)
			}
		}
		if notifier == nil {
			continue
		}
		users, err := q.ListAcmAreaMemberIDsByRole(ctx, db.ListAcmAreaMemberIDsByRoleParams{AreaIds: []int64{p.AcmAreaID}, Role: "ACM-USER"})
		if err != nil {
			return fmt.Errorf("list acm area members: %w", err)
		}
		if len(users) == 0 {
			continue
		}
		planID := p.ID
		if err := notifier.Send(ctx, q, notification.Message{
			Type: "vault_plan.assignment_needed", Title: "Penetapan vault perlu ditinjau ulang",
			Body: fmt.Sprintf("Request %s diedit dan disetujui ulang; tinjau penetapan vault area Anda.", req.RequestNumber),
			Link: fmt.Sprintf("/cit/vault-plans/%d", planID), EntityType: "vendor_request_vault_plan", EntityID: &planID, Email: true,
		}, notification.Recipients{UserIDs: users}); err != nil {
			return fmt.Errorf("notify vault plan %d: %w", planID, err)
		}
	}
	return nil
}
