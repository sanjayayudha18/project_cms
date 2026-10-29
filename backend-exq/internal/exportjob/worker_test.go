package exportjob

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/cimb-niaga/cms/pkg/csvexport"
)

// wkStore is an in-memory jobStore with the same guards as the SQL (status='running').
type wkStore struct {
	mu        sync.Mutex
	queue     []Job
	status    map[int64]Status
	chunks    map[int64][][]byte
	progress  []int64
	succeeded map[int64][3]int64 // rows, bytes, chunks
	failedMsg map[int64]string
	released  []int64
	deleted   []int64

	progressErr error
	claims      int
}

func newWkStore(jobs ...Job) *wkStore {
	s := &wkStore{
		status: map[int64]Status{}, chunks: map[int64][][]byte{},
		succeeded: map[int64][3]int64{}, failedMsg: map[int64]string{},
	}
	for _, j := range jobs {
		s.add(j)
	}
	return s
}

func (s *wkStore) add(j Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue = append(s.queue, j)
	s.status[j.ID] = StatusQueued
}

func (s *wkStore) get(id int64) Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status[id]
}

func (s *wkStore) set(id int64, st Status) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status[id] = st
}

func (s *wkStore) InsertChunk(_ context.Context, id int64, seq int32, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if int(seq) != len(s.chunks[id]) {
		return errors.New("duplicate/out of order seq")
	}
	s.chunks[id] = append(s.chunks[id], data)
	return nil
}

func (s *wkStore) ClaimNext(context.Context) (Job, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claims++
	for len(s.queue) > 0 {
		j := s.queue[0]
		s.queue = s.queue[1:]
		if s.status[j.ID] == StatusQueued {
			s.status[j.ID] = StatusRunning
			return j, true, nil
		}
	}
	return Job{}, false, nil
}

func (s *wkStore) UpdateProgress(_ context.Context, id, rows int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.progressErr != nil {
		return 0, s.progressErr
	}
	if s.status[id] != StatusRunning {
		return 0, nil
	}
	s.progress = append(s.progress, rows)
	return 1, nil
}

func (s *wkStore) MarkSucceeded(_ context.Context, id, rows, b int64, c int32) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status[id] != StatusRunning {
		return 0, nil
	}
	s.status[id] = StatusSucceeded
	s.succeeded[id] = [3]int64{rows, b, int64(c)}
	return 1, nil
}

func (s *wkStore) MarkFailed(_ context.Context, id int64, msg string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status[id] != StatusRunning {
		return false, nil
	}
	s.status[id] = StatusFailed
	s.failedMsg[id] = msg
	delete(s.chunks, id)
	return true, nil
}

func (s *wkStore) Release(_ context.Context, id int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status[id] != StatusRunning {
		return false, nil
	}
	s.status[id] = StatusQueued
	s.released = append(s.released, id)
	delete(s.chunks, id)
	return true, nil
}

func (s *wkStore) DeleteChunks(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted = append(s.deleted, id)
	delete(s.chunks, id)
	return nil
}

func (s *wkStore) file(id int64) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return bytes.Join(s.chunks[id], nil)
}

type wkReader struct {
	mu              sync.Mutex
	begun, rollback int
	err             error
}

func (r *wkReader) Begin(context.Context) (Querier, func(), error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, nil, r.err
	}
	r.begun++
	return nil, func() { r.mu.Lock(); r.rollback++; r.mu.Unlock() }, nil
}

// fnProducer runs fn as the produce step.
type fnProducer struct {
	fn func(ctx context.Context, emit func([]string) error) error
}

func (fnProducer) Header() []string { return []string{"id", "name"} }
func (p fnProducer) Produce(ctx context.Context, _ Querier, _ Params, _ time.Time, emit func([]string) error) error {
	return p.fn(ctx, emit)
}

func rowsProducer(n int) Producer {
	return fnProducer{func(_ context.Context, emit func([]string) error) error {
		for i := range n {
			if err := emit([]string{strconv.Itoa(i), "=x"}); err != nil {
				return err
			}
		}
		return nil
	}}
}

func errProducer(err error) Producer {
	return fnProducer{func(context.Context, func([]string) error) error { return err }}
}

func blockProducer() Producer {
	return fnProducer{func(ctx context.Context, emit func([]string) error) error {
		if err := emit([]string{"1", "a"}); err != nil {
			return err
		}
		<-ctx.Done()
		return ctx.Err()
	}}
}

func testJob(id int64) Job {
	return Job{ID: id, Params: Params{Kind: KindVendor, Entity: "vendor"}}
}

func testWorker(t *testing.T, store *wkStore, reader *wkReader, p Producer) *Worker {
	t.Helper()
	w := newWorker(store, reader, map[Kind]map[string]Producer{KindVendor: {"vendor": p}},
		WorkerConfig{Workers: 1, PollInterval: time.Hour})
	w.log = slog.New(slog.NewTextHandler(io.Discard, nil))
	return w
}

func claimAndRun(t *testing.T, w *Worker, store *wkStore) Job {
	t.Helper()
	j, ok, err := store.ClaimNext(context.Background())
	if !ok || err != nil {
		t.Fatalf("claim: ok=%v err=%v", ok, err)
	}
	w.run(j)
	return j
}

