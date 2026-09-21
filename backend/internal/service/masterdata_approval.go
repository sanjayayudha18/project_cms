package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

// ErrMasterDataChangeStale means the target entity changed between submit
// and approval (T2.5) -- the change is marked 'stale' and not applied. The
// maker must review the current state and resubmit.
var ErrMasterDataChangeStale = errors.New("entity has changed since this change request was submitted")

// MasterDataApprovalOrchestrator is the narrow approval.Orchestrator surface
// MasterDataApprovalService needs. Includes SubmitForApproval (passed
// through unchanged, see MasterDataApprovalService.SubmitForApproval below)
// so a *MasterDataApprovalService can satisfy the same
// handler.ApprovalOrchestrator interface the generic /api/v1/approvals
// endpoint needs -- letting that shared endpoint (used by every
// document_type: invoice, vendor_request, master_data, ...) route through
// this service instead of the raw orchestrator, which is what actually
// wires the apply-on-approve hook (T2.4) into the live approve/reject path.
// For any document_type other than "master_data" this is a pure passthrough
// (see Approve/Reject below) -- existing flows are unaffected.
type MasterDataApprovalOrchestrator interface {
	SubmitForApproval(ctx context.Context, makerID int64, documentType string, documentID int64, amount pgtype.Numeric, actorIP string) (db.ApprovalRequest, bool, error)
	Approve(ctx context.Context, requestID, actorID int64, actorIP string) (db.ApprovalRequest, error)
	Reject(ctx context.Context, requestID, actorID int64, actorIP string) (db.ApprovalRequest, error)
}

// MasterDataApprovalPool is the transaction-opening surface the
// apply-on-approve hook needs. *pgxpool.Pool satisfies this via Begin --
// mirrors rolemgmt.Pool's narrow-interface convention.
type MasterDataApprovalPool interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// MasterDataApprovalRepo is the (non-transactional) repository surface
// MasterDataApprovalService needs outside the apply transaction: reading
// the staged change and recording its approved/apply-failed status, both of
// which must persist independent of whether the apply transaction commits.
type MasterDataApprovalRepo interface {
	GetByID(ctx context.Context, id int64) (db.MasterDataChangeRequest, error)
	MarkApproved(ctx context.Context, id int64) error
	MarkApplyFailed(ctx context.Context, id int64, errMsg string) error
	MarkStale(ctx context.Context, id int64, errMsg string) error
	MarkRejected(ctx context.Context, id int64) error
	ListByBatch(ctx context.Context, batchID int64) ([]db.MasterDataChangeRequest, error)
}

// MasterDataApprovalAuditWriter is the narrow audit-write dependency
// MasterDataApprovalService.Reject needs (no entity mutation happens on
// reject, so no transaction is required -- see Approve/applyMasterDataChange
// for the transactional audit write on the apply path).
type MasterDataApprovalAuditWriter interface {
	Write(ctx context.Context, entry audit.Entry) error
}

// MasterDataApprovalService wraps approval.Orchestrator.Approve with the
// master-data apply-on-approve hook (plan.md T2.4): once a master_data
// approval request reaches its final step, the corresponding
// master_data_change_requests row is applied to the real entity -- in one
// transaction with the status flip to 'applied' and its audit entry. An
// apply failure rolls back only that transaction; the approval decision
// itself (already committed inside Orchestrator.Approve) and the
// change request's 'approved' status are unaffected -- the row stays
// approved-but-not-applied, with the error recorded, for retry/diagnosis.
type MasterDataApprovalService struct {
	orchestrator MasterDataApprovalOrchestrator
	repo         MasterDataApprovalRepo
	pool         MasterDataApprovalPool
	appliers     ApplierRegistry
	audit        MasterDataApprovalAuditWriter
}

// NewMasterDataApprovalService creates a MasterDataApprovalService with the given dependencies.
func NewMasterDataApprovalService(orchestrator MasterDataApprovalOrchestrator, repo MasterDataApprovalRepo, pool MasterDataApprovalPool, appliers ApplierRegistry, auditWriter MasterDataApprovalAuditWriter) *MasterDataApprovalService {
	return &MasterDataApprovalService{orchestrator: orchestrator, repo: repo, pool: pool, appliers: appliers, audit: auditWriter}
}

