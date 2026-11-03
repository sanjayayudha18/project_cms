package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

const (
	notifyBatchID   int64 = 5
	notifyMakerID   int64 = 7
	notifyActorID   int64 = 8
	notifyRequestID int64 = 99
	notifyHeadID    int64 = 10
)

// notifyRow is one master_data_change_requests row as the fake store holds it.
type notifyRow struct {
	Status string
	Error  string
}

// notifyRig is an in-memory approval world: the orchestrator, repo, pool, applier
// and audit writer the service needs. Tx writes land in the world only on commit,
// so a comparison of two rigs covers everything a run left behind.
type notifyRig struct {
	MasterDataApprovalOrchestrator

	rows         map[int64]notifyRow
	order        []int64
	batched      bool
	failApply    map[int64]error
	commitErr    error
	reqStatus    string
	docType      string
	orchErr      error
	auditErr     error
	failNotifyDB bool
	cancelAt     string
	cancel       context.CancelFunc

	audits    []string
	committed []string
	liveTx    *notifyTx
}

func newNotifyRig(batched bool, status string) *notifyRig {
	r := &notifyRig{rows: map[int64]notifyRow{}, batched: batched, failApply: map[int64]error{}, reqStatus: "approved", docType: masterDataDocumentType}
	r.order = []int64{notifyHeadID}
	if batched {
		r.order = []int64{notifyHeadID, 11, 12}
	}
	for _, id := range r.order {
		r.rows[id] = notifyRow{Status: status}
	}
	return r
}

func (r *notifyRig) service(n ImportNotifier) *MasterDataApprovalService {
	svc := NewMasterDataApprovalService(r, r, r, ApplierRegistry{"vendor": r}, r)
	if n != nil {
		svc = svc.WithImportNotifier(n)
	}
	return svc
}

func (r *notifyRig) hit(point string) {
	if r.cancelAt == point {
		r.cancel()
	}
}

func (r *notifyRig) request(err error) (db.ApprovalRequest, error) {
	r.hit("orchestrator")
	if err != nil {
		return db.ApprovalRequest{}, err
	}
	return db.ApprovalRequest{ID: notifyRequestID, MakerID: notifyMakerID, DocumentType: r.docType, DocumentID: notifyHeadID, Status: r.reqStatus}, nil
}

func (r *notifyRig) Approve(context.Context, int64, int64, string) (db.ApprovalRequest, error) {
	return r.request(r.orchErr)
}

func (r *notifyRig) Reject(context.Context, int64, int64, string) (db.ApprovalRequest, error) {
	return r.request(r.orchErr)
}

func (r *notifyRig) change(id int64) db.MasterDataChangeRequest {
	c := db.MasterDataChangeRequest{ID: id, EntityType: "vendor", Op: "create", MakerID: notifyMakerID, Status: r.rows[id].Status}
	if r.batched {
		batchID := notifyBatchID
		c.BatchID = &batchID
	}
	return c
}

// GetByID honours cancellation like a real query would; the notification load
// carries importNotifyTimeout's deadline, which is how failNotifyDB singles it out.
func (r *notifyRig) GetByID(ctx context.Context, id int64) (db.MasterDataChangeRequest, error) {
	if err := ctx.Err(); err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	if _, hasDeadline := ctx.Deadline(); hasDeadline && r.failNotifyDB {
		return db.MasterDataChangeRequest{}, errors.New("db down")
	}
	return r.change(id), nil
}

func (r *notifyRig) ListByBatch(context.Context, int64) ([]db.MasterDataChangeRequest, error) {
	out := make([]db.MasterDataChangeRequest, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.change(id))
	}
	return out, nil
}

func (r *notifyRig) MarkApproved(_ context.Context, id int64) error {
	r.rows[id] = notifyRow{Status: "approved"}
	return nil
}

func (r *notifyRig) MarkApplyFailed(_ context.Context, id int64, msg string) error {
	row := r.rows[id]
	row.Error = msg
	r.rows[id] = row
	return nil
}

func (r *notifyRig) MarkStale(_ context.Context, id int64, msg string) error {
	if r.liveTx != nil && !r.liveTx.closed {
		return fmt.Errorf("MarkStale(%d) while the apply tx still holds row locks", id)
	}
	r.rows[id] = notifyRow{Status: "stale", Error: msg}
	return nil
}

