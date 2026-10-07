//go:build integration

package repository

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestRbacReadRepository_ListUserHierarchy verifies the users<->roles join
// surfaces supervisor_id/approval_level/role/auth_source for a known row,
// against a real DB transaction rolled back on cleanup — same harness
// convention as audit_log_repository_test.go.
func TestRbacReadRepository_ListUserHierarchy(t *testing.T) {
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

	var userID int64
	if err := tx.QueryRow(ctx, "SELECT id FROM users ORDER BY id LIMIT 1").Scan(&userID); err != nil {
		t.Fatalf("fetch a seed user id: %v", err)
	}
	level := int32(4)
	if _, err := tx.Exec(ctx, "UPDATE users SET supervisor_id = NULL, approval_level = $2 WHERE id = $1", userID, level); err != nil {
		t.Fatalf("seed approval_level: %v", err)
	}

	var wantRole, wantAuthSource, wantUsername, wantFullName string
	if err := tx.QueryRow(ctx,
		"SELECT r.role, u.auth_source, u.username, u.full_name FROM users u JOIN roles r ON r.id = u.role_id WHERE u.id = $1",
		userID,
	).Scan(&wantRole, &wantAuthSource, &wantUsername, &wantFullName); err != nil {
		t.Fatalf("fetch expected role/auth_source/username/full_name: %v", err)
	}

	repo := NewRbacReadRepository(tx)
	rows, err := repo.ListUserHierarchy(ctx)
	if err != nil {
		t.Fatalf("ListUserHierarchy: %v", err)
	}

	var found bool
	for _, row := range rows {
		if row.ID != userID {
			continue
		}
		found = true
		if row.ApprovalLevel == nil || *row.ApprovalLevel != level {
			t.Errorf("ApprovalLevel = %v, want %d", row.ApprovalLevel, level)
		}
		if row.Role != wantRole {
			t.Errorf("Role = %q, want %q", row.Role, wantRole)
		}
		if row.AuthSource != wantAuthSource {
			t.Errorf("AuthSource = %q, want %q", row.AuthSource, wantAuthSource)
		}
		if row.Username != wantUsername {
			t.Errorf("Username = %q, want %q", row.Username, wantUsername)
		}
		if row.FullName != wantFullName {
			t.Errorf("FullName = %q, want %q", row.FullName, wantFullName)
		}
	}
	if !found {
		t.Errorf("ListUserHierarchy did not include seeded user id=%d", userID)
	}
}

// TestRbacReadRepository_ListDelegations_OnlyActive verifies the read
// repository excludes delegations whose end_at has already passed.
func TestRbacReadRepository_ListDelegations_OnlyActive(t *testing.T) {
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

	now := time.Now()
	var activeID, endedID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO approval_delegations (from_user_id, to_user_id, start_at, end_at) VALUES ($1, $2, $3, $4) RETURNING id`,
		fromID, toID, now, now.Add(5*24*time.Hour),
	).Scan(&activeID); err != nil {
		t.Fatalf("seed active delegation: %v", err)
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO approval_delegations (from_user_id, to_user_id, start_at, end_at) VALUES ($1, $2, $3, $4) RETURNING id`,
		fromID, toID, now.Add(-10*24*time.Hour), now.Add(-5*24*time.Hour),
	).Scan(&endedID); err != nil {
		t.Fatalf("seed ended delegation: %v", err)
	}

	repo := NewRbacReadRepository(tx)
	rows, err := repo.ListDelegations(ctx)
	if err != nil {
		t.Fatalf("ListDelegations: %v", err)
	}

	var sawActive, sawEnded bool
	for _, row := range rows {
		if row.ID == activeID {
			sawActive = true
		}
		if row.ID == endedID {
			sawEnded = true
		}
	}
	if !sawActive {
		t.Errorf("ListDelegations did not include active delegation id=%d", activeID)
	}
	if sawEnded {
		t.Errorf("ListDelegations included already-ended delegation id=%d, want excluded", endedID)
	}
}

// TestRbacReadRepository_ListLeaves verifies a seeded leave round-trips.
func TestRbacReadRepository_ListLeaves(t *testing.T) {
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

	var userID int64
	if err := tx.QueryRow(ctx, "SELECT id FROM users ORDER BY id LIMIT 1").Scan(&userID); err != nil {
		t.Fatalf("fetch a seed user id: %v", err)
	}

	now := time.Now()
	var leaveID int64
	reason := "cuti tahunan"
	if err := tx.QueryRow(ctx,
		`INSERT INTO user_leaves (user_id, start_at, end_at, reason) VALUES ($1, $2, $3, $4) RETURNING id`,
		userID, now, now.Add(3*24*time.Hour), reason,
	).Scan(&leaveID); err != nil {
		t.Fatalf("seed leave: %v", err)
	}

	repo := NewRbacReadRepository(tx)
	rows, err := repo.ListLeaves(ctx)
	if err != nil {
		t.Fatalf("ListLeaves: %v", err)
	}

	var found bool
	for _, row := range rows {
		if row.ID != leaveID {
			continue
		}
		found = true
		if row.UserID != userID {
			t.Errorf("UserID = %d, want %d", row.UserID, userID)
		}
		if row.Reason == nil || *row.Reason != reason {
			t.Errorf("Reason = %v, want %q", row.Reason, reason)
		}
	}
	if !found {
		t.Errorf("ListLeaves did not include seeded leave id=%d", leaveID)
	}
}

// TestRbacReadRepository_ListApprovalPolicies_ExactMoney verifies a seeded
// policy's min/max amounts round-trip as exact numeric strings, not floats
// (project golden rule: money is numeric, never float) — same
// amount.Value()+fmt.Sprintf("%v", ...) path the handler layer uses to
// serialize money on the wire (see approval_handler.go's toApprovalRequestResponse).
func TestRbacReadRepository_ListApprovalPolicies_ExactMoney(t *testing.T) {
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

	docType := "rbac_read_repo_test_" + time.Now().Format("150405.000000000")
	var policyID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO approval_policies (document_type, min_amount, max_amount, required_level)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		docType, "1234.56", "999999999999.99", 3,
	).Scan(&policyID); err != nil {
		t.Fatalf("seed policy: %v", err)
	}

	repo := NewRbacReadRepository(tx)
	rows, err := repo.ListApprovalPolicies(ctx)
	if err != nil {
		t.Fatalf("ListApprovalPolicies: %v", err)
	}

	var found bool
	for _, row := range rows {
		if row.ID != policyID {
			continue
		}
		found = true
		minVal, _ := row.MinAmount.Value()
		maxVal, _ := row.MaxAmount.Value()
		if gotMin := fmt.Sprintf("%v", minVal); gotMin != "1234.56" {
			t.Errorf("MinAmount = %s, want exact 1234.56", gotMin)
		}
		if gotMax := fmt.Sprintf("%v", maxVal); gotMax != "999999999999.99" {
			t.Errorf("MaxAmount = %s, want exact 999999999999.99", gotMax)
		}
	}
	if !found {
		t.Errorf("ListApprovalPolicies did not include seeded policy id=%d", policyID)
	}
}
