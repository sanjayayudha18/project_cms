package approval

import (
	"fmt"
	"time"

	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Orchestrator drives the maker-checker state machine: submit, approve,
// reject. Every transition writes an audit_logs entry. Writes go through
// store, which callers wire to the primary DB pool (Sec 6: write -> primary).
type Orchestrator struct {
	store        RequestStore
	chainRepo    ChainRepository
	availability AvailabilityRepository
	audit        AuditWriter
	now          func() time.Time
}

// NewOrchestrator creates an Orchestrator. now defaults to time.Now if nil
// (tests can override it for deterministic leave/delegation checks).
func NewOrchestrator(store RequestStore, chainRepo ChainRepository, availability AvailabilityRepository, auditWriter AuditWriter, now func() time.Time) *Orchestrator {
	if now == nil {
		now = time.Now
	}
	return &Orchestrator{store: store, chainRepo: chainRepo, availability: availability, audit: auditWriter, now: now}
}

// SubmitForApproval looks up the required level for (documentType, amount),
// builds the approver chain from makerID's supervisors, and creates the
// request + its steps. Idempotent per (documentType, documentID): a second
// submit for the same document returns the existing request unchanged, with
// created=false so callers (e.g. the HTTP handler) can tell first submission
// apart from a repeat.
func (o *Orchestrator) SubmitForApproval(ctx context.Context, makerID int64, documentType string, documentID int64, amount pgtype.Numeric, actorIP string) (request db.ApprovalRequest, created bool, err error) {
	existing, err := o.store.FindRequestByDocument(ctx, documentType, documentID)
	if err != nil {
		return db.ApprovalRequest{}, false, fmt.Errorf("check existing request for %s/%d: %w", documentType, documentID, err)
	}
	if existing != nil {
		return *existing, false, nil
	}

	requiredLevel, err := o.store.FindPolicy(ctx, documentType, amount)
	if err != nil {
		return db.ApprovalRequest{}, false, fmt.Errorf("document_type=%s: %w: %v", documentType, ErrPolicyNotFound, err)
	}

	chain, err := BuildChain(ctx, o.chainRepo, makerID, requiredLevel)
	if err != nil {
		return db.ApprovalRequest{}, false, err
	}

	request, err = o.store.CreateRequest(ctx, makerID, documentType, documentID, amount, requiredLevel)
	if err != nil {
		return db.ApprovalRequest{}, false, fmt.Errorf("create approval request: %w", err)
	}

	for _, approver := range chain {
		if _, err := o.store.CreateStep(ctx, request.ID, *approver.ApprovalLevel, approver.UserID); err != nil {
			return db.ApprovalRequest{}, false, fmt.Errorf("create approval step (level %d): %w", *approver.ApprovalLevel, err)
		}
	}

	if err := o.audit.Write(ctx, audit.Entry{
		ActorID:    makerID,
		Action:     "submit",
		EntityType: "approval_request",
		EntityID:   request.ID,
		After:      request,
		IP:         actorIP,
	}); err != nil {
		return db.ApprovalRequest{}, false, fmt.Errorf("write audit log: %w", err)
	}

	return request, true, nil
}

// Approve validates that actorID is the effective approver (assigned approver,
// or their delegate if on leave) for the request's current pending step, then
// advances it. On the last step, the request itself becomes approved.
func (o *Orchestrator) Approve(ctx context.Context, requestID, actorID int64, actorIP string) (db.ApprovalRequest, error) {
	request, step, err := o.loadPendingStep(ctx, requestID)
	if err != nil {
		return db.ApprovalRequest{}, err
	}

	effectiveApprover, err := ResolveEffectiveApprover(ctx, o.availability, step.AssignedApproverID, request.MakerID, o.now())
	if err != nil {
		return db.ApprovalRequest{}, err
	}
	if actorID != effectiveApprover {
		return db.ApprovalRequest{}, fmt.Errorf("actor %d, expected approver %d for step %d: %w", actorID, effectiveApprover, step.ID, ErrNotAuthorized)
	}

	updatedStep, err := o.store.UpdateStepStatus(ctx, step.ID, "approved", actorID, o.now())
	if err != nil {
		return db.ApprovalRequest{}, fmt.Errorf("update approval step %d: %w", step.ID, err)
	}

	if err := o.audit.Write(ctx, audit.Entry{
		ActorID:    actorID,
		Action:     "approve",
		EntityType: "approval_step",
		EntityID:   updatedStep.ID,
		Before:     map[string]string{"status": "pending"},
		After:      map[string]string{"status": "approved"},
		IP:         actorIP,
	}); err != nil {
		return db.ApprovalRequest{}, fmt.Errorf("write audit log: %w", err)
	}

	steps, err := o.store.ListSteps(ctx, requestID)
	if err != nil {
		return db.ApprovalRequest{}, fmt.Errorf("list steps for request %d: %w", requestID, err)
	}
	var maxLevel int32
	for _, s := range steps {
		if s.StepLevel > maxLevel {
			maxLevel = s.StepLevel
		}
	}

	if updatedStep.StepLevel < maxLevel {
		// More steps above this one — request stays pending.
		return request, nil
	}

	finalRequest, err := o.store.UpdateRequestStatus(ctx, requestID, "approved")
	if err != nil {
		return db.ApprovalRequest{}, fmt.Errorf("update approval request %d: %w", requestID, err)
	}

	if err := o.audit.Write(ctx, audit.Entry{
		ActorID:    actorID,
		Action:     "final_approve",
		EntityType: "approval_request",
		EntityID:   requestID,
		Before:     map[string]string{"status": "pending"},
		After:      map[string]string{"status": "approved"},
		IP:         actorIP,
	}); err != nil {
		return db.ApprovalRequest{}, fmt.Errorf("write audit log: %w", err)
	}

	return finalRequest, nil
}

// Reject validates that actorID is the effective approver for the request's
// current pending step, then stops the whole request as rejected.
func (o *Orchestrator) Reject(ctx context.Context, requestID, actorID int64, actorIP string) (db.ApprovalRequest, error) {
	request, step, err := o.loadPendingStep(ctx, requestID)
	if err != nil {
		return db.ApprovalRequest{}, err
	}

	effectiveApprover, err := ResolveEffectiveApprover(ctx, o.availability, step.AssignedApproverID, request.MakerID, o.now())
	if err != nil {
		return db.ApprovalRequest{}, err
	}
	if actorID != effectiveApprover {
		return db.ApprovalRequest{}, fmt.Errorf("actor %d, expected approver %d for step %d: %w", actorID, effectiveApprover, step.ID, ErrNotAuthorized)
	}

	if _, err := o.store.UpdateStepStatus(ctx, step.ID, "rejected", actorID, o.now()); err != nil {
		return db.ApprovalRequest{}, fmt.Errorf("update approval step %d: %w", step.ID, err)
	}

	finalRequest, err := o.store.UpdateRequestStatus(ctx, requestID, "rejected")
	if err != nil {
		return db.ApprovalRequest{}, fmt.Errorf("update approval request %d: %w", requestID, err)
	}

	if err := o.audit.Write(ctx, audit.Entry{
		ActorID:    actorID,
		Action:     "reject",
		EntityType: "approval_request",
		EntityID:   requestID,
		Before:     map[string]string{"status": "pending"},
		After:      map[string]string{"status": "rejected"},
		IP:         actorIP,
	}); err != nil {
		return db.ApprovalRequest{}, fmt.Errorf("write audit log: %w", err)
	}

	return finalRequest, nil
}

// loadPendingStep fetches the request and validates it has an active pending
// step to act on, shared by Approve and Reject.
func (o *Orchestrator) loadPendingStep(ctx context.Context, requestID int64) (db.ApprovalRequest, db.ApprovalStep, error) {
	request, err := o.store.GetRequest(ctx, requestID)
	if err != nil {
		return db.ApprovalRequest{}, db.ApprovalStep{}, fmt.Errorf("load approval request %d: %w", requestID, err)
	}
	if request.Status != "pending" {
		return db.ApprovalRequest{}, db.ApprovalStep{}, fmt.Errorf("request %d has status=%s: %w", requestID, request.Status, ErrRequestNotPending)
	}

	step, err := o.store.FindPendingStep(ctx, requestID)
	if err != nil {
		return db.ApprovalRequest{}, db.ApprovalStep{}, fmt.Errorf("load pending step for request %d: %w", requestID, err)
	}
	if step == nil {
		return db.ApprovalRequest{}, db.ApprovalStep{}, fmt.Errorf("request %d: %w", requestID, ErrRequestNotPending)
	}

	return request, *step, nil
}