func (r *notifyRig) MarkRejected(_ context.Context, id int64) error {
	r.rows[id] = notifyRow{Status: "rejected"}
	return nil
}

func (r *notifyRig) Write(_ context.Context, e audit.Entry) error {
	r.hit("audit")
	if r.auditErr != nil {
		return r.auditErr
	}
	r.audits = append(r.audits, fmt.Sprintf("%d %s %s %d %s", e.ActorID, e.Action, e.EntityType, e.EntityID, e.IP))
	return nil
}

func (r *notifyRig) Begin(context.Context) (pgx.Tx, error) {
	r.liveTx = &notifyTx{rig: r}
	return r.liveTx, nil
}

func (r *notifyRig) Apply(_ context.Context, _ pgx.Tx, c db.MasterDataChangeRequest) (int64, any, error) {
	r.hit("apply")
	if err := r.failApply[c.ID]; err != nil {
		return 0, nil, err
	}
	return c.ID + 1000, map[string]int64{"id": c.ID}, nil
}

func (r *notifyRig) CurrentState(context.Context, pgx.Tx, int64) (any, error) {
	return nil, nil
}

// notifyTx answers the two statements applyOne runs and buffers them until commit.
type notifyTx struct {
	pgx.Tx
	rig     *notifyRig
	applied []int64
	audits  []string
	closed  bool
}

func (t *notifyTx) Exec(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
	t.applied = append(t.applied, args[0].(int64))
	return pgconn.CommandTag{}, nil
}

func (t *notifyTx) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	t.audits = append(t.audits, fmt.Sprintf("%v %v %v %v %s", args[0], args[1], args[2], args[3], args[5]))
	return notifyAuditRow{}
}

func (t *notifyTx) Commit(context.Context) error {
	t.rig.hit("commit")
	t.closed = true
	if t.rig.commitErr != nil {
		return t.rig.commitErr
	}
	for _, id := range t.applied {
		t.rig.rows[id] = notifyRow{Status: "applied"}
	}
	t.rig.committed = append(t.rig.committed, t.audits...)
	return nil
}

func (t *notifyTx) Rollback(context.Context) error {
	t.closed = true
	return nil
}

type notifyAuditRow struct{}

func (notifyAuditRow) Scan(...any) error { return nil }

// recordingNotifier records calls and the context each one received. before, when
// set, runs at the start of every call (to panic, stall or inspect the world).
type recordingNotifier struct {
	calls  []string
	ctxs   []context.Context
	before func()
}

func (n *recordingNotifier) record(ctx context.Context, name string, a, b int64) {
	n.calls = append(n.calls, fmt.Sprintf("%s(%d,%d)", name, a, b))
	n.ctxs = append(n.ctxs, ctx)
	if n.before != nil {
		n.before()
	}
}

func (n *recordingNotifier) BatchSubmitted(ctx context.Context, batchID, approvalRequestID int64) {
	n.record(ctx, "submitted", batchID, approvalRequestID)
}

func (n *recordingNotifier) BatchApplied(ctx context.Context, batchID, makerID int64) {
	n.record(ctx, "applied", batchID, makerID)
}

func (n *recordingNotifier) BatchRejected(ctx context.Context, batchID, makerID int64) {
	n.record(ctx, "rejected", batchID, makerID)
}

func (n *recordingNotifier) BatchApplyFailed(ctx context.Context, batchID, makerID int64) {
	n.record(ctx, "apply_failed", batchID, makerID)
}

// notifyScenario is one way through the service. run returns what the caller sees.
type notifyScenario struct {
	name       string
	rowStatus  string
	arrange    func(r *notifyRig)
	run        func(ctx context.Context, svc *MasterDataApprovalService) (db.ApprovalRequest, error)
	wantCall   string
	cancelAt   string
	wantErrSub string
}

func runApprove(ctx context.Context, svc *MasterDataApprovalService) (db.ApprovalRequest, error) {
	return svc.Approve(ctx, notifyRequestID, notifyActorID, "10.0.0.1")
}

