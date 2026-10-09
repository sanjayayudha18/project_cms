//go:build integration

package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/cimb-niaga/cms/backend/internal/notification"
)

// cit-send-vendor S4 on real Postgres: vendor scope (FR5), no cross-vendor
// leak (NFR4), accept / reject per party (FR4) and the request transitions.

type vendorOrderFixture struct {
	*vaultPlanFixture
	orders     *VendorOrderService
	reqID      int64
	vendor2    int64
	wideUser   VendorActor // fixture vendor, no branch pin: replenish x2 + cash1 vault
	branchUser VendorActor // fixture vendor pinned to the terminal's replenish branch
	cash2User  VendorActor // other vendor pinned to cash2: one vault party
}

func newVendorOrderFixture(t *testing.T) *vendorOrderFixture {
	t.Helper()
	f := &vendorOrderFixture{vaultPlanFixture: newVaultPlanFixture(t)}
	ctx := context.Background()
	if err := f.pool.QueryRow(ctx, `SELECT vendor_id FROM vendor_branches WHERE id = $1`, f.cash2).Scan(&f.vendor2); err != nil {
		t.Fatal(err)
	}
	f.wideUser = f.vendorUser(t, f.vendorID, nil)
	f.branchUser = f.vendorUser(t, f.vendorID, &f.branchID)
	f.cash2User = f.vendorUser(t, f.vendor2, &f.cash2)
	f.orders = NewVendorOrderService(f.pool, f.pool).WithNotifier(notification.NewService(false))
	f.reqID = f.sentToVendor(t)
	return f
}

func (f *vendorOrderFixture) vendorUser(t *testing.T, vendorID int64, branch *int64) VendorActor {
	t.Helper()
	ctx := context.Background()
	var id int64
	if err := f.pool.QueryRow(ctx, `INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source, password_hash, vendor_id, vendor_branch_id)
		SELECT id, $1, $1, $1 || '@vendor.test', false, 'local', 'x', $2, $3 FROM roles WHERE role = 'VENDOR-USER' RETURNING id`,
		"itvnd-"+uuid.NewString()[:8], vendorID, branch).Scan(&id); err != nil {
		t.Fatalf("seed vendor user: %v", err)
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = f.pool.Exec(c, `DELETE FROM notification_emails WHERE notification_id IN (SELECT id FROM notifications WHERE recipient_user_id = $1)`, id)
		_, _ = f.pool.Exec(c, `DELETE FROM notifications WHERE recipient_user_id = $1`, id)
		// audit_logs keeps the user as actor: deactivate if the delete is refused.
		_, _ = f.pool.Exec(c, `UPDATE users SET is_active = false, deleted_at = now() WHERE id = $1`, id)
		_, _ = f.pool.Exec(c, `DELETE FROM users WHERE id = $1`, id)
	})
	v := vendorID
	return VendorActor{Actor: Actor{UserID: id, Role: vendorUserRole}, ClaimVendorID: &v}
}

// party id of (branch, role) on the fixture request.
func (f *vendorOrderFixture) party(t *testing.T, branch int64, role string) int64 {
	t.Helper()
	var id int64
	if err := f.pool.QueryRow(context.Background(), `SELECT id FROM vendor_request_vendor_parties
		WHERE vendor_request_id = $1 AND vendor_branch_id = $2 AND role = $3`, f.reqID, branch, role).Scan(&id); err != nil {
		t.Fatalf("party %d/%s: %v", branch, role, err)
	}
	return id
}

