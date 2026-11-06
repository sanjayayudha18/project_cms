//go:build integration

package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"

	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Spec vendor-pks task 5.5 (Property 9): the Direktur Utama invariant holds for
// any order of create/update/disable/enable/roles, and two approvals that both
// make a director for the same vendor leave exactly one applied. Skips without
// DATABASE_URL (harness).

func countUsers(t testing.TB, svc *MasterDataApprovalService) int {
	t.Helper()
	var n int
	if err := dbtx(svc).QueryRow(context.Background(), `SELECT count(*) FROM users`).Scan(&n); err != nil {
		t.Fatalf("count users: %v", err)
	}
	return n
}

func queryInt64s(t testing.TB, svc *MasterDataApprovalService, sql string, args ...any) []int64 {
	t.Helper()
	rows, err := dbtx(svc).Query(context.Background(), sql, args...)
	if err != nil {
		t.Fatalf("query %q: %v", sql, err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, v)
	}
	return out
}

// assertDirectorInvariants checks Property 9 for one vendor straight from the DB.
func assertDirectorInvariants(t testing.TB, svc *MasterDataApprovalService, vendorID int64) {
	t.Helper()
	ctx := context.Background()
	q := dbtx(svc)

	var active int
	if err := q.QueryRow(ctx, `SELECT count(*) FROM vendor_pics WHERE vendor_id = $1 AND is_director AND is_active AND deleted_at IS NULL`, vendorID).Scan(&active); err != nil {
		t.Fatalf("count active directors: %v", err)
	}
	if active > 1 {
		t.Errorf("vendor %d has %d active directors, want <= 1", vendorID, active)
	}

	var bad int
	if err := q.QueryRow(ctx, `SELECT count(*) FROM vendor_pics WHERE vendor_id = $1 AND is_director
		AND (vendor_branch_id IS NOT NULL OR cardinality(roles) > 0 OR is_notification_recipient)`, vendorID).Scan(&bad); err != nil {
		t.Fatalf("count malformed directors: %v", err)
	}
	if bad != 0 {
		t.Errorf("vendor %d has %d directors with a branch, roles or notification flag", vendorID, bad)
	}

	// Director emails never reach a notification list.
	erows, err := q.Query(ctx, `SELECT email FROM vendor_pics WHERE vendor_id = $1 AND is_director AND email IS NOT NULL`, vendorID)
	if err != nil {
		t.Fatalf("load director emails: %v", err)
	}
	var directorEmails []string
	for erows.Next() {
		var e string
		if err := erows.Scan(&e); err != nil {
			t.Fatalf("scan: %v", err)
		}
		directorEmails = append(directorEmails, e)
	}
	erows.Close()

	queries := db.New(q)
	vendorWide, err := queries.ListVendorNotificationPicEmails(ctx, []int64{vendorID})
	if err != nil {
		t.Fatalf("ListVendorNotificationPicEmails: %v", err)
	}
	lists := [][]string{vendorWide}
	for _, b := range queryInt64s(t, svc, `SELECT id FROM vendor_branches WHERE vendor_id = $1`, vendorID) {
		for _, role := range AllPicRoles() {
			l, err := queries.ListVendorBranchNotificationPicEmails(ctx, db.ListVendorBranchNotificationPicEmailsParams{BranchID: b, PicRole: role})
			if err != nil {
				t.Fatalf("ListVendorBranchNotificationPicEmails: %v", err)
			}
			lists = append(lists, l)
		}
	}
	for _, e := range directorEmails {
		for _, l := range lists {
			if slices.Contains(l, e) {
				t.Errorf("director email %q leaked into a notification list %v", e, l)
			}
		}
	}
}

type picOp struct {
	kind     int // 0 create, 1 update, 2 disable, 3 enable, 4 set roles
	target   int // index into the vendor's PICs
	director bool
	recip    bool
	hasMail  bool
	roles    []string
}

