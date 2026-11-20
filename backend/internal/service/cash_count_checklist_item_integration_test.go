//go:build integration

package service

import (
	"context"
	"os"
	"regexp"
	"testing"
)

// Task 5.1/6.3 (AC-5.1-06, AC-5.1-07, AC-5.1-13): checklist-item lifecycle through the real
// apply-on-approve path against the dev DB. Every row this test writes carries the item_text
// prefix below and is deleted on cleanup, so the dev checklist is empty again afterwards.

const checklistITPrefix = "IT63-"

func checklistHarness(t *testing.T) (*MasterDataApprovalService, string, *testHelpers) {
	t.Helper()
	svc, tag, h := harness(t) // skips without DATABASE_URL
	svc.appliers["cash_count_checklist_item"] = CashCountChecklistItemApplier{}
	t.Cleanup(func() { // LIFO: runs before harness cleanup
		ids := `SELECT id FROM cash_count_checklist_items WHERE item_text LIKE $1`
		pat := checklistITPrefix + tag + "%"
		if _, err := h.pool.Exec(h.ctx, `DELETE FROM audit_logs WHERE entity_type = 'cash_count_checklist_item' AND entity_id IN (`+ids+`)`, pat); err != nil {
			t.Errorf("cleanup audit_logs: %v", err)
		}
		if _, err := h.pool.Exec(h.ctx, `DELETE FROM cash_count_checklist_items WHERE item_text LIKE $1`, pat); err != nil {
			t.Errorf("cleanup checklist items: %v", err)
		}
	})
	return svc, tag, h
}

// AC-5.1-07: the migration seeds no checklist items.
func TestCashCountChecklist_MigrationSeedsNothing_Integration(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/041_cash_count_parameters.sql")
	if err != nil {
		t.Fatalf("read migration 041: %v", err)
	}
	if regexp.MustCompile(`(?i)INSERT\s+INTO\s+(public\.)?cash_count_checklist_items`).Match(raw) {
		t.Error("migration 041 inserts into cash_count_checklist_items; AC-5.1-07 says it must not")
	}

	_, _, h := checklistHarness(t)
	var n int
	if err := h.pool.QueryRow(h.ctx, `SELECT count(*) FROM cash_count_checklist_items WHERE item_text NOT LIKE $1`, checklistITPrefix+"%").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("dev checklist has %d non-test rows, want 0 (AC-5.1-07)", n)
	}
}

