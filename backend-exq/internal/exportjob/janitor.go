package exportjob

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
)

// janitorStepTimeout bounds each janitor statement so one slow step cannot eat the tick.
const janitorStepTimeout = 10 * time.Second

// janitorQueries is the primary-side SQL the janitor runs (*repository.Queries).
// Every statement is a guarded, idempotent UPDATE/DELETE, safe on many instances.
type janitorQueries interface {
	FailTimedOutExportJobs(ctx context.Context, arg repository.FailTimedOutExportJobsParams) ([]int64, error)
	RecoverStaleDownloads(ctx context.Context, leaseSeconds float64) (int64, error)
	ExpireExportJobs(ctx context.Context) ([]int64, error)
	DeleteOrphanExportJobChunks(ctx context.Context) (int64, error)
}

// Janitor enforces the time limits on export jobs (Req 9.9, 10.5-10.7): it fails
// jobs running too long, recovers stale downloads, expires unread files and sweeps
// orphan chunks, so personal data in export_job_chunks never outlives its window.
type Janitor struct {
	q        janitorQueries
	interval time.Duration
	log      *slog.Logger

	jobTimeout    time.Duration
	downloadLease time.Duration
	stepTimeout   time.Duration
}

// NewJanitor runs on the primary pool; interval is WorkerConfig.JanitorInterval (<= 1m).
func NewJanitor(primary *pgxpool.Pool, interval time.Duration) *Janitor {
	return newJanitor(repository.New(primary), interval)
}

func newJanitor(q janitorQueries, interval time.Duration) *Janitor {
	return &Janitor{
		q: q, interval: interval, log: slog.Default(),
		jobTimeout: JobTimeout, downloadLease: DownloadLease, stepTimeout: janitorStepTimeout,
	}
}

// Run ticks once immediately, then every interval, until ctx is cancelled.
func (j *Janitor) Run(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	j.tick(ctx)
	t := time.NewTicker(j.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			j.tick(ctx)
		}
	}
}

// tick runs the four steps in order; a failing step is logged and the rest still run.
// Recover (2) runs before expire (3) so a download recovered past its retention
// expires in the same tick (Req 10.6).
func (j *Janitor) tick(ctx context.Context) {
	var failed, recovered, expired, orphans int64
	ok := true

	j.step(ctx, "fail_timed_out", &ok, func(c context.Context) error {
		ids, err := j.q.FailTimedOutExportJobs(c, repository.FailTimedOutExportJobsParams{
			ErrorMessage: MsgTimeout, TimeoutSeconds: j.jobTimeout.Seconds(),
		})
		failed = int64(len(ids))
		return err
	})
	j.step(ctx, "recover_downloads", &ok, func(c context.Context) error {
		n, err := j.q.RecoverStaleDownloads(c, j.downloadLease.Seconds())
		recovered = n
		return err
	})
	j.step(ctx, "expire", &ok, func(c context.Context) error {
		ids, err := j.q.ExpireExportJobs(c)
		expired = int64(len(ids))
		return err
	})
	j.step(ctx, "sweep_orphans", &ok, func(c context.Context) error {
		n, err := j.q.DeleteOrphanExportJobChunks(c)
		orphans = n
		return err
	})

	j.log.Info("export_janitor_tick", "failed", failed, "recovered", recovered,
		"expired", expired, "orphans", orphans, "ok", ok)
}

func (j *Janitor) step(ctx context.Context, name string, ok *bool, fn func(context.Context) error) {
	c, cancel := context.WithTimeout(ctx, j.stepTimeout)
	defer cancel()
	if err := fn(c); err != nil {
		*ok = false
		j.log.Error("export_janitor_step_failed", "step", name, "error", err.Error())
	}
}
