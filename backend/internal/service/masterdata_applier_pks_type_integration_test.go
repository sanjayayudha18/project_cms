//go:build integration

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/approval"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/repository"
)

// Spec vendor-pks-type task 3.4 (integration): PksTypeApplier + PksTypeAdminService against a
// real DB. Skips without DATABASE_URL (harness). Dev must have migration 043.
// Property 4: a type that any vendor_pks row (replaced_at IS NULL, terminated included) uses
// cannot be disabled, at submit (early 409) and at apply (FOR UPDATE + in-use re-check).

// pksTypeHarness is harness() with the applier registered. Cleanup (LIFO) removes the PKS rows,
// audit rows and the types this test created, before harness cleanup deletes vendors/changes.
func pksTypeHarness(t *testing.T) (*MasterDataApprovalService, string, *testHelpers) {
	t.Helper()
	svc, tag, h := harness(t)
	svc.appliers["vendor_pks_type"] = PksTypeApplier{}
	t.Cleanup(func() {
		like := pksTypeTestCode(tag) + "%"
		ex := func(what, sql string) {
			if _, err := h.pool.Exec(h.ctx, sql, like); err != nil {
				t.Logf("cleanup: %s: %v", what, err)
			}
		}
		ex("vendor_pks", `DELETE FROM vendor_pks WHERE pks_type_id IN (SELECT id FROM vendor_pks_types WHERE code LIKE $1)`)
		ex("audit_logs", `DELETE FROM audit_logs WHERE entity_type = 'vendor_pks_type'
			AND entity_id IN (SELECT id FROM vendor_pks_types WHERE code LIKE $1)`)
		ex("vendor_pks_types", `DELETE FROM vendor_pks_types WHERE code LIKE $1`)
	})
	return svc, tag, h
}

// pksTypeTestCode returns a valid, test-unique code prefix (8 chars; callers append 1 char).
func pksTypeTestCode(tag string) string {
	if len(tag) > 7 {
		tag = tag[len(tag)-7:]
	}
	return "T" + tag
}

func pksTypeRow(t testing.TB, h *testHelpers, code string) (db.VendorPksType, bool) {
	t.Helper()
	row, err := db.New(h.pool).GetPksTypeByCode(h.ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.VendorPksType{}, false
	}
	if err != nil {
		t.Fatalf("load type %s: %v", code, err)
	}
	return row, true
}

func pksTypeSnapshot(t testing.TB, h *testHelpers, id int64) db.VendorPksType {
	t.Helper()
	row, err := db.New(h.pool).GetPksType(h.ctx, id)
	if err != nil {
		t.Fatalf("snapshot type %d: %v", id, err)
	}
	return row
}

func pksTypeAudits(t testing.TB, h *testHelpers, id int64) int {
	return countInt(t, h.pool, `SELECT count(*) FROM audit_logs WHERE entity_type = 'vendor_pks_type'
		AND action = 'master_data_applied' AND entity_id = $1`, id)
}

// seedTypeUse inserts a vendor_pks row of the type (not replaced), optionally terminated.
func seedTypeUse(t testing.TB, h *testHelpers, vendorID, typeID int64, wod string, terminated bool) {
	t.Helper()
	sql := `INSERT INTO vendor_pks (vendor_id, pks_type_id, wod_number, pks_name, effective_date, expiry_date, created_by)
		VALUES ($1, $2, $3, 'PKS TEST', '2026-01-01', '2027-12-31', $4)`
	if terminated {
		sql = `INSERT INTO vendor_pks (vendor_id, pks_type_id, wod_number, pks_name, effective_date, expiry_date, created_by,
			terminated_at, terminated_by, termination_reason)
			VALUES ($1, $2, $3, 'PKS TEST', '2026-01-01', '2027-12-31', $4, now(), $4, 'uji')`
	}
	if _, err := h.pool.Exec(h.ctx, sql, vendorID, typeID, wod, h.makerID); err != nil {
		t.Fatalf("seed vendor_pks: %v", err)
	}
}

func approveTypeCreate(t *testing.T, svc *MasterDataApprovalService, h *testHelpers, code, name string) (int64, error) {
	t.Helper()
	id := h.insertPending("vendor_pks_type", "create", nil, PksTypeCreatePayload{Code: code, Name: name})
	return id, approveChange(t, svc, id)
}

