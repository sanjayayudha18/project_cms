package approval

import (
	"context"
	"time"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// AdminStore is the write access the admin handler needs to manage the
// hierarchy, delegations, and leaves (RBAC-Setup Task 8). Admin-only by
// convention of the caller (RequireRoles at the route), not enforced here.
type AdminStore interface {
	// SetUserHierarchy sets a user's supervisor_id / approval_level.
	SetUserHierarchy(ctx context.Context, userID int64, supervisorID *int64, approvalLevel *int32) (db.UpdateUserHierarchyRow, error)
	// CreateDelegation creates a delegation. Returns ErrDelegationOverlap if
	// the range overlaps an existing delegation for the same fromUserID.
	CreateDelegation(ctx context.Context, fromUserID, toUserID int64, startAt, endAt time.Time, reason *string) (db.ApprovalDelegation, error)
	// RevokeDelegation shortens end_at to now (or the given time), preserving
	// the row for audit history. Returns (nil, nil) if it had already ended.
	RevokeDelegation(ctx context.Context, id int64, revokedAt time.Time) (*db.ApprovalDelegation, error)
	// CreateLeave records a leave window for a user.
	CreateLeave(ctx context.Context, userID int64, startAt, endAt time.Time, reason *string) (db.UserLeave, error)
}
