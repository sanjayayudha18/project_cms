//go:build integration

package service

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/notification"
)

// notification plan B4 / spec AC1-AC2: approving a completion report that
// pushes an ATM over quota notifies the report's maker + every active
// ATM-SPV inside the approve tx (needs migrations 021 + 025).

func notifiedUsers(t *testing.T, tx pgx.Tx, requestID int64) []int64 {
	t.Helper()
	rows, err := tx.Query(context.Background(), `
		SELECT recipient_user_id FROM notifications
		WHERE entity_type = 'vendor_request' AND entity_id = $1 AND type = 'visit_quota.over_quota'
		ORDER BY recipient_user_id`, requestID)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		t.Fatal(err)
	}
	return ids
}

func TestIntegration_OverQuotaNotifiesMakerAndATMSPV(t *testing.T) {
	f := setupQuotaFixture(t)
	f.vr.WithNotifier(notification.NewService(true))
	ctx := context.Background()
	term := f.seedPackageATM(t, "N", "PAKET 3")

	var lastID int64
	for i := 0; i < 4; i++ {
		id := f.approvedRequest(t, term)
		over := f.complete(t, id, map[string]string{term: "success"})
		if i < 3 {
			if got := notifiedUsers(t, f.tx, id); len(got) != 0 || len(over) != 0 {
				t.Fatalf("visit %d within quota notified %v (over %v)", i+1, got, over)
			}
		}
		lastID = id
	}

	rows, err := f.tx.Query(ctx, `
		SELECT u.id FROM users u JOIN roles r ON r.id = u.role_id
		WHERE u.is_active AND u.deleted_at IS NULL AND (u.id = $1 OR r.role = 'ATM-SPV')
		ORDER BY u.id`, f.maker.UserID)
	if err != nil {
		t.Fatal(err)
	}
	want, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		t.Fatal(err)
	}
	got := notifiedUsers(t, f.tx, lastID)
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	if len(want) == 0 || len(got) != len(want) {
		t.Fatalf("notified = %v, want %v (maker %d + active ATM-SPV)", got, want, f.maker.UserID)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("notified = %v, want %v", got, want)
		}
	}

	var pending, distinctAddr int
	if err := f.tx.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE e.status = 'pending'), count(DISTINCT lower(e.to_address))
		FROM notification_emails e JOIN notifications n ON n.id = e.notification_id
		WHERE n.entity_type = 'vendor_request' AND n.entity_id = $1`, lastID).Scan(&pending, &distinctAddr); err != nil {
		t.Fatal(err)
	}
	if pending == 0 || pending != distinctAddr {
		t.Fatalf("outbox pending = %d, distinct addresses = %d; want one pending row per address", pending, distinctAddr)
	}
}

type failingNotifier struct{}

func (failingNotifier) Send(context.Context, notification.Querier, notification.Message, notification.Recipients) error {
	return errors.New("notify boom")
}

func TestIntegration_OverQuotaNotifyFailureRollsBackApprove(t *testing.T) {
	f := setupQuotaFixture(t)
	ctx := context.Background()
	term := f.seedPackageATM(t, "F", "PAKET 3")
	for i := 0; i < 3; i++ {
		f.complete(t, f.approvedRequest(t, term), map[string]string{term: "success"})
	}
	before, _ := f.remaining(t, term)

	f.vr.WithNotifier(failingNotifier{})
	id := f.approvedRequest(t, term)
	if _, err := f.vr.SubmitCompletion(ctx, f.maker, id, []CompletionResultInput{{term, "success"}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.vr.ApproveCompletion(ctx, f.checker, id); err == nil {
		t.Fatal("approve must fail when the notification cannot be written")
	}
	if after, _ := f.remaining(t, term); after != before {
		t.Fatalf("remaining %d -> %d: visit not rolled back", before, after)
	}
	if d, _ := f.vr.Get(ctx, id); d.Status != "completion_pending" {
		t.Fatalf("status = %s, want completion_pending", d.Status)
	}
}
