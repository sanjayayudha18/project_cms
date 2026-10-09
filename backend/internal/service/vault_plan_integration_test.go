//go:build integration

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/cimb-niaga/cms/backend/internal/notification"
)

// cit-acm-plan S6 end to end on real Postgres: two ACM areas for one request,
// tiered candidates with DSR saldo/capacity, save/submit/approve/reject at the
// ACM layer, auto vault_review, ATM-SPV reject/approve, RBAC and four-eyes.

type vaultPlanFixture struct {
	*vaultFlowFixture
	plans     *VaultPlanService
	terminal2 string
	area1     int64
	area2     int64
	spvID     int64
	cash1     int64 // tier 1: same vendor, same region, DSR saldo 1.5M (100K)
	cash2     int64 // tier 2: other vendor, same region, no vault -> unknown
	cash3     int64 // tier 3: same vendor, other region
}

func newVaultPlanFixture(t *testing.T) *vaultPlanFixture {
	t.Helper()
	f := &vaultPlanFixture{vaultFlowFixture: newVaultFlowFixture(t)}
	ctx := context.Background()
	pool := f.pool
	tag := uuid.NewString()[:8]
	row := func(sql string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := pool.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
			t.Fatalf("seed %q: %v", sql, err)
		}
		return id
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed %q: %v", sql, err)
		}
	}
	var vendorName string
	if err := pool.QueryRow(ctx, `SELECT name FROM vendors WHERE id = $1`, f.vendorID).Scan(&vendorName); err != nil {
		t.Fatal(err)
	}

	// Second replenish branch + ATM (same vendor + region, so one request).
	branch2 := row(`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, region_code) VALUES ($1, $2, 'B2', 'ITEST') RETURNING id`, f.vendorID, "ITB2-"+tag)
	pkg2 := row(`INSERT INTO vendor_packages_branch (vendor_branch_id, package_code, machine_group, price_class, tier_min, base_price, currency, effective_start_date)
		VALUES ($1, 'PAKET T2', 'ATM', 'REGULAR', 1, 0, 'IDR', CURRENT_DATE - 365) RETURNING id`, branch2)
	f.terminal2 = "ITT2-" + tag
	atm2 := row(`INSERT INTO atms (terminal_id, location_id, machine_type, brand, model, operation_hours, deployment_type, is_active)
		SELECT $1, location_id, 'ATM', 'TestBrand', 'TestModel', '24_HOURS', 'OFFSITE', true FROM atms WHERE terminal_id = $2 RETURNING id`, f.terminal2, f.terminal)
	exec(`INSERT INTO atm_vendor_packages (atm_id, vendor_package_id, effective_start_date, is_active) VALUES ($1, $2, now() - interval '30 days', true)`, atm2, pkg2)

	// Cash branches.
	f.cash1 = row(`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, region_code, category) VALUES ($1, $2, 'Cash 1', 'ITEST', 'CASH') RETURNING id`, f.vendorID, "ITC1-"+tag)
	f.cash3 = row(`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, region_code, category) VALUES ($1, $2, 'Cash 3', 'ITOTHER', 'CASH') RETURNING id`, f.vendorID, "ITC3-"+tag)
	vendor2 := row(`INSERT INTO vendors (code, name) VALUES ($1, $1) RETURNING id`, "ITV2-"+tag)
	f.cash2 = row(`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, region_code, category) VALUES ($1, $2, 'Cash 2', 'ITEST', 'ATM_CASH') RETURNING id`, vendor2, "ITC2-"+tag)
	vault1 := row(`INSERT INTO vendor_vaults (vendor_branch_id, vault_code, type, category, currency_code) VALUES ($1, $2, 'CASH', 'CASH', 'IDR') RETURNING id`, f.cash1, "ITVLT-"+tag)
	upload := row(`INSERT INTO dsr_uploads (filename, vendor, report_date, daily_status) VALUES ($1, $2, $3, 'completed') RETURNING id`,
		"it-"+tag+".xlsx", vendorName, jakartaCalendarDate(0))
	exec(`INSERT INTO dsr_daily_rows (upload_id, row_no, location, section, flow, line_label,
		denom_100k_idr, denom_50k_idr, denom_20k_idr, denom_10k_idr, denom_5k_idr, denom_2k_idr, denom_1k_idr, line_total_idr)
		VALUES ($1, 1, 'IT VAULT', 'd0', 'saldo_akhir', 'SALDO AKHIR', 1500000, 0, 0, 0, 0, 0, 0, 1500000)`, upload)
	exec(`INSERT INTO dsr_location_vault_maps (vendor_id, dsr_location, vendor_vault_id) VALUES ($1, 'it vault', $2)`, f.vendorID, vault1)

	// Areas: area1 = branch of ATM 1, area2 = branch2. ACM-USER + ACM-SPV in both.
	f.area1 = f.area(t, true)
	f.area2 = f.area(t, false)
	f.spvID = row(`INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source)
		SELECT id, $1, $1, $1 || '@test.local', true, 'ldap' FROM roles WHERE role = 'ACM-SPV' RETURNING id`, "itspv-"+tag)
	exec(`INSERT INTO acm_area_branches (acm_area_id, vendor_branch_id, created_by) VALUES ($1, $2, $3)`, f.area2, branch2, f.creator.UserID)
	exec(`INSERT INTO acm_area_members (acm_area_id, user_id, created_by) VALUES ($1, $2, $3)`, f.area1, f.spvID, f.creator.UserID)
	exec(`INSERT INTO acm_area_members (acm_area_id, user_id, created_by) VALUES ($1, $2, $3)`, f.area2, f.spvID, f.creator.UserID)

	f.plans = NewVaultPlanService(pool, pool).WithNotifier(notification.NewService(false))

	// Runs before the S5 fixture's cleanup (LIFO): children first.
	t.Cleanup(func() {
		c := context.Background()
		ex := func(sql string, args ...any) { _, _ = pool.Exec(c, sql, args...) }
		ex(`DELETE FROM vendor_request_vault_assignments WHERE vendor_request_id IN (SELECT id FROM vendor_requests WHERE vendor_id = $1)`, f.vendorID)
		ex(`DELETE FROM vendor_request_vault_plans WHERE vendor_request_id IN (SELECT id FROM vendor_requests WHERE vendor_id = $1)`, f.vendorID)
		ex(`DELETE FROM acm_area_members WHERE user_id = $1`, f.spvID)
		ex(`DELETE FROM acm_area_branches WHERE vendor_branch_id = $1`, branch2)
		ex(`DELETE FROM notification_emails WHERE notification_id IN (SELECT id FROM notifications WHERE recipient_user_id = $1)`, f.spvID)
		ex(`DELETE FROM notifications WHERE recipient_user_id = $1`, f.spvID)
		// The request creator is an existing dev user: drop only this test's vault notifications.
		ex(`DELETE FROM notification_emails WHERE notification_id IN (SELECT id FROM notifications WHERE recipient_user_id = $1 AND type LIKE 'vault_plan.%')`, f.creator.UserID)
		ex(`DELETE FROM notifications WHERE recipient_user_id = $1 AND type LIKE 'vault_plan.%'`, f.creator.UserID)
		ex(`UPDATE users SET is_active = false, deleted_at = now() WHERE id = $1`, f.spvID) // see vault_flow fixture
		ex(`DELETE FROM users WHERE id = $1`, f.spvID)
		ex(`DELETE FROM dsr_location_vault_maps WHERE vendor_id = $1`, f.vendorID)
		ex(`DELETE FROM dsr_daily_rows WHERE upload_id = $1`, upload)
		ex(`DELETE FROM dsr_uploads WHERE id = $1`, upload)
		ex(`DELETE FROM vendor_vaults WHERE id = $1`, vault1)
		ex(`DELETE FROM vendor_request_tickets WHERE terminal_id = $1`, f.terminal2)
		ex(`DELETE FROM vendor_request_items WHERE terminal_id = $1`, f.terminal2)
		ex(`DELETE FROM atm_vendor_packages WHERE atm_id = $1`, atm2)
		ex(`DELETE FROM atms WHERE id = $1`, atm2)
		ex(`DELETE FROM vendor_packages_branch WHERE id = $1`, pkg2)
		ex(`DELETE FROM vendor_branches WHERE id = ANY($1)`, []int64{branch2, f.cash1, f.cash2, f.cash3})
		ex(`DELETE FROM vendors WHERE id = $1`, vendor2)
	})
	return f
}

