package approval

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/audit"
)

// Maker-checker rules of the Orchestrator (be-tech-debt C3): maker != checker,
// only the effective approver of the *current* step may act, delegation on
// leave, policy level decides chain length, reject stops everything. Built on
// the fakes in orchestrator_test.go / effective_approver_test.go.

func submitInvoice(t *testing.T, orch *Orchestrator, docID int64) int64 {
	t.Helper()
	req, _, err := orch.SubmitForApproval(context.Background(), 1, "invoice", docID, pgtype.Numeric{Valid: true}, "ip")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	return req.ID
}

// stepStatuses returns level -> status for the request's steps.
func stepStatuses(t *testing.T, store *fakeStore, requestID int64) map[int32]string {
	t.Helper()
	steps, _ := store.ListSteps(context.Background(), requestID)
	out := map[int32]string{}
	for _, s := range steps {
		out[s.StepLevel] = s.Status
	}
	return out
}

func TestOrchestrator_MakerCannotActOnOwnRequest(t *testing.T) {
	// Maker 1 -> 20 (L2) -> 30 (L3). Also the case where the L2 approver is on
	// leave and has delegated to the maker: the delegation must not let the
	// maker approve their own request.
	tests := []struct {
		name         string
		availability *fakeAvailabilityRepository
	}{
		{"maker acts directly", &fakeAvailabilityRepository{}},
		{"approver on leave delegated to maker", &fakeAvailabilityRepository{onLeave: map[int64]bool{20: true}, delegates: map[int64]int64{20: 1}}},
	}
	actions := map[string]func(*Orchestrator, int64) error{
		"approve": func(o *Orchestrator, req int64) error {
			_, err := o.Approve(context.Background(), req, 1, "ip")
			return err
		},
		"reject": func(o *Orchestrator, req int64) error {
			_, err := o.Reject(context.Background(), req, 1, "ip")
			return err
		},
	}
	for _, tt := range tests {
		for name, act := range actions {
			t.Run(tt.name+"/"+name, func(t *testing.T) {
				chain, store := twoLevelChain()
				auditW := &fakeAuditWriter{}
				orch := NewOrchestrator(store, chain, tt.availability, auditW, nil)
				reqID := submitInvoice(t, orch, 9100)

				if err := act(orch, reqID); !errors.Is(err, ErrNotAuthorized) {
					t.Fatalf("%s by maker: err = %v, want ErrNotAuthorized", name, err)
				}
				if got := stepStatuses(t, store, reqID); got[2] != "pending" || got[3] != "pending" {
					t.Fatalf("steps after refused %s = %v, want all pending", name, got)
				}
				if store.requests[reqID].Status != "pending" {
					t.Fatalf("request status = %q, want pending", store.requests[reqID].Status)
				}
				if len(auditW.entries) != 1 { // submit only
					t.Fatalf("audit entries = %d, want 1 (no audit for a refused action)", len(auditW.entries))
				}
			})
		}
	}
}

func TestOrchestrator_OnlyCurrentStepApproverMayAct(t *testing.T) {
	chain, store := twoLevelChain()
	orch := NewOrchestrator(store, chain, &fakeAvailabilityRepository{}, &fakeAuditWriter{}, nil)
	ctx := context.Background()
	reqID := submitInvoice(t, orch, 9101)

	// The L3 approver may not skip ahead while L2 is still pending.
	if _, err := orch.Approve(ctx, reqID, 30, "ip"); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("L3 approve while L2 pending: err = %v, want ErrNotAuthorized", err)
	}
	if _, err := orch.Reject(ctx, reqID, 30, "ip"); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("L3 reject while L2 pending: err = %v, want ErrNotAuthorized", err)
	}
	if got := stepStatuses(t, store, reqID); got[2] != "pending" || got[3] != "pending" {
		t.Fatalf("steps = %v, want all pending", got)
	}

	// After L2 approves, the L2 approver can no longer act on the L3 step.
	if _, err := orch.Approve(ctx, reqID, 20, "ip"); err != nil {
		t.Fatalf("L2 approve: %v", err)
	}
	if _, err := orch.Approve(ctx, reqID, 20, "ip"); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("L2 approver acting on L3 step: err = %v, want ErrNotAuthorized", err)
	}
}

