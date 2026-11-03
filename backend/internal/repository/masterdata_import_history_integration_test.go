//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Import-history queries (master-data-import-revamp 1.3). Everything runs in the
// integrationTx transaction, which is rolled back on cleanup: nothing is left in
// the dev database. The batches use a private entity name so rows already in
// the database never show up in the filtered assertions.
const historyEntity = "itest_history_entity"

// historyRollbackPrefix mirrors service.BatchRollbackPrefix (repository must not import service).
const historyRollbackPrefix = "batch rolled back: "

type historyFixture struct {
	t      *testing.T
	ctx    context.Context
	tx     pgx.Tx
	repo   *MasterDataImportBatchRepository
	makerA int64
	makerB int64
	nameB  string
	seq    int64
}

func newHistoryFixture(t *testing.T) *historyFixture {
	t.Helper()
	tx := integrationTx(t)
	ctx := context.Background()
	rows, err := tx.Query(ctx, `SELECT id, full_name FROM users ORDER BY id LIMIT 2`)
	if err != nil {
		t.Fatalf("select users: %v", err)
	}
	type user struct {
		id   int64
		name string
	}
	var us []user
	for rows.Next() {
		var u user
		if err := rows.Scan(&u.id, &u.name); err != nil {
			t.Fatal(err)
		}
		us = append(us, u)
	}
	rows.Close()
	if len(us) < 2 {
		t.Skip("need two seeded users")
	}
	return &historyFixture{t: t, ctx: ctx, tx: tx, repo: NewMasterDataImportBatchRepository(tx, tx),
		makerA: us[0].id, makerB: us[1].id, nameB: us[1].name}
}

func historyPtr[T any](v T) *T { return &v }

// batch stages a batch of n update rows by maker with a unique hash.
func (f *historyFixture) batch(maker int64, n int, fileName *string) *ImportBatch {
	f.t.Helper()
	f.seq++
	rows := make([]db.CreateMasterDataChangeRequestParams, n)
	for i := range rows {
		id := f.seq*1000 + int64(i)
		rows[i] = db.CreateMasterDataChangeRequestParams{EntityType: historyEntity, EntityID: &id, Op: "update", Payload: []byte(`{}`), MakerID: maker}
	}
	b, existing, err := f.repo.Create(f.ctx, historyEntity, fmt.Sprintf("%064d", f.seq), fileName, maker, rows)
	if err != nil || existing {
		f.t.Fatalf("Create batch = %+v, %v, %v", b, existing, err)
	}
	return b
}

func (f *historyFixture) setHead(batch *ImportBatch, status string, errText *string) {
	f.t.Helper()
	if _, err := f.tx.Exec(f.ctx, `UPDATE master_data_change_requests SET status = $1, error = $2 WHERE id = $3`, status, errText, batch.HeadID); err != nil {
		f.t.Fatalf("set head status: %v", err)
	}
}

func (f *historyFixture) approval(batch *ImportBatch) int64 {
	f.t.Helper()
	var id int64
	if err := f.tx.QueryRow(f.ctx, `INSERT INTO approval_requests (maker_id, document_type, document_id, amount, required_level, status)
		VALUES ($1, 'master_data_change', $2, 0, 2, 'pending') RETURNING id`, batch.MakerID, batch.HeadID).Scan(&id); err != nil {
		f.t.Fatalf("seed approval: %v", err)
	}
	if err := f.repo.SetApprovalRequestID(f.ctx, batch.ID, id); err != nil {
		f.t.Fatal(err)
	}
	return id
}

func (f *historyFixture) step(requestID int64, level int, assigned int64, status string, actedBy *int64, actedAt *time.Time) {
	f.t.Helper()
	if _, err := f.tx.Exec(f.ctx, `INSERT INTO approval_steps (request_id, step_level, assigned_approver_id, acted_by_id, status, acted_at)
		VALUES ($1, $2, $3, $4, $5, $6)`, requestID, level, assigned, actedBy, status, actedAt); err != nil {
		f.t.Fatalf("seed step: %v", err)
	}
}

