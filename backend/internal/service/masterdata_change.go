package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// ErrMasterDataForbidden means the caller is not allowed to stage master-data
// changes: no authenticated caller in ctx, a role outside
// masterDataMakerRoles, or a makerID that isn't the authenticated caller.
// Handlers map this to 403.
var ErrMasterDataForbidden = errors.New("actor is not authorized to change master data")

// masterDataMakerRoles are re-checked here, independent of the route guard
// (plan.md T3.6, "RBAC at middleware AND service"): a master-data route
// mounted without RequireRoles, or a new caller of these services, still
// cannot stage a change. Same set as the routes' RequireRoles(...).
var masterDataMakerRoles = map[string]bool{
	"ADMIN":       true,
	"ADMIN_PARAM": true,
}

// authorizeMasterDataMaker reports whether ctx carries an authenticated
// caller who is makerID and holds a master-data maker role.
func authorizeMasterDataMaker(ctx context.Context, makerID int64) error {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok || authCtx == nil || authCtx.UserID != makerID || !masterDataMakerRoles[strings.ToUpper(strings.TrimSpace(authCtx.Role))] {
		return ErrMasterDataForbidden
	}
	return nil
}

// ErrMasterDataChangePending means the target entity already has a pending
// change request -- a second submit is rejected (T2.2) rather than allowed
// to stack, so a maker can't bury an approver in duplicate/conflicting
// requests for the same entity. Handlers map this to 409.
var ErrMasterDataChangePending = errors.New("a pending change request already exists for this entity")

// masterDataDocumentType is the document_type every master-data change
// request routes through in approval_requests/approval_policies -- T1.7
// seeds a single policy row for it (range [0,1), required_level=1), since
// master-data changes carry no monetary amount.
const masterDataDocumentType = "master_data"

// MasterDataChangeAuditWriter is the narrow audit-write dependency
// MasterDataChangeService needs, mirroring VendorAdminAuditWriter.
type MasterDataChangeAuditWriter interface {
	Write(ctx context.Context, entry audit.Entry) error
}

// MasterDataChangeOrchestrator is the narrow approval.Orchestrator surface
// MasterDataChangeService needs to route a change request into the
// maker-checker chain.
type MasterDataChangeOrchestrator interface {
	SubmitForApproval(ctx context.Context, makerID int64, documentType string, documentID int64, amount pgtype.Numeric, actorIP string) (db.ApprovalRequest, bool, error)
}

// MasterDataChangeRepo is the repository surface MasterDataChangeService
// needs. *repository.MasterDataChangeRepository satisfies this automatically.
type MasterDataChangeRepo interface {
	FindPending(ctx context.Context, entityType string, entityID int64) (*int64, error)
	Create(ctx context.Context, arg db.CreateMasterDataChangeRequestParams) (db.MasterDataChangeRequest, error)
	SetApprovalRequestID(ctx context.Context, id, approvalRequestID int64) error
	GetByID(ctx context.Context, id int64) (db.MasterDataChangeRequest, error)
	List(ctx context.Context, arg db.ListMasterDataChangeRequestsParams) ([]db.MasterDataChangeRequest, error)
	Count(ctx context.Context, arg db.CountMasterDataChangeRequestsParams) (int64, error)
	MarkRejected(ctx context.Context, id int64) error
}

// MasterDataChangeService is the generic maker-checker entrypoint for every
// master-data create/update/disable/enable (plan.md Fase 2, Golden Rule #3
// / D1). It owns the staging write to master_data_change_requests and
// routes it through approval.Orchestrator -- the actual entity mutation
// happens only later, on final approval (T2.4), via the applier registry
// (T2.3). This service is entity-agnostic: it does not know how to read or
// write vendors/vaults/pics/etc. itself.
type MasterDataChangeService struct {
	repo         MasterDataChangeRepo
	orchestrator MasterDataChangeOrchestrator
	audit        MasterDataChangeAuditWriter
}

// NewMasterDataChangeService creates a MasterDataChangeService with the given dependencies.
func NewMasterDataChangeService(repo MasterDataChangeRepo, orchestrator MasterDataChangeOrchestrator, auditWriter MasterDataChangeAuditWriter) *MasterDataChangeService {
	return &MasterDataChangeService{repo: repo, orchestrator: orchestrator, audit: auditWriter}
}

// SubmitRequest holds the data for one master-data change submission.
// EntityID is nil for Op="create" (the entity doesn't exist yet); required
// for update/disable/enable. Before is the entity's current state at submit
// time (nil for create) -- callers are responsible for fetching it, since
// this service has no repository access to vendors/vaults/pics/etc. itself;
// it is kept for diff display and staleness detection (T2.5). BatchID
// groups rows from a single CSV import (T5.4); nil for a normal submit.
type SubmitRequest struct {
	EntityType string
	Op         string // create | update | disable | enable
	EntityID   *int64
	Payload    any
	Before     any
	BatchID    *int64
}

