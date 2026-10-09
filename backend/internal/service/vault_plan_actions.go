package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

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

// VaultAssignmentInput is one ATM's chosen vault branch (FR4.3).
type VaultAssignmentInput struct {
	TerminalID    string  `json:"terminal_id"`
	VaultBranchID int64   `json:"vault_branch_id"`
	IsUrgent      bool    `json:"is_urgent"`
	UrgentReason  *string `json:"urgent_reason"`
}

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
	p.Status, p.SubmittedBy = locked.Status, locked.SubmittedBy // fresh after the lock

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

// SaveAssignments replaces the plan's assignments (draft only) and refreshes
// snapshots (FR4.3). Not every ATM needs a vault yet; Submit checks that.
func (s *VaultPlanService) SaveAssignments(ctx context.Context, actor Actor, id int64, in []VaultAssignmentInput) (*VaultPlanDetail, error) {
	err := s.planTx(ctx, actor, id, "ACM-USER", func(q *db.Queries, aw *audit.Writer, p db.GetVaultPlanRow, req db.VendorRequest) error {
		if p.Status != planDraft || req.Status != "vault_assignment" {
			return ErrInvalidTransition
		}
		rows, err := validateAssignments(ctx, q, p, in)
		if err != nil {
			return err
		}
		if err := q.ClearVaultPlanAssignments(ctx, p.ID); err != nil {
			return fmt.Errorf("clear assignments: %w", err)
		}
		logged := make([]map[string]any, len(rows))
		for i, r := range rows {
			if _, err := q.InsertVaultAssignment(ctx, r); err != nil {
				var pgErr *pgconn.PgError
				if errors.As(err, &pgErr) && pgErr.ConstraintName == "vendor_request_vault_assignments_atm_uq" {
					// The ATM's branch moved area mid-flow; its old row sits in another area's plan.
					return &ValidationError{Field: "assignments", Message: fmt.Sprintf(
						"ATM %s masih tercatat di rencana area lain untuk request ini; rencana itu harus disimpan ulang (atau ditolak dulu) sebelum ATM ini ditetapkan di sini", r.TerminalID)}
				}
				return fmt.Errorf("insert assignment %s: %w", r.TerminalID, err)
			}
			logged[i] = map[string]any{"terminal_id": r.TerminalID, "vault_branch_id": r.VaultBranchID, "tier": r.Tier,
				"is_urgent": r.IsUrgent, "urgent_reason": r.UrgentReason}
		}
		if err := recomputeSnapshots(ctx, q, p); err != nil {
			return err
		}
		return aw.Write(ctx, planAudit(actor, p, "vault_assignments_saved", p.Status, p.Status, map[string]any{"assignments": logged}))
	})
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, db.New(s.pool), actor, id)
}

// validateAssignments checks FR4.3 per input and returns the rows to insert:
// ATM belongs to the plan (once), vault is an active cash-capable branch,
// tier 3 only as urgent, urgent reason 10-500 chars.
func validateAssignments(ctx context.Context, q *db.Queries, p db.GetVaultPlanRow, in []VaultAssignmentInput) ([]db.InsertVaultAssignmentParams, error) {
	atms, err := q.ListVaultPlanAtms(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("list plan atms: %w", err)
	}
	byTerminal := make(map[string]db.ListVaultPlanAtmsRow, len(atms))
	for _, a := range atms {
		byTerminal[a.TerminalID] = a
	}
	branches, err := candidateBranchIndex(ctx, q)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := make([]db.InsertVaultAssignmentParams, 0, len(in))
	for _, a := range in {
		atm, ok := byTerminal[a.TerminalID]
		if !ok {
			return nil, &ValidationError{Field: "assignments", Message: fmt.Sprintf("ATM %s bukan bagian dari rencana ini", a.TerminalID)}
		}
		if seen[a.TerminalID] {
			return nil, &ValidationError{Field: "assignments", Message: fmt.Sprintf("ATM %s muncul lebih dari sekali", a.TerminalID)}
		}
		seen[a.TerminalID] = true
		b, ok := branches[a.VaultBranchID]
		if !ok {
			return nil, &ValidationError{Field: "assignments", Message: fmt.Sprintf("ATM %s: branch vault tidak aktif / bukan CASH-ATM_CASH / tanpa kode region", a.TerminalID)}
		}
		tier := vaultTier(b.VendorID, b.RegionCode, atm.ReplenishVendorID, atm.ReplenishRegionCode)
		var reason *string
		if a.IsUrgent {
			r := strings.TrimSpace(deref(a.UrgentReason))
			if n := len([]rune(r)); n < 10 || n > 500 {
				return nil, &ValidationError{Field: "assignments", Message: fmt.Sprintf("ATM %s: alasan urgent wajib 10-500 karakter", a.TerminalID)}
			}
			reason = &r
		}
		if tier == 3 && !a.IsUrgent {
			return nil, &ValidationError{Field: "assignments", Message: fmt.Sprintf("ATM %s: branch beda region hanya boleh sebagai urgent", a.TerminalID)}
		}
		out = append(out, db.InsertVaultAssignmentParams{VaultPlanID: p.ID, VendorRequestID: p.VendorRequestID, TerminalID: a.TerminalID,
			ReplenishBranchID: atm.ReplenishBranchID, VaultBranchID: a.VaultBranchID, Tier: tier, IsUrgent: a.IsUrgent, UrgentReason: reason})
	}
	return out, nil
}