func TestWorkerRunSuccess(t *testing.T) {
	store, reader := newWkStore(testJob(1)), &wkReader{}
	w := testWorker(t, store, reader, rowsProducer(3))
	claimAndRun(t, w, store)

	if got := store.get(1); got != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded", got)
	}
	var buf bytes.Buffer
	cw, _ := csvexport.NewWriter(&buf, []string{"id", "name"})
	for i := range 3 {
		_ = cw.Write([]string{strconv.Itoa(i), "=x"})
	}
	_ = cw.Flush()
	file := store.file(1)
	if !bytes.Equal(file, buf.Bytes()) {
		t.Fatalf("file = %q, want %q", file, buf.Bytes())
	}
	got := store.succeeded[1]
	if got != [3]int64{3, int64(len(file)), 1} {
		t.Fatalf("succeeded args = %v, want rows 3, bytes %d, chunks 1", got, len(file))
	}
	if reader.begun != 1 || reader.rollback != 1 {
		t.Fatalf("replica tx begun=%d rollback=%d, want 1/1", reader.begun, reader.rollback)
	}
}

func TestWorkerRunHeaderOnly(t *testing.T) {
	store := newWkStore(testJob(1))
	w := testWorker(t, store, &wkReader{}, rowsProducer(0))
	claimAndRun(t, w, store)
	if store.get(1) != StatusSucceeded || store.succeeded[1][0] != 0 || store.succeeded[1][2] != 1 {
		t.Fatalf("header-only: status %s args %v", store.get(1), store.succeeded[1])
	}
}

func TestWorkerRunFailures(t *testing.T) {
	cases := []struct {
		name    string
		p       Producer
		entity  string
		setup   func(*Worker, *wkReader)
		wantMsg string
	}{
		{name: "file too large", p: rowsProducer(5000), setup: func(w *Worker, _ *wkReader) { w.maxBytes = 100 }, wantMsg: MsgFileTooLarge},
		{name: "too many rows", p: rowsProducer(3), setup: func(w *Worker, _ *wkReader) { w.maxRows = 2 }, wantMsg: MsgTooManyRows},
		{name: "timeout", p: blockProducer(), setup: func(w *Worker, _ *wkReader) { w.jobTimeout = 20 * time.Millisecond }, wantMsg: MsgTimeout},
		{name: "recovery conflict sqlstate", p: errProducer(&pgconn.PgError{Code: "40001", Message: "x"}), wantMsg: MsgReplicaConflict},
		{name: "recovery conflict message", p: errProducer(errors.New("ERROR: canceling statement due to conflict with recovery")), wantMsg: MsgReplicaConflict},
		{name: "generic", p: errProducer(errors.New("boom secret-cell")), wantMsg: MsgExportFailed},
		{name: "unknown producer", p: rowsProducer(1), entity: "nope", wantMsg: MsgExportFailed},
		{name: "replica begin fails", p: rowsProducer(1), setup: func(_ *Worker, r *wkReader) { r.err = errors.New("no replica") }, wantMsg: MsgExportFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			j := testJob(7)
			if tc.entity != "" {
				j.Params.Entity = tc.entity
			}
			store, reader := newWkStore(j), &wkReader{}
			w := testWorker(t, store, reader, tc.p)
			if tc.setup != nil {
				tc.setup(w, reader)
			}
			claimAndRun(t, w, store)
			if got := store.get(7); got != StatusFailed {
				t.Fatalf("status = %s, want failed", got)
			}
			if got := store.failedMsg[7]; got != tc.wantMsg {
				t.Fatalf("message = %q, want %q", got, tc.wantMsg)
			}
			if len(store.chunks[7]) != 0 {
				t.Fatalf("chunks left after failure: %d", len(store.chunks[7]))
			}
			if reader.begun != reader.rollback {
				t.Fatalf("replica tx not rolled back: begun=%d rollback=%d", reader.begun, reader.rollback)
			}
		})
	}
}

func TestWorkerLogsNoCellData(t *testing.T) {
	store := newWkStore(testJob(1))
	w := testWorker(t, store, &wkReader{}, rowsProducer(2))
	var buf bytes.Buffer
	w.log = slog.New(slog.NewTextHandler(&buf, nil))
	claimAndRun(t, w, store)
	out := buf.String()
	for _, want := range []string{"export_job_started", "export_job_succeeded", "id=1", "kind=vendor", "entity=vendor", "rows=2", "duration_ms="} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Fatalf("log missing %q:\n%s", want, out)
		}
	}
	if bytes.Contains([]byte(out), []byte("'=x")) {
		t.Fatalf("log contains cell data:\n%s", out)
	}
}

func TestWorkerSucceedZeroRowsDeletesChunks(t *testing.T) {
	store := newWkStore(testJob(1))
	// The janitor fails the job while it is producing.
	p := fnProducer{func(_ context.Context, emit func([]string) error) error {
		store.set(1, StatusFailed)
		return emit([]string{"1", "a"})
	}}
	w := testWorker(t, store, &wkReader{}, p)
	claimAndRun(t, w, store)
	if store.get(1) != StatusFailed {
		t.Fatalf("status = %s, want failed (janitor's)", store.get(1))
	}
	if len(store.deleted) != 1 || store.deleted[0] != 1 || len(store.chunks[1]) != 0 {
		t.Fatalf("chunks not deleted: deleted=%v chunks=%d", store.deleted, len(store.chunks[1]))
	}
}

