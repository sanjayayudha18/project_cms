package approval

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

// fakeStore is an in-memory RequestStore for table-driven/scenario tests —
// no DB needed, per the project's "mock repos for unit tests" convention.
type fakeStore struct {
	nextRequestID  int64
	nextStepID     int64
	requests       map[int64]db.ApprovalRequest
	byDocument     map[string]int64
	steps          map[int64]db.ApprovalStep
	stepsByRequest map[int64][]int64
	policies       map[string]int32

	// error injection for exercising orchestrator error-handling branches
	forceFindRequestErr error
	forceCreateStepErr  error
}

func newFakeStore(policies map[string]int32) *fakeStore {
	return &fakeStore{
		requests:       map[int64]db.ApprovalRequest{},
		byDocument:     map[string]int64{},
		steps:          map[int64]db.ApprovalStep{},
		stepsByRequest: map[int64][]int64{},
		policies:       policies,
	}
}

func docKey(documentType string, documentID int64) string {
	return fmt.Sprintf("%s|%d", documentType, documentID)
}

func (f *fakeStore) FindPolicy(_ context.Context, documentType string, _ pgtype.Numeric) (int32, error) {
	level, ok := f.policies[documentType]
	if !ok {
		return 0, fmt.Errorf("no policy for document_type=%s", documentType)
	}
	return level, nil
}

func (f *fakeStore) FindRequestByDocument(_ context.Context, documentType string, documentID int64) (*db.ApprovalRequest, error) {
	if f.forceFindRequestErr != nil {
		return nil, f.forceFindRequestErr
	}
	id, ok := f.byDocument[docKey(documentType, documentID)]
	if !ok {
		return nil, nil
	}
	req := f.requests[id]
	return &req, nil
}

func (f *fakeStore) CreateRequest(_ context.Context, makerID int64, documentType string, documentID int64, amount pgtype.Numeric, requiredLevel int32) (db.ApprovalRequest, error) {
	f.nextRequestID++
	req := db.ApprovalRequest{
		ID:            f.nextRequestID,
		MakerID:       makerID,
		DocumentType:  documentType,
		DocumentID:    documentID,
		Amount:        amount,
		RequiredLevel: requiredLevel,
		Status:        "pending",
	}
	f.requests[req.ID] = req
	f.byDocument[docKey(documentType, documentID)] = req.ID
	return req, nil
}

func (f *fakeStore) GetRequest(_ context.Context, id int64) (db.ApprovalRequest, error) {
	req, ok := f.requests[id]
	if !ok {
		return db.ApprovalRequest{}, fmt.Errorf("request %d not found", id)
	}
	return req, nil
}

func (f *fakeStore) UpdateRequestStatus(_ context.Context, id int64, status string) (db.ApprovalRequest, error) {
	req, ok := f.requests[id]
	if !ok {
		return db.ApprovalRequest{}, fmt.Errorf("request %d not found", id)
	}
	req.Status = status
	f.requests[id] = req
	return req, nil
}

func (f *fakeStore) CreateStep(_ context.Context, requestID int64, stepLevel int32, approverID int64) (db.ApprovalStep, error) {
	if f.forceCreateStepErr != nil {
		return db.ApprovalStep{}, f.forceCreateStepErr
	}
	f.nextStepID++
	step := db.ApprovalStep{
		ID:                 f.nextStepID,
		RequestID:          requestID,
		StepLevel:          stepLevel,
		AssignedApproverID: approverID,
		Status:             "pending",
	}
	f.steps[step.ID] = step
	f.stepsByRequest[requestID] = append(f.stepsByRequest[requestID], step.ID)
	return step, nil
}

func (f *fakeStore) ListSteps(_ context.Context, requestID int64) ([]db.ApprovalStep, error) {
	var out []db.ApprovalStep
	for _, id := range f.stepsByRequest[requestID] {
		out = append(out, f.steps[id])
	}
	return out, nil
}