func TestOrchestrator_DelegateActsWhileApproverOnLeave(t *testing.T) {
	chain, store := twoLevelChain()
	availability := &fakeAvailabilityRepository{onLeave: map[int64]bool{20: true}, delegates: map[int64]int64{20: 25}}
	auditW := &fakeAuditWriter{}
	at := time.Date(2026, 10, 15, 3, 0, 0, 0, time.UTC)
	orch := NewOrchestrator(store, chain, availability, auditW, func() time.Time { return at })
	ctx := context.Background()
	reqID := submitInvoice(t, orch, 9102)

	if _, err := orch.Approve(ctx, reqID, 20, "ip"); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("assigned approver on leave: err = %v, want ErrNotAuthorized (delegate must act)", err)
	}
	if _, err := orch.Approve(ctx, reqID, 25, "ip"); err != nil {
		t.Fatalf("delegate approve: %v", err)
	}

	steps, _ := store.ListSteps(ctx, reqID)
	for _, s := range steps {
		if s.StepLevel != 2 {
			continue
		}
		if s.Status != "approved" || s.ActedByID == nil || *s.ActedByID != 25 || !s.ActedAt.Time.Equal(at) {
			t.Fatalf("L2 step = %+v, want approved by delegate 25 at %v", s, at)
		}
		if s.AssignedApproverID != 20 {
			t.Fatalf("assigned approver = %d, want 20 (unchanged by delegation)", s.AssignedApproverID)
		}
	}
	if last := auditW.entries[len(auditW.entries)-1]; last.Action != "approve" || last.ActorID != 25 {
		t.Fatalf("last audit = %+v, want approve by actor 25", last)
	}
}

func TestOrchestrator_ApproverOnLeaveWithoutDelegateBlocksStep(t *testing.T) {
	chain, store := twoLevelChain()
	availability := &fakeAvailabilityRepository{onLeave: map[int64]bool{20: true}}
	orch := NewOrchestrator(store, chain, availability, &fakeAuditWriter{}, nil)
	reqID := submitInvoice(t, orch, 9103)

	for _, actor := range []int64{20, 30, 1} {
		if _, err := orch.Approve(context.Background(), reqID, actor, "ip"); !errors.Is(err, ErrApproverOnLeaveNoDelegate) {
			t.Fatalf("approve by %d with L2 approver on leave and no delegate: err = %v, want ErrApproverOnLeaveNoDelegate", actor, err)
		}
	}
	if got := stepStatuses(t, store, reqID); got[2] != "pending" {
		t.Fatalf("L2 step = %q, want pending", got[2])
	}
}

func TestOrchestrator_PolicyLevelDecidesChainLength(t *testing.T) {
	chain, _ := twoLevelChain()
	store := newFakeStore(map[string]int32{"vendor_request": 2})
	auditW := &fakeAuditWriter{}
	orch := NewOrchestrator(store, chain, &fakeAvailabilityRepository{}, auditW, nil)
	ctx := context.Background()

	req, _, err := orch.SubmitForApproval(ctx, 1, "vendor_request", 9104, pgtype.Numeric{Valid: true}, "ip")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if got := stepStatuses(t, store, req.ID); len(got) != 1 || got[2] != "pending" {
		t.Fatalf("steps = %v, want only the L2 step", got)
	}
	final, err := orch.Approve(ctx, req.ID, 20, "ip")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if final.Status != "approved" {
		t.Fatalf("status after the only step = %q, want approved", final.Status)
	}
	if got := auditW.entries[len(auditW.entries)-1].Action; got != "final_approve" {
		t.Fatalf("last audit action = %q, want final_approve", got)
	}
}

func TestOrchestrator_RejectAtLastLevelAfterEarlierApproval(t *testing.T) {
	chain, store := twoLevelChain()
	auditW := &fakeAuditWriter{}
	orch := NewOrchestrator(store, chain, &fakeAvailabilityRepository{}, auditW, nil)
	ctx := context.Background()
	reqID := submitInvoice(t, orch, 9105)

	if _, err := orch.Approve(ctx, reqID, 20, "ip"); err != nil {
		t.Fatalf("L2 approve: %v", err)
	}
	rejected, err := orch.Reject(ctx, reqID, 30, "ip")
	if err != nil {
		t.Fatalf("L3 reject: %v", err)
	}
	if rejected.Status != "rejected" {
		t.Fatalf("status = %q, want rejected", rejected.Status)
	}
	if got := stepStatuses(t, store, reqID); got[2] != "approved" || got[3] != "rejected" {
		t.Fatalf("steps = %v, want L2 approved, L3 rejected", got)
	}
	if _, err := orch.Reject(ctx, reqID, 30, "ip"); !errors.Is(err, ErrRequestNotPending) {
		t.Fatalf("second reject: err = %v, want ErrRequestNotPending", err)
	}
	wantActions := []string{"submit", "approve", "reject"}
	if len(auditW.entries) != len(wantActions) {
		t.Fatalf("audit entries = %d, want %d", len(auditW.entries), len(wantActions))
	}
	for i, want := range wantActions {
		if auditW.entries[i].Action != want {
			t.Fatalf("audit[%d] = %q, want %q", i, auditW.entries[i].Action, want)
		}
	}
}

