package exportjob

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
	"github.com/cimb-niaga/cms/pkg/csvexport"
)

// User-facing failure messages (Req 9.8, 9.9, 10.11). Raw errors go to logs only.
const (
	MsgFileTooLarge    = "File melebihi batas 100 MB"
	MsgTooManyRows     = "File melebihi batas 1.000.000 baris"
	MsgTimeout         = "Melewati batas waktu proses"
	MsgReplicaConflict = "Data sedang diperbarui, coba lagi"
	MsgExportFailed    = "Gagal membuat file ekspor"
)

// finalizeTimeout bounds the status/chunk writes made after a job's own context
// is done (timeout, shutdown), so a job is never left `running` by a dead context.
const finalizeTimeout = 5 * time.Second

var (
	errUnknownProducer = errors.New("no producer for kind/entity")
	// errJobLost: a progress update matched no running row (janitor already failed it).
	errJobLost = errors.New("job no longer running")
)

// jobStore is the primary-side state of export jobs. Every write goes to the primary (Req 6.3).
type jobStore interface {
	ChunkSink
	// ClaimNext moves the oldest queued job to running (SKIP LOCKED). ok=false: queue empty.
	ClaimNext(ctx context.Context) (job Job, ok bool, err error)
	UpdateProgress(ctx context.Context, id, rows int64) (int64, error)
	MarkSucceeded(ctx context.Context, id, rows, bytes int64, chunks int32) (int64, error)
	// MarkFailed and Release run in one tx: guarded UPDATE, then delete chunks only
	// when the row transitioned (a job someone else already moved keeps its chunks).
	MarkFailed(ctx context.Context, id int64, msg string) (bool, error)
	Release(ctx context.Context, id int64) (bool, error)
	DeleteChunks(ctx context.Context, id int64) error
}

// snapshotReader opens the read-only snapshot on the replica (Req 6.1, 6.2).
type snapshotReader interface {
	Begin(ctx context.Context) (q Querier, rollback func(), err error)
}

// Worker runs export jobs: cfg.Workers goroutines, each claim -> run -> repeat.
type Worker struct {
	store     jobStore
	reader    snapshotReader
	producers map[Kind]map[string]Producer
	cfg       WorkerConfig
	log       *slog.Logger
	now       func() time.Time

	jobTimeout time.Duration
	maxRows    int64
	maxBytes   int64

	wake chan struct{} // buffered 1: Wake never blocks

	// jobsCtx is the parent of every job context; cancelled only by Wait after the grace.
	jobsCtx    context.Context
	cancelJobs context.CancelFunc

	mu       sync.Mutex
	stopped  bool
	stopCh   chan struct{}
	stopOnce sync.Once
	loops    sync.WaitGroup
}

// NewWorker wires the worker: primary for job state and chunks, replica for master data
// (pass primary twice when there is no replica, Req 11.1).
func NewWorker(primary, replica *pgxpool.Pool, producers map[Kind]map[string]Producer, cfg WorkerConfig) *Worker {
	return newWorker(pgStore{pool: primary, q: repository.New(primary)}, pgReader{pool: replica}, producers, cfg)
}

func newWorker(store jobStore, reader snapshotReader, producers map[Kind]map[string]Producer, cfg WorkerConfig) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		store: store, reader: reader, producers: producers, cfg: cfg,
		log: slog.Default(), now: time.Now,
		jobTimeout: JobTimeout, maxRows: MaxRows, maxBytes: MaxBytes,
		wake:    make(chan struct{}, 1),
		jobsCtx: ctx, cancelJobs: cancel,
		stopCh: make(chan struct{}),
	}
}

// Wake asks an idle goroutine to claim now instead of at the next poll. Never blocks;
// pass it to NewService, which calls it after a Create commits.
func (w *Worker) Wake() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// Run starts cfg.Workers claim loops and blocks until they all exit. Cancelling ctx
// (or calling Wait) stops claiming; a job already claimed keeps running until Wait's grace.
func (w *Worker) Run(ctx context.Context) {
	w.mu.Lock()
	if w.stopped {
		w.mu.Unlock()
		return
	}
	n := max(w.cfg.Workers, 1)
	w.loops.Add(n)
	w.mu.Unlock()

	for range n {
		go func() {
			defer w.loops.Done()
			w.loop(ctx)
		}()
	}
	w.loops.Wait()
}

// Wait is the shutdown step (Req 9.10): stop claiming, give running jobs up to grace,
// then cancel the rest; a cancelled job is released to queued (chunks deleted) by its
// own goroutine, so it ends either succeeded/failed or released, never stuck running.
func (w *Worker) Wait(grace time.Duration) {
	w.mu.Lock()
	w.stopped = true
	w.mu.Unlock()
	w.stopOnce.Do(func() { close(w.stopCh) })

	done := make(chan struct{})
	go func() { w.loops.Wait(); close(done) }()

	t := time.NewTimer(grace)
	defer t.Stop()
	select {
	case <-done:
		return
	case <-t.C:
	}
	w.cancelJobs()
	select {
	case <-done:
	case <-time.After(finalizeTimeout + time.Second):
		// Only if a producer ignores ctx; the janitor fails it after JobTimeout.
		w.log.Warn("export_worker_shutdown_incomplete")
	}
}