func (f *vendorOrderFixture) replenish2(t *testing.T) int64 {
	t.Helper()
	var b int64
	if err := f.pool.QueryRow(context.Background(), `SELECT vendor_branch_id FROM vendor_request_vendor_parties
		WHERE vendor_request_id = $1 AND role = 'replenish' AND vendor_branch_id <> $2`, f.reqID, f.branchID).Scan(&b); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestIntegration_VendorOrder_ScopeAndNoLeak(t *testing.T) {
	f := newVendorOrderFixture(t)
	ctx := context.Background()
	cash1Vault := f.party(t, f.cash1, partyRoleVault)
	cash2Vault := f.party(t, f.cash2, partyRoleVault)

	// Fixture vendors are created fresh, so every listed row belongs to this request.
	list := func(va VendorActor) []VendorOrderSummary {
		t.Helper()
		l, err := f.orders.List(ctx, va, VendorOrderFilter{})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		return l.Items
	}
	if got := list(f.wideUser); len(got) != 3 {
		t.Fatalf("vendor-wide user sees %d parties, want 3 (2 replenish + cash1 vault)", len(got))
	}
	if got := list(f.branchUser); len(got) != 1 || got[0].BranchID != f.branchID || got[0].Role != partyRoleReplenish {
		t.Fatalf("branch-pinned user sees %+v, want only its replenish party", got)
	}
	got := list(f.cash2User)
	if len(got) != 1 || got[0].ID != cash2Vault || got[0].AtmCount != 1 || !got[0].CanDecide {
		t.Fatalf("cash2 user sees %+v, want only its vault party (1 ATM, decidable)", got)
	}

	// Leak: another vendor's party (and a sibling branch's) is a 404, also for actions.
	for name, tc := range map[string]struct {
		va    VendorActor
		party int64
	}{
		"other vendor":   {f.cash2User, cash1Vault},
		"sibling branch": {f.branchUser, cash1Vault},
	} {
		if _, err := f.orders.Get(ctx, tc.va, tc.party); !errors.Is(err, ErrVendorOrderNotFound) {
			t.Fatalf("%s Get: want ErrVendorOrderNotFound, got %v", name, err)
		}
		if _, err := f.orders.Accept(ctx, tc.va, tc.party); !errors.Is(err, ErrVendorOrderNotFound) {
			t.Fatalf("%s Accept: want ErrVendorOrderNotFound, got %v", name, err)
		}
	}
	d, err := f.orders.Get(ctx, f.cash2User, cash2Vault)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Content.Atms) != 1 || d.Content.Atms[0].TerminalID != f.terminal2 || d.Content.Atms[0].Counterpart.ID != f.replenish2(t) {
		t.Fatalf("cash2 detail = %+v, want only terminal2 with its replenish branch", d.Content)
	}
	if len(d.Totals) != 1 || d.Totals[0] != (DenomAmount{Denom: 100000, Amount: 1000000}) || d.Currency != "IDR" {
		t.Fatalf("cash2 totals = %+v %s", d.Totals, d.Currency)
	}

	// Not a vendor, or a stale vendor claim: not authorized.
	if _, err := f.orders.List(ctx, VendorActor{Actor: f.checker}, VendorOrderFilter{}); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("internal user: want ErrNotAuthorized, got %v", err)
	}
	stale := f.cash2User
	stale.ClaimVendorID = &f.vendorID
	if _, err := f.orders.List(ctx, stale, VendorOrderFilter{}); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("claim mismatch: want ErrNotAuthorized, got %v", err)
	}
}

func TestIntegration_VendorOrder_AcceptAll(t *testing.T) {
	f := newVendorOrderFixture(t)
	ctx := context.Background()
	r2 := f.replenish2(t)
	var ve *ValidationError
	if _, err := f.orders.Reject(ctx, f.cash2User, f.party(t, f.cash2, partyRoleVault), "pendek"); !errors.As(err, &ve) {
		t.Fatalf("short reason: want ValidationError, got %v", err)
	}
	for _, p := range []int64{f.party(t, f.branchID, partyRoleReplenish), f.party(t, r2, partyRoleReplenish)} {
		if _, err := f.orders.Accept(ctx, f.wideUser, p); err != nil {
			t.Fatalf("accept %d: %v", p, err)
		}
	}
	if _, err := f.orders.Accept(ctx, f.wideUser, f.party(t, f.branchID, partyRoleReplenish)); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("second decision: want ErrInvalidTransition, got %v", err)
	}
	if s := f.requestStatus(t, f.reqID); s != "sent_to_vendor" {
		t.Fatalf("after 2 of 4 = %s, want sent_to_vendor", s)
	}

	// The last two accept in parallel: the request moves exactly once.
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, c := range []struct {
		va    VendorActor
		party int64
	}{{f.wideUser, f.party(t, f.cash1, partyRoleVault)}, {f.cash2User, f.party(t, f.cash2, partyRoleVault)}} {
		wg.Add(1)
		go func(i int, va VendorActor, p int64) {
			defer wg.Done()
			_, errs[i] = f.orders.Accept(ctx, va, p)
		}(i, c.va, c.party)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatalf("parallel accept: %v", err)
		}
	}
	if s := f.requestStatus(t, f.reqID); s != "vendor_accepted" {
		t.Fatalf("after all accepted = %s, want vendor_accepted", s)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM audit_logs WHERE entity_type = 'vendor_request' AND entity_id = $1 AND action = 'vendor_vendor_accepted'`, f.reqID); n != 1 {
		t.Fatalf("request accepted audits = %d, want 1", n)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM audit_logs WHERE entity_type = 'vendor_request_vendor_party' AND action = 'vendor_accepted'
		AND entity_id IN (SELECT id FROM vendor_request_vendor_parties WHERE vendor_request_id = $1)`, f.reqID); n != 4 {
		t.Fatalf("party accept audits = %d, want 4", n)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM notifications WHERE recipient_user_id = $1 AND type = 'vendor_order.all_accepted' AND entity_id = $2`,
		f.creator.UserID, f.reqID); n != 1 {
		t.Fatalf("creator all-accepted notifications = %d, want 1", n)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM vendor_requests WHERE id = $1 AND vendor_accepted_at IS NOT NULL`, f.reqID); n != 1 {
		t.Fatal("vendor_accepted_at not stamped")
	}
}

