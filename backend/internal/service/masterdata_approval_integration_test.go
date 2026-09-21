//go:build integration

package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/repository"
)

// testHelpers bundles the per-test insert/seed helpers and tracks every row
// id they create, so harness's cleanup can delete exactly what this test
// wrote -- no shared/global mutable state between tests.
type testHelpers struct {
	t               *testing.T
	ctx             context.Context
	pool            *pgxpool.Pool
	makerID         int64
	vendorIDs       []int64
	vendorBranchIDs []int64
	vendorVaultIDs  []int64
	vendorPicIDs    []int64
	vendorPkgIDs    []int64
	assignmentIDs   []int64
	atmIDs          []int64
	changeIDs       []int64
}

func (h *testHelpers) insertPending(entityType, op string, entityID *int64, payload any) int64 {
	return h.insertPendingWithBefore(entityType, op, entityID, payload, nil)
}

func (h *testHelpers) insertPendingWithBefore(entityType, op string, entityID *int64, payload, before any) int64 {
	h.t.Helper()
	row, err := db.New(h.pool).CreateMasterDataChangeRequest(h.ctx, db.CreateMasterDataChangeRequestParams{
		EntityType: entityType,
		EntityID:   entityID,
		Op:         op,
		Payload:    mustJSON(h.t, payload),
		Before:     jsonOrNil(h.t, before),
		MakerID:    h.makerID,
	})
	if err != nil {
		h.t.Fatalf("insert pending change request: %v", err)
	}
	h.changeIDs = append(h.changeIDs, row.ID)
	return row.ID
}

func (h *testHelpers) seedVendor(code, name string) int64 {
	h.t.Helper()
	var id int64
	if err := h.pool.QueryRow(h.ctx, `INSERT INTO vendors (code, name) VALUES ($1, $2) RETURNING id`, code, name).Scan(&id); err != nil {
		h.t.Fatalf("seed vendor: %v", err)
	}
	h.vendorIDs = append(h.vendorIDs, id)
	return id
}

// trackVendor registers a vendor id created indirectly (e.g. by an apply,
// not by seedVendor) so cleanup still deletes it.
func (h *testHelpers) trackVendor(id int64) {
	h.vendorIDs = append(h.vendorIDs, id)
}

// trackVendorBranch registers a vendor_branch id created indirectly (e.g. by
// an apply, not by seedVendorBranch) so cleanup still deletes it.
func (h *testHelpers) trackVendorBranch(id int64) {
	h.vendorBranchIDs = append(h.vendorBranchIDs, id)
}

// seedVendorBranch inserts a branch for an existing vendorID (see seedVendor).
func (h *testHelpers) seedVendorBranch(vendorID int64, code, name string) int64 {
	h.t.Helper()
	var id int64
	if err := h.pool.QueryRow(h.ctx, `INSERT INTO vendor_branches (vendor_id, branch_code, branch_name) VALUES ($1, $2, $3) RETURNING id`, vendorID, code, name).Scan(&id); err != nil {
		h.t.Fatalf("seed vendor branch: %v", err)
	}
	h.vendorBranchIDs = append(h.vendorBranchIDs, id)
	return id
}

// seedVendorPackage inserts a package under an existing branch (see seedVendorBranch).
func (h *testHelpers) seedVendorPackage(branchID int64, code string) int64 {
	h.t.Helper()
	var id int64
	if err := h.pool.QueryRow(h.ctx, `INSERT INTO vendor_packages (vendor_branch_id, code, priority_class, price) VALUES ($1, $2, 'ALL', 1) RETURNING id`, branchID, code).Scan(&id); err != nil {
		h.t.Fatalf("seed vendor package: %v", err)
	}
	h.vendorPkgIDs = append(h.vendorPkgIDs, id)
	return id
}

func (h *testHelpers) cleanup() {
	if len(h.changeIDs) > 0 {
		if _, err := h.pool.Exec(h.ctx, `DELETE FROM audit_logs WHERE entity_type = 'master_data_change_request' AND entity_id = ANY($1)`, h.changeIDs); err != nil {
			h.t.Logf("cleanup: delete audit_logs (change requests): %v", err)
		}
		if _, err := h.pool.Exec(h.ctx, `DELETE FROM master_data_change_requests WHERE id = ANY($1)`, h.changeIDs); err != nil {
			h.t.Logf("cleanup: delete master_data_change_requests: %v", err)
		}
	}
	if len(h.atmIDs) > 0 {
		if _, err := h.pool.Exec(h.ctx, `DELETE FROM audit_logs WHERE entity_type = 'atm' AND entity_id = ANY($1)`, h.atmIDs); err != nil {
			h.t.Logf("cleanup: delete audit_logs (atms): %v", err)
		}
		if _, err := h.pool.Exec(h.ctx, `DELETE FROM atms WHERE id = ANY($1)`, h.atmIDs); err != nil {
			h.t.Logf("cleanup: delete atms: %v", err)
		}
	}
	if len(h.assignmentIDs) > 0 {
		if _, err := h.pool.Exec(h.ctx, `DELETE FROM audit_logs WHERE entity_type = 'atm_assignment' AND entity_id = ANY($1)`, h.assignmentIDs); err != nil {
			h.t.Logf("cleanup: delete audit_logs (atm_assignments): %v", err)
		}
		if _, err := h.pool.Exec(h.ctx, `DELETE FROM atm_vendor_packages WHERE id = ANY($1)`, h.assignmentIDs); err != nil {
			h.t.Logf("cleanup: delete atm_vendor_packages: %v", err)
		}
	}
	if len(h.vendorPkgIDs) > 0 {
		if _, err := h.pool.Exec(h.ctx, `DELETE FROM audit_logs WHERE entity_type = 'vendor_package' AND entity_id = ANY($1)`, h.vendorPkgIDs); err != nil {
			h.t.Logf("cleanup: delete audit_logs (vendor_packages): %v", err)
		}
		if _, err := h.pool.Exec(h.ctx, `DELETE FROM vendor_packages WHERE id = ANY($1)`, h.vendorPkgIDs); err != nil {
			h.t.Logf("cleanup: delete vendor_packages: %v", err)
		}
	}
	if len(h.vendorPicIDs) > 0 {
		if _, err := h.pool.Exec(h.ctx, `DELETE FROM audit_logs WHERE entity_type = 'vendor_pic' AND entity_id = ANY($1)`, h.vendorPicIDs); err != nil {
			h.t.Logf("cleanup: delete audit_logs (vendor_pics): %v", err)
		}
		if _, err := h.pool.Exec(h.ctx, `DELETE FROM vendor_pics WHERE id = ANY($1)`, h.vendorPicIDs); err != nil {
			h.t.Logf("cleanup: delete vendor_pics: %v", err)
		}
	}
	if len(h.vendorVaultIDs) > 0 {
		if _, err := h.pool.Exec(h.ctx, `DELETE FROM audit_logs WHERE entity_type = 'vendor_vault' AND entity_id = ANY($1)`, h.vendorVaultIDs); err != nil {
			h.t.Logf("cleanup: delete audit_logs (vendor_vaults): %v", err)
		}
		if _, err := h.pool.Exec(h.ctx, `DELETE FROM vendor_vaults WHERE id = ANY($1)`, h.vendorVaultIDs); err != nil {
			h.t.Logf("cleanup: delete vendor_vaults: %v", err)
		}
	}
	if len(h.vendorBranchIDs) > 0 {
		if _, err := h.pool.Exec(h.ctx, `DELETE FROM audit_logs WHERE entity_type = 'vendor_branch' AND entity_id = ANY($1)`, h.vendorBranchIDs); err != nil {
			h.t.Logf("cleanup: delete audit_logs (vendor_branches): %v", err)
		}
		if _, err := h.pool.Exec(h.ctx, `DELETE FROM vendor_branches WHERE id = ANY($1)`, h.vendorBranchIDs); err != nil {
			h.t.Logf("cleanup: delete vendor_branches: %v", err)
		}
	}
	if len(h.vendorIDs) > 0 {
		if _, err := h.pool.Exec(h.ctx, `DELETE FROM audit_logs WHERE entity_type = 'vendor' AND entity_id = ANY($1)`, h.vendorIDs); err != nil {
			h.t.Logf("cleanup: delete audit_logs (vendors): %v", err)
		}
		if _, err := h.pool.Exec(h.ctx, `DELETE FROM vendors WHERE id = ANY($1)`, h.vendorIDs); err != nil {
			h.t.Logf("cleanup: delete vendors: %v", err)
		}
	}
}