func TestPksTypeApplier_Lifecycle_Integration(t *testing.T) {
	svc, tag, h := pksTypeHarness(t)
	code := pksTypeTestCode(tag) + "A"

	// create -> active row + one audit entry in the apply tx
	if _, err := approveTypeCreate(t, svc, h, code, "Tipe Uji"); err != nil {
		t.Fatalf("create: %v", err)
	}
	row, ok := pksTypeRow(t, h, code)
	if !ok || !row.IsActive || row.Name != "Tipe Uji" {
		t.Fatalf("created type = %+v ok=%v", row, ok)
	}
	if n := pksTypeAudits(t, h, row.ID); n != 1 {
		t.Errorf("create audit rows = %d, want 1", n)
	}

	// duplicate code -> unique index refuses at apply; nothing inserted
	dupID, err := approveTypeCreate(t, svc, h, code, "Duplikat")
	if !errors.Is(err, ErrPksTypeConflict) {
		t.Fatalf("duplicate create: err = %v, want ErrPksTypeConflict", err)
	}
	if n := countInt(t, h.pool, `SELECT count(*) FROM vendor_pks_types WHERE upper(code) = $1`, code); n != 1 {
		t.Errorf("types with code %s = %d, want 1", code, n)
	}
	if s := changeStatus(t, h.pool, dupID); s == "applied" {
		t.Errorf("duplicate change status = %q, must not be applied", s)
	}
	if n := pksTypeAudits(t, h, row.ID); n != 1 {
		t.Errorf("audit rows after failed duplicate = %d, want 1 (failed apply writes none)", n)
	}

	// update -> name only; code untouched
	updID := h.insertPendingWithBefore("vendor_pks_type", "update", &row.ID, PksTypeUpdatePayload{Name: "Nama Baru"}, pksTypeSnapshot(t, h, row.ID))
	if err := approveChange(t, svc, updID); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got, _ := pksTypeRow(t, h, code); got.Name != "Nama Baru" || got.Code != code {
		t.Errorf("after update = %+v, want name changed and code kept", got)
	}

	// disable (not in use) -> inactive
	disBefore := pksTypeSnapshot(t, h, row.ID)
	disID := h.insertPendingWithBefore("vendor_pks_type", "disable", &row.ID, nil, disBefore)
	if err := approveChange(t, svc, disID); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if got, _ := pksTypeRow(t, h, code); got.IsActive {
		t.Error("type still active after disable")
	}

	// update staged against the pre-disable snapshot -> stale, not applied
	staleID := h.insertPendingWithBefore("vendor_pks_type", "update", &row.ID, PksTypeUpdatePayload{Name: "Terlambat"}, disBefore)
	if err := approveChange(t, svc, staleID); !errors.Is(err, ErrMasterDataChangeStale) {
		t.Fatalf("update after disable: err = %v, want ErrMasterDataChangeStale", err)
	}
	if got, _ := pksTypeRow(t, h, code); got.Name != "Nama Baru" {
		t.Errorf("name = %q, stale update must not apply", got.Name)
	}

	// enable -> active again; a second enable (type already active) -> stale, not applied
	enID := h.insertPendingWithBefore("vendor_pks_type", "enable", &row.ID, nil, pksTypeSnapshot(t, h, row.ID))
	if err := approveChange(t, svc, enID); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if got, _ := pksTypeRow(t, h, code); !got.IsActive {
		t.Error("type not active after enable")
	}
	en2 := h.insertPending("vendor_pks_type", "enable", &row.ID, nil)
	if err := approveChange(t, svc, en2); !errors.Is(err, ErrMasterDataChangeStale) {
		t.Fatalf("second enable: err = %v, want ErrMasterDataChangeStale", err)
	}
}