func (w *Worker) shouldStop(ctx context.Context) bool {
	select {
	case <-w.stopCh:
		return true
	default:
		return ctx.Err() != nil
	}
}

func (w *Worker) loop(ctx context.Context) {
	for {
		if w.shouldStop(ctx) {
			return
		}
		// Background ctx: a claim committed while shutdown cancels ctx must still return
		// the job to us, or it would sit `running` until the janitor.
		cctx, cancel := context.WithTimeout(context.Background(), finalizeTimeout)
		job, ok, err := w.store.ClaimNext(cctx)
		cancel()
		if err != nil {
			w.log.Error("export_job_claim_failed", "error", err)
		}
		if ok {
			w.Wake() // another idle goroutine may take the next queued job
			w.run(job)
			continue
		}
		if !w.sleep(ctx) {
			return
		}
	}
}

// sleep waits PollInterval, a wake, or stop. false = stop.
func (w *Worker) sleep(ctx context.Context) bool {
	t := time.NewTimer(w.cfg.PollInterval)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-w.stopCh:
		return false
	case <-w.wake:
		return true
	case <-t.C:
		return true
	}
}

type runStats struct {
	rows, bytes int64
	chunks      int32
}

func (w *Worker) run(job Job) {
	start := time.Now()
	attrs := []any{"id", job.ID, "kind", string(job.Params.Kind), "entity", job.Params.Entity}
	w.log.Info("export_job_started", attrs...)

	ctx, cancel := context.WithTimeout(w.jobsCtx, w.jobTimeout)
	defer cancel()
	st, err := w.produce(ctx, job)

	fctx, fcancel := context.WithTimeout(context.Background(), finalizeTimeout)
	defer fcancel()
	if err == nil {
		err = w.succeed(fctx, job.ID, st)
		if err == nil {
			w.log.Info("export_job_succeeded", append(attrs, "duration_ms", time.Since(start).Milliseconds(),
				"rows", st.rows, "bytes", st.bytes, "chunks", st.chunks)...)
			return
		}
	}

	attrs = append(attrs, "duration_ms", time.Since(start).Milliseconds(), "rows", st.rows, "error", err.Error())
	if !errors.Is(err, ErrFileTooLarge) && !errors.Is(err, ErrTooManyRows) && w.jobsCtx.Err() != nil {
		if _, rerr := w.store.Release(fctx, job.ID); rerr != nil {
			w.log.Error("export_job_release_failed", append(attrs, "release_error", rerr.Error())...)
			return
		}
		w.log.Info("export_job_released", attrs...)
		return
	}
	msg := failureMessage(ctx, err)
	if _, ferr := w.store.MarkFailed(fctx, job.ID, msg); ferr != nil {
		// Still running in the DB; the janitor fails it after JobTimeout.
		attrs = append(attrs, "mark_error", ferr.Error())
	}
	w.log.Warn("export_job_failed", append(attrs, "message", msg)...)
}

// succeed marks the job succeeded. Zero rows = it is no longer ours (janitor failed it):
// drop the chunks we wrote after that.
func (w *Worker) succeed(ctx context.Context, id int64, st runStats) error {
	n, err := w.store.MarkSucceeded(ctx, id, st.rows, st.bytes, st.chunks)
	if err != nil {
		return fmt.Errorf("mark succeeded: %w", err)
	}
	if n == 0 {
		if err := w.store.DeleteChunks(ctx, id); err != nil {
			return fmt.Errorf("%w; delete chunks: %w", errJobLost, err)
		}
		return errJobLost
	}
	return nil
}

func (w *Worker) produce(ctx context.Context, job Job) (runStats, error) {
	var st runStats
	p, ok := w.producers[job.Params.Kind][job.Params.Entity]
	if !ok {
		return st, fmt.Errorf("%w: %s/%s", errUnknownProducer, job.Params.Kind, job.Params.Entity)
	}
	q, rollback, err := w.reader.Begin(ctx)
	if err != nil {
		return st, fmt.Errorf("begin replica snapshot: %w", err)
	}
	defer rollback() // read-only: nothing to commit

	asOf := w.now().In(wib)
	cw := NewChunkWriter(ctx, w.store, job.ID)
	cw.maxBytes = w.maxBytes
	out, err := csvexport.NewWriter(cw, p.Header())
	if err != nil {
		return st, err
	}
	emit := func(cells []string) error {
		if st.rows >= w.maxRows {
			return ErrTooManyRows
		}
		if err := out.Write(cells); err != nil {
			return err
		}
		st.rows++
		if st.rows%PageSize == 0 {
			return w.progress(ctx, job.ID, st.rows)
		}
		return nil
	}
	if err := p.Produce(ctx, q, job.Params, asOf, emit); err != nil {
		return st, err
	}
	if err := out.Flush(); err != nil {
		return st, err
	}
	if err := cw.Close(); err != nil {
		return st, err
	}
	st.bytes, st.chunks = cw.Bytes(), cw.Chunks()
	return st, nil
}

