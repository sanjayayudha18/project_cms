package exportjob

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
)

type fakeJanitorQ struct {
	mu      sync.Mutex
	calls   []string
	failArg repository.FailTimedOutExportJobsParams
	lease   float64
	errs    map[string]error
	noDL    bool // some call's ctx had no deadline
	ticks   chan struct{}
}

func (f *fakeJanitorQ) record(ctx context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, name)
	if _, dl := ctx.Deadline(); !dl {
		f.noDL = true
	}
	return f.errs[name]
}

func (f *fakeJanitorQ) FailTimedOutExportJobs(ctx context.Context, arg repository.FailTimedOutExportJobsParams) ([]int64, error) {
	f.mu.Lock()
	f.failArg = arg
	f.mu.Unlock()
	return []int64{1, 2}, f.record(ctx, "fail")
}

func (f *fakeJanitorQ) RecoverStaleDownloads(ctx context.Context, lease float64) (int64, error) {
	f.mu.Lock()
	f.lease = lease
	f.mu.Unlock()
	return 3, f.record(ctx, "recover")
}

func (f *fakeJanitorQ) ExpireExportJobs(ctx context.Context) ([]int64, error) {
	return []int64{4, 5, 6, 7}, f.record(ctx, "expire")
}

func (f *fakeJanitorQ) DeleteOrphanExportJobChunks(ctx context.Context) (int64, error) {
	err := f.record(ctx, "orphans")
	if f.ticks != nil {
		f.ticks <- struct{}{}
	}
	return 5, err
}

func (f *fakeJanitorQ) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func testJanitor(q janitorQueries, interval time.Duration, logOut io.Writer) *Janitor {
	j := newJanitor(q, interval)
	j.log = slog.New(slog.NewTextHandler(logOut, nil))
	return j
}

func TestJanitorTick_OrderArgsAndLog(t *testing.T) {
	q := &fakeJanitorQ{}
	var buf bytes.Buffer
	j := testJanitor(q, time.Minute, &buf)
	j.jobTimeout, j.downloadLease = 90*time.Second, 45*time.Second

	j.tick(context.Background())

	if got, want := strings.Join(q.snapshot(), ","), "fail,recover,expire,orphans"; got != want {
		t.Fatalf("order = %s, want %s", got, want)
	}
	if q.failArg.TimeoutSeconds != 90 || q.failArg.ErrorMessage != MsgTimeout {
		t.Errorf("fail arg = %+v", q.failArg)
	}
	if q.lease != 45 {
		t.Errorf("lease = %v, want 45", q.lease)
	}
	if q.noDL {
		t.Error("every step must run under its own deadline")
	}
	log := buf.String()
	for _, s := range []string{"export_janitor_tick", "failed=2", "recovered=3", "expired=4", "orphans=5", "ok=true"} {
		if !strings.Contains(log, s) {
			t.Errorf("log missing %q: %s", s, log)
		}
	}
}

func TestJanitorDefaults(t *testing.T) {
	j := newJanitor(&fakeJanitorQ{}, time.Second)
	if j.jobTimeout != JobTimeout || j.downloadLease != DownloadLease {
		t.Fatalf("defaults = %v/%v", j.jobTimeout, j.downloadLease)
	}
	q := &fakeJanitorQ{}
	j = testJanitor(q, time.Second, io.Discard)
	j.tick(context.Background())
	if q.failArg.TimeoutSeconds != 600 || q.lease != 300 {
		t.Errorf("seconds = %v/%v, want 600/300", q.failArg.TimeoutSeconds, q.lease)
	}
}

func TestJanitorTick_FailingStepDoesNotStopOthers(t *testing.T) {
	for _, bad := range []string{"fail", "recover", "expire", "orphans"} {
		t.Run(bad, func(t *testing.T) {
			q := &fakeJanitorQ{errs: map[string]error{bad: errors.New("boom")}}
			var buf bytes.Buffer
			testJanitor(q, time.Minute, &buf).tick(context.Background())

			if n := len(q.snapshot()); n != 4 {
				t.Fatalf("ran %d steps, want 4", n)
			}
			log := buf.String()
			if !strings.Contains(log, "export_janitor_step_failed") || !strings.Contains(log, "ok=false") {
				t.Errorf("failure not logged: %s", log)
			}
		})
	}
}

func TestJanitorRun_InitialTickThenTickerThenStop(t *testing.T) {
	q := &fakeJanitorQ{ticks: make(chan struct{}, 8)}
	j := testJanitor(q, 20*time.Millisecond, io.Discard)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { j.Run(ctx); close(done) }()

	for i := range 3 { // initial tick + two ticker ticks
		select {
		case <-q.ticks:
		case <-time.After(2 * time.Second):
			t.Fatalf("tick %d did not happen", i)
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after ctx cancel")
	}
}

func TestJanitorRun_InitialTickIsImmediate(t *testing.T) {
	q := &fakeJanitorQ{ticks: make(chan struct{}, 1)}
	j := testJanitor(q, time.Hour, io.Discard)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go j.Run(ctx)
	select {
	case <-q.ticks:
	case <-time.After(2 * time.Second):
		t.Fatal("no tick at start")
	}
}

func TestJanitorRun_CancelledCtxDoesNothing(t *testing.T) {
	q := &fakeJanitorQ{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	testJanitor(q, time.Millisecond, io.Discard).Run(ctx)
	if n := len(q.snapshot()); n != 0 {
		t.Fatalf("ran %d steps on a cancelled ctx", n)
	}
}
