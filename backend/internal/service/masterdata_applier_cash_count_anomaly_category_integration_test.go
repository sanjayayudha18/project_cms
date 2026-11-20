//go:build integration

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// ScDD 5.1 task 5.3 (AC-5.1-05, AC-5.1-13): lifecycle of cash_count_anomaly_category
// through the real apply-on-approve path. Skips without DATABASE_URL (harness).

// anomalyHarness is harness() with the anomaly applier registered; it removes the
// rows (and their audit entries) this test created, by unique code prefix.
func anomalyHarness(t *testing.T) (*MasterDataApprovalService, string, *testHelpers) {
	t.Helper()
	svc, tag, h := harness(t)
	svc.appliers[cashCountAnomalyCategoryEntity] = CashCountAnomalyCategoryApplier{}
	t.Cleanup(func() {
		pat := "ITANO%"
		if _, err := h.pool.Exec(h.ctx, `DELETE FROM audit_logs WHERE entity_type = $1
			AND entity_id IN (SELECT id FROM cash_count_anomaly_categories WHERE code LIKE $2)`, cashCountAnomalyCategoryEntity, pat); err != nil {
			t.Logf("cleanup: delete audit_logs (anomaly): %v", err)
		}
		if _, err := h.pool.Exec(h.ctx, `DELETE FROM cash_count_anomaly_categories WHERE code LIKE $1`, pat); err != nil {
			t.Logf("cleanup: delete anomaly categories: %v", err)
		}
	})
	return svc, tag, h
}

func anomalyBefore(t *testing.T, h *testHelpers, id int64) *db.GetCashCountAnomalyCategoryAdminByIDRow {
	t.Helper()
	row, err := db.New(h.pool).GetCashCountAnomalyCategoryAdminByID(h.ctx, id)
	if err != nil {
		t.Fatalf("load anomaly category %d: %v", id, err)
	}
	return &row
}

func TestCashCountAnomalyCategoryApplier_Lifecycle_Integration(t *testing.T) {
	svc, tag, h := anomalyHarness(t)
	ctx := context.Background()
	q := dbtx(svc)
	code := "ITANO_" + tag[len(tag)-10:]
	desc := "deskripsi awal"

	// Nothing before approval (AC-5.1-05 maker-checker): the pending row is not applied.
	createID := h.insertPending(cashCountAnomalyCategoryEntity, "create", nil,
		CashCountAnomalyCategoryPayload{Code: code, Name: "Mesin Rusak", Description: &desc})
	var n int
	if err := q.QueryRow(ctx, `SELECT count(*) FROM cash_count_anomaly_categories WHERE code = $1`, code).Scan(&n); err != nil || n != 0 {
		t.Fatalf("before approve: rows=%d err=%v, want 0", n, err)
	}

	// create
	if err := approveChange(t, svc, createID); err != nil {
		t.Fatalf("create: %v", err)
	}
	var id int64
	var name string
	var active bool
	if err := q.QueryRow(ctx, `SELECT id, name, is_active FROM cash_count_anomaly_categories WHERE code = $1`, code).Scan(&id, &name, &active); err != nil {
		t.Fatalf("created row: %v", err)
	}
	if name != "Mesin Rusak" || !active {
		t.Errorf("created: name=%q active=%v", name, active)
	}
	if got := changeStatus(t, h.pool, createID); got != "applied" {
		t.Errorf("create change status = %q, want applied", got)
	}

	// duplicate code -> unique violation at apply, nothing inserted, change not applied
	dupID := h.insertPending(cashCountAnomalyCategoryEntity, "create", nil,
		CashCountAnomalyCategoryPayload{Code: code, Name: "Duplikat"})
	if err := approveChange(t, svc, dupID); err == nil {
		t.Error("duplicate code: want apply error")
	}
	if got := changeStatus(t, h.pool, dupID); got == "applied" {
		t.Errorf("duplicate change status = applied, want failed")
	}
	if err := q.QueryRow(ctx, `SELECT count(*) FROM cash_count_anomaly_categories WHERE code = $1`, code).Scan(&n); err != nil || n != 1 {
		t.Fatalf("rows for code = %d (err %v), want 1", n, err)
	}

	// update (with the real Before snapshot: no false stale)
	newDesc := "deskripsi baru"
	if err := approveChange(t, svc, h.insertPendingWithBefore(cashCountAnomalyCategoryEntity, "update", &id,
		CashCountAnomalyCategoryUpdatePayload{Name: "Mesin Rusak Berat", Description: &newDesc}, anomalyBefore(t, h, id))); err != nil {
		t.Fatalf("update: %v", err)
	}
	var gotDesc *string
	if err := q.QueryRow(ctx, `SELECT name, description FROM cash_count_anomaly_categories WHERE id = $1`, id).Scan(&name, &gotDesc); err != nil {
		t.Fatal(err)
	}
	if name != "Mesin Rusak Berat" || gotDesc == nil || *gotDesc != newDesc {
		t.Errorf("after update: name=%q desc=%v", name, gotDesc)
	}

	// disable -> the "active" list no longer shows it
	if err := approveChange(t, svc, h.insertPendingWithBefore(cashCountAnomalyCategoryEntity, "disable", &id, nil, anomalyBefore(t, h, id))); err != nil {
		t.Fatalf("disable: %v", err)
	}
	var deleted bool
	if err := q.QueryRow(ctx, `SELECT is_active, deleted_at IS NOT NULL FROM cash_count_anomaly_categories WHERE id = $1`, id).Scan(&active, &deleted); err != nil {
		t.Fatal(err)
	}
	if active || !deleted {
		t.Errorf("after disable: active=%v deleted_at set=%v, want false/true", active, deleted)
	}
	if err := q.QueryRow(ctx, `SELECT count(*) FROM cash_count_anomaly_categories WHERE code = $1 AND is_active`, code).Scan(&n); err != nil || n != 0 {
		t.Errorf("active rows after disable = %d (err %v), want 0", n, err)
	}

	// enable
	if err := approveChange(t, svc, h.insertPendingWithBefore(cashCountAnomalyCategoryEntity, "enable", &id, nil, anomalyBefore(t, h, id))); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := q.QueryRow(ctx, `SELECT is_active, deleted_at IS NOT NULL FROM cash_count_anomaly_categories WHERE id = $1`, id).Scan(&active, &deleted); err != nil {
		t.Fatal(err)
	}
	if !active || deleted {
		t.Errorf("after enable: active=%v deleted_at set=%v, want true/false", active, deleted)
	}

	// AC-5.1-13: one master_data_applied audit per applied change (create, update, disable, enable),
	// with before/after on the update.
	if n := countInt(t, h.pool, `SELECT count(*) FROM audit_logs WHERE entity_type = $1 AND action = 'master_data_applied' AND entity_id = $2`,
		cashCountAnomalyCategoryEntity, id); n != 4 {
		t.Errorf("master_data_applied audits = %d, want 4", n)
	}
	var auditBefore, auditAfter []byte
	if err := q.QueryRow(ctx, `SELECT before, after FROM audit_logs WHERE entity_type = $1 AND action = 'master_data_applied' AND entity_id = $2
		ORDER BY id LIMIT 1 OFFSET 1`, cashCountAnomalyCategoryEntity, id).Scan(&auditBefore, &auditAfter); err != nil {
		t.Fatalf("load update audit: %v", err)
	}
	if !json.Valid(auditBefore) || !json.Valid(auditAfter) || len(auditBefore) == 0 || len(auditAfter) == 0 {
		t.Errorf("update audit missing before/after: before=%s after=%s", auditBefore, auditAfter)
	}
}

