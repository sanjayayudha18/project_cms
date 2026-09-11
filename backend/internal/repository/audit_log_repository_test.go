//go:build integration

package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestAuditLogRepository seeds a small, uniquely-tagged set of audit_logs
// rows and exercises List/Count/GetByID against a real DB transaction
// rolled back on cleanup — same harness convention as internal/audit's
// writer_test.go. entityType is randomized-by-time so this test never
// collides with rows already in the table or with a concurrent test run.
func TestAuditLogRepository(t *testing.T) {
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

	var actorA, actorB int64
	if err := tx.QueryRow(ctx, "SELECT id FROM users ORDER BY id LIMIT 1").Scan(&actorA); err != nil {
		t.Fatalf("fetch seed user A id: %v", err)
	}
	if err := tx.QueryRow(ctx, "SELECT id FROM users ORDER BY id DESC LIMIT 1").Scan(&actorB); err != nil {
		t.Fatalf("fetch seed user B id: %v", err)
	}

	entityType := "audit_log_repo_test_" + time.Now().Format("150405.000000000")
	base := time.Date(2026, 1, 10, 8, 0, 0, 0, time.UTC)

	type seed struct {
		id        int64
		actorID   int64
		action    string
		entityID  int64
		createdAt time.Time
	}
	rows := []seed{
		{actorID: actorA, action: "create", entityID: 1, createdAt: base},
		{actorID: actorA, action: "update", entityID: 1, createdAt: base.Add(1 * time.Minute)},
		{actorID: actorB, action: "approve", entityID: 2, createdAt: base.Add(2 * time.Minute)},
		{actorID: actorA, action: "update", entityID: 2, createdAt: base.Add(3 * time.Minute)},
		{actorID: actorB, action: "reject", entityID: 3, createdAt: base.Add(4 * time.Minute)},
	}
	for i := range rows {
		err := tx.QueryRow(ctx,
			`INSERT INTO audit_logs (actor_id, action, entity_type, entity_id, before, after, ip, created_at)
			 VALUES ($1, $2, $3, $4, NULL, '{"k":"v"}'::jsonb, '10.0.0.1', $5)
			 RETURNING id`,
			rows[i].actorID, rows[i].action, entityType, rows[i].entityID, rows[i].createdAt,
		).Scan(&rows[i].id)
		if err != nil {
			t.Fatalf("seed row %d: %v", i, err)
		}
	}

	repo := NewAuditLogRepository(tx)

	t.Run("no filter returns all rows ordered created_at DESC, id DESC", func(t *testing.T) {
		got, err := repo.List(ctx, AuditLogFilter{EntityType: &entityType}, 100, 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != len(rows) {
			t.Fatalf("len(got) = %d, want %d", len(got), len(rows))
		}
		for i, r := range got {
			want := rows[len(rows)-1-i] // reverse insertion order
			if r.ID != want.id {
				t.Errorf("position %d: id = %d, want %d (created_at DESC, id DESC violated)", i, r.ID, want.id)
			}
		}
	})

	t.Run("actor_id filter", func(t *testing.T) {
		got, err := repo.List(ctx, AuditLogFilter{EntityType: &entityType, ActorID: &actorB}, 100, 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("len(got) = %d, want 2", len(got))
		}
		for _, r := range got {
			if r.ActorID != actorB {
				t.Errorf("ActorID = %d, want %d", r.ActorID, actorB)
			}
		}
	})

	t.Run("action filter", func(t *testing.T) {
		action := "update"
		got, err := repo.List(ctx, AuditLogFilter{EntityType: &entityType, Action: &action}, 100, 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("len(got) = %d, want 2", len(got))
		}
		for _, r := range got {
			if r.Action != "update" {
				t.Errorf("Action = %q, want update", r.Action)
			}
		}
	})

	t.Run("entity_type + entity_id combined filter (single entity history)", func(t *testing.T) {
		entityID := int64(1)
		got, err := repo.List(ctx, AuditLogFilter{EntityType: &entityType, EntityID: &entityID}, 100, 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("len(got) = %d, want 2", len(got))
		}
		for _, r := range got {
			if r.EntityID != 1 {
				t.Errorf("EntityID = %d, want 1", r.EntityID)
			}
		}
	})

	t.Run("date range boundaries are inclusive", func(t *testing.T) {
		from := rows[1].createdAt
		to := rows[3].createdAt
		got, err := repo.List(ctx, AuditLogFilter{EntityType: &entityType, DateFrom: &from, DateTo: &to}, 100, 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("len(got) = %d, want 3 (rows[1..3] inclusive)", len(got))
		}
		gotIDs := map[int64]bool{}
		for _, r := range got {
			gotIDs[r.ID] = true
		}
		for _, want := range []int64{rows[1].id, rows[2].id, rows[3].id} {
			if !gotIDs[want] {
				t.Errorf("expected row id %d in range result, got %v", want, gotIDs)
			}
		}
	})

	t.Run("Count matches List length for the same filter", func(t *testing.T) {
		f := AuditLogFilter{EntityType: &entityType, ActorID: &actorA}
		listed, err := repo.List(ctx, f, 100, 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		count, err := repo.Count(ctx, f)
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if int64(len(listed)) != count {
			t.Errorf("Count = %d, len(List) = %d, want equal", count, len(listed))
		}
	})

	t.Run("Count vs page consistency across pagination", func(t *testing.T) {
		f := AuditLogFilter{EntityType: &entityType}
		count, err := repo.Count(ctx, f)
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		page1, err := repo.List(ctx, f, 2, 0)
		if err != nil {
			t.Fatalf("List page1: %v", err)
		}
		page2, err := repo.List(ctx, f, 2, 2)
		if err != nil {
			t.Fatalf("List page2: %v", err)
		}
		page3, err := repo.List(ctx, f, 2, 4)
		if err != nil {
			t.Fatalf("List page3: %v", err)
		}
		total := len(page1) + len(page2) + len(page3)
		if int64(total) != count {
			t.Errorf("pages summed to %d rows, Count = %d, want equal", total, count)
		}
	})

	t.Run("GetByID returns before/after for an existing row", func(t *testing.T) {
		got, err := repo.GetByID(ctx, rows[3].id)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got == nil {
			t.Fatal("GetByID returned nil, want a row")
		}
		if got.Before != nil {
			t.Errorf("Before = %s, want nil (seeded NULL)", got.Before)
		}
		if got.After == nil {
			t.Fatal("After = nil, want seeded JSON")
		}
	})

	t.Run("GetByID returns nil for a non-existent id", func(t *testing.T) {
		got, err := repo.GetByID(ctx, -1)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got != nil {
			t.Errorf("GetByID(-1) = %+v, want nil", got)
		}
	})
}
