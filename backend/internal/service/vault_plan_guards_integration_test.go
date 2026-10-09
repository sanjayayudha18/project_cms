//go:build integration

package service

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// cit-acm-plan FR4.3–4.5 guard rails the happy-path flow tests don't reach:
// per-input validation of SaveAssignments, state checks, and four-eyes on Reject.
func TestIntegration_VaultPlan_Guards(t *testing.T) {
	f := newVaultPlanFixture(t)
	ctx := context.Background()
	acmUser := Actor{UserID: f.acmUserID, Role: "ACM-USER"}
	acmSpv := Actor{UserID: f.spvID, Role: "ACM-SPV"}
	_, plan1, _ := f.approvedTwoAtms(t)

	short := "terlalu"
	long := strings.Repeat("x", 501)
	invalid := []struct {
		name string
		in   []VaultAssignmentInput
		want string
	}{
		{"duplicate atm", []VaultAssignmentInput{{TerminalID: f.terminal, VaultBranchID: f.cash1}, {TerminalID: f.terminal, VaultBranchID: f.cash1}}, "lebih dari sekali"},
		{"unknown vault branch", []VaultAssignmentInput{{TerminalID: f.terminal, VaultBranchID: -1}}, "tidak aktif"},
		{"urgent reason too short", []VaultAssignmentInput{{TerminalID: f.terminal, VaultBranchID: f.cash3, IsUrgent: true, UrgentReason: &short}}, "10-500"},
		{"urgent reason too long", []VaultAssignmentInput{{TerminalID: f.terminal, VaultBranchID: f.cash3, IsUrgent: true, UrgentReason: &long}}, "10-500"},
		{"urgent without reason", []VaultAssignmentInput{{TerminalID: f.terminal, VaultBranchID: f.cash3, IsUrgent: true}}, "10-500"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			var ve *ValidationError
			_, err := f.plans.SaveAssignments(ctx, acmUser, plan1, tc.in)
			if !errors.As(err, &ve) || !strings.Contains(ve.Message, tc.want) {
				t.Fatalf("want ValidationError containing %q, got %v", tc.want, err)
			}
		})
	}

	// A draft plan cannot be rejected; an empty reason is refused before any lookup.
	if _, err := f.plans.Reject(ctx, acmSpv, plan1, "cek kapasitas"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("reject draft: want ErrInvalidTransition, got %v", err)
	}
	if _, err := f.plans.Reject(ctx, acmSpv, plan1, "  "); !errors.Is(err, ErrRejectReasonEmpty) {
		t.Fatalf("reject without reason: want ErrRejectReasonEmpty, got %v", err)
	}

	if _, err := f.plans.SaveAssignments(ctx, acmUser, plan1, []VaultAssignmentInput{{TerminalID: f.terminal, VaultBranchID: f.cash1}}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := f.plans.Submit(ctx, acmUser, plan1); err != nil {
		t.Fatalf("submit: %v", err)
	}

	// Pending: no more edits or re-submits; the maker cannot reject their own plan.
	if _, err := f.plans.SaveAssignments(ctx, acmUser, plan1, []VaultAssignmentInput{{TerminalID: f.terminal, VaultBranchID: f.cash1}}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("save pending: want ErrInvalidTransition, got %v", err)
	}
	if _, err := f.plans.Submit(ctx, acmUser, plan1); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("resubmit pending: want ErrInvalidTransition, got %v", err)
	}
	if _, err := f.plans.Reject(ctx, Actor{UserID: f.acmUserID, Role: "ACM-SPV"}, plan1, "cek kapasitas"); !errors.Is(err, ErrSelfApproval) {
		t.Fatalf("maker rejects own plan: want ErrSelfApproval, got %v", err)
	}
	if d, err := f.plans.Reject(ctx, acmSpv, plan1, "cek kapasitas"); err != nil || d.Plan.Status != planDraft {
		t.Fatalf("reject pending: %+v %v", d, err)
	}
}
