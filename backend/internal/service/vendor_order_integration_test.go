//go:build integration

package service

import (
	"context"
	"errors"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/notification"
	"github.com/cimb-niaga/cms/backend/internal/vendorrole"
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
	users      *[]int64    // vendor users seeded by vendorUserAs, removed by deleteVendorUsers
}

// deleteVendorUsers removes the test-only vendor users and the audit rows they wrote, after the base
// fixtures removed every request, party and realization that points at them. Own connection: the
// fixture pool is already closed by then.
func deleteVendorUsers(t *testing.T, ids []int64) {
	if len(ids) == 0 {
		return
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Errorf("cleanup vendor users: %v", err)
		return
	}
	defer func() { _ = conn.Close(ctx) }()
	for _, sql := range []string{
		`DELETE FROM notification_emails WHERE notification_id IN (SELECT id FROM notifications WHERE recipient_user_id = ANY($1))`,
		`DELETE FROM notifications WHERE recipient_user_id = ANY($1)`,
		`DELETE FROM audit_logs WHERE actor_id = ANY($1)`,
		`DELETE FROM users WHERE id = ANY($1)`,
	} {
		if _, err := conn.Exec(ctx, sql, ids); err != nil {
			t.Errorf("cleanup vendor users %v: %v", ids, err)
			return
		}
	}
}

func newVendorOrderFixture(t *testing.T) *vendorOrderFixture {
	t.Helper()
	users := &[]int64{}
	// Registered before the base fixtures, so it runs after them (LIFO).
	t.Cleanup(func() { deleteVendorUsers(t, *users) })
	f := &vendorOrderFixture{vaultPlanFixture: newVaultPlanFixture(t), users: users}
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
	return f.vendorUserAs(t, vendorrole.User, vendorID, branch)
}

