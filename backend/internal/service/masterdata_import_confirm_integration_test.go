//go:build integration

package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/repository"
)

// insertingOrchestrator stands in for approval.Orchestrator on the SUBMIT side:
// it inserts a real approval_requests row (change_requests.approval_request_id
// is a foreign key) and remembers its id for cleanup.
type insertingOrchestrator struct {
	pool  *pgxpool.Pool
	ids   []int64
	calls int
}

func (o *insertingOrchestrator) SubmitForApproval(ctx context.Context, makerID int64, documentType string, documentID int64, _ pgtype.Numeric, _ string) (db.ApprovalRequest, bool, error) {
	o.calls++
	var id int64
	err := o.pool.QueryRow(ctx, `INSERT INTO approval_requests (maker_id, document_type, document_id, amount, required_level, status)
		VALUES ($1, $2, $3, 0, 1, 'pending') RETURNING id`, makerID, documentType, documentID).Scan(&id)
	if err != nil {
		return db.ApprovalRequest{}, false, err
	}
	o.ids = append(o.ids, id)
	return db.ApprovalRequest{ID: id, DocumentType: documentType, DocumentID: documentID}, false, nil
}

type importFixture struct {
	t        *testing.T
	ctx      context.Context
	h        *testHelpers
	svc      *MasterDataApprovalService
	importer *MasterDataImporter
	orch     *insertingOrchestrator
	tag      string
	batches  []int64
}

// newImportFixture wires the importer as production does (real repos on the
// pool) and registers a cleanup that removes everything the test staged/applied.
func newImportFixture(t *testing.T) *importFixture {
	svc, tag, h := harness(t)
	f := &importFixture{t: t, h: h, svc: svc, tag: tag, orch: &insertingOrchestrator{pool: h.pool}}
	f.ctx = adminCtx(h.makerID)
	repo := repository.NewMasterDataExportRepository(h.pool)
	f.importer = NewMasterDataImporter(repo).WithConfirm(ImportConfirmDeps{
		Batches:      repository.NewMasterDataImportBatchRepository(h.pool, h.pool),
		Orchestrator: f.orch,
		Audit:        audit.NewWriter(h.pool),
		Services: func(sub ImportSubmitter) ImportServices {
			return ImportServices{Vendors: NewVendorAdminService(repository.NewVendorAdminRepository(h.pool, h.pool), sub)}
		},
	})
	// Registered after harness's cleanup, so it runs BEFORE it: batches and their
	// approvals must go before the rows harness deletes.
	t.Cleanup(f.cleanup)
	return f
}

func (f *importFixture) cleanup() {
	ctx, pool := context.Background(), f.h.pool
	exec := func(sql string, args ...any) {
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			f.t.Logf("cleanup: %v", err)
		}
	}
	exec(`DELETE FROM audit_logs WHERE entity_type = 'vendor' AND entity_id IN (SELECT id FROM vendors WHERE code LIKE $1)`, "IT-"+f.tag+"%")
	exec(`DELETE FROM audit_logs WHERE entity_type = 'master_data_change_request' AND entity_id IN (SELECT id FROM master_data_change_requests WHERE batch_id = ANY($1))`, f.batches)
	exec(`DELETE FROM audit_logs WHERE entity_type = 'master_data_import_batch' AND entity_id = ANY($1)`, f.batches)
	exec(`DELETE FROM master_data_change_requests WHERE batch_id = ANY($1)`, f.batches)
	exec(`DELETE FROM approval_requests WHERE id = ANY($1)`, f.orch.ids)
	exec(`DELETE FROM master_data_import_batches WHERE id = ANY($1)`, f.batches)
	exec(`DELETE FROM vendors WHERE code LIKE $1`, "IT-"+f.tag+"%")
}

func (f *importFixture) confirm(csv string) *ImportConfirmResult {
	f.t.Helper()
	res, err := f.importer.Confirm(f.ctx, f.h.makerID, ExportVendors, "test.csv", strings.NewReader(csv), "127.0.0.1")
	if err != nil {
		f.t.Fatalf("Confirm: %v", err)
	}
	f.batches = append(f.batches, res.BatchID)
	return res
}