// Property 4: disable passed submit, then a PKS of the type appeared before approve.
func TestPksTypeApplier_Disable_PksCreatedBeforeApprove_Refused_Integration(t *testing.T) {
	for _, terminated := range []bool{false, true} {
		t.Run(fmt.Sprintf("terminated=%v", terminated), func(t *testing.T) {
			svc, tag, h := pksTypeHarness(t)
			code := pksTypeTestCode(tag) + "B"
			if _, err := approveTypeCreate(t, svc, h, code, "Dipakai"); err != nil {
				t.Fatalf("create: %v", err)
			}
			typ, _ := pksTypeRow(t, h, code)

			// submit side: nothing uses the type yet, so the service stages the disable.
			sub := &fakeVendorBranchSubmitter{}
			psvc := NewPksTypeAdminService(repository.NewPksTypeRepository(h.pool, h.pool), sub)
			if _, err := psvc.Disable(adminCtx(h.makerID), h.makerID, typ.ID, "ip"); err != nil || !sub.submitCalled {
				t.Fatalf("Disable at submit: err=%v submitted=%v, want staged", err, sub.submitCalled)
			}
			changeID := h.insertPendingWithBefore("vendor_pks_type", "disable", &typ.ID, nil, pksTypeSnapshot(t, h, typ.ID))

			// a PKS of this type is created before the checker approves
			vendorID := h.seedVendor("PT-"+tag, "Vendor Type "+tag)
			t.Cleanup(func() { deleteVendorPksOf(h.ctx, t, h.pool, []int64{vendorID}) })
			seedTypeUse(t, h, vendorID, typ.ID, "WOD-"+tag, terminated)

			if err := approveChange(t, svc, changeID); !errors.Is(err, ErrPksTypeInUse) {
				t.Fatalf("approve disable: err = %v, want ErrPksTypeInUse", err)
			}
			if got, _ := pksTypeRow(t, h, code); !got.IsActive {
				t.Error("type was disabled despite being in use")
			}
			if s := changeStatus(t, h.pool, changeID); s == "applied" {
				t.Errorf("change status = %q, must not be applied", s)
			}
			if n := pksTypeAudits(t, h, typ.ID); n != 1 { // only the create's audit
				t.Errorf("applied audit rows = %d, want 1 (create only)", n)
			}

			// and now the early 409 at submit too (a terminated PKS still blocks)
			sub2 := &fakeVendorBranchSubmitter{}
			psvc = NewPksTypeAdminService(repository.NewPksTypeRepository(h.pool, h.pool), sub2)
			if _, err := psvc.Disable(adminCtx(h.makerID), h.makerID, typ.ID, "ip"); !errors.Is(err, ErrPksTypeInUse) || sub2.submitCalled {
				t.Fatalf("Disable at submit: err = %v submitted=%v, want ErrPksTypeInUse and no submit", err, sub2.submitCalled)
			}
		})
	}
}

