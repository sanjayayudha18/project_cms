//go:build integration

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// fetchUserIDs returns n distinct seeded user ids, skipping the test if
// fewer exist — same pattern as approval's E2E integration test. Cancel's
// authz union rule and audit FK (audit_logs.actor_id -> users(id)) both need
// real, distinct user rows, not arbitrary integers.
func fetchUserIDs(t *testing.T, pool *pgxpool.Pool, n int) []int64 {
	t.Helper()
	ctx := context.Background()
	rows, err := pool.Query(ctx, "SELECT id FROM users ORDER BY id LIMIT $1", n)
	if err != nil {
		t.Fatalf("fetch seed users: %v", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan user id: %v", err)
		}
		ids = append(ids, id)
	}
	if len(ids) < n {
		t.Skipf("need at least %d seeded users for this test, found %d", n, len(ids))
	}
	return ids
}

func countAuditEntries(t *testing.T, pool *pgxpool.Pool, entityID int64, action string) int64 {
	t.Helper()
	var count int64
	err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM audit_logs WHERE entity_type = 'vendor_request' AND entity_id = $1 AND action = $2`,
		entityID, action).Scan(&count)
	if err != nil {
		t.Fatalf("count audit entries: %v", err)
	}
	return count
}

// TestIntegration_Cancel_SoftCancelPreservesRowAndAudits covers Task 7.3
// (Req 5.2, 5.4): cancel sets is_canceled=true AND status='cancelled', never
// deletes the row or its items, and writes exactly one 'cancel' audit entry.
func TestIntegration_Cancel_SoftCancelPreservesRowAndAudits(t *testing.T) {
	pool, vendorID, createdBy, terminalID := setupNumberGeneratorHarness(t)
	svc := NewVendorRequestService(pool)
	ctx := context.Background()
	creator := Actor{UserID: createdBy, Role: "ATM-USER"}

	created, err := svc.Create(ctx, creator, manualCreateInput(vendorID, terminalID, 50000))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := svc.Cancel(ctx, creator, created.ID, "test reason"); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	var status string
	var isCanceled bool
	if err := pool.QueryRow(ctx, `SELECT status, is_canceled FROM vendor_requests WHERE id = $1`, created.ID).
		Scan(&status, &isCanceled); err != nil {
		t.Fatalf("read back vendor_requests row: %v", err)
	}
	if status != "cancelled" || !isCanceled {
		t.Errorf("status=%q is_canceled=%v, want status=cancelled is_canceled=true", status, isCanceled)
	}

	var itemCount int64
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM vendor_request_items WHERE vendor_request_id = $1`, created.ID).
		Scan(&itemCount); err != nil {
		t.Fatalf("count items: %v", err)
	}
	if itemCount != 1 {
		t.Errorf("item count = %d, want 1 (cancel must not delete items)", itemCount)
	}

	if got := countAuditEntries(t, pool, created.ID, "cancel"); got != 1 {
		t.Errorf("cancel audit entries = %d, want exactly 1", got)
	}
}

// TestIntegration_Cancel_AlreadyCanceledRejectsWithoutDuplicateAudit covers
// Task 7.3 (Req 5.7): a second cancel on an already-canceled request is
// rejected with ErrAlreadyCanceled and writes no additional audit entry.
func TestIntegration_Cancel_AlreadyCanceledRejectsWithoutDuplicateAudit(t *testing.T) {
	pool, vendorID, createdBy, terminalID := setupNumberGeneratorHarness(t)
	svc := NewVendorRequestService(pool)
	ctx := context.Background()
	creator := Actor{UserID: createdBy, Role: "ATM-USER"}

	created, err := svc.Create(ctx, creator, manualCreateInput(vendorID, terminalID, 50000))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Cancel(ctx, creator, created.ID, "test reason"); err != nil {
		t.Fatalf("first cancel: %v", err)
	}

	_, err = svc.Cancel(ctx, creator, created.ID, "test reason")
	if !errors.Is(err, ErrAlreadyCanceled) {
		t.Fatalf("second cancel error = %v, want ErrAlreadyCanceled", err)
	}

	if got := countAuditEntries(t, pool, created.ID, "cancel"); got != 1 {
		t.Errorf("cancel audit entries after duplicate attempt = %d, want still exactly 1", got)
	}
}