// harness opens a real DB connection and wires MasterDataApprovalService
// exactly as production does: repo/audit go straight to the pool (durable,
// independent of any apply transaction), and pool.Begin opens the apply
// transaction. This matters here specifically -- unlike
// internal/rolemgmt's single-transaction harness, MasterDataApprovalService
// intentionally spans two transactional scopes (MarkApproved/MarkStale/
// MarkApplyFailed must survive the apply tx rolling back), so wrapping
// everything in one outer tx-as-pool (the usual convention, see
// internal/rolemgmt/service_integration_test.go) would let a real
// transaction's rollback silently undo a write that must be durable --
// exactly the bug this harness avoids by using two genuinely independent
// connections/transactions, same as production.
//
// Since nothing here rolls back, cleanup deletes exactly the rows this test
// created, tracked by id in testHelpers -- not by rollback.
//
// The orchestrator is a per-test fake (canApproveOnce below):
// approval.Orchestrator's own chain-building/hierarchy logic is already
// covered by internal/approval's own tests, so these tests exercise only
// what T2.3/T2.4/T2.5/T2.6 added -- the applier registry, the transactional
// apply-on-approve hook, staleness detection, and the reject path.
func harness(t *testing.T) (svc *MasterDataApprovalService, tag string, h *testHelpers) {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}
	t.Cleanup(pool.Close)

	var makerID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM users WHERE username = 'Yudha'`).Scan(&makerID); err != nil {
		t.Fatalf("loading seeded admin user: %v", err)
	}

	h = &testHelpers{t: t, ctx: ctx, pool: pool, makerID: makerID}
	t.Cleanup(h.cleanup)

	repo := repository.NewMasterDataChangeRepository(pool)
	svc = NewMasterDataApprovalService(nil, repo, pool, ApplierRegistry{
		"vendor":         VendorApplier{},
		"vendor_branch":  VendorBranchApplier{},
		"vendor_vault":   VendorVaultApplier{},
		"vendor_pic":     VendorPicApplier{},
		"vendor_package": VendorPackageApplier{},
		"atm_assignment": ATMAssignmentApplier{},
		"atm":            ATMApplier{},
	}, audit.NewWriter(pool))

	return svc, strconv.FormatInt(time.Now().UnixNano(), 10), h
}

// canApproveOnce is a minimal MasterDataApprovalOrchestrator fake that
// returns a fixed result regardless of requestID.
type canApproveOnce struct {
	result db.ApprovalRequest
	err    error
}

func (c *canApproveOnce) SubmitForApproval(ctx context.Context, makerID int64, documentType string, documentID int64, amount pgtype.Numeric, actorIP string) (db.ApprovalRequest, bool, error) {
	return c.result, false, c.err
}

func (c *canApproveOnce) Approve(ctx context.Context, requestID, actorID int64, actorIP string) (db.ApprovalRequest, error) {
	return c.result, c.err
}

func (c *canApproveOnce) Reject(ctx context.Context, requestID, actorID int64, actorIP string) (db.ApprovalRequest, error) {
	return c.result, c.err
}

func TestMasterDataApprovalService_Approve_Create_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()

	changeID := h.insertPending("vendor", "create", nil, vendorCreatePayload{
		Code: "IT-" + tag, Name: "Integration Test Vendor " + tag,
	})
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "approved"}}

	request, err := svc.Approve(ctx, 999, 1, "127.0.0.1")
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if request.Status != "approved" {
		t.Fatalf("request.Status = %q, want approved", request.Status)
	}

	change, err := svc.repo.GetByID(ctx, changeID)
	if err != nil {
		t.Fatalf("reload change request: %v", err)
	}
	if change.Status != "applied" {
		t.Fatalf("change.Status = %q, want applied", change.Status)
	}
	if change.Error != nil {
		t.Fatalf("change.Error = %q, want nil", *change.Error)
	}

	var vendorID int64
	var vendorName string
	if err := dbtx(svc).QueryRow(ctx, `SELECT id, name FROM vendors WHERE code = $1`, "IT-"+tag).Scan(&vendorID, &vendorName); err != nil {
		t.Fatalf("querying created vendor: %v", err)
	}
	h.trackVendor(vendorID)
	if vendorName != "Integration Test Vendor "+tag {
		t.Errorf("vendor name = %q, want %q", vendorName, "Integration Test Vendor "+tag)
	}

	var auditCount int
	if err := dbtx(svc).QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE entity_type = 'vendor' AND action = 'master_data_applied' AND entity_id = $1`,
		vendorID).Scan(&auditCount); err != nil {
		t.Fatalf("counting audit_logs: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("audit_logs rows for master_data_applied = %d, want 1", auditCount)
	}
}

