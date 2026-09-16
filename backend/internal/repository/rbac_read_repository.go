package repository

import (
	"context"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// RbacReadRepository is the read-only repository backing the RBAC settings
// menu's list views (user hierarchy, delegations, leaves, policies).
//
// ponytail: uses dbPool; swap for the dbRead pool when DATABASE_REPLICA_URL
// wiring lands (same TODO convention as AuditLogRepository / cmd/api/main.go).
type RbacReadRepository struct {
	queries *db.Queries
}

// NewRbacReadRepository creates an RbacReadRepository wrapping the given database connection.
func NewRbacReadRepository(dbConn db.DBTX) *RbacReadRepository {
	return &RbacReadRepository{queries: db.New(dbConn)}
}

// ListUserHierarchy returns every user's supervisor_id, approval_level, role, and auth_source.
func (r *RbacReadRepository) ListUserHierarchy(ctx context.Context) ([]db.ListUserHierarchyRow, error) {
	return r.queries.ListUserHierarchy(ctx)
}

// ListDelegations returns active (non-revoked) approval delegations, newest first.
func (r *RbacReadRepository) ListDelegations(ctx context.Context) ([]db.ApprovalDelegation, error) {
	return r.queries.ListDelegations(ctx)
}

// ListLeaves returns user leaves, newest first.
func (r *RbacReadRepository) ListLeaves(ctx context.Context) ([]db.UserLeave, error) {
	return r.queries.ListLeaves(ctx)
}

// ListApprovalPolicies returns approval policies ordered by document_type, min_amount.
func (r *RbacReadRepository) ListApprovalPolicies(ctx context.Context) ([]db.ApprovalPolicy, error) {
	return r.queries.ListApprovalPolicies(ctx)
}
