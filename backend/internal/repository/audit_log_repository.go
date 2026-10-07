package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// timestamptzFromPtr is the inverse of timestamptzToPtr: converts a *time.Time
// into a pgtype.Timestamptz, producing an invalid (SQL NULL) zero value when
// nil, so optional date filters map cleanly onto sqlc.narg timestamptz args.
func timestamptzFromPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// AuditLogFilter holds the optional list/count filters for audit_logs reads.
// A nil field means "no filter on this field".
type AuditLogFilter struct {
	ActorID    *int64
	Action     *string
	EntityType *string
	EntityID   *int64
	DateFrom   *time.Time
	DateTo     *time.Time
}

// AuditLogRepository is the read-only repository over audit_logs backing the
// Audit Log Viewer. audit_logs stays append-only: no write method is defined
// here (see internal/audit.Writer for the sole INSERT path).
//
// ponytail: uses dbPool; swap for the dbRead pool when DATABASE_REPLICA_URL
// wiring lands (same TODO convention as cmd/api/main.go's other read services).
type AuditLogRepository struct {
	queries *db.Queries
}

// NewAuditLogRepository creates an AuditLogRepository wrapping the given database connection.
func NewAuditLogRepository(dbConn db.DBTX) *AuditLogRepository {
	return &AuditLogRepository{queries: db.New(dbConn)}
}

// List returns one page of audit_logs matching f, ordered by created_at DESC, id DESC.
func (r *AuditLogRepository) List(ctx context.Context, f AuditLogFilter, limit, offset int32) ([]db.AuditLog, error) {
	return r.queries.ListAuditLogs(ctx, db.ListAuditLogsParams{
		ActorID:    f.ActorID,
		Action:     f.Action,
		EntityType: f.EntityType,
		EntityID:   f.EntityID,
		DateFrom:   timestamptzFromPtr(f.DateFrom),
		DateTo:     timestamptzFromPtr(f.DateTo),
		Limit:      limit,
		Offset:     offset,
	})
}

// Count returns the total number of audit_logs rows matching f (mirrors List's
// filters, ignoring pagination), for the {page, page_size, total} response.
func (r *AuditLogRepository) Count(ctx context.Context, f AuditLogFilter) (int64, error) {
	return r.queries.CountAuditLogs(ctx, db.CountAuditLogsParams{
		ActorID:    f.ActorID,
		Action:     f.Action,
		EntityType: f.EntityType,
		EntityID:   f.EntityID,
		DateFrom:   timestamptzFromPtr(f.DateFrom),
		DateTo:     timestamptzFromPtr(f.DateTo),
	})
}

// GetByID retrieves a single audit_logs row by id, including before/after.
// Returns nil, nil if no matching row is found.
func (r *AuditLogRepository) GetByID(ctx context.Context, id int64) (*db.AuditLog, error) {
	row, err := r.queries.GetAuditLogByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}
