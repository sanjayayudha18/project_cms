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
	"github.com/cimb-niaga/cms/backend/internal/vendorrole"
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

// cit-send-vendor FR7.1 / S4 + vendor-party-scope Req 10.1 (e): a vault party
// of branch A reaches only vault-capable roles pinned to A or vendor-wide, and
// only PICs with the vault role; a replenish-only user, a cashcount-only PIC
// and anything pinned to sibling branch B are left out.
func TestIntegration_SendVaultPartyScopesToRoleAndPIC(t *testing.T) {
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
		INSERT INTO vendor_pics (vendor_id, vendor_branch_id, name, email, is_notification_recipient, roles) VALUES
		  ($1, $2,   'pic vault a',     'pic.vault.a@vendor.test',   true, '{vault}'),
		  ($1, $2,   'pic cashcount a', 'pic.cashcount@vendor.test', true, '{cashcount}'),
		  ($1, $2,   'pic replen a',    'pic.replen@vendor.test',    true, '{replenish}'),
		  ($1, NULL, 'pic wide',        'pic.wide@vendor.test',      true, '{vault,cashcount}'),
		  ($1, $3,   'pic vault b',     'pic.vault.b@vendor.test',   true, '{vault}')`, vendorID, branchA, branchB); err != nil {
		t.Fatal(err)
	}
	ids := activeUserIDs(t, tx, 6)
	setup := []struct {
		id     int64
		role   string
		branch *int64
		want   int
	}{
		{ids[0], vendorrole.VaultUser, &branchA, 1}, // pinned vault user
		{ids[1], vendorrole.VaultUser, nil, 1},      // vendor-wide vault user
		{ids[2], vendorrole.User, &branchA, 1},      // both party roles
		{ids[3], vendorrole.ATMUser, &branchA, 0},   // replenish only
		{ids[4], vendorrole.VaultUser, &branchB, 0}, // sibling branch
		{ids[5], "", nil, 0},                        // internal user, untouched
	}
	for _, u := range setup[:5] {
		if _, err := tx.Exec(ctx, `UPDATE users SET vendor_id = $1, vendor_branch_id = $2,
			role_id = (SELECT id FROM roles WHERE role = $3) WHERE id = $4`, vendorID, u.branch, u.role, u.id); err != nil {
			t.Fatal(err)
		}
	}

	msg := Message{Type: testType, Title: "Uji pihak vault", Email: true}
	to := Recipients{VendorParties: []VendorPartyTarget{{
		BranchID: branchA, UserRoles: vendorrole.ForParty("vault"), PicRole: "vault",
	}}}
	if err := NewService(true).Send(ctx, db.New(tx), msg, to); err != nil {
		t.Fatal(err)
	}
	for _, u := range setup {
		if got := countFor(t, tx, "SELECT count(*) FROM notifications WHERE type = $1 AND recipient_user_id = $2", testType, u.id); got != u.want {
			t.Fatalf("user %d (%s) notifications = %d, want %d", u.id, u.role, got, u.want)
		}
	}
	for addr, want := range map[string]int{
		"pic.vault.a@vendor.test": 1, "pic.wide@vendor.test": 1,
		"pic.cashcount@vendor.test": 0, "pic.replen@vendor.test": 0, "pic.vault.b@vendor.test": 0,
	} {
		if got := countFor(t, tx, "SELECT count(*) FROM notification_emails WHERE to_address = $1", addr); got != want {
			t.Fatalf("emails to %s = %d, want %d", addr, got, want)
		}
	}
}
