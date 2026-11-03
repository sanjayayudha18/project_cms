//go:build integration

package approval

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

// auditFailingOn writes through a real writer, except for one action. The rows
// written before the failure are what a rollback has to undo.
type auditFailingOn struct {
	real   *audit.Writer
	action string
}

func (a auditFailingOn) Write(ctx context.Context, entry audit.Entry) error {
	if entry.Action == a.action {
		return errors.New("audit down")
	}
	return a.real.Write(ctx, entry)
}

func (a auditFailingOn) WriteTx(ctx context.Context, tx db.DBTX, entry audit.Entry) error {
	if entry.Action == a.action {
		return errors.New("audit down")
	}
	return a.real.WriteTx(ctx, tx, entry)
}

// TestDecision_AuditFailureLeavesNothing runs on the pool, as production does, so
// a rollback is a real one. A failed audit must leave the decision undone, and the
// same approver must then be able to make it again.
func TestDecision_AuditFailureLeavesNothing(t *testing.T) {
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
	maker, l2, l3 := users[0], users[1], users[2]

	chain := &fakeChainRepository{users: map[int64]ApproverInfo{
		maker: {UserID: maker, SupervisorID: int64p(l2)},
		l2:    {UserID: l2, SupervisorID: int64p(l3), ApprovalLevel: int32p(2)},
		l3:    {UserID: l3, ApprovalLevel: int32p(3)},
	}}
	var amount pgtype.Numeric
	if err := amount.Scan("150000000.00"); err != nil { // seeded invoice policy: >= 100jt -> L3
		t.Fatalf("parse amount: %v", err)
	}

	type decide = func(o *Orchestrator, requestID int64) (db.ApprovalRequest, error)
	approveAs := func(actor int64) decide {
		return func(o *Orchestrator, id int64) (db.ApprovalRequest, error) {
			return o.Approve(ctx, id, actor, "127.0.0.1")
		}
	}
	rejectAs := func(actor int64) decide {
		return func(o *Orchestrator, id int64) (db.ApprovalRequest, error) {
			return o.Reject(ctx, id, actor, "127.0.0.1")
		}
	}

	cases := []struct {
		name           string
		l2Done         bool // L2 already approved before the failing decision
		failAction     string
		decide         decide
		wantStatus     string
		wantStepsAfter []string
		wantAudits     []string // after the retry, this decision's audit rows only
	}{
		{name: "approve mid-chain", failAction: "approve", decide: approveAs(l2), wantStatus: "pending",
			wantStepsAfter: []string{"approved", "pending"}, wantAudits: []string{"approve"}},
		{name: "final approve, step audit fails", l2Done: true, failAction: "approve", decide: approveAs(l3), wantStatus: "approved",
			wantStepsAfter: []string{"approved", "approved"}, wantAudits: []string{"approve", "final_approve"}},
		{name: "final approve, request audit fails", l2Done: true, failAction: "final_approve", decide: approveAs(l3), wantStatus: "approved",
			wantStepsAfter: []string{"approved", "approved"}, wantAudits: []string{"approve", "final_approve"}},
		{name: "reject", failAction: "reject", decide: rejectAs(l2), wantStatus: "rejected",
			wantStepsAfter: []string{"rejected", "pending"}, wantAudits: []string{"reject"}},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := NewRepository(pool)
			real := audit.NewWriter(pool)
			orch := NewOrchestrator(repo, chain, repo, real, nil)

			req, created, err := orch.SubmitForApproval(ctx, maker, "invoice", time.Now().UnixNano()+int64(i), amount, "127.0.0.1")
			if err != nil || !created {
				t.Fatalf("submit: created = %v, err = %v", created, err)
			}
			const ofRequest = `(entity_type = 'approval_request' AND entity_id = $1) OR (entity_type = 'approval_step' AND entity_id IN (SELECT id FROM approval_steps WHERE request_id = $1))`
			t.Cleanup(func() {
				for _, q := range []string{
					`DELETE FROM audit_logs WHERE ` + ofRequest,
					`DELETE FROM approval_steps WHERE request_id = $1`,
					`DELETE FROM approval_requests WHERE id = $1`,
				} {
					if _, err := pool.Exec(ctx, q, req.ID); err != nil {
						t.Errorf("cleanup: %v", err)
					}
				}
			})
			if tc.l2Done {
				if _, err := orch.Approve(ctx, req.ID, l2, "127.0.0.1"); err != nil {
					t.Fatalf("L2 approve: %v", err)
				}
			}
			stepStatuses := func() []string {
				t.Helper()
				steps, err := repo.ListSteps(ctx, req.ID)
				if err != nil {
					t.Fatalf("list steps: %v", err)
				}
				out := make([]string, len(steps))
				for i, s := range steps {
					out[i] = s.Status
				}
				return out
			}
			requestStatus := func() string {
				t.Helper()
				r, err := repo.GetRequest(ctx, req.ID)
				if err != nil {
					t.Fatalf("get request: %v", err)
				}
				return r.Status
			}
			// Returns the decision's audit actions, not counting those from before it.
			var skip int
			decisionAudits := func() []string {
				t.Helper()
				rows, err := pool.Query(ctx, `SELECT action FROM audit_logs WHERE action <> 'submit' AND (`+ofRequest+`) ORDER BY id`, req.ID)
				if err != nil {
					t.Fatalf("list audits: %v", err)
				}
				actions, err := pgx.CollectRows(rows, pgx.RowTo[string])
				if err != nil {
					t.Fatalf("scan audits: %v", err)
				}
				return actions[skip:]
			}
			skip = len(decisionAudits())
			stepsBefore := stepStatuses()

			failing := NewOrchestrator(repo, chain, repo, auditFailingOn{real: real, action: tc.failAction}, nil)
			if _, err := tc.decide(failing, req.ID); err == nil {
				t.Fatal("decision with failing audit: want an error")
			}
			if got := stepStatuses(); !slices.Equal(got, stepsBefore) {
				t.Errorf("after failure: steps = %v, want %v", got, stepsBefore)
			}
			if got := requestStatus(); got != "pending" {
				t.Errorf("after failure: request = %s, want pending", got)
			}
			if got := decisionAudits(); len(got) != 0 {
				t.Errorf("after failure: audits = %v, want none", got)
			}

			if _, err := tc.decide(orch, req.ID); err != nil {
				t.Fatalf("retry: %v", err)
			}
			if got := stepStatuses(); !slices.Equal(got, tc.wantStepsAfter) {
				t.Errorf("after retry: steps = %v, want %v", got, tc.wantStepsAfter)
			}
			if got := requestStatus(); got != tc.wantStatus {
				t.Errorf("after retry: request = %s, want %s", got, tc.wantStatus)
			}
			if got := decisionAudits(); !slices.Equal(got, tc.wantAudits) {
				t.Errorf("after retry: audits = %v, want %v", got, tc.wantAudits)
			}
		})
	}
}
