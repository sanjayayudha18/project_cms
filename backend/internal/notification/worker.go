package notification

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

const (
	batchSize     = 50
	retention     = 90 * 24 * time.Hour // spec FR3, PO decision #5
	purgeEvery    = 24 * time.Hour
	maxLastErrLen = 500
)

// retryBackoff: wait after the 1st, 2nd and 3rd failed attempt; the 4th
// failure marks the email failed (intent decision 2: "retry 3x").
var retryBackoff = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute}

// Mailer delivers one plain-text email.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// emailStore is the subset of *db.Queries the worker uses.
type emailStore interface {
	ClaimPendingNotificationEmails(ctx context.Context, limit int32) ([]db.ClaimPendingNotificationEmailsRow, error)
	MarkNotificationEmailSent(ctx context.Context, id int64) error
	MarkNotificationEmailRetry(ctx context.Context, arg db.MarkNotificationEmailRetryParams) error
	MarkNotificationEmailFailed(ctx context.Context, arg db.MarkNotificationEmailFailedParams) error
	DeleteOldNotificationEmails(ctx context.Context, before pgtype.Timestamptz) (int64, error)
	DeleteOldNotifications(ctx context.Context, before pgtype.Timestamptz) (int64, error)
}

// Worker sends queued emails (when mailer != nil) and purges notifications
// older than 90 days once a day. One goroutine in cmd/api runs it.
type Worker struct {
	inTx      func(ctx context.Context, fn func(emailStore) error) error
	mailer    Mailer // nil = SMTP not configured: only purge runs
	interval  time.Duration
	now       func() time.Time
	lastPurge time.Time // in memory: a restart purges again, DELETE is idempotent
}

// NewWorker builds a Worker on the primary pool. Pass a nil mailer when
// SMTP_HOST is empty.
func NewWorker(pool *pgxpool.Pool, mailer Mailer, interval time.Duration) *Worker {
	return &Worker{
		inTx: func(ctx context.Context, fn func(emailStore) error) error {
			return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error { return fn(db.New(tx)) })
		},
		mailer:   mailer,
		interval: interval,
		now:      time.Now,
	}
}

// Run ticks until ctx is cancelled (shutdown).
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		w.tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	if w.mailer != nil {
		if err := w.sendBatch(ctx); err != nil && ctx.Err() == nil {
			slog.ErrorContext(ctx, "notification email batch failed", "error", err)
		}
	}
	now := w.now()
	if now.Sub(w.lastPurge) < purgeEvery {
		return
	}
	if err := w.purge(ctx, now); err != nil {
		if ctx.Err() == nil {
			slog.ErrorContext(ctx, "notification retention purge failed", "error", err)
		}
		return
	}
	w.lastPurge = now
}

// sendBatch claims up to batchSize due emails and sends them in one tx.
// ponytail: SMTP calls run while the claim tx holds the row locks (bounded by
// the mailer's 30 s deadline × batchSize); move to claim-then-send-outside-tx
// if batches ever get large. A failure to record a result rolls the batch
// back, so those emails are retried later (at-least-once delivery).
func (w *Worker) sendBatch(ctx context.Context) error {
	return w.inTx(ctx, func(s emailStore) error {
		rows, err := s.ClaimPendingNotificationEmails(ctx, batchSize)
		if err != nil {
			return fmt.Errorf("claim pending emails: %w", err)
		}
		for _, r := range rows {
			if err := w.deliver(ctx, s, r); err != nil {
				return err
			}
		}
		return nil
	})
}

func (w *Worker) deliver(ctx context.Context, s emailStore, r db.ClaimPendingNotificationEmailsRow) error {
	sendErr := w.mailer.Send(ctx, r.ToAddress, r.Subject, r.Body)
	if sendErr == nil {
		if err := s.MarkNotificationEmailSent(ctx, r.ID); err != nil {
			return fmt.Errorf("mark email %d sent: %w", r.ID, err)
		}
		return nil
	}

	lastErr := truncateRunes(sendErr.Error(), maxLastErrLen)
	attempt := int(r.Attempts) + 1
	slog.WarnContext(ctx, "notification email send failed", "notification_email_id", r.ID, "attempt", attempt)
	if attempt > len(retryBackoff) {
		if err := s.MarkNotificationEmailFailed(ctx, db.MarkNotificationEmailFailedParams{ID: r.ID, LastError: &lastErr}); err != nil {
			return fmt.Errorf("mark email %d failed: %w", r.ID, err)
		}
		return nil
	}
	next := pgtype.Timestamptz{Time: w.now().Add(retryBackoff[attempt-1]), Valid: true}
	if err := s.MarkNotificationEmailRetry(ctx, db.MarkNotificationEmailRetryParams{ID: r.ID, LastError: &lastErr, NextAttemptAt: next}); err != nil {
		return fmt.Errorf("mark email %d retry: %w", r.ID, err)
	}
	return nil
}

func (w *Worker) purge(ctx context.Context, now time.Time) error {
	cutoff := pgtype.Timestamptz{Time: now.Add(-retention), Valid: true}
	return w.inTx(ctx, func(s emailStore) error {
		emails, err := s.DeleteOldNotificationEmails(ctx, cutoff)
		if err != nil {
			return fmt.Errorf("purge notification emails: %w", err)
		}
		notifications, err := s.DeleteOldNotifications(ctx, cutoff)
		if err != nil {
			return fmt.Errorf("purge notifications: %w", err)
		}
		if emails+notifications > 0 {
			slog.InfoContext(ctx, "notification retention purge", "emails", emails, "notifications", notifications)
		}
		return nil
	})
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
