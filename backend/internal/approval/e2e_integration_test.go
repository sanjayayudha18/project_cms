//go:build integration

package approval

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/audit"
)

// TestEndToEnd_ThreeLevelHierarchyWithDelegationOnLeave is the RBAC-Setup
// Task 8 demo: admin wires a 3-level hierarchy (maker -> L2 -> L3) + a
// delegation covering L2's leave, then runs the real Task 6/7 orchestrator
// end to end and confirms the request reaches "approved" via the delegate,
// not the absent L2 approver. Real Postgres, rolled back on cleanup.
func TestEndToEnd_ThreeLevelHierarchyWithDelegationOnLeave(t *testing.T) {
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

	// Four distinct seeded users: maker, L2 (goes on leave), L2's delegate, L3.
	var userIDs []int64
	rows, err := tx.Query(ctx, "SELECT id FROM users ORDER BY id LIMIT 4")
	if err != nil {
		t.Fatalf("fetch seed users: %v", err)
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan user id: %v", err)
		}
		userIDs = append(userIDs, id)
	}
	rows.Close()
	if len(userIDs) < 4 {
		t.Skip("need at least 4 seeded users for this demo")
	}
	makerID, l2ID, delegateID, l3ID := userIDs[0], userIDs[1], userIDs[2], userIDs[3]

	repo := NewRepository(tx)
	auditWriter := audit.NewWriter(tx)
	now := time.Now()

	// --- Admin: wire the 3-level hierarchy (Task 8) ---
	l2Level, l3Level := int32(2), int32(3)
	if _, err := repo.SetUserHierarchy(ctx, makerID, &l2ID, nil); err != nil {
		t.Fatalf("set maker hierarchy: %v", err)
	}
	if _, err := repo.SetUserHierarchy(ctx, l2ID, &l3ID, &l2Level); err != nil {
		t.Fatalf("set L2 hierarchy: %v", err)
	}
	if _, err := repo.SetUserHierarchy(ctx, l3ID, nil, &l3Level); err != nil {
		t.Fatalf("set L3 hierarchy: %v", err)
	}

	// --- Admin: L2 is on leave, delegate covers it (Task 8) ---
	if _, err := repo.CreateLeave(ctx, l2ID, now.Add(-1*time.Hour), now.Add(24*time.Hour), nil); err != nil {
		t.Fatalf("create leave: %v", err)
	}
	if _, err := repo.CreateDelegation(ctx, l2ID, delegateID, now.Add(-1*time.Hour), now.Add(24*time.Hour), nil); err != nil {
		t.Fatalf("create delegation: %v", err)
	}

	// --- Policy: this document_type/amount requires level 3, reusing the
	// seeded "invoice" policy from migration 022 (>=100jt -> L3). ---
	amount := pgtype.Numeric{}
	if err := amount.Scan("150000000.00"); err != nil {
		t.Fatalf("parse amount: %v", err)
	}

	orch := NewOrchestrator(repo, repo, repo, auditWriter, func() time.Time { return now })

	// --- Task 6/7: submit, then approve following the delegation ---
	docID := int64(900001) // synthetic id, tx is rolled back regardless
	req, created, err := orch.SubmitForApproval(ctx, makerID, "invoice", docID, amount, "127.0.0.1")
	if err != nil {
		t.Fatalf("SubmitForApproval: %v", err)
	}
	if !created {
		t.Fatalf("SubmitForApproval: created = false, want true")
	}
	if req.RequiredLevel != 3 {
		t.Fatalf("RequiredLevel = %d, want 3", req.RequiredLevel)
	}

	// L2's step must be approved by the delegate, not L2 (who is on leave).
	if _, err := orch.Approve(ctx, req.ID, l2ID, "127.0.0.1"); err == nil {
		t.Fatalf("Approve() by absent L2 = nil error, want rejection (L2 is on leave)")
	}
	afterL2, err := orch.Approve(ctx, req.ID, delegateID, "127.0.0.1")
	if err != nil {
		t.Fatalf("Approve() by delegate: %v", err)
	}
	if afterL2.Status != "pending" {
		t.Fatalf("status after delegate approves L2 = %q, want pending (L3 still outstanding)", afterL2.Status)
	}

	afterL3, err := orch.Approve(ctx, req.ID, l3ID, "127.0.0.1")
	if err != nil {
		t.Fatalf("Approve() by L3: %v", err)
	}
	if afterL3.Status != "approved" {
		t.Fatalf("final status = %q, want approved", afterL3.Status)
	}

	// Audit trail: submit, approve (delegate on L2's step), final_approve.
	var auditCount int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM audit_logs
		 WHERE (entity_type = 'approval_request' AND entity_id = $1)
		    OR (entity_type = 'approval_step' AND entity_id IN (
		           SELECT id FROM approval_steps WHERE request_id = $1 AND assigned_approver_id = $2
		        ))`,
		req.ID, l2ID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if auditCount < 3 {
		t.Errorf("audit_logs rows for this request/step = %d, want >= 3 (submit, approve, final_approve)", auditCount)
	}

	var actedByID *int64
	if err := tx.QueryRow(ctx, "SELECT acted_by_id FROM approval_steps WHERE request_id=$1 AND assigned_approver_id=$2", req.ID, l2ID).Scan(&actedByID); err != nil {
		t.Fatalf("read acted_by_id: %v", err)
	}
	if actedByID == nil || *actedByID != delegateID {
		t.Errorf("L2 step acted_by_id = %v, want delegate %d (trace of who actually acted while L2 was on leave)", actedByID, delegateID)
	}
}