// A replaced (history) PKS no longer counts as use.
func TestPksTypeApplier_Disable_ReplacedPksDoesNotBlock_Integration(t *testing.T) {
	svc, tag, h := pksTypeHarness(t)
	code := pksTypeTestCode(tag) + "C"
	if _, err := approveTypeCreate(t, svc, h, code, "Riwayat"); err != nil {
		t.Fatalf("create: %v", err)
	}
	typ, _ := pksTypeRow(t, h, code)
	vendorID := h.seedVendor("PH-"+tag, "Vendor History "+tag)
	t.Cleanup(func() { deleteVendorPksOf(h.ctx, t, h.pool, []int64{vendorID}) })
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO vendor_pks (vendor_id, pks_type_id, wod_number, pks_name, effective_date, expiry_date, created_by, replaced_at)
		VALUES ($1, $2, $3, 'PKS LAMA', '2025-01-01', '2025-12-31', $4, now())`, vendorID, typ.ID, "WOD-H-"+tag, h.makerID); err != nil {
		t.Fatalf("seed history pks: %v", err)
	}
	id := h.insertPendingWithBefore("vendor_pks_type", "disable", &typ.ID, nil, pksTypeSnapshot(t, h, typ.ID))
	if err := approveChange(t, svc, id); err != nil {
		t.Fatalf("disable with only history PKS: %v", err)
	}
}

// A stale Before (type renamed after submit) is not applied.
func TestPksTypeApplier_Update_StaleBefore_NotApplied_Integration(t *testing.T) {
	svc, tag, h := pksTypeHarness(t)
	code := pksTypeTestCode(tag) + "D"
	if _, err := approveTypeCreate(t, svc, h, code, "Asli"); err != nil {
		t.Fatalf("create: %v", err)
	}
	typ, _ := pksTypeRow(t, h, code)
	id := h.insertPendingWithBefore("vendor_pks_type", "update", &typ.ID, PksTypeUpdatePayload{Name: "Usulan"}, pksTypeSnapshot(t, h, typ.ID))

	if _, err := h.pool.Exec(h.ctx, `UPDATE vendor_pks_types SET name = 'Disunting Lain' WHERE id = $1`, typ.ID); err != nil {
		t.Fatalf("concurrent edit: %v", err)
	}
	if err := approveChange(t, svc, id); !errors.Is(err, ErrMasterDataChangeStale) {
		t.Fatalf("approve: err = %v, want ErrMasterDataChangeStale", err)
	}
	if got, _ := pksTypeRow(t, h, code); got.Name != "Disunting Lain" {
		t.Errorf("name = %q, want the concurrent edit kept", got.Name)
	}
}

// Maker != checker (and any refusal of the approval chain) leaves the change pending and
// writes nothing. The rule itself lives in approval.Orchestrator; this proves the master-data
// apply hook honours it for this entity type.
func TestPksTypeApplier_OrchestratorRefusal_LeavesChangePending_Integration(t *testing.T) {
	svc, tag, h := pksTypeHarness(t)
	code := pksTypeTestCode(tag) + "E"
	changeID := h.insertPending("vendor_pks_type", "create", nil, PksTypeCreatePayload{Code: code, Name: "Ditolak"})
	svc.orchestrator = &canApproveOnce{err: approval.ErrNotAuthorized}

	if _, err := svc.Approve(h.ctx, 999, h.makerID, "127.0.0.1"); !errors.Is(err, approval.ErrNotAuthorized) {
		t.Fatalf("Approve: err = %v, want ErrNotAuthorized", err)
	}
	if s := changeStatus(t, h.pool, changeID); s != "pending" {
		t.Errorf("change status = %q, want pending", s)
	}
	if _, ok := pksTypeRow(t, h, code); ok {
		t.Error("type created although the chain refused")
	}
}

// Reject leaves the table untouched.
func TestPksTypeApplier_Reject_NoEntityChange_Integration(t *testing.T) {
	svc, tag, h := pksTypeHarness(t)
	code := pksTypeTestCode(tag) + "F"
	changeID := h.insertPending("vendor_pks_type", "create", nil, PksTypeCreatePayload{Code: code, Name: "Batal"})
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "rejected"}}
	if _, err := svc.Reject(h.ctx, 999, h.makerID, "127.0.0.1"); err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if _, ok := pksTypeRow(t, h, code); ok {
		t.Error("type created on reject")
	}
	if s := changeStatus(t, h.pool, changeID); s != "rejected" {
		t.Errorf("change status = %q, want rejected", s)
	}
}

// countingDBTX counts the statements that reach one connection, to prove which pool a read uses.
type countingDBTX struct {
	db.DBTX
	n int
}

func (c *countingDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	c.n++
	return c.DBTX.Query(ctx, sql, args...)
}

func (c *countingDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	c.n++
	return c.DBTX.QueryRow(ctx, sql, args...)
}

// The list screen reads from the replica; submit-time lookups stay on the primary.
func TestPksTypeRepository_ListOnReplica_Integration(t *testing.T) {
	_, tag, h := pksTypeHarness(t)
	code := pksTypeTestCode(tag) + "H"
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO vendor_pks_types (code, name) VALUES ($1, 'Replika')`, code); err != nil {
		t.Fatalf("seed type: %v", err)
	}
	primary, replica := &countingDBTX{DBTX: h.pool}, &countingDBTX{DBTX: h.pool}
	repo := repository.NewPksTypeRepository(primary, replica)

	rows, err := repo.List(h.ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	found := false
	for _, r := range rows {
		if r.Code == code && r.IsActive && !r.InUse {
			found = true
		}
	}
	if !found {
		t.Errorf("seeded type %s not in list (active, not in use)", code)
	}
	if replica.n != 1 || primary.n != 0 {
		t.Errorf("List hit replica=%d primary=%d, want 1/0", replica.n, primary.n)
	}

	if _, err := repo.FindByCode(h.ctx, code); err != nil {
		t.Fatalf("FindByCode: %v", err)
	}
	if _, err := repo.InUse(h.ctx, 1); err != nil {
		t.Fatalf("InUse: %v", err)
	}
	if primary.n != 2 || replica.n != 1 {
		t.Errorf("submit lookups hit primary=%d replica=%d, want 2/1", primary.n, replica.n)
	}
}