func (f *historyFixture) list(fl ImportBatchFilter) ([]db.ListMasterDataImportBatchesRow, int64) {
	f.t.Helper()
	fl.Entity = historyPtr(historyEntity)
	if fl.Limit == 0 {
		fl.Limit = 100
	}
	rows, total, err := f.repo.ListBatches(f.ctx, fl)
	if err != nil {
		f.t.Fatalf("ListBatches: %v", err)
	}
	return rows, total
}

func TestImportHistory_StatusIsDerivedFromHeadRow(t *testing.T) {
	f := newHistoryFixture(t)
	pending := f.batch(f.makerA, 2, historyPtr("pending.csv"))
	stale := f.batch(f.makerA, 2, historyPtr("stale.csv"))
	f.setHead(stale, "stale", historyPtr(historyRollbackPrefix+"x"))
	// A non-head row with another status must not leak into the batch status.
	if _, err := f.tx.Exec(f.ctx, `UPDATE master_data_change_requests SET status = 'applied' WHERE batch_id = $1 AND id <> $2`, stale.ID, stale.HeadID); err != nil {
		t.Fatal(err)
	}

	rows, total := f.list(ImportBatchFilter{})
	if total != 2 || len(rows) != 2 {
		t.Fatalf("total=%d rows=%d, want 2/2", total, len(rows))
	}
	if rows[0].ID != stale.ID || rows[1].ID != pending.ID {
		t.Fatalf("order = %d,%d want newest first %d,%d", rows[0].ID, rows[1].ID, stale.ID, pending.ID)
	}
	if rows[0].HeadStatus != "stale" || rows[1].HeadStatus != "pending" {
		t.Fatalf("status = %q,%q want stale,pending", rows[0].HeadStatus, rows[1].HeadStatus)
	}
	if rows[0].HeadError == nil || rows[1].HeadError != nil {
		t.Fatalf("head errors = %v,%v", rows[0].HeadError, rows[1].HeadError)
	}
	if rows[1].FileName == nil || *rows[1].FileName != "pending.csv" || rows[1].RowCount != 2 {
		t.Fatalf("file/rows = %v/%d", rows[1].FileName, rows[1].RowCount)
	}

	got, err := f.repo.GetBatch(f.ctx, stale.ID)
	if err != nil || got.HeadStatus != "stale" || got.ID != stale.ID {
		t.Fatalf("GetBatch = %+v, %v", got, err)
	}
	if _, err := f.repo.GetBatch(f.ctx, -1); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("GetBatch(unknown) err = %v, want pgx.ErrNoRows", err)
	}
}

