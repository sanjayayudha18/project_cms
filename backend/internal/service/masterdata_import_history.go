package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/repository"
)

// BatchRollbackPrefix starts the error text of every change row that was rolled
// back only because another row of its batch failed. The row without this
// prefix carries the real cause (master-data-import-revamp Req 6.2).
const BatchRollbackPrefix = "batch rolled back: "

const (
	importHistoryDefaultPageSize = 20
	importHistoryMaxPageSize     = 100
	importDetailDefaultPageSize  = 50
)

// ErrImportBatchNotFound maps to 404.
var ErrImportBatchNotFound = errors.New("import batch not found")

// MasterDataImportHistoryRepo is what the history service reads (primary pool).
// *repository.MasterDataImportBatchRepository satisfies it. The list filter is
// the repository's own type so there is a single filter definition; the rows
// are sqlc types that this service maps to its API types.
type MasterDataImportHistoryRepo interface {
	ListBatches(ctx context.Context, f repository.ImportBatchFilter) ([]db.ListMasterDataImportBatchesRow, int64, error)
	GetBatch(ctx context.Context, id int64) (db.GetMasterDataImportBatchRow, error)
	ListByBatch(ctx context.Context, batchID int64) ([]db.MasterDataChangeRequest, error)
}

// ImportBatchSummary is one history row. Status is the raw status of the
// batch's head change row; labels belong to the frontend.
type ImportBatchSummary struct {
	ID                int64      `json:"id"`
	Entity            string     `json:"entity"`
	FileName          *string    `json:"file_name"`
	RowCount          int32      `json:"row_count"`
	MakerID           int64      `json:"maker_id"`
	MakerName         string     `json:"maker_name"`
	CreatedAt         time.Time  `json:"created_at"`
	Status            string     `json:"status"`
	ApprovalRequestID *int64     `json:"approval_request_id"`
	Error             *string    `json:"error"`
	DecidedAt         *time.Time `json:"decided_at"`
	DecidedByName     *string    `json:"decided_by_name"`
}