// T4.3: legal_name/npwp round-trip through the real vendors table (incl. its
// 15/16-digit CHECK), an update using the REAL new-shape "before" snapshot
// applies without a false stale, and a blank clears the columns to NULL.
func TestVendorApplier_Approve_LegalNameAndNPWP_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()

	code := "ITNP-" + tag
	changeID := h.insertPending("vendor", "create", nil, vendorCreatePayload{
		Code: code, Name: "NPWP Vendor " + tag, LegalName: "PT NPWP Sejahtera", NPWP: "012345678901000",
	})
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "approved"}}
	if _, err := svc.Approve(ctx, 999, 1, "127.0.0.1"); err != nil {
		t.Fatalf("Approve(create) error = %v", err)
	}

	var vendorID int64
	var legal, npwp *string
	if err := dbtx(svc).QueryRow(ctx, `SELECT id, legal_name, npwp FROM vendors WHERE code = $1`, code).Scan(&vendorID, &legal, &npwp); err != nil {
		t.Fatalf("querying created vendor: %v", err)
	}
	h.trackVendor(vendorID)
	if legal == nil || *legal != "PT NPWP Sejahtera" || npwp == nil || *npwp != "012345678901000" {
		t.Errorf("created vendor legal_name=%v npwp=%v, want PT NPWP Sejahtera / 012345678901000", legal, npwp)
	}

	realBefore := func() *db.GetVendorAdminByIDRow {
		row, err := db.New(h.pool).GetVendorAdminByID(ctx, vendorID)
		if err != nil {
			t.Fatalf("GetVendorAdminByID: %v", err)
		}
		return &row
	}

	// Change the NPWP only (service resolves the kept legal_name into the payload).
	upd := h.insertPendingWithBefore("vendor", "update", &vendorID,
		vendorUpdatePayload{Name: "NPWP Vendor " + tag, LegalName: "PT NPWP Sejahtera", NPWP: "0123456789012345"}, realBefore())
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: upd, Status: "approved"}}
	if _, err := svc.Approve(ctx, 999, 1, "127.0.0.1"); err != nil {
		t.Fatalf("Approve(update) with a real new-shape snapshot must apply, got %v", err)
	}
	if err := dbtx(svc).QueryRow(ctx, `SELECT legal_name, npwp FROM vendors WHERE id = $1`, vendorID).Scan(&legal, &npwp); err != nil {
		t.Fatal(err)
	}
	if legal == nil || *legal != "PT NPWP Sejahtera" || npwp == nil || *npwp != "0123456789012345" {
		t.Errorf("after update legal_name=%v npwp=%v, want kept name and 16-digit npwp", legal, npwp)
	}

	// Blank in the payload clears both to NULL.
	clr := h.insertPendingWithBefore("vendor", "update", &vendorID, vendorUpdatePayload{Name: "NPWP Vendor " + tag}, realBefore())
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: clr, Status: "approved"}}
	if _, err := svc.Approve(ctx, 999, 1, "127.0.0.1"); err != nil {
		t.Fatalf("Approve(clear) error = %v", err)
	}
	if err := dbtx(svc).QueryRow(ctx, `SELECT legal_name, npwp FROM vendors WHERE id = $1`, vendorID).Scan(&legal, &npwp); err != nil {
		t.Fatal(err)
	}
	if legal != nil || npwp != nil {
		t.Errorf("blank payload must clear to NULL, got legal_name=%v npwp=%v", legal, npwp)
	}
}

// T4.1: two creates for the same vendor code can be pending at once (the
// submit-time FindByCode check only sees committed rows). The DB unique
// constraint decides at apply time and must surface as the clean
// ErrVendorCodeConflict -- no second vendor row, failure recorded on the
// change request without the raw constraint name.
func TestVendorApplier_Approve_DuplicateCodeAtApply_CleanConflict_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()

	code := "ITDUP-" + tag
	h.seedVendor(code, "Existing Vendor "+tag)

	changeID := h.insertPending("vendor", "create", nil, vendorCreatePayload{Code: code, Name: "Duplicate " + tag})
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "approved"}}

	_, err := svc.Approve(ctx, 999, 1, "127.0.0.1")
	if !errors.Is(err, ErrVendorCodeConflict) {
		t.Fatalf("Approve() error = %v, want ErrVendorCodeConflict", err)
	}

	var n int
	if err := dbtx(svc).QueryRow(ctx, `SELECT count(*) FROM vendors WHERE code = $1`, code).Scan(&n); err != nil || n != 1 {
		t.Fatalf("vendors with code %s = %d (err %v), want exactly the pre-existing 1", code, n, err)
	}
	var recorded *string
	if err := dbtx(svc).QueryRow(ctx, `SELECT error FROM master_data_change_requests WHERE id = $1`, changeID).Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if recorded == nil || !strings.Contains(*recorded, ErrVendorCodeConflict.Error()) || strings.Contains(*recorded, "vendors_code") {
		t.Errorf("recorded apply error should be the clean conflict message, got %v", recorded)
	}
}

func TestMasterDataApprovalService_Approve_UpdateThenDisable_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()

	vendorID := h.seedVendor("IT2-"+tag, "Before Update "+tag)

	updateChangeID := h.insertPending("vendor", "update", &vendorID, vendorUpdatePayload{Name: "After Update " + tag})
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: updateChangeID, Status: "approved"}}
	if _, err := svc.Approve(ctx, 999, 1, "127.0.0.1"); err != nil {
		t.Fatalf("Approve(update) error = %v", err)
	}

	var name string
	if err := dbtx(svc).QueryRow(ctx, `SELECT name FROM vendors WHERE id = $1`, vendorID).Scan(&name); err != nil {
		t.Fatalf("querying updated vendor: %v", err)
	}
	if name != "After Update "+tag {
		t.Errorf("vendor name after update = %q, want %q", name, "After Update "+tag)
	}

	disableChangeID := h.insertPending("vendor", "disable", &vendorID, nil)
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: disableChangeID, Status: "approved"}}
	if _, err := svc.Approve(ctx, 999, 1, "127.0.0.1"); err != nil {
		t.Fatalf("Approve(disable) error = %v", err)
	}

	var isActive bool
	if err := dbtx(svc).QueryRow(ctx, `SELECT is_active FROM vendors WHERE id = $1`, vendorID).Scan(&isActive); err != nil {
		t.Fatalf("querying disabled vendor: %v", err)
	}
	if isActive {
		t.Error("expected vendor to be disabled (is_active=false)")
	}
}

func TestMasterDataApprovalService_Approve_MissingApplier_LeavesApprovedNotApplied_Integration(t *testing.T) {
	svc, _, h := harness(t)
	ctx := context.Background()

	changeID := h.insertPending("vendor_pic", "create", nil, map[string]string{"name": "no applier yet"})
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "approved"}}

	_, err := svc.Approve(ctx, 999, 1, "127.0.0.1")
	if err == nil {
		t.Fatal("expected an error when no applier is registered for entity_type=vendor_pic")
	}

	change, getErr := svc.repo.GetByID(ctx, changeID)
	if getErr != nil {
		t.Fatalf("reload change request: %v", getErr)
	}
	if change.Status != "approved" {
		t.Fatalf("change.Status = %q, want approved (approved-but-not-applied)", change.Status)
	}
	if change.Error == nil || *change.Error == "" {
		t.Fatal("expected change.Error to be set")
	}
}

func TestMasterDataApprovalService_Approve_NonMasterDataDocument_SkipsApply_Integration(t *testing.T) {
	svc, _, _ := harness(t)
	ctx := context.Background()

	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: "invoice", DocumentID: 12345, Status: "approved"}}

	request, err := svc.Approve(ctx, 999, 1, "127.0.0.1")
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if request.DocumentType != "invoice" {
		t.Fatalf("request.DocumentType = %q, want invoice (untouched)", request.DocumentType)
	}
}

