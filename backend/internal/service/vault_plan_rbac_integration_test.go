//go:build integration

package service

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// be-tech-debt C4.3: vault plan role / area-scope / state guards on every
// mutating step (SaveAllocations, Submit, Approve, Reject, ReviewRequest),
// beyond the non-member Get, ADMIN save and self-approval cases already in
// vault_plan_integration_test.go.
func TestIntegration_VaultPlan_RoleScopeAndStateGuards(t *testing.T) {
	f := newVaultPlanFixture(t)
	ctx := context.Background()
	acmUser := Actor{UserID: f.acmUserID, Role: "ACM-USER"}
	acmSpv := Actor{UserID: f.spvID, Role: "ACM-SPV"}
	// Right role names, but not members of the plan's ACM area.
	outsiderUser := Actor{UserID: f.checker.UserID, Role: "ACM-USER"}
	outsiderSpv := Actor{UserID: f.checker.UserID, Role: "ACM-SPV"}
	reqID, plan := f.approvedTwoAtms(t)
	alloc := []AllocationLineInput{line(f.vault1, "2000000")}

	t.Run("unknown plan", func(t *testing.T) {
		const missing = int64(9_000_000_000)
		if _, err := f.plans.Submit(ctx, acmUser, missing); !errors.Is(err, ErrVaultPlanNotFound) {
			t.Errorf("submit: err = %v, want ErrVaultPlanNotFound", err)
		}
		if _, err := f.plans.Approve(ctx, acmSpv, missing); !errors.Is(err, ErrVaultPlanNotFound) {
			t.Errorf("approve: err = %v, want ErrVaultPlanNotFound", err)
		}
	})

	t.Run("wrong role per step", func(t *testing.T) {
		admin := Actor{UserID: f.creator.UserID, Role: "ADMIN"}
		cases := map[string]error{}
		_, cases["ACM-SPV saves"] = f.plans.SaveAllocations(ctx, acmSpv, plan, alloc, nil)
		_, cases["ACM-SPV submits"] = f.plans.Submit(ctx, acmSpv, plan)
		_, cases["ACM-USER approves"] = f.plans.Approve(ctx, acmUser, plan)
		_, cases["ACM-USER rejects"] = f.plans.Reject(ctx, acmUser, plan, "alasan penolakan")
		_, cases["ADMIN approves"] = f.plans.Approve(ctx, admin, plan)
		_, cases["ATM-SPV approves"] = f.plans.Approve(ctx, f.checker, plan)
		for name, err := range cases {
			if !errors.Is(err, ErrNotAuthorized) {
				t.Errorf("%s: err = %v, want ErrNotAuthorized", name, err)
			}
		}
	})

	t.Run("non-member of the area cannot change the plan", func(t *testing.T) {
		if _, err := f.plans.SaveAllocations(ctx, outsiderUser, plan, alloc, nil); !errors.Is(err, ErrVaultPlanNotFound) {
			t.Errorf("save: err = %v, want ErrVaultPlanNotFound", err)
		}
		if _, err := f.plans.Submit(ctx, outsiderUser, plan); !errors.Is(err, ErrVaultPlanNotFound) {
			t.Errorf("submit: err = %v, want ErrVaultPlanNotFound", err)
		}
	})

	t.Run("approving a draft is an invalid transition", func(t *testing.T) {
		if _, err := f.plans.Approve(ctx, acmSpv, plan); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("err = %v, want ErrInvalidTransition", err)
		}
	})

	t.Run("review before the ACM approval", func(t *testing.T) {
		if err := f.plans.ReviewRequest(ctx, f.checker, reqID, true, ""); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("review in vault_assignment: err = %v, want ErrInvalidTransition", err)
		}
	})

	if _, err := f.plans.SaveAllocations(ctx, acmUser, plan, alloc, nil); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := f.plans.Submit(ctx, acmUser, plan); err != nil {
		t.Fatalf("submit: %v", err)
	}

	t.Run("non-member checker cannot approve or reject a pending plan", func(t *testing.T) {
		if _, err := f.plans.Approve(ctx, outsiderSpv, plan); !errors.Is(err, ErrVaultPlanNotFound) {
			t.Errorf("approve: err = %v, want ErrVaultPlanNotFound", err)
		}
		if _, err := f.plans.Reject(ctx, outsiderSpv, plan, "alasan penolakan"); !errors.Is(err, ErrVaultPlanNotFound) {
			t.Errorf("reject: err = %v, want ErrVaultPlanNotFound", err)
		}
		if s := f.requestStatus(t, reqID); s != "vault_assignment" {
			t.Errorf("request status = %q, want vault_assignment", s)
		}
	})

	if _, err := f.plans.Approve(ctx, acmSpv, plan); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if s := f.requestStatus(t, reqID); s != "vault_review" {
		t.Fatalf("request status = %q, want vault_review", s)
	}

	t.Run("review guards", func(t *testing.T) {
		if err := f.plans.ReviewRequest(ctx, acmSpv, reqID, true, ""); !errors.Is(err, ErrNotChecker) {
			t.Errorf("ACM-SPV reviews: err = %v, want ErrNotChecker", err)
		}
		if err := f.plans.ReviewRequest(ctx, f.checker, 9_000_000_000, true, ""); !errors.Is(err, ErrNotFound) {
			t.Errorf("unknown request: err = %v, want ErrNotFound", err)
		}
		if err := f.plans.ReviewRequest(ctx, f.checker, reqID, false, "  "); !errors.Is(err, ErrRejectReasonEmpty) {
			t.Errorf("reject without reason: err = %v, want ErrRejectReasonEmpty", err)
		}
		var ve *ValidationError
		if err := f.plans.ReviewRequest(ctx, f.checker, reqID, false, strings.Repeat("x", 501)); !errors.As(err, &ve) {
			t.Errorf("reject reason too long: err = %v, want ValidationError", err)
		}
		if s := f.requestStatus(t, reqID); s != "vault_review" {
			t.Errorf("request status = %q, want vault_review (nothing applied)", s)
		}
	})

	t.Run("cancelled request cannot be reviewed", func(t *testing.T) {
		if _, err := f.pool.Exec(ctx, `UPDATE vendor_requests SET is_canceled = true WHERE id = $1`, reqID); err != nil {
			t.Fatalf("cancel: %v", err)
		}
		if err := f.plans.ReviewRequest(ctx, f.checker, reqID, true, ""); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("err = %v, want ErrInvalidTransition", err)
		}
	})
}