// TestIntegration_Cancel_AuthorizationUnionRule covers Task 7.3 (Req 5.3,
// 5.8): on a pending_approval request, the creator (Maker) and a
// non-creator Checker (SPV) may both cancel; an unrelated actor may not.
func TestIntegration_Cancel_AuthorizationUnionRule(t *testing.T) {
	pool, vendorID, _, terminalID := setupNumberGeneratorHarness(t)
	ids := fetchUserIDs(t, pool, 3)
	creatorID, checkerID, unrelatedID := ids[0], ids[1], ids[2]
	svc := NewVendorRequestService(pool)
	ctx := context.Background()
	creator := Actor{UserID: creatorID, Role: "ATM-USER"}

	t.Run("checker (non-creator) may cancel a pending request", func(t *testing.T) {
		created, err := svc.Create(ctx, creator, manualCreateInput(vendorID, terminalID, 50000))
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := svc.Submit(ctx, creator, created.ID); err != nil {
			t.Fatalf("submit: %v", err)
		}
		checker := Actor{UserID: checkerID, Role: "ATM-SPV"}
		if _, err := svc.Cancel(ctx, checker, created.ID, "test reason"); err != nil {
			t.Fatalf("checker cancel: %v", err)
		}
	})

	t.Run("unrelated non-checker actor may not cancel a pending request", func(t *testing.T) {
		created, err := svc.Create(ctx, creator, manualCreateInput(vendorID, terminalID, 100000))
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := svc.Submit(ctx, creator, created.ID); err != nil {
			t.Fatalf("submit: %v", err)
		}
		unrelated := Actor{UserID: unrelatedID, Role: "ATM-USER"}
		_, err = svc.Cancel(ctx, unrelated, created.ID, "test reason")
		if !errors.Is(err, ErrNotAuthorized) {
			t.Fatalf("unrelated actor cancel error = %v, want ErrNotAuthorized", err)
		}
	})
}

