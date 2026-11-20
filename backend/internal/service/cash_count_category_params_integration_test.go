//go:build integration

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/approval"
	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/repository"
)

// ScDD 5.1 task 4.4 (AC-5.1-04, AC-5.1-13): real submit + two-step approval for
// "Parameter Kategori Vault". Shares the dev DB with other tests, so it records the
// active set it found and cleanup puts it back: the DB is never left without
// exactly one active set.
func TestCashCountCategoryParams_SubmitApprove_Integration(t *testing.T) {
	_, tag, h := harness(t) // skips without DATABASE_URL
	ctx, pool := h.ctx, h.pool

	type set struct {
		ID                int64
		Min, Max          string
		High, Medium, Low int16
		Superseded        bool
	}
	load := func(where string, args ...any) []set {
		t.Helper()
		rows, err := pool.Query(ctx, `SELECT id, medium_min_amount::text, medium_max_amount::text, onsite_high, onsite_medium, onsite_low, superseded_at IS NOT NULL
			FROM cash_count_category_params WHERE `+where+` ORDER BY id`, args...)
		if err != nil {
			t.Fatalf("load sets: %v", err)
		}
		defer rows.Close()
		var out []set
		for rows.Next() {
			var s set
			if err := rows.Scan(&s.ID, &s.Min, &s.Max, &s.High, &s.Medium, &s.Low, &s.Superseded); err != nil {
				t.Fatalf("scan set: %v", err)
			}
			out = append(out, s)
		}
		return out
	}
	activeSets := func() []set { return load(`superseded_at IS NULL`) }

	orig := activeSets()
	if len(orig) != 1 {
		t.Fatalf("precondition: %d active sets, want 1", len(orig))
	}
	origSet := orig[0]
	historyBefore := len(load(`true`))

	// maker (CC-CWM) -> approver1 (level 1, the checker) -> approver2 (level 2).
	user := func(suffix, role string, supervisor *int64, level *int32) int64 {
		t.Helper()
		name := "itccp" + tag + "-" + suffix
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source, supervisor_id, approval_level)
			SELECT id, $1, $1, $1 || '@ccp.example.com', true, 'ldap', $2::bigint, $3::int FROM roles WHERE role = $4 RETURNING id`,
			name, supervisor, level, role).Scan(&id); err != nil {
			t.Fatalf("seed user %s: %v", suffix, err)
		}
		return id
	}
	one, two := int32(1), int32(2)
	approver2 := user("a2", "ADMIN", nil, &two)
	approver1 := user("a1", "ADMIN", &approver2, &one)
	maker := user("m", "CC-CWM", &approver1, nil)
	users := []int64{maker, approver1, approver2}

	t.Cleanup(func() { // LIFO: runs before harness cleanup
		c := context.Background()
		for _, s := range []string{
			`DELETE FROM notification_emails WHERE notification_id IN (SELECT id FROM notifications WHERE recipient_user_id = ANY($1))`,
			`DELETE FROM notifications WHERE recipient_user_id = ANY($1)`,
			`DELETE FROM audit_logs WHERE actor_id = ANY($1)`,
			`DELETE FROM cash_count_category_params WHERE created_by = ANY($1)`,
			`DELETE FROM master_data_change_requests WHERE maker_id = ANY($1)`,
			`DELETE FROM approval_steps WHERE request_id IN (SELECT id FROM approval_requests WHERE maker_id = ANY($1))`,
			`DELETE FROM approval_requests WHERE maker_id = ANY($1)`,
			`DELETE FROM users WHERE id = ANY($1)`,
		} {
			if _, err := pool.Exec(c, s, users); err != nil {
				t.Errorf("cleanup %q: %v", s, err)
			}
		}
		// Restore the original active set (test rows are gone, so the active-unique index allows it).
		if _, err := pool.Exec(c, `UPDATE cash_count_category_params SET superseded_at = NULL
			WHERE id = $1 AND NOT EXISTS (SELECT 1 FROM cash_count_category_params WHERE superseded_at IS NULL)`, origSet.ID); err != nil {
			t.Errorf("cleanup: restore active set %d: %v", origSet.ID, err)
		}
		var n int
		if err := pool.QueryRow(c, `SELECT count(*) FROM cash_count_category_params WHERE superseded_at IS NULL`).Scan(&n); err != nil || n != 1 {
			t.Errorf("cleanup: %d active sets (err %v), want 1", n, err)
		}
	})

	repo := approval.NewRepository(pool)
	auditWriter := audit.NewWriter(pool)
	orchestrator := approval.NewOrchestrator(repo, repo, repo, auditWriter, nil)
	changeRepo := repository.NewMasterDataChangeRepository(pool)
	params := NewCashCountCategoryParamsAdminService(
		repository.NewCashCountCategoryParamsAdminRepository(pool, pool),
		NewMasterDataChangeService(changeRepo, orchestrator, auditWriter))
	approvals := NewMasterDataApprovalService(orchestrator, changeRepo, pool,
		ApplierRegistry{cashCountCategoryParamsEntity: CashCountCategoryParamsApplier{}}, auditWriter)
	makerCtx := ctxWithRole(maker, "CC-CWM")

	submit := func(minAmt, maxAmt string, hi, med, lo int16) (changeID, requestID int64) {
		t.Helper()
		change, err := params.Create(makerCtx, maker, CashCountCategoryParamsPayload{
			MediumMinAmount: minAmt, MediumMaxAmount: maxAmt, OnsiteHigh: hi, OnsiteMedium: med, OnsiteLow: lo,
		}, "127.0.0.1")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if change.ApprovalRequestID == nil {
			t.Fatal("change has no approval request")
		}
		return change.ID, *change.ApprovalRequestID
	}
	unchanged := func(step string) {
		t.Helper()
		if got := activeSets(); len(got) != 1 || got[0] != origSet {
			t.Fatalf("%s: active sets = %+v, want only %+v", step, got, origSet)
		}
	}

	// Two pending proposals at once are allowed; the second one approved must not apply.
	changeA, reqA := submit("7000000000", "20000000000.5", 4, 3, 2)
	changeB, reqB := submit("6000000000.00", "16000000000.00", 5, 4, 3)
	unchanged("after submit")

	// Maker can't approve their own change, and the checker out of turn can't either: nothing changes.
	for _, who := range []int64{maker, approver2} {
		if _, err := approvals.Approve(adminCtx(who), reqA, who, "127.0.0.1"); !errors.Is(err, approval.ErrNotAuthorized) {
			t.Fatalf("Approve by %d = %v, want approval.ErrNotAuthorized", who, err)
		}
	}
	unchanged("after refused approve")
	if st := changeStatus(t, pool, changeA); st != "pending" {
		t.Fatalf("change A after refused approve = %q, want pending", st)
	}

	// Checker approve (master_data policy = one step): new set active, old set kept as history, exactly one active.
	if _, err := approvals.Approve(adminCtx(approver1), reqA, approver1, "127.0.0.1"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if st := changeStatus(t, pool, changeA); st != "applied" {
		t.Fatalf("change A = %q, want applied", st)
	}
	active := activeSets()
	if len(active) != 1 {
		t.Fatalf("after approve: %d active sets, want 1", len(active))
	}
	want := set{ID: active[0].ID, Min: "7000000000.00", Max: "20000000000.50", High: 4, Medium: 3, Low: 2}
	if active[0] != want || active[0].ID == origSet.ID {
		t.Errorf("active set = %+v, want %+v (new id)", active[0], want)
	}
	if old := load(`id = $1`, origSet.ID); len(old) != 1 || !old[0].Superseded {
		t.Errorf("original set %d = %+v, want kept and superseded", origSet.ID, old)
	}
	if n := len(load(`true`)); n != historyBefore+1 {
		t.Errorf("history rows = %d, want %d", n, historyBefore+1)
	}
	var createdBy int64
	if err := pool.QueryRow(ctx, `SELECT created_by FROM cash_count_category_params WHERE id = $1`, active[0].ID).Scan(&createdBy); err != nil || createdBy != maker {
		t.Errorf("created_by = %d (err %v), want maker %d", createdBy, err, maker)
	}

	// AC-13: one apply audit by the checker with before (old set) / after (new set); the change keeps the maker.
	var beforeJSON, afterJSON []byte
	if err := pool.QueryRow(ctx, `SELECT before, after FROM audit_logs
		WHERE action = 'master_data_applied' AND entity_type = $1 AND entity_id = $2 AND actor_id = $3`,
		cashCountCategoryParamsEntity, active[0].ID, approver1).Scan(&beforeJSON, &afterJSON); err != nil {
		t.Fatalf("apply audit by checker: %v", err)
	}
	var before, after struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(beforeJSON, &before); err != nil || before.ID != origSet.ID {
		t.Errorf("audit before = %s (err %v), want set %d", beforeJSON, err, origSet.ID)
	}
	if err := json.Unmarshal(afterJSON, &after); err != nil || after.ID != active[0].ID {
		t.Errorf("audit after = %s (err %v), want set %d", afterJSON, err, active[0].ID)
	}
	var changeMaker int64
	if err := pool.QueryRow(ctx, `SELECT maker_id FROM master_data_change_requests WHERE id = $1`, changeA).Scan(&changeMaker); err != nil || changeMaker != maker {
		t.Errorf("change maker_id = %d (err %v), want %d", changeMaker, err, maker)
	}

	// B was proposed against the original set: approving it now must not apply.
	if _, err := approvals.Approve(adminCtx(approver1), reqB, approver1, "127.0.0.1"); err == nil {
		t.Error("approve B (stale): want apply error")
	}
	if st := changeStatus(t, pool, changeB); st == "applied" {
		t.Errorf("change B = %q, want not applied", st)
	}
	if got := activeSets(); len(got) != 1 || got[0] != active[0] {
		t.Errorf("after stale B: active sets = %+v, want only %+v", got, active[0])
	}
}
