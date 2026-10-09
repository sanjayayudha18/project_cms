//go:build integration

package notification

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Real-Postgres checks for notification plan "Tests" (needs migration 025).
// Every test runs in a tx that is rolled back.

func integrationTx(t *testing.T) pgx.Tx {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	return tx
}

func activeUserIDs(t *testing.T, tx pgx.Tx, n int) []int64 {
	t.Helper()
	rows, err := tx.Query(context.Background(),
		"SELECT id FROM users WHERE is_active AND deleted_at IS NULL ORDER BY id LIMIT $1", n)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil || len(ids) < n {
		t.Skipf("need %d active users (got %d): %v", n, len(ids), err)
	}
	return ids
}

func countFor(t *testing.T, tx pgx.Tx, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := tx.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

const testType = "integration.test"

func TestIntegration_SendRollsBackWithCallerTx(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()
	user := activeUserIDs(t, tx, 1)[0]

	inner, err := tx.Begin(ctx) // savepoint = the business tx
	if err != nil {
		t.Fatal(err)
	}
	msg := Message{Type: testType, Title: "Uji", Email: true}
	if err := NewService(true).Send(ctx, db.New(inner), msg, Recipients{UserIDs: []int64{user}}); err != nil {
		t.Fatal(err)
	}
	if got := countFor(t, inner, "SELECT count(*) FROM notifications WHERE type = $1", testType); got != 1 {
		t.Fatalf("inside tx: %d notifications, want 1", got)
	}
	if err := inner.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if got := countFor(t, tx, "SELECT count(*) FROM notifications WHERE type = $1", testType); got != 0 {
		t.Fatalf("after rollback: %d notifications, want 0", got)
	}
	if got := countFor(t, tx, "SELECT count(*) FROM notification_emails WHERE subject = 'Uji'"); got != 0 {
		t.Fatalf("after rollback: %d outbox rows, want 0", got)
	}
}

func TestIntegration_SendExpandsVendorUsersAndPICs(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()
	var vendorID int64
	if err := tx.QueryRow(ctx, `INSERT INTO vendors (code, name) VALUES ('NTF-IT', 'NTF-IT') RETURNING id`).Scan(&vendorID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO vendor_pics (vendor_id, name, email, is_notification_recipient) VALUES
		  ($1, 'pic on',  'pic.on@vendor.test',  true),
		  ($1, 'pic off', 'pic.off@vendor.test', false)`, vendorID); err != nil {
		t.Fatal(err)
	}
	user := activeUserIDs(t, tx, 1)[0]
	if _, err := tx.Exec(ctx, `UPDATE users SET vendor_id = $1 WHERE id = $2`, vendorID, user); err != nil {
		t.Fatal(err)
	}

	msg := Message{Type: testType, Title: "Uji vendor", Email: true}
	if err := NewService(true).Send(ctx, db.New(tx), msg, Recipients{VendorIDs: []int64{vendorID}}); err != nil {
		t.Fatal(err)
	}
	if got := countFor(t, tx, "SELECT count(*) FROM notifications WHERE type = $1 AND recipient_user_id = $2", testType, user); got != 1 {
		t.Fatalf("vendor user notifications = %d, want 1", got)
	}
	if got := countFor(t, tx, "SELECT count(*) FROM notification_emails WHERE to_address = 'pic.on@vendor.test' AND notification_id IS NULL AND status = 'pending'"); got != 1 {
		t.Fatalf("flagged PIC emails = %d, want 1", got)
	}
	if got := countFor(t, tx, "SELECT count(*) FROM notification_emails WHERE to_address = 'pic.off@vendor.test'"); got != 0 {
		t.Fatalf("unflagged PIC emails = %d, want 0", got)
	}
}

// cit-send-vendor FR7.1 / S4: a branch reaches users pinned to it plus
// vendor-wide users, and flagged PICs of that branch or vendor-wide; a user or
// PIC pinned to a sibling branch of the same vendor is left out.
func TestIntegration_SendScopesToVendorBranch(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()
	var vendorID, branchA, branchB int64
	if err := tx.QueryRow(ctx, `INSERT INTO vendors (code, name) VALUES ('NTF-BR', 'NTF-BR') RETURNING id`).Scan(&vendorID); err != nil {
		t.Fatal(err)
	}
	for code, dst := range map[string]*int64{"NTF-A": &branchA, "NTF-B": &branchB} {
		if err := tx.QueryRow(ctx, `INSERT INTO vendor_branches (vendor_id, branch_code, branch_name) VALUES ($1, $2, $2) RETURNING id`,
			vendorID, code).Scan(dst); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO vendor_pics (vendor_id, vendor_branch_id, name, email, is_notification_recipient) VALUES
		  ($1, $2,   'pic a',    'pic.a@vendor.test',    true),
		  ($1, $3,   'pic b',    'pic.b@vendor.test',    true),
		  ($1, NULL, 'pic wide', 'pic.wide@vendor.test', true)`, vendorID, branchA, branchB); err != nil {
		t.Fatal(err)
	}
	users := activeUserIDs(t, tx, 3)
	pinned, wide, sibling := users[0], users[1], users[2]
	for _, u := range []struct {
		id     int64
		branch *int64
	}{{pinned, &branchA}, {wide, nil}, {sibling, &branchB}} {
		if _, err := tx.Exec(ctx, `UPDATE users SET vendor_id = $1, vendor_branch_id = $2 WHERE id = $3`, vendorID, u.branch, u.id); err != nil {
			t.Fatal(err)
		}
	}

	msg := Message{Type: testType, Title: "Uji branch", Email: true}
	if err := NewService(true).Send(ctx, db.New(tx), msg, Recipients{VendorBranchIDs: []int64{branchA}}); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[int64]int{pinned: 1, wide: 1, sibling: 0} {
		if got := countFor(t, tx, "SELECT count(*) FROM notifications WHERE type = $1 AND recipient_user_id = $2", testType, id); got != want {
			t.Fatalf("user %d notifications = %d, want %d", id, got, want)
		}
	}
	for addr, want := range map[string]int{"pic.a@vendor.test": 1, "pic.wide@vendor.test": 1, "pic.b@vendor.test": 0} {
		if got := countFor(t, tx, "SELECT count(*) FROM notification_emails WHERE to_address = $1", addr); got != want {
			t.Fatalf("emails to %s = %d, want %d", addr, got, want)
		}
	}
}

func TestIntegration_RepositoryScopesToOwner(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()
	ids := activeUserIDs(t, tx, 2)
	owner, other := ids[0], ids[1]
	if err := NewService(false).Send(ctx, db.New(tx), Message{Type: testType, Title: "Milik owner"}, Recipients{UserIDs: []int64{owner}}); err != nil {
		t.Fatal(err)
	}
	var nid int64
	if err := tx.QueryRow(ctx, "SELECT id FROM notifications WHERE type = $1 AND recipient_user_id = $2", testType, owner).Scan(&nid); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(tx, tx)

	if found, err := repo.MarkRead(ctx, nid, other); err != nil || found {
		t.Fatalf("other user MarkRead = (%v, %v), want not found", found, err)
	}
	if found, err := repo.MarkRead(ctx, nid, owner); err != nil || !found {
		t.Fatalf("owner MarkRead = (%v, %v), want found", found, err)
	}
	if found, err := repo.MarkRead(ctx, nid, owner); err != nil || !found {
		t.Fatalf("second MarkRead = (%v, %v), want found (idempotent)", found, err)
	}
	rows, total, err := repo.List(ctx, other, false, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.ID == nid {
			t.Fatalf("other user sees owner's notification (total %d)", total)
		}
	}
}

func TestIntegration_PurgeDeletesOnlyOldRows(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()
	user := activeUserIDs(t, tx, 1)[0]
	if _, err := tx.Exec(ctx, `
		INSERT INTO notifications (recipient_user_id, type, title, created_at) VALUES
		  ($1, $2, 'lama', now() - interval '91 days'),
		  ($1, $2, 'baru', now() - interval '89 days')`, user, testType); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO notification_emails (to_address, subject, body, status, created_at) VALUES
		  ('old.sent@test.id',    'x', 'x', 'sent',    now() - interval '91 days'),
		  ('old.pending@test.id', 'x', 'x', 'pending', now() - interval '91 days')`); err != nil {
		t.Fatal(err)
	}
	w := &Worker{
		inTx: func(ctx context.Context, fn func(emailStore) error) error { return fn(db.New(tx)) },
		now:  time.Now,
	}
	if err := w.purge(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := countFor(t, tx, "SELECT count(*) FROM notifications WHERE type = $1", testType); got != 1 {
		t.Fatalf("notifications left = %d, want 1 (the 89-day-old one)", got)
	}
	if got := countFor(t, tx, "SELECT count(*) FROM notification_emails WHERE to_address = 'old.sent@test.id'"); got != 0 {
		t.Fatal("old sent email not purged")
	}
	if got := countFor(t, tx, "SELECT count(*) FROM notification_emails WHERE to_address = 'old.pending@test.id'"); got != 1 {
		t.Fatal("pending email must never be purged")
	}
}
