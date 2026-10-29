package exportjob

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
)

// Actor is the caller, taken from middleware.AuthContext by the handler.
// IP is r.RemoteAddr after TrustedRealIP (Req 8.5).
type Actor struct {
	UserID   int64
	Username string
	Role     string
	IP       string
}

// Job is one export_jobs row.
type Job struct {
	ID          int64
	RequestedBy int64
	Params      Params

	VendorCode, BranchCode, FileName string

	Status       Status
	RowCount     int64
	ByteSize     *int64
	ChunkCount   *int64
	ErrorMessage *string

	CreatedAt                                                                    time.Time
	StartedAt, FinishedAt, ExpiresAt, DownloadStartedAt, DownloadedAt, ExpiredAt *time.Time
}

func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

// jobFromRow maps a repository row to Job.
func jobFromRow(r repository.ExportJob) Job {
	j := Job{
		ID:          r.ID,
		RequestedBy: r.RequestedBy,
		Params: Params{
			Kind:         Kind(r.Kind),
			Entity:       r.Entity,
			VendorID:     r.VendorID,
			BranchID:     r.VendorBranchID,
			StatusFilter: r.StatusFilter,
			PriceLevel:   r.PriceLevel,
		},
		VendorCode:        strOrEmpty(r.VendorCode),
		BranchCode:        strOrEmpty(r.BranchCode),
		FileName:          r.FileName,
		Status:            Status(r.Status),
		RowCount:          r.RowCount,
		ByteSize:          r.ByteSize,
		ErrorMessage:      r.ErrorMessage,
		CreatedAt:         r.CreatedAt.Time,
		StartedAt:         tsPtr(r.StartedAt),
		FinishedAt:        tsPtr(r.FinishedAt),
		ExpiresAt:         tsPtr(r.ExpiresAt),
		DownloadStartedAt: tsPtr(r.DownloadStartedAt),
		DownloadedAt:      tsPtr(r.DownloadedAt),
		ExpiredAt:         tsPtr(r.ExpiredAt),
	}
	if r.ChunkCount != nil {
		c := int64(*r.ChunkCount)
		j.ChunkCount = &c
	}
	return j
}
