//go:build integration

package service

import (
	"context"
	"errors"
	"testing"
)

// Feature: replenishment-request-enhancements, Property 6: Reject memindahkan pending ke rejected dengan satu audit
// Feature: replenishment-request-enhancements, Property 7: Four-eyes — creator tidak boleh approve/reject
//
// Task 9 verification: no production code changes were needed for Req 2 --
// Reject already implements pending_approval -> rejected, four-eyes
// (ErrSelfApproval), ErrInvalidTransition, ErrNotChecker, and the route gate
// is vendorRequestCheckerRoles (internal/handler/vendor_request_handler.go).
// These tests exercise that existing behavior end-to-end against real
// Postgres, reusing the harness from vendor_request_number_integration_test.go
// and vendor_request_cancel_integration_test.go.

// TestIntegration_Reject_PendingToRejectedWithSingleAudit covers Task 9.1
// (Property 6, Req 2.5): a non-creator Checker rejecting a pending_approval
// request with a valid reason moves it to 'rejected', persists the reason,
// and writes exactly one 'reject' audit entry.
func TestIntegration_Reject_PendingToRejectedWithSingleAudit(t *testing.T) {
	pool, vendorID, _, terminalID := setupNumberGeneratorHarness(t)
	ids := fetchUserIDs(t, pool, 2)
	creatorID, checkerID := ids[0], ids[1]
	svc := NewVendorRequestService(pool)
	ctx := context.Background()
	creator := Actor{UserID: creatorID, Role: "ATM-USER"}
	checker := Actor{UserID: checkerID, Role: "ATM-SPV"}

	created, err := svc.Create(ctx, creator, manualCreateInput(vendorID, terminalID, 50000))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Submit(ctx, creator, created.ID); err != nil {
		t.Fatalf("submit: %v", err)
	}

	const reason = "data tidak sesuai kondisi lapangan"
	result, err := svc.Reject(ctx, checker, created.ID, reason)
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if result.Status != "rejected" {
		t.Errorf("status = %q, want rejected", result.Status)
	}
	if result.RejectionReason != reason {
		t.Errorf("rejection_reason = %q, want %q", result.RejectionReason, reason)
	}

	var status, dbReason string
	if err := pool.QueryRow(ctx, `SELECT status, rejection_reason FROM vendor_requests WHERE id = $1`, created.ID).
		Scan(&status, &dbReason); err != nil {
		t.Fatalf("read back vendor_requests row: %v", err)
	}
	if status != "rejected" || dbReason != reason {
		t.Errorf("status=%q rejection_reason=%q, want status=rejected rejection_reason=%q", status, dbReason, reason)
	}

	if got := countAuditEntries(t, pool, created.ID, "reject"); got != 1 {
		t.Errorf("reject audit entries = %d, want exactly 1", got)
	}
}

// TestIntegration_FourEyes_CreatorCannotApproveOrReject covers Task 9.2
// (Property 7, Req 2.7, 6.3): a Checker who is also the creator of a
// pending_approval request is denied ErrSelfApproval on both Approve and
// Reject, and the request's state does not change.
func TestIntegration_FourEyes_CreatorCannotApproveOrReject(t *testing.T) {
	pool, vendorID, _, terminalID := setupNumberGeneratorHarness(t)
	ids := fetchUserIDs(t, pool, 1)
	creatorID := ids[0]
	svc := NewVendorRequestService(pool)
	ctx := context.Background()
	// The creator holds a Checker role (ADMIN is both maker and checker,
	// design.md "Roles") so isChecker(role) passes and ErrSelfApproval is
	// what actually gates the self-approval, not a plain ErrNotChecker.
	creatorAsChecker := Actor{UserID: creatorID, Role: "ADMIN"}

	t.Run("approve", func(t *testing.T) {
		created, err := svc.Create(ctx, creatorAsChecker, manualCreateInput(vendorID, terminalID, 50000))
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := svc.Submit(ctx, creatorAsChecker, created.ID); err != nil {
			t.Fatalf("submit: %v", err)
		}

		_, err = svc.Approve(ctx, creatorAsChecker, created.ID)
		if !errors.Is(err, ErrSelfApproval) {
			t.Fatalf("approve by creator error = %v, want ErrSelfApproval", err)
		}

		var status string
		if err := pool.QueryRow(ctx, `SELECT status FROM vendor_requests WHERE id = $1`, created.ID).Scan(&status); err != nil {
			t.Fatalf("read back status: %v", err)
		}
		if status != "pending_approval" {
			t.Errorf("status = %q after denied self-approval, want unchanged pending_approval", status)
		}
	})

	t.Run("reject", func(t *testing.T) {
		created, err := svc.Create(ctx, creatorAsChecker, manualCreateInput(vendorID, terminalID, 100000))
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := svc.Submit(ctx, creatorAsChecker, created.ID); err != nil {
			t.Fatalf("submit: %v", err)
		}

		_, err = svc.Reject(ctx, creatorAsChecker, created.ID, "mencoba menolak permintaan sendiri")
		if !errors.Is(err, ErrSelfApproval) {
			t.Fatalf("reject by creator error = %v, want ErrSelfApproval", err)
		}

		var status string
		if err := pool.QueryRow(ctx, `SELECT status FROM vendor_requests WHERE id = $1`, created.ID).Scan(&status); err != nil {
			t.Fatalf("read back status: %v", err)
		}
		if status != "pending_approval" {
			t.Errorf("status = %q after denied self-rejection, want unchanged pending_approval", status)
		}
	})
}
