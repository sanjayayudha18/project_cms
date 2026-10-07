//go:build integration

package audit

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// TestWriter_Write verifies field mapping and that before/after JSON round-trip
// through Postgres, against a real DB transaction rolled back on cleanup —
// same harness convention as atm_portal_integration_test.go.
func TestWriter_Write(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}
	t.Cleanup(pool.Close)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { tx.Rollback(ctx) })

	var actorID int64
	if err := tx.QueryRow(ctx, "SELECT id FROM users LIMIT 1").Scan(&actorID); err != nil {
		t.Fatalf("fetch a seed user id: %v", err)
	}

	w := NewWriter(tx)

	before := map[string]string{"status": "pending"}
	after := map[string]string{"status": "approved"}

	err = w.Write(ctx, Entry{
		ActorID:    actorID,
		Action:     "approve",
		EntityType: "approval_request",
		EntityID:   42,
		Before:     before,
		After:      after,
		IP:         "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	var row db.AuditLog
	err = tx.QueryRow(ctx,
		"SELECT id, actor_id, action, entity_type, entity_id, before, after, ip, created_at FROM audit_logs WHERE entity_type=$1 AND entity_id=$2",
		"approval_request", int64(42),
	).Scan(&row.ID, &row.ActorID, &row.Action, &row.EntityType, &row.EntityID, &row.Before, &row.After, &row.IP, &row.CreatedAt)
	if err != nil {
		t.Fatalf("read back row: %v", err)
	}

	if row.ActorID != actorID {
		t.Errorf("ActorID = %d, want %d", row.ActorID, actorID)
	}
	if row.Action != "approve" {
		t.Errorf("Action = %q, want %q", row.Action, "approve")
	}
	if row.EntityType != "approval_request" {
		t.Errorf("EntityType = %q, want %q", row.EntityType, "approval_request")
	}
	if row.EntityID != 42 {
		t.Errorf("EntityID = %d, want 42", row.EntityID)
	}
	if row.IP == nil || *row.IP != "127.0.0.1" {
		t.Errorf("IP = %v, want 127.0.0.1", row.IP)
	}

	var gotBefore, gotAfter map[string]string
	if err := json.Unmarshal(row.Before, &gotBefore); err != nil {
		t.Fatalf("unmarshal before: %v", err)
	}
	if err := json.Unmarshal(row.After, &gotAfter); err != nil {
		t.Fatalf("unmarshal after: %v", err)
	}
	if gotBefore["status"] != "pending" {
		t.Errorf("before[status] = %q, want %q", gotBefore["status"], "pending")
	}
	if gotAfter["status"] != "approved" {
		t.Errorf("after[status] = %q, want %q", gotAfter["status"], "approved")
	}
}

// TestWriter_Write_NilBeforeAfter verifies Before/After are optional (nil -> SQL NULL).
func TestWriter_Write_NilBeforeAfter(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}
	t.Cleanup(pool.Close)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { tx.Rollback(ctx) })

	var actorID int64
	if err := tx.QueryRow(ctx, "SELECT id FROM users LIMIT 1").Scan(&actorID); err != nil {
		t.Fatalf("fetch a seed user id: %v", err)
	}

	w := NewWriter(tx)
	err = w.Write(ctx, Entry{
		ActorID:    actorID,
		Action:     "create",
		EntityType: "invoice",
		EntityID:   7,
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	var before, after []byte
	err = tx.QueryRow(ctx,
		"SELECT before, after FROM audit_logs WHERE entity_type=$1 AND entity_id=$2",
		"invoice", int64(7),
	).Scan(&before, &after)
	if err != nil {
		t.Fatalf("read back row: %v", err)
	}
	if before != nil {
		t.Errorf("before = %s, want NULL", before)
	}
	if after != nil {
		t.Errorf("after = %s, want NULL", after)
	}
}
