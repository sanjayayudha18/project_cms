//go:build integration

package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/cimb-niaga/cms/backend/internal/notification"
)

// cit-vault-select S3 end to end on real Postgres: one plan per request (area
// of the request's region), vault candidates with DSR saldo/capacity, the
// per-vault allocation of the order (save/submit rules), tiket pemenuhan vault,
// ACM approve/reject, ATM-SPV reject, RBAC and four-eyes.

type vaultPlanFixture struct {
	*vaultFlowFixture
	plans     *VaultPlanService
	terminal2 string
	area1     int64
	spvID     int64
	cash1     int64 // branch: same vendor, same region (tier 1)
	cash2     int64 // branch: other vendor, same region (tier 2)
	cash3     int64 // branch: same vendor, other region (tier 3)
	vault1    int64 // in cash1, DSR saldo 1.5M (100K)
	vault1b   int64 // in cash1, no DSR -> unknown
	vault2    int64 // in cash2, unknown
	vault3    int64 // in cash3, unknown
	codes     map[int64]string
}

func newVaultPlanFixture(t *testing.T) *vaultPlanFixture {
	t.Helper()
	f := &vaultPlanFixture{vaultFlowFixture: newVaultFlowFixture(t), codes: map[int64]string{}}
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

	// Cash branches and their vaults.
	f.cash1 = row(`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, region_code, category) VALUES ($1, $2, 'Cash 1', 'ITEST', 'CASH') RETURNING id`, f.vendorID, "ITC1-"+tag)
	f.cash3 = row(`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, region_code, category) VALUES ($1, $2, 'Cash 3', 'ITOTHER', 'CASH') RETURNING id`, f.vendorID, "ITC3-"+tag)
	vendor2 := row(`INSERT INTO vendors (code, name) VALUES ($1, $1) RETURNING id`, "ITV2-"+tag)
	f.cash2 = row(`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, region_code, category) VALUES ($1, $2, 'Cash 2', 'ITEST', 'ATM_CASH') RETURNING id`, vendor2, "ITC2-"+tag)
	vault := func(branch int64, code string) int64 {
		id := row(`INSERT INTO vendor_vaults (vendor_branch_id, vault_code, type, category, currency_code) VALUES ($1, $2, 'CASH', 'CASH', 'IDR') RETURNING id`, branch, code)
		f.codes[id] = code
		return id
	}
	f.vault1 = vault(f.cash1, "ITV1-"+tag)
	f.vault1b = vault(f.cash1, "ITV1B-"+tag)
	f.vault2 = vault(f.cash2, "ITV2-"+tag)
	f.vault3 = vault(f.cash3, "ITV3-"+tag)
	upload := row(`INSERT INTO dsr_uploads (filename, vendor, report_date, daily_status) VALUES ($1, $2, $3, 'completed') RETURNING id`,
		"it-"+tag+".xlsx", vendorName, jakartaCalendarDate(0))
	exec(`INSERT INTO dsr_daily_rows (upload_id, row_no, location, section, flow, line_label,
		denom_100k_idr, denom_50k_idr, denom_20k_idr, denom_10k_idr, denom_5k_idr, denom_2k_idr, denom_1k_idr, line_total_idr)
		VALUES ($1, 1, 'IT VAULT', 'd0', 'saldo_akhir', 'SALDO AKHIR', 1500000, 0, 0, 0, 0, 0, 0, 1500000)`, upload)
	exec(`INSERT INTO dsr_location_vault_maps (vendor_id, dsr_location, vendor_vault_id) VALUES ($1, 'it vault', $2)`, f.vendorID, f.vault1)

	// One area holding the fixture region; ACM-USER (from the base fixture) + ACM-SPV are members.
	f.area1 = f.area(t, true)
	f.spvID = row(`INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source)
		SELECT id, $1, $1, $1 || '@test.local', true, 'ldap' FROM roles WHERE role = 'ACM-SPV' RETURNING id`, "itspv-"+tag)
	exec(`INSERT INTO acm_area_members (acm_area_id, user_id, created_by) VALUES ($1, $2, $3)`, f.area1, f.spvID, f.creator.UserID)

	f.plans = NewVaultPlanService(pool, pool).WithNotifier(notification.NewService(false))
	vaults := []int64{f.vault1, f.vault1b, f.vault2, f.vault3}

	// Runs before the base fixture's cleanup (LIFO): children first.
	t.Cleanup(func() {
		c := context.Background()
		ex := func(sql string, args ...any) { _, _ = pool.Exec(c, sql, args...) }
		cleanupVendorParties(c, pool, f.vendorID)
		ex(`DELETE FROM vendor_request_vault_tickets WHERE vendor_vault_id = ANY($1)`, vaults)
		ex(`DELETE FROM vendor_request_vault_allocations WHERE vendor_vault_id = ANY($1)`, vaults)
		ex(`DELETE FROM vendor_request_vault_plans WHERE vendor_request_id IN (SELECT id FROM vendor_requests WHERE vendor_id = $1)`, f.vendorID)
		ex(`DELETE FROM acm_area_members WHERE user_id = $1`, f.spvID)
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
		ex(`DELETE FROM vendor_vaults WHERE id = ANY($1)`, vaults)
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

// approvedRequest creates a request with 1,000,000 (100K) per given ATM, approves
// it and returns the request and its single plan.
func (f *vaultPlanFixture) approvedRequest(t *testing.T, terminals ...string) (requestID, planID int64) {
	t.Helper()
	ctx := context.Background()
	in := manualCreateInput(f.vendorID, terminals[0], 100000)
	for _, term := range terminals[1:] {
		in.Items = append(in.Items, ItemInput{TerminalID: term, Denom: 100000, AmountReplenish: 1000000})
	}
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
	if err := f.pool.QueryRow(ctx, `SELECT id FROM vendor_request_vault_plans WHERE vendor_request_id = $1 AND status <> 'cancelled'`, created.ID).Scan(&planID); err != nil {
		t.Fatalf("plan of request %d: %v", created.ID, err)
	}
	return created.ID, planID
}

// approvedTwoAtms: one request for both fixture ATMs, order 2,000,000 (100K).
func (f *vaultPlanFixture) approvedTwoAtms(t *testing.T) (requestID, planID int64) {
	return f.approvedRequest(t, f.terminal, f.terminal2)
}

func (f *vaultPlanFixture) requestStatus(t *testing.T, id int64) string {
	t.Helper()
	var s string
	if err := f.pool.QueryRow(context.Background(), `SELECT status FROM vendor_requests WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

// ticket is the expected vault ticket number for the fixture's replenish date (today).
func (f *vaultPlanFixture) ticket(vault int64, seq string) string {
	return f.codes[vault] + "_" + jakartaCalendarDate(0).Format("20060102") + "_" + seq
}

// tickets returns ticket_number -> is_active for a request.
func (f *vaultPlanFixture) tickets(t *testing.T, requestID int64) map[string]bool {
	t.Helper()
	rows, err := f.pool.Query(context.Background(), `SELECT ticket_number, is_active FROM vendor_request_vault_tickets WHERE vendor_request_id = $1`, requestID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var n string
		var active bool
		if err := rows.Scan(&n, &active); err != nil {
			t.Fatal(err)
		}
		out[n] = active
	}
	return out
}

// line allocates amount (100K denom) to a vault.
func line(vault int64, amount string) AllocationLineInput {
	return AllocationLineInput{VendorVaultID: vault, Amounts: map[string]string{"100000": amount}}
}

func TestIntegration_VaultPlan_FullFlow(t *testing.T) {
	f := newVaultPlanFixture(t)
	ctx := context.Background()
	acmUser := Actor{UserID: f.acmUserID, Role: "ACM-USER"}
	acmSpv := Actor{UserID: f.spvID, Role: "ACM-SPV"}
	reqID, plan := f.approvedTwoAtms(t)

	// RBAC: a user outside the area sees nothing; ADMIN reads but cannot write.
	if _, err := f.plans.Get(ctx, Actor{UserID: f.creator.UserID, Role: "ACM-USER"}, plan); !errors.Is(err, ErrVaultPlanNotFound) {
		t.Fatalf("non-member Get: want ErrVaultPlanNotFound, got %v", err)
	}
	admin := Actor{UserID: f.creator.UserID, Role: "ADMIN"}
	d, err := f.plans.Get(ctx, admin, plan)
	if err != nil || len(d.Atms) != 2 || d.OrderTotals["100000"] != "2000000" || deref(d.Plan.RegionCode) != fixtureRegion {
		t.Fatalf("ADMIN Get = %+v %v, want 2 ATMs, order 2000000, region %s", d, err, fixtureRegion)
	}
	if _, err := f.plans.SaveAllocations(ctx, admin, plan, nil, nil); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("ADMIN save: want ErrNotAuthorized, got %v", err)
	}
	if mine, err := f.plans.List(ctx, acmUser, VaultPlanListFilter{AreaID: &f.area1}); err != nil || len(mine) != 1 || mine[0].ID != plan {
		t.Fatalf("member list = %+v %v, want plan %d", mine, err, plan)
	}
	if others, err := f.plans.List(ctx, Actor{UserID: f.creator.UserID, Role: "ACM-USER"}, VaultPlanListFilter{AreaID: &f.area1}); err != nil || len(others) != 0 {
		t.Fatalf("non-member list = %d plans (%v), want 0", len(others), err)
	}

	// Candidates per branch -> vault: tier 1 (known saldo first), tier 2; tier 3 only when urgent.
	branches, err := f.plans.Candidates(ctx, acmUser, plan, false)
	if err != nil {
		t.Fatal(err)
	}
	byBranch := map[int64]VaultBranchOption{}
	for _, b := range branches {
		byBranch[b.VendorBranchID] = b
	}
	c1 := byBranch[f.cash1]
	if c1.Tier != 1 || len(c1.Vaults) != 2 || c1.Vaults[0].VendorVaultID != f.vault1 || !c1.Vaults[0].SaldoKnown ||
		c1.Vaults[0].Capacity["100000"] != "1500000" || c1.Vaults[1].SaldoKnown {
		t.Fatalf("cash1 = %+v, want tier 1, vault1 (saldo 1500000) then vault1b (unknown)", c1)
	}
	if c2 := byBranch[f.cash2]; c2.Tier != 2 || len(c2.Vaults) != 1 {
		t.Fatalf("cash2 = %+v, want tier 2 with one vault", c2)
	}
	if _, ok := byBranch[f.cash3]; ok {
		t.Fatal("tier 3 offered without urgent")
	}
	urgent, _ := f.plans.Candidates(ctx, acmUser, plan, true)
	found := false
	for _, b := range urgent {
		found = found || (b.VendorBranchID == f.cash3 && b.Tier == 3)
	}
	if !found {
		t.Fatal("urgent candidates must include tier 3 cash3")
	}

	// Save: tier 3 needs urgent; amounts must be multiples. A short total saves but cannot submit.
	var ve *ValidationError
	if _, err := f.plans.SaveAllocations(ctx, acmUser, plan, []AllocationLineInput{line(f.vault3, "2000000")}, nil); !errors.As(err, &ve) {
		t.Fatalf("tier 3 non-urgent: want ValidationError, got %v", err)
	}
	if _, err := f.plans.SaveAllocations(ctx, acmUser, plan, []AllocationLineInput{line(f.vault1, "1550000")}, nil); !errors.As(err, &ve) {
		t.Fatalf("not a multiple: want ValidationError, got %v", err)
	}
	if _, err := f.plans.SaveAllocations(ctx, acmUser, plan, []AllocationLineInput{line(f.vault1, "1500000"), line(f.vault1b, "400000")}, nil); err != nil {
		t.Fatalf("save short: %v", err)
	}
	if _, err := f.plans.Submit(ctx, acmUser, plan); !errors.As(err, &ve) || !strings.Contains(ve.Message, "kurang Rp100000") {
		t.Fatalf("submit short: want 'kurang Rp100000', got %v", err)
	}
	// Excess needs a buffer reason; with it the plan submits and tickets are issued.
	if _, err := f.plans.SaveAllocations(ctx, acmUser, plan, []AllocationLineInput{line(f.vault1, "1500000"), line(f.vault1b, "600000")}, nil); err != nil {
		t.Fatalf("save excess: %v", err)
	}
	if _, err := f.plans.Submit(ctx, acmUser, plan); !errors.As(err, &ve) || !strings.Contains(ve.Message, "buffer") {
		t.Fatalf("submit excess without reason: want buffer error, got %v", err)
	}
	buffer := "cadangan antrean ATM ramai akhir pekan"
	if _, err := f.plans.SaveAllocations(ctx, acmUser, plan, []AllocationLineInput{line(f.vault1, "1500000"), line(f.vault1b, "600000")}, &buffer); err != nil {
		t.Fatalf("save with buffer reason: %v", err)
	}
	if d, err = f.plans.Submit(ctx, acmUser, plan); err != nil {
		t.Fatalf("submit: %v", err)
	}
	t1, t1b := f.ticket(f.vault1, "001"), f.ticket(f.vault1b, "001")
	if len(d.Allocations) != 2 || deref(d.Plan.BufferReason) != buffer {
		t.Fatalf("submitted plan = %+v", d)
	}
	for _, a := range d.Allocations {
		switch a.VendorVaultID {
		case f.vault1:
			if deref(a.TicketNumber) != t1 || a.CapacityWarning || a.Saldo["100000"] != "1500000" || a.Capacity["100000"] != "1500000" {
				t.Fatalf("vault1 row = %+v, want ticket %s, saldo/capacity 1500000, no warning", a, t1)
			}
		case f.vault1b:
			if deref(a.TicketNumber) != t1b || !a.CapacityWarning || a.Saldo != nil {
				t.Fatalf("vault1b row = %+v, want ticket %s, unknown saldo -> warning", a, t1b)
			}
		}
	}

	// Four-eyes; ACM-SPV reject keeps the tickets.
	if _, err := f.plans.Approve(ctx, Actor{UserID: f.acmUserID, Role: "ACM-SPV"}, plan); !errors.Is(err, ErrSelfApproval) {
		t.Fatalf("maker approves own plan: want ErrSelfApproval, got %v", err)
	}
	if d, err = f.plans.Reject(ctx, acmSpv, plan, "kurangi buffer"); err != nil || d.Plan.Status != planDraft {
		t.Fatalf("reject: %+v %v", d, err)
	}
	if got := f.tickets(t, reqID); len(got) != 2 || !got[t1] || !got[t1b] {
		t.Fatalf("tickets after reject = %v, want both active", got)
	}

	// Resubmit: vault1 with a new amount keeps its number, vault1b dropped -> inactive; no excess -> reason cleared.
	if _, err := f.plans.SaveAllocations(ctx, acmUser, plan, []AllocationLineInput{line(f.vault1, "2000000")}, &buffer); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	if d, err = f.plans.Submit(ctx, acmUser, plan); err != nil || d.Plan.BufferReason != nil {
		t.Fatalf("resubmit = %+v %v, want buffer_reason cleared", d, err)
	}
	if got := f.tickets(t, reqID); len(got) != 2 || !got[t1] || got[t1b] {
		t.Fatalf("tickets after resubmit = %v, want %s active, %s inactive", got, t1, t1b)
	}
	if d, err = f.plans.Approve(ctx, acmSpv, plan); err != nil || d.Plan.Status != planAcmApproved {
		t.Fatalf("approve: %+v %v", d, err)
	}
	// vault1: saldo 1.5M, allocated 2M -> warning, but never blocking.
	if a := d.Allocations[0]; !a.CapacityWarning || a.Capacity["100000"] != "1500000" {
		t.Fatalf("vault1 snapshot = %+v, want capacity 1500000 + warning", a)
	}
	if s := f.requestStatus(t, reqID); s != "vault_review" {
		t.Fatalf("request after the plan approved = %s, want vault_review", s)
	}

	// ATM-SPV review: ACM people cannot review; reject -> plan draft, tickets untouched.
	if err := f.plans.ReviewRequest(ctx, Actor{UserID: f.spvID, Role: "ATM-SPV"}, reqID, false, "x"); !errors.Is(err, ErrSelfApproval) {
		t.Fatalf("ACM approver reviewing: want ErrSelfApproval, got %v", err)
	}
	if err := f.plans.ReviewRequest(ctx, f.checker, reqID, false, "vault terlalu jauh"); err != nil {
		t.Fatalf("vault reject: %v", err)
	}
	if s := f.requestStatus(t, reqID); s != "vault_assignment" {
		t.Fatalf("after vault reject = %s, want vault_assignment", s)
	}
	// vault1b comes back -> reactivated with its old number, not a new one.
	if _, err := f.plans.SaveAllocations(ctx, acmUser, plan, []AllocationLineInput{line(f.vault1, "1500000"), line(f.vault1b, "500000")}, nil); err != nil {
		t.Fatalf("save after vault reject: %v", err)
	}
	if _, err := f.plans.Submit(ctx, acmUser, plan); err != nil {
		t.Fatalf("submit after vault reject: %v", err)
	}
	if got := f.tickets(t, reqID); len(got) != 2 || !got[t1] || !got[t1b] {
		t.Fatalf("tickets after vault1b returns = %v, want both active, same numbers", got)
	}

	// Audit + notification + the ATM-SPV view of the same plan.
	entries, err := f.plans.Audit(ctx, acmUser, plan)
	if err != nil || len(entries) < 8 {
		t.Fatalf("plan audit = %d entries (%v), want >= 8", len(entries), err)
	}
	var reactivated string
	if err := f.pool.QueryRow(ctx, `SELECT after->'vault_tickets'->>'reactivated' FROM audit_logs
		WHERE entity_type = 'vendor_request_vault_plan' AND entity_id = $1 AND action = 'submit' ORDER BY id DESC LIMIT 1`, plan).Scan(&reactivated); err != nil ||
		!strings.Contains(reactivated, t1b) {
		t.Fatalf("last submit audit reactivated = %q (%v), want %s", reactivated, err, t1b)
	}
	var n int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE recipient_user_id = $1 AND type = 'vault_plan.submitted'`, f.spvID).Scan(&n); err != nil || n != 3 {
		t.Fatalf("ACM-SPV submit notifications = %d (%v), want 3", n, err)
	}
	review, err := f.plans.ForRequest(ctx, reqID)
	if err != nil || review == nil || review.Plan.ID != plan || len(review.Allocations) != 2 || len(review.Tickets) != 2 {
		t.Fatalf("ForRequest = %+v %v", review, err)
	}
}

// FR5.2/FR5.3/FR6.1/FR6.3: capacity counts other requests' saved allocations
// on the same date (not the plan being edited, not cancelled requests);
// ticket numbers run per (vault, date) across requests; cancel keeps tickets.
func TestIntegration_VaultPlan_CapacityAndTicketsAcrossRequests(t *testing.T) {
	f := newVaultPlanFixture(t)
	ctx := context.Background()
	acmUser := Actor{UserID: f.acmUserID, Role: "ACM-USER"}
	reqA, planA := f.approvedRequest(t, f.terminal)
	reqB, planB := f.approvedRequest(t, f.terminal2)

	capacity := func(plan int64) string {
		t.Helper()
		branches, err := f.plans.Candidates(ctx, acmUser, plan, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range branches {
			for _, v := range b.Vaults {
				if v.VendorVaultID == f.vault1 {
					return v.Capacity["100000"]
				}
			}
		}
		t.Fatal("vault1 not offered")
		return ""
	}
	if _, err := f.plans.SaveAllocations(ctx, acmUser, planA, []AllocationLineInput{line(f.vault1, "1000000")}, nil); err != nil {
		t.Fatal(err)
	}
	if got := capacity(planA); got != "1500000" {
		t.Fatalf("own plan sees capacity %s, want 1500000 (not counting itself)", got)
	}
	if got := capacity(planB); got != "500000" {
		t.Fatalf("other request sees capacity %s, want 500000", got)
	}
	if _, err := f.plans.Submit(ctx, acmUser, planA); err != nil {
		t.Fatal(err)
	}
	if _, err := f.plans.SaveAllocations(ctx, acmUser, planB, []AllocationLineInput{line(f.vault1, "1000000")}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := f.plans.Submit(ctx, acmUser, planB); err != nil {
		t.Fatal(err)
	}
	if got := f.tickets(t, reqA); !got[f.ticket(f.vault1, "001")] {
		t.Fatalf("request A tickets = %v, want _001", got)
	}
	if got := f.tickets(t, reqB); !got[f.ticket(f.vault1, "002")] {
		t.Fatalf("request B tickets = %v, want _002 (same vault + date)", got)
	}

	// Cancel A: its plan is cancelled, its ticket stays as it was, B's capacity is freed.
	if _, err := f.svc.Cancel(ctx, f.checker, reqA, "batal uji"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if got := f.tickets(t, reqA); len(got) != 1 || !got[f.ticket(f.vault1, "001")] {
		t.Fatalf("request A tickets after cancel = %v, want unchanged", got)
	}
	if got := capacity(planB); got != "1500000" {
		t.Fatalf("capacity after A cancelled = %s, want 1500000", got)
	}
}
