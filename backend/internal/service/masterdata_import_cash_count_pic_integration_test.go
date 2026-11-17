//go:build integration

package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/repository"
)

// ScDD 5.1 task 9.2 — replace-all CSV import of mapping PIC against a real DB
// (AC-5.1-11, AC-5.1-13).

// scopedPicImportRepo is the real primary repo, except that ActivePics only
// returns mappings of this test's own users. Replace-all disables every active
// mapping missing from the file; other tests/sessions create mappings in the
// same DB at the same time, and this keeps the batch from ever disabling theirs.
type scopedPicImportRepo struct {
	*repository.CashCountPicAdminRepository
	users map[int64]bool
}

func (r scopedPicImportRepo) ActivePics(ctx context.Context) ([]db.ListCashCountPicsAdminRow, error) {
	rows, err := r.CashCountPicAdminRepository.ActivePics(ctx)
	if err != nil {
		return nil, err
	}
	out := rows[:0:0]
	for _, row := range rows {
		if r.users[row.UserID] {
			out = append(out, row)
		}
	}
	return out, nil
}

type picImportFixture struct {
	*importFixture
	region, area string
	areaID       int64
	users        map[string]int64 // short name -> user id
	checkerID    int64
}

func newPicImportFixture(t *testing.T) *picImportFixture {
	base := newImportFixture(t)
	f := &picImportFixture{importFixture: base, users: map[string]int64{}}
	h, tail := f.h, f.tag[len(f.tag)-8:]
	f.region, f.area = "Q"+tail, "IT_"+tail
	t.Cleanup(f.cleanupPics) // LIFO: runs before base/harness cleanup, and calls base cleanup itself first

	vendorID := h.seedVendor("ITP-"+f.tag, "Vendor PIC "+tail)
	branchID := h.seedVendorBranch(vendorID, "IT"+tail, "Cabang PIC "+tail)
	if _, err := h.pool.Exec(f.ctx, `UPDATE vendor_branches SET region_code = $2 WHERE id = $1`, branchID, f.region); err != nil {
		t.Fatalf("seed region: %v", err)
	}
	if err := h.pool.QueryRow(f.ctx, `INSERT INTO cash_count_areas (code, name) VALUES ($1, $2) RETURNING id`, f.area, "Area "+tail).Scan(&f.areaID); err != nil {
		t.Fatalf("seed area: %v", err)
	}
	for name, role := range map[string]string{"c1": "CC-COORDINATOR", "c2": "CC-COORDINATOR", "e1": "CC-EXECUTOR", "e2": "CC-EXECUTOR", "chk": "CC-CWM"} {
		var id int64
		err := h.pool.QueryRow(f.ctx, `INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source)
			VALUES ((SELECT id FROM roles WHERE role = $1), $2, $2, $2 || '@example.test', true, 'ldap') RETURNING id`,
			role, f.username(name)).Scan(&id)
		if err != nil {
			t.Fatalf("seed user %s: %v", name, err)
		}
		f.users[name] = id
	}
	f.checkerID = f.users["chk"]

	scope := map[int64]bool{}
	for _, id := range f.users {
		scope[id] = true
	}
	repo := scopedPicImportRepo{repository.NewCashCountPicAdminRepository(h.pool, h.pool), scope}
	f.importer = NewMasterDataImporter(repository.NewMasterDataExportRepository(h.pool)).
		WithCashCountPics(repo).
		WithConfirm(ImportConfirmDeps{
			Batches:      repository.NewMasterDataImportBatchRepository(h.pool, h.pool),
			Orchestrator: f.orch,
			Audit:        audit.NewWriter(h.pool),
			Services: func(sub ImportSubmitter) ImportServices {
				return ImportServices{CashCountPICs: NewCashCountPicAdminService(repo, sub)}
			},
		})
	f.svc.appliers["cash_count_pic"] = CashCountPicApplier{}
	f.ctx = ctxWithRole(h.makerID, "CC-CWM")
	return f
}

func (f *picImportFixture) username(name string) string { return "it92_" + f.tag + "_" + name }

func (f *picImportFixture) cleanupPics() {
	ctx, pool := context.Background(), f.h.pool
	ids := make([]int64, 0, len(f.users))
	for _, id := range f.users {
		ids = append(ids, id)
	}
	exec := func(sql string, args ...any) {
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			f.t.Logf("cleanup: %v", err)
		}
	}
	exec(`DELETE FROM audit_logs WHERE entity_type = 'cash_count_pic' AND entity_id IN (SELECT id FROM cash_count_pics WHERE user_id = ANY($1))`, ids)
	exec(`DELETE FROM cash_count_pics WHERE user_id = ANY($1)`, ids)
	f.importFixture.cleanup() // change requests, approvals, batches and their audit rows
	exec(`DELETE FROM audit_logs WHERE actor_id = ANY($1)`, ids)
	exec(`DELETE FROM users WHERE id = ANY($1)`, ids)
	exec(`DELETE FROM cash_count_areas WHERE code = $1`, f.area)
}

