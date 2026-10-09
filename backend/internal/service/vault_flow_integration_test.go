//go:build integration

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/notification"
)

// cit-acm-plan S5 against real Postgres: approve hands the request to ACM
// (vault_assignment + one plan per area + ACM-USER notification), cancel
// cascades to plans, FR7.3 branch assignment creates plans late, and laporan
// selesai works from ready and returns to ready (legacy approved is covered
// by atm_visit_quota_integration_test.go).

type vaultFlowFixture struct {
	pool      *pgxpool.Pool
	svc       *VendorRequestService
	vendorID  int64
	branchID  int64
	terminal  string
	creator   Actor
	checker   Actor
	acmUserID int64
}

func newVaultFlowFixture(t *testing.T) *vaultFlowFixture {
	t.Helper()
	pool, vendorID, _, terminal := setupNumberGeneratorHarness(t)
	ids := fetchUserIDs(t, pool, 2)
	ctx := context.Background()
	f := &vaultFlowFixture{
		pool: pool, vendorID: vendorID, terminal: terminal,
		svc:     NewVendorRequestService(pool).WithNotifier(notification.NewService(false)),
		creator: Actor{UserID: ids[0], Role: "ATM-USER"},
		checker: Actor{UserID: ids[1], Role: "ATM-SPV"},
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM vendor_branches WHERE vendor_id = $1`, vendorID).Scan(&f.branchID); err != nil {
		t.Fatalf("load branch: %v", err)
	}
	name := "itacm-" + uuid.NewString()[:8]
	if err := pool.QueryRow(ctx, `INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source)
		SELECT id, $1, $1, $1 || '@test.local', true, 'ldap' FROM roles WHERE role = 'ACM-USER' RETURNING id`, name).Scan(&f.acmUserID); err != nil {
		t.Fatalf("seed ACM-USER: %v", err)
	}
	// Registered after the harness, so this runs first (LIFO): plans/areas
	// before the harness deletes requests and branches.
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM vendor_request_vault_plans WHERE vendor_request_id IN (SELECT id FROM vendor_requests WHERE vendor_id = $1)`, vendorID)
		_, _ = pool.Exec(c, `DELETE FROM acm_area_members WHERE user_id = $1`, f.acmUserID)
		_, _ = pool.Exec(c, `DELETE FROM acm_area_branches WHERE vendor_branch_id = $1`, f.branchID)
		_, _ = pool.Exec(c, `DELETE FROM acm_areas WHERE name LIKE 'ITVF-%' AND created_by = $1`, f.creator.UserID)
		_, _ = pool.Exec(c, `DELETE FROM notification_emails WHERE notification_id IN (SELECT id FROM notifications WHERE recipient_user_id = $1)`, f.acmUserID)
		_, _ = pool.Exec(c, `DELETE FROM notifications WHERE recipient_user_id = $1`, f.acmUserID)
		// audit_logs (append-only) may reference the user as actor; then the delete
		// fails and the user must at least not stay an active ACM member candidate.
		_, _ = pool.Exec(c, `UPDATE users SET is_active = false, deleted_at = now() WHERE id = $1`, f.acmUserID)
		_, _ = pool.Exec(c, `DELETE FROM users WHERE id = $1`, f.acmUserID)
	})
	return f
}

