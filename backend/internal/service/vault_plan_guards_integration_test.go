//go:build integration

package service

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// cit-vault-select FR4.2/FR4.5 guard rails the happy-path flow tests don't
// reach: per-row validation of SaveAllocations against real data, state
// checks, and four-eyes on Reject.
func TestIntegration_VaultPlan_Guards(t *testing.T) {
	f := newVaultPlanFixture(t)
	ctx := context.Background()
	acmUser := Actor{UserID: f.acmUserID, Role: "ACM-USER"}
	acmSpv := Actor{UserID: f.spvID, Role: "ACM-SPV"}
	_, plan := f.approvedTwoAtms(t)

	short := "terlalu"
	long := strings.Repeat("x", 501)
	urgent := func(reason *string) AllocationLineInput {
		l := line(f.vault3, "2000000")
		l.IsUrgent, l.UrgentReason = true, reason
		return l
	}
	invalid := []struct {
		name   string
		in     []AllocationLineInput
		buffer *string
		want   string
	}{
		{"duplicate vault", []AllocationLineInput{line(f.vault1, "1000000"), line(f.vault1, "1000000")}, nil, "lebih dari sekali"},
		{"unknown vault", []AllocationLineInput{line(-1, "1000000")}, nil, "tidak aktif"},
		{"unknown denom", []AllocationLineInput{{VendorVaultID: f.vault1, Amounts: map[string]string{"75000": "75000"}}}, nil, "tidak dikenal"},
		{"no amount", []AllocationLineInput{{VendorVaultID: f.vault1, Amounts: map[string]string{}}}, nil, "minimal satu"},
		{"urgent reason too short", []AllocationLineInput{urgent(&short)}, nil, "10-500"},
		{"urgent reason too long", []AllocationLineInput{urgent(&long)}, nil, "10-500"},
		{"urgent without reason", []AllocationLineInput{urgent(nil)}, nil, "10-500"},
		{"buffer reason too short", []AllocationLineInput{line(f.vault1, "2000000")}, &short, "10-500"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			var ve *ValidationError
			_, err := f.plans.SaveAllocations(ctx, acmUser, plan, tc.in, tc.buffer)
			if !errors.As(err, &ve) || !strings.Contains(ve.Message, tc.want) {
				t.Fatalf("want ValidationError containing %q, got %v", tc.want, err)
			}
		})
	}

	// Nothing saved yet: submit refuses; a draft cannot be rejected; an empty reason is refused first.
	var ve *ValidationError
	if _, err := f.plans.Submit(ctx, acmUser, plan); !errors.As(err, &ve) {
		t.Fatalf("submit without rows: want ValidationError, got %v", err)
	}
	if _, err := f.plans.Reject(ctx, acmSpv, plan, "cek kapasitas"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("reject draft: want ErrInvalidTransition, got %v", err)
	}
	if _, err := f.plans.Reject(ctx, acmSpv, plan, "  "); !errors.Is(err, ErrRejectReasonEmpty) {
		t.Fatalf("reject without reason: want ErrRejectReasonEmpty, got %v", err)
	}

	// Urgent tier 3 is accepted with a proper reason.
	reason := "kas region sendiri kosong"
	if _, err := f.plans.SaveAllocations(ctx, acmUser, plan, []AllocationLineInput{urgent(&reason)}, nil); err != nil {
		t.Fatalf("save urgent tier 3: %v", err)
	}
	if _, err := f.plans.Submit(ctx, acmUser, plan); err != nil {
		t.Fatalf("submit: %v", err)
	}

	// Pending: no more edits or re-submits; the maker cannot reject their own plan.
	if _, err := f.plans.SaveAllocations(ctx, acmUser, plan, []AllocationLineInput{line(f.vault1, "2000000")}, nil); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("save pending: want ErrInvalidTransition, got %v", err)
	}
	if _, err := f.plans.Submit(ctx, acmUser, plan); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("resubmit pending: want ErrInvalidTransition, got %v", err)
	}
	if _, err := f.plans.Reject(ctx, Actor{UserID: f.acmUserID, Role: "ACM-SPV"}, plan, "cek kapasitas"); !errors.Is(err, ErrSelfApproval) {
		t.Fatalf("maker rejects own plan: want ErrSelfApproval, got %v", err)
	}
	if d, err := f.plans.Reject(ctx, acmSpv, plan, "cek kapasitas"); err != nil || d.Plan.Status != planDraft {
		t.Fatalf("reject pending: %+v %v", d, err)
	}
}