func runReject(ctx context.Context, svc *MasterDataApprovalService) (db.ApprovalRequest, error) {
	return svc.Reject(ctx, notifyRequestID, notifyActorID, "10.0.0.1")
}

func runRetry(ctx context.Context, svc *MasterDataApprovalService) (db.ApprovalRequest, error) {
	return db.ApprovalRequest{}, svc.RetryApply(ctx, notifyHeadID, notifyActorID, "10.0.0.1")
}

func failHeadApply(r *notifyRig) { r.failApply[notifyHeadID] = errors.New("boom") }

func notifyScenarios() []notifyScenario {
	return []notifyScenario{
		{name: "approve with a next step", rowStatus: "pending", run: runApprove, cancelAt: "orchestrator",
			arrange: func(r *notifyRig) { r.reqStatus = "pending" }, wantCall: "submitted(5,99)"},
		{name: "approve applied", rowStatus: "pending", run: runApprove, cancelAt: "commit", wantCall: "applied(5,7)"},
		{name: "approve apply failed", rowStatus: "pending", run: runApprove, cancelAt: "apply",
			arrange: failHeadApply, wantCall: "apply_failed(5,7)", wantErrSub: "approved but apply failed"},
		{name: "reject", rowStatus: "pending", run: runReject, cancelAt: "audit", wantCall: "rejected(5,7)"},
		{name: "retry applied", rowStatus: "approved", run: runRetry, cancelAt: "commit", wantCall: "applied(5,7)"},
		{name: "retry apply failed", rowStatus: "approved", run: runRetry, cancelAt: "apply",
			arrange: failHeadApply, wantCall: "apply_failed(5,7)", wantErrSub: "boom"},
	}
}

func (sc notifyScenario) rig(batched bool) *notifyRig {
	r := newNotifyRig(batched, sc.rowStatus)
	if sc.arrange != nil {
		sc.arrange(r)
	}
	return r
}

func TestApprovalNotify_CallPerScenario(t *testing.T) {
	for _, sc := range notifyScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			n := &recordingNotifier{}
			_, err := sc.run(adminCtx(notifyActorID), sc.rig(true).service(n))
			checkScenarioErr(t, err, sc.wantErrSub)
			if !slices.Equal(n.calls, []string{sc.wantCall}) {
				t.Fatalf("notifier calls = %v, want [%s]", n.calls, sc.wantCall)
			}
		})
	}
}

func checkScenarioErr(t *testing.T, err error, wantSub string) {
	t.Helper()
	if wantSub == "" {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), wantSub) {
		t.Fatalf("err = %v, want it to contain %q", err, wantSub)
	}
}

func TestApprovalNotify_SingleRecordNeverNotifies(t *testing.T) {
	for _, sc := range notifyScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			n := &recordingNotifier{}
			_, err := sc.run(adminCtx(notifyActorID), sc.rig(false).service(n))
			checkScenarioErr(t, err, sc.wantErrSub)
			if len(n.calls) != 0 {
				t.Fatalf("notifier calls = %v, want none", n.calls)
			}
		})
	}
}

// A decision that did not go through must stay silent, and so must a document
// that is not master data at all.
func TestApprovalNotify_NoCallWhenNothingWasDecided(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name      string
		rowStatus string
		arrange   func(r *notifyRig)
		ctx       context.Context
		run       func(ctx context.Context, svc *MasterDataApprovalService) (db.ApprovalRequest, error)
		wantErr   error
	}{
		{"approve refused", "pending", func(r *notifyRig) { r.orchErr = boom }, adminCtx(notifyActorID), runApprove, boom},
		{"reject refused", "pending", func(r *notifyRig) { r.orchErr = boom }, adminCtx(notifyActorID), runReject, boom},
		{"reject audit failed", "pending", func(r *notifyRig) { r.auditErr = boom }, adminCtx(notifyActorID), runReject, boom},
		{"approve other document pending", "pending", func(r *notifyRig) { r.docType = "invoice"; r.reqStatus = "pending" }, adminCtx(notifyActorID), runApprove, nil},
		{"approve other document final", "pending", func(r *notifyRig) { r.docType = "invoice" }, adminCtx(notifyActorID), runApprove, nil},
		{"reject other document", "pending", func(r *notifyRig) { r.docType = "invoice" }, adminCtx(notifyActorID), runReject, nil},
		{"retry on a row not awaiting apply", "stale", nil, adminCtx(notifyActorID), runRetry, ErrMasterDataNotRetryable},
		{"retry without the admin role", "approved", nil, ctxWithRole(notifyActorID, "ATM-USER"), runRetry, ErrMasterDataForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNotifyRig(true, tt.rowStatus)
			if tt.arrange != nil {
				tt.arrange(r)
			}
			n := &recordingNotifier{}
			_, err := tt.run(tt.ctx, r.service(n))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if len(n.calls) != 0 {
				t.Fatalf("notifier calls = %v, want none", n.calls)
			}
		})
	}
}