// Submit stages a master-data change request and routes it into the
// maker-checker chain: the row is written with status=pending here, the
// actual entity mutation happens only after approval (T2.4). Amount is
// always zero -- master-data changes carry no monetary value, see T1.7.
func (s *MasterDataChangeService) Submit(ctx context.Context, makerID int64, req SubmitRequest, actorIP string) (db.MasterDataChangeRequest, error) {
	if err := authorizeMasterDataMaker(ctx, makerID); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	if err := validateMasterDataOp(req.Op, req.EntityID); err != nil {
		return db.MasterDataChangeRequest{}, err
	}

	// Only update/disable/enable target an existing entity_id -- a create
	// has no entity yet, so there's nothing to collide on (and the DB's
	// partial unique index correctly does not block concurrent creates of
	// the same entity_type, since entity_id NULL is never equal to itself).
	if req.EntityID != nil {
		pendingID, err := s.repo.FindPending(ctx, req.EntityType, *req.EntityID)
		if err != nil {
			return db.MasterDataChangeRequest{}, fmt.Errorf("check pending change request: %w", err)
		}
		if pendingID != nil {
			return db.MasterDataChangeRequest{}, ErrMasterDataChangePending
		}
	}

	payload, err := marshalOrNilJSON(req.Payload)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("marshal payload: %w", err)
	}
	if payload == nil {
		// payload is NOT NULL in the DB; disable/enable ops carry no field
		// changes (Before holds the pre-toggle snapshot instead), so there is
		// nothing to marshal -- use an empty object rather than a NULL.
		payload = []byte("{}")
	}
	before, err := marshalOrNilJSON(req.Before)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("marshal before snapshot: %w", err)
	}

	change, err := s.repo.Create(ctx, db.CreateMasterDataChangeRequestParams{
		EntityType: req.EntityType,
		EntityID:   req.EntityID,
		Op:         req.Op,
		Payload:    payload,
		Before:     before,
		MakerID:    makerID,
		BatchID:    req.BatchID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			// Pre-check above missed a race; the DB's partial unique index
			// (entity_type, entity_id) WHERE status='pending' is the source
			// of truth.
			return db.MasterDataChangeRequest{}, ErrMasterDataChangePending
		}
		return db.MasterDataChangeRequest{}, fmt.Errorf("create change request: %w", err)
	}

	var amount pgtype.Numeric
	if err := amount.Scan("0"); err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("build zero amount: %w", err)
	}

	approvalRequest, _, err := s.orchestrator.SubmitForApproval(ctx, makerID, masterDataDocumentType, change.ID, amount, actorIP)
	if err != nil {
		s.abandon(ctx, change.ID)
		return db.MasterDataChangeRequest{}, fmt.Errorf("submit for approval: %w", err)
	}

	if err := s.repo.SetApprovalRequestID(ctx, change.ID, approvalRequest.ID); err != nil {
		s.abandon(ctx, change.ID)
		return db.MasterDataChangeRequest{}, fmt.Errorf("link approval request: %w", err)
	}
	change.ApprovalRequestID = &approvalRequest.ID

	if err := s.audit.Write(ctx, audit.Entry{
		ActorID:    makerID,
		Action:     "submit",
		EntityType: "master_data_change_request",
		EntityID:   change.ID,
		After:      change,
		IP:         actorIP,
	}); err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("write audit log: %w", err)
	}

	return change, nil
}

// Get returns a change request by id, with its full payload/before for diff
// display (T2.7). Read-only, never writes an audit entry.
func (s *MasterDataChangeService) Get(ctx context.Context, id int64) (db.MasterDataChangeRequest, error) {
	return s.repo.GetByID(ctx, id)
}

// List returns a page of change requests, filtered by entity_type/status
// (both optional) (T2.7). Read-only, never writes an audit entry.
func (s *MasterDataChangeService) List(ctx context.Context, arg db.ListMasterDataChangeRequestsParams) ([]db.MasterDataChangeRequest, error) {
	return s.repo.List(ctx, arg)
}

// Count returns the total number of change requests matching List's filters,
// without pagination (T2.7). Read-only, never writes an audit entry.
func (s *MasterDataChangeService) Count(ctx context.Context, arg db.CountMasterDataChangeRequestsParams) (int64, error) {
	return s.repo.Count(ctx, arg)
}

// validateMasterDataOp mirrors the DB CHECK constraint
// (master_data_change_requests_entity_id_chk) as an early, friendlier
// validation error instead of a raw constraint violation.
func validateMasterDataOp(op string, entityID *int64) error {
	switch op {
	case "create":
		if entityID != nil {
			return &ValidationError{Field: "entity_id", Message: "must be empty for op=create"}
		}
	case "update", "disable", "enable":
		if entityID == nil {
			return &ValidationError{Field: "entity_id", Message: "required for op=" + op}
		}
	default:
		return &ValidationError{Field: "op", Message: "must be one of create, update, disable, enable"}
	}
	return nil
}

func marshalOrNilJSON(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

// abandon frees the pending-unique slot of a change whose approval routing
// failed; without it the entity would be blocked by a request nobody can approve.
func (s *MasterDataChangeService) abandon(ctx context.Context, id int64) {
	if err := s.repo.MarkRejected(ctx, id); err != nil {
		slog.Error("master-data: could not release change after failed submit", "change_id", id, "error", err)
	}
}