// area creates an active ACM area holding the fixture branch (optional) with the ACM-USER as member.
func (f *vaultFlowFixture) area(t *testing.T, withBranch bool) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	if err := f.pool.QueryRow(ctx, `INSERT INTO acm_areas (name, created_by) VALUES ($1, $2) RETURNING id`,
		"ITVF-"+uuid.NewString()[:8], f.creator.UserID).Scan(&id); err != nil {
		t.Fatalf("insert area: %v", err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO acm_area_members (acm_area_id, user_id, created_by) VALUES ($1, $2, $3)`, id, f.acmUserID, f.creator.UserID); err != nil {
		t.Fatalf("insert member: %v", err)
	}
	if withBranch {
		if _, err := f.pool.Exec(ctx, `INSERT INTO acm_area_branches (acm_area_id, vendor_branch_id, created_by) VALUES ($1, $2, $3)`, id, f.branchID, f.creator.UserID); err != nil {
			t.Fatalf("insert area branch: %v", err)
		}
	}
	return id
}

// approved creates, submits and ATM-SPV-approves a request.
func (f *vaultFlowFixture) approved(t *testing.T) *VendorRequestDetail {
	t.Helper()
	ctx := context.Background()
	created, err := f.svc.Create(ctx, f.creator, manualCreateInput(f.vendorID, f.terminal, 50000))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := f.svc.Submit(ctx, f.creator, created.ID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	d, err := f.svc.Approve(ctx, f.checker, created.ID)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	return d
}

func (f *vaultFlowFixture) plans(t *testing.T, requestID int64) map[int64]string {
	t.Helper()
	rows, err := f.pool.Query(context.Background(), `SELECT acm_area_id, status FROM vendor_request_vault_plans WHERE vendor_request_id = $1`, requestID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var area int64
		var st string
		if err := rows.Scan(&area, &st); err != nil {
			t.Fatal(err)
		}
		out[area] = st
	}
	return out
}

func (f *vaultFlowFixture) notificationCount(t *testing.T) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM notifications WHERE recipient_user_id = $1 AND type = 'vault_plan.assignment_needed'`, f.acmUserID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestIntegration_VaultFlow_ApproveCreatesPlanAndCancelCascades(t *testing.T) {
	f := newVaultFlowFixture(t)
	ctx := context.Background()
	areaID := f.area(t, true)

	d := f.approved(t)
	if d.Status != "vault_assignment" {
		t.Fatalf("status after approve = %q, want vault_assignment", d.Status)
	}
	var vaultFlow bool
	var approvedBy *int64
	if err := f.pool.QueryRow(ctx, `SELECT vault_flow, approved_by FROM vendor_requests WHERE id = $1`, d.ID).Scan(&vaultFlow, &approvedBy); err != nil {
		t.Fatal(err)
	}
	if !vaultFlow || approvedBy == nil || *approvedBy != f.checker.UserID {
		t.Fatalf("vault_flow=%v approved_by=%v, want true/%d", vaultFlow, approvedBy, f.checker.UserID)
	}
	if got := f.plans(t, d.ID); len(got) != 1 || got[areaID] != "draft" {
		t.Fatalf("plans = %v, want one draft plan for area %d", got, areaID)
	}
	if n := f.notificationCount(t); n != 1 {
		t.Fatalf("ACM-USER notifications = %d, want 1", n)
	}

	// Laporan selesai is not possible before the ACM/ATM review is done.
	if _, err := f.svc.SubmitCompletion(ctx, f.creator, d.ID, []CompletionResultInput{{TerminalID: f.terminal, Result: "success"}}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("completion from vault_assignment: want ErrInvalidTransition, got %v", err)
	}

	// Cancel (checker) cascades to the plan.
	if _, err := f.svc.Cancel(ctx, f.checker, d.ID, "batal uji"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if got := f.plans(t, d.ID); got[areaID] != "cancelled" {
		t.Fatalf("plan after cancel = %v, want cancelled", got)
	}
}

func TestIntegration_VaultFlow_BranchWithoutAreaGetsPlanOnAssignment(t *testing.T) {
	f := newVaultFlowFixture(t)
	ctx := context.Background()
	areaID := f.area(t, false) // branch not yet in any area

	d := f.approved(t)
	if got := f.plans(t, d.ID); len(got) != 0 {
		t.Fatalf("plans = %v, want none while the branch has no area", got)
	}
	acm := NewAcmAreaAdminService(f.pool, f.pool).WithNotifier(notification.NewService(false))
	warnings, err := acm.Warnings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range warnings {
		found = found || w.VendorBranchID == f.branchID
	}
	if !found {
		t.Fatalf("branch %d missing from FR7.4 warnings %v", f.branchID, warnings)
	}

	// FR7.3: ADMIN assigns the branch -> the waiting request gets its plan.
	adminID := fetchUserIDs(t, f.pool, 1)[0]
	if _, err := acm.SetBranches(ctx, adminID, "ADMIN", areaID, []int64{f.branchID}, ""); err != nil {
		t.Fatalf("SetBranches: %v", err)
	}
	if got := f.plans(t, d.ID); got[areaID] != "draft" {
		t.Fatalf("plans after assignment = %v, want draft for area %d", got, areaID)
	}
	if n := f.notificationCount(t); n != 1 {
		t.Fatalf("ACM-USER notifications = %d, want 1", n)
	}
}

func TestIntegration_VaultFlow_CompletionFromReadyReturnsToReady(t *testing.T) {
	f := newVaultFlowFixture(t)
	ctx := context.Background()
	d := f.approved(t)
	// vault_review -> ready is S6 (ATM-SPV vault-approve); jump there directly.
	if _, err := f.pool.Exec(ctx, `UPDATE vendor_requests SET status = 'ready' WHERE id = $1`, d.ID); err != nil {
		t.Fatal(err)
	}

	results := []CompletionResultInput{{TerminalID: f.terminal, Result: "success"}}
	if got, err := f.svc.SubmitCompletion(ctx, f.creator, d.ID, results); err != nil || got.Status != "completion_pending" {
		t.Fatalf("submit completion from ready: %v %v", got, err)
	}
	got, err := f.svc.RejectCompletion(ctx, f.checker, d.ID, "foto kurang")
	if err != nil {
		t.Fatalf("reject completion: %v", err)
	}
	if got.Status != "ready" || got.CompletionRejectionReason == nil {
		t.Fatalf("after reject: status=%q reason=%v, want ready + reason (vault_flow request)", got.Status, got.CompletionRejectionReason)
	}
}