// A change whose Before snapshot no longer matches is retired as stale and not applied.
func TestCashCountAnomalyCategoryApplier_Stale_Integration(t *testing.T) {
	svc, tag, h := anomalyHarness(t)
	ctx := context.Background()
	code := "ITANOS_" + tag[len(tag)-10:]

	if err := approveChange(t, svc, h.insertPending(cashCountAnomalyCategoryEntity, "create", nil,
		CashCountAnomalyCategoryPayload{Code: code, Name: "Awal"})); err != nil {
		t.Fatalf("create: %v", err)
	}
	var id int64
	if err := dbtx(svc).QueryRow(ctx, `SELECT id FROM cash_count_anomaly_categories WHERE code = $1`, code).Scan(&id); err != nil {
		t.Fatal(err)
	}
	stale := anomalyBefore(t, h, id) // snapshot, then someone else changes the row
	if err := approveChange(t, svc, h.insertPendingWithBefore(cashCountAnomalyCategoryEntity, "update", &id,
		CashCountAnomalyCategoryUpdatePayload{Name: "Diubah Lain"}, anomalyBefore(t, h, id))); err != nil {
		t.Fatalf("concurrent update: %v", err)
	}

	staleID := h.insertPendingWithBefore(cashCountAnomalyCategoryEntity, "update", &id,
		CashCountAnomalyCategoryUpdatePayload{Name: "Dari Snapshot Lama"}, stale)
	if err := approveChange(t, svc, staleID); err == nil {
		t.Fatal("stale update: want error")
	}
	var name string
	if err := dbtx(svc).QueryRow(ctx, `SELECT name FROM cash_count_anomaly_categories WHERE id = $1`, id).Scan(&name); err != nil || name != "Diubah Lain" {
		t.Errorf("name = %q (err %v), want Diubah Lain (stale change must not apply)", name, err)
	}
	if got := changeStatus(t, h.pool, staleID); got != "stale" {
		t.Errorf("stale change status = %q, want stale", got)
	}
}

// Rejected by the checker -> nothing changes (AC-5.1-05 maker-checker).
func TestCashCountAnomalyCategoryApplier_Reject_Integration(t *testing.T) {
	svc, tag, h := anomalyHarness(t)
	ctx := context.Background()
	code := "ITANOR_" + tag[len(tag)-10:]

	changeID := h.insertPending(cashCountAnomalyCategoryEntity, "create", nil,
		CashCountAnomalyCategoryPayload{Code: code, Name: "Ditolak"})
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "rejected"}}
	if _, err := svc.Reject(ctx, 999, 1, "127.0.0.1"); err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if got := changeStatus(t, h.pool, changeID); got != "rejected" {
		t.Errorf("change status = %q, want rejected", got)
	}
	if n := countInt(t, h.pool, `SELECT count(*) FROM cash_count_anomaly_categories WHERE code = $1`, code); n != 0 {
		t.Errorf("rows after reject = %d, want 0", n)
	}
}
