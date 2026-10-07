package approval

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// pgExclusionViolation is the Postgres error code for an EXCLUDE constraint
// violation (approval_delegations_no_overlap, migration 025).
const pgExclusionViolation = "23P01"

// Repository implements ChainRepository and AvailabilityRepository using
// sqlc-generated queries.
type Repository struct {
	queries *db.Queries
}

// NewRepository creates a Repository wrapping the given database connection.
func NewRepository(dbConn db.DBTX) *Repository {
	return &Repository{queries: db.New(dbConn)}
}

// GetApproverInfo implements ChainRepository.
func (r *Repository) GetApproverInfo(ctx context.Context, userID int64) (ApproverInfo, error) {
	row, err := r.queries.GetUserApprovalInfo(ctx, userID)
	if err != nil {
		return ApproverInfo{}, err
	}
	return ApproverInfo{
		UserID:        row.ID,
		SupervisorID:  row.SupervisorID,
		ApprovalLevel: row.ApprovalLevel,
	}, nil
}

// IsOnLeave implements AvailabilityRepository.
func (r *Repository) IsOnLeave(ctx context.Context, userID int64, at time.Time) (bool, error) {
	_, err := r.queries.FindActiveLeave(ctx, db.FindActiveLeaveParams{UserID: userID, At: pgtype.Timestamptz{Time: at, Valid: true}})
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// FindActiveDelegate implements AvailabilityRepository.
func (r *Repository) FindActiveDelegate(ctx context.Context, fromUserID int64, at time.Time) (*int64, error) {
	toUserID, err := r.queries.FindActiveDelegate(ctx, db.FindActiveDelegateParams{FromUserID: fromUserID, At: pgtype.Timestamptz{Time: at, Valid: true}})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &toUserID, nil
}

// FindPolicy implements RequestStore.
func (r *Repository) FindPolicy(ctx context.Context, documentType string, amount pgtype.Numeric) (int32, error) {
	return r.queries.FindApprovalPolicy(ctx, db.FindApprovalPolicyParams{DocumentType: documentType, Amount: amount})
}

// FindRequestByDocument implements RequestStore. Returns (nil, nil) if no row matches.
func (r *Repository) FindRequestByDocument(ctx context.Context, documentType string, documentID int64) (*db.ApprovalRequest, error) {
	row, err := r.queries.FindApprovalRequestByDocument(ctx, db.FindApprovalRequestByDocumentParams{DocumentType: documentType, DocumentID: documentID})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// CreateRequest implements RequestStore.
func (r *Repository) CreateRequest(ctx context.Context, makerID int64, documentType string, documentID int64, amount pgtype.Numeric, requiredLevel int32) (db.ApprovalRequest, error) {
	return r.queries.CreateApprovalRequest(ctx, db.CreateApprovalRequestParams{
		MakerID:       makerID,
		DocumentType:  documentType,
		DocumentID:    documentID,
		Amount:        amount,
		RequiredLevel: requiredLevel,
	})
}

// GetRequest implements RequestStore.
func (r *Repository) GetRequest(ctx context.Context, id int64) (db.ApprovalRequest, error) {
	return r.queries.GetApprovalRequest(ctx, id)
}

// UpdateRequestStatus implements RequestStore.
func (r *Repository) UpdateRequestStatus(ctx context.Context, id int64, status string) (db.ApprovalRequest, error) {
	return r.queries.UpdateApprovalRequestStatus(ctx, db.UpdateApprovalRequestStatusParams{ID: id, Status: status})
}

// CreateStep implements RequestStore.
func (r *Repository) CreateStep(ctx context.Context, requestID int64, stepLevel int32, approverID int64) (db.ApprovalStep, error) {
	return r.queries.CreateApprovalStep(ctx, db.CreateApprovalStepParams{RequestID: requestID, StepLevel: stepLevel, AssignedApproverID: approverID})
}

// ListSteps implements RequestStore.
func (r *Repository) ListSteps(ctx context.Context, requestID int64) ([]db.ApprovalStep, error) {
	return r.queries.ListApprovalSteps(ctx, requestID)
}

// FindPendingStep implements RequestStore. Returns (nil, nil) if no step is pending.
func (r *Repository) FindPendingStep(ctx context.Context, requestID int64) (*db.ApprovalStep, error) {
	row, err := r.queries.FindPendingApprovalStep(ctx, requestID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// UpdateStepStatus implements RequestStore.
func (r *Repository) UpdateStepStatus(ctx context.Context, id int64, status string, actedByID int64, actedAt time.Time) (db.ApprovalStep, error) {
	return r.queries.UpdateApprovalStepStatus(ctx, db.UpdateApprovalStepStatusParams{
		ID:        id,
		Status:    status,
		ActedByID: &actedByID,
		ActedAt:   pgtype.Timestamptz{Time: actedAt, Valid: true},
	})
}

// InboxItem is one pending step directly assigned to the caller.
type InboxItem struct {
	StepID       int64
	RequestID    int64
	StepLevel    int32
	DocumentType string
	DocumentID   int64
	Amount       pgtype.Numeric
	MakerID      int64
}

// ListInboxForApprover returns the caller's pending steps (GET /inbox).
func (r *Repository) ListInboxForApprover(ctx context.Context, approverID int64) ([]InboxItem, error) {
	rows, err := r.queries.ListPendingStepsForApprover(ctx, approverID)
	if err != nil {
		return nil, err
	}
	items := make([]InboxItem, len(rows))
	for i, row := range rows {
		items[i] = InboxItem{
			StepID:       row.StepID,
			RequestID:    row.RequestID,
			StepLevel:    row.StepLevel,
			DocumentType: row.DocumentType,
			DocumentID:   row.DocumentID,
			Amount:       row.Amount,
			MakerID:      row.MakerID,
		}
	}
	return items, nil
}

// SetUserHierarchy implements AdminStore.
func (r *Repository) SetUserHierarchy(ctx context.Context, userID int64, supervisorID *int64, approvalLevel *int32) (db.UpdateUserHierarchyRow, error) {
	return r.queries.UpdateUserHierarchy(ctx, db.UpdateUserHierarchyParams{
		ID:            userID,
		SupervisorID:  supervisorID,
		ApprovalLevel: approvalLevel,
	})
}

// CreateDelegation implements AdminStore. Wraps a Postgres exclusion-violation
// (overlapping range) into ErrDelegationOverlap.
func (r *Repository) CreateDelegation(ctx context.Context, fromUserID, toUserID int64, startAt, endAt time.Time, reason *string) (db.ApprovalDelegation, error) {
	row, err := r.queries.CreateApprovalDelegation(ctx, db.CreateApprovalDelegationParams{
		FromUserID: fromUserID,
		ToUserID:   toUserID,
		StartAt:    pgtype.Timestamptz{Time: startAt, Valid: true},
		EndAt:      pgtype.Timestamptz{Time: endAt, Valid: true},
		Reason:     reason,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgExclusionViolation {
			return db.ApprovalDelegation{}, fmt.Errorf("from_user_id=%d: %w", fromUserID, ErrDelegationOverlap)
		}
		return db.ApprovalDelegation{}, err
	}
	return row, nil
}

// RevokeDelegation implements AdminStore. Returns (nil, nil) if the
// delegation had already ended (no row matched the WHERE end_at > $2 guard).
func (r *Repository) RevokeDelegation(ctx context.Context, id int64, revokedAt time.Time) (*db.ApprovalDelegation, error) {
	row, err := r.queries.RevokeApprovalDelegation(ctx, db.RevokeApprovalDelegationParams{
		ID:    id,
		EndAt: pgtype.Timestamptz{Time: revokedAt, Valid: true},
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// CreateLeave implements AdminStore.
func (r *Repository) CreateLeave(ctx context.Context, userID int64, startAt, endAt time.Time, reason *string) (db.UserLeave, error) {
	return r.queries.CreateUserLeave(ctx, db.CreateUserLeaveParams{
		UserID:  userID,
		StartAt: pgtype.Timestamptz{Time: startAt, Valid: true},
		EndAt:   pgtype.Timestamptz{Time: endAt, Valid: true},
		Reason:  reason,
	})
}