func (f *importFixture) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	if err := f.h.pool.QueryRow(f.ctx, sql, args...).Scan(&n); err != nil {
		f.t.Fatalf("count: %v", err)
	}
	return n
}

func (f *importFixture) vendorsCSV(codes ...string) string {
	var sb strings.Builder
	sb.WriteString(vendorHeader)
	for _, c := range codes {
		sb.WriteString(",IT-" + f.tag + c + ",Vendor " + c + ",,,,,,\n")
	}
	return sb.String()
}

func (f *importFixture) head(batchID int64) int64 {
	f.t.Helper()
	var head int64
	if err := f.h.pool.QueryRow(f.ctx, `SELECT min(id) FROM master_data_change_requests WHERE batch_id = $1`, batchID).Scan(&head); err != nil {
		f.t.Fatal(err)
	}
	return head
}

// approve runs the real apply-on-approve hook for the batch's single approval.
func (f *importFixture) approve(res *ImportConfirmResult) error {
	f.svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: f.head(res.BatchID), Status: "approved"}}
	_, err := f.svc.Approve(f.ctx, *res.ApprovalRequestID, f.h.makerID, "127.0.0.1")
	return err
}

func TestMasterDataImportConfirm_OneBatchOneApproval_IdempotentPerFile_AppliedAtomically_Integration(t *testing.T) {
	f := newImportFixture(t)
	csv := f.vendorsCSV("A", "B", "C")

	res := f.confirm(csv)

	if res.Existing || res.Rows != 3 || res.ApprovalRequestID == nil {
		t.Fatalf("first confirm = %+v, want a new 3-row batch with an approval", res)
	}
	if n := f.count(`SELECT count(*) FROM master_data_change_requests WHERE batch_id = $1 AND status = 'pending' AND approval_request_id = $2`, res.BatchID, *res.ApprovalRequestID); n != 3 {
		t.Errorf("%d rows share the batch's single approval, want 3", n)
	}
	if n := f.count(`SELECT count(*) FROM vendors WHERE code LIKE $1`, "IT-"+f.tag+"%"); n != 0 {
		t.Fatalf("%d vendors exist before approval: confirm must only stage", n)
	}

	again := f.confirm(csv)
	if !again.Existing || again.BatchID != res.BatchID || f.orch.calls != 1 {
		t.Errorf("same file again = %+v (approval calls %d), want the existing batch and no second approval", again, f.orch.calls)
	}
	if n := f.count(`SELECT count(*) FROM master_data_change_requests WHERE batch_id = $1`, res.BatchID); n != 3 {
		t.Errorf("%d change requests after re-upload, want still 3", n)
	}

	if err := f.approve(res); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if n := f.count(`SELECT count(*) FROM vendors WHERE code LIKE $1`, "IT-"+f.tag+"%"); n != 3 {
		t.Errorf("%d vendors after approval, want all 3 applied", n)
	}
	if n := f.count(`SELECT count(*) FROM master_data_change_requests WHERE batch_id = $1 AND status = 'applied'`, res.BatchID); n != 3 {
		t.Errorf("%d rows applied, want 3", n)
	}

	// After apply the file no longer validates ("already exists"), but the same file still answers "done".
	if done := f.confirm(csv); !done.Existing || done.BatchID != res.BatchID {
		t.Errorf("re-upload after apply = %+v, want the existing batch", done)
	}
}