// seedPic inserts an active mapping directly (test setup, not the flow under test).
func (f *picImportFixture) seedPic(name, role string) int64 {
	f.t.Helper()
	var region, area any
	if role == picRoleCoordinator {
		region = f.region
	} else {
		area = f.areaID
	}
	var id int64
	if err := f.h.pool.QueryRow(f.ctx, `INSERT INTO cash_count_pics (pic_role, user_id, region_code, area_id) VALUES ($1, $2, $3, $4) RETURNING id`,
		role, f.users[name], region, area).Scan(&id); err != nil {
		f.t.Fatalf("seed pic: %v", err)
	}
	return id
}

// csv builds a mapping PIC file; each row is {pic_role, short user name}.
func (f *picImportFixture) csv(rows ...[2]string) string {
	var sb strings.Builder
	sb.WriteString("pic_role,username,region_code,area_code\n")
	for _, r := range rows {
		if r[0] == picRoleCoordinator {
			fmt.Fprintf(&sb, "%s,%s,%s,\n", r[0], f.username(r[1]), f.region)
		} else {
			fmt.Fprintf(&sb, "%s,%s,,%s\n", r[0], f.username(r[1]), f.area)
		}
	}
	return sb.String()
}

func (f *picImportFixture) confirmPics(csv string) (*ImportConfirmResult, error) {
	res, err := f.importer.Confirm(f.ctx, f.h.makerID, ExportCashCountPICs, "pics.csv", strings.NewReader(csv), "127.0.0.1")
	if err == nil {
		f.batches = append(f.batches, res.BatchID)
	}
	return res, err
}

func (f *picImportFixture) approvePics(res *ImportConfirmResult) error {
	f.svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: f.head(res.BatchID), Status: "approved"}}
	_, err := f.svc.Approve(f.ctx, *res.ApprovalRequestID, f.checkerID, "127.0.0.1")
	return err
}

func (f *picImportFixture) active(id int64) bool {
	f.t.Helper()
	var on bool
	if err := f.h.pool.QueryRow(f.ctx, `SELECT is_active AND deleted_at IS NULL FROM cash_count_pics WHERE id = $1`, id).Scan(&on); err != nil {
		f.t.Fatalf("load pic %d: %v", id, err)
	}
	return on
}

func (f *picImportFixture) activeFor(name string) int {
	return f.count(`SELECT count(*) FROM cash_count_pics WHERE user_id = $1 AND is_active`, f.users[name])
}

func TestCashCountPicImport_ReplaceAll_OneApproval_Idempotent_Audited_Integration(t *testing.T) {
	f := newPicImportFixture(t)
	keep := f.seedPic("e1", picRoleExecutor)
	dropExec := f.seedPic("e2", picRoleExecutor)
	dropCoord := f.seedPic("c1", picRoleCoordinator)
	csv := f.csv([2]string{picRoleExecutor, "e1"}, [2]string{picRoleCoordinator, "c2"})

	res, err := f.confirmPics(csv)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}

	// One batch, one approval: 1 create (c2) + 2 disables (e2, c1); e1 unchanged is not staged.
	if res.Existing || res.ApprovalRequestID == nil || f.orch.calls != 1 {
		t.Fatalf("confirm = %+v (approval calls %d), want a new batch with one approval", res, f.orch.calls)
	}
	rows, err := f.svc.repo.ListByBatch(f.ctx, res.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	byOp := map[string]int{}
	for _, r := range rows {
		if r.EntityType != cashCountPicEntity || r.Status != "pending" || r.ApprovalRequestID == nil || *r.ApprovalRequestID != *res.ApprovalRequestID || r.MakerID != f.h.makerID {
			t.Errorf("change %d = %s/%s approval %v maker %d, want pending cash_count_pic on the batch's approval by the maker",
				r.ID, r.EntityType, r.Status, r.ApprovalRequestID, r.MakerID)
		}
		byOp[r.Op]++
		if r.Op == "disable" && (r.EntityID == nil || (*r.EntityID != dropExec && *r.EntityID != dropCoord)) {
			t.Errorf("disable targets %v, want only the mappings missing from the file", r.EntityID)
		}
	}
	if byOp["create"] != 1 || byOp["disable"] != 2 || len(rows) != 3 {
		t.Fatalf("staged ops = %v, want create:1 disable:2", byOp)
	}
	if !f.active(dropExec) || !f.active(dropCoord) || f.activeFor("c2") != 0 {
		t.Fatalf("mappings changed before approval: confirm must only stage")
	}

	// Same file (SHA-256) again: existing batch, no second approval, no new rows.
	again, err := f.confirmPics(csv)
	if err != nil || !again.Existing || again.BatchID != res.BatchID || f.orch.calls != 1 {
		t.Fatalf("same file again = %+v, %v (approval calls %d), want the existing batch", again, err, f.orch.calls)
	}
	if n := f.count(`SELECT count(*) FROM master_data_change_requests WHERE batch_id = $1`, res.BatchID); n != 3 {
		t.Errorf("%d change requests after re-upload, want still 3", n)
	}

	if err := f.approvePics(res); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if !f.active(keep) || f.active(dropExec) || f.active(dropCoord) || f.activeFor("c2") != 1 {
		t.Errorf("after approve: e1=%v e2=%v c1=%v c2 active=%d; want only e1 and c2 active",
			f.active(keep), f.active(dropExec), f.active(dropCoord), f.activeFor("c2"))
	}
	if n := f.count(`SELECT count(*) FROM master_data_change_requests WHERE batch_id = $1 AND status = 'applied'`, res.BatchID); n != 3 {
		t.Errorf("%d rows applied, want 3", n)
	}

	// AC-5.1-13: one audit row per applied change, by the checker (maker is on the change row), with before/after.
	var created int64
	if err := f.h.pool.QueryRow(f.ctx, `SELECT id FROM cash_count_pics WHERE user_id = $1 AND is_active`, f.users["c2"]).Scan(&created); err != nil {
		t.Fatal(err)
	}
	if n := f.count(`SELECT count(*) FROM audit_logs WHERE entity_type = 'cash_count_pic' AND action = 'master_data_applied'
		AND actor_id = $1 AND after IS NOT NULL AND entity_id = ANY($2)`, f.checkerID, []int64{created, dropExec, dropCoord}); n != 3 {
		t.Errorf("%d apply audit rows by the checker, want 3", n)
	}
	if n := f.count(`SELECT count(*) FROM audit_logs WHERE entity_type = 'cash_count_pic' AND action = 'master_data_applied'
		AND before IS NOT NULL AND entity_id = ANY($1)`, []int64{dropExec, dropCoord}); n != 2 {
		t.Errorf("%d disable audit rows carry a before state, want 2", n)
	}

	// After apply the same file still answers "done", never a second batch.
	if done, err := f.confirmPics(csv); err != nil || !done.Existing || done.BatchID != res.BatchID {
		t.Errorf("re-upload after apply = %+v, %v; want the existing batch", done, err)
	}
}