func (f *fakeStore) FindPendingStep(_ context.Context, requestID int64) (*db.ApprovalStep, error) {
	var found *db.ApprovalStep
	for _, id := range f.stepsByRequest[requestID] {
		s := f.steps[id]
		if s.Status == "pending" && (found == nil || s.StepLevel < found.StepLevel) {
			sc := s
			found = &sc
		}
	}
	return found, nil
}

func (f *fakeStore) UpdateStepStatus(_ context.Context, id int64, status string, actedByID int64, actedAt time.Time) (db.ApprovalStep, error) {
	s, ok := f.steps[id]
	if !ok {
		return db.ApprovalStep{}, fmt.Errorf("step %d not found", id)
	}
	s.Status = status
	s.ActedByID = &actedByID
	s.ActedAt = pgtype.Timestamptz{Time: actedAt, Valid: true}
	f.steps[id] = s
	return s, nil
}

// fakeAuditWriter records every entry written, for asserting "every
// transition writes audit_logs".
type fakeAuditWriter struct {
	entries  []audit.Entry
	forceErr error
}

func (f *fakeAuditWriter) Write(_ context.Context, entry audit.Entry) error {
	if f.forceErr != nil {
		return f.forceErr
	}
	f.entries = append(f.entries, entry)
	return nil
}

// twoLevelChain is the common test fixture: maker (1) -> supervisor 20
// (level 2) -> supervisor 30 (level 3). document_type "invoice" requires
// level 3, mirroring the 150jt example in task.md.
func twoLevelChain() (*fakeChainRepository, *fakeStore) {
	chain := &fakeChainRepository{users: map[int64]ApproverInfo{
		1:  {UserID: 1, SupervisorID: int64p(20)},
		20: {UserID: 20, SupervisorID: int64p(30), ApprovalLevel: int32p(2)},
		30: {UserID: 30, ApprovalLevel: int32p(3)},
	}}
	store := newFakeStore(map[string]int32{"invoice": 3})
	return chain, store
}

func TestOrchestrator_SubmitForApproval_DoubleSubmitNotDuplicated(t *testing.T) {
	chain, store := twoLevelChain()
	availability := &fakeAvailabilityRepository{}
	auditW := &fakeAuditWriter{}
	orch := NewOrchestrator(store, chain, availability, auditW, nil)

	ctx := context.Background()
	amount := pgtype.Numeric{Valid: true}

	first, firstCreated, err := orch.SubmitForApproval(ctx, 1, "invoice", 9001, amount, "127.0.0.1")
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	if !firstCreated {
		t.Errorf("first submit: created = false, want true")
	}

	second, secondCreated, err := orch.SubmitForApproval(ctx, 1, "invoice", 9001, amount, "127.0.0.1")
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}
	if secondCreated {
		t.Errorf("second submit: created = true, want false (already exists)")
	}

	if first.ID != second.ID {
		t.Errorf("second submit created a new request: first=%d second=%d", first.ID, second.ID)
	}
	if len(store.requests) != 1 {
		t.Errorf("requests table has %d rows, want 1", len(store.requests))
	}

	submitCount := 0
	for _, e := range auditW.entries {
		if e.Action == "submit" {
			submitCount++
		}
	}
	if submitCount != 1 {
		t.Errorf("submit audit entries = %d, want 1 (double submit must not write a second one)", submitCount)
	}
}