// TestIntegration_Cancel_ApprovedIsCheckerOnly covers
// replenishment-request-enhancements Task 7/8 (Req 3.1, 3.7): an approved
// request IS cancelable, but only by a Checker — the creator (a Maker, not a
// Checker here) gets ErrNotChecker, while a Checker succeeds. This supersedes
// the old request-replenish-to-vendor Task 7.3 assumption that approved was
// entirely non-cancelable (ErrInvalidTransition); Req 3 replaces that rule.
func TestIntegration_Cancel_ApprovedIsCheckerOnly(t *testing.T) {
	pool, vendorID, _, terminalID := setupNumberGeneratorHarness(t)
	ids := fetchUserIDs(t, pool, 2)
	creatorID, checkerID := ids[0], ids[1]
	svc := NewVendorRequestService(pool)
	ctx := context.Background()
	creator := Actor{UserID: creatorID, Role: "ATM-USER"}
	checker := Actor{UserID: checkerID, Role: "ATM-SPV"}

	created, err := svc.Create(ctx, creator, manualCreateInput(vendorID, terminalID, 50000))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Submit(ctx, creator, created.ID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := svc.Approve(ctx, checker, created.ID); err != nil {
		t.Fatalf("approve: %v", err)
	}

	if _, err := svc.Cancel(ctx, creator, created.ID, "test reason"); !errors.Is(err, ErrNotChecker) {
		t.Fatalf("non-checker creator cancel on approved request error = %v, want ErrNotChecker", err)
	}

	if _, err := svc.Cancel(ctx, checker, created.ID, "test reason"); err != nil {
		t.Fatalf("checker cancel on approved request: %v", err)
	}

	var status string
	var isCanceled bool
	if err := pool.QueryRow(ctx, `SELECT status, is_canceled FROM vendor_requests WHERE id = $1`, created.ID).
		Scan(&status, &isCanceled); err != nil {
		t.Fatalf("read back vendor_requests row: %v", err)
	}
	if status != "cancelled" || !isCanceled {
		t.Errorf("status=%q is_canceled=%v, want status=cancelled is_canceled=true", status, isCanceled)
	}
}

// TestIntegration_Cancel_ApprovedPreservesRowsAndSingleAudit covers Task 8.4
// (Property 9, Req 3.4, 3.6): canceling an approved request with N items sets
// is_canceled=true, keeps the row and every item (soft cancel, no delete),
// and writes exactly one 'cancel' audit entry carrying actor/before/after
// (including the reason)/ip/created_at.
func TestIntegration_Cancel_ApprovedPreservesRowsAndSingleAudit(t *testing.T) {
	pool, vendorID, _, terminalID := setupNumberGeneratorHarness(t)
	ids := fetchUserIDs(t, pool, 2)
	creatorID, checkerID := ids[0], ids[1]
	svc := NewVendorRequestService(pool)
	ctx := context.Background()
	creator := Actor{UserID: creatorID, Role: "ATM-USER"}
	checker := Actor{UserID: checkerID, Role: "ATM-SPV", IP: "203.0.113.5"}

	today := jakartaCalendarDate(0)
	created, err := svc.Create(ctx, creator, CreateVendorRequestInput{
		ForecastDate:    today,
		ReplenishDate:   today,
		RequestCategory: "emergency",
		IsManual:        true,
		VendorID:        vendorID,
		Items: []ItemInput{
			{TerminalID: terminalID, Denom: 50000, AmountReplenish: 1000000},
			{TerminalID: terminalID, Denom: 100000, AmountReplenish: 2000000},
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Submit(ctx, creator, created.ID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := svc.Approve(ctx, checker, created.ID); err != nil {
		t.Fatalf("approve: %v", err)
	}

	const reason = "vendor batal, order salah kirim"
	if _, err := svc.Cancel(ctx, checker, created.ID, reason); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	var status string
	var isCanceled bool
	if err := pool.QueryRow(ctx, `SELECT status, is_canceled FROM vendor_requests WHERE id = $1`, created.ID).
		Scan(&status, &isCanceled); err != nil {
		t.Fatalf("read back vendor_requests row: %v", err)
	}
	if status != "cancelled" || !isCanceled {
		t.Fatalf("status=%q is_canceled=%v, want status=cancelled is_canceled=true", status, isCanceled)
	}

	var itemCount int64
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM vendor_request_items WHERE vendor_request_id = $1`, created.ID).
		Scan(&itemCount); err != nil {
		t.Fatalf("count items: %v", err)
	}
	if itemCount != 2 {
		t.Fatalf("item count = %d, want 2 (cancel must not delete items)", itemCount)
	}

	rows, err := pool.Query(ctx, `
		SELECT actor_id, before, after, ip, created_at
		FROM audit_logs WHERE entity_type = 'vendor_request' AND entity_id = $1 AND action = 'cancel'`,
		created.ID)
	if err != nil {
		t.Fatalf("query cancel audit entries: %v", err)
	}
	defer rows.Close()

	testStart := time.Now().Add(-time.Minute)
	count := 0
	for rows.Next() {
		count++
		var actorID int64
		var before, after []byte
		var ip *string
		var createdAt time.Time
		if err := rows.Scan(&actorID, &before, &after, &ip, &createdAt); err != nil {
			t.Fatalf("scan audit row: %v", err)
		}
		if actorID != checkerID {
			t.Errorf("audit actor_id = %d, want %d", actorID, checkerID)
		}
		// jsonb's text output isn't the exact bytes json.Marshal produced on
		// insert (e.g. Postgres prints a space after ':'), so compare parsed
		// values instead of raw substrings.
		var beforeMap, afterMap map[string]any
		if err := json.Unmarshal(before, &beforeMap); err != nil {
			t.Fatalf("unmarshal audit before: %v", err)
		}
		if err := json.Unmarshal(after, &afterMap); err != nil {
			t.Fatalf("unmarshal audit after: %v", err)
		}
		if beforeMap["state"] != "approved" {
			t.Errorf("audit before[state] = %v, want approved", beforeMap["state"])
		}
		if afterMap["reason"] != reason {
			t.Errorf("audit after[reason] = %v, want %q", afterMap["reason"], reason)
		}
		if ip == nil || *ip != checker.IP {
			t.Errorf("audit ip = %v, want %q", ip, checker.IP)
		}
		if createdAt.Before(testStart) {
			t.Errorf("audit created_at = %v, want a timestamp from this test run", createdAt)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate audit rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("cancel audit entries = %d, want exactly 1", count)
	}
}

// TestIntegration_Cancel_ApprovedIdempotency covers Task 8.5 (Property 11,
// Req 3.8): a second cancel of an already-canceled (formerly approved)
// request is rejected with ErrAlreadyCanceled, is_canceled stays true, and no
// second 'cancel' audit entry is written.
func TestIntegration_Cancel_ApprovedIdempotency(t *testing.T) {
	pool, vendorID, _, terminalID := setupNumberGeneratorHarness(t)
	ids := fetchUserIDs(t, pool, 2)
	creatorID, checkerID := ids[0], ids[1]
	svc := NewVendorRequestService(pool)
	ctx := context.Background()
	creator := Actor{UserID: creatorID, Role: "ATM-USER"}
	checker := Actor{UserID: checkerID, Role: "ATM-SPV"}

	created, err := svc.Create(ctx, creator, manualCreateInput(vendorID, terminalID, 50000))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Submit(ctx, creator, created.ID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := svc.Approve(ctx, checker, created.ID); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := svc.Cancel(ctx, checker, created.ID, "first cancel"); err != nil {
		t.Fatalf("first cancel: %v", err)
	}

	_, err = svc.Cancel(ctx, checker, created.ID, "second cancel attempt")
	if !errors.Is(err, ErrAlreadyCanceled) {
		t.Fatalf("second cancel error = %v, want ErrAlreadyCanceled", err)
	}

	var isCanceled bool
	if err := pool.QueryRow(ctx, `SELECT is_canceled FROM vendor_requests WHERE id = $1`, created.ID).Scan(&isCanceled); err != nil {
		t.Fatalf("read back is_canceled: %v", err)
	}
	if !isCanceled {
		t.Errorf("is_canceled = false after duplicate cancel attempt, want still true")
	}
	if got := countAuditEntries(t, pool, created.ID, "cancel"); got != 1 {
		t.Errorf("cancel audit entries after duplicate attempt = %d, want still exactly 1", got)
	}
}

// TestIntegration_List_ExcludesCanceledByDefault covers Task 8.6 (Property
// 12, Req 3.9): List excludes canceled requests unless IncludeCanceled=true.
func TestIntegration_List_ExcludesCanceledByDefault(t *testing.T) {
	pool, vendorID, createdBy, terminalID := setupNumberGeneratorHarness(t)
	svc := NewVendorRequestService(pool)
	ctx := context.Background()
	actor := Actor{UserID: createdBy, Role: "ADMIN"}

	kept, err := svc.Create(ctx, actor, manualCreateInput(vendorID, terminalID, 50000))
	if err != nil {
		t.Fatalf("create kept: %v", err)
	}
	canceled, err := svc.Create(ctx, actor, manualCreateInput(vendorID, terminalID, 100000))
	if err != nil {
		t.Fatalf("create to-be-canceled: %v", err)
	}
	if _, err := svc.Cancel(ctx, actor, canceled.ID, "test reason"); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	defaultResult, err := svc.List(ctx, ListVendorRequestParams{CreatedBy: createdBy, Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("List (default): %v", err)
	}
	assertContainsID := func(t *testing.T, rows []VendorRequestSummary, id int64, want bool) {
		t.Helper()
		found := false
		for _, r := range rows {
			if r.ID == id {
				found = true
			}
		}
		if found != want {
			t.Errorf("request %d present = %v, want %v", id, found, want)
		}
	}
	assertContainsID(t, defaultResult.Data, kept.ID, true)
	assertContainsID(t, defaultResult.Data, canceled.ID, false)

	includedResult, err := svc.List(ctx, ListVendorRequestParams{CreatedBy: createdBy, IncludeCanceled: true, Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("List (include_canceled): %v", err)
	}
	assertContainsID(t, includedResult.Data, kept.ID, true)
	assertContainsID(t, includedResult.Data, canceled.ID, true)
}
