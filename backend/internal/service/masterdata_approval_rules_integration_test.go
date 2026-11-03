//go:build integration

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/approval"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

// be-tech-debt C4.1: apply-on-approve guard rails beyond the happy/stale paths
// already in masterdata_approval_integration_test.go.

// A stale change is final: it cannot be retried into applying the outdated
// payload, the entity keeps the concurrent edit, and no master_data_applied
// audit row is written for it.
func TestMasterDataApprovalService_StaleChange_NotRetryable_NoAppliedAudit_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := adminCtx(h.makerID)

	vendorID := h.seedVendor("IT5-"+tag, "Original "+tag)
	before := db.GetVendorAdminByIDRow{ID: vendorID, Code: "IT5-" + tag, Name: "Original " + tag, IsActive: true}
	if _, err := dbtx(svc).Exec(ctx, `UPDATE vendors SET name = $1 WHERE id = $2`, "Concurrent "+tag, vendorID); err != nil {
		t.Fatalf("simulate concurrent update: %v", err)
	}
	changeID := h.insertPendingWithBefore("vendor", "update", &vendorID, vendorUpdatePayload{Name: "Outdated Edit " + tag}, before)
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "approved"}}

	if _, err := svc.Approve(ctx, 999, h.makerID, "127.0.0.1"); !errors.Is(err, ErrMasterDataChangeStale) {
		t.Fatalf("Approve() = %v, want ErrMasterDataChangeStale", err)
	}
	if err := svc.RetryApply(ctx, changeID, h.makerID, "127.0.0.1"); !errors.Is(err, ErrMasterDataNotRetryable) {
		t.Fatalf("RetryApply(stale) = %v, want ErrMasterDataNotRetryable", err)
	}

	var name string
	if err := dbtx(svc).QueryRow(ctx, `SELECT name FROM vendors WHERE id = $1`, vendorID).Scan(&name); err != nil {
		t.Fatalf("load vendor: %v", err)
	}
	if name != "Concurrent "+tag {
		t.Fatalf("vendor name = %q, want the concurrent edit kept", name)
	}
	var applied int
	if err := dbtx(svc).QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action = 'master_data_applied' AND entity_type = 'vendor' AND entity_id = $1`, vendorID).Scan(&applied); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if applied != 0 {
		t.Fatalf("master_data_applied audit rows = %d, want 0", applied)
	}
}

// The entity the maker snapshotted no longer exists at apply time: the
// staleness lookup fails, nothing is written, and the change stays
// approved-but-not-applied with the error recorded (retryable by an admin).
func TestMasterDataApprovalService_EntityGoneAtApply_ApprovedNotApplied_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()

	missingID := int64(9_000_000_000)
	before := db.GetVendorAdminByIDRow{ID: missingID, Code: "GONE-" + tag, Name: "Gone " + tag, IsActive: true}
	changeID := h.insertPendingWithBefore("vendor", "update", &missingID, vendorUpdatePayload{Name: "Edit " + tag}, before)
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "approved"}}

	if _, err := svc.Approve(ctx, 999, h.makerID, "127.0.0.1"); err == nil || errors.Is(err, ErrMasterDataChangeStale) {
		t.Fatalf("Approve() = %v, want a non-stale apply error", err)
	}
	change, err := svc.repo.GetByID(ctx, changeID)
	if err != nil {
		t.Fatalf("reload change: %v", err)
	}
	if change.Status != "approved" || change.Error == nil || *change.Error == "" {
		t.Fatalf("change = status %q error %v, want approved with error recorded", change.Status, change.Error)
	}
	var n int
	if err := dbtx(svc).QueryRow(ctx, `SELECT count(*) FROM vendors WHERE name = $1`, "Edit "+tag).Scan(&n); err != nil {
		t.Fatalf("count vendors: %v", err)
	}
	if n != 0 {
		t.Fatalf("vendors written = %d, want 0", n)
	}
}

// A refusal from the approval chain (e.g. maker != checker) must not touch
// the staged change: no apply on approve, no reject flip on reject.
func TestMasterDataApprovalService_OrchestratorRefusal_LeavesChangePending_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()

	changeID := h.insertPending("vendor", "create", nil, vendorCreatePayload{Code: "REF-" + tag, Name: "Refused " + tag})
	svc.orchestrator = &canApproveOnce{err: approval.ErrNotAuthorized}

	if _, err := svc.Approve(ctx, 999, h.makerID, "127.0.0.1"); !errors.Is(err, approval.ErrNotAuthorized) {
		t.Fatalf("Approve() = %v, want ErrNotAuthorized", err)
	}
	if _, err := svc.Reject(ctx, 999, h.makerID, "127.0.0.1"); !errors.Is(err, approval.ErrNotAuthorized) {
		t.Fatalf("Reject() = %v, want ErrNotAuthorized", err)
	}
	change, err := svc.repo.GetByID(ctx, changeID)
	if err != nil {
		t.Fatalf("reload change: %v", err)
	}
	if change.Status != "pending" {
		t.Fatalf("change.Status = %q, want pending", change.Status)
	}
	var n int
	if err := dbtx(svc).QueryRow(ctx, `SELECT count(*) FROM vendors WHERE code = $1`, "REF-"+tag).Scan(&n); err != nil {
		t.Fatalf("count vendors: %v", err)
	}
	if n != 0 {
		t.Fatalf("vendors created = %d, want 0", n)
	}
}