func TestOrchestrator_Approve_ClimbsToApproved(t *testing.T) {
	chain, store := twoLevelChain()
	availability := &fakeAvailabilityRepository{}
	auditW := &fakeAuditWriter{}
	orch := NewOrchestrator(store, chain, availability, auditW, nil)

	ctx := context.Background()
	req, _, err := orch.SubmitForApproval(ctx, 1, "invoice", 9002, pgtype.Numeric{Valid: true}, "127.0.0.1")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if req.RequiredLevel != 3 {
		t.Fatalf("required_level = %d, want 3", req.RequiredLevel)
	}

	afterL2, err := orch.Approve(ctx, req.ID, 20, "127.0.0.1")
	if err != nil {
		t.Fatalf("approve L2: %v", err)
	}
	if afterL2.Status != "pending" {
		t.Errorf("status after L2 approve = %q, want pending (L3 still outstanding)", afterL2.Status)
	}

	afterL3, err := orch.Approve(ctx, req.ID, 30, "127.0.0.1")
	if err != nil {
		t.Fatalf("approve L3: %v", err)
	}
	if afterL3.Status != "approved" {
		t.Errorf("status after L3 approve = %q, want approved", afterL3.Status)
	}

	wantActions := []string{"submit", "approve", "approve", "final_approve"}
	if len(auditW.entries) != len(wantActions) {
		t.Fatalf("audit entries = %d, want %d (%v)", len(auditW.entries), len(wantActions), auditW.entries)
	}
	for i, want := range wantActions {
		if auditW.entries[i].Action != want {
			t.Errorf("audit[%d].Action = %q, want %q", i, auditW.entries[i].Action, want)
		}
	}
}

func TestOrchestrator_Approve_MakerRejected(t *testing.T) {
	chain, store := twoLevelChain()
	availability := &fakeAvailabilityRepository{}
	auditW := &fakeAuditWriter{}
	orch := NewOrchestrator(store, chain, availability, auditW, nil)

	ctx := context.Background()
	req, _, err := orch.SubmitForApproval(ctx, 1, "invoice", 9003, pgtype.Numeric{Valid: true}, "127.0.0.1")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	if _, err := orch.Approve(ctx, req.ID, 1, "127.0.0.1"); err == nil {
		t.Fatalf("Approve() by maker (actor=1) = nil error, want rejection")
	}
}

func TestOrchestrator_Reject_Stops(t *testing.T) {
	chain, store := twoLevelChain()
	availability := &fakeAvailabilityRepository{}
	auditW := &fakeAuditWriter{}
	orch := NewOrchestrator(store, chain, availability, auditW, nil)

	ctx := context.Background()
	req, _, err := orch.SubmitForApproval(ctx, 1, "invoice", 9004, pgtype.Numeric{Valid: true}, "127.0.0.1")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	rejected, err := orch.Reject(ctx, req.ID, 20, "127.0.0.1")
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if rejected.Status != "rejected" {
		t.Errorf("status = %q, want rejected", rejected.Status)
	}

	// L3 step must never have been touched.
	steps, _ := store.ListSteps(ctx, req.ID)
	for _, s := range steps {
		if s.StepLevel == 3 && s.Status != "pending" {
			t.Errorf("L3 step status = %q, want still pending (reject must not cascade approval)", s.Status)
		}
	}

	// A subsequent approve attempt must be refused — the request is no longer pending.
	if _, err := orch.Approve(ctx, req.ID, 30, "127.0.0.1"); err == nil {
		t.Errorf("Approve() after reject = nil error, want error (request no longer pending)")
	}

	wantActions := []string{"submit", "reject"}
	if len(auditW.entries) != len(wantActions) {
		t.Fatalf("audit entries = %d, want %d (%v)", len(auditW.entries), len(wantActions), auditW.entries)
	}
}

