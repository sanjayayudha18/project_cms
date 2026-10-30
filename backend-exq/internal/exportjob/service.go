package exportjob

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend-exq/internal/audit"
	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
)

// createQueries is the slice of *repository.Queries that Create needs.
type createQueries interface {
	GetVendorCodeByID(ctx context.Context, vendorID int64) (string, error)
	GetVendorBranchForVendor(ctx context.Context, arg repository.GetVendorBranchForVendorParams) (string, error)
	LockExportJobsForUser(ctx context.Context, userID int64) error
	FindActiveExportJobByHash(ctx context.Context, arg repository.FindActiveExportJobByHashParams) (repository.ExportJob, error)
	CountActiveExportJobs(ctx context.Context, requestedBy int64) (int64, error)
	InsertExportJob(ctx context.Context, arg repository.InsertExportJobParams) (repository.ExportJob, error)
}

// readQueries is the slice of *repository.Queries that List and Get need.
// Both queries filter on requested_by in SQL, so ownership never depends on Go code alone.
type readQueries interface {
	ListExportJobsForUser(ctx context.Context, arg repository.ListExportJobsForUserParams) ([]repository.ExportJob, error)
	GetExportJobForUser(ctx context.Context, arg repository.GetExportJobForUserParams) (repository.ExportJob, error)
}

// List page size (Req 12.6, design API table: limit 1-50, default 20).
const (
	DefaultListLimit = 20
	MaxListLimit     = 50
)

// auditWriter is satisfied by *audit.Writer.
type auditWriter interface {
	Write(ctx context.Context, q repository.DBTX, e audit.Entry) error
}

// txFunc runs fn in one primary transaction: commit when fn returns nil, rollback otherwise.
// fn gets the raw tx (for the audit writer) and queries bound to it.
type txFunc func(ctx context.Context, fn func(tx repository.DBTX, q createQueries) error) error

// Service is the export job service. All reads and writes go to the primary (Req 6, Sec 6).
type Service struct {
	q     createQueries // primary, outside a tx (scope resolution)
	r     readQueries   // primary (read-after-write, Req 6.3)
	inTx  txFunc
	dl    downloadQueries // primary: role check, re-read, chunks, abort
	dlTx  downloadTxFunc
	audit auditWriter
	wake  func() // must not block (worker uses a buffered chan of 1)
	now   func() time.Time
}

// NewService wires the service on the primary pool. wake may be nil.
func NewService(primary *pgxpool.Pool, w *audit.Writer, wake func()) *Service {
	q := repository.New(primary)
	return &Service{
		q: q,
		r: q,
		inTx: func(ctx context.Context, fn func(repository.DBTX, createQueries) error) error {
			return pgx.BeginFunc(ctx, primary, func(tx pgx.Tx) error {
				return fn(tx, repository.New(tx))
			})
		},
		dl: q,
		dlTx: func(ctx context.Context, fn func(repository.DBTX, downloadQueries) error) error {
			return pgx.BeginFunc(ctx, primary, func(tx pgx.Tx) error {
				return fn(tx, repository.New(tx))
			})
		},
		audit: w,
		wake:  wake,
		now:   time.Now,
	}
}

func wrapInternal(op string, err error) error {
	return fmt.Errorf("%w: %s: %w", ErrInternal, op, err)
}

// resolveScope returns vendor_code / branch_code. Unknown vendor and a cabang
// that is missing or belongs to another vendor give the same ErrNotFound (Req 2.4, 3.2).
// Inactive vendors resolve (Req 1.6).
func (s *Service) resolveScope(ctx context.Context, p Params) (vendorCode, branchCode *string, err error) {
	notFound := func(op string, err error) error {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return wrapInternal(op, err)
	}
	if p.VendorID != nil {
		code, err := s.q.GetVendorCodeByID(ctx, *p.VendorID)
		if err != nil {
			return nil, nil, notFound("get vendor", err)
		}
		vendorCode = &code
	}
	if p.BranchID != nil {
		if p.VendorID == nil {
			return nil, nil, ErrNotFound // ParseParams never allows this; fail closed anyway
		}
		code, err := s.q.GetVendorBranchForVendor(ctx, repository.GetVendorBranchForVendorParams{
			BranchID: *p.BranchID, VendorID: *p.VendorID,
		})
		if err != nil {
			return nil, nil, notFound("get vendor branch", err)
		}
		branchCode = &code
	}
	return vendorCode, branchCode, nil
}

// statusFilterOf: status_filter is set for master-data kinds and NULL for report kinds (migration 035).
func statusFilterOf(p Params) *string {
	if p.Report != nil {
		return nil
	}
	return &p.StatusFilter
}

// requestedAfter is the audit `after` of export_requested (design ExqAuditWriter; no row_count).
// Report kinds keep the legacy report_exported shape (report + filter) so old and new rows read alike.
func requestedAfter(r repository.ExportJob, a Actor) map[string]any {
	if rp := reportParamsFromJSON(r.ReportParams); rp != nil && IsReportKind(Kind(r.Kind)) {
		return map[string]any{
			"export_job_id": r.ID,
			"export_kind":   r.Kind,
			"report":        r.Entity,
			"filter":        rp.legacyFilter(Kind(r.Kind)),
			"username":      a.Username,
			"role":          a.Role,
		}
	}
	return map[string]any{
		"export_job_id": r.ID,
		"export_kind":   r.Kind,
		"export_entity": r.Entity,
		"vendor_id":     r.VendorID,
		"vendor_code":   r.VendorCode,
		"branch_id":     r.VendorBranchID,
		"branch_code":   r.BranchCode,
		"status_filter": r.StatusFilter,
		"price_level":   r.PriceLevel,
		"username":      a.Username,
		"role":          a.Role,
	}
}

