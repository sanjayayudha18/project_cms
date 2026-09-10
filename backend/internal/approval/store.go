package approval

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

// RequestStore is the read/write access the orchestrator needs for
// approval_requests and approval_steps. FindRequestByDocument and
// FindPendingStep return (nil, nil) when no row matches, following the
// AuthRepository.FindByUsername convention already used in this codebase.
type RequestStore interface {
	FindPolicy(ctx context.Context, documentType string, amount pgtype.Numeric) (int32, error)
	FindRequestByDocument(ctx context.Context, documentType string, documentID int64) (*db.ApprovalRequest, error)
	CreateRequest(ctx context.Context, makerID int64, documentType string, documentID int64, amount pgtype.Numeric, requiredLevel int32) (db.ApprovalRequest, error)
	GetRequest(ctx context.Context, id int64) (db.ApprovalRequest, error)
	UpdateRequestStatus(ctx context.Context, id int64, status string) (db.ApprovalRequest, error)
	CreateStep(ctx context.Context, requestID int64, stepLevel int32, approverID int64) (db.ApprovalStep, error)
	ListSteps(ctx context.Context, requestID int64) ([]db.ApprovalStep, error)
	FindPendingStep(ctx context.Context, requestID int64) (*db.ApprovalStep, error)
	UpdateStepStatus(ctx context.Context, id int64, status string, actedByID int64, actedAt time.Time) (db.ApprovalStep, error)
}

// AuditWriter is the one method the orchestrator needs from audit.Writer,
// kept as an interface so tests can fake it without a real DB.
type AuditWriter interface {
	Write(ctx context.Context, entry audit.Entry) error
}