func TestMasterDataApprovalService_Approve_StaleEntity_MarksStale_NoApply_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()

	vendorID := h.seedVendor("IT3-"+tag, "Original Name "+tag)
	// Snapshot "before" as it would have looked at submit time...
	staleBefore := db.GetVendorAdminByIDRow{ID: vendorID, Code: "IT3-" + tag, Name: "Original Name " + tag, IsActive: true}

	// ...but the entity changed in between (someone else updated it directly).
	if _, err := dbtx(svc).Exec(ctx, `UPDATE vendors SET name = $1 WHERE id = $2`, "Changed Behind Your Back "+tag, vendorID); err != nil {
		t.Fatalf("simulate concurrent update: %v", err)
	}

	changeID := h.insertPendingWithBefore("vendor", "update", &vendorID, vendorUpdatePayload{Name: "Attempted New Name " + tag}, staleBefore)
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "approved"}}

	_, err := svc.Approve(ctx, 999, 1, "127.0.0.1")
	if !errors.Is(err, ErrMasterDataChangeStale) {
		t.Fatalf("Approve() error = %v, want ErrMasterDataChangeStale", err)
	}

	change, getErr := svc.repo.GetByID(ctx, changeID)
	if getErr != nil {
		t.Fatalf("reload change request: %v", getErr)
	}
	if change.Status != "stale" {
		t.Fatalf("change.Status = %q, want stale", change.Status)
	}

	// The entity itself must be untouched by the rolled-back apply attempt.
	var name string
	if err := dbtx(svc).QueryRow(ctx, `SELECT name FROM vendors WHERE id = $1`, vendorID).Scan(&name); err != nil {
		t.Fatalf("querying vendor: %v", err)
	}
	if name != "Changed Behind Your Back "+tag {
		t.Errorf("vendor name = %q, want unchanged %q", name, "Changed Behind Your Back "+tag)
	}
}

func TestMasterDataApprovalService_Approve_UnchangedEntity_AppliesNormally_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()

	vendorID := h.seedVendor("IT4-"+tag, "Steady Name "+tag)
	before := db.GetVendorAdminByIDRow{ID: vendorID, Code: "IT4-" + tag, Name: "Steady Name " + tag, IsActive: true}

	changeID := h.insertPendingWithBefore("vendor", "update", &vendorID, vendorUpdatePayload{Name: "Updated Name " + tag}, before)
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "approved"}}

	if _, err := svc.Approve(ctx, 999, 1, "127.0.0.1"); err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	change, getErr := svc.repo.GetByID(ctx, changeID)
	if getErr != nil {
		t.Fatalf("reload change request: %v", getErr)
	}
	if change.Status != "applied" {
		t.Fatalf("change.Status = %q, want applied", change.Status)
	}

	var name string
	if err := dbtx(svc).QueryRow(ctx, `SELECT name FROM vendors WHERE id = $1`, vendorID).Scan(&name); err != nil {
		t.Fatalf("querying vendor: %v", err)
	}
	if name != "Updated Name "+tag {
		t.Errorf("vendor name = %q, want %q", name, "Updated Name "+tag)
	}
}

func TestMasterDataApprovalService_Reject_MasterData_NoEntityChange_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()

	changeID := h.insertPending("vendor", "create", nil, vendorCreatePayload{Code: "IT5-" + tag, Name: "Should Not Exist " + tag})
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "rejected"}}

	request, err := svc.Reject(ctx, 999, 1, "127.0.0.1")
	if err != nil {
		t.Fatalf("Reject() error = %v", err)
	}
	if request.Status != "rejected" {
		t.Fatalf("request.Status = %q, want rejected", request.Status)
	}

	change, getErr := svc.repo.GetByID(ctx, changeID)
	if getErr != nil {
		t.Fatalf("reload change request: %v", getErr)
	}
	if change.Status != "rejected" {
		t.Fatalf("change.Status = %q, want rejected", change.Status)
	}

	var vendorCount int
	if err := dbtx(svc).QueryRow(ctx, `SELECT COUNT(*) FROM vendors WHERE code = $1`, "IT5-"+tag).Scan(&vendorCount); err != nil {
		t.Fatalf("counting vendors: %v", err)
	}
	if vendorCount != 0 {
		t.Errorf("expected no vendor created on reject, found %d", vendorCount)
	}

	var auditCount int
	if err := dbtx(svc).QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE entity_type = 'master_data_change_request' AND entity_id = $1 AND action = 'reject'`,
		changeID).Scan(&auditCount); err != nil {
		t.Fatalf("counting audit_logs: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("audit_logs rows for reject = %d, want 1", auditCount)
	}
}

func TestMasterDataApprovalService_Reject_NonMasterDataDocument_NoChangeRequestTouched_Integration(t *testing.T) {
	svc, _, _ := harness(t)
	ctx := context.Background()

	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: "invoice", DocumentID: 12345, Status: "rejected"}}

	request, err := svc.Reject(ctx, 999, 1, "127.0.0.1")
	if err != nil {
		t.Fatalf("Reject() error = %v", err)
	}
	if request.DocumentType != "invoice" {
		t.Fatalf("request.DocumentType = %q, want invoice (untouched)", request.DocumentType)
	}
}

func TestVendorBranchApplier_Approve_Create_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()

	vendorID := h.seedVendor("ITVB-"+tag, "Vendor For Branch Test "+tag)
	changeID := h.insertPending("vendor_branch", "create", nil, VendorBranchPayload{
		VendorID: vendorID, BranchCode: "ITVBR-" + tag, BranchName: "Branch " + tag,
	})
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "approved"}}

	request, err := svc.Approve(ctx, 999, 1, "127.0.0.1")
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if request.Status != "approved" {
		t.Fatalf("request.Status = %q, want approved", request.Status)
	}

	change, err := svc.repo.GetByID(ctx, changeID)
	if err != nil {
		t.Fatalf("reload change request: %v", err)
	}
	if change.Status != "applied" {
		t.Fatalf("change.Status = %q, want applied", change.Status)
	}

	var branchID int64
	var branchName string
	if err := dbtx(svc).QueryRow(ctx, `SELECT id, branch_name FROM vendor_branches WHERE branch_code = $1`, "ITVBR-"+tag).Scan(&branchID, &branchName); err != nil {
		t.Fatalf("querying created vendor branch: %v", err)
	}
	h.trackVendorBranch(branchID)
	if branchName != "Branch "+tag {
		t.Errorf("branch name = %q, want %q", branchName, "Branch "+tag)
	}

	var auditCount int
	if err := dbtx(svc).QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE entity_type = 'vendor_branch' AND action = 'master_data_applied' AND entity_id = $1`,
		branchID).Scan(&auditCount); err != nil {
		t.Fatalf("counting audit_logs: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("audit_logs rows for master_data_applied = %d, want 1", auditCount)
	}
}

func TestVendorBranchApplier_Approve_UpdateThenDisable_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()

	vendorID := h.seedVendor("ITVB2-"+tag, "Vendor For Branch Test 2 "+tag)
	branchID := h.seedVendorBranch(vendorID, "ITVBR2-"+tag, "Before Update "+tag)

	updateChangeID := h.insertPending("vendor_branch", "update", &branchID, VendorBranchUpdatePayload{BranchName: "After Update " + tag})
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: updateChangeID, Status: "approved"}}
	if _, err := svc.Approve(ctx, 999, 1, "127.0.0.1"); err != nil {
		t.Fatalf("Approve(update) error = %v", err)
	}

	var name string
	if err := dbtx(svc).QueryRow(ctx, `SELECT branch_name FROM vendor_branches WHERE id = $1`, branchID).Scan(&name); err != nil {
		t.Fatalf("querying updated branch: %v", err)
	}
	if name != "After Update "+tag {
		t.Errorf("branch name after update = %q, want %q", name, "After Update "+tag)
	}

	disableChangeID := h.insertPending("vendor_branch", "disable", &branchID, nil)
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: disableChangeID, Status: "approved"}}
	if _, err := svc.Approve(ctx, 999, 1, "127.0.0.1"); err != nil {
		t.Fatalf("Approve(disable) error = %v", err)
	}

	var isActive bool
	if err := dbtx(svc).QueryRow(ctx, `SELECT is_active FROM vendor_branches WHERE id = $1`, branchID).Scan(&isActive); err != nil {
		t.Fatalf("querying disabled branch: %v", err)
	}
	if isActive {
		t.Error("expected branch to be disabled (is_active=false)")
	}
}

