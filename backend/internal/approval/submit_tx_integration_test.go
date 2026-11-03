//go:build integration

package approval

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/audit"
)

// TestSubmitForApproval_FailureLeavesNothing runs on the pool, as production does,
// so a rollback is a real one. Each failure comes before the commit; a retry of the
// same document must then create the request as if the first attempt never ran.
func TestSubmitForApproval_FailureLeavesNothing(t *testing.T) {
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

	rows, err := pool.Query(ctx, `SELECT id FROM users ORDER BY id LIMIT 3`)
	if err != nil {
		t.Fatalf("fetch seed users: %v", err)
	}
	users, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		t.Fatalf("scan seed users: %v", err)
	}
	if len(users) < 3 {
		t.Fatalf("need 3 seeded users, have %d", len(users))
	}
	maker, approver1, approver2 := users[0], users[1], users[2]
	const missingUser = int64(-1) // approval_steps.assigned_approver_id FK rejects it

	chainTo := func(lastApprover int64) *fakeChainRepository {
		return &fakeChainRepository{users: map[int64]ApproverInfo{
			maker:        {UserID: maker, SupervisorID: int64p(approver1)},
			approver1:    {UserID: approver1, SupervisorID: int64p(lastApprover), ApprovalLevel: int32p(2)},
			lastApprover: {UserID: lastApprover, ApprovalLevel: int32p(3)},
		}}
	}
	var amount pgtype.Numeric
	if err := amount.Scan("150000000.00"); err != nil { // seeded invoice policy: >= 100jt -> L3
		t.Fatalf("parse amount: %v", err)
	}

	cases := []struct {
		name  string
		chain *fakeChainRepository
		audit AuditWriter
	}{
		{name: "second step violates its FK", chain: chainTo(missingUser), audit: audit.NewWriter(pool)},
		{name: "audit write fails", chain: chainTo(approver2), audit: &fakeAuditWriter{forceErr: errors.New("audit down")}},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			docID := time.Now().UnixNano() + int64(i)
			doc := strconv.FormatInt(docID, 10)
			count := func(query string) int {
				t.Helper()
				var n int
				if err := pool.QueryRow(ctx, query, doc).Scan(&n); err != nil {
					t.Fatalf("count: %v", err)
				}
				return n
			}
			const (
				requests = `SELECT count(*) FROM approval_requests WHERE document_type = 'invoice' AND document_id = $1::bigint`
				steps    = `SELECT count(*) FROM approval_steps s JOIN approval_requests r ON r.id = s.request_id WHERE r.document_type = 'invoice' AND r.document_id = $1::bigint`
				audits   = `SELECT count(*) FROM audit_logs WHERE entity_type = 'approval_request' AND action = 'submit' AND after::text LIKE '%' || $1 || '%'`
			)
			t.Cleanup(func() {
				for _, q := range []string{
					`DELETE FROM audit_logs WHERE entity_type = 'approval_request' AND action = 'submit' AND after::text LIKE '%' || $1 || '%'`,
					`DELETE FROM approval_steps WHERE request_id IN (SELECT id FROM approval_requests WHERE document_type = 'invoice' AND document_id = $1::bigint)`,
					`DELETE FROM approval_requests WHERE document_type = 'invoice' AND document_id = $1::bigint`,
				} {
					if _, err := pool.Exec(ctx, q, doc); err != nil {
						t.Errorf("cleanup: %v", err)
					}
				}
			})

			repo := NewRepository(pool)
			orch := NewOrchestrator(repo, tc.chain, repo, tc.audit, nil)
			if _, created, err := orch.SubmitForApproval(ctx, maker, "invoice", docID, amount, "127.0.0.1"); err == nil || created {
				t.Fatalf("created = %v, err = %v; want a failure", created, err)
			}
			if r, s, a := count(requests), count(steps), count(audits); r != 0 || s != 0 || a != 0 {
				t.Fatalf("after failure: %d requests, %d steps, %d audits; want none", r, s, a)
			}

			retry := NewOrchestrator(repo, chainTo(approver2), repo, audit.NewWriter(pool), nil)
			req, created, err := retry.SubmitForApproval(ctx, maker, "invoice", docID, amount, "127.0.0.1")
			if err != nil || !created {
				t.Fatalf("retry: created = %v, err = %v; want a fresh request", created, err)
			}
			if pending, err := repo.FindPendingStep(ctx, req.ID); err != nil || pending == nil {
				t.Fatalf("retry: pending step = %v, err = %v; the request must be approvable", pending, err)
			}
			if r, s, a := count(requests), count(steps), count(audits); r != 1 || s != 2 || a != 1 {
				t.Errorf("after retry: %d requests, %d steps, %d audits; want 1, 2, 1", r, s, a)
			}
		})
	}
}