// progress writes row_count for the dialog (Req 12.8). A DB error only logs: progress is
// cosmetic and the final count is written on success, so a blip must not fail a long
// export. Zero rows means the janitor already failed the job, so stop working on it.
func (w *Worker) progress(ctx context.Context, id, rows int64) error {
	n, err := w.store.UpdateProgress(ctx, id, rows)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		w.log.Warn("export_job_progress_failed", "id", id, "rows", rows, "error", err.Error())
		return nil
	}
	if n == 0 {
		return errJobLost
	}
	return nil
}

// failureMessage maps a run error to the short message stored on the job.
func failureMessage(jobCtx context.Context, err error) string {
	switch {
	case errors.Is(err, ErrFileTooLarge):
		return MsgFileTooLarge
	case errors.Is(err, ErrTooManyRows):
		return MsgTooManyRows
	case errors.Is(jobCtx.Err(), context.DeadlineExceeded), errors.Is(err, context.DeadlineExceeded):
		return MsgTimeout
	case isRecoveryConflict(err):
		return MsgReplicaConflict
	default:
		return MsgExportFailed
	}
}

// isRecoveryConflict: a hot-standby query cancelled by WAL replay. A read-only
// REPEATABLE READ tx gets 40001 on the replica only from a recovery conflict.
func isRecoveryConflict(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "40001" {
		return true
	}
	return strings.Contains(err.Error(), "conflict with recovery")
}

// pgStore is jobStore on the primary pool.
type pgStore struct {
	pool *pgxpool.Pool
	q    *repository.Queries
}

func (s pgStore) InsertChunk(ctx context.Context, jobID int64, seq int32, data []byte) error {
	return QueriesSink{Q: s.q}.InsertChunk(ctx, jobID, seq, data)
}

func (s pgStore) ClaimNext(ctx context.Context) (Job, bool, error) {
	r, err := s.q.ClaimNextExportJob(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, err
	}
	return jobFromRow(r), true, nil
}

func (s pgStore) UpdateProgress(ctx context.Context, id, rows int64) (int64, error) {
	return s.q.UpdateExportJobProgress(ctx, repository.UpdateExportJobProgressParams{RowCount: rows, ID: id})
}

func (s pgStore) MarkSucceeded(ctx context.Context, id, rows, bytes int64, chunks int32) (int64, error) {
	return s.q.MarkExportJobSucceeded(ctx, repository.MarkExportJobSucceededParams{
		RetentionSeconds: Retention.Seconds(), RowCount: rows, ByteSize: bytes, ChunkCount: chunks, ID: id,
	})
}

func (s pgStore) MarkFailed(ctx context.Context, id int64, msg string) (bool, error) {
	return s.transition(ctx, id, func(q *repository.Queries) (int64, error) {
		return q.MarkExportJobFailed(ctx, repository.MarkExportJobFailedParams{ErrorMessage: msg, ID: id})
	})
}

func (s pgStore) Release(ctx context.Context, id int64) (bool, error) {
	return s.transition(ctx, id, func(q *repository.Queries) (int64, error) { return q.ReleaseExportJob(ctx, id) })
}

func (s pgStore) DeleteChunks(ctx context.Context, id int64) error {
	return s.q.DeleteExportJobChunks(ctx, id)
}

// transition runs the guarded UPDATE and, when it matched, deletes the chunks in the
// same tx: the row lock keeps a released job invisible to other claimers until its
// chunks are gone, and a failed job is never seen with chunks (Property 10).
func (s pgStore) transition(ctx context.Context, id int64, update func(*repository.Queries) (int64, error)) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after commit
	q := s.q.WithTx(tx)
	n, err := update(q)
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}
	if err := q.DeleteExportJobChunks(ctx, id); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

// pgReader opens REPEATABLE READ READ ONLY on the replica pool (Req 6.1, 6.2).
type pgReader struct{ pool *pgxpool.Pool }

func (r pgReader) Begin(ctx context.Context) (Querier, func(), error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, nil, err
	}
	rollback := func() {
		// Fresh ctx: the job ctx may be done; the conn must still go back clean.
		rctx, cancel := context.WithTimeout(context.Background(), finalizeTimeout)
		defer cancel()
		_ = tx.Rollback(rctx) // read-only tx; a failed rollback closes the conn
	}
	return repository.New(tx), rollback, nil
}