func TestVendorVaultApplier_Approve_CreateUpdateDisable_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()

	vendorID := h.seedVendor("ITVV-"+tag, "Vendor For Vault Test "+tag)
	branchID := h.seedVendorBranch(vendorID, "ITVVB-"+tag, "Branch "+tag)

	changeID := h.insertPending("vendor_vault", "create", nil, VendorVaultPayload{
		VendorBranchID: branchID, VaultCode: "ITVV-V-" + tag,
		VendorVaultUpdatePayload: VendorVaultUpdatePayload{
			Category: "CASH", CurrencyCode: "IDR",
			MinCapacityAmount: sp("1000000.50"), MaxCapacityAmount: sp("9000000000.00"),
			Latitude: sp("-6.200000"), Longitude: sp("106.816666"),
		},
	})
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "approved"}}
	if _, err := svc.Approve(ctx, 999, 1, "127.0.0.1"); err != nil {
		t.Fatalf("Approve(create) error = %v", err)
	}

	var vaultID int64
	var typ, category, minC, maxC, lat string
	if err := dbtx(svc).QueryRow(ctx,
		`SELECT id, type, category, min_capacity_amount::text, max_capacity_amount::text, latitude::text FROM vendor_vaults WHERE vault_code = $1`,
		"ITVV-V-"+tag).Scan(&vaultID, &typ, &category, &minC, &maxC, &lat); err != nil {
		t.Fatalf("querying created vault: %v", err)
	}
	h.vendorVaultIDs = append(h.vendorVaultIDs, vaultID)
	if category != "CASH" || typ != "CASH" || minC != "1000000.50" || maxC != "9000000000.00" || lat != "-6.200000" {
		t.Errorf("unexpected vault row: type=%s category=%s min=%s max=%s lat=%s", typ, category, minC, maxC, lat)
	}

	// Update: change category, drop capacities (NULL).
	updateID := h.insertPending("vendor_vault", "update", &vaultID, VendorVaultUpdatePayload{Category: "ATM", CurrencyCode: "IDR"})
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: updateID, Status: "approved"}}
	if _, err := svc.Approve(ctx, 999, 1, "127.0.0.1"); err != nil {
		t.Fatalf("Approve(update) error = %v", err)
	}
	var minIsNull bool
	if err := dbtx(svc).QueryRow(ctx, `SELECT category, min_capacity_amount IS NULL FROM vendor_vaults WHERE id = $1`, vaultID).Scan(&category, &minIsNull); err != nil {
		t.Fatalf("querying updated vault: %v", err)
	}
	if category != "ATM" || !minIsNull {
		t.Errorf("after update: category=%s minIsNull=%v, want ATM/true", category, minIsNull)
	}

	// Disable.
	disableID := h.insertPending("vendor_vault", "disable", &vaultID, nil)
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: disableID, Status: "approved"}}
	if _, err := svc.Approve(ctx, 999, 1, "127.0.0.1"); err != nil {
		t.Fatalf("Approve(disable) error = %v", err)
	}
	var active bool
	if err := dbtx(svc).QueryRow(ctx, `SELECT is_active FROM vendor_vaults WHERE id = $1`, vaultID).Scan(&active); err != nil {
		t.Fatalf("querying disabled vault: %v", err)
	}
	if active {
		t.Error("expected vault disabled")
	}
}

func TestVendorPicApplier_Approve_CreateUpdateDisable_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()

	vendorID := h.seedVendor("ITVP-"+tag, "Vendor For PIC Test "+tag)

	changeID := h.insertPending("vendor_pic", "create", nil, VendorPicPayload{
		VendorID: vendorID,
		VendorPicUpdatePayload: VendorPicUpdatePayload{
			Name: "PIC " + tag, Position: sp("Kepala Cabang"), Phone: sp("+62 812-3456-7890"),
			Email: sp("pic@example.com"), IsNotificationRecipient: true,
		},
	})
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "approved"}}
	if _, err := svc.Approve(ctx, 999, 1, "127.0.0.1"); err != nil {
		t.Fatalf("Approve(create) error = %v", err)
	}

	var picID int64
	var name string
	var notify bool
	if err := dbtx(svc).QueryRow(ctx, `SELECT id, name, is_notification_recipient FROM vendor_pics WHERE vendor_id = $1`, vendorID).Scan(&picID, &name, &notify); err != nil {
		t.Fatalf("querying created pic: %v", err)
	}
	h.vendorPicIDs = append(h.vendorPicIDs, picID)
	if name != "PIC "+tag || !notify {
		t.Errorf("unexpected pic row: name=%q notify=%v", name, notify)
	}

	// Update: drop position/phone/email, unflag notification recipient.
	updateID := h.insertPending("vendor_pic", "update", &picID, VendorPicUpdatePayload{Name: "PIC Renamed " + tag})
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: updateID, Status: "approved"}}
	if _, err := svc.Approve(ctx, 999, 1, "127.0.0.1"); err != nil {
		t.Fatalf("Approve(update) error = %v", err)
	}
	var phoneIsNull bool
	if err := dbtx(svc).QueryRow(ctx, `SELECT name, phone IS NULL, is_notification_recipient FROM vendor_pics WHERE id = $1`, picID).Scan(&name, &phoneIsNull, &notify); err != nil {
		t.Fatalf("querying updated pic: %v", err)
	}
	if name != "PIC Renamed "+tag || !phoneIsNull || notify {
		t.Errorf("after update: name=%q phoneIsNull=%v notify=%v", name, phoneIsNull, notify)
	}

	// Disable.
	disableID := h.insertPending("vendor_pic", "disable", &picID, nil)
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: disableID, Status: "approved"}}
	if _, err := svc.Approve(ctx, 999, 1, "127.0.0.1"); err != nil {
		t.Fatalf("Approve(disable) error = %v", err)
	}
	var active, deleted bool
	if err := dbtx(svc).QueryRow(ctx, `SELECT is_active, deleted_at IS NOT NULL FROM vendor_pics WHERE id = $1`, picID).Scan(&active, &deleted); err != nil {
		t.Fatalf("querying disabled pic: %v", err)
	}
	if active || !deleted {
		t.Errorf("expected soft-disabled pic, got is_active=%v deleted=%v", active, deleted)
	}
}