// approvedTwoAtms creates a request with 1,000,000 (100K) for each ATM and approves it.
func (f *vaultPlanFixture) approvedTwoAtms(t *testing.T) (requestID, plan1, plan2 int64) {
	t.Helper()
	ctx := context.Background()
	in := manualCreateInput(f.vendorID, f.terminal, 100000)
	in.Items = append(in.Items, ItemInput{TerminalID: f.terminal2, Denom: 100000, AmountReplenish: 1000000})
	created, err := f.svc.Create(ctx, f.creator, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := f.svc.Submit(ctx, f.creator, created.ID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := f.svc.Approve(ctx, f.checker, created.ID); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT id FROM vendor_request_vault_plans WHERE vendor_request_id = $1 AND acm_area_id = $2`, created.ID, f.area1).Scan(&plan1); err != nil {
		t.Fatalf("plan area1: %v", err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT id FROM vendor_request_vault_plans WHERE vendor_request_id = $1 AND acm_area_id = $2`, created.ID, f.area2).Scan(&plan2); err != nil {
		t.Fatalf("plan area2: %v", err)
	}
	return created.ID, plan1, plan2
}

func (f *vaultPlanFixture) requestStatus(t *testing.T, id int64) string {
	t.Helper()
	var s string
	if err := f.pool.QueryRow(context.Background(), `SELECT status FROM vendor_requests WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestIntegration_VaultPlan_FullFlow(t *testing.T) {
	f := newVaultPlanFixture(t)
	ctx := context.Background()
	acmUser := Actor{UserID: f.acmUserID, Role: "ACM-USER"}
	acmSpv := Actor{UserID: f.spvID, Role: "ACM-SPV"}
	reqID, plan1, plan2 := f.approvedTwoAtms(t)

	// RBAC: a user outside the area sees nothing; ADMIN reads but cannot write.
	if _, err := f.plans.Get(ctx, Actor{UserID: f.creator.UserID, Role: "ACM-USER"}, plan1); !errors.Is(err, ErrVaultPlanNotFound) {
		t.Fatalf("non-member Get: want ErrVaultPlanNotFound, got %v", err)
	}
	admin := Actor{UserID: f.creator.UserID, Role: "ADMIN"}
	if _, err := f.plans.Get(ctx, admin, plan1); err != nil {
		t.Fatalf("ADMIN Get: %v", err)
	}
	if _, err := f.plans.SaveAssignments(ctx, admin, plan1, nil); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("ADMIN save: want ErrNotAuthorized, got %v", err)
	}

	// List is scoped to the actor's areas; ADMIN sees every area.
	mine, err := f.plans.List(ctx, acmUser, VaultPlanListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	listed := map[int64]bool{}
	for _, p := range mine {
		listed[p.ID] = true
	}
	if !listed[plan1] || !listed[plan2] {
		t.Fatalf("member list misses plans %d/%d: %v", plan1, plan2, listed)
	}
	if others, err := f.plans.List(ctx, Actor{UserID: f.creator.UserID, Role: "ACM-USER"}, VaultPlanListFilter{AreaID: &f.area1}); err != nil || len(others) != 0 {
		t.Fatalf("non-member list = %d plans (%v), want 0", len(others), err)
	}
	if all, err := f.plans.List(ctx, admin, VaultPlanListFilter{Status: planDraft, AreaID: &f.area1}); err != nil || len(all) != 1 {
		t.Fatalf("ADMIN list area1 drafts = %d (%v), want 1", len(all), err)
	}

	// Candidates: tier 1 with known saldo first, tier 2 unknown after; tier 3 only when urgent.
	cands, err := f.plans.Candidates(ctx, acmUser, plan1, f.terminal, false)
	if err != nil {
		t.Fatal(err)
	}
	pos := map[int64]VaultCandidate{}
	for _, c := range cands {
		pos[c.VendorBranchID] = c
	}
	c1, c2 := pos[f.cash1], pos[f.cash2]
	if c1.Tier != 1 || !c1.SaldoKnown || c1.Capacity["100000"] != "1500000" || c1.Warning {
		t.Fatalf("cash1 candidate = %+v, want tier 1, saldo known, capacity 1500000, no warning", c1)
	}
	if c2.Tier != 2 || c2.SaldoKnown || !c2.Warning {
		t.Fatalf("cash2 candidate = %+v, want tier 2, unknown saldo, warning", c2)
	}
	if _, ok := pos[f.cash3]; ok {
		t.Fatal("tier 3 offered without urgent")
	}
	urgentCands, _ := f.plans.Candidates(ctx, acmUser, plan1, f.terminal, true)
	found := false
	for _, c := range urgentCands {
		found = found || (c.VendorBranchID == f.cash3 && c.Tier == 3)
	}
	if !found {
		t.Fatal("urgent candidates must include tier 3 cash3")
	}

	// Tier 3 without urgent is refused; with a reason it is accepted.
	var ve *ValidationError
	if _, err := f.plans.SaveAssignments(ctx, acmUser, plan1, []VaultAssignmentInput{{TerminalID: f.terminal, VaultBranchID: f.cash3}}); !errors.As(err, &ve) {
		t.Fatalf("tier 3 non-urgent: want ValidationError, got %v", err)
	}
	reason := "kas region sendiri kosong"
	d, err := f.plans.SaveAssignments(ctx, acmUser, plan1, []VaultAssignmentInput{{TerminalID: f.terminal, VaultBranchID: f.cash3, IsUrgent: true, UrgentReason: &reason}})
	if err != nil || d.Atms[0].Assignment == nil || d.Atms[0].Assignment.Tier != 3 {
		t.Fatalf("tier 3 urgent save: %+v %v", d, err)
	}
	// Re-save (replace-all) onto cash1.
	if _, err := f.plans.SaveAssignments(ctx, acmUser, plan1, []VaultAssignmentInput{{TerminalID: f.terminal, VaultBranchID: f.cash1}}); err != nil {
		t.Fatalf("save plan1: %v", err)
	}

	// Submit plan1; the submitter cannot approve it; the ACM-SPV can.
	if _, err := f.plans.Submit(ctx, acmUser, plan1); err != nil {
		t.Fatalf("submit plan1: %v", err)
	}
	if _, err := f.plans.Approve(ctx, Actor{UserID: f.acmUserID, Role: "ACM-SPV"}, plan1); !errors.Is(err, ErrSelfApproval) {
		t.Fatalf("maker approves own plan: want ErrSelfApproval, got %v", err)
	}
	if _, err := f.plans.Approve(ctx, acmSpv, plan1); err != nil {
		t.Fatalf("approve plan1: %v", err)
	}
	if s := f.requestStatus(t, reqID); s != "vault_assignment" {
		t.Fatalf("request after 1 of 2 areas = %s, want vault_assignment", s)
	}

	// Plan2: submit without assignment is refused; ACM-SPV reject sends it back.
	if _, err := f.plans.Submit(ctx, acmUser, plan2); !errors.As(err, &ve) {
		t.Fatalf("submit incomplete: want ValidationError, got %v", err)
	}
	if _, err := f.plans.SaveAssignments(ctx, acmUser, plan2, []VaultAssignmentInput{{TerminalID: f.terminal2, VaultBranchID: f.cash1}}); err != nil {
		t.Fatalf("save plan2: %v", err)
	}
	if _, err := f.plans.Submit(ctx, acmUser, plan2); err != nil {
		t.Fatalf("submit plan2: %v", err)
	}
	if d, err = f.plans.Reject(ctx, acmSpv, plan2, "cek kapasitas"); err != nil || d.Plan.Status != planDraft {
		t.Fatalf("reject plan2: %+v %v", d, err)
	}
	if _, err := f.plans.Submit(ctx, acmUser, plan2); err != nil {
		t.Fatalf("resubmit plan2: %v", err)
	}
	if d, err = f.plans.Approve(ctx, acmSpv, plan2); err != nil {
		t.Fatalf("approve plan2: %v", err)
	}
	// Both ATMs on cash1 (1.5M saldo, 2 x 1M order): each sees 0.5M left -> warning.
	if a := d.Atms[0].Assignment; a == nil || !a.CapacityWarning || a.Capacity["100000"] != "500000" || a.Saldo["100000"] != "1500000" {
		t.Fatalf("plan2 snapshot = %+v, want saldo 1500000, capacity 500000, warning", a)
	}
	if s := f.requestStatus(t, reqID); s != "vault_review" {
		t.Fatalf("request after all areas approved = %s, want vault_review", s)
	}

	// ATM-SPV review: ACM people cannot review (four-eyes across layers); reject resets; approve -> ready.
	if err := f.plans.ReviewRequest(ctx, Actor{UserID: f.spvID, Role: "ATM-SPV"}, reqID, true, ""); !errors.Is(err, ErrSelfApproval) {
		t.Fatalf("ACM approver reviewing: want ErrSelfApproval, got %v", err)
	}
	if err := f.plans.ReviewRequest(ctx, f.checker, reqID, false, "vault terlalu jauh"); err != nil {
		t.Fatalf("vault reject: %v", err)
	}
	if s := f.requestStatus(t, reqID); s != "vault_assignment" {
		t.Fatalf("after vault reject = %s, want vault_assignment", s)
	}
	var drafts int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM vendor_request_vault_plans WHERE vendor_request_id = $1 AND status = 'draft'`, reqID).Scan(&drafts); err != nil || drafts != 2 {
		t.Fatalf("plans back to draft = %d (%v), want 2", drafts, err)
	}
	for _, id := range []int64{plan1, plan2} {
		if _, err := f.plans.Submit(ctx, acmUser, id); err != nil {
			t.Fatalf("resubmit %d: %v", id, err)
		}
		if _, err := f.plans.Approve(ctx, acmSpv, id); err != nil {
			t.Fatalf("reapprove %d: %v", id, err)
		}
	}
	if err := f.plans.ReviewRequest(ctx, f.checker, reqID, true, ""); err != nil {
		t.Fatalf("vault approve: %v", err)
	}
	if s := f.requestStatus(t, reqID); s != "ready" {
		t.Fatalf("after vault approve = %s, want ready", s)
	}

	// Notifications: ACM-SPV got the submits; audit trail exists for the plan.
	var n int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE recipient_user_id = $1 AND type = 'vault_plan.submitted'`, f.spvID).Scan(&n); err != nil || n < 4 {
		t.Fatalf("ACM-SPV submit notifications = %d (%v), want >= 4", n, err)
	}
	entries, err := f.plans.Audit(ctx, acmUser, plan2)
	if err != nil || len(entries) < 5 {
		t.Fatalf("plan2 audit = %d entries (%v), want >= 5", len(entries), err)
	}
	review, err := f.plans.ForRequest(ctx, reqID)
	if err != nil || len(review.Plans) != 2 || len(review.Assignments) != 2 {
		t.Fatalf("ForRequest = %+v %v", review, err)
	}
}

// Review R1: ADMIN moves a replenish branch to another area mid-flow. An ATM
// already assigned stays in the plan that holds its row (FR7.3); an unassigned
// ATM follows the branch's new area. Before the fix the new area's plan listed
// the ATM too, so saving it hit the (request, ATM) unique key -> 500.
func TestIntegration_VaultPlan_BranchMovedArea(t *testing.T) {
	f := newVaultPlanFixture(t)
	ctx := context.Background()
	acmUser := Actor{UserID: f.acmUserID, Role: "ACM-USER"}
	acmSpv := Actor{UserID: f.spvID, Role: "ACM-SPV"}
	reqID, plan1, plan2 := f.approvedTwoAtms(t)
	var branch2 int64
	if err := f.pool.QueryRow(ctx, `SELECT vendor_branch_id FROM acm_area_branches WHERE acm_area_id = $1`, f.area2).Scan(&branch2); err != nil {
		t.Fatal(err)
	}
	moveBranch2 := func(to int64) {
		t.Helper()
		if _, err := f.pool.Exec(ctx, `UPDATE acm_area_branches SET acm_area_id = $1 WHERE vendor_branch_id = $2`, to, branch2); err != nil {
			t.Fatal(err)
		}
	}
	atmsOf := func(planID int64) []string {
		t.Helper()
		d, err := f.plans.Get(ctx, acmUser, planID)
		if err != nil {
			t.Fatal(err)
		}
		out := []string{}
		for _, a := range d.Atms {
			out = append(out, a.TerminalID)
		}
		return out
	}

	// Unassigned ATM follows its branch: after the move, plan1 owns terminal2.
	moveBranch2(f.area1)
	if got := atmsOf(plan1); len(got) != 2 {
		t.Fatalf("plan1 after move = %v, want both ATMs", got)
	}
	if got := atmsOf(plan2); len(got) != 0 {
		t.Fatalf("plan2 after move = %v, want none", got)
	}
	moveBranch2(f.area2)

	// Assigned ATM stays: area2 saves terminal2, then the branch moves to area1.
	if _, err := f.plans.SaveAssignments(ctx, acmUser, plan2, []VaultAssignmentInput{{TerminalID: f.terminal2, VaultBranchID: f.cash1}}); err != nil {
		t.Fatal(err)
	}
	moveBranch2(f.area1)
	if got := atmsOf(plan1); len(got) != 1 || got[0] != f.terminal {
		t.Fatalf("plan1 = %v, want only %s (terminal2 kept by plan2)", got, f.terminal)
	}
	if got := atmsOf(plan2); len(got) != 1 || got[0] != f.terminal2 {
		t.Fatalf("plan2 = %v, want %s", got, f.terminal2)
	}
	// Plan1 cannot take terminal2 (not part of it) -> 422, not 500.
	var ve *ValidationError
	if _, err := f.plans.SaveAssignments(ctx, acmUser, plan1, []VaultAssignmentInput{
		{TerminalID: f.terminal, VaultBranchID: f.cash1}, {TerminalID: f.terminal2, VaultBranchID: f.cash1},
	}); !errors.As(err, &ve) {
		t.Fatalf("plan1 taking terminal2: want ValidationError, got %v", err)
	}
	// Both plans complete normally -> vault_review.
	if _, err := f.plans.SaveAssignments(ctx, acmUser, plan1, []VaultAssignmentInput{{TerminalID: f.terminal, VaultBranchID: f.cash1}}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{plan1, plan2} {
		if _, err := f.plans.Submit(ctx, acmUser, id); err != nil {
			t.Fatalf("submit %d: %v", id, err)
		}
		if _, err := f.plans.Approve(ctx, acmSpv, id); err != nil {
			t.Fatalf("approve %d: %v", id, err)
		}
	}
	if s := f.requestStatus(t, reqID); s != "vault_review" {
		t.Fatalf("request = %s, want vault_review", s)
	}

	// Review R3: ACM-USERs get a link they can open (their plan), the creator the request.
	if err := f.plans.ReviewRequest(ctx, f.checker, reqID, true, ""); err != nil {
		t.Fatal(err)
	}
	var acmLink, creatorLink string
	if err := f.pool.QueryRow(ctx, `SELECT link FROM notifications WHERE recipient_user_id = $1 AND type = 'vault_plan.reviewed' ORDER BY id DESC LIMIT 1`, f.acmUserID).Scan(&acmLink); err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT link FROM notifications WHERE recipient_user_id = $1 AND type = 'vault_plan.reviewed' ORDER BY id DESC LIMIT 1`, f.creator.UserID).Scan(&creatorLink); err != nil {
		t.Fatal(err)
	}
	if acmLink != fmt.Sprintf("/cit/vault-plans/%d", plan1) && acmLink != fmt.Sprintf("/cit/vault-plans/%d", plan2) {
		t.Fatalf("ACM-USER link = %q, want a vault plan page", acmLink)
	}
	if creatorLink != fmt.Sprintf("/replenishment/vendor-requests/%d", reqID) {
		t.Fatalf("creator link = %q", creatorLink)
	}
}