// SubmitForApproval passes through to the underlying orchestrator
// unchanged. Master-data submissions never call this directly (they go
// through MasterDataChangeService.Submit, which calls the orchestrator
// itself and stages a master_data_change_requests row); this only exists so
// *MasterDataApprovalService satisfies the full handler.ApprovalOrchestrator
// interface for the shared /api/v1/approvals endpoint, which every
// document_type's submit still goes through.
func (s *MasterDataApprovalService) SubmitForApproval(ctx context.Context, makerID int64, documentType string, documentID int64, amount pgtype.Numeric, actorIP string) (db.ApprovalRequest, bool, error) {
	return s.orchestrator.SubmitForApproval(ctx, makerID, documentType, documentID, amount, actorIP)
}

// Approve advances the approval chain for requestID. If this was the final
// step and the request is a master-data change, it is applied immediately
// afterward. An apply failure is returned as an error, but does not change
// the fact that the approval itself succeeded -- request is still returned
// alongside the error so callers can tell "approved, apply pending retry"
// apart from "approval itself failed" (e.g. actor not authorized).
func (s *MasterDataApprovalService) Approve(ctx context.Context, requestID, actorID int64, actorIP string) (db.ApprovalRequest, error) {
	request, err := s.orchestrator.Approve(ctx, requestID, actorID, actorIP)
	if err != nil {
		return db.ApprovalRequest{}, err
	}

	if request.DocumentType != masterDataDocumentType || request.Status != "approved" {
		// Not a master-data request, or not yet the final step (more
		// approval levels remain) -- nothing to apply yet.
		return request, nil
	}

	if applyErr := s.applyMasterDataChange(ctx, actorID, request.DocumentID, actorIP); applyErr != nil {
		return request, fmt.Errorf("approved but apply failed (change request %d): %w", request.DocumentID, applyErr)
	}

	return request, nil
}

// Reject advances the approval chain's reject path for requestID. Rejecting
// a master-data change makes no entity mutation (T2.6): the change request
// is simply flipped to 'rejected', mirrored with its own audit entry
// (entity_type="master_data_change_request", action="reject") alongside
// the orchestrator's own entity_type="approval_request" audit entry.
func (s *MasterDataApprovalService) Reject(ctx context.Context, requestID, actorID int64, actorIP string) (db.ApprovalRequest, error) {
	request, err := s.orchestrator.Reject(ctx, requestID, actorID, actorIP)
	if err != nil {
		return db.ApprovalRequest{}, err
	}

	if request.DocumentType != masterDataDocumentType {
		return request, nil
	}

	ids := []int64{request.DocumentID}
	head, err := s.repo.GetByID(ctx, request.DocumentID)
	if err != nil {
		return request, fmt.Errorf("load change request: %w", err)
	}
	if head.BatchID != nil {
		// Rejecting an import batch's single approval rejects every row of it (T5.4).
		rows, err := s.repo.ListByBatch(ctx, *head.BatchID)
		if err != nil {
			return request, fmt.Errorf("load batch: %w", err)
		}
		ids = ids[:0]
		for _, c := range rows {
			ids = append(ids, c.ID)
		}
	}
	for _, id := range ids {
		if err := s.repo.MarkRejected(ctx, id); err != nil {
			return request, fmt.Errorf("mark change request rejected: %w", err)
		}
	}

	if err := s.audit.Write(ctx, audit.Entry{
		ActorID:    actorID,
		Action:     "reject",
		EntityType: "master_data_change_request",
		EntityID:   request.DocumentID,
		IP:         actorIP,
	}); err != nil {
		return request, fmt.Errorf("write audit log: %w", err)
	}

	return request, nil
}