func TestVendorPackageApplier_Approve_CreateUpdateDisable_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()

	vendorID := h.seedVendor("ITVK-"+tag, "Vendor For Package Test "+tag)
	branchID := h.seedVendorBranch(vendorID, "ITVKB-"+tag, "Branch "+tag)

	changeID := h.insertPending("vendor_package", "create", nil, VendorPackagePayload{
		VendorBranchID: branchID, Code: "ITVK-P-" + tag,
		VendorPackageUpdatePayload: VendorPackageUpdatePayload{PriorityClass: "ALL", Price: "1500000.50"},
	})
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "approved"}}
	if _, err := svc.Approve(ctx, 999, 1, "127.0.0.1"); err != nil {
		t.Fatalf("Approve(create) error = %v", err)
	}

	var pkgID int64
	var class, price string
	var active bool
	if err := dbtx(svc).QueryRow(ctx, `SELECT id, priority_class, price::text, is_active FROM vendor_packages WHERE vendor_branch_id = $1 AND code = $2`,
		branchID, "ITVK-P-"+tag).Scan(&pkgID, &class, &price, &active); err != nil {
		t.Fatalf("querying created package: %v", err)
	}
	h.vendorPkgIDs = append(h.vendorPkgIDs, pkgID)
	if class != "ALL" || price != "1500000.50" || !active {
		t.Errorf("unexpected package row: class=%s price=%s active=%v", class, price, active)
	}

	updateID := h.insertPending("vendor_package", "update", &pkgID, VendorPackageUpdatePayload{PriorityClass: "VIP", Price: "2000000"})
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: updateID, Status: "approved"}}
	if _, err := svc.Approve(ctx, 999, 1, "127.0.0.1"); err != nil {
		t.Fatalf("Approve(update) error = %v", err)
	}
	if err := dbtx(svc).QueryRow(ctx, `SELECT priority_class, price::text FROM vendor_packages WHERE id = $1`, pkgID).Scan(&class, &price); err != nil {
		t.Fatalf("querying updated package: %v", err)
	}
	if class != "VIP" || price != "2000000.00" {
		t.Errorf("after update: class=%s price=%s, want VIP/2000000.00", class, price)
	}

	disableID := h.insertPending("vendor_package", "disable", &pkgID, nil)
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: disableID, Status: "approved"}}
	if _, err := svc.Approve(ctx, 999, 1, "127.0.0.1"); err != nil {
		t.Fatalf("Approve(disable) error = %v", err)
	}
	var deleted bool
	if err := dbtx(svc).QueryRow(ctx, `SELECT is_active, deleted_at IS NOT NULL FROM vendor_packages WHERE id = $1`, pkgID).Scan(&active, &deleted); err != nil {
		t.Fatalf("querying disabled package: %v", err)
	}
	if active || !deleted {
		t.Errorf("expected soft-disabled package, got is_active=%v deleted=%v", active, deleted)
	}
}

// approveATM stages an atm change (optionally with a real "before" snapshot)
// and approves it, returning the change id and Approve's error.
func approveATM(t *testing.T, svc *MasterDataApprovalService, h *testHelpers, op string, entityID *int64, payload, before any) (int64, error) {
	t.Helper()
	changeID := h.insertPendingWithBefore("atm", op, entityID, payload, before)
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "approved"}}
	_, err := svc.Approve(context.Background(), 999, 1, "127.0.0.1")
	return changeID, err
}

func testATMPayload(locationID int64, brand string) atmUpdatePayload {
	return atmUpdatePayload{
		LocationID: locationID, MachineType: "ATM", Brand: brand, Model: "SelfServ",
		OperationHours: "24 Hours", DeploymentType: "Onsite",
	}
}

// T4.2: ATMApplier lifecycle, with the real GetATMAdminByID row as the
// "before" snapshot exactly as ATMAdminService.Update captures it. ATM rows
// carry numeric money columns and timestamptz, so this proves those survive
// the jsonb round-trip without a false "stale" (an unchanged ATM must apply),
// while a genuine concurrent edit is still caught.
func TestATMApplier_Approve_Lifecycle_SnapshotRoundTrip_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()

	var locationID int64
	if err := h.pool.QueryRow(ctx, `SELECT id FROM locations ORDER BY id LIMIT 1`).Scan(&locationID); err != nil {
		t.Fatalf("picking a location: %v", err)
	}

	term := "ITATM-" + tag
	create := atmCreatePayload{TerminalID: term, atmUpdatePayload: testATMPayload(locationID, "NCR")}
	create.CapacityAmount, create.LowThresholdAmount, create.PriorityClass = sp("1234567890123456.78"), sp("100.50"), sp("VIP")
	if _, err := approveATM(t, svc, h, "create", nil, create, nil); err != nil {
		t.Fatalf("Approve(create) error = %v", err)
	}

	var atmID int64
	var capacity, low, priority string
	if err := dbtx(svc).QueryRow(ctx, `SELECT id, capacity_amount::text, low_threshold_amount::text, priority_class FROM atms WHERE terminal_id = $1`, term).Scan(&atmID, &capacity, &low, &priority); err != nil {
		t.Fatalf("querying created atm: %v", err)
	}
	h.atmIDs = append(h.atmIDs, atmID)
	if capacity != "1234567890123456.78" || low != "100.50" || priority != "VIP" {
		t.Errorf("created row: capacity=%s low=%s priority=%s, want exact decimals + VIP", capacity, low, priority)
	}

	realBefore := func() *db.GetATMAdminByIDRow {
		row, err := db.New(h.pool).GetATMAdminByID(ctx, atmID)
		if err != nil {
			t.Fatalf("GetATMAdminByID: %v", err)
		}
		return &row
	}

	// Update with the real snapshot, nothing else touching the row: must apply.
	upd := testATMPayload(locationID, "Diebold") // capacity/low/priority omitted => NULL
	if _, err := approveATM(t, svc, h, "update", &atmID, upd, realBefore()); err != nil {
		t.Fatalf("Approve(update) with an unchanged before-snapshot must apply, got %v (numeric/timestamptz snapshot round-trip regression?)", err)
	}
	var brand, gotTerm string
	var capIsNull bool
	if err := dbtx(svc).QueryRow(ctx, `SELECT brand, terminal_id, capacity_amount IS NULL FROM atms WHERE id = $1`, atmID).Scan(&brand, &gotTerm, &capIsNull); err != nil {
		t.Fatal(err)
	}
	if brand != "Diebold" || gotTerm != term || !capIsNull {
		t.Errorf("after update: brand=%s terminal=%s capacityNull=%v, want Diebold/%s/true (terminal_id must be untouched)", brand, gotTerm, capIsNull, term)
	}

	// A concurrent change after the snapshot is taken => stale, not applied.
	staleBefore := realBefore()
	if _, err := h.pool.Exec(ctx, `UPDATE atms SET brand = 'Changed Elsewhere' WHERE id = $1`, atmID); err != nil {
		t.Fatal(err)
	}
	changeID, err := approveATM(t, svc, h, "update", &atmID, testATMPayload(locationID, "Should Not Apply"), staleBefore)
	if !errors.Is(err, ErrMasterDataChangeStale) {
		t.Fatalf("Approve(update on changed atm) error = %v, want ErrMasterDataChangeStale", err)
	}
	if err := dbtx(svc).QueryRow(ctx, `SELECT brand FROM atms WHERE id = $1`, atmID).Scan(&brand); err != nil || brand != "Changed Elsewhere" {
		t.Errorf("stale change must not apply: brand=%q err=%v", brand, err)
	}
	if change, err := svc.repo.GetByID(ctx, changeID); err != nil || change.Status != "stale" {
		t.Errorf("change status = %v (err %v), want stale", change.Status, err)
	}

	// Disable then Enable, each with a fresh real snapshot.
	if _, err := approveATM(t, svc, h, "disable", &atmID, nil, realBefore()); err != nil {
		t.Fatalf("Approve(disable) error = %v", err)
	}
	var active, deleted bool
	if err := dbtx(svc).QueryRow(ctx, `SELECT is_active, deleted_at IS NOT NULL FROM atms WHERE id = $1`, atmID).Scan(&active, &deleted); err != nil || active || !deleted {
		t.Fatalf("after disable: active=%v deleted=%v err=%v", active, deleted, err)
	}
	if _, err := approveATM(t, svc, h, "enable", &atmID, nil, realBefore()); err != nil {
		t.Fatalf("Approve(enable) error = %v", err)
	}
	if err := dbtx(svc).QueryRow(ctx, `SELECT is_active, deleted_at IS NOT NULL FROM atms WHERE id = $1`, atmID).Scan(&active, &deleted); err != nil || !active || deleted {
		t.Fatalf("after enable: active=%v deleted=%v err=%v", active, deleted, err)
	}
}

