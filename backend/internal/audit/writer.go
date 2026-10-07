// Package audit writes append-only audit_logs rows. It has no HTTP or auth
// dependency: callers (handlers/services) resolve the actor and IP themselves
// (via middleware.GetAuthContext + chi's RealIP-normalized r.RemoteAddr) and
// pass them in on Entry.
package audit

import (
	"context"
	"encoding/json"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Entry is one audit trail row to write. Before/After are marshaled to JSON;
// leave them nil when there is no prior/resulting state to record.
type Entry struct {
	ActorID    int64
	Action     string
	EntityType string
	EntityID   int64
	Before     any
	After      any
	IP         string
}

// Writer appends audit_logs rows. There is no Update/Delete method by design —
// audit_logs is append-only (see migration 023).
type Writer struct {
	queries *db.Queries
}

// NewWriter creates a Writer wrapping the given database connection.
func NewWriter(dbConn db.DBTX) *Writer {
	return &Writer{queries: db.New(dbConn)}
}

// Write appends one audit_logs row.
func (w *Writer) Write(ctx context.Context, entry Entry) error {
	before, err := marshalOrNil(entry.Before)
	if err != nil {
		return err
	}
	after, err := marshalOrNil(entry.After)
	if err != nil {
		return err
	}

	var ip *string
	if entry.IP != "" {
		ip = &entry.IP
	}

	_, err = w.queries.CreateAuditLog(ctx, db.CreateAuditLogParams{
		ActorID:    entry.ActorID,
		Action:     entry.Action,
		EntityType: entry.EntityType,
		EntityID:   entry.EntityID,
		Before:     before,
		After:      after,
		IP:         ip,
	})
	return err
}

func marshalOrNil(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}
