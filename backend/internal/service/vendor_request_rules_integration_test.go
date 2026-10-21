//go:build integration

package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/vendorrole"
)

// be-tech-debt C4.2: vendor_request transition / completion / kuota guards not
// covered by the existing reject, cancel, four-eyes and visit-quota tests.

func TestIntegration_VendorRequestTransition_Guards(t *testing.T) {
	pool, vendorID, _, terminalID := setupNumberGeneratorHarness(t)
	ids := fetchUserIDs(t, pool, 2)
	svc := NewVendorRequestService(pool)
	ctx := context.Background()
	creator := Actor{UserID: ids[0], Role: "ADMIN"} // maker and checker role at once
	other := Actor{UserID: ids[1], Role: "ATM-USER"}
	checker := Actor{UserID: ids[1], Role: "ATM-SPV"}
	status := func(id int64) string {
		t.Helper()
		var s string
		if err := pool.QueryRow(ctx, `SELECT status FROM vendor_requests WHERE id = $1`, id).Scan(&s); err != nil {
			t.Fatalf("read status: %v", err)
		}
		return s
	}

	t.Run("unknown request is ErrNotFound for every transition", func(t *testing.T) {
		const missing = int64(9_000_000_000)
		calls := map[string]func() error{
			"submit":  func() error { _, err := svc.Submit(ctx, creator, missing); return err },
			"approve": func() error { _, err := svc.Approve(ctx, checker, missing); return err },
			"reject":  func() error { _, err := svc.Reject(ctx, checker, missing, "alasan yang cukup panjang"); return err },
		}
		for name, call := range calls {
			if err := call(); !errors.Is(err, ErrNotFound) {
				t.Errorf("%s: err = %v, want ErrNotFound", name, err)
			}
		}
	})

	t.Run("state guard runs before the actor guard", func(t *testing.T) {
		created, err := svc.Create(ctx, creator, manualCreateInput(vendorID, terminalID, 50000))
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		// On a draft, approve by the creator is an invalid transition, not
		// ErrSelfApproval: the state is checked first (design.md order).
		if _, err := svc.Approve(ctx, creator, created.ID); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("approve draft by creator: err = %v, want ErrInvalidTransition", err)
		}
		if _, err := svc.Reject(ctx, checker, created.ID, "alasan yang cukup panjang"); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("reject draft: err = %v, want ErrInvalidTransition", err)
		}
		if got := countAuditEntries(t, pool, created.ID, "approve") + countAuditEntries(t, pool, created.ID, "reject"); got != 0 {
			t.Errorf("approve/reject audit rows = %d, want 0", got)
		}
		if s := status(created.ID); s != "draft" {
			t.Errorf("status = %q, want draft", s)
		}
	})

	t.Run("only the creator submits", func(t *testing.T) {
		created, err := svc.Create(ctx, creator, manualCreateInput(vendorID, terminalID, 100000))
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := svc.Submit(ctx, other, created.ID); !errors.Is(err, ErrNotCreator) {
			t.Fatalf("submit by non-creator: err = %v, want ErrNotCreator", err)
		}
		if s := status(created.ID); s != "draft" {
			t.Errorf("status = %q, want draft", s)
		}
	})

	t.Run("submit without items is refused", func(t *testing.T) {
		created, err := svc.Create(ctx, creator, manualCreateInput(vendorID, terminalID, 50000))
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM vendor_request_items WHERE vendor_request_id = $1`, created.ID); err != nil {
			t.Fatalf("empty the request: %v", err)
		}
		if _, err := svc.Submit(ctx, creator, created.ID); !errors.Is(err, ErrEmptyItems) {
			t.Fatalf("submit empty: err = %v, want ErrEmptyItems", err)
		}
		if s := status(created.ID); s != "draft" {
			t.Errorf("status = %q, want draft", s)
		}
	})

	t.Run("a non-checker role cannot approve", func(t *testing.T) {
		created, err := svc.Create(ctx, creator, manualCreateInput(vendorID, terminalID, 100000))
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := svc.Submit(ctx, creator, created.ID); err != nil {
			t.Fatalf("submit: %v", err)
		}
		if _, err := svc.Approve(ctx, other, created.ID); !errors.Is(err, ErrNotChecker) {
			t.Fatalf("approve by ATM-USER: err = %v, want ErrNotChecker", err)
		}
		if s := status(created.ID); s != "pending_approval" {
			t.Errorf("status = %q, want pending_approval", s)
		}
	})
}

func TestIntegration_Completion_Guards(t *testing.T) {
	f := setupQuotaFixture(t)
	ctx := context.Background()
	term := f.seedPackageATM(t, "G", "PAKET 5")
	results := []CompletionResultInput{{TerminalID: term, Result: completionSuccess}}

	t.Run("unknown request", func(t *testing.T) {
		if _, err := f.vr.SubmitCompletion(ctx, f.maker, 9_000_000_000, results); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("cancelled request cannot be reported", func(t *testing.T) {
		id := f.approvedRequest(t, term)
		if _, err := f.tx.Exec(ctx, `UPDATE vendor_requests SET is_canceled = true WHERE id = $1`, id); err != nil {
			t.Fatalf("cancel: %v", err)
		}
		if _, err := f.vr.SubmitCompletion(ctx, f.maker, id, results); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("err = %v, want ErrInvalidTransition", err)
		}
	})

	t.Run("vendor user is refused on the internal path", func(t *testing.T) {
		id := f.approvedRequest(t, term)
		for _, role := range vendorrole.All { // vendor-party-scope: every vendor role, not only VENDOR-USER
			vendor := Actor{UserID: f.maker.UserID, Role: role}
			if _, err := f.vr.SubmitCompletion(ctx, vendor, id, results); !errors.Is(err, ErrNotAuthorized) {
				t.Fatalf("%s: err = %v, want ErrNotAuthorized", role, err)
			}
		}
		var s string
		if err := f.tx.QueryRow(ctx, `SELECT status FROM vendor_requests WHERE id = $1`, id).Scan(&s); err != nil || s != "approved" {
			t.Fatalf("status = %q err = %v, want approved", s, err)
		}
	})

	t.Run("reject reason over 500 chars", func(t *testing.T) {
		var verr *ValidationError
		if _, err := f.vr.RejectCompletion(ctx, f.checker, 1, strings.Repeat("x", 501)); !errors.As(err, &verr) || verr.Field != "reason" {
			t.Fatalf("err = %v, want ValidationError on reason", err)
		}
	})
}

func TestIntegration_VisitQuota_ServiceRBACAndEdges(t *testing.T) {
	f := setupQuotaFixture(t)
	ctx := context.Background()

	t.Run("non-checker roles cannot reset or cancel", func(t *testing.T) {
		term := f.seedPackageATM(t, "RB", "PAKET 5")
		for _, role := range []string{"ATM-USER", vendorrole.User, ""} {
			a := Actor{UserID: f.maker.UserID, Role: role}
			if _, err := f.quota.ResetATM(ctx, a, term); !errors.Is(err, ErrNotChecker) {
				t.Errorf("ResetATM as %q: err = %v, want ErrNotChecker", role, err)
			}
			if _, err := f.quota.ResetVendor(ctx, a, f.vendorID); !errors.Is(err, ErrNotChecker) {
				t.Errorf("ResetVendor as %q: err = %v, want ErrNotChecker", role, err)
			}
			if err := f.quota.CancelVisit(ctx, a, 1, "alasan"); !errors.Is(err, ErrNotChecker) {
				t.Errorf("CancelVisit as %q: err = %v, want ErrNotChecker", role, err)
			}
		}
	})

	t.Run("reset of an unknown ATM", func(t *testing.T) {
		if _, err := f.quota.ResetATM(ctx, f.checker, "NO-SUCH-"+f.marker); !errors.Is(err, ErrAtmNotFound) {
			t.Fatalf("err = %v, want ErrAtmNotFound", err)
		}
	})

	t.Run("cancel visit: unknown id and long reason", func(t *testing.T) {
		if err := f.quota.CancelVisit(ctx, f.checker, 9_000_000_000, "alasan"); !errors.Is(err, ErrVisitNotFound) {
			t.Errorf("unknown visit: err = %v, want ErrVisitNotFound", err)
		}
		var verr *ValidationError
		if err := f.quota.CancelVisit(ctx, f.checker, 1, strings.Repeat("x", 501)); !errors.As(err, &verr) {
			t.Errorf("long reason: err = %v, want ValidationError", err)
		}
	})

	t.Run("visit on an ATM without a known quota is counted but touches no counter", func(t *testing.T) {
		// Package label with no package_frequencies row -> cr_frequency unknown.
		term := f.seedPackageATM(t, "UK", "PAKET TANPA FREKUENSI")
		id := f.approvedRequest(t, term)
		if over := f.complete(t, id, map[string]string{term: completionSuccess}); len(over) != 0 {
			t.Fatalf("over quota = %v, want none", over)
		}
		v, err := f.quota.Get(ctx, term)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if v.HasQuota || len(v.Visits) != 1 || v.Visits[0].QuotaKnown {
			t.Fatalf("view = has_quota %v visits %+v, want no quota row and one quota_known=false visit", v.HasQuota, v.Visits)
		}
		if err := f.quota.CancelVisit(ctx, f.checker, v.Visits[0].ID, "salah input"); err != nil {
			t.Fatalf("cancel unknown-quota visit: %v", err)
		}
		if v, _ = f.quota.Get(ctx, term); v.HasQuota || v.Visits[0].CancelledAt == nil {
			t.Fatalf("after cancel: has_quota %v cancelled_at %v, want no quota row and cancelled", v.HasQuota, v.Visits[0].CancelledAt)
		}
	})
}
