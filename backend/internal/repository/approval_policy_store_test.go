//go:build integration

package repository

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

func numericParam(t *testing.T, s string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		t.Fatalf("numericParam(%q): %v", s, err)
	}
	return n
}

// TestApprovalPolicyStore_CreateGetUpdate verifies the primary-pool
// create/get/update path, including exact-decimal round-tripping (project
// golden rule: money is numeric, never float) — against a real DB
// transaction rolled back on cleanup, same harness convention as
// audit_log_repository_test.go.
func TestApprovalPolicyStore_CreateGetUpdate(t *testing.T) {
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

	store := NewApprovalPolicyStore(tx)
	docType := "approval_policy_store_test_" + time.Now().Format("150405.000000000")

	created, err := store.Create(ctx, db.CreateApprovalPolicyParams{
		DocumentType:  docType,
		MinAmount:     numericParam(t, "1000.00"),
		MaxAmount:     numericParam(t, "50000000.50"),
		RequiredLevel: 2,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if minVal, _ := created.MinAmount.Value(); fmt.Sprintf("%v", minVal) != "1000.00" {
		t.Errorf("created.MinAmount = %v, want exact 1000.00", minVal)
	}
	if maxVal, _ := created.MaxAmount.Value(); fmt.Sprintf("%v", maxVal) != "50000000.50" {
		t.Errorf("created.MaxAmount = %v, want exact 50000000.50", maxVal)
	}

	got, err := store.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatalf("Get(%d) = nil, want the created policy", created.ID)
	}
	if got.DocumentType != docType || got.RequiredLevel != 2 {
		t.Errorf("Get(%d) = %+v, want document_type=%q required_level=2", created.ID, got, docType)
	}

	updated, err := store.Update(ctx, db.UpdateApprovalPolicyParams{
		ID:            created.ID,
		DocumentType:  docType,
		MinAmount:     numericParam(t, "2000.00"),
		MaxAmount:     numericParam(t, "60000000.75"),
		RequiredLevel: 3,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.RequiredLevel != 3 {
		t.Errorf("updated.RequiredLevel = %d, want 3", updated.RequiredLevel)
	}
	if minVal, _ := updated.MinAmount.Value(); fmt.Sprintf("%v", minVal) != "2000.00" {
		t.Errorf("updated.MinAmount = %v, want exact 2000.00", minVal)
	}

	afterUpdate, err := store.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if afterUpdate == nil || afterUpdate.RequiredLevel != 3 {
		t.Errorf("Get after update = %+v, want required_level=3", afterUpdate)
	}
}

// TestApprovalPolicyStore_Get_NotFound verifies Get returns (nil, nil) for a
// non-existent id, matching the AuthRepository.FindByUsername convention.
func TestApprovalPolicyStore_Get_NotFound(t *testing.T) {
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

	store := NewApprovalPolicyStore(tx)
	got, err := store.Get(ctx, -1)
	if err != nil {
		t.Fatalf("Get(-1): %v", err)
	}
	if got != nil {
		t.Errorf("Get(-1) = %+v, want nil", got)
	}
}