// Two creates for the same terminal_id can be pending at once; the DB unique
// constraint decides at apply time and must surface as the clean
// ErrATMTerminalIDConflict -- no second row, no raw constraint name recorded.
func TestATMApplier_Approve_DuplicateTerminalAtApply_CleanConflict_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()

	var locationID int64
	if err := h.pool.QueryRow(ctx, `SELECT id FROM locations ORDER BY id LIMIT 1`).Scan(&locationID); err != nil {
		t.Fatalf("picking a location: %v", err)
	}
	term := "ITATMD-" + tag
	first := atmCreatePayload{TerminalID: term, atmUpdatePayload: testATMPayload(locationID, "NCR")}
	if _, err := approveATM(t, svc, h, "create", nil, first, nil); err != nil {
		t.Fatalf("Approve(first create) error = %v", err)
	}
	var atmID int64
	if err := dbtx(svc).QueryRow(ctx, `SELECT id FROM atms WHERE terminal_id = $1`, term).Scan(&atmID); err != nil {
		t.Fatal(err)
	}
	h.atmIDs = append(h.atmIDs, atmID)

	dup := atmCreatePayload{TerminalID: term, atmUpdatePayload: testATMPayload(locationID, "Diebold")}
	changeID, err := approveATM(t, svc, h, "create", nil, dup, nil)
	if !errors.Is(err, ErrATMTerminalIDConflict) {
		t.Fatalf("Approve(duplicate create) error = %v, want ErrATMTerminalIDConflict", err)
	}
	var n int
	if err := dbtx(svc).QueryRow(ctx, `SELECT count(*) FROM atms WHERE terminal_id = $1`, term).Scan(&n); err != nil || n != 1 {
		t.Fatalf("atms with terminal_id %s = %d (err %v), want exactly 1", term, n, err)
	}
	var recorded *string
	if err := dbtx(svc).QueryRow(ctx, `SELECT error FROM master_data_change_requests WHERE id = $1`, changeID).Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if recorded == nil || !strings.Contains(*recorded, ErrATMTerminalIDConflict.Error()) {
		t.Errorf("recorded apply error should be the clean conflict message, got %v", recorded)
	}
}

// assignmentFixture seeds vendor -> branch -> package and returns an existing
// ATM id plus the package id. Test periods live in 2020, well clear of the real
// data (every existing assignment starts 2026-01-01, open-ended).
func assignmentFixture(t *testing.T, h *testHelpers, tag string) (atmID, pkgID int64) {
	t.Helper()
	vendorID := h.seedVendor("ITAS-"+tag, "Vendor For Assignment Test "+tag)
	branchID := h.seedVendorBranch(vendorID, "ITASB-"+tag, "Branch "+tag)
	pkgID = h.seedVendorPackage(branchID, "ITAS-P-"+tag)
	if err := h.pool.QueryRow(h.ctx, `SELECT id FROM atms WHERE deleted_at IS NULL ORDER BY id LIMIT 1`).Scan(&atmID); err != nil {
		t.Fatalf("picking an existing ATM: %v", err)
	}
	return atmID, pkgID
}

// approveAssignment stages a change and approves it, returning Approve's error.
func approveAssignment(t *testing.T, svc *MasterDataApprovalService, h *testHelpers, op string, entityID *int64, payload any) (changeID int64, err error) {
	t.Helper()
	changeID = h.insertPending("atm_assignment", op, entityID, payload)
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "approved"}}
	_, err = svc.Approve(context.Background(), 999, 1, "127.0.0.1")
	return changeID, err
}

func trackAssignment(t *testing.T, svc *MasterDataApprovalService, h *testHelpers, atmID, pkgID int64, start string) int64 {
	t.Helper()
	var id int64
	if err := dbtx(svc).QueryRow(context.Background(),
		`SELECT id FROM atm_vendor_packages WHERE atm_id = $1 AND vendor_package_id = $2 AND effective_start_date = $3::date`, atmID, pkgID, start).Scan(&id); err != nil {
		t.Fatalf("querying assignment starting %s: %v", start, err)
	}
	h.assignmentIDs = append(h.assignmentIDs, id)
	return id
}

func TestATMAssignmentApplier_Approve_CreateUpdateDisableEnable_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()
	atmID, pkgID := assignmentFixture(t, h, tag)

	if _, err := approveAssignment(t, svc, h, "create", nil, ATMAssignmentPayload{
		ATMID: atmID, ATMAssignmentUpdatePayload: ATMAssignmentUpdatePayload{VendorPackageID: pkgID, EffectiveStartDate: "2020-01-01", EffectiveEndDate: sp("2020-12-31")},
	}); err != nil {
		t.Fatalf("Approve(create) error = %v", err)
	}
	id := trackAssignment(t, svc, h, atmID, pkgID, "2020-01-01")

	var end string
	var active bool
	if err := dbtx(svc).QueryRow(ctx, `SELECT effective_end_date::text, is_active FROM atm_vendor_packages WHERE id = $1`, id).Scan(&end, &active); err != nil {
		t.Fatal(err)
	}
	if end != "2020-12-31" || !active {
		t.Errorf("after create: end=%s active=%v", end, active)
	}

	// Update: shorten the period, then make it open-ended (NULL end).
	if _, err := approveAssignment(t, svc, h, "update", &id, ATMAssignmentUpdatePayload{VendorPackageID: pkgID, EffectiveStartDate: "2020-01-01", EffectiveEndDate: sp("2020-06-30")}); err != nil {
		t.Fatalf("Approve(update) error = %v", err)
	}
	if err := dbtx(svc).QueryRow(ctx, `SELECT effective_end_date::text FROM atm_vendor_packages WHERE id = $1`, id).Scan(&end); err != nil || end != "2020-06-30" {
		t.Fatalf("after update: end=%s err=%v", end, err)
	}

	// Disable releases the period; Enable takes it back.
	if _, err := approveAssignment(t, svc, h, "disable", &id, nil); err != nil {
		t.Fatalf("Approve(disable) error = %v", err)
	}
	if err := dbtx(svc).QueryRow(ctx, `SELECT is_active FROM atm_vendor_packages WHERE id = $1`, id).Scan(&active); err != nil || active {
		t.Fatalf("after disable: active=%v err=%v", active, err)
	}
	if _, err := approveAssignment(t, svc, h, "enable", &id, nil); err != nil {
		t.Fatalf("Approve(enable) error = %v", err)
	}
	if err := dbtx(svc).QueryRow(ctx, `SELECT is_active FROM atm_vendor_packages WHERE id = $1`, id).Scan(&active); err != nil || !active {
		t.Fatalf("after enable: active=%v err=%v", active, err)
	}
}