// Create queues a new export job, or returns the caller's active job with the
// same params (created=false, no audit). p must come from ParseParams.
// Req 1.1, 1.6, 2.1, 2.4, 3.2, 7.2, 7.3, 8.1, 8.3-8.8, 9.1, 9.3-9.5, 14.7.
//
// Concurrency: the per-user advisory xact lock serialises dedupe + count +
// insert, and is held until commit, so a second Create of the same user sees
// the first one's committed row. A violation of export_jobs_active_dedupe_uq
// therefore cannot come from two Creates; if it ever happens the lock invariant
// is broken and it surfaces as ErrInternal (500) instead of being masked.
func (s *Service) Create(ctx context.Context, a Actor, p Params) (Job, bool, error) {
	if a.UserID == 0 {
		return Job{}, false, ErrUnauthenticated
	}
	if !allowed(p.Kind, a.Role) {
		return Job{}, false, ErrForbidden
	}
	vendorCode, branchCode, err := s.resolveScope(ctx, p)
	if err != nil {
		return Job{}, false, err
	}

	hash := ParamsHash(p)
	var (
		row     repository.ExportJob
		created bool
	)
	err = s.inTx(ctx, func(tx repository.DBTX, q createQueries) error {
		if err := q.LockExportJobsForUser(ctx, a.UserID); err != nil {
			return wrapInternal("lock user", err)
		}
		existing, err := q.FindActiveExportJobByHash(ctx, repository.FindActiveExportJobByHashParams{
			RequestedBy: a.UserID, ParamsHash: hash,
		})
		if err == nil {
			row = existing
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return wrapInternal("find active job", err)
		}
		n, err := q.CountActiveExportJobs(ctx, a.UserID)
		if err != nil {
			return wrapInternal("count active jobs", err)
		}
		if n >= MaxActivePerUser {
			return ErrTooManyActive
		}
		row, err = q.InsertExportJob(ctx, repository.InsertExportJobParams{
			RequestedBy:    a.UserID,
			Kind:           string(p.Kind),
			Entity:         p.Entity,
			VendorID:       p.VendorID,
			VendorCode:     vendorCode,
			VendorBranchID: p.BranchID,
			BranchCode:     branchCode,
			StatusFilter:   statusFilterOf(p),
			PriceLevel:     p.PriceLevel,
			ParamsHash:     hash,
			FileName:       FileName(p, strOrEmpty(vendorCode), strOrEmpty(branchCode), s.now()),
			ReportParams:   reportParamsJSON(p.Report),
		})
		if err != nil {
			return wrapInternal("insert job", err)
		}
		job := jobFromRow(row)
		entityType, entityID := KindSpecs[p.Kind].EntityRef(job)
		if err := s.audit.Write(ctx, tx, audit.Entry{
			ActorID:    a.UserID,
			Action:     ActionExportRequested,
			EntityType: entityType,
			EntityID:   entityID,
			After:      requestedAfter(row, a),
			IP:         a.IP,
		}); err != nil {
			return wrapInternal("audit export_requested", err) // rollback: no job without audit (Req 8.1)
		}
		created = true
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrTooManyActive) || errors.Is(err, ErrInternal) {
			return Job{}, false, err
		}
		return Job{}, false, wrapInternal("commit", err)
	}
	if created && s.wake != nil {
		s.wake()
	}
	return jobFromRow(row), created, nil
}

// List returns the caller's own jobs, newest first (order from the query).
// limit <= 0 -> DefaultListLimit, > MaxListLimit -> MaxListLimit. Req 6.3, 7.5, 12.6.
// No role check (D23, Req 7.1): a user whose role was revoked still sees their
// own rows; the download re-checks the role (Req 7.6).
func (s *Service) List(ctx context.Context, a Actor, limit int) ([]Job, error) {
	if a.UserID == 0 {
		return nil, ErrUnauthenticated
	}
	switch {
	case limit <= 0:
		limit = DefaultListLimit
	case limit > MaxListLimit:
		limit = MaxListLimit
	}
	rows, err := s.r.ListExportJobsForUser(ctx, repository.ListExportJobsForUserParams{
		RequestedBy: a.UserID, RowLimit: int32(limit),
	})
	if err != nil {
		return nil, wrapInternal("list jobs", err)
	}
	jobs := make([]Job, 0, len(rows))
	for _, r := range rows {
		if r.RequestedBy != a.UserID { // SQL already filters; fail closed if it ever doesn't
			continue
		}
		jobs = append(jobs, jobFromRow(r))
	}
	return jobs, nil
}

// Get returns one of the caller's jobs. Missing and someone else's job give the
// same ErrNotFound, so existence is never revealed (Req 7.5). Read on primary (Req 6.3).
// No role check (D23), as for List.
func (s *Service) Get(ctx context.Context, a Actor, id int64) (Job, error) {
	if a.UserID == 0 {
		return Job{}, ErrUnauthenticated
	}
	if id <= 0 {
		return Job{}, ErrNotFound
	}
	row, err := s.r.GetExportJobForUser(ctx, repository.GetExportJobForUserParams{
		ID: id, RequestedBy: a.UserID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, wrapInternal("get job", err)
	}
	if row.RequestedBy != a.UserID { // SQL already filters; fail closed if it ever doesn't
		return Job{}, ErrNotFound
	}
	return jobFromRow(row), nil
}
