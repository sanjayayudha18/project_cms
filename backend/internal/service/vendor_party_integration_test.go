//go:build integration

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// cit-send-vendor S3 on real Postgres: ATM-SPV vault approve sends the request
// to every replenish + vault branch (FR2.1), content holds no internal data
// (FR3.3), vendor users in scope are notified (FR7.1), and a resend only
// touches parties that changed, were rejected, or all after a re-approval
// (FR2.2/FR2.3).

// sentToVendor drives a two-ATM request through ACM to sent_to_vendor:
// terminal -> cash1 (same vendor), terminal2 -> cash2 (other vendor).
func (f *vaultPlanFixture) sentToVendor(t *testing.T) int64 {
	t.Helper()
	ctx := context.Background()
	acmUser := Actor{UserID: f.acmUserID, Role: "ACM-USER"}
	acmSpv := Actor{UserID: f.spvID, Role: "ACM-SPV"}
	reqID, plan1, plan2 := f.approvedTwoAtms(t)
	for plan, in := range map[int64]VaultAssignmentInput{
		plan1: {TerminalID: f.terminal, VaultBranchID: f.cash1},
		plan2: {TerminalID: f.terminal2, VaultBranchID: f.cash2},
	} {
		if _, err := f.plans.SaveAssignments(ctx, acmUser, plan, []VaultAssignmentInput{in}); err != nil {
			t.Fatalf("save plan %d: %v", plan, err)
		}
		if _, err := f.plans.Submit(ctx, acmUser, plan); err != nil {
			t.Fatalf("submit plan %d: %v", plan, err)
		}
		if _, err := f.plans.Approve(ctx, acmSpv, plan); err != nil {
			t.Fatalf("approve plan %d: %v", plan, err)
		}
	}
	if err := f.plans.ReviewRequest(ctx, f.checker, reqID, true, ""); err != nil {
		t.Fatalf("vault approve: %v", err)
	}
	return reqID
}

type partyState struct {
	id      int64
	status  string
	content string
}

type querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func partiesOf(t *testing.T, q querier, reqID int64) map[partyKey]partyState {
	t.Helper()
	rows, err := q.Query(context.Background(),
		`SELECT id, vendor_branch_id, role, status, content::text FROM vendor_request_vendor_parties WHERE vendor_request_id = $1`, reqID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[partyKey]partyState{}
	for rows.Next() {
		var k partyKey
		var s partyState
		if err := rows.Scan(&s.id, &k.branchID, &k.role, &s.status, &s.content); err != nil {
			t.Fatal(err)
		}
		out[k] = s
	}
	return out
}

func countRows(t *testing.T, q querier, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := q.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

func TestIntegration_VendorParty_SendAndResync(t *testing.T) {
	f := newVaultPlanFixture(t)
	ctx := context.Background()

	// A vendor user of cash2's vendor, pinned to cash2 (FR7.1 recipient).
	var vendorUser int64
	if err := f.pool.QueryRow(ctx, `INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source, password_hash, vendor_id, vendor_branch_id)
		SELECT r.id, $1, $1, $1 || '@vendor.test', false, 'local', 'x', b.vendor_id, b.id
		FROM roles r, vendor_branches b WHERE r.role = 'VENDOR-USER' AND b.id = $2 RETURNING id`,
		"itvnd-"+uuid.NewString()[:8], f.cash2).Scan(&vendorUser); err != nil {
		t.Fatalf("seed vendor user: %v", err)
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = f.pool.Exec(c, `DELETE FROM notification_emails WHERE notification_id IN (SELECT id FROM notifications WHERE recipient_user_id = $1)`, vendorUser)
		_, _ = f.pool.Exec(c, `DELETE FROM notifications WHERE recipient_user_id = $1`, vendorUser)
		_, _ = f.pool.Exec(c, `UPDATE users SET is_active = false, deleted_at = now() WHERE id = $1`, vendorUser)
		_, _ = f.pool.Exec(c, `DELETE FROM users WHERE id = $1`, vendorUser)
	})

	reqID := f.sentToVendor(t)

	// FR2.1: request sent, four parties pending (2 replenish + cash1 + cash2 vault).
	var status string
	var vendorSent, sentAtSet bool
	if err := f.pool.QueryRow(ctx, `SELECT status, vendor_sent, sent_at IS NOT NULL FROM vendor_requests WHERE id = $1`, reqID).
		Scan(&status, &vendorSent, &sentAtSet); err != nil || status != "sent_to_vendor" || !vendorSent || !sentAtSet {
		t.Fatalf("request = %s sent=%v sent_at=%v (%v)", status, vendorSent, sentAtSet, err)
	}
	parties := partiesOf(t, f.pool, reqID)
	if len(parties) != 4 {
		t.Fatalf("parties = %v, want 4", parties)
	}
	var replenish2 int64
	for k, p := range parties {
		if p.status != "pending" {
			t.Fatalf("party %+v status %s, want pending", k, p.status)
		}
		if k.role == partyRoleReplenish && k.branchID != f.branchID {
			replenish2 = k.branchID
		}
	}
	cash2 := parties[partyKey{f.cash2, partyRoleVault}]
	if !strings.Contains(cash2.content, f.terminal2) || strings.Contains(cash2.content, `"`+f.terminal+`"`) {
		t.Fatalf("cash2 content must hold only %s: %s", f.terminal2, cash2.content)
	}
	for k, p := range parties {
		for _, internal := range []string{"tier", "saldo", "capacity", "urgent", "price"} {
			if strings.Contains(strings.ToLower(p.content), internal) {
				t.Fatalf("party %+v content leaks %q: %s", k, internal, p.content)
			}
		}
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM vendor_request_vendor_party_events e JOIN vendor_request_vendor_parties p ON p.id = e.party_id
		WHERE p.vendor_request_id = $1 AND e.event = 'sent' AND e.actor_id = $2`, reqID, f.checker.UserID); n != 4 {
		t.Fatalf("sent events = %d, want 4", n)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM notifications WHERE recipient_user_id = $1 AND type = 'vendor_order.sent'
		AND entity_type = 'vendor_request_vendor_party'`, vendorUser); n != 1 {
		t.Fatalf("vendor user notifications = %d, want 1 (own branch only)", n)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM notifications WHERE recipient_user_id = $1 AND entity_id = $2`, vendorUser, cash2.id); n != 1 {
		t.Fatalf("vendor user notification is not about the cash2 party")
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM audit_logs WHERE entity_type = 'vendor_request' AND entity_id = $1
		AND action = 'vault_approve' AND jsonb_array_length(after->'vendor_parties') = 4`, reqID); n != 1 {
		t.Fatalf("vault_approve audit with 4 parties = %d, want 1", n)
	}

	// Resync scenarios each run in a rolled-back tx on top of the sent request.
	resync := func(t *testing.T, setup func(tx pgx.Tx) error) ([]partyNotice, map[partyKey]partyState) {
		t.Helper()
		tx, err := f.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = tx.Rollback(ctx) })
		if err := setup(tx); err != nil {
			t.Fatalf("setup: %v", err)
		}
		q := db.New(tx)
		req, err := q.GetVendorRequestForUpdate(ctx, reqID)
		if err != nil {
			t.Fatal(err)
		}
		notices, err := syncVendorParties(ctx, q, f.checker.UserID, req)
		if err != nil {
			t.Fatalf("sync: %v", err)
		}
		return notices, partiesOf(t, tx, reqID)
	}
	events := func(ns []partyNotice) map[int64]string {
		out := map[int64]string{}
		for _, n := range ns {
			out[n.branchID] = n.event
		}
		return out
	}
	decide := func(tx pgx.Tx, status, role string, branch int64, reason *string) error {
		_, err := tx.Exec(ctx, `UPDATE vendor_request_vendor_parties SET status = $1, decided_by = $2, decided_at = now(), rejection_reason = $3
			WHERE vendor_request_id = $4 AND role = $5 AND vendor_branch_id = $6`, status, f.checker.UserID, reason, reqID, role, branch)
		return err
	}

	t.Run("vault changed: only changed parties", func(t *testing.T) {
		notices, after := resync(t, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `UPDATE vendor_request_vault_assignments SET vault_branch_id = $1 WHERE vendor_request_id = $2 AND terminal_id = $3`,
				f.cash1, reqID, f.terminal2); err != nil {
				return err
			}
			return decide(tx, "accepted", partyRoleReplenish, f.branchID, nil)
		})
		got := events(notices)
		if len(got) != 3 || got[f.cash2] != "withdrawn" || got[f.cash1] != "resent" || got[replenish2] != "resent" {
			t.Fatalf("notices = %v, want cash2 withdrawn, cash1 + replenish %d resent", got, replenish2)
		}
		if after[partyKey{f.branchID, partyRoleReplenish}].status != "accepted" {
			t.Fatal("unchanged accepted party was reset")
		}
		if after[partyKey{f.cash2, partyRoleVault}].status != "withdrawn" || after[partyKey{f.cash1, partyRoleVault}].status != "pending" {
			t.Fatalf("after = %v", after)
		}
	})

	t.Run("rejected party with same content is resent", func(t *testing.T) {
		reason := "tidak sanggup hari itu"
		notices, after := resync(t, func(tx pgx.Tx) error {
			return decide(tx, "rejected", partyRoleReplenish, f.branchID, &reason)
		})
		if got := events(notices); len(got) != 1 || got[f.branchID] != "resent" {
			t.Fatalf("notices = %v, want only the rejected party resent", got)
		}
		if after[partyKey{f.branchID, partyRoleReplenish}].status != "pending" {
			t.Fatal("rejected party not back to pending")
		}
	})

	t.Run("re-approved after edit: every party resent", func(t *testing.T) {
		notices, _ := resync(t, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `UPDATE vendor_requests SET approved_at = now() + interval '1 hour' WHERE id = $1`, reqID); err != nil {
				return err
			}
			return decide(tx, "accepted", partyRoleVault, f.cash1, nil)
		})
		if len(notices) != 4 {
			t.Fatalf("notices = %v, want 4 resent", notices)
		}
		for _, n := range notices {
			if n.event != "resent" {
				t.Fatalf("notice %+v, want resent", n)
			}
		}
	})
}