func TestVendorPicDirector_RandomOps_Invariants_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	roleSvc := NewVendorPicRoleService(h.pool)
	ctx := adminCtx(h.makerID)
	usersBefore := countUsers(t, svc)
	iter := 0

	opGen := rapid.Custom(func(rt *rapid.T) picOp {
		return picOp{
			kind:     rapid.IntRange(0, 4).Draw(rt, "kind"),
			target:   rapid.IntRange(0, 5).Draw(rt, "target"),
			director: rapid.Bool().Draw(rt, "director"),
			recip:    rapid.Bool().Draw(rt, "recipient"),
			hasMail:  rapid.Bool().Draw(rt, "has_mail"),
			roles:    rapid.SliceOfNDistinct(rapid.SampledFrom(AllPicRoles()), 0, 3, func(s string) string { return s }).Draw(rt, "roles"),
		}
	})

	rapid.Check(t, func(rt *rapid.T) {
		iter++
		vendorID := h.seedVendor(fmt.Sprintf("ITDIR-%s-%d", tag, iter), "Vendor Director "+tag)
		h.seedVendorBranch(vendorID, fmt.Sprintf("ITDIRB-%s-%d", tag, iter), "Cabang "+tag)
		var pics []int64

		for step, op := range rapid.SliceOfN(opGen, 1, 14).Draw(rt, "ops") {
			var email *string
			if op.hasMail {
				s := fmt.Sprintf("dir-%s-%d-%d@example.com", tag, iter, step)
				email = &s
			}
			target := func() (int64, bool) {
				if len(pics) == 0 {
					return 0, false
				}
				return pics[op.target%len(pics)], true
			}
			// Payloads follow the service rules; the DB is the last line of defence.
			dir := op.director
			base := VendorPicUpdatePayload{Name: fmt.Sprintf("PIC %d", step), Email: email, IsDirector: &dir}
			if !op.director {
				base.IsNotificationRecipient = op.recip
			}

			var err error
			switch op.kind {
			case 0:
				err = approveChange(t, svc, h.insertPending("vendor_pic", "create", nil, VendorPicPayload{VendorID: vendorID, VendorPicUpdatePayload: base}))
				if err == nil {
					ids := queryInt64s(t, svc, `SELECT max(id) FROM vendor_pics WHERE vendor_id = $1`, vendorID)
					pics = append(pics, ids...)
					h.vendorPicIDs = append(h.vendorPicIDs, ids...)
				}
			case 1:
				if id, ok := target(); ok {
					err = approveChange(t, svc, h.insertPending("vendor_pic", "update", &id, base))
				}
			case 2, 3:
				if id, ok := target(); ok {
					err = approveChange(t, svc, h.insertPending("vendor_pic", map[int]string{2: "disable", 3: "enable"}[op.kind], &id, nil))
				}
			case 4:
				if id, ok := target(); ok {
					_, err = roleSvc.SetRoles(ctx, h.makerID, vendorID, id, op.roles, "127.0.0.1")
				}
			}
			// A refused step (stale, director with roles, not found, ...) is fine;
			// an invariant break after any step is not.
			if err != nil {
				rt.Logf("step %d kind %d refused: %v", step, op.kind, err)
			}
			assertDirectorInvariants(t, svc, vendorID)
		}
	})

	if got := countUsers(t, svc); got != usersBefore {
		t.Errorf("users rows changed %d -> %d; a director PIC must not create a login", usersBefore, got)
	}
}