func TestImportHistory_DecidedFromLatestActedStep(t *testing.T) {
	f := newHistoryFixture(t)
	undecided := f.batch(f.makerA, 1, nil)
	decided := f.batch(f.makerA, 1, nil)

	// undecided: only a pending step -> no decided_* values.
	f.step(f.approval(undecided), 1, f.makerB, "pending", nil, nil)

	// decided: step 1 acted earlier by A, step 2 acted later by B (latest wins); step 3 still pending.
	req := f.approval(decided)
	early, late := time.Now().Add(-2*time.Hour), time.Now().Add(-1*time.Hour)
	f.step(req, 1, f.makerA, "approved", &f.makerA, &early)
	f.step(req, 2, f.makerB, "approved", &f.makerB, &late)
	f.step(req, 3, f.makerA, "pending", nil, nil)
	f.setHead(decided, "applied", nil)

	rows, _ := f.list(ImportBatchFilter{})
	byID := map[int64]db.ListMasterDataImportBatchesRow{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	if r := byID[undecided.ID]; r.DecidedAt.Valid || r.DecidedByName != nil {
		t.Fatalf("undecided batch has decided_* = %v / %v", r.DecidedAt, r.DecidedByName)
	}
	r := byID[decided.ID]
	if !r.DecidedAt.Valid || r.DecidedAt.Time.Sub(late).Abs() > time.Millisecond {
		t.Fatalf("decided_at = %v, want %v", r.DecidedAt, late)
	}
	if r.DecidedByName == nil || *r.DecidedByName != f.nameB {
		t.Fatalf("decided_by_name = %v, want %q", r.DecidedByName, f.nameB)
	}
	if one, err := f.repo.GetBatch(f.ctx, decided.ID); err != nil || one.DecidedByName == nil || *one.DecidedByName != f.nameB {
		t.Fatalf("GetBatch decided = %+v, %v", one, err)
	}
}

func TestImportHistory_FiltersAndPaging(t *testing.T) {
	f := newHistoryFixture(t)
	a1 := f.batch(f.makerA, 1, nil)
	a2 := f.batch(f.makerA, 1, nil)
	b1 := f.batch(f.makerB, 1, nil)
	f.setHead(a2, "rejected", nil)

	if _, total := f.list(ImportBatchFilter{}); total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	if rows, total := f.list(ImportBatchFilter{MakerID: &f.makerB}); total != 1 || len(rows) != 1 || rows[0].ID != b1.ID {
		t.Fatalf("maker filter = %d rows, total %d", len(rows), total)
	}
	if rows, total := f.list(ImportBatchFilter{Status: historyPtr("rejected")}); total != 1 || rows[0].ID != a2.ID {
		t.Fatalf("status filter = %d rows, total %d", len(rows), total)
	}
	if rows, total := f.list(ImportBatchFilter{Status: historyPtr("pending"), MakerID: &f.makerA}); total != 1 || rows[0].ID != a1.ID {
		t.Fatalf("status+maker filter = %d rows, total %d", len(rows), total)
	}
	if _, total := f.list(ImportBatchFilter{Status: historyPtr("applied")}); total != 0 {
		t.Fatalf("applied total = %d, want 0", total)
	}
	other := "itest_history_other"
	if rows, total, err := f.repo.ListBatches(f.ctx, ImportBatchFilter{Entity: &other, Limit: 10}); err != nil || total != 0 || len(rows) != 0 {
		t.Fatalf("other entity = %d rows, total %d, %v", len(rows), total, err)
	}
	// Paging: total is the filtered count, not the page size; newest first.
	p1, total := f.list(ImportBatchFilter{Limit: 2})
	p2, _ := f.list(ImportBatchFilter{Limit: 2, Offset: 2})
	if total != 3 || len(p1) != 2 || len(p2) != 1 || p1[0].ID != b1.ID || p2[0].ID != a1.ID {
		t.Fatalf("paging p1=%d p2=%d total=%d", len(p1), len(p2), total)
	}
}

func TestImportHistory_FileNameCheckConstraint(t *testing.T) {
	f := newHistoryFixture(t)
	f.batch(f.makerA, 1, historyPtr(strings.Repeat("x", 255))) // boundary accepted
	f.batch(f.makerA, 1, nil)                                  // NULL accepted
	for name, v := range map[string]string{"empty": "", "256 chars": strings.Repeat("x", 256)} {
		f.seq++
		id := f.seq * 1000
		_, _, err := f.repo.Create(f.ctx, historyEntity, fmt.Sprintf("%064d", f.seq), historyPtr(v), f.makerA,
			[]db.CreateMasterDataChangeRequestParams{{EntityType: historyEntity, EntityID: &id, Op: "update", Payload: []byte(`{}`), MakerID: f.makerA}})
		if err == nil || !strings.Contains(err.Error(), "master_data_import_batches_file_name_chk") {
			t.Fatalf("%s file name: err = %v, want file_name CHECK violation", name, err)
		}
	}
}

func TestImportHistory_ListPendingApproversUsesLowestPendingStep(t *testing.T) {
	f := newHistoryFixture(t)
	b := f.batch(f.makerA, 1, nil)
	req := f.approval(b)
	now := time.Now()
	f.step(req, 1, f.makerA, "approved", &f.makerA, &now)
	f.step(req, 2, f.makerB, "pending", nil, nil)
	f.step(req, 3, f.makerA, "pending", nil, nil)

	got, err := f.repo.ListPendingApprovers(f.ctx, req)
	if err != nil || len(got) != 1 || got[0] != f.makerB {
		t.Fatalf("pending approvers = %v, %v, want [%d]", got, err, f.makerB)
	}
}

func TestImportHistory_ListByBatchOrderedByID(t *testing.T) {
	f := newHistoryFixture(t)
	b := f.batch(f.makerA, 3, nil)
	rows, err := f.repo.ListByBatch(f.ctx, b.ID)
	if err != nil || len(rows) != 3 || rows[0].ID != b.HeadID || rows[0].ID >= rows[1].ID || rows[1].ID >= rows[2].ID {
		t.Fatalf("ListByBatch = %d rows, %v", len(rows), err)
	}
}
