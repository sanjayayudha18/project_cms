//go:build integration

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/repository"
)

// ccPicFixture seeds unique users/area/branch (tag-suffixed) and removes them again.
// The approval service is rebuilt here with only the cash_count_pic applier, so this file does
// not depend on the shared harness registry.
type ccPicFixture struct {
	t      *testing.T
	h      *testHelpers
	svc    *MasterDataApprovalService
	pool   *pgxpool.Pool
	region string
	coord1 int64
	coord2 int64
	exec1  int64
	areaID int64
	picIDs []int64
}

func newCcPicFixture(t *testing.T) *ccPicFixture {
	t.Helper()
	_, tag, h := harness(t) // skips without DATABASE_URL
	svc := NewMasterDataApprovalService(nil, repository.NewMasterDataChangeRepository(h.pool), h.pool,
		ApplierRegistry{"cash_count_pic": CashCountPicApplier{}}, audit.NewWriter(h.pool))
	f := &ccPicFixture{t: t, h: h, svc: svc, pool: h.pool}

	short := tag
	if len(short) > 6 {
		short = short[len(short)-6:]
	}
	f.region = "T83" + short // ^[A-Z0-9]{2,10}$, unique per run
	vendorID := h.seedVendor("IT83-"+tag, "Vendor PIC "+tag)
	branchID := h.seedVendorBranch(vendorID, "IT83B-"+tag, "Branch PIC "+tag)
	seedTestRegion(t, h.pool, f.region, "IT cash count "+f.region)
	f.exec(`UPDATE vendor_branches SET region_code = $2 WHERE id = $1`, branchID, f.region)

	f.coord1 = f.seedUser("CC-COORDINATOR", "it83c1_"+tag)
	f.coord2 = f.seedUser("CC-COORDINATOR", "it83c2_"+tag)
	f.exec1 = f.seedUser("CC-EXECUTOR", "it83e1_"+tag)
	if err := f.pool.QueryRow(h.ctx, `INSERT INTO cash_count_areas (code, name) VALUES ($1, $2) RETURNING id`,
		"IT83"+short, "Area PIC "+tag).Scan(&f.areaID); err != nil {
		t.Fatalf("seed area: %v", err)
	}

	// registered after harness's cleanup, so it runs first (pics/areas/users before vendor rows)
	t.Cleanup(func() {
		ctx := context.Background()
		users := []int64{f.coord1, f.coord2, f.exec1}
		for _, q := range []struct {
			sql string
			arg any
		}{
			{`DELETE FROM audit_logs WHERE entity_type = 'cash_count_pic' AND entity_id = ANY($1)`, f.picIDs},
			{`DELETE FROM cash_count_pics WHERE user_id = ANY($1)`, users},
			{`DELETE FROM cash_count_areas WHERE id = $1`, f.areaID},
			{`DELETE FROM users WHERE id = ANY($1)`, users},
		} {
			if _, err := f.pool.Exec(ctx, q.sql, q.arg); err != nil {
				t.Logf("cleanup %q: %v", q.sql, err)
			}
		}
	})
	return f
}

func (f *ccPicFixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.pool.Exec(f.h.ctx, sql, args...); err != nil {
		f.t.Fatalf("exec %q: %v", sql, err)
	}
}

func (f *ccPicFixture) seedUser(role, username string) int64 {
	f.t.Helper()
	var id int64
	if err := f.pool.QueryRow(f.h.ctx, `INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source)
		SELECT id, $2, $2, $2 || '@test.local', true, 'ldap' FROM roles WHERE role = $1 RETURNING id`, role, username).Scan(&id); err != nil {
		f.t.Fatalf("seed user %s: %v", username, err)
	}
	return id
}

func (f *ccPicFixture) deactivateUser(id int64) {
	f.t.Helper()
	f.exec(`UPDATE users SET is_active = false WHERE id = $1`, id)
}

func (f *ccPicFixture) picState(id int64) (userID int64, active bool) {
	f.t.Helper()
	if err := f.pool.QueryRow(f.h.ctx, `SELECT user_id, is_active FROM cash_count_pics WHERE id = $1`, id).Scan(&userID, &active); err != nil {
		f.t.Fatalf("load pic %d: %v", id, err)
	}
	return userID, active
}

