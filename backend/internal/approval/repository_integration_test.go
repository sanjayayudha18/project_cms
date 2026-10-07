//go:build integration

package approval

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestRepository_CreateDelegation_OverlapRejected verifies the
// approval_delegations_no_overlap EXCLUDE constraint (migration 025) is
// surfaced as ErrDelegationOverlap, against a real DB transaction rolled
// back on cleanup — same harness convention as internal/audit's writer_test.go.
func TestRepository_CreateDelegation_OverlapRejected(t *testing.T) {
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

	var fromID, toID1, toID2 int64
	if err := tx.QueryRow(ctx, "SELECT id FROM users ORDER BY id LIMIT 1").Scan(&fromID); err != nil {
		t.Fatalf("fetch a seed user id: %v", err)
	}
	if err := tx.QueryRow(ctx, "SELECT id FROM users ORDER BY id OFFSET 1 LIMIT 1").Scan(&toID1); err != nil {
		t.Fatalf("fetch a second seed user id: %v", err)
	}
	if err := tx.QueryRow(ctx, "SELECT id FROM users ORDER BY id OFFSET 2 LIMIT 1").Scan(&toID2); err != nil {
		t.Fatalf("fetch a third seed user id: %v", err)
	}

	repo := NewRepository(tx)
	now := time.Now()

	_, err = repo.CreateDelegation(ctx, fromID, toID1, now, now.Add(5*24*time.Hour), nil)
	if err != nil {
		t.Fatalf("first delegation: %v", err)
	}

	// A failed statement aborts the rest of this Postgres transaction (25P02),
	// so the overlap check below must be the last statement in this tx —
	// the non-overlap follow-up check lives in its own tx/test instead.
	_, err = repo.CreateDelegation(ctx, fromID, toID2, now.Add(1*24*time.Hour), now.Add(10*24*time.Hour), nil)
	if err == nil {
		t.Fatalf("overlapping delegation: got nil error, want ErrDelegationOverlap")
	}
	if !errors.Is(err, ErrDelegationOverlap) {
		t.Errorf("overlapping delegation error = %v, want errors.Is(_, ErrDelegationOverlap)", err)
	}
}

// TestRepository_CreateDelegation_NonOverlappingSucceeds verifies two
// non-overlapping ranges for the same from_user_id are both accepted.
func TestRepository_CreateDelegation_NonOverlappingSucceeds(t *testing.T) {
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

	var fromID, toID1, toID2 int64
	if err := tx.QueryRow(ctx, "SELECT id FROM users ORDER BY id LIMIT 1").Scan(&fromID); err != nil {
		t.Fatalf("fetch a seed user id: %v", err)
	}
	if err := tx.QueryRow(ctx, "SELECT id FROM users ORDER BY id OFFSET 1 LIMIT 1").Scan(&toID1); err != nil {
		t.Fatalf("fetch a second seed user id: %v", err)
	}
	if err := tx.QueryRow(ctx, "SELECT id FROM users ORDER BY id OFFSET 2 LIMIT 1").Scan(&toID2); err != nil {
		t.Fatalf("fetch a third seed user id: %v", err)
	}

	repo := NewRepository(tx)
	now := time.Now()

	if _, err := repo.CreateDelegation(ctx, fromID, toID1, now, now.Add(5*24*time.Hour), nil); err != nil {
		t.Fatalf("first delegation: %v", err)
	}
	if _, err := repo.CreateDelegation(ctx, fromID, toID2, now.Add(6*24*time.Hour), now.Add(10*24*time.Hour), nil); err != nil {
		t.Errorf("non-overlapping delegation: %v", err)
	}
}

// TestRepository_RevokeDelegation verifies "cabut" shortens end_at rather
// than deleting, and is a no-op (nil, nil) for an already-ended delegation.
func TestRepository_RevokeDelegation(t *testing.T) {
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

	var fromID, toID int64
	if err := tx.QueryRow(ctx, "SELECT id FROM users ORDER BY id LIMIT 1").Scan(&fromID); err != nil {
		t.Fatalf("fetch a seed user id: %v", err)
	}
	if err := tx.QueryRow(ctx, "SELECT id FROM users ORDER BY id OFFSET 1 LIMIT 1").Scan(&toID); err != nil {
		t.Fatalf("fetch a second seed user id: %v", err)
	}

	repo := NewRepository(tx)
	now := time.Now()

	created, err := repo.CreateDelegation(ctx, fromID, toID, now, now.Add(5*24*time.Hour), nil)
	if err != nil {
		t.Fatalf("create delegation: %v", err)
	}

	revoked, err := repo.RevokeDelegation(ctx, created.ID, now.Add(1*time.Hour))
	if err != nil {
		t.Fatalf("revoke delegation: %v", err)
	}
	if revoked == nil {
		t.Fatalf("revoke delegation: got nil, want a shortened row")
	}
	// Postgres timestamptz has microsecond precision, so compare truncated to
	// that rather than exact nanosecond equality.
	wantEndAt := now.Add(1 * time.Hour).Truncate(time.Microsecond)
	if !revoked.EndAt.Time.Truncate(time.Microsecond).Equal(wantEndAt) {
		t.Errorf("EndAt = %v, want %v", revoked.EndAt.Time, wantEndAt)
	}

	// Second revoke on an already-ended delegation is a no-op.
	again, err := repo.RevokeDelegation(ctx, created.ID, now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("second revoke: %v", err)
	}
	if again != nil {
		t.Errorf("second revoke = %+v, want nil (already ended)", again)
	}
}

// TestRepository_ListInboxForApprover verifies the inbox only surfaces
// pending steps directly assigned to the caller on pending requests.
func TestRepository_ListInboxForApprover(t *testing.T) {
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

	var makerID, approverID int64
	if err := tx.QueryRow(ctx, "SELECT id FROM users ORDER BY id LIMIT 1").Scan(&makerID); err != nil {
		t.Fatalf("fetch a seed user id: %v", err)
	}
	if err := tx.QueryRow(ctx, "SELECT id FROM users ORDER BY id OFFSET 1 LIMIT 1").Scan(&approverID); err != nil {
		t.Fatalf("fetch a second seed user id: %v", err)
	}

	repo := NewRepository(tx)

	var amount pgtype.Numeric
	if err := amount.Scan("100000000.00"); err != nil {
		t.Fatalf("parse amount: %v", err)
	}
	req, err := repo.CreateRequest(ctx, makerID, "invoice", 900555, amount, 2)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if _, err := repo.CreateStep(ctx, req.ID, 2, approverID); err != nil {
		t.Fatalf("create step: %v", err)
	}

	items, err := repo.ListInboxForApprover(ctx, approverID)
	if err != nil {
		t.Fatalf("list inbox: %v", err)
	}

	found := false
	for _, item := range items {
		if item.RequestID == req.ID {
			found = true
			if item.DocumentType != "invoice" || item.DocumentID != 900555 {
				t.Errorf("inbox item = %+v, want document_type=invoice document_id=900555", item)
			}
		}
	}
	if !found {
		t.Errorf("inbox for approver %d did not include request %d", approverID, req.ID)
	}
}