var errBoom = errors.New("boom")

// failingAvailability fails the leave lookup.
type failingAvailability struct{}

func (failingAvailability) IsOnLeave(context.Context, int64, time.Time) (bool, error) {
	return false, errBoom
}
func (failingAvailability) FindActiveDelegate(context.Context, int64, time.Time) (*int64, error) {
	return nil, errBoom
}

// countingFailAudit fails the failOn-th write (1-based) and accepts the rest.
type countingFailAudit struct {
	failOn int
	writes int
}

func (c *countingFailAudit) Write(context.Context, audit.Entry) error {
	c.writes++
	if c.writes == c.failOn {
		return errBoom
	}
	return nil
}

func TestOrchestrator_ApproveRejectErrorBranches(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		// arm runs right before the action under test.
		arm    func(*Orchestrator, *fakeStore, *fakeAuditWriter)
		atL3   bool // approve L2 first, act as the L3 approver
		reject bool
	}{
		{name: "approve: availability lookup fails", arm: func(o *Orchestrator, _ *fakeStore, _ *fakeAuditWriter) { o.availability = failingAvailability{} }},
		{name: "approve: update step fails", arm: func(_ *Orchestrator, s *fakeStore, _ *fakeAuditWriter) { s.forceUpdateStepErr = errBoom }},
		{name: "approve: step audit fails", arm: func(_ *Orchestrator, _ *fakeStore, a *fakeAuditWriter) { a.forceErr = errBoom }},
		{name: "approve: list steps fails", arm: func(_ *Orchestrator, s *fakeStore, _ *fakeAuditWriter) { s.forceListStepsErr = errBoom }},
		{name: "approve: final request update fails", atL3: true, arm: func(_ *Orchestrator, s *fakeStore, _ *fakeAuditWriter) { s.forceUpdateRequestErr = errBoom }},
		{name: "reject: availability lookup fails", reject: true, arm: func(o *Orchestrator, _ *fakeStore, _ *fakeAuditWriter) { o.availability = failingAvailability{} }},
		{name: "reject: update step fails", reject: true, arm: func(_ *Orchestrator, s *fakeStore, _ *fakeAuditWriter) { s.forceUpdateStepErr = errBoom }},
		{name: "reject: request update fails", reject: true, arm: func(_ *Orchestrator, s *fakeStore, _ *fakeAuditWriter) { s.forceUpdateRequestErr = errBoom }},
		{name: "reject: audit fails", reject: true, arm: func(_ *Orchestrator, _ *fakeStore, a *fakeAuditWriter) { a.forceErr = errBoom }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain, store := twoLevelChain()
			auditW := &fakeAuditWriter{}
			orch := NewOrchestrator(store, chain, &fakeAvailabilityRepository{}, auditW, nil)
			reqID := submitInvoice(t, orch, 9106)
			actor := int64(20)
			if tt.atL3 {
				if _, err := orch.Approve(ctx, reqID, 20, "ip"); err != nil {
					t.Fatalf("L2 approve: %v", err)
				}
				actor = 30
			}
			tt.arm(orch, store, auditW)

			var err error
			if tt.reject {
				_, err = orch.Reject(ctx, reqID, actor, "ip")
			} else {
				_, err = orch.Approve(ctx, reqID, actor, "ip")
			}
			if !errors.Is(err, errBoom) {
				t.Fatalf("err = %v, want wrapped errBoom", err)
			}
		})
	}
}

func TestOrchestrator_FinalApproveAuditFailureIsReturned(t *testing.T) {
	chain, store := twoLevelChain()
	auditW := &countingFailAudit{failOn: 4} // submit, approve L2, approve L3, final_approve
	orch := NewOrchestrator(store, chain, &fakeAvailabilityRepository{}, auditW, nil)
	ctx := context.Background()
	reqID := submitInvoice(t, orch, 9107)
	if _, err := orch.Approve(ctx, reqID, 20, "ip"); err != nil {
		t.Fatalf("L2 approve: %v", err)
	}
	if _, err := orch.Approve(ctx, reqID, 30, "ip"); !errors.Is(err, errBoom) {
		t.Fatalf("final approve with failing audit: err = %v, want wrapped errBoom", err)
	}
}
