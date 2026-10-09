package service

import (
	"context"
	"fmt"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/notification"
)

// createVaultPlans creates the missing draft vault plans (one per request x
// ACM area, cit-acm-plan FR4.1/FR7.3), audits each, and notifies the ACM-USER
// members of each new plan's area (FR6.3/N1) -- all inside the caller's tx.
// requestID nil = every request currently in vault_assignment. ATMs whose
// replenish branch has no area get no plan; they surface in the ADMIN warning
// list (FR7.4) until a branch assignment calls this again.
func createVaultPlans(ctx context.Context, q *db.Queries, aw *audit.Writer, notifier Notifier, actor Actor, requestID *int64) error {
	created, err := q.CreateMissingVaultPlans(ctx, requestID)
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
			Body:       "Ada Vendor Request yang menunggu penetapan branch vault untuk ATM di area Anda.",
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
