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
func (r *MasterDataImportBatchRepository) Create(ctx context.Context, entity, fileHash string, fileName *string, makerID int64, rows []db.CreateMasterDataChangeRequestParams) (batch *ImportBatch, existing bool, err error) {
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
		Entity: entity, FileHash: fileHash, FileName: fileName, MakerID: makerID, RowCount: int32(len(rows)), //nolint:gosec // G115: rows <= MasterDataImportMaxRows (2000)
	})
	if err != nil {
		return nil, false, fmt.Errorf("create batch: %w", err)
	}
	out := &ImportBatch{ID: b.ID, MakerID: makerID, RowCount: b.RowCount, FileName: b.FileName}
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

// ImportBatchFilter narrows ListBatches. Nil pointers mean "no filter".
type ImportBatchFilter struct {
	Entity, Status *string
	MakerID        *int64
	Limit, Offset  int64
}

// ListBatches returns one page of import batches (newest first) and the total
// matching the filter. Rows are the sqlc type; the service maps them (the
// repository must not import service).
func (r *MasterDataImportBatchRepository) ListBatches(ctx context.Context, f ImportBatchFilter) ([]db.ListMasterDataImportBatchesRow, int64, error) {
	total, err := r.q.CountMasterDataImportBatches(ctx, db.CountMasterDataImportBatchesParams{Entity: f.Entity, Status: f.Status, MakerID: f.MakerID})
	if err != nil {
		return nil, 0, fmt.Errorf("count import batches: %w", err)
	}
	rows, err := r.q.ListMasterDataImportBatches(ctx, db.ListMasterDataImportBatchesParams{
		Entity: f.Entity, Status: f.Status, MakerID: f.MakerID, PageLimit: f.Limit, PageOffset: f.Offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list import batches: %w", err)
	}
	return rows, total, nil
}

// GetBatch returns one batch; pgx.ErrNoRows is passed through for the caller to map to 404.
func (r *MasterDataImportBatchRepository) GetBatch(ctx context.Context, id int64) (db.GetMasterDataImportBatchRow, error) {
	return r.q.GetMasterDataImportBatch(ctx, id)
}

// ListByBatch returns every change request of the batch, ordered by id.
func (r *MasterDataImportBatchRepository) ListByBatch(ctx context.Context, batchID int64) ([]db.MasterDataChangeRequest, error) {
	return r.q.ListMasterDataChangeRequestsByBatch(ctx, &batchID)
}

// ListPendingApprovers returns the assigned approvers of the lowest pending step.
func (r *MasterDataImportBatchRepository) ListPendingApprovers(ctx context.Context, approvalRequestID int64) ([]int64, error) {
	return r.q.ListPendingApproversForRequest(ctx, approvalRequestID)
}

// SetApprovalRequestID links every change request of the batch to its single approval request.
func (r *MasterDataImportBatchRepository) SetApprovalRequestID(ctx context.Context, batchID, approvalRequestID int64) error {
	return r.q.SetMasterDataBatchApprovalID(ctx, db.SetMasterDataBatchApprovalIDParams{BatchID: &batchID, ApprovalRequestID: &approvalRequestID})
}
