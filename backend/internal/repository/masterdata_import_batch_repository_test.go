//go:build integration

package repository

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// integrationTx opens a transaction on DATABASE_URL that is rolled back on cleanup.
func integrationTx(t *testing.T) pgx.Tx {
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
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	return tx
}

// CSV import = one batch, one approval, all-or-nothing, idempotent per file
// hash (CLAUDE.md Sec 12). The tx doubles as the batch "pool": Begin opens a
// savepoint, so a failed batch is rolled back without touching the outer tx.
func TestMasterDataImportBatchRepository(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()
	var makerID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM users ORDER BY id LIMIT 1`).Scan(&makerID); err != nil {
		t.Skipf("no seeded user: %v", err)
	}
	repo := NewMasterDataImportBatchRepository(tx, tx)
	changes := NewMasterDataChangeRepository(tx)
	const entity = "itest_entity"
	hashA, hashB := strings.Repeat("a", 64), strings.Repeat("b", 64)
	row := func(entityID int64) db.CreateMasterDataChangeRequestParams {
		return db.CreateMasterDataChangeRequestParams{EntityType: entity, EntityID: &entityID, Op: "update", Payload: []byte(`{}`), MakerID: makerID}
	}

	if b, err := repo.FindOpen(ctx, entity, hashA); err != nil || b != nil {
		t.Fatalf("FindOpen before create = %v, %v", b, err)
	}
	batch, existing, err := repo.Create(ctx, entity, hashA, makerID, []db.CreateMasterDataChangeRequestParams{row(1), row(2)})
	if err != nil || existing || batch.RowCount != 2 || batch.HeadID == 0 {
		t.Fatalf("Create = %+v, %v, %v", batch, existing, err)
	}
	staged, err := changes.ListByBatch(ctx, batch.ID)
	if err != nil || len(staged) != 2 || staged[0].ID != batch.HeadID {
		t.Fatalf("staged rows = %+v, %v (head %d)", staged, err, batch.HeadID)
	}

	// Same file again: the open batch comes back, nothing new is written.
	again, existing, err := repo.Create(ctx, entity, hashA, makerID, []db.CreateMasterDataChangeRequestParams{row(3)})
	if err != nil || !existing || again.ID != batch.ID {
		t.Fatalf("re-upload = %+v, %v, %v, want existing batch %d", again, existing, err, batch.ID)
	}
	if id, _ := changes.FindPending(ctx, entity, 3); id != nil {
		t.Fatal("re-upload staged a new row")
	}
	if found, err := repo.FindOpen(ctx, entity, hashA); err != nil || found == nil || found.ID != batch.ID || found.HeadID != batch.HeadID {
		t.Fatalf("FindOpen = %+v, %v", found, err)
	}

	// One approval for the whole batch.
	var approvalID int64
	if err := tx.QueryRow(ctx, `INSERT INTO approval_requests (maker_id, document_type, document_id, amount, required_level, status)
		VALUES ($1, 'master_data_change', $2, 0, 1, 'pending') RETURNING id`, makerID, batch.HeadID).Scan(&approvalID); err != nil {
		t.Fatalf("seed approval: %v", err)
	}
	if err := repo.SetApprovalRequestID(ctx, batch.ID, approvalID); err != nil {
		t.Fatal(err)
	}
	staged, _ = changes.ListByBatch(ctx, batch.ID)
	for _, c := range staged {
		if c.ApprovalRequestID == nil || *c.ApprovalRequestID != approvalID {
			t.Fatalf("change %d approval = %v, want %d", c.ID, c.ApprovalRequestID, approvalID)
		}
	}

	// A row colliding with a pending change fails the whole batch and names the row.
	_, _, err = repo.Create(ctx, entity, hashB, makerID, []db.CreateMasterDataChangeRequestParams{row(9), row(2)})
	var rowErr *BatchRowError
	if !errors.As(err, &rowErr) || rowErr.BatchRowIndex() != 1 || rowErr.Unwrap() == nil || !strings.Contains(rowErr.Error(), "batch row 1") {
		t.Fatalf("colliding batch err = %v, want BatchRowError index 1", err)
	}
	if b, err := repo.FindOpen(ctx, entity, hashB); err != nil || b != nil {
		t.Fatalf("failed batch left behind: %v, %v", b, err)
	}
	if id, _ := changes.FindPending(ctx, entity, 9); id != nil {
		t.Fatal("row 0 of the failed batch was kept")
	}
}
