// Package audit appends audit_logs rows for backend-extapi. Like backend-exq's
// writer it takes the DBTX per call (write in the caller's transaction), and
// it also supports Before, which update/disable/enable need.
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cimb-niaga/cms/backend-extapi/internal/repository"
)

// ErrInvalidEntry is returned before any DB access when ActorID or Action is missing.
var ErrInvalidEntry = errors.New("audit: actor_id and action are required")

// Entry is one audit trail row. Nil Before/After are stored as NULL.
type Entry struct {
	ActorID    int64
	Action     string
	EntityType string
	EntityID   int64
	Before     any
	After      any
	IP         string
}

// Writer appends audit_logs rows. Insert only (append-only table).
type Writer struct{}

// NewWriter returns a Writer.
func NewWriter() *Writer { return &Writer{} }

func marshal(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

// Write appends one audit_logs row using q (a pool or a tx).
func (w *Writer) Write(ctx context.Context, q repository.DBTX, e Entry) error {
	if e.ActorID == 0 || e.Action == "" {
		return ErrInvalidEntry
	}
	before, err := marshal(e.Before)
	if err != nil {
		return fmt.Errorf("audit: marshal before: %w", err)
	}
	after, err := marshal(e.After)
	if err != nil {
		return fmt.Errorf("audit: marshal after: %w", err)
	}
	var ip *string
	if e.IP != "" {
		ip = &e.IP
	}
	if _, err := repository.New(q).InsertAuditLog(ctx, repository.InsertAuditLogParams{
		ActorID: e.ActorID, Action: e.Action, EntityType: e.EntityType, EntityID: e.EntityID,
		Before: before, After: after, IP: ip,
	}); err != nil {
		return fmt.Errorf("audit: insert audit_logs: %w", err)
	}
	return nil
}