// applyMasterDataChange runs the T2.4 apply-on-approve flow for one
// already-finally-approved change request.
//
// A change request that belongs to an import batch (T5.4) is the batch's head:
// the batch has ONE approval, and approving it applies every row of the batch
// in ONE transaction -- all rows or none. If any row fails (stale, duplicate,
// overlap, ...) nothing of the batch is written; the failing row records its
// own error and the head notes the rollback.
func (s *MasterDataApprovalService) applyMasterDataChange(ctx context.Context, actorID, changeID int64, actorIP string) error {
	if err := s.repo.MarkApproved(ctx, changeID); err != nil {
		return fmt.Errorf("mark change request approved: %w", err)
	}

	head, err := s.repo.GetByID(ctx, changeID)
	if err != nil {
		return fmt.Errorf("load change request: %w", err)
	}
	changes := []db.MasterDataChangeRequest{head}
	if head.BatchID != nil {
		if changes, err = s.repo.ListByBatch(ctx, *head.BatchID); err != nil {
			return fmt.Errorf("load batch: %w", err)
		}
		for _, c := range changes {
			if c.ID != changeID {
				if err := s.repo.MarkApproved(ctx, c.ID); err != nil {
					return fmt.Errorf("mark change request approved: %w", err)
				}
			}
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin apply tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, change := range changes {
		if err := s.applyOne(ctx, tx, actorID, change, actorIP); err != nil {
			if head.BatchID != nil {
				// Nothing of the batch landed: retire every row so the same file can be
				// uploaded again (FindOpen skips stale batches) once the cause is fixed.
				msg := fmt.Sprintf("batch rolled back: change request %d failed: %v", change.ID, err)
				for _, c := range changes {
					_ = s.repo.MarkStale(ctx, c.ID, msg)
				}
			}
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		_ = s.repo.MarkApplyFailed(ctx, changeID, err.Error())
		return fmt.Errorf("commit apply tx: %w", err)
	}
	return nil
}

// applyOne applies one change inside the caller's transaction: staleness
// check, entity mutation, status flip and audit entry. It records its own
// failure on the change request; committing is the caller's job.
func (s *MasterDataApprovalService) applyOne(ctx context.Context, tx pgx.Tx, actorID int64, change db.MasterDataChangeRequest, actorIP string) error {
	changeID := change.ID
	applier, ok := s.appliers[change.EntityType]
	if !ok {
		applyErr := fmt.Errorf("no applier registered for entity_type=%s", change.EntityType)
		_ = s.repo.MarkApplyFailed(ctx, changeID, applyErr.Error())
		return applyErr
	}

	// T2.5 staleness check: only meaningful for update/disable/enable
	// (op=create has no prior entity state to compare against) and only
	// when the maker actually captured a Before snapshot at submit time --
	// a nil Before on a non-create op skips the check rather than treating
	// it as an error, since Submit does not hard-require Before.
	if change.Op != "create" && change.EntityID != nil && len(change.Before) > 0 {
		current, stateErr := applier.CurrentState(ctx, tx, *change.EntityID)
		if stateErr != nil {
			_ = s.repo.MarkApplyFailed(ctx, changeID, stateErr.Error())
			return fmt.Errorf("load current state for staleness check: %w", stateErr)
		}
		same, cmpErr := statesEqual(change.Before, current)
		if cmpErr != nil {
			_ = s.repo.MarkApplyFailed(ctx, changeID, cmpErr.Error())
			return fmt.Errorf("compare current state for staleness check: %w", cmpErr)
		}
		if !same {
			_ = s.repo.MarkStale(ctx, changeID, ErrMasterDataChangeStale.Error())
			return ErrMasterDataChangeStale
		}
	}

	entityID, after, applyErr := applier.Apply(ctx, tx, change)
	if applyErr != nil {
		_ = s.repo.MarkApplyFailed(ctx, changeID, applyErr.Error())
		return fmt.Errorf("apply change: %w", applyErr)
	}

	if err := db.New(tx).MarkMasterDataChangeRequestApplied(ctx, changeID); err != nil {
		_ = s.repo.MarkApplyFailed(ctx, changeID, err.Error())
		return fmt.Errorf("mark applied: %w", err)
	}

	var before any
	if len(change.Before) > 0 {
		before = json.RawMessage(change.Before)
	}
	if err := audit.NewWriter(tx).Write(ctx, audit.Entry{
		ActorID:    actorID,
		Action:     "master_data_applied",
		EntityType: change.EntityType,
		EntityID:   entityID,
		Before:     before,
		After:      after,
		IP:         actorIP,
	}); err != nil {
		_ = s.repo.MarkApplyFailed(ctx, changeID, err.Error())
		return fmt.Errorf("write audit log: %w", err)
	}

	return nil
}

// statesEqual reports whether before (raw jsonb from
// master_data_change_requests.before) and current (an Applier.CurrentState
// result) represent the same entity state. Both sides are decoded into
// map[string]any before comparing, so this is robust to struct field
// ordering -- it only requires before and current to have been produced
// from the same JSON field set (Applier.CurrentState's contract).
func statesEqual(before []byte, current any) (bool, error) {
	currentJSON, err := json.Marshal(current)
	if err != nil {
		return false, fmt.Errorf("marshal current state: %w", err)
	}

	var beforeMap, currentMap map[string]any
	if err := json.Unmarshal(before, &beforeMap); err != nil {
		return false, fmt.Errorf("unmarshal before snapshot: %w", err)
	}
	if err := json.Unmarshal(currentJSON, &currentMap); err != nil {
		return false, fmt.Errorf("unmarshal current state: %w", err)
	}

	return reflect.DeepEqual(beforeMap, currentMap), nil
}