func TestApprovalNotify_NilNotifierIsSafe(t *testing.T) {
	for _, sc := range notifyScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			_, err := sc.run(adminCtx(notifyActorID), sc.rig(true).service(nil))
			checkScenarioErr(t, err, sc.wantErrSub)
		})
		t.Run(sc.name+" explicit nil", func(t *testing.T) {
			svc := sc.rig(true).service(nil).WithImportNotifier(nil)
			_, err := sc.run(adminCtx(notifyActorID), svc)
			checkScenarioErr(t, err, sc.wantErrSub)
		})
	}
}

// The notification must describe a decision that is already durable.
func TestApprovalNotify_FiresAfterTheDecisionIsDurable(t *testing.T) {
	tests := []struct {
		name       string
		run        func(ctx context.Context, svc *MasterDataApprovalService) (db.ApprovalRequest, error)
		wantStatus string
		wantAudit  int
	}{
		{"applied", runApprove, "applied", 0},
		{"rejected", runReject, "rejected", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNotifyRig(true, "pending")
			n := &recordingNotifier{}
			n.before = func() {
				for _, id := range r.order {
					if r.rows[id].Status != tt.wantStatus {
						t.Errorf("row %d at notify time = %q, want %q", id, r.rows[id].Status, tt.wantStatus)
					}
				}
				if len(r.audits) != tt.wantAudit {
					t.Errorf("audit entries at notify time = %d, want %d", len(r.audits), tt.wantAudit)
				}
			}
			if _, err := tt.run(adminCtx(notifyActorID), r.service(n)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(n.calls) != 1 {
				t.Fatalf("notifier calls = %v, want one", n.calls)
			}
		})
	}
}

func TestApprovalNotify_RollbackMessages(t *testing.T) {
	const causeID, rollbackNote = int64(11), BatchRollbackPrefix + "change request 11 failed"
	tests := []struct {
		name      string
		applyErr  error
		wantCause string
	}{
		{"plain error", errors.New("duplicate terminal"), "apply change: duplicate terminal"},
		{"error text empty", errors.New(""), "apply change: "},
		{"error text starts with the prefix", errors.New(BatchRollbackPrefix + "weird"), "apply change: " + BatchRollbackPrefix + "weird"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNotifyRig(true, "pending")
			r.failApply[causeID] = tt.applyErr

			if _, err := runApprove(adminCtx(notifyActorID), r.service(&recordingNotifier{})); err == nil {
				t.Fatal("expected an apply error")
			}
			for _, id := range r.order {
				row := r.rows[id]
				if row.Status != "stale" {
					t.Errorf("row %d status = %q, want stale", id, row.Status)
				}
				want := rollbackNote
				if id == causeID {
					want = tt.wantCause
				}
				if row.Error != want {
					t.Errorf("row %d error = %q, want %q", id, row.Error, want)
				}
				if got := isFailedRow(&row.Error); got != (id == causeID) {
					t.Errorf("row %d isFailedRow = %v, want %v", id, got, id == causeID)
				}
			}
			if len(r.committed) != 0 {
				t.Errorf("rolled-back batch committed audit entries: %v", r.committed)
			}
		})
	}
}

