package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// ImportBatchPool is what the batch repository needs from *pgxpool.Pool: the
// staging of a whole import file must be ONE transaction (all rows or none).
type ImportBatchPool interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// ImportBatch is a staged import file: its id, its first change request
// (HeadID, the approval's document_id) and the approval request once submitted
// (nil until then). It is the sqlc row type so the service layer can name it
// without importing this package.
type ImportBatch = db.FindOpenMasterDataImportBatchRow

// BatchRowError says which staged row (index into the rows passed to Create)
// violated the pending-change unique index.
type BatchRowError struct {
	Index int
	Err   error
}

func (e *BatchRowError) Error() string { return fmt.Sprintf("batch row %d: %v", e.Index, e.Err) }
func (e *BatchRowError) Unwrap() error { return e.Err }

// BatchRowIndex lets the service layer find the offending row without importing this package.
func (e *BatchRowError) BatchRowIndex() int { return e.Index }

// MasterDataImportBatchRepository stages CSV import batches (plan.md T5.4).
// Primary pool only: it writes.
type MasterDataImportBatchRepository struct {
	pool ImportBatchPool
	q    *db.Queries
}

// NewMasterDataImportBatchRepository creates the repository; conn and pool are
// the same primary *pgxpool.Pool.
func NewMasterDataImportBatchRepository(conn db.DBTX, pool ImportBatchPool) *MasterDataImportBatchRepository {
	return &MasterDataImportBatchRepository{pool: pool, q: db.New(conn)}
}

// FindOpen returns the latest batch of (entity, fileHash) whose changes are not
// rejected/stale, or nil.
func (r *MasterDataImportBatchRepository) FindOpen(ctx context.Context, entity, fileHash string) (*ImportBatch, error) {
	return findOpenBatch(ctx, r.q, entity, fileHash)
}

func findOpenBatch(ctx context.Context, q *db.Queries, entity, fileHash string) (*ImportBatch, error) {
	row, err := q.FindOpenMasterDataImportBatch(ctx, db.FindOpenMasterDataImportBatchParams{Entity: entity, FileHash: fileHash})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// Create stages the batch atomically. An advisory lock on (entity, fileHash)
// serializes concurrent uploads of the same file; if an open batch already
// exists it is returned with existing=true and nothing is written. A row that
// collides with another pending change of the same entity fails the whole
// batch with a *BatchRowError.
func (r *MasterDataImportBatchRepository) Create(ctx context.Context, entity, fileHash string, makerID int64, rows []db.CreateMasterDataChangeRequestParams) (batch *ImportBatch, existing bool, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("begin batch tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := r.q.WithTx(tx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, entity+":"+fileHash); err != nil {
		return nil, false, fmt.Errorf("lock batch: %w", err)
	}
	if found, err := findOpenBatch(ctx, q, entity, fileHash); err != nil {
		return nil, false, fmt.Errorf("find batch: %w", err)
	} else if found != nil {
		return found, true, nil
	}

	b, err := q.CreateMasterDataImportBatch(ctx, db.CreateMasterDataImportBatchParams{
		Entity: entity, FileHash: fileHash, MakerID: makerID, RowCount: int32(len(rows)),
	})
	if err != nil {
		return nil, false, fmt.Errorf("create batch: %w", err)
	}
	out := &ImportBatch{ID: b.ID, MakerID: makerID, RowCount: b.RowCount}
	for i, row := range rows {
		row.BatchID = &b.ID
		change, err := q.CreateMasterDataChangeRequest(ctx, row)
		if err != nil {
			var pg *pgconn.PgError
			if errors.As(err, &pg) && pg.Code == "23505" {
				return nil, false, &BatchRowError{Index: i, Err: err}
			}
			return nil, false, fmt.Errorf("create change request %d: %w", i, err)
		}
		if i == 0 {
			out.HeadID = change.ID
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("commit batch: %w", err)
	}
	return out, false, nil
}

// SetApprovalRequestID links every change request of the batch to its single approval request.
func (r *MasterDataImportBatchRepository) SetApprovalRequestID(ctx context.Context, batchID, approvalRequestID int64) error {
	return r.q.SetMasterDataBatchApprovalID(ctx, db.SetMasterDataBatchApprovalIDParams{BatchID: &batchID, ApprovalRequestID: &approvalRequestID})
}