func TestIntegration_VendorOrder_ReplenishReject(t *testing.T) {
	f := newVendorOrderFixture(t)
	ctx := context.Background()
	reason := "armada tidak tersedia tanggal itu"
	d, err := f.orders.Reject(ctx, f.branchUser, f.party(t, f.branchID, partyRoleReplenish), "  "+reason+"  ")
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if d.Status != "rejected" || d.RejectionReason == nil || *d.RejectionReason != reason || d.CanDecide {
		t.Fatalf("rejected party = %+v", d)
	}
	if s := f.requestStatus(t, f.reqID); s != "vendor_rejected" {
		t.Fatalf("request = %s, want vendor_rejected", s)
	}
	// Remaining pending parties are frozen until the request is sent again (FR4.5).
	if _, err := f.orders.Accept(ctx, f.cash2User, f.party(t, f.cash2, partyRoleVault)); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("decision after request left sent_to_vendor: want ErrInvalidTransition, got %v", err)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM notifications WHERE recipient_user_id = $1 AND type = 'vendor_order.replenish_rejected'
		AND entity_id = $2 AND body LIKE '%' || $3 || '%'`, f.creator.UserID, f.reqID, reason); n != 1 {
		t.Fatalf("creator reject notifications = %d, want 1", n)
	}

	// Review R1: the internal "Status Vendor" panel (FR6.6) on real rows.
	panel, err := f.orders.RequestParties(ctx, f.reqID)
	if err != nil {
		t.Fatalf("RequestParties: %v", err)
	}
	if len(panel.Parties) != 4 || panel.Parties[0].Role != partyRoleReplenish || panel.Parties[3].Role != partyRoleVault {
		t.Fatalf("parties = %+v, want 4 with replenish first", panel.Parties)
	}
	var rejected *RequestVendorParty
	for i := range panel.Parties {
		if panel.Parties[i].Status == "rejected" {
			rejected = &panel.Parties[i]
		}
	}
	if rejected == nil || rejected.BranchID != f.branchID || rejected.RejectionReason == nil || *rejected.RejectionReason != reason ||
		rejected.DecidedByName == nil || rejected.DecidedAt == nil || rejected.VendorName == "" {
		t.Fatalf("rejected party = %+v", rejected)
	}
	events := map[string]int{}
	for _, e := range panel.Events {
		events[e.Event]++
		if e.ActorName == "" || e.CreatedAt == nil {
			t.Fatalf("event without actor/time: %+v", e)
		}
	}
	if events["sent"] != 4 || events["rejected"] != 1 || len(panel.Events) != 5 {
		t.Fatalf("events = %v, want 4 sent + 1 rejected", events)
	}
}

func TestIntegration_VendorOrder_VaultRejectBackToAcm(t *testing.T) {
	f := newVendorOrderFixture(t)
	ctx := context.Background()
	cash1, cash2 := f.cash1, f.cash2
	if _, err := f.orders.Accept(ctx, f.branchUser, f.party(t, f.branchID, partyRoleReplenish)); err != nil {
		t.Fatal(err)
	}
	var approvedBefore string
	if err := f.pool.QueryRow(ctx, `SELECT approved_at::text FROM vendor_requests WHERE id = $1`, f.reqID).Scan(&approvedBefore); err != nil {
		t.Fatal(err)
	}
	reason := "saldo vault tidak cukup"
	if _, err := f.orders.Reject(ctx, f.cash2User, f.party(t, cash2, partyRoleVault), reason); err != nil {
		t.Fatalf("vault reject: %v", err)
	}
	if s := f.requestStatus(t, f.reqID); s != "vault_assignment" {
		t.Fatalf("request = %s, want vault_assignment", s)
	}
	var approvedAfter string
	if err := f.pool.QueryRow(ctx, `SELECT approved_at::text FROM vendor_requests WHERE id = $1`, f.reqID).Scan(&approvedAfter); err != nil || approvedAfter != approvedBefore {
		t.Fatalf("approved_at changed %s -> %s (%v): resend would force every party", approvedBefore, approvedAfter, err)
	}
	// Only the area plan holding terminal2 (vault cash2) is back to draft, with the vendor's reason.
	rows, err := f.pool.Query(ctx, `SELECT p.status, p.rejection_reason, p.rejected_by FROM vendor_request_vault_plans p
		JOIN vendor_request_vault_assignments a ON a.vault_plan_id = p.id WHERE p.vendor_request_id = $1 ORDER BY a.terminal_id = $2`, f.reqID, f.terminal2)
	if err != nil {
		t.Fatal(err)
	}
	var statuses []string
	for rows.Next() {
		var s string
		var r *string
		var by *int64
		if err := rows.Scan(&s, &r, &by); err != nil {
			t.Fatal(err)
		}
		if s == "draft" && (r == nil || *r != reason || by == nil || *by != f.cash2User.UserID) {
			t.Fatalf("draft plan reason/by = %v/%v", r, by)
		}
		statuses = append(statuses, s)
	}
	rows.Close()
	if strings.Join(statuses, ",") != "acm_approved,draft" {
		t.Fatalf("plan statuses (terminal, terminal2) = %v, want acm_approved,draft", statuses)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM notifications WHERE recipient_user_id = $1 AND type = 'vendor_order.vault_rejected'`, f.acmUserID); n != 1 {
		t.Fatalf("ACM-USER vault-reject notifications = %d, want 1", n)
	}

	// ACM moves terminal2 to cash1, ACM-SPV approves, ATM-SPV approves: resend only what changed.
	var plan2 int64
	if err := f.pool.QueryRow(ctx, `SELECT vault_plan_id FROM vendor_request_vault_assignments WHERE vendor_request_id = $1 AND terminal_id = $2`,
		f.reqID, f.terminal2).Scan(&plan2); err != nil {
		t.Fatal(err)
	}
	acmUser := Actor{UserID: f.acmUserID, Role: "ACM-USER"}
	if d, err := f.plans.Get(ctx, acmUser, plan2); err != nil || !d.Plan.RejectedByVendor {
		t.Fatalf("plan2 RejectedByVendor = %v (%v), want true (S8 label)", d != nil && d.Plan.RejectedByVendor, err)
	}
	if _, err := f.plans.SaveAssignments(ctx, acmUser, plan2, []VaultAssignmentInput{{TerminalID: f.terminal2, VaultBranchID: cash1}}); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	if _, err := f.plans.Submit(ctx, acmUser, plan2); err != nil {
		t.Fatalf("re-submit: %v", err)
	}
	if _, err := f.plans.Approve(ctx, Actor{UserID: f.spvID, Role: "ACM-SPV"}, plan2); err != nil {
		t.Fatalf("re-approve: %v", err)
	}
	if err := f.plans.ReviewRequest(ctx, f.checker, f.reqID, true, ""); err != nil {
		t.Fatalf("vault approve: %v", err)
	}
	after := partiesOf(t, f.pool, f.reqID)
	want := map[partyKey]string{
		{f.branchID, partyRoleReplenish}:      "accepted",  // unchanged, kept
		{f.replenish2(t), partyRoleReplenish}: "pending",   // counterpart changed
		{cash1, partyRoleVault}:               "pending",   // gained terminal2
		{cash2, partyRoleVault}:               "withdrawn", // no longer used
	}
	for k, s := range want {
		if after[k].status != s {
			t.Fatalf("party %+v = %s, want %s (all: %v)", k, after[k].status, s, after)
		}
	}
	if s := f.requestStatus(t, f.reqID); s != "sent_to_vendor" {
		t.Fatalf("request = %s, want sent_to_vendor", s)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM notifications WHERE recipient_user_id = $1 AND type = 'vendor_order.withdrawn'`, f.cash2User.UserID); n != 1 {
		t.Fatalf("cash2 withdrawn notifications = %d, want 1", n)
	}
}

// S5 (FR6.1/FR6.2/FR2.3): replenish reject -> ATM-SPV returns -> maker revises
// and drops terminal2 -> re-approval reopens plan1, cancels the now-empty
// plan2, keeps terminal's vault -> every remaining party must accept again.
func TestIntegration_VendorOrder_ReturnEditResend(t *testing.T) {
	f := newVendorOrderFixture(t)
	ctx := context.Background()
	r2 := f.replenish2(t)
	if _, err := f.orders.Reject(ctx, f.branchUser, f.party(t, f.branchID, partyRoleReplenish), "armada tidak tersedia"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.VendorReturn(ctx, f.creator, f.reqID, "kurangi ATM"); !errors.Is(err, ErrNotChecker) {
		t.Fatalf("maker returns: want ErrNotChecker, got %v", err)
	}
	if _, err := f.svc.VendorReturn(ctx, f.checker, f.reqID, "  "); !errors.Is(err, ErrRejectReasonEmpty) {
		t.Fatalf("empty reason: want ErrRejectReasonEmpty, got %v", err)
	}
	d, err := f.svc.VendorReturn(ctx, f.checker, f.reqID, "kurangi ATM")
	if err != nil || d.Status != "rejected" {
		t.Fatalf("return: %+v %v", d, err)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM notifications WHERE recipient_user_id = $1 AND type = 'vendor_request.vendor_returned' AND entity_id = $2`,
		f.creator.UserID, f.reqID); n != 1 {
		t.Fatalf("creator return notifications = %d, want 1", n)
	}
	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(), `DELETE FROM notifications WHERE recipient_user_id = $1 AND type = 'vendor_request.vendor_returned' AND entity_id = $2`,
			f.creator.UserID, f.reqID)
	})

	if _, err := f.svc.Revise(ctx, f.creator, f.reqID); err != nil {
		t.Fatalf("revise: %v", err)
	}
	seedDmaaRows(t, f.pool, f.terminal, 100000) // UpdateItems validates against dmaa_atm_forecast
	keep := []ItemInput{{TerminalID: f.terminal, PeriodePred: jakartaCalendarDate(0), Denom: 100000, AmountReplenish: 1000000}}
	if _, err := f.svc.UpdateItems(ctx, f.creator, f.reqID, keep); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM vendor_request_vault_assignments WHERE vendor_request_id = $1 AND terminal_id = $2`, f.reqID, f.terminal2); n != 0 {
		t.Fatalf("removed ATM keeps %d vault assignment(s)", n)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM vendor_request_vault_assignments WHERE vendor_request_id = $1 AND terminal_id = $2`, f.reqID, f.terminal); n != 1 {
		t.Fatal("kept ATM lost its vault assignment")
	}
	if _, err := f.svc.Submit(ctx, f.creator, f.reqID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := f.svc.Approve(ctx, f.checker, f.reqID); err != nil {
		t.Fatalf("re-approve: %v", err)
	}
	var plan1, plan2 string
	if err := f.pool.QueryRow(ctx, `SELECT
		(SELECT status FROM vendor_request_vault_plans WHERE vendor_request_id = $1 AND acm_area_id = $2),
		(SELECT status FROM vendor_request_vault_plans WHERE vendor_request_id = $1 AND acm_area_id = $3)`,
		f.reqID, f.area1, f.area2).Scan(&plan1, &plan2); err != nil || plan1 != "draft" || plan2 != "cancelled" {
		t.Fatalf("plans after re-approve = %s/%s (%v), want draft/cancelled", plan1, plan2, err)
	}

	var planID int64
	if err := f.pool.QueryRow(ctx, `SELECT id FROM vendor_request_vault_plans WHERE vendor_request_id = $1 AND acm_area_id = $2`, f.reqID, f.area1).Scan(&planID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.plans.Submit(ctx, Actor{UserID: f.acmUserID, Role: "ACM-USER"}, planID); err != nil {
		t.Fatalf("resubmit plan1: %v", err)
	}
	if _, err := f.plans.Approve(ctx, Actor{UserID: f.spvID, Role: "ACM-SPV"}, planID); err != nil {
		t.Fatalf("reapprove plan1: %v", err)
	}
	if err := f.plans.ReviewRequest(ctx, f.checker, f.reqID, true, ""); err != nil {
		t.Fatalf("vault approve: %v", err)
	}
	after := partiesOf(t, f.pool, f.reqID)
	want := map[partyKey]string{
		{f.branchID, partyRoleReplenish}: "pending", // rejected before, resent
		{r2, partyRoleReplenish}:         "withdrawn",
		{f.cash1, partyRoleVault}:        "pending", // same content, resent: every party accepts again (Q12)
		{f.cash2, partyRoleVault}:        "withdrawn",
	}
	for k, s := range want {
		if after[k].status != s {
			t.Fatalf("party %+v = %s, want %s (all: %v)", k, after[k].status, s, after)
		}
	}
}

// FR6.3/FR6.4: laporan selesai only once every vendor accepted; a rejected
// report goes back to vendor_accepted; cancel after sending tells the vendors.
func TestIntegration_VendorOrder_CompletionGateAndCancel(t *testing.T) {
	f := newVendorOrderFixture(t)
	ctx := context.Background()
	results := []CompletionResultInput{{f.terminal, "success"}, {f.terminal2, "success"}}
	if _, err := f.svc.SubmitCompletion(ctx, f.creator, f.reqID, results); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("completion before vendors accept: want ErrInvalidTransition, got %v", err)
	}
	for va, p := range map[*VendorActor]int64{
		&f.wideUser:  f.party(t, f.branchID, partyRoleReplenish),
		&f.cash2User: f.party(t, f.cash2, partyRoleVault),
	} {
		if _, err := f.orders.Accept(ctx, *va, p); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []int64{f.party(t, f.replenish2(t), partyRoleReplenish), f.party(t, f.cash1, partyRoleVault)} {
		if _, err := f.orders.Accept(ctx, f.wideUser, p); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.svc.SubmitCompletion(ctx, f.creator, f.reqID, results); err != nil {
		t.Fatalf("completion after vendor_accepted: %v", err)
	}
	if _, err := f.svc.RejectCompletion(ctx, f.checker, f.reqID, "foto kurang"); err != nil {
		t.Fatalf("reject completion: %v", err)
	}
	if s := f.requestStatus(t, f.reqID); s != "vendor_accepted" {
		t.Fatalf("after completion reject = %s, want vendor_accepted", s)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM vendor_requests WHERE id = $1 AND completion_rejection_reason = 'foto kurang'
		AND completion_rejected_by = $2`, f.reqID, f.checker.UserID); n != 1 {
		t.Fatal("completion rejection not stamped on the way back to vendor_accepted")
	}

	if _, err := f.svc.Cancel(ctx, f.creator, f.reqID, "batal"); !errors.Is(err, ErrNotChecker) {
		t.Fatalf("maker cancels a sent request: want ErrNotChecker, got %v", err)
	}
	if _, err := f.svc.Cancel(ctx, f.checker, f.reqID, "ATM ditutup"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM vendor_request_vendor_party_events e JOIN vendor_request_vendor_parties p ON p.id = e.party_id
		WHERE p.vendor_request_id = $1 AND e.event = 'cancelled'`, f.reqID); n != 4 {
		t.Fatalf("cancelled events = %d, want 4", n)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM notifications WHERE recipient_user_id = $1 AND type = 'vendor_order.cancelled'`, f.cash2User.UserID); n != 1 {
		t.Fatalf("vendor cancel notifications = %d, want 1", n)
	}
	d, err := f.orders.Get(ctx, f.cash2User, f.party(t, f.cash2, partyRoleVault))
	if err != nil || !d.IsCanceled || d.CanDecide || d.Status != "accepted" {
		t.Fatalf("vendor view after cancel = %+v %v, want canceled, no action, party status kept", d, err)
	}
}