// NFR3: the last two area approvals racing each other must move the request to
// vault_review exactly once (both lock the request row first).
func TestIntegration_VaultPlan_ParallelApprove(t *testing.T) {
	f := newVaultPlanFixture(t)
	ctx := context.Background()
	acmUser := Actor{UserID: f.acmUserID, Role: "ACM-USER"}
	acmSpv := Actor{UserID: f.spvID, Role: "ACM-SPV"}
	reqID, plan1, plan2 := f.approvedTwoAtms(t)
	for id, term := range map[int64]string{plan1: f.terminal, plan2: f.terminal2} {
		if _, err := f.plans.SaveAssignments(ctx, acmUser, id, []VaultAssignmentInput{{TerminalID: term, VaultBranchID: f.cash1}}); err != nil {
			t.Fatal(err)
		}
		if _, err := f.plans.Submit(ctx, acmUser, id); err != nil {
			t.Fatal(err)
		}
	}

	errs := make(chan error, 2)
	for _, id := range []int64{plan1, plan2} {
		go func(id int64) {
			_, err := f.plans.Approve(ctx, acmSpv, id)
			errs <- err
		}(id)
	}
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("parallel approve: %v", err)
		}
	}
	if s := f.requestStatus(t, reqID); s != "vault_review" {
		t.Fatalf("request = %s, want vault_review", s)
	}
	var n int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE entity_type = 'vendor_request' AND entity_id = $1 AND action = 'vault_ready'`, reqID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("vault_ready audits = %d (%v), want exactly 1", n, err)
	}
}