// The exclusion constraint is the authoritative overlap guard: a request that
// passed submit-time checks (e.g. two overlapping requests pending at once)
// must fail at apply with a clean domain error, insert nothing, and record the
// failure on the change request.
func TestATMAssignmentApplier_Approve_OverlapAtApply_CleanErrorNoInsert_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := context.Background()
	atmID, pkgID := assignmentFixture(t, h, tag)

	if _, err := approveAssignment(t, svc, h, "create", nil, ATMAssignmentPayload{
		ATMID: atmID, ATMAssignmentUpdatePayload: ATMAssignmentUpdatePayload{VendorPackageID: pkgID, EffectiveStartDate: "2020-01-01", EffectiveEndDate: sp("2020-12-31")},
	}); err != nil {
		t.Fatalf("Approve(first create) error = %v", err)
	}
	firstID := trackAssignment(t, svc, h, atmID, pkgID, "2020-01-01")

	overlapping := ATMAssignmentPayload{ATMID: atmID, ATMAssignmentUpdatePayload: ATMAssignmentUpdatePayload{VendorPackageID: pkgID, EffectiveStartDate: "2020-06-01", EffectiveEndDate: sp("2021-06-01")}}
	changeID, err := approveAssignment(t, svc, h, "create", nil, overlapping)
	if !errors.Is(err, ErrATMAssignmentOverlap) {
		t.Fatalf("want ErrATMAssignmentOverlap, got %v", err)
	}
	var n int
	if err := dbtx(svc).QueryRow(ctx, `SELECT count(*) FROM atm_vendor_packages WHERE atm_id = $1 AND vendor_package_id = $2 AND effective_start_date = '2020-06-01'`, atmID, pkgID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("overlapping row must not be inserted: n=%d err=%v", n, err)
	}
	var recorded *string
	if err := dbtx(svc).QueryRow(ctx, `SELECT error FROM master_data_change_requests WHERE id = $1`, changeID).Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if recorded == nil || strings.Contains(*recorded, "atm_vendor_packages_no_overlap") || !strings.Contains(*recorded, "tumpang tindih") {
		t.Errorf("recorded apply error should be the clean message, got %v", recorded)
	}

	// Open-ended request starting inside the closed period overlaps too (NULL end = unbounded).
	_, err = approveAssignment(t, svc, h, "create", nil, ATMAssignmentPayload{ATMID: atmID, ATMAssignmentUpdatePayload: ATMAssignmentUpdatePayload{VendorPackageID: pkgID, EffectiveStartDate: "2020-12-31"}})
	if !errors.Is(err, ErrATMAssignmentOverlap) {
		t.Errorf("open-ended request touching the last day (inclusive end) must overlap, got %v", err)
	}

	// A later, non-overlapping period is fine (kept clear of the 2020-06-01..2021-06-01
	// request re-applied below).
	if _, err := approveAssignment(t, svc, h, "create", nil, ATMAssignmentPayload{ATMID: atmID, ATMAssignmentUpdatePayload: ATMAssignmentUpdatePayload{VendorPackageID: pkgID, EffectiveStartDate: "2022-01-01", EffectiveEndDate: sp("2022-12-31")}}); err != nil {
		t.Fatalf("non-overlapping period should apply, got %v", err)
	}
	trackAssignment(t, svc, h, atmID, pkgID, "2022-01-01")

	// Once the first assignment is disabled its period is released.
	if _, err := approveAssignment(t, svc, h, "disable", &firstID, nil); err != nil {
		t.Fatalf("Approve(disable) error = %v", err)
	}
	if _, err := approveAssignment(t, svc, h, "create", nil, overlapping); err != nil {
		t.Fatalf("after disabling the blocker the same request should apply, got %v", err)
	}
	trackAssignment(t, svc, h, atmID, pkgID, "2020-06-01")

	// Re-enabling the disabled one now collides with the new occupant.
	if _, err := approveAssignment(t, svc, h, "enable", &firstID, nil); !errors.Is(err, ErrATMAssignmentOverlap) {
		t.Errorf("re-enable into an occupied period must overlap, got %v", err)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// jsonOrNil marshals v, or returns nil if v is nil (Before is optional).
func jsonOrNil(t *testing.T, v any) []byte {
	if v == nil {
		return nil
	}
	return mustJSON(t, v)
}

// dbtx exposes the harness's underlying pool for assertion queries (mirrors
// rolemgmt's queryRowScan helper's svc.pool.(db.DBTX) type assertion).
func dbtx(svc *MasterDataApprovalService) db.DBTX {
	return svc.pool.(db.DBTX)
}

// A change approved but not applied (transient failure) is re-applied by RetryApply;
// a second retry is refused because the change is then 'applied'.
func TestMasterDataApprovalService_RetryApply_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	ctx := adminCtx(h.makerID)

	changeID := h.insertPending("vendor", "create", nil, vendorCreatePayload{Code: "RT-" + tag, Name: "Retry Vendor " + tag})
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "approved"}}

	registry := svc.appliers
	svc.appliers = ApplierRegistry{} // simulate the apply failing
	if _, err := svc.Approve(ctx, 999, h.makerID, "127.0.0.1"); err == nil {
		t.Fatal("expected apply failure")
	}
	svc.appliers = registry

	if err := svc.RetryApply(context.Background(), changeID, h.makerID, "127.0.0.1"); !errors.Is(err, ErrMasterDataForbidden) {
		t.Fatalf("RetryApply without auth = %v, want ErrMasterDataForbidden", err)
	}
	if err := svc.RetryApply(ctx, changeID, h.makerID, "127.0.0.1"); err != nil {
		t.Fatalf("RetryApply() error = %v", err)
	}
	var vendorID int64
	if err := h.pool.QueryRow(ctx, `SELECT id FROM vendors WHERE code = $1`, "RT-"+tag).Scan(&vendorID); err != nil {
		t.Fatalf("load applied vendor: %v", err)
	}
	h.vendorIDs = append(h.vendorIDs, vendorID) // cleanup deletes it
	change, err := svc.repo.GetByID(ctx, changeID)
	if err != nil || change.Status != "applied" {
		t.Fatalf("status = %q err = %v, want applied", change.Status, err)
	}
	if err := svc.RetryApply(ctx, changeID, h.makerID, "127.0.0.1"); !errors.Is(err, ErrMasterDataNotRetryable) {
		t.Fatalf("second RetryApply = %v, want ErrMasterDataNotRetryable", err)
	}
}