// vendorUserAs seeds an active local user with a vendor role; the token role matches the row.
func (f *vendorOrderFixture) vendorUserAs(t *testing.T, role string, vendorID int64, branch *int64) VendorActor {
	t.Helper()
	ctx := context.Background()
	var id int64
	if err := f.pool.QueryRow(ctx, `INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source, password_hash, vendor_id, vendor_branch_id)
		SELECT id, $1, $1, $1 || '@vendor.test', false, 'local', 'x', $2, $3 FROM roles WHERE role = $4 RETURNING id`,
		"itvnd-"+uuid.NewString()[:8], vendorID, branch, role).Scan(&id); err != nil {
		t.Fatalf("seed vendor user: %v", err)
	}
	*f.users = append(*f.users, id)
	// Runs before the base cleanup (LIFO), while requests/parties still point at the user: only
	// release the vendor / cabang FK so the base fixture can delete them. The row itself goes in
	// deleteVendorUsers, after the base cleanup.
	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(), `UPDATE users SET vendor_id = NULL, vendor_branch_id = NULL WHERE id = $1`, id)
	})
	v := vendorID
	return VendorActor{Actor: Actor{UserID: id, Role: role}, ClaimVendorID: &v}
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
	if len(got) != 1 || got[0].ID != cash2Vault || got[0].AtmCount != 0 || got[0].VaultCount != 1 || !got[0].CanDecide {
		t.Fatalf("cash2 user sees %+v, want only its vault party (1 vault, no ATM, decidable)", got)
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
	c := d.Content
	if len(c.Atms) != 0 || len(c.Vaults) != 1 || c.Vaults[0].Vault.ID != f.vault2 || c.Vaults[0].TicketNumber != f.ticket(f.vault2, "001") ||
		len(c.ReplenishBranches) != 2 {
		t.Fatalf("cash2 detail = %+v, want vault2 + ticket, both replenish branches, no ATM", c)
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

// vendor-party-scope Req 10.1 (a)-(d) + Req 4.1/4.2: one branch ("Medan" = the fixture's
// replenish branch) is both a replenish and a vault party of the same request. Each vendor
// role sees only its party roles, a branch-pinned account only its branch, VENDOR-USER is
// unchanged, and a role changed after the token was issued is refused at once.
func TestIntegration_VendorOrder_PartyRoleScope(t *testing.T) {
	f := newVendorOrderFixture(t)
	ctx := context.Background()
	mdnReplenish := f.party(t, f.branchID, partyRoleReplenish)
	otherReplenish := f.party(t, f.replenish2(t), partyRoleReplenish)
	cash1Vault := f.party(t, f.cash1, partyRoleVault)
	var mdnVault int64
	if err := f.pool.QueryRow(ctx, `INSERT INTO vendor_request_vendor_parties (vendor_request_id, vendor_branch_id, role, content)
		SELECT vendor_request_id, $2, 'vault', content FROM vendor_request_vendor_parties WHERE id = $1 RETURNING id`,
		cash1Vault, f.branchID).Scan(&mdnVault); err != nil {
		t.Fatalf("seed Medan vault party: %v", err)
	}

	atmMdn := f.vendorUserAs(t, vendorrole.ATMUser, f.vendorID, &f.branchID)
	vaultMdn := f.vendorUserAs(t, vendorrole.VaultUser, f.vendorID, &f.branchID)
	atmWide := f.vendorUserAs(t, vendorrole.ATMUser, f.vendorID, nil)
	vaultWide := f.vendorUserAs(t, vendorrole.VaultUser, f.vendorID, nil)

	// Listed ids (fixture vendors are fresh, so every row belongs to this request); Count must agree.
	listIDs := func(va VendorActor) []int64 {
		t.Helper()
		l, err := f.orders.List(ctx, va, VendorOrderFilter{})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if l.Total != int64(len(l.Items)) {
			t.Fatalf("count %d != listed %d", l.Total, len(l.Items))
		}
		ids := make([]int64, 0, len(l.Items))
		for _, it := range l.Items {
			ids = append(ids, it.ID)
		}
		slices.Sort(ids)
		return ids
	}
	sorted := func(ids ...int64) []int64 { slices.Sort(ids); return ids }
	for _, tc := range []struct {
		name string
		va   VendorActor
		want []int64
	}{
		{"(a) vault Medan: Medan vault only", vaultMdn, sorted(mdnVault)},
		{"(b) ATM Medan: Medan replenish only", atmMdn, sorted(mdnReplenish)},
		{"ATM vendor-wide: both replenish, no vault", atmWide, sorted(mdnReplenish, otherReplenish)},
		{"vault vendor-wide: both vaults, no replenish", vaultWide, sorted(mdnVault, cash1Vault)},
		{"(c) VENDOR-USER Medan: both roles of Medan", f.branchUser, sorted(mdnReplenish, mdnVault)},
		{"(c) VENDOR-USER vendor-wide: everything", f.wideUser, sorted(mdnReplenish, otherReplenish, cash1Vault, mdnVault)},
	} {
		if got := listIDs(tc.va); !slices.Equal(got, tc.want) {
			t.Errorf("%s: list = %v, want %v", tc.name, got, tc.want)
		}
	}

	// Detail and actions outside the scope are 404, never a leak.
	for _, tc := range []struct {
		name  string
		va    VendorActor
		party int64
	}{
		{"(a) vault Medan -> replenish Medan", vaultMdn, mdnReplenish},
		{"(b) ATM Medan -> replenish other branch", atmMdn, otherReplenish},
		{"ATM Medan -> vault Medan", atmMdn, mdnVault},
		{"ATM vendor-wide -> vault", atmWide, cash1Vault},
		{"vault vendor-wide -> replenish", vaultWide, mdnReplenish},
	} {
		if _, err := f.orders.Get(ctx, tc.va, tc.party); !errors.Is(err, ErrVendorOrderNotFound) {
			t.Errorf("%s Get: want ErrVendorOrderNotFound, got %v", tc.name, err)
		}
		if _, err := f.orders.Accept(ctx, tc.va, tc.party); !errors.Is(err, ErrVendorOrderNotFound) {
			t.Errorf("%s Accept: want ErrVendorOrderNotFound, got %v", tc.name, err)
		}
		if _, err := f.orders.Reject(ctx, tc.va, tc.party, "armada tidak tersedia"); !errors.Is(err, ErrVendorOrderNotFound) {
			t.Errorf("%s Reject: want ErrVendorOrderNotFound, got %v", tc.name, err)
		}
	}
	if d, err := f.orders.Get(ctx, vaultMdn, mdnVault); err != nil || d.Role != partyRoleVault {
		t.Fatalf("vault Medan own party: %+v, %v", d, err)
	}

	// Req 4.1/4.2: the vault account cannot report realisasi (404) nor list it (403); ATM + VENDOR-USER can list.
	if _, err := f.orders.SubmitRealization(ctx, vaultMdn, mdnReplenish, realizationRows(t, f.pool, f.reqID, nil)); !errors.Is(err, ErrVendorOrderNotFound) {
		t.Fatalf("vault realization: want ErrVendorOrderNotFound, got %v", err)
	}
	today := jakartaCalendarDate(0)
	rf := ReplenishmentFilter{From: today.AddDate(0, 0, -7), To: today}
	if _, err := f.orders.ListRealizations(ctx, vaultMdn, rf); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("vault realization list: want ErrNotAuthorized, got %v", err)
	}
	for _, va := range []VendorActor{atmMdn, f.wideUser} {
		if _, err := f.orders.ListRealizations(ctx, va, rf); err != nil {
			t.Fatalf("%s realization list: %v", va.Role, err)
		}
	}

	// (d) role changed in the DB after the token was issued: the old token is refused (403) everywhere.
	if _, err := f.pool.Exec(ctx, `UPDATE users SET role_id = (SELECT id FROM roles WHERE role = $2) WHERE id = $1`,
		atmMdn.UserID, vendorrole.VaultUser); err != nil {
		t.Fatal(err)
	}
	if _, err := f.orders.List(ctx, atmMdn, VendorOrderFilter{}); !errors.Is(err, ErrNotAuthorized) {
		t.Errorf("(d) stale role List: want ErrNotAuthorized, got %v", err)
	}
	if _, err := f.orders.Get(ctx, atmMdn, mdnReplenish); !errors.Is(err, ErrNotAuthorized) {
		t.Errorf("(d) stale role Get: want ErrNotAuthorized, got %v", err)
	}
	if _, err := f.orders.Accept(ctx, atmMdn, mdnReplenish); !errors.Is(err, ErrNotAuthorized) {
		t.Errorf("(d) stale role Accept: want ErrNotAuthorized, got %v", err)
	}
	if _, err := f.orders.ListRealizations(ctx, atmMdn, rf); !errors.Is(err, ErrNotAuthorized) {
		t.Errorf("(d) stale role realization list: want ErrNotAuthorized, got %v", err)
	}
	fresh := atmMdn
	fresh.Role = vendorrole.VaultUser // a new token carries the new role and its scope
	if got := listIDs(fresh); !slices.Equal(got, sorted(mdnVault)) {
		t.Errorf("(d) new token: list = %v, want %v", got, sorted(mdnVault))
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM vendor_request_vendor_parties WHERE vendor_request_id = $1 AND status <> 'pending'`, f.reqID); n != 0 {
		t.Fatalf("refused calls decided %d parties", n)
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
	// The request's plan (it allocates to cash2) is back to draft, with the vendor's reason (FR7.5).
	var plan int64
	var planStatus string
	var planReason *string
	var rejectedBy *int64
	if err := f.pool.QueryRow(ctx, `SELECT id, status, rejection_reason, rejected_by FROM vendor_request_vault_plans
		WHERE vendor_request_id = $1 AND status <> 'cancelled'`, f.reqID).Scan(&plan, &planStatus, &planReason, &rejectedBy); err != nil {
		t.Fatal(err)
	}
	if planStatus != "draft" || deref(planReason) != reason || deref(rejectedBy) != f.cash2User.UserID {
		t.Fatalf("plan after vault reject = %s / %v / %v, want draft + vendor reason", planStatus, planReason, rejectedBy)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM notifications WHERE recipient_user_id = $1 AND type = 'vendor_order.vault_rejected'`, f.acmUserID); n != 1 {
		t.Fatalf("ACM-USER vault-reject notifications = %d, want 1", n)
	}

	// ACM moves vault2's share to vault1, ACM-SPV approves, ATM-SPV approves: resend only what changed.
	acmUser := Actor{UserID: f.acmUserID, Role: "ACM-USER"}
	if d, err := f.plans.Get(ctx, acmUser, plan); err != nil || !d.Plan.RejectedByVendor {
		t.Fatalf("plan RejectedByVendor = %v (%v), want true (S8 label)", d != nil && d.Plan.RejectedByVendor, err)
	}
	if _, err := f.plans.SaveAllocations(ctx, acmUser, plan, []AllocationLineInput{line(f.vault1, "2000000")}, nil); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	if _, err := f.plans.Submit(ctx, acmUser, plan); err != nil {
		t.Fatalf("re-submit: %v", err)
	}
	if _, err := f.plans.Approve(ctx, Actor{UserID: f.spvID, Role: "ACM-SPV"}, plan); err != nil {
		t.Fatalf("re-approve: %v", err)
	}
	if err := f.plans.ReviewRequest(ctx, f.checker, f.reqID, true, ""); err != nil {
		t.Fatalf("vault approve: %v", err)
	}
	after := partiesOf(t, f.pool, f.reqID)
	want := map[partyKey]string{
		{f.branchID, partyRoleReplenish}:      "pending",   // accepted, but its vault list changed (V6)
		{f.replenish2(t), partyRoleReplenish}: "pending",   // vault list changed
		{cash1, partyRoleVault}:               "pending",   // vault1 amount changed
		{cash2, partyRoleVault}:               "withdrawn", // no longer used
	}
	if got := f.tickets(t, f.reqID); got[f.ticket(f.vault2, "001")] || !got[f.ticket(f.vault1, "001")] {
		t.Fatalf("tickets = %v, want vault2 inactive, vault1 active", got)
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
// and drops terminal2 -> re-approval reopens the request's plan with its
// allocations -> ACM re-checks the total -> every remaining party accepts again.
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
	// Allocations are per request: the edit keeps them (2 vaults, 2M) as ACM's starting point.
	if n := countRows(t, f.pool, `SELECT count(*) FROM vendor_request_vault_allocations WHERE vendor_request_id = $1`, f.reqID); n != 2 {
		t.Fatalf("allocations after edit = %d, want 2 (kept)", n)
	}
	if _, err := f.svc.Submit(ctx, f.creator, f.reqID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := f.svc.Approve(ctx, f.checker, f.reqID); err != nil {
		t.Fatalf("re-approve: %v", err)
	}
	var planID int64
	var planStatus string
	if err := f.pool.QueryRow(ctx, `SELECT id, status FROM vendor_request_vault_plans WHERE vendor_request_id = $1 AND status <> 'cancelled'`,
		f.reqID).Scan(&planID, &planStatus); err != nil || planStatus != "draft" {
		t.Fatalf("plan after re-approve = %s (%v), want the same plan back in draft", planStatus, err)
	}

	// Order is now 1M: the kept 2M is excess -> submit needs a buffer reason; ACM trims to vault1 1M.
	acmUser := Actor{UserID: f.acmUserID, Role: "ACM-USER"}
	var ve *ValidationError
	if _, err := f.plans.Submit(ctx, acmUser, planID); !errors.As(err, &ve) {
		t.Fatalf("submit with excess and no reason: want ValidationError, got %v", err)
	}
	if _, err := f.plans.SaveAllocations(ctx, acmUser, planID, []AllocationLineInput{line(f.vault1, "1000000")}, nil); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	if _, err := f.plans.Submit(ctx, acmUser, planID); err != nil {
		t.Fatalf("resubmit plan: %v", err)
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
	// replenishment-realisasi FR2.5: the vendor reports a vendor_sent request, not ATM-USER.
	if _, err := f.svc.SubmitCompletion(ctx, f.creator, f.reqID, results); !errors.Is(err, ErrCompletionByVendor) {
		t.Fatalf("internal completion on a vendor request: want ErrCompletionByVendor, got %v", err)
	}
	if _, err := f.orders.SubmitRealization(ctx, f.wideUser, f.party(t, f.branchID, partyRoleReplenish), realizationRows(t, f.pool, f.reqID, nil)); err != nil {
		t.Fatalf("vendor realization after vendor_accepted: %v", err)
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

// realizationRows builds a vendor report for every active ticket of the request: success, dated the
// schedule (or today if the schedule is still ahead, K1), amounts = order. ATMs in failed get no
// date/amounts.
func realizationRows(t *testing.T, pool *pgxpool.Pool, reqID int64, failed map[string]bool) []RealizationRowInput {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT o.terminal_id, LEAST(o.replenish_date, (now() AT TIME ZONE 'Asia/Jakarta')::date), o.denom, o.order_amount
		FROM (SELECT t.terminal_id, t.replenish_date, i.denom, SUM(i.amount_replenish)::bigint AS order_amount
		      FROM vendor_request_tickets t
		      JOIN vendor_requests vr ON vr.request_number = t.request_number
		      JOIN vendor_request_items i ON i.vendor_request_id = vr.id AND i.terminal_id = t.terminal_id
		      WHERE vr.id = $1 AND t.is_active
		      GROUP BY t.terminal_id, t.replenish_date, i.denom) o
		ORDER BY o.terminal_id`, reqID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	byTerminal := map[string]*RealizationRowInput{}
	var order []string
	for rows.Next() {
		var term string
		var d time.Time
		var denom int32
		var amount int64
		if err := rows.Scan(&term, &d, &denom, &amount); err != nil {
			t.Fatal(err)
		}
		r, ok := byTerminal[term]
		if !ok {
			r = &RealizationRowInput{TerminalID: term, Result: completionSuccess}
			if failed[term] {
				r.Result = completionFailed
			} else {
				r.RealizedDate, r.Amounts = &d, map[int32]int64{}
			}
			byTerminal[term] = r
			order = append(order, term)
		}
		if r.Amounts != nil {
			r.Amounts[denom] = amount
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	out := make([]RealizationRowInput, len(order))
	for i, term := range order {
		out[i] = *byTerminal[term]
	}
	return out
}

// replenishment-realisasi FR2: vendor realisasi report -> SPV reject -> resubmit -> approve.
func TestIntegration_VendorOrder_Realization(t *testing.T) {
	f := newVendorOrderFixture(t)
	ctx := context.Background()
	// Approve records visits; drop them before the fixture deletes the request and ATMs (LIFO).
	t.Cleanup(func() {
		c := context.Background()
		atms := `SELECT id FROM atms WHERE terminal_id = ANY($1)`
		_, _ = f.pool.Exec(c, `DELETE FROM atm_visits WHERE vendor_request_id = $1`, f.reqID)
		_, _ = f.pool.Exec(c, `DELETE FROM atm_visit_quotas WHERE atm_id IN (`+atms+`)`, []string{f.terminal, f.terminal2})
	})
	replenish1 := f.party(t, f.branchID, partyRoleReplenish)
	if _, err := f.orders.SubmitRealization(ctx, f.wideUser, replenish1, realizationRows(t, f.pool, f.reqID, nil)); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("report before vendor_accepted: want ErrInvalidTransition, got %v", err)
	}
	for _, p := range []int64{replenish1, f.party(t, f.replenish2(t), partyRoleReplenish), f.party(t, f.cash1, partyRoleVault)} {
		if _, err := f.orders.Accept(ctx, f.wideUser, p); err != nil {
			t.Fatal(err)
		}
	}
	cash2Vault := f.party(t, f.cash2, partyRoleVault)
	if _, err := f.orders.Accept(ctx, f.cash2User, cash2Vault); err != nil {
		t.Fatal(err)
	}
	rows := realizationRows(t, f.pool, f.reqID, nil)

	// vault party never reports; out-of-scope party is 404; a branch-pinned user cannot cover a
	// two-branch request; a typo is a ValidationError.
	if _, err := f.orders.SubmitRealization(ctx, f.cash2User, cash2Vault, rows); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("vault party report: want ErrInvalidTransition, got %v", err)
	}
	if _, err := f.orders.SubmitRealization(ctx, f.cash2User, replenish1, rows); !errors.Is(err, ErrVendorOrderNotFound) {
		t.Fatalf("out of scope report: want ErrVendorOrderNotFound, got %v", err)
	}
	var ve *ValidationError
	if _, err := f.orders.SubmitRealization(ctx, f.branchUser, replenish1, rows); !errors.As(err, &ve) {
		t.Fatalf("branch user on a two-branch request: want ValidationError, got %v", err)
	}
	// review R1: a vendor-wide user of another vendor holding a replenish party of the same request
	// may not report the ATMs vendor 1 manages; a withdrawn party may not report at all.
	if f.vendor2 == f.vendorID {
		t.Fatal("fixture: cash2 must belong to another vendor")
	}
	other := f.vendorUser(t, f.vendor2, nil)
	var otherParty int64
	if err := f.pool.QueryRow(ctx, `INSERT INTO vendor_request_vendor_parties
		(vendor_request_id, vendor_branch_id, role, status, content, decided_by, decided_at)
		SELECT vendor_request_id, $2, 'replenish', 'accepted', content, $3, now() FROM vendor_request_vendor_parties WHERE id = $1
		RETURNING id`, replenish1, f.cash2, other.UserID).Scan(&otherParty); err != nil {
		t.Fatal(err)
	}
	if _, err := f.orders.SubmitRealization(ctx, other, otherParty, rows); !errors.As(err, &ve) {
		t.Fatalf("other vendor's user on a request it does not fully manage: want ValidationError, got %v", err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE vendor_request_vendor_parties SET status = 'withdrawn', decided_by = NULL, decided_at = NULL
		WHERE id = $1`, otherParty); err != nil {
		t.Fatal(err)
	}
	if _, err := f.orders.SubmitRealization(ctx, other, otherParty, rows); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("withdrawn party report: want ErrInvalidTransition, got %v", err)
	}
	if _, err := f.orders.SubmitRealization(ctx, f.wideUser, replenish1, rows[:1]); !errors.As(err, &ve) {
		t.Fatalf("missing ATM: want ValidationError, got %v", err)
	}

	d, err := f.orders.SubmitRealization(ctx, f.wideUser, replenish1, rows)
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if d.RequestStatus != "completion_pending" || d.CanReport || len(d.Realizations) != len(rows) {
		t.Fatalf("after report = status %s can_report %v realizations %d", d.RequestStatus, d.CanReport, len(d.Realizations))
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM vendor_requests WHERE id = $1 AND completion_submitted_by = $2`, f.reqID, f.wideUser.UserID); n != 1 {
		t.Fatal("maker of the report is not the vendor user")
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM audit_logs WHERE entity_type = 'vendor_request' AND entity_id = $1
		AND action = 'submit_completion' AND actor_id = $2`, f.reqID, f.wideUser.UserID); n != 1 {
		t.Fatal("report not audited")
	}

	// reject -> vendor notified, back to vendor_accepted; resubmit with one ATM failed drops its amounts.
	if _, err := f.svc.RejectCompletion(ctx, f.checker, f.reqID, "nominal salah"); err != nil {
		t.Fatalf("reject report: %v", err)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM notifications WHERE recipient_user_id = $1 AND type = 'vendor_order.realization_rejected'`, f.wideUser.UserID); n != 1 {
		t.Fatalf("vendor reject notifications = %d, want 1", n)
	}
	failedTerm := rows[0].TerminalID
	if _, err := f.orders.SubmitRealization(ctx, f.wideUser, replenish1, realizationRows(t, f.pool, f.reqID, map[string]bool{failedTerm: true})); err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM vendor_request_realizations r JOIN vendor_request_tickets t ON t.id = r.ticket_id
		WHERE r.vendor_request_id = $1 AND t.terminal_id = $2 AND r.realized_date IS NULL
		AND NOT EXISTS (SELECT 1 FROM vendor_request_realization_amounts a WHERE a.realization_id = r.id)`, f.reqID, failedTerm); n != 1 {
		t.Fatal("failed ATM still carries a date or amounts after resubmit")
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM vendor_request_realizations WHERE vendor_request_id = $1`, f.reqID); n != len(rows) {
		t.Fatalf("realizations = %d, want one per ATM (%d)", n, len(rows))
	}

	if _, _, err := f.svc.ApproveCompletion(ctx, f.checker, f.reqID); err != nil {
		t.Fatalf("approve report: %v", err)
	}
	if s := f.requestStatus(t, f.reqID); s != "completed" {
		t.Fatalf("after approve = %s, want completed", s)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM atm_visits WHERE vendor_request_id = $1 AND cancelled_at IS NULL`, f.reqID); n != len(rows)-1 {
		t.Fatalf("visits = %d, want %d (failed ATM not counted)", n, len(rows)-1)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM notifications WHERE recipient_user_id = $1 AND type = 'vendor_order.realization_approved'`, f.wideUser.UserID); n != 1 {
		t.Fatalf("vendor approve notifications = %d, want 1", n)
	}
	detail, err := f.svc.Get(ctx, f.reqID)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range detail.Atms {
		if a.TerminalID == failedTerm && (a.RealizedDate != nil || len(a.RealizedAmounts) != 0) {
			t.Fatalf("failed ATM on internal detail = %+v", a)
		}
		if a.TerminalID != failedTerm && (a.RealizedDate == nil || len(a.RealizedAmounts) == 0) {
			t.Fatalf("reported ATM missing realisasi on internal detail = %+v", a)
		}
	}
}

// cit-send-vendor FR6.4: cancel skips withdrawn parties -- no cancelled event,
// no notice to a vendor that no longer has a part in the request.
func TestIntegration_VendorOrder_CancelSkipsWithdrawnParty(t *testing.T) {
	f := newVendorOrderFixture(t)
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `UPDATE vendor_request_vendor_parties SET status = 'withdrawn' WHERE id = $1`, f.party(t, f.cash2, partyRoleVault)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Cancel(ctx, f.checker, f.reqID, "ATM ditutup"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM vendor_request_vendor_party_events e JOIN vendor_request_vendor_parties p ON p.id = e.party_id
		WHERE p.vendor_request_id = $1 AND e.event = 'cancelled'`, f.reqID); n != 3 {
		t.Fatalf("cancelled events = %d, want 3 (withdrawn party skipped)", n)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM notifications WHERE recipient_user_id = $1 AND type = 'vendor_order.cancelled'`, f.cash2User.UserID); n != 0 {
		t.Fatalf("withdrawn vendor cancel notifications = %d, want 0", n)
	}
}