func TestOrchestrator_SubmitForApproval_ErrorBranches(t *testing.T) {
	ctx := context.Background()
	amount := pgtype.Numeric{Valid: true}

	t.Run("store lookup error", func(t *testing.T) {
		chain, store := twoLevelChain()
		store.forceFindRequestErr = fmt.Errorf("db unavailable")
		orch := NewOrchestrator(store, chain, &fakeAvailabilityRepository{}, &fakeAuditWriter{}, nil)

		if _, _, err := orch.SubmitForApproval(ctx, 1, "invoice", 1, amount, "ip"); err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("no policy for document_type", func(t *testing.T) {
		chain, store := twoLevelChain()
		orch := NewOrchestrator(store, chain, &fakeAvailabilityRepository{}, &fakeAuditWriter{}, nil)

		_, _, err := orch.SubmitForApproval(ctx, 1, "unknown_type", 1, amount, "ip")
		if !errors.Is(err, ErrPolicyNotFound) {
			t.Fatalf("err = %v, want errors.Is(_, ErrPolicyNotFound)", err)
		}
	})

	t.Run("chain incomplete", func(t *testing.T) {
		chain := &fakeChainRepository{users: map[int64]ApproverInfo{
			1: {UserID: 1}, // no supervisor at all
		}}
		store := newFakeStore(map[string]int32{"invoice": 3})
		orch := NewOrchestrator(store, chain, &fakeAvailabilityRepository{}, &fakeAuditWriter{}, nil)

		if _, _, err := orch.SubmitForApproval(ctx, 1, "invoice", 1, amount, "ip"); err == nil {
			t.Fatalf("expected chain-incomplete error, got nil")
		}
	})

	t.Run("create step error", func(t *testing.T) {
		chain, store := twoLevelChain()
		store.forceCreateStepErr = fmt.Errorf("insert failed")
		orch := NewOrchestrator(store, chain, &fakeAvailabilityRepository{}, &fakeAuditWriter{}, nil)

		if _, _, err := orch.SubmitForApproval(ctx, 1, "invoice", 1, amount, "ip"); err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("audit write error", func(t *testing.T) {
		chain, store := twoLevelChain()
		auditW := &fakeAuditWriter{forceErr: fmt.Errorf("audit sink down")}
		orch := NewOrchestrator(store, chain, &fakeAvailabilityRepository{}, auditW, nil)

		if _, _, err := orch.SubmitForApproval(ctx, 1, "invoice", 1, amount, "ip"); err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}

func TestOrchestrator_Reject_NotAuthorized(t *testing.T) {
	chain, store := twoLevelChain()
	orch := NewOrchestrator(store, chain, &fakeAvailabilityRepository{}, &fakeAuditWriter{}, nil)

	ctx := context.Background()
	req, _, err := orch.SubmitForApproval(ctx, 1, "invoice", 9010, pgtype.Numeric{Valid: true}, "ip")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	// actor 999 is neither the assigned approver (20) nor the maker.
	if _, err := orch.Reject(ctx, req.ID, 999, "ip"); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("err = %v, want errors.Is(_, ErrNotAuthorized)", err)
	}
}

func TestOrchestrator_Approve_RequestNotFound(t *testing.T) {
	chain, store := twoLevelChain()
	orch := NewOrchestrator(store, chain, &fakeAvailabilityRepository{}, &fakeAuditWriter{}, nil)

	if _, err := orch.Approve(context.Background(), 99999, 1, "ip"); err == nil {
		t.Fatalf("expected error for nonexistent request, got nil")
	}
}

func TestOrchestrator_Approve_AlreadyApproved(t *testing.T) {
	chain, store := twoLevelChain()
	orch := NewOrchestrator(store, chain, &fakeAvailabilityRepository{}, &fakeAuditWriter{}, nil)

	ctx := context.Background()
	req, _, err := orch.SubmitForApproval(ctx, 1, "invoice", 9011, pgtype.Numeric{Valid: true}, "ip")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := orch.Approve(ctx, req.ID, 20, "ip"); err != nil {
		t.Fatalf("approve L2: %v", err)
	}
	if _, err := orch.Approve(ctx, req.ID, 30, "ip"); err != nil {
		t.Fatalf("approve L3: %v", err)
	}

	// Request is now "approved" -- a further approve attempt must be refused.
	if _, err := orch.Approve(ctx, req.ID, 30, "ip"); !errors.Is(err, ErrRequestNotPending) {
		t.Fatalf("err = %v, want errors.Is(_, ErrRequestNotPending)", err)
	}
}
