package notification

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

type fakeStore struct {
	pending []db.ClaimPendingNotificationEmailsRow
	sent    []int64
	retries []db.MarkNotificationEmailRetryParams
	failed  []db.MarkNotificationEmailFailedParams
	purged  []time.Time
}

func (f *fakeStore) ClaimPendingNotificationEmails(_ context.Context, _ int32) ([]db.ClaimPendingNotificationEmailsRow, error) {
	return f.pending, nil
}

func (f *fakeStore) MarkNotificationEmailSent(_ context.Context, id int64) error {
	f.sent = append(f.sent, id)
	return nil
}

func (f *fakeStore) MarkNotificationEmailRetry(_ context.Context, arg db.MarkNotificationEmailRetryParams) error {
	f.retries = append(f.retries, arg)
	return nil
}

func (f *fakeStore) MarkNotificationEmailFailed(_ context.Context, arg db.MarkNotificationEmailFailedParams) error {
	f.failed = append(f.failed, arg)
	return nil
}

func (f *fakeStore) DeleteOldNotificationEmails(_ context.Context, before pgtype.Timestamptz) (int64, error) {
	f.purged = append(f.purged, before.Time)
	return 0, nil
}

func (f *fakeStore) DeleteOldNotifications(_ context.Context, before pgtype.Timestamptz) (int64, error) {
	f.purged = append(f.purged, before.Time)
	return 0, nil
}

type fakeMailer struct {
	failFor map[string]bool
	sentTo  []string
}

func (m *fakeMailer) Send(_ context.Context, to, _, _ string) error {
	if m.failFor[to] {
		return errors.New("smtp: 451 try later")
	}
	m.sentTo = append(m.sentTo, to)
	return nil
}

var fixedNow = time.Date(2026, 10, 7, 3, 0, 0, 0, time.UTC)

func newTestWorker(store *fakeStore, mailer Mailer) *Worker {
	return &Worker{
		inTx:     func(_ context.Context, fn func(emailStore) error) error { return fn(store) },
		mailer:   mailer,
		interval: time.Minute,
		now:      func() time.Time { return fixedNow },
	}
}

func TestWorker_SendBatch(t *testing.T) {
	store := &fakeStore{pending: []db.ClaimPendingNotificationEmailsRow{
		{ID: 1, ToAddress: "ok@bank.co.id", Attempts: 0},
		{ID: 2, ToAddress: "down@bank.co.id", Attempts: 0}, // 1st failure -> +1m
		{ID: 3, ToAddress: "down@bank.co.id", Attempts: 1}, // 2nd failure -> +5m
		{ID: 4, ToAddress: "down@bank.co.id", Attempts: 2}, // 3rd failure -> +15m
		{ID: 5, ToAddress: "down@bank.co.id", Attempts: 3}, // 4th failure -> failed
	}}
	mailer := &fakeMailer{failFor: map[string]bool{"down@bank.co.id": true}}

	if err := newTestWorker(store, mailer).sendBatch(context.Background()); err != nil {
		t.Fatal(err)
	}

	if len(store.sent) != 1 || store.sent[0] != 1 {
		t.Fatalf("sent = %v, want [1]", store.sent)
	}
	wantNext := map[int64]time.Duration{2: time.Minute, 3: 5 * time.Minute, 4: 15 * time.Minute}
	if len(store.retries) != len(wantNext) {
		t.Fatalf("retries = %+v, want %d", store.retries, len(wantNext))
	}
	for _, r := range store.retries {
		if got := r.NextAttemptAt.Time.Sub(fixedNow); got != wantNext[r.ID] {
			t.Fatalf("email %d next attempt in %v, want %v", r.ID, got, wantNext[r.ID])
		}
		if r.LastError == nil || *r.LastError == "" {
			t.Fatalf("email %d retry without last_error", r.ID)
		}
	}
	if len(store.failed) != 1 || store.failed[0].ID != 5 {
		t.Fatalf("failed = %+v, want [5]", store.failed)
	}
}

func TestWorker_TruncatesLastError(t *testing.T) {
	if got := truncateRunes(string(make([]rune, 600)), 500); len([]rune(got)) != 500 {
		t.Fatalf("len = %d, want 500", len([]rune(got)))
	}
}

func TestWorker_TickPurgesOncePerDay(t *testing.T) {
	store := &fakeStore{}
	w := newTestWorker(store, nil) // SMTP disabled: purge still runs
	ctx := context.Background()

	w.tick(ctx)
	if len(store.purged) != 2 {
		t.Fatalf("purged = %d calls, want 2 on first tick", len(store.purged))
	}
	if want := fixedNow.Add(-retention); !store.purged[0].Equal(want) {
		t.Fatalf("cutoff = %v, want %v", store.purged[0], want)
	}

	w.tick(ctx) // same day: no purge
	if len(store.purged) != 2 {
		t.Fatalf("purged again within 24h (%d calls)", len(store.purged))
	}

	w.now = func() time.Time { return fixedNow.Add(25 * time.Hour) }
	w.tick(ctx)
	if len(store.purged) != 4 {
		t.Fatalf("purged = %d calls, want 4 after 24h", len(store.purged))
	}
}

func TestWorker_TickWithoutMailerSendsNothing(t *testing.T) {
	store := &fakeStore{pending: []db.ClaimPendingNotificationEmailsRow{{ID: 1, ToAddress: "a@b.co"}}}
	newTestWorker(store, nil).tick(context.Background())
	if len(store.sent)+len(store.retries)+len(store.failed) != 0 {
		t.Fatal("SMTP disabled must not touch the outbox")
	}
}
