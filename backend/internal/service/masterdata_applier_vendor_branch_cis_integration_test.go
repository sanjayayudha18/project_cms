//go:build integration

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// vendor-cis-limit FR1.1–1.3: Limit CIS goes through the vendor_branch Applier; an update submitted
// with a Before snapshot from before migration 038 (no cis_* keys) is stale, never applied.
func TestVendorBranchApplier_CisLimit_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()
	q := dbtx(svc)

	vendorID := h.seedVendor("ITCIS-"+tag, "Vendor CIS "+tag)
	branchID := h.seedVendorBranch(vendorID, "ITCISB-"+tag, "Branch CIS "+tag)
	readLimit := func() *string {
		t.Helper()
		var s *string
		var cur string
		if err := q.QueryRow(ctx, `SELECT cis_limit_amount::text, cis_limit_currency FROM vendor_branches WHERE id = $1`, branchID).Scan(&s, &cur); err != nil {
			t.Fatal(err)
		}
		if cur != "IDR" {
			t.Errorf("currency = %q, want IDR", cur)
		}
		return s
	}
	update := func(limit *string) error {
		before, err := db.New(q).GetVendorBranchAdminByID(ctx, branchID) // same shape as CurrentState
		if err != nil {
			t.Fatal(err)
		}
		return approveChange(t, svc, h.insertPendingWithBefore("vendor_branch", "update", &branchID,
			VendorBranchUpdatePayload{BranchName: "Branch CIS " + tag, Category: "ATM", CisLimitAmount: limit}, before))
	}

	if got := readLimit(); got != nil {
		t.Fatalf("new branch limit = %v, want NULL", *got)
	}
	if err := update(sp("2500000000")); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	if got := readLimit(); got == nil || *got != "2500000000.00" {
		t.Fatalf("limit = %v, want 2500000000.00", showStr(got))
	}
	if err := update(nil); err != nil {
		t.Fatalf("clear limit: %v", err)
	}
	if got := readLimit(); got != nil {
		t.Fatalf("cleared limit = %v, want NULL", *got)
	}
	if err := update(sp("750000000.50")); err != nil {
		t.Fatalf("set limit again: %v", err)
	}

	// Pre-038 Before snapshot (no cis_* keys) -> stale, limit untouched.
	oldBefore := map[string]any{"id": branchID, "vendor_id": vendorID, "branch_code": "ITCISB-" + tag,
		"branch_name": "Branch CIS " + tag, "location_id": nil, "region": nil, "region_code": nil,
		"category": "ATM", "is_active": true, "deleted_at": nil}
	err := approveChange(t, svc, h.insertPendingWithBefore("vendor_branch", "update", &branchID,
		VendorBranchUpdatePayload{BranchName: "Branch CIS " + tag, Category: "ATM"}, oldBefore))
	if !errors.Is(err, ErrMasterDataChangeStale) {
		t.Fatalf("pre-038 update: err = %v, want stale", err)
	}
	if got := readLimit(); got == nil || *got != "750000000.50" {
		t.Fatalf("limit after stale change = %v, want 750000000.50", showStr(got))
	}

	// create carries the limit too
	if err := approveChange(t, svc, h.insertPending("vendor_branch", "create", nil, VendorBranchPayload{
		VendorID: vendorID, BranchCode: "ITCISC-" + tag, BranchName: "Created " + tag, Category: "CASH",
		CisLimitAmount: sp("1000"),
	})); err != nil {
		t.Fatalf("create: %v", err)
	}
	var created string
	if err := q.QueryRow(ctx, `SELECT cis_limit_amount::text FROM vendor_branches WHERE branch_code = $1`, "ITCISC-"+tag).Scan(&created); err != nil || created != "1000.00" {
		t.Fatalf("created limit = %q (err %v), want 1000.00", created, err)
	}
	if _, err := q.Exec(ctx, `DELETE FROM vendor_branches WHERE branch_code = $1`, "ITCISC-"+tag); err != nil {
		t.Fatalf("cleanup created branch: %v", err)
	}

	// DB CHECK is the last net.
	if _, err := q.Exec(ctx, `UPDATE vendor_branches SET cis_limit_amount = 0 WHERE id = $1`, branchID); err == nil {
		t.Error("CHECK vendor_branches_cis_limit_amount_chk: want error for 0")
	}
}