func TestMasterDataImportConfirm_OneBadRowRollsBackTheWholeBatch_Integration(t *testing.T) {
	f := newImportFixture(t)
	csv := f.vendorsCSV("A", "B")
	res := f.confirm(csv)

	// Between confirm and approval someone creates B directly: applying row B must fail...
	f.h.seedVendor("IT-"+f.tag+"B", "Sneaked in")

	err := f.approve(res)

	if !errors.Is(err, ErrVendorCodeConflict) {
		t.Fatalf("approve error = %v, want ErrVendorCodeConflict", err)
	}
	// ...and row A, which applied fine on its own, must NOT stay behind.
	if n := f.count(`SELECT count(*) FROM vendors WHERE code = $1`, "IT-"+f.tag+"A"); n != 0 {
		t.Errorf("vendor A exists: a failed batch must apply all rows or none")
	}
	if n := f.count(`SELECT count(*) FROM master_data_change_requests WHERE batch_id = $1 AND status = 'applied'`, res.BatchID); n != 0 {
		t.Errorf("%d rows marked applied after a rolled-back batch", n)
	}
	if n := f.count(`SELECT count(*) FROM master_data_change_requests WHERE batch_id = $1 AND error IS NOT NULL`, res.BatchID); n == 0 {
		t.Errorf("the failure was not recorded on any row")
	}
	// The batch is retired (stale), so the same file is no longer treated as "open"
	// and can be uploaded again once the cause is fixed.
	if n := f.count(`SELECT count(*) FROM master_data_change_requests WHERE batch_id = $1 AND status <> 'stale'`, res.BatchID); n != 0 {
		t.Errorf("%d rows of a rolled-back batch are not stale", n)
	}
	if open, err := f.importer.confirm.Batches.FindOpen(context.Background(), "vendors", fmt.Sprintf("%x", sha256.Sum256([]byte(csv)))); err != nil || open != nil {
		t.Errorf("FindOpen after rollback = %v, %v; want nil", open, err)
	}
}

// A row that fails WITHOUT an SQL error (stale) leaves the apply tx alive and
// holding the row locks of the rows before it. Marking those rows stale through
// the pool must not wait on that tx (bugfixes.md 2026-11-02).
func TestMasterDataImportConfirm_MiddleRowStale_ReturnsPromptlyAndRetiresWholeBatch_Integration(t *testing.T) {
	f := newImportFixture(t)
	res := f.confirm(f.vendorsCSV("A", "B", "C"))

	rows, err := f.svc.repo.ListByBatch(f.ctx, res.BatchID)
	if err != nil || len(rows) != 3 {
		t.Fatalf("ListByBatch = %d rows, %v; want 3", len(rows), err)
	}
	head, middle, last := rows[0].ID, rows[1].ID, rows[2].ID
	bystander := f.h.seedVendor("IT-"+f.tag+"Z", "Bystander")
	if _, err := f.h.pool.Exec(f.ctx, `UPDATE master_data_change_requests SET op = 'update', entity_id = $2, before = '{"name":"not the current state"}' WHERE id = $1`, middle, bystander); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(f.ctx, 3*time.Second)
	defer cancel()
	f.svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: head, Status: "approved"}}
	_, err = f.svc.Approve(ctx, *res.ApprovalRequestID, f.h.makerID, "127.0.0.1")

	if ctx.Err() != nil {
		t.Errorf("Approve waited until its context expired: the rollback bookkeeping blocked on the apply tx")
	}
	if !errors.Is(err, ErrMasterDataChangeStale) {
		t.Errorf("approve error = %v, want ErrMasterDataChangeStale", err)
	}
	if n := f.count(`SELECT count(*) FROM vendors WHERE code = ANY($1)`, []string{"IT-" + f.tag + "A", "IT-" + f.tag + "B", "IT-" + f.tag + "C"}); n != 0 {
		t.Errorf("%d vendors exist: a failed batch must apply no row", n)
	}
	if n := f.count(`SELECT count(*) FROM vendors WHERE id = $1 AND name = 'Bystander'`, bystander); n != 1 {
		t.Errorf("the stale row's target vendor changed")
	}

	wantErr := map[int64]string{
		head:   fmt.Sprintf("%schange request %d failed", BatchRollbackPrefix, middle),
		middle: ErrMasterDataChangeStale.Error(),
		last:   fmt.Sprintf("%schange request %d failed", BatchRollbackPrefix, middle),
	}
	for id, want := range wantErr {
		c, err := f.svc.repo.GetByID(f.ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		got := "<nil>"
		if c.Error != nil {
			got = *c.Error
		}
		if c.Status != "stale" || got != want {
			t.Errorf("row %d = status %q error %q, want stale %q", id, c.Status, got, want)
		}
	}
}

// unmarshalableAfterApplier succeeds but returns an after-state the audit writer
// cannot encode: an audit failure without an SQL error, after the change row was
// already flipped to 'applied' inside the tx.
type unmarshalableAfterApplier struct{ VendorApplier }