func TestWorkerProgress(t *testing.T) {
	t.Run("page boundaries", func(t *testing.T) {
		store := newWkStore(testJob(1))
		w := testWorker(t, store, &wkReader{}, rowsProducer(2*PageSize+5))
		claimAndRun(t, w, store)
		if len(store.progress) != 2 || store.progress[0] != PageSize || store.progress[1] != 2*PageSize {
			t.Fatalf("progress = %v", store.progress)
		}
		if store.succeeded[1][0] != 2*PageSize+5 {
			t.Fatalf("final rows = %d", store.succeeded[1][0])
		}
	})
	t.Run("write error does not fail the job", func(t *testing.T) {
		store := newWkStore(testJob(1))
		store.progressErr = errors.New("blip")
		w := testWorker(t, store, &wkReader{}, rowsProducer(PageSize+1))
		claimAndRun(t, w, store)
		if store.get(1) != StatusSucceeded {
			t.Fatalf("status = %s, want succeeded", store.get(1))
		}
	})
	t.Run("job lost stops early", func(t *testing.T) {
		store := newWkStore(testJob(1))
		emitted := 0
		p := fnProducer{func(_ context.Context, emit func([]string) error) error {
			for i := range 3 * PageSize {
				if i == PageSize-1 {
					store.set(1, StatusFailed) // janitor timeout
				}
				if err := emit([]string{"1", "a"}); err != nil {
					return err
				}
				emitted++
			}
			return nil
		}}
		w := testWorker(t, store, &wkReader{}, p)
		claimAndRun(t, w, store)
		if emitted >= 3*PageSize {
			t.Fatal("producer was not stopped")
		}
		if store.get(1) != StatusFailed {
			t.Fatalf("status = %s", store.get(1))
		}
	})
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestWorkerRunLoopDrainsQueue(t *testing.T) {
	store := newWkStore(testJob(1), testJob(2), testJob(3))
	w := testWorker(t, store, &wkReader{}, rowsProducer(1))
	w.cfg.Workers = 2
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()

	waitFor(t, "all jobs succeeded", func() bool {
		return store.get(1) == StatusSucceeded && store.get(2) == StatusSucceeded && store.get(3) == StatusSucceeded
	})
	cancel()
	<-done
}

func TestWorkerWake(t *testing.T) {
	store := newWkStore()
	w := testWorker(t, store, &wkReader{}, rowsProducer(1))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	waitFor(t, "first empty claim", func() bool { store.mu.Lock(); defer store.mu.Unlock(); return store.claims >= 1 })

	store.add(testJob(9))
	for range 5 {
		w.Wake() // never blocks, even when nobody is listening
	}
	waitFor(t, "woken job succeeded", func() bool { return store.get(9) == StatusSucceeded })
}

func TestWorkerShutdownReleasesRunningJob(t *testing.T) {
	store := newWkStore(testJob(1), testJob(2))
	w := testWorker(t, store, &wkReader{}, blockProducer())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	waitFor(t, "job running", func() bool { return store.get(1) == StatusRunning })

	cancel()
	w.Wait(20 * time.Millisecond)
	<-done

	if store.get(1) != StatusQueued || len(store.released) != 1 || len(store.chunks[1]) != 0 {
		t.Fatalf("status=%s released=%v chunks=%d", store.get(1), store.released, len(store.chunks[1]))
	}
	if store.get(2) != StatusQueued {
		t.Fatalf("second job claimed during shutdown: %s", store.get(2))
	}
}

func TestWorkerShutdownLetsJobFinishWithinGrace(t *testing.T) {
	store := newWkStore(testJob(1))
	started := make(chan struct{})
	p := fnProducer{func(_ context.Context, emit func([]string) error) error {
		close(started)
		time.Sleep(30 * time.Millisecond)
		return emit([]string{"1", "a"})
	}}
	w := testWorker(t, store, &wkReader{}, p)
	go w.Run(context.Background())
	<-started
	w.Wait(2 * time.Second) // Wait alone stops claiming; no ctx cancel needed
	if store.get(1) != StatusSucceeded || len(store.released) != 0 {
		t.Fatalf("status=%s released=%v", store.get(1), store.released)
	}
}

func TestWorkerRunAfterWaitReturns(t *testing.T) {
	store := newWkStore(testJob(1))
	w := testWorker(t, store, &wkReader{}, rowsProducer(1))
	w.Wait(time.Millisecond)
	w.Run(context.Background()) // must return at once
	if store.get(1) != StatusQueued {
		t.Fatalf("job claimed after Wait: %s", store.get(1))
	}
}

func TestFailureMessageShutdownNotTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := failureMessage(ctx, context.Canceled); got != MsgExportFailed {
		t.Fatalf("cancel mapped to %q", got)
	}
}