func (f *ccPicFixture) picCount(userID int64) int {
	f.t.Helper()
	var n int
	if err := f.pool.QueryRow(f.h.ctx, `SELECT count(*) FROM cash_count_pics WHERE user_id = $1`, userID).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

// createPic stages + approves a create and returns the new id (error = apply failure).
func (f *ccPicFixture) createPic(p CashCountPicPayload) (int64, error) {
	f.t.Helper()
	if err := approveChange(f.t, f.svc, f.h.insertPending("cash_count_pic", "create", nil, p)); err != nil {
		return 0, err
	}
	var id int64
	if err := f.pool.QueryRow(f.h.ctx, `SELECT id FROM cash_count_pics WHERE user_id = $1 AND is_active`, p.UserID).Scan(&id); err != nil {
		f.t.Fatalf("load created pic: %v", err)
	}
	f.picIDs = append(f.picIDs, id)
	return id, nil
}

// stageWithBefore stages op on an existing pic with the same Before snapshot the service stores.
func (f *ccPicFixture) stageWithBefore(op string, id int64, payload any) int64 {
	f.t.Helper()
	before, err := db.New(f.pool).GetCashCountPicAdminByID(f.h.ctx, id)
	if err != nil {
		f.t.Fatalf("load before: %v", err)
	}
	return f.h.insertPendingWithBefore("cash_count_pic", op, &id, payload, before)
}

func (f *ccPicFixture) auditCount(id int64, withBeforeAfter bool) int {
	f.t.Helper()
	q := `SELECT count(*) FROM audit_logs WHERE entity_type = 'cash_count_pic' AND entity_id = $1 AND action = 'master_data_applied'`
	if withBeforeAfter {
		q += ` AND before IS NOT NULL AND after IS NOT NULL`
	}
	var n int
	if err := f.pool.QueryRow(f.h.ctx, q, id).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

// AC-5.1-09, AC-5.1-10, AC-5.1-13: create/update/disable/enable through approve; audit per apply.
func TestCashCountPicApplier_Lifecycle_Integration(t *testing.T) {
	f := newCcPicFixture(t)
	region := f.region

	// nothing changes while a change is only pending
	pendingID := f.h.insertPending("cash_count_pic", "create", nil,
		CashCountPicPayload{PicRole: "COORDINATOR", UserID: f.coord1, RegionCode: &region})
	if n := f.picCount(f.coord1); n != 0 {
		t.Fatalf("pic rows before approve = %d, want 0", n)
	}
	if err := approveChange(t, f.svc, pendingID); err != nil {
		t.Fatalf("create coordinator: %v", err)
	}
	var coordPic int64
	if err := f.pool.QueryRow(f.h.ctx, `SELECT id FROM cash_count_pics WHERE user_id = $1 AND is_active`, f.coord1).Scan(&coordPic); err != nil {
		t.Fatalf("created coordinator pic: %v", err)
	}
	f.picIDs = append(f.picIDs, coordPic)
	if n := f.auditCount(coordPic, false); n != 1 {
		t.Errorf("audit rows after create = %d, want 1", n)
	}

	// EXECUTOR on an area
	execPic, err := f.createPic(CashCountPicPayload{PicRole: "EXECUTOR", UserID: f.exec1, AreaID: &f.areaID})
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	if u, a := f.picState(execPic); u != f.exec1 || !a {
		t.Errorf("executor pic = user %d active %v", u, a)
	}

	// same (role, user, region) again -> active-unique index => conflict, still one row
	if _, err := f.createPic(CashCountPicPayload{PicRole: "COORDINATOR", UserID: f.coord1, RegionCode: &region}); !errors.Is(err, ErrCashCountPicConflict) {
		t.Errorf("duplicate create err = %v, want ErrCashCountPicConflict", err)
	}
	if n := f.picCount(f.coord1); n != 1 {
		t.Errorf("rows for coord1 after duplicate = %d, want 1", n)
	}

	// a user without the CC-COORDINATOR role is refused by the guarded insert
	if _, err := f.createPic(CashCountPicPayload{PicRole: "COORDINATOR", UserID: f.exec1, RegionCode: &region}); !errors.Is(err, errCashCountPicInvalid) {
		t.Errorf("wrong-role create err = %v, want errCashCountPicInvalid", err)
	}

	// update -> coord2; audit carries before and after
	if err := approveChange(t, f.svc, f.stageWithBefore("update", coordPic, CashCountPicUpdatePayload{UserID: f.coord2, RegionCode: &region})); err != nil {
		t.Fatalf("update: %v", err)
	}
	if u, a := f.picState(coordPic); u != f.coord2 || !a {
		t.Errorf("after update: user %d active %v, want %d/true", u, a, f.coord2)
	}
	if n := f.auditCount(coordPic, true); n != 1 {
		t.Errorf("audit rows with before+after = %d, want 1 (the update)", n)
	}

	// disable ("hapus" = nonaktif, row stays), then enable again
	if err := approveChange(t, f.svc, f.stageWithBefore("disable", coordPic, nil)); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, a := f.picState(coordPic); a {
		t.Error("pic still active after disable")
	}
	if err := approveChange(t, f.svc, f.stageWithBefore("enable", coordPic, nil)); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if _, a := f.picState(coordPic); !a {
		t.Error("pic not active after enable")
	}
	if n := f.auditCount(coordPic, false); n != 4 {
		t.Errorf("audit rows = %d, want 4 (create, update, disable, enable)", n)
	}
}

// AC-5.1-09: the user is re-guarded at apply time; a user disabled between submit and approve
// makes the change fail (409 via errCashCountPicInvalid) and nothing is written.
func TestCashCountPicApplier_UserDisabledBeforeApprove_Integration(t *testing.T) {
	f := newCcPicFixture(t)
	region := f.region

	// create: user deactivated after the change was staged
	changeID := f.h.insertPending("cash_count_pic", "create", nil,
		CashCountPicPayload{PicRole: "COORDINATOR", UserID: f.coord1, RegionCode: &region})
	f.deactivateUser(f.coord1)
	if err := approveChange(t, f.svc, changeID); !errors.Is(err, errCashCountPicInvalid) {
		t.Fatalf("create with disabled user err = %v, want errCashCountPicInvalid", err)
	}
	if n := f.picCount(f.coord1); n != 0 {
		t.Errorf("rows after failed create = %d, want 0", n)
	}
	if c, err := f.svc.repo.GetByID(f.h.ctx, changeID); err != nil || c.Status == "applied" {
		t.Errorf("change status = %v (err %v), want not applied", c.Status, err)
	}

	// update to the (now inactive) coord1; the pic keeps its user
	pic, err := f.createPic(CashCountPicPayload{PicRole: "COORDINATOR", UserID: f.coord2, RegionCode: &region})
	if err != nil {
		t.Fatalf("create coord2: %v", err)
	}
	if err := approveChange(t, f.svc, f.stageWithBefore("update", pic, CashCountPicUpdatePayload{UserID: f.coord1, RegionCode: &region})); !errors.Is(err, errCashCountPicInvalid) {
		t.Fatalf("update to disabled user err = %v, want errCashCountPicInvalid", err)
	}
	if u, _ := f.picState(pic); u != f.coord2 {
		t.Errorf("pic user after failed update = %d, want %d", u, f.coord2)
	}

	// enable: owner deactivated while the mapping was disabled
	if err := approveChange(t, f.svc, f.stageWithBefore("disable", pic, nil)); err != nil {
		t.Fatalf("disable: %v", err)
	}
	enableID := f.stageWithBefore("enable", pic, nil)
	f.deactivateUser(f.coord2)
	if err := approveChange(t, f.svc, enableID); !errors.Is(err, errCashCountPicInvalid) {
		t.Fatalf("enable with disabled user err = %v, want errCashCountPicInvalid", err)
	}
	if _, a := f.picState(pic); a {
		t.Error("pic became active despite disabled user")
	}
}

// AC-5.1-10: a rejected change leaves the data untouched and the maker may submit again.
func TestCashCountPicApplier_Reject_NoChange_Integration(t *testing.T) {
	f := newCcPicFixture(t)
	region := f.region
	payload := CashCountPicPayload{PicRole: "COORDINATOR", UserID: f.coord1, RegionCode: &region}

	changeID := f.h.insertPending("cash_count_pic", "create", nil, payload)
	f.svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "rejected"}}
	if _, err := f.svc.Reject(f.h.ctx, 999, 1, "127.0.0.1"); err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if c, err := f.svc.repo.GetByID(f.h.ctx, changeID); err != nil || c.Status != "rejected" {
		t.Fatalf("change status = %v (err %v), want rejected", c.Status, err)
	}
	if n := f.picCount(f.coord1); n != 0 {
		t.Errorf("rows after reject = %d, want 0", n)
	}

	// maker resubmits the same mapping; this time it is approved
	if _, err := f.createPic(payload); err != nil {
		t.Fatalf("resubmit after reject: %v", err)
	}
	if n := f.picCount(f.coord1); n != 1 {
		t.Errorf("rows after resubmit = %d, want 1", n)
	}
}