func (unmarshalableAfterApplier) Apply(context.Context, pgx.Tx, db.MasterDataChangeRequest) (int64, any, error) {
	return 1, make(chan int), nil
}

// Same lock, single-record path: recording the failure on the row the tx itself
// updated must not wait on that tx.
func TestMasterDataApproval_AuditEncodeFailure_RecordsErrorWithoutBlocking_Integration(t *testing.T) {
	svc, _, h := harness(t)
	svc.appliers["vendor"] = unmarshalableAfterApplier{}
	changeID := h.insertPending("vendor", "create", nil, map[string]string{"code": "unused"})

	ctx, cancel := context.WithTimeout(adminCtx(h.makerID), 3*time.Second)
	defer cancel()
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "approved"}}
	_, err := svc.Approve(ctx, 1, h.makerID, "127.0.0.1")

	if ctx.Err() != nil {
		t.Errorf("Approve waited until its context expired: MarkApplyFailed blocked on the apply tx")
	}
	if err == nil {
		t.Fatalf("Approve error = nil, want the audit failure")
	}
	c, err := svc.repo.GetByID(context.Background(), changeID)
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != "approved" || c.Error == nil || *c.Error == "" {
		t.Errorf("change = status %q error %v, want approved with the error recorded", c.Status, c.Error)
	}
}

func TestMasterDataImportConfirm_RejectRejectsWholeBatch_AndFileMayBeReuploaded_Integration(t *testing.T) {
	f := newImportFixture(t)
	csv := f.vendorsCSV("A", "B")
	res := f.confirm(csv)

	f.svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: f.head(res.BatchID), Status: "rejected"}}
	if _, err := f.svc.Reject(f.ctx, *res.ApprovalRequestID, f.h.makerID, "127.0.0.1"); err != nil {
		t.Fatalf("reject: %v", err)
	}

	if n := f.count(`SELECT count(*) FROM master_data_change_requests WHERE batch_id = $1 AND status = 'rejected'`, res.BatchID); n != 2 {
		t.Errorf("%d rows rejected, want the whole batch (2)", n)
	}
	retry := f.confirm(csv)
	if retry.Existing || retry.BatchID == res.BatchID {
		t.Errorf("re-upload after rejection = %+v, want a NEW batch", retry)
	}
}

func TestMasterDataImportConfirm_InvalidFileStagesNothing_NoChangesIsAnError_Integration(t *testing.T) {
	f := newImportFixture(t)
	bad := vendorHeader + ",IT-" + f.tag + "X,,,,,,,\n" // name missing

	_, err := f.importer.Confirm(f.ctx, f.h.makerID, ExportVendors, "test.csv", strings.NewReader(bad), "127.0.0.1")

	var invalid *ImportInvalidError
	if !errors.As(err, &invalid) || len(invalid.Errors) == 0 {
		t.Fatalf("want ImportInvalidError with the row errors, got %v", err)
	}
	if f.orch.calls != 0 {
		t.Errorf("an invalid file must not reach approval")
	}

	// A file whose rows all equal current data has nothing to stage.
	var buf strings.Builder
	exp := NewMasterDataExporter(repository.NewMasterDataExportRepository(f.h.pool))
	if _, err := exp.Write(f.ctx, ExportVendors, "all", &buf, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := f.importer.Confirm(f.ctx, f.h.makerID, ExportVendors, "test.csv", strings.NewReader(buf.String()), "127.0.0.1"); !errors.Is(err, ErrImportNothingToDo) {
		t.Errorf("unchanged export: want ErrImportNothingToDo, got %v", err)
	}
}

func TestMasterDataImportConfirm_RequiresMakerIdentity_Integration(t *testing.T) {
	f := newImportFixture(t)

	_, err := f.importer.Confirm(adminCtx(f.h.makerID+1), f.h.makerID, ExportVendors, "test.csv", strings.NewReader(f.vendorsCSV("A")), "127.0.0.1")

	if !errors.Is(err, ErrMasterDataForbidden) {
		t.Fatalf("want ErrMasterDataForbidden when the token user is not the maker, got %v", err)
	}
}
