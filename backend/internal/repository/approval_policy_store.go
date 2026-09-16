package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// ApprovalPolicyStore is the primary-pool write access for approval_policies,
// backing the RBAC settings menu's policy create/edit endpoints. Get returns
// (nil, nil) when no row matches (AuthRepository.FindByUsername convention),
// so the handler can read the before-state for the audit trail on update.
type ApprovalPolicyStore struct {
	queries *db.Queries
}

// NewApprovalPolicyStore creates an ApprovalPolicyStore wrapping the given database connection.
func NewApprovalPolicyStore(dbConn db.DBTX) *ApprovalPolicyStore {
	return &ApprovalPolicyStore{queries: db.New(dbConn)}
}

// Create inserts a new approval policy.
func (s *ApprovalPolicyStore) Create(ctx context.Context, arg db.CreateApprovalPolicyParams) (db.ApprovalPolicy, error) {
	return s.queries.CreateApprovalPolicy(ctx, arg)
}

// Get returns the approval policy by id, or (nil, nil) if it does not exist.
func (s *ApprovalPolicyStore) Get(ctx context.Context, id int64) (*db.ApprovalPolicy, error) {
	row, err := s.queries.GetApprovalPolicy(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// Update overwrites an existing approval policy's fields.
func (s *ApprovalPolicyStore) Update(ctx context.Context, arg db.UpdateApprovalPolicyParams) (db.ApprovalPolicy, error) {
	return s.queries.UpdateApprovalPolicy(ctx, arg)
}