func TestCashCountPicImport_InvalidRowStagesNothing_Integration(t *testing.T) {
	f := newPicImportFixture(t)
	existing := f.seedPic("e2", picRoleExecutor)
	// Row 2 is valid; row 3 names an unknown user: the whole file is refused.
	bad := f.csv([2]string{picRoleExecutor, "e1"}) + picRoleCoordinator + ",it92_" + f.tag + "_nobody," + f.region + ",\n"

	_, err := f.confirmPics(bad)

	var invalid *ImportInvalidError
	if !errors.As(err, &invalid) || len(invalid.Errors) == 0 {
		t.Fatalf("want ImportInvalidError, got %v", err)
	}
	if f.orch.calls != 0 {
		t.Errorf("an invalid file reached approval")
	}
	if open, err := f.importer.confirm.Batches.FindOpen(context.Background(), ExportCashCountPICs, fmt.Sprintf("%x", sha256.Sum256([]byte(bad)))); err != nil || open != nil {
		t.Errorf("FindOpen for an invalid file = %v, %v; want nil", open, err)
	}
	if !f.active(existing) || f.activeFor("e1") != 0 {
		t.Errorf("mappings changed by an invalid file")
	}
}

func TestCashCountPicImport_OneRowFailsAtApply_WholeBatchRolledBack_Integration(t *testing.T) {
	f := newPicImportFixture(t)
	dropExec := f.seedPic("e2", picRoleExecutor)
	csv := f.csv([2]string{picRoleExecutor, "e1"}, [2]string{picRoleCoordinator, "c2"})
	res, err := f.confirmPics(csv)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}

	// Between confirm and approval c2 is deactivated: the guarded create of c2 fails at apply...
	if _, err := f.h.pool.Exec(f.ctx, `UPDATE users SET is_active = false WHERE id = $1`, f.users["c2"]); err != nil {
		t.Fatal(err)
	}

	err = f.approvePics(res)

	if !errors.Is(err, errCashCountPicInvalid) {
		t.Fatalf("approve error = %v, want errCashCountPicInvalid", err)
	}
	// ...and none of the batch stays applied: e1 not created, e2 not disabled.
	if f.activeFor("e1") != 0 || !f.active(dropExec) {
		t.Errorf("e1 active=%d, e2 active=%v: a failed batch must apply all rows or none", f.activeFor("e1"), f.active(dropExec))
	}
	if n := f.count(`SELECT count(*) FROM master_data_change_requests WHERE batch_id = $1 AND status <> 'stale'`, res.BatchID); n != 0 {
		t.Errorf("%d rows of a rolled-back batch are not stale", n)
	}
	if n := f.count(`SELECT count(*) FROM audit_logs WHERE entity_type = 'cash_count_pic' AND action = 'master_data_applied' AND entity_id = $1`, dropExec); n != 0 {
		t.Errorf("an apply audit row survived the rollback")
	}
	if open, err := f.importer.confirm.Batches.FindOpen(context.Background(), ExportCashCountPICs, fmt.Sprintf("%x", sha256.Sum256([]byte(csv)))); err != nil || open != nil {
		t.Errorf("FindOpen after rollback = %v, %v; want nil (file may be uploaded again)", open, err)
	}
}