func TestCashCountChecklistItemApplier_Lifecycle_Integration(t *testing.T) {
	svc, tag, h := checklistHarness(t)
	ctx := context.Background()
	text := func(s string) string { return checklistITPrefix + tag + " " + s }
	const entity = "cash_count_checklist_item"
	day1 := "DAY1"

	// create: nothing exists until approve
	create := h.insertPending(entity, "create", nil, CashCountChecklistItemPayload{Category: "SELECTED_MACHINE", Day: &day1, PointNo: 901, ItemText: text("a")})
	var n int
	if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM cash_count_checklist_items WHERE item_text = $1`, text("a")).Scan(&n); err != nil || n != 0 {
		t.Fatalf("before approve: rows=%d err=%v, want 0", n, err)
	}
	if err := approveChange(t, svc, create); err != nil {
		t.Fatalf("create: %v", err)
	}
	var id int64
	var gotDay *string
	var active bool
	if err := h.pool.QueryRow(ctx, `SELECT id, day, is_active FROM cash_count_checklist_items WHERE item_text = $1`, text("a")).Scan(&id, &gotDay, &active); err != nil {
		t.Fatalf("created item: %v", err)
	}
	if gotDay == nil || *gotDay != "DAY1" || !active {
		t.Errorf("created item day=%v active=%v, want DAY1/true", gotDay, active)
	}

	// AC-13: audit row written with the apply, after state present, before absent on create
	assertAudit := func(wantBefore bool) {
		t.Helper()
		var cnt int
		if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs
			WHERE entity_type = $1 AND entity_id = $2 AND action = 'master_data_applied'
			  AND (before IS NOT NULL) = $3`, entity, id, wantBefore).Scan(&cnt); err != nil || cnt == 0 {
			t.Errorf("audit (before=%v): count=%d err=%v, want >=1", wantBefore, cnt, err)
		}
	}
	assertAudit(false)

	// duplicate active point (same category+day) -> unique index refuses at apply
	if err := approveChange(t, svc, h.insertPending(entity, "create", nil, CashCountChecklistItemPayload{Category: "SELECTED_MACHINE", Day: &day1, PointNo: 901, ItemText: text("dup")})); err == nil {
		t.Error("duplicate point: want apply error")
	}
	// same point on the other day is fine
	day2 := "DAY2"
	if err := approveChange(t, svc, h.insertPending(entity, "create", nil, CashCountChecklistItemPayload{Category: "SELECTED_MACHINE", Day: &day2, PointNo: 901, ItemText: text("d2")})); err != nil {
		t.Errorf("same point on DAY2: %v", err)
	}
	// NON_MACHINE with a day -> table CHECK refuses; SELECTED_MACHINE without day likewise
	if err := approveChange(t, svc, h.insertPending(entity, "create", nil, CashCountChecklistItemPayload{Category: "NON_MACHINE", Day: &day1, PointNo: 902, ItemText: text("bad1")})); err == nil {
		t.Error("NON_MACHINE with day: want apply error")
	}
	if err := approveChange(t, svc, h.insertPending(entity, "create", nil, CashCountChecklistItemPayload{Category: "SELECTED_MACHINE", PointNo: 903, ItemText: text("bad2")})); err == nil {
		t.Error("SELECTED_MACHINE without day: want apply error")
	}
	if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM cash_count_checklist_items WHERE item_text IN ($1, $2, $3)`, text("dup"), text("bad1"), text("bad2")).Scan(&n); err != nil || n != 0 {
		t.Errorf("rejected creates left %d rows (err %v), want 0", n, err)
	}

	// stale Before -> not applied
	if err := approveChange(t, svc, h.insertPendingWithBefore(entity, "update", &id,
		CashCountChecklistItemUpdatePayload{Day: &day1, PointNo: 901, ItemText: text("stale")},
		map[string]any{"id": id, "category": "SELECTED_MACHINE", "day": "DAY1", "point_no": 1, "item_text": "not what is stored", "is_active": true})); err == nil {
		t.Error("stale update: want error")
	}
	var gotText string
	if err := h.pool.QueryRow(ctx, `SELECT item_text FROM cash_count_checklist_items WHERE id = $1`, id).Scan(&gotText); err != nil || gotText != text("a") {
		t.Fatalf("after stale update text=%q err=%v, want unchanged", gotText, err)
	}

	// update (Before = current state, as the service stores it)
	rtx, err := h.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cur, err := CashCountChecklistItemApplier{}.CurrentState(ctx, rtx, id)
	_ = rtx.Rollback(ctx)
	if err != nil {
		t.Fatalf("current state: %v", err)
	}
	if err := approveChange(t, svc, h.insertPendingWithBefore(entity, "update", &id,
		CashCountChecklistItemUpdatePayload{Day: &day1, PointNo: 905, ItemText: text("a2")}, cur)); err != nil {
		t.Fatalf("update: %v", err)
	}
	var pt int16
	if err := h.pool.QueryRow(ctx, `SELECT point_no, item_text FROM cash_count_checklist_items WHERE id = $1`, id).Scan(&pt, &gotText); err != nil || pt != 905 || gotText != text("a2") {
		t.Errorf("after update point=%d text=%q err=%v", pt, gotText, err)
	}
	assertAudit(true)

	// disable, then the point is reusable; re-enabling the original while reused -> refused
	if err := approveChange(t, svc, h.insertPending(entity, "disable", &id, nil)); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if err := h.pool.QueryRow(ctx, `SELECT is_active FROM cash_count_checklist_items WHERE id = $1`, id).Scan(&active); err != nil || active {
		t.Fatalf("after disable active=%v err=%v", active, err)
	}
	if err := approveChange(t, svc, h.insertPending(entity, "create", nil, CashCountChecklistItemPayload{Category: "SELECTED_MACHINE", Day: &day1, PointNo: 905, ItemText: text("reuse")})); err != nil {
		t.Fatalf("reuse freed point: %v", err)
	}
	if err := approveChange(t, svc, h.insertPending(entity, "enable", &id, nil)); err == nil {
		t.Error("enable while point reused: want apply error")
	}
	if err := h.pool.QueryRow(ctx, `SELECT is_active FROM cash_count_checklist_items WHERE id = $1`, id).Scan(&active); err != nil || active {
		t.Errorf("after refused enable active=%v err=%v, want false", active, err)
	}

	// free the point again, then enable works
	var reuseID int64
	if err := h.pool.QueryRow(ctx, `SELECT id FROM cash_count_checklist_items WHERE item_text = $1`, text("reuse")).Scan(&reuseID); err != nil {
		t.Fatal(err)
	}
	if err := approveChange(t, svc, h.insertPending(entity, "disable", &reuseID, nil)); err != nil {
		t.Fatalf("disable reuse: %v", err)
	}
	if err := approveChange(t, svc, h.insertPending(entity, "enable", &id, nil)); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := h.pool.QueryRow(ctx, `SELECT is_active FROM cash_count_checklist_items WHERE id = $1`, id).Scan(&active); err != nil || !active {
		t.Errorf("after enable active=%v err=%v, want true", active, err)
	}

	// audit rows exist for every applied op on this item (create, update, disable, enable)
	var audits int
	if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE entity_type = $1 AND entity_id = $2 AND action = 'master_data_applied'`, entity, id).Scan(&audits); err != nil || audits < 4 {
		t.Errorf("audit rows for item = %d (err %v), want >= 4", audits, err)
	}
}
