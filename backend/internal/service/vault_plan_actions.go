package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/notification"
)

// Plan statuses (vendor_request_vault_plans.status).
const (
	planDraft       = "draft"
	planPending     = "pending_acm_approval"
	planAcmApproved = "acm_approved"
)

// planTx is the shared tx -> role check -> lock request then plan -> scope
// check -> fn -> commit flow. Lock order (request, plan) is the same in every
// path so two approvers of different areas serialize on the request (NFR3).
func (s *VaultPlanService) planTx(ctx context.Context, actor Actor, id int64, role string,
	fn func(q *db.Queries, aw *audit.Writer, p db.GetVaultPlanRow, req db.VendorRequest) error) error {
	if !hasRole(actor, role) {
		return ErrNotAuthorized // ADMIN is read-only here (FR6.2)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	p, err := planFor(ctx, q, actor, id)
	if err != nil {
		return err
	}
	req, err := q.GetVendorRequestForUpdate(ctx, p.VendorRequestID)
	if err != nil {
		return fmt.Errorf("lock vendor request %d: %w", p.VendorRequestID, err)
	}
	locked, err := q.LockVaultPlan(ctx, id)
	if err != nil {
		return fmt.Errorf("lock vault plan %d: %w", id, err)
	}
	p.Status, p.SubmittedBy, p.BufferReason = locked.Status, locked.SubmittedBy, locked.BufferReason // fresh after the lock

	if err := fn(q, audit.NewWriter(tx), p, req); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func planAudit(actor Actor, p db.GetVaultPlanRow, action, from, to string, extra map[string]any) audit.Entry {
	after := map[string]any{"state": to}
	for k, v := range extra {
		after[k] = v
	}
	return audit.Entry{ActorID: actor.UserID, Action: action, EntityType: vaultPlanEntity, EntityID: p.ID,
		Before: map[string]any{"state": from}, After: after, IP: actor.IP}
}

// SaveAllocations replaces the plan's vault rows and buffer reason (draft
// only) and refreshes snapshots (FR4.1, FR4.2, FR4.4). The total may still be
// short of the order; Submit checks that.
func (s *VaultPlanService) SaveAllocations(ctx context.Context, actor Actor, id int64, in []AllocationLineInput, bufferReason *string) (*VaultPlanDetail, error) {
	err := s.planTx(ctx, actor, id, "ACM-USER", func(q *db.Queries, aw *audit.Writer, p db.GetVaultPlanRow, req db.VendorRequest) error {
		if p.Status != planDraft || req.Status != "vault_assignment" {
			return ErrInvalidTransition
		}
		lines, err := checkAllocationLines(ctx, q, p, in)
		if err != nil {
			return err
		}
		buffer, err := optionalReason(bufferReason, "buffer_reason")
		if err != nil {
			return err
		}
		if err := q.ClearVaultPlanAllocations(ctx, p.ID); err != nil {
			return fmt.Errorf("clear allocations: %w", err)
		}
		logged := make([]map[string]any, len(lines))
		for i, l := range lines {
			if err := insertAllocation(ctx, q, p, l); err != nil {
				return err
			}
			logged[i] = map[string]any{"vendor_vault_id": l.vault.VendorVaultID, "vault_code": l.vault.VaultCode, "tier": l.tier,
				"amounts": amountsJSON(l.amounts), "is_urgent": l.isUrgent, "urgent_reason": l.urgentReason}
		}
		if err := q.SetVaultPlanBufferReason(ctx, db.SetVaultPlanBufferReasonParams{ID: p.ID, BufferReason: buffer}); err != nil {
			return fmt.Errorf("save buffer reason: %w", err)
		}
		if err := refreshAllocationSnapshots(ctx, q, p); err != nil {
			return err
		}
		return aw.Write(ctx, planAudit(actor, p, "vault_allocations_saved", p.Status, p.Status,
			map[string]any{"allocations": logged, "buffer_reason": buffer}))
	})
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, db.New(s.pool), actor, id)
}

// checkAllocationLines loads the vaults named in `in` that are eligible now and
// runs the FR4.2 rules against the request's vendor + region.
func checkAllocationLines(ctx context.Context, q *db.Queries, p db.GetVaultPlanRow, in []AllocationLineInput) ([]allocationLine, error) {
	ids := make([]int64, 0, len(in)) // never nil: nil means "all vaults" to the query
	for _, l := range in {
		ids = append(ids, l.VendorVaultID)
	}
	rows, err := q.ListVaultCandidateVaults(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list eligible vaults: %w", err)
	}
	eligible := make(map[int64]db.ListVaultCandidateVaultsRow, len(rows))
	for _, r := range rows {
		eligible[r.VendorVaultID] = r
	}
	return validateAllocationLines(in, eligible, requestScope{vendorID: deref(p.VendorID), regionCode: p.RegionCode})
}

func insertAllocation(ctx context.Context, q *db.Queries, p db.GetVaultPlanRow, l allocationLine) error {
	allocID, err := q.InsertVaultAllocation(ctx, db.InsertVaultAllocationParams{VaultPlanID: p.ID, VendorRequestID: p.VendorRequestID,
		VendorVaultID: l.vault.VendorVaultID, VaultBranchID: l.vault.VendorBranchID, Tier: l.tier, IsUrgent: l.isUrgent, UrgentReason: l.urgentReason})
	if err != nil {
		return fmt.Errorf("insert allocation %s: %w", l.vault.VaultCode, err)
	}
	for d, a := range l.amounts {
		if err := q.InsertVaultAllocationAmount(ctx, db.InsertVaultAllocationAmountParams{AllocationID: allocID, Denom: d, Amount: a}); err != nil {
			return fmt.Errorf("insert allocation amount %s %d: %w", l.vault.VaultCode, d, err)
		}
	}
	return nil
}

// optionalReason trims r; empty = nil, else 10-500 chars.
func optionalReason(r *string, field string) (*string, error) {
	t := strings.TrimSpace(deref(r))
	if t == "" {
		return nil, nil
	}
	if n := len([]rune(t)); n < reasonMinLen || n > reasonMaxLen {
		return nil, &ValidationError{Field: field, Message: fmt.Sprintf("wajib %d-%d karakter", reasonMinLen, reasonMaxLen)}
	}
	return &t, nil
}

// storedAllocationLines re-reads the saved rows as inputs, so Submit
// re-validates them against current data (FR4.3).
func storedAllocationLines(ctx context.Context, q *db.Queries, planID int64) ([]AllocationLineInput, error) {
	rows, amounts, err := planAllocations(ctx, q, planID)
	if err != nil {
		return nil, err
	}
	out := make([]AllocationLineInput, len(rows))
	for i, r := range rows {
		out[i] = AllocationLineInput{VendorVaultID: r.VendorVaultID, Amounts: amountsJSON(amounts[r.ID]), IsUrgent: r.IsUrgent, UrgentReason: r.UrgentReason}
	}
	return out, nil
}

// Submit sends the draft plan to the area's ACM-SPV (FR4.3-FR4.5): saved rows
// re-validated, every denom covered (excess needs the buffer reason),
// snapshots refreshed and vault tickets synced (FR6.2), all in one tx.
func (s *VaultPlanService) Submit(ctx context.Context, actor Actor, id int64) (*VaultPlanDetail, error) {
	err := s.planTx(ctx, actor, id, "ACM-USER", func(q *db.Queries, aw *audit.Writer, p db.GetVaultPlanRow, req db.VendorRequest) error {
		if p.Status != planDraft || req.Status != "vault_assignment" {
			return ErrInvalidTransition
		}
		stored, err := storedAllocationLines(ctx, q, p.ID)
		if err != nil {
			return err
		}
		if len(stored) == 0 {
			return allocErr("pilih minimal satu vault")
		}
		lines, err := checkAllocationLines(ctx, q, p, stored)
		if err != nil {
			return err
		}
		order, err := orderTotals(ctx, q, p.VendorRequestID)
		if err != nil {
			return err
		}
		buffer, err := checkCoverage(order, allocationTotals(lines), p.BufferReason)
		if err != nil {
			return err
		}
		if err := q.SetVaultPlanBufferReason(ctx, db.SetVaultPlanBufferReasonParams{ID: p.ID, BufferReason: buffer}); err != nil {
			return fmt.Errorf("save buffer reason: %w", err)
		}
		if err := refreshAllocationSnapshots(ctx, q, p); err != nil {
			return err
		}
		tickets, err := syncVaultTickets(ctx, q, req, lines)
		if err != nil {
			return err
		}
		if err := q.SetVaultPlanStatus(ctx, db.SetVaultPlanStatusParams{Status: planPending, ActorID: &actor.UserID, ID: p.ID}); err != nil {
			return fmt.Errorf("submit vault plan: %w", err)
		}
		if err := aw.Write(ctx, planAudit(actor, p, "submit", p.Status, planPending,
			map[string]any{"vault_tickets": tickets, "buffer_reason": buffer})); err != nil {
			return err
		}
		return s.notify(ctx, q, []int64{p.AcmAreaID}, "ACM-SPV", nil, notification.Message{
			Type: "vault_plan.submitted", Title: "Penetapan vault menunggu persetujuan",
			Body: fmt.Sprintf("Penetapan vault %s (area %s) diajukan dan menunggu persetujuan Anda.", p.RequestNumber, p.AcmAreaName),
			Link: fmt.Sprintf("/cit/vault-plans/%d", p.ID), EntityType: vaultPlanEntity, EntityID: &p.ID, Email: true,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, db.New(s.pool), actor, id)
}

// syncVaultTickets issues / keeps / reactivates / deactivates the request's
// vault tickets to match the allocated vaults (FR6.2).
func syncVaultTickets(ctx context.Context, q *db.Queries, req db.VendorRequest, lines []allocationLine) (vaultTicketAudit, error) {
	existing, err := q.ListVaultTicketsForRequest(ctx, req.ID)
	if err != nil {
		return vaultTicketAudit{}, fmt.Errorf("list vault tickets: %w", err)
	}
	wanted := make([]int64, len(lines))
	for i, l := range lines {
		wanted[i] = l.vault.VendorVaultID
	}
	return applyVaultTickets(ctx, q, req.ID, req.ReplenishDate, planVaultTickets(existing, wanted))
}

// Approve is the ACM-SPV approval of the request's plan (FR4.5). One plan per
// request, so the request moves to vault_review in the same tx (FR4.6, no
// notification by N1).
func (s *VaultPlanService) Approve(ctx context.Context, actor Actor, id int64) (*VaultPlanDetail, error) {
	err := s.planTx(ctx, actor, id, "ACM-SPV", func(q *db.Queries, aw *audit.Writer, p db.GetVaultPlanRow, req db.VendorRequest) error {
		if p.Status != planPending || req.Status != "vault_assignment" {
			return ErrInvalidTransition
		}
		if p.SubmittedBy != nil && *p.SubmittedBy == actor.UserID {
			return ErrSelfApproval
		}
		if err := q.SetVaultPlanStatus(ctx, db.SetVaultPlanStatusParams{Status: planAcmApproved, ActorID: &actor.UserID, ID: p.ID}); err != nil {
			return fmt.Errorf("approve vault plan: %w", err)
		}
		if err := aw.Write(ctx, planAudit(actor, p, "approve", p.Status, planAcmApproved, nil)); err != nil {
			return err
		}
		next, ok := nextState(req.Status, actionVaultReady)
		if !ok {
			return ErrInvalidTransition
		}
		if err := q.SetVendorRequestStatusOnly(ctx, db.SetVendorRequestStatusOnlyParams{Status: next, ID: req.ID}); err != nil {
			return fmt.Errorf("move request to %s: %w", next, err)
		}
		return aw.Write(ctx, audit.Entry{ActorID: actor.UserID, Action: string(actionVaultReady), EntityType: "vendor_request",
			EntityID: req.ID, Before: map[string]string{"state": req.Status}, After: map[string]string{"state": next}, IP: actor.IP})
	})
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, db.New(s.pool), actor, id)
}

// Reject sends an area plan back to draft with a reason (FR4.5) and tells the area's ACM-USERs.
func (s *VaultPlanService) Reject(ctx context.Context, actor Actor, id int64, reason string) (*VaultPlanDetail, error) {
	r, err := checkReason(reason, "rejection_reason")
	if err != nil {
		return nil, err
	}
	err = s.planTx(ctx, actor, id, "ACM-SPV", func(q *db.Queries, aw *audit.Writer, p db.GetVaultPlanRow, req db.VendorRequest) error {
		if p.Status != planPending || req.Status != "vault_assignment" {
			return ErrInvalidTransition
		}
		if p.SubmittedBy != nil && *p.SubmittedBy == actor.UserID {
			return ErrSelfApproval
		}
		if err := q.SetVaultPlanStatus(ctx, db.SetVaultPlanStatusParams{Status: planDraft, ActorID: &actor.UserID, Reason: &r, ID: p.ID}); err != nil {
			return fmt.Errorf("reject vault plan: %w", err)
		}
		if err := aw.Write(ctx, planAudit(actor, p, "reject", p.Status, planDraft, map[string]any{"rejection_reason": r})); err != nil {
			return err
		}
		return s.notify(ctx, q, []int64{p.AcmAreaID}, "ACM-USER", nil, notification.Message{
			Type: "vault_plan.rejected", Title: "Penetapan vault ditolak ACM-SPV",
			Body: fmt.Sprintf("Penetapan vault %s (area %s) ditolak: %s", p.RequestNumber, p.AcmAreaName, r),
			Link: fmt.Sprintf("/cit/vault-plans/%d", p.ID), EntityType: vaultPlanEntity, EntityID: &p.ID, Email: true,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, db.New(s.pool), actor, id)
}

// ReviewRequest is the ATM-SPV / BRANCH-ATM-SPV decision on the whole
// recommendation (FR5.2/5.3): approve -> sent_to_vendor, parties synced and
// notified in the same tx (cit-send-vendor FR2); reject (reason) ->
// vault_assignment with every area plan back to draft. The checker must not be
// anyone who submitted or approved an area plan of the request.
func (s *VaultPlanService) ReviewRequest(ctx context.Context, actor Actor, requestID int64, approve bool, reason string) error {
	if !isChecker(actor.Role) {
		return ErrNotChecker
	}
	var r *string
	a := actionVaultApprove
	if !approve {
		trimmed, err := checkReason(reason, "vault_rejection_reason")
		if err != nil {
			return err
		}
		r, a = &trimmed, actionVaultReject
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	req, err := q.GetVendorRequestForUpdate(ctx, requestID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock vendor request %d: %w", requestID, err)
	}
	next, ok := nextState(req.Status, a)
	if !ok || req.IsCanceled {
		return ErrInvalidTransition
	}
	plans, err := q.ListVaultPlansForRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("list plans of request %d: %w", requestID, err)
	}
	if plannedBy(plans, actor.UserID) {
		return ErrSelfApproval
	}
	if !approve {
		if err := q.ResetVaultPlansToDraft(ctx, requestID); err != nil {
			return fmt.Errorf("reset plans of request %d: %w", requestID, err)
		}
	}
	if err := q.SetVendorRequestVaultReview(ctx, db.SetVendorRequestVaultReviewParams{Status: next, ActorID: actor.UserID, Reason: r, ID: requestID}); err != nil {
		return fmt.Errorf("review vendor request %d: %w", requestID, err)
	}
	after := map[string]any{"state": next}
	if r != nil {
		after["rejection_reason"] = *r
	}
	var notices []partyNotice
	if approve {
		if notices, err = syncVendorParties(ctx, q, actor.UserID, req); err != nil {
			return err
		}
		after["vendor_parties"] = auditParties(notices)
	}
	if err := audit.NewWriter(tx).Write(ctx, audit.Entry{ActorID: actor.UserID, Action: string(a), EntityType: "vendor_request",
		EntityID: requestID, Before: map[string]string{"state": req.Status}, After: after, IP: actor.IP}); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}

	if err := notifyParties(ctx, s.notifier, q, req, notices); err != nil {
		return err
	}
	if err := s.notifyReviewed(ctx, q, req, plans, r); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// plannedBy reports whether userID submitted or approved any area plan.
func plannedBy(plans []db.ListVaultPlansForRequestRow, userID int64) bool {
	for _, p := range plans {
		if deref(p.SubmittedBy) == userID || deref(p.ApprovedBy) == userID {
			return true
		}
	}
	return false
}

// notifyReviewed tells each area's ACM-USERs and the request creator about the
// ATM-SPV decision; rejection carries the reason (nil reason = approved).
func (s *VaultPlanService) notifyReviewed(ctx context.Context, q *db.Queries, req db.VendorRequest, plans []db.ListVaultPlansForRequestRow, reason *string) error {
	title, body := "Penetapan vault disetujui tim ATM", fmt.Sprintf("Penetapan vault %s disetujui; request dikirim ke vendor.", req.RequestNumber)
	if reason != nil {
		title, body = "Penetapan vault ditolak tim ATM", fmt.Sprintf("Penetapan vault %s ditolak: %s", req.RequestNumber, *reason)
	}
	// Each recipient gets a link it can open (review R3): ACM-USERs cannot open
	// Vendor Request pages, so they get their area's plan; the creator gets the request.
	for _, p := range plans {
		planID := p.ID
		if err := s.notify(ctx, q, []int64{p.AcmAreaID}, "ACM-USER", nil, notification.Message{
			Type: "vault_plan.reviewed", Title: title, Body: body,
			Link: fmt.Sprintf("/cit/vault-plans/%d", planID), EntityType: vaultPlanEntity, EntityID: &planID, Email: true,
		}); err != nil {
			return err
		}
	}
	requestID := req.ID
	return s.notify(ctx, q, nil, "", []int64{req.CreatedBy}, notification.Message{
		Type: "vault_plan.reviewed", Title: title, Body: body,
		Link: fmt.Sprintf("/replenishment/vendor-requests/%d", requestID), EntityType: "vendor_request", EntityID: &requestID, Email: true,
	})
}

// notify sends msg to the members of areas holding role plus extra users (FR6.3), inside the caller's tx.
func (s *VaultPlanService) notify(ctx context.Context, q *db.Queries, areaIDs []int64, role string, extra []int64, msg notification.Message) error {
	if s.notifier == nil {
		return nil
	}
	users := extra
	if len(areaIDs) > 0 {
		ids, err := q.ListAcmAreaMemberIDsByRole(ctx, db.ListAcmAreaMemberIDsByRoleParams{AreaIds: areaIDs, Role: role})
		if err != nil {
			return fmt.Errorf("list acm area members: %w", err)
		}
		users = append(users, ids...)
	}
	if len(users) == 0 {
		return nil
	}
	if err := s.notifier.Send(ctx, q, msg, notification.Recipients{UserIDs: users}); err != nil {
		return fmt.Errorf("notify %s: %w", msg.Type, err)
	}
	return nil
}

// checkReason trims and bounds a rejection reason (1-500 chars, like Vendor Request reject).
func checkReason(reason, field string) (string, error) {
	r := strings.TrimSpace(reason)
	if r == "" {
		return "", ErrRejectReasonEmpty
	}
	if len([]rune(r)) > 500 {
		return "", &ValidationError{Field: field, Message: "maksimal 500 karakter"}
	}
	return r, nil
}