// ImportBatchPage is one page of the history list.
type ImportBatchPage struct {
	Items    []ImportBatchSummary `json:"items"`
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

// ImportBatchCounts counts the change rows of the WHOLE batch per operation.
type ImportBatchCounts struct {
	Create  int `json:"create"`
	Update  int `json:"update"`
	Enable  int `json:"enable"`
	Disable int `json:"disable"`
}

// ImportBatchChange is one change row of a batch. Failed marks the row that
// caused a rollback (error set and not the rollback prefix). Same fields as the
// handler's masterDataChangeToResponse plus Failed.
type ImportBatchChange struct {
	ID                int64           `json:"id"`
	EntityType        string          `json:"entity_type"`
	EntityID          *int64          `json:"entity_id"`
	Op                string          `json:"op"`
	Payload           json.RawMessage `json:"payload"`
	Before            json.RawMessage `json:"before"`
	Status            string          `json:"status"`
	MakerID           int64           `json:"maker_id"`
	ApprovalRequestID *int64          `json:"approval_request_id"`
	BatchID           *int64          `json:"batch_id"`
	Error             *string         `json:"error"`
	Failed            bool            `json:"failed"`
	CreatedAt         *time.Time      `json:"created_at"`
	UpdatedAt         *time.Time      `json:"updated_at"`
}

// ImportBatchDetail is a batch with one page of its change rows.
type ImportBatchDetail struct {
	ImportBatchSummary
	Counts  ImportBatchCounts   `json:"counts"`
	Changes []ImportBatchChange `json:"changes"`
	// FailureReason is the error of the first failed row of the WHOLE batch (it
	// may sit on another page); nil when no row carries a real cause (old batches).
	FailureReason *string `json:"failure_reason"`
	Total         int     `json:"total"`
	Page          int     `json:"page"`
	PageSize      int     `json:"page_size"`
}

// MasterDataImportHistoryService serves the import history. Read-only, primary
// pool (a batch just confirmed must show up at once), no audit (Req 9.3).
type MasterDataImportHistoryService struct{ repo MasterDataImportHistoryRepo }

// NewMasterDataImportHistoryService creates the service.
func NewMasterDataImportHistoryService(repo MasterDataImportHistoryRepo) *MasterDataImportHistoryService {
	return &MasterDataImportHistoryService{repo: repo}
}

// List returns one page of batches. f's Limit/Offset are overwritten from
// page/pageSize (defaults 1/20, pageSize capped at 100).
func (s *MasterDataImportHistoryService) List(ctx context.Context, f repository.ImportBatchFilter, page, pageSize int) (ImportBatchPage, error) {
	if err := authorizeMasterDataAdmin(ctx); err != nil {
		return ImportBatchPage{}, err
	}
	page, pageSize = normalizePaging(page, pageSize, importHistoryDefaultPageSize, importHistoryMaxPageSize)
	f.Limit, f.Offset = int64(pageSize), int64((page-1)*pageSize)
	rows, total, err := s.repo.ListBatches(ctx, f)
	if err != nil {
		return ImportBatchPage{}, fmt.Errorf("list import batches: %w", err)
	}
	items := make([]ImportBatchSummary, len(rows))
	for n, r := range rows {
		items[n] = ImportBatchSummary{ID: r.ID, Entity: r.Entity, FileName: r.FileName, RowCount: r.RowCount,
			MakerID: r.MakerID, MakerName: r.MakerName, CreatedAt: r.CreatedAt.Time, Status: r.HeadStatus,
			ApprovalRequestID: r.ApprovalRequestID, Error: r.HeadError, DecidedAt: timePtr(r.DecidedAt), DecidedByName: r.DecidedByName}
	}
	return ImportBatchPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// Get returns one batch with page `page` of its change rows (default size 50).
// All rows are loaded (<= MasterDataImportMaxRows) so Counts covers the batch.
func (s *MasterDataImportHistoryService) Get(ctx context.Context, id int64, page, pageSize int) (ImportBatchDetail, error) {
	if err := authorizeMasterDataAdmin(ctx); err != nil {
		return ImportBatchDetail{}, err
	}
	b, err := s.repo.GetBatch(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ImportBatchDetail{}, ErrImportBatchNotFound
	}
	if err != nil {
		return ImportBatchDetail{}, fmt.Errorf("get import batch %d: %w", id, err)
	}
	rows, err := s.repo.ListByBatch(ctx, id)
	if err != nil {
		return ImportBatchDetail{}, fmt.Errorf("list batch %d changes: %w", id, err)
	}
	page, pageSize = normalizePaging(page, pageSize, importDetailDefaultPageSize, MasterDataImportMaxRows)
	changes := make([]ImportBatchChange, 0, min(pageSize, len(rows)))
	for _, c := range pageSlice(rows, page, pageSize) {
		changes = append(changes, changeFrom(c))
	}
	return ImportBatchDetail{
		ImportBatchSummary: ImportBatchSummary{ID: b.ID, Entity: b.Entity, FileName: b.FileName, RowCount: b.RowCount,
			MakerID: b.MakerID, MakerName: b.MakerName, CreatedAt: b.CreatedAt.Time, Status: b.HeadStatus,
			ApprovalRequestID: b.ApprovalRequestID, Error: b.HeadError, DecidedAt: timePtr(b.DecidedAt), DecidedByName: b.DecidedByName},
		Counts: countOps(rows), Changes: changes, FailureReason: failureReason(rows),
		Total: len(rows), Page: page, PageSize: pageSize,
	}, nil
}

// failureReason returns the error of the first failed row, or nil.
func failureReason(rows []db.MasterDataChangeRequest) *string {
	for _, r := range rows {
		if isFailedRow(r.Error) {
			return r.Error
		}
	}
	return nil
}

// pageSlice returns the 1-based page of rows; out-of-range pages are empty.
// Concatenating pages 1..n yields every row once, in order.
func pageSlice[T any](rows []T, page, pageSize int) []T {
	if page < 1 || pageSize < 1 {
		return nil
	}
	start := (page - 1) * pageSize
	if start >= len(rows) {
		return nil
	}
	return rows[start:min(start+pageSize, len(rows))]
}

// normalizePaging applies defaults and the cap to page/size.
func normalizePaging(page, size, def, maxSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = def
	}
	return page, min(size, maxSize)
}

func countOps(rows []db.MasterDataChangeRequest) ImportBatchCounts {
	var c ImportBatchCounts
	for _, r := range rows {
		switch r.Op {
		case "create":
			c.Create++
		case "update":
			c.Update++
		case "enable":
			c.Enable++
		case "disable":
			c.Disable++
		}
	}
	return c
}

// isFailedRow reports the row that caused a rollback.
func isFailedRow(errText *string) bool {
	return errText != nil && *errText != "" && !strings.HasPrefix(*errText, BatchRollbackPrefix)
}

func changeFrom(c db.MasterDataChangeRequest) ImportBatchChange {
	out := ImportBatchChange{ID: c.ID, EntityType: c.EntityType, EntityID: c.EntityID, Op: c.Op,
		Payload: json.RawMessage(c.Payload), Status: c.Status, MakerID: c.MakerID,
		ApprovalRequestID: c.ApprovalRequestID, BatchID: c.BatchID, Error: c.Error, Failed: isFailedRow(c.Error),
		CreatedAt: timePtr(c.CreatedAt), UpdatedAt: timePtr(c.UpdatedAt)}
	if len(c.Before) > 0 {
		out.Before = json.RawMessage(c.Before)
	}
	return out
}

func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}