func TestBatchCauseMessage_AlwaysReadsAsCause(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"keeps the original text", errors.New("duplicate terminal"), "duplicate terminal"},
		{"stale error unchanged", ErrMasterDataChangeStale, ErrMasterDataChangeStale.Error()},
		{"empty text", errors.New(""), "apply failed"},
		{"text that starts with the prefix", errors.New(BatchRollbackPrefix + "x"), "apply failed: " + BatchRollbackPrefix + "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := batchCauseMessage(tt.err)
			if got != tt.want {
				t.Fatalf("batchCauseMessage = %q, want %q", got, tt.want)
			}
			if !isFailedRow(&got) {
				t.Fatalf("isFailedRow(%q) = false, the row would read as rolled back", got)
			}
		})
	}
}

// notifyOutcome is everything a run leaves behind or hands back.
type notifyOutcome struct {
	Request   db.ApprovalRequest
	Err       string
	Rows      map[int64]notifyRow
	Audits    []string
	Committed []string
}

func runForOutcome(sc notifyScenario, n ImportNotifier, failNotifyDB bool) notifyOutcome {
	r := sc.rig(true)
	r.failNotifyDB = failNotifyDB
	req, err := sc.run(adminCtx(notifyActorID), r.service(n))
	out := notifyOutcome{Request: req, Rows: r.rows, Audits: r.audits, Committed: r.committed}
	if err != nil {
		out.Err = err.Error()
	}
	return out
}

// Property 9, unit level: whatever goes wrong while notifying, the caller's result,
// the stored rows and the audit trail equal a run whose notifier worked.
func TestApprovalNotify_Property9_FailureChangesNothing(t *testing.T) {
	disruptions := []struct {
		name         string
		notifier     func() *recordingNotifier
		failNotifyDB bool
		wantCalls    int
	}{
		{name: "notifier panics", notifier: func() *recordingNotifier {
			return &recordingNotifier{before: func() { panic("smtp exploded") }}
		}, wantCalls: 1},
		{name: "notifier is slow", notifier: func() *recordingNotifier {
			return &recordingNotifier{before: func() { time.Sleep(20 * time.Millisecond) }}
		}, wantCalls: 1},
		{name: "notification load fails", notifier: func() *recordingNotifier { return &recordingNotifier{} }, failNotifyDB: true},
	}
	for _, sc := range notifyScenarios() {
		baseline := runForOutcome(sc, &recordingNotifier{}, false)
		for _, d := range disruptions {
			t.Run(sc.name+"/"+d.name, func(t *testing.T) {
				n := d.notifier()
				got := runForOutcome(sc, n, d.failNotifyDB)
				if !reflect.DeepEqual(got, baseline) {
					t.Fatalf("outcome differs from the normal run\n got: %+v\nwant: %+v", got, baseline)
				}
				if len(n.calls) != d.wantCalls {
					t.Fatalf("notifier calls = %v, want %d", n.calls, d.wantCalls)
				}
			})
		}
	}
}

// A client that goes away mid-request must not cost the maker the notification.
func TestApprovalNotify_CancelledContextStillNotifies(t *testing.T) {
	for _, sc := range notifyScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(adminCtx(notifyActorID))
			defer cancel()
			r := sc.rig(true)
			r.cancelAt, r.cancel = sc.cancelAt, cancel
			n := &recordingNotifier{}
			// Read the notifier's ctx during the call: notifyBatch cancels it on return.
			var notifyCtxErr error
			var deadline time.Time
			var hasDeadline bool
			n.before = func() {
				notifyCtxErr = n.ctxs[0].Err()
				deadline, hasDeadline = n.ctxs[0].Deadline()
			}

			if _, err := sc.run(ctx, r.service(n)); (err != nil) != (sc.wantErrSub != "") {
				t.Fatalf("unexpected result: %v", err)
			}
			if ctx.Err() == nil {
				t.Fatalf("request ctx was never cancelled at %q", sc.cancelAt)
			}
			if !slices.Equal(n.calls, []string{sc.wantCall}) {
				t.Fatalf("notifier calls = %v, want [%s]", n.calls, sc.wantCall)
			}
			if notifyCtxErr != nil {
				t.Fatalf("notifier ctx err = %v, want a live ctx", notifyCtxErr)
			}
			if !hasDeadline || time.Until(deadline) > importNotifyTimeout {
				t.Fatalf("notifier ctx deadline = %v (set=%v), want within %v", deadline, hasDeadline, importNotifyTimeout)
			}
		})
	}
}
