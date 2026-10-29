package exportjob

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend-exq/internal/audit"
	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
)

// downloadQueries is the slice of *repository.Queries the one-time download needs (primary only).
type downloadQueries interface {
	GetActiveUserRole(ctx context.Context, userID int64) (string, error)
	GetExportJobForUser(ctx context.Context, arg repository.GetExportJobForUserParams) (repository.ExportJob, error)
	StartExportJobDownload(ctx context.Context, arg repository.StartExportJobDownloadParams) (repository.ExportJob, error)
	CompleteExportJobDownload(ctx context.Context, id int64) (int64, error)
	AbortExportJobDownload(ctx context.Context, id int64) (int64, error)
	GetExportJobChunk(ctx context.Context, arg repository.GetExportJobChunkParams) ([]byte, error)
	DeleteExportJobChunks(ctx context.Context, exportJobID int64) error
}

// downloadTxFunc runs fn in one primary transaction (commit on nil, rollback otherwise).
type downloadTxFunc func(ctx context.Context, fn func(tx repository.DBTX, q downloadQueries) error) error

// ChunkIterator yields the file chunks in seq order; a non-nil error ends the iteration.
type ChunkIterator = iter.Seq2[[]byte, error]

// errNotClaimed rolls back the claim tx when the guarded UPDATE matched no row.
var errNotClaimed = errors.New("exportjob: download not claimed")

// StartDownload claims the caller's succeeded job for its one-time download
// (succeeded -> downloading) and writes the download audit row. Req 7.6, 8.2, 8.6, 9.11, 10.2, 10.9.
//
// Order: own job (404) -> current role from DB vs KindSpecs[kind].Roles (403) ->
// one tx { guarded claim; audit }. The claim and its audit commit together: an
// audit failure rolls the claim back (same end state as AbortDownload, without
// a second statement that could itself fail), so no byte is ever sent without
// an audit row. Rejections (401/403/404/409/410) write no audit and change no state.
func (s *Service) StartDownload(ctx context.Context, a Actor, id int64) (Job, error) {
	if a.UserID == 0 {
		return Job{}, ErrUnauthenticated
	}
	pre, err := s.Get(ctx, a, id)
	if err != nil {
		return Job{}, err
	}
	role, err := s.dl.GetActiveUserRole(ctx, a.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrForbidden // inactive / deleted user (Req 7.6)
	}
	if err != nil {
		return Job{}, wrapInternal("get user role", err)
	}
	if !allowed(pre.Params.Kind, role) {
		return Job{}, ErrForbidden
	}

	var claimed repository.ExportJob
	err = s.dlTx(ctx, func(tx repository.DBTX, q downloadQueries) error {
		row, err := q.StartExportJobDownload(ctx, repository.StartExportJobDownloadParams{
			ID: id, RequestedBy: a.UserID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotClaimed
		}
		if err != nil {
			return wrapInternal("claim download", err)
		}
		job := jobFromRow(row)
		spec := KindSpecs[job.Params.Kind]
		entityType, entityID := spec.EntityRef(job)
		after := requestedAfter(row, a)
		after["row_count"] = row.RowCount
		if err := s.audit.Write(ctx, tx, audit.Entry{
			ActorID:    a.UserID,
			Action:     spec.DownloadAction,
			EntityType: entityType,
			EntityID:   entityID,
			After:      after,
			IP:         a.IP,
		}); err != nil {
			return wrapInternal("audit "+spec.DownloadAction, err) // rollback undoes the claim (Req 8.2)
		}
		claimed = row
		return nil
	})
	switch {
	case errors.Is(err, errNotClaimed):
		return Job{}, s.notClaimedReason(ctx, a, id)
	case errors.Is(err, ErrInternal):
		return Job{}, err
	case err != nil:
		return Job{}, wrapInternal("commit download claim", err)
	}
	return jobFromRow(claimed), nil
}

// notClaimedReason re-reads the caller's job after a rejected claim and maps its status (Req 9.11).
func (s *Service) notClaimedReason(ctx context.Context, a Actor, id int64) error {
	row, err := s.dl.GetExportJobForUser(ctx, repository.GetExportJobForUserParams{ID: id, RequestedBy: a.UserID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return wrapInternal("re-read job", err)
	}
	if row.RequestedBy != a.UserID {
		return ErrNotFound
	}
	switch Status(row.Status) {
	case StatusDownloaded, StatusExpired:
		return ErrGone
	case StatusSucceeded:
		if !row.ExpiresAt.Valid || !row.ExpiresAt.Time.After(s.now()) {
			return ErrGone // retention passed, janitor has not expired it yet
		}
		return ErrNotReady // raced with an abort / janitor recovery; retry is safe
	default: // queued, running, failed, downloading
		return ErrNotReady
	}
}

// OpenChunks yields chunks seq 0..chunkCount-1 of jobID, one query per chunk on
// the primary, so the whole file is never held in memory. A missing chunk ends
// the iteration with an ErrInternal error. Req 10.10.
func (s *Service) OpenChunks(ctx context.Context, jobID int64, chunkCount int64) ChunkIterator {
	return func(yield func([]byte, error) bool) {
		if chunkCount < 0 || chunkCount > int64(^uint32(0)>>1) {
			yield(nil, fmt.Errorf("%w: invalid chunk count %d", ErrInternal, chunkCount))
			return
		}
		for seq := int32(0); int64(seq) < chunkCount; seq++ {
			data, err := s.dl.GetExportJobChunk(ctx, repository.GetExportJobChunkParams{ExportJobID: jobID, Seq: seq})
			if errors.Is(err, pgx.ErrNoRows) {
				yield(nil, fmt.Errorf("%w: job %d chunk %d missing", ErrInternal, jobID, seq))
				return
			}
			if err != nil {
				yield(nil, wrapInternal(fmt.Sprintf("read chunk %d", seq), err))
				return
			}
			if !yield(data, nil) {
				return
			}
		}
	}
}

// CompleteDownload marks a fully sent download downloaded and deletes its chunks
// in one tx (Req 10.3). The guarded status UPDATE runs first: when the job is no
// longer downloading (janitor recovered it) it returns ErrNotDownloading and the
// tx rolls back, so that job's chunks are left untouched.
func (s *Service) CompleteDownload(ctx context.Context, id int64) error {
	err := s.dlTx(ctx, func(_ repository.DBTX, q downloadQueries) error {
		n, err := q.CompleteExportJobDownload(ctx, id)
		if err != nil {
			return wrapInternal("complete download", err)
		}
		if n == 0 {
			return ErrNotDownloading
		}
		if err := q.DeleteExportJobChunks(ctx, id); err != nil {
			return wrapInternal("delete chunks", err)
		}
		return nil
	})
	if err == nil || errors.Is(err, ErrNotDownloading) || errors.Is(err, ErrInternal) {
		return err
	}
	return wrapInternal("commit complete download", err)
}

// AbortDownload returns a downloading job to succeeded so it can be downloaded
// again (Req 10.4). Callers pass a background ctx: the request ctx is usually
// already cancelled. Zero rows (janitor got there first) -> ErrNotDownloading.
func (s *Service) AbortDownload(ctx context.Context, id int64) error {
	n, err := s.dl.AbortExportJobDownload(ctx, id)
	if err != nil {
		return wrapInternal("abort download", err)
	}
	if n == 0 {
		return ErrNotDownloading
	}
	return nil
}