// Two approvals that both create a director for the same vendor, run in
// parallel: one applied, one stale (partial unique vendor_pics_director_uq).
func TestVendorPicDirector_ParallelApprove_OneAppliedOneStale_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := adminCtx(h.makerID)

	for round := 0; round < 5; round++ {
		vendorID := h.seedVendor(fmt.Sprintf("ITDIRR-%s-%d", tag, round), "Vendor Director race "+tag)
		yes := true
		ids := make([]int64, 2)
		for i := range ids {
			ids[i] = h.insertPending("vendor_pic", "create", nil, VendorPicPayload{
				VendorID: vendorID, VendorPicUpdatePayload: VendorPicUpdatePayload{Name: fmt.Sprintf("Dirut %d", i), IsDirector: &yes, Roles: &[]string{}},
			})
			if _, err := h.pool.Exec(ctx, `UPDATE master_data_change_requests SET status = 'approved' WHERE id = $1`, ids[i]); err != nil {
				t.Fatalf("mark approved: %v", err)
			}
		}

		errs := make([]error, 2)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i, id := range ids {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				errs[i] = svc.RetryApply(ctx, id, h.makerID, "127.0.0.1")
			}()
		}
		close(start)
		wg.Wait()

		applied, stale := 0, 0
		for i, id := range ids {
			switch st := changeStatus(t, h.pool, id); {
			case st == "applied" && errs[i] == nil:
				applied++
			case st == "stale" && errors.Is(errs[i], ErrMasterDataChangeStale):
				stale++
			default:
				t.Errorf("round %d change %d: status %q err %v", round, id, st, errs[i])
			}
		}
		if applied != 1 || stale != 1 {
			t.Errorf("round %d: applied=%d stale=%d, want 1/1", round, applied, stale)
		}
		h.vendorPicIDs = append(h.vendorPicIDs, queryInt64s(t, svc, `SELECT id FROM vendor_pics WHERE vendor_id = $1`, vendorID)...)
		assertDirectorInvariants(t, svc, vendorID)
	}
}

// Req 1.6: roles on a director are refused with no write and no audit; releasing the
// flag through an update keeps the roles empty (Req 1.7).
func TestVendorPicDirector_RolesRefusedAndRelease_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	roleSvc := NewVendorPicRoleService(h.pool)
	ctx := adminCtx(h.makerID)
	vendorID := h.seedVendor("ITDIRX-"+tag, "Vendor Director roles "+tag)

	yes, no := true, false
	name := "Dirut " + tag
	if err := approveChange(t, svc, h.insertPending("vendor_pic", "create", nil, VendorPicPayload{
		VendorID: vendorID, VendorPicUpdatePayload: VendorPicUpdatePayload{Name: name, IsDirector: &yes, Roles: &[]string{}},
	})); err != nil {
		t.Fatalf("create director: %v", err)
	}
	picIDs := queryInt64s(t, svc, `SELECT id FROM vendor_pics WHERE vendor_id = $1`, vendorID)
	if len(picIDs) != 1 {
		t.Fatalf("want one pic, got %v", picIDs)
	}
	picID := picIDs[0]
	h.vendorPicIDs = append(h.vendorPicIDs, picID)

	if _, err := roleSvc.SetRoles(ctx, h.makerID, vendorID, picID, []string{"vault"}, "127.0.0.1"); !errors.Is(err, ErrVendorDirectorNoRoles) {
		t.Fatalf("SetRoles on director: want ErrVendorDirectorNoRoles, got %v", err)
	}
	if got := picRoles(t, svc, picID); len(got) != 0 {
		t.Errorf("roles after refused SetRoles = %q, want empty", got)
	}
	if n := picRolesAuditCount(t, svc, picID); n != 0 {
		t.Errorf("audit rows = %d, want 0 for a refused change", n)
	}
	if _, err := roleSvc.SetRoles(ctx, h.makerID, vendorID, picID, []string{}, ""); err != nil {
		t.Errorf("SetRoles(empty) on director: %v", err)
	}

	if err := approveChange(t, svc, h.insertPending("vendor_pic", "update", &picID, VendorPicUpdatePayload{Name: name, IsDirector: &no})); err != nil {
		t.Fatalf("release director flag: %v", err)
	}
	var isDirector bool
	if err := dbtx(svc).QueryRow(context.Background(), `SELECT is_director FROM vendor_pics WHERE id = $1`, picID).Scan(&isDirector); err != nil || isDirector {
		t.Errorf("is_director = %v, %v; want false", isDirector, err)
	}
	if got := picRoles(t, svc, picID); len(got) != 0 {
		t.Errorf("roles after release = %q, want still empty", got)
	}
	assertDirectorInvariants(t, svc, vendorID)
}