// Submit sends a complete draft plan to the area's ACM-SPV (FR4.4): every ATM
// assigned, assignments re-validated against current data, snapshots refreshed.
func (s *VaultPlanService) Submit(ctx context.Context, actor Actor, id int64) (*VaultPlanDetail, error) {
	err := s.planTx(ctx, actor, id, "ACM-USER", func(q *db.Queries, aw *audit.Writer, p db.GetVaultPlanRow, req db.VendorRequest) error {
		if p.Status != planDraft || req.Status != "vault_assignment" {
			return ErrInvalidTransition
		}
		atms, err := q.ListVaultPlanAtms(ctx, p.ID)
		if err != nil {
			return fmt.Errorf("list plan atms: %w", err)
		}
		current := make([]VaultAssignmentInput, 0, len(atms))
		for _, a := range atms {
			if a.VaultBranchID == nil {
				return &ValidationError{Field: "assignments", Message: fmt.Sprintf("ATM %s belum punya branch vault", a.TerminalID)}
			}
			current = append(current, VaultAssignmentInput{TerminalID: a.TerminalID, VaultBranchID: *a.VaultBranchID,
				IsUrgent: deref(a.IsUrgent), UrgentReason: a.UrgentReason})
		}
		if _, err := validateAssignments(ctx, q, p, current); err != nil {
			return err
		}
		if err := recomputeSnapshots(ctx, q, p); err != nil {
			return err
		}
		if err := q.SetVaultPlanStatus(ctx, db.SetVaultPlanStatusParams{Status: planPending, ActorID: &actor.UserID, ID: p.ID}); err != nil {
			return fmt.Errorf("submit vault plan: %w", err)
		}
		if err := aw.Write(ctx, planAudit(actor, p, "submit", p.Status, planPending, nil)); err != nil {
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

// Approve is the ACM-SPV approval of one area plan (FR4.5). When every ATM of
// the request is covered by an approved plan, the request moves to
// vault_review in the same tx (FR4.6, no notification by N1).
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
		missing, err := q.CountRequestAtmsWithoutApprovedVault(ctx, req.ID)
		if err != nil {
			return fmt.Errorf("check request coverage: %w", err)
		}
		if missing > 0 {
			return nil
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
	for _, p := range plans {
		if deref(p.SubmittedBy) == actor.UserID || deref(p.ApprovedBy) == actor.UserID {
			return ErrSelfApproval
		}
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
	title, body := "Penetapan vault disetujui tim ATM", fmt.Sprintf("Penetapan vault %s disetujui; request dikirim ke vendor.", req.RequestNumber)
	if !approve {
		title, body = "Penetapan vault ditolak tim ATM", fmt.Sprintf("Penetapan vault %s ditolak: %s", req.RequestNumber, *r)
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
	if err := s.notify(ctx, q, nil, "", []int64{req.CreatedBy}, notification.Message{
		Type: "vault_plan.reviewed", Title: title, Body: body,
		Link: fmt.Sprintf("/replenishment/vendor-requests/%d", requestID), EntityType: "vendor_request", EntityID: &requestID, Email: true,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
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
