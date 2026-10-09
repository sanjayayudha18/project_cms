//go:build integration

package repository

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// TestMasterDataChangeRepository covers the maker-checker staging row
// (CLAUDE.md Sec 12): pending idempotency guard, approval link, and every
// status transition, on a real DB transaction rolled back on cleanup.
func TestMasterDataChangeRepository(t *testing.T) {
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
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	var makerID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM users ORDER BY id LIMIT 1`).Scan(&makerID); err != nil {
		t.Skipf("no seeded user: %v", err)
	}
	repo := NewMasterDataChangeRepository(tx)
	entityType := "itest_entity" // unused by real screens, so counts below are exact
	create := func(entityID int64) db.MasterDataChangeRequest {
		t.Helper()
		row, err := repo.Create(ctx, db.CreateMasterDataChangeRequestParams{
			EntityType: entityType, EntityID: &entityID, Op: "update", Payload: []byte(`{"name":"x"}`), MakerID: makerID,
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		return row
	}
	status := func(id int64) (string, *string) {
		t.Helper()
		row, err := repo.GetByID(ctx, id)
		if err != nil {
			t.Fatalf("get %d: %v", id, err)
		}
		return row.Status, row.Error
	}

	first := create(1)
	if first.Status != "pending" || first.MakerID != makerID {
		t.Fatalf("created = %+v", first)
	}
	if id, err := repo.FindPending(ctx, entityType, 1); err != nil || id == nil || *id != first.ID {
		t.Fatalf("FindPending = %v, %v, want %d", id, err, first.ID)
	}
	if id, err := repo.FindPending(ctx, entityType, 2); err != nil || id != nil {
		t.Fatalf("FindPending other entity = %v, %v, want nil", id, err)
	}

	var approvalID int64
	if err := tx.QueryRow(ctx, `INSERT INTO approval_requests (maker_id, document_type, document_id, amount, required_level, status)
		VALUES ($1, 'master_data_change', $2, 0, 1, 'pending') RETURNING id`, makerID, first.ID).Scan(&approvalID); err != nil {
		t.Fatalf("seed approval: %v", err)
	}
	if err := repo.SetApprovalRequestID(ctx, first.ID, approvalID); err != nil {
		t.Fatal(err)
	}
	if row, _ := repo.GetByID(ctx, first.ID); row.ApprovalRequestID == nil || *row.ApprovalRequestID != approvalID {
		t.Fatalf("approval link = %v, want %d", row.ApprovalRequestID, approvalID)
	}

	// approved-but-not-applied: status stays approved, error recorded; no longer pending.
	if err := repo.MarkApproved(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkApplyFailed(ctx, first.ID, "boom"); err != nil {
		t.Fatal(err)
	}
	if s, e := status(first.ID); s != "approved" || e == nil || *e != "boom" {
		t.Fatalf("after apply failed = %s, %v", s, e)
	}
	if id, _ := repo.FindPending(ctx, entityType, 1); id != nil {
		t.Fatal("approved change still found as pending")
	}

	stale := create(1) // the entity is free for a new submit once the first left pending
	if err := repo.MarkStale(ctx, stale.ID, "changed since submit"); err != nil {
		t.Fatal(err)
	}
	if s, e := status(stale.ID); s != "stale" || e == nil {
		t.Fatalf("after stale = %s, %v", s, e)
	}
	rejected := create(3)
	if err := repo.MarkRejected(ctx, rejected.ID); err != nil {
		t.Fatal(err)
	}
	if s, _ := status(rejected.ID); s != "rejected" {
		t.Fatalf("after reject = %s", s)
	}

	et, st := entityType, "stale"
	if n, err := repo.Count(ctx, db.CountMasterDataChangeRequestsParams{EntityType: &et}); err != nil || n != 3 {
		t.Fatalf("Count(entity_type) = %d, %v, want 3", n, err)
	}
	rows, err := repo.List(ctx, db.ListMasterDataChangeRequestsParams{EntityType: &et, Status: &st, PageLimit: 10})
	if err != nil || len(rows) != 1 || rows[0].ID != stale.ID {
		t.Fatalf("List(stale) = %v, %v", rows, err)
	}
	if rows, err := repo.ListByBatch(ctx, -1); err != nil || len(rows) != 0 {
		t.Fatalf("ListByBatch(unknown) = %v, %v", rows, err)
	}
}
