// Package audit writes append-only audit_logs rows for backend-exq. It mirrors
// backend/internal/audit but takes the DBTX per call so callers can write in
// the same transaction as the work being audited.
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
)

// ErrInvalidEntry is returned before any DB access when ActorID or Action is missing.
var ErrInvalidEntry = errors.New("audit: actor_id and action are required")

// Entry is one audit trail row. Before is always NULL (export jobs have no prior state).
type Entry struct {
	ActorID    int64
	Action     string
	EntityType string
	EntityID   int64
	After      map[string]any
	IP         string
}

// Writer appends audit_logs rows. Insert only, by design (append-only table).
type Writer struct{}

// NewWriter returns a Writer.
func NewWriter() *Writer { return &Writer{} }

// Write appends one audit_logs row using q (a pool or a tx).
func (w *Writer) Write(ctx context.Context, q repository.DBTX, e Entry) error {
	if e.ActorID == 0 || e.Action == "" {
		return ErrInvalidEntry
	}
	var after []byte
	if e.After != nil {
		b, err := json.Marshal(e.After)
		if err != nil {
			return fmt.Errorf("audit: marshal after: %w", err)
		}
		after = b
	}
	var ip *string
	if e.IP != "" {
		ip = &e.IP
	}
	if _, err := repository.New(q).InsertAuditLog(ctx, repository.InsertAuditLogParams{
		ActorID:    e.ActorID,
		Action:     e.Action,
		EntityType: e.EntityType,
		EntityID:   e.EntityID,
		After:      after,
		IP:         ip,
	}); err != nil {
		return fmt.Errorf("audit: insert audit_logs: %w", err)
	}
	return nil
}
