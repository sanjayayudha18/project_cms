package exportjob

import (
	"context"
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
)

// dlFake is an in-memory primary for the download path. A tx works on copies
// and publishes them only on commit, so a rollback leaves jobs and chunks as they were.
type dlFake struct {
	roles   map[int64]string // active users only
	jobs    map[int64]repository.ExportJob
	chunks  map[int64]map[int32][]byte
	now     time.Time
	claims  int // successful StartExportJobDownload calls (committed or not)
	commits int
	rolls   int
	chunkQs []int32
	failGet error // GetExportJobChunk error
}

func (f *dlFake) GetActiveUserRole(_ context.Context, uid int64) (string, error) {
	if r, ok := f.roles[uid]; ok {
		return r, nil
	}
	return "", pgx.ErrNoRows
}

func (f *dlFake) GetExportJobForUser(_ context.Context, a repository.GetExportJobForUserParams) (repository.ExportJob, error) {
	j, ok := f.jobs[a.ID]
	if !ok || j.RequestedBy != a.RequestedBy {
		return repository.ExportJob{}, pgx.ErrNoRows
	}
	return j, nil
}

func (f *dlFake) ListExportJobsForUser(context.Context, repository.ListExportJobsForUserParams) ([]repository.ExportJob, error) {
	return nil, nil
}

func (f *dlFake) StartExportJobDownload(_ context.Context, a repository.StartExportJobDownloadParams) (repository.ExportJob, error) {
	j, ok := f.jobs[a.ID]
	if !ok || j.RequestedBy != a.RequestedBy || j.Status != "succeeded" || !j.ExpiresAt.Valid || !j.ExpiresAt.Time.After(f.now) {
		return repository.ExportJob{}, pgx.ErrNoRows
	}
	j.Status = "downloading"
	j.DownloadStartedAt = pgtype.Timestamptz{Time: f.now, Valid: true}
	f.jobs[a.ID] = j
	f.claims++
	return j, nil
}

func (f *dlFake) setStatus(id int64, from, to string) int64 {
	j, ok := f.jobs[id]
	if !ok || j.Status != from {
		return 0
	}
	j.Status = to
	f.jobs[id] = j
	return 1
}

func (f *dlFake) CompleteExportJobDownload(_ context.Context, id int64) (int64, error) {
	return f.setStatus(id, "downloading", "downloaded"), nil
}

func (f *dlFake) AbortExportJobDownload(_ context.Context, id int64) (int64, error) {
	return f.setStatus(id, "downloading", "succeeded"), nil
}

func (f *dlFake) GetExportJobChunk(_ context.Context, a repository.GetExportJobChunkParams) ([]byte, error) {
	f.chunkQs = append(f.chunkQs, a.Seq)
	if f.failGet != nil {
		return nil, f.failGet
	}
	d, ok := f.chunks[a.ExportJobID][a.Seq]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	return d, nil
}

func (f *dlFake) DeleteExportJobChunks(_ context.Context, id int64) error {
	delete(f.chunks, id)
	return nil
}

func (f *dlFake) tx(_ context.Context, fn func(repository.DBTX, downloadQueries) error) error {
	jobs, chunks := maps.Clone(f.jobs), maps.Clone(f.chunks)
	if err := fn(nil, f); err != nil {
		f.jobs, f.chunks = jobs, chunks
		f.rolls++
		return err
	}
	f.commits++
	return nil
}

const (
	dlOwner = int64(7)
	dlOther = int64(8)
	dlJobID = int64(42)
)

func dlJob(status string, expiresIn time.Duration) repository.ExportJob {
	vid, bid := int64(3), int64(31)
	vcode, bcode := "ABA", "MDN"
	cc := int32(2)
	return repository.ExportJob{
		ID: dlJobID, RequestedBy: dlOwner, Kind: string(KindCabang), Entity: "pics",
		VendorID: &vid, VendorCode: &vcode, VendorBranchID: &bid, BranchCode: &bcode,
		StatusFilter: "active", FileName: "f.csv", Status: status, RowCount: 42, ChunkCount: &cc,
		ExpiresAt: pgtype.Timestamptz{Time: fixedNow.Add(expiresIn), Valid: true},
	}
}

func newDL(job repository.ExportJob) (*dlFake, *fakeAudit, *Service) {
	f := &dlFake{
		roles:  map[int64]string{dlOwner: "ADMIN_PARAM"},
		jobs:   map[int64]repository.ExportJob{job.ID: job},
		chunks: map[int64]map[int32][]byte{job.ID: {0: []byte("a,b\n"), 1: []byte("1,2\n")}},
		now:    fixedNow,
	}
	au := &fakeAudit{}
	return f, au, &Service{r: f, dl: f, dlTx: f.tx, audit: au, now: func() time.Time { return fixedNow }}
}

var dlActor = Actor{UserID: dlOwner, Username: "admin.param", Role: "ADMIN_PARAM", IP: "10.0.0.1"}

func TestStartDownload_Success_AuditAndClaim(t *testing.T) {
	f, au, svc := newDL(dlJob("succeeded", time.Hour))
	job, err := svc.StartDownload(context.Background(), dlActor, dlJobID)
	if err != nil {
		t.Fatalf("StartDownload: %v", err)
	}
	if job.Status != StatusDownloading || f.jobs[dlJobID].Status != "downloading" || f.commits != 1 {
		t.Fatalf("claim not committed: job=%s db=%s commits=%d", job.Status, f.jobs[dlJobID].Status, f.commits)
	}
	if len(au.entries) != 1 {
		t.Fatalf("audit rows = %d, want 1", len(au.entries))
	}
	e := au.entries[0]
	if e.Action != ActionMasterDataExported || e.ActorID != dlOwner || e.EntityType != "vendor_branch" || e.EntityID != 31 || e.IP != "10.0.0.1" {
		t.Fatalf("audit entry = %+v", e)
	}
	want := requestedAfter(f.jobs[dlJobID], dlActor)
	want["row_count"] = int64(42)
	if len(e.After) != len(want) {
		t.Fatalf("after keys = %v, want %v", e.After, want)
	}
	for _, k := range []string{"export_job_id", "export_kind", "export_entity", "username", "role", "row_count"} {
		if e.After[k] != want[k] {
			t.Errorf("after[%s] = %v, want %v", k, e.After[k], want[k])
		}
	}
}

func TestStartDownload_Rejections_NoAuditNoClaim(t *testing.T) {
	cases := []struct {
		name   string
		job    repository.ExportJob
		actor  Actor
		id     int64
		revoke bool
		role   string
		want   error
	}{
		{"unauthenticated", dlJob("succeeded", time.Hour), Actor{}, dlJobID, false, "", ErrUnauthenticated},
		{"other user's job", dlJob("succeeded", time.Hour), Actor{UserID: dlOther, Role: "ADMIN"}, dlJobID, false, "", ErrNotFound},
		{"missing job", dlJob("succeeded", time.Hour), dlActor, 999, false, "", ErrNotFound},
		{"user inactive", dlJob("succeeded", time.Hour), dlActor, dlJobID, true, "", ErrForbidden},
		{"role revoked in DB (JWT still admin)", dlJob("succeeded", time.Hour), dlActor, dlJobID, false, "ATM-USER", ErrForbidden},
		{"queued", dlJob("queued", time.Hour), dlActor, dlJobID, false, "", ErrNotReady},
		{"running", dlJob("running", time.Hour), dlActor, dlJobID, false, "", ErrNotReady},
		{"failed", dlJob("failed", time.Hour), dlActor, dlJobID, false, "", ErrNotReady},
		{"downloading", dlJob("downloading", time.Hour), dlActor, dlJobID, false, "", ErrNotReady},
		{"downloaded", dlJob("downloaded", time.Hour), dlActor, dlJobID, false, "", ErrGone},
		{"expired", dlJob("expired", -time.Hour), dlActor, dlJobID, false, "", ErrGone},
		{"succeeded past expires_at", dlJob("succeeded", -time.Second), dlActor, dlJobID, false, "", ErrGone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, au, svc := newDL(tc.job)
			if tc.revoke {
				delete(f.roles, dlOwner)
			}
			if tc.role != "" {
				f.roles[dlOwner] = tc.role
			}
			before := f.jobs[dlJobID].Status
			_, err := svc.StartDownload(context.Background(), tc.actor, tc.id)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if len(au.entries) != 0 || f.commits != 0 || f.jobs[dlJobID].Status != before {
				t.Fatalf("side effect: audit=%d commits=%d status %s->%s", len(au.entries), f.commits, before, f.jobs[dlJobID].Status)
			}
		})
	}
}

func TestStartDownload_RoleCheckedBeforeClaim(t *testing.T) {
	f, _, svc := newDL(dlJob("succeeded", time.Hour))
	f.roles[dlOwner] = "VENDOR-USER"
	if _, err := svc.StartDownload(context.Background(), dlActor, dlJobID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v", err)
	}
	if f.claims != 0 {
		t.Fatalf("claim attempted with forbidden role")
	}
}

func TestStartDownload_AuditFailure_ClaimRolledBack(t *testing.T) {
	f, au, svc := newDL(dlJob("succeeded", time.Hour))
	au.err = errors.New("insert failed")
	_, err := svc.StartDownload(context.Background(), dlActor, dlJobID)
	if !errors.Is(err, ErrInternal) {
		t.Fatalf("err = %v, want ErrInternal", err)
	}
	if f.jobs[dlJobID].Status != "succeeded" || f.rolls != 1 || f.commits != 0 {
		t.Fatalf("claim not reverted: status=%s rolls=%d commits=%d", f.jobs[dlJobID].Status, f.rolls, f.commits)
	}
	au.err = nil // retry works: the job is still downloadable
	if _, err := svc.StartDownload(context.Background(), dlActor, dlJobID); err != nil {
		t.Fatalf("retry: %v", err)
	}
}

func TestStartDownload_DBErrors_Internal(t *testing.T) {
	f, _, svc := newDL(dlJob("succeeded", time.Hour))
	svc.dl = roleErr{f}
	if _, err := svc.StartDownload(context.Background(), dlActor, dlJobID); !errors.Is(err, ErrInternal) {
		t.Fatalf("err = %v", err)
	}
}

type roleErr struct{ *dlFake }

func (roleErr) GetActiveUserRole(context.Context, int64) (string, error) {
	return "", errors.New("conn reset")
}

func TestCompleteDownload_DeletesChunksAndMarksDownloaded(t *testing.T) {
	f, _, svc := newDL(dlJob("downloading", time.Hour))
	if err := svc.CompleteDownload(context.Background(), dlJobID); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if f.jobs[dlJobID].Status != "downloaded" || f.chunks[dlJobID] != nil || f.commits != 1 {
		t.Fatalf("status=%s chunks=%v commits=%d", f.jobs[dlJobID].Status, f.chunks[dlJobID], f.commits)
	}
}

func TestCompleteDownload_NotDownloading_LeavesChunks(t *testing.T) {
	f, _, svc := newDL(dlJob("succeeded", time.Hour)) // janitor recovered it
	err := svc.CompleteDownload(context.Background(), dlJobID)
	if !errors.Is(err, ErrNotDownloading) {
		t.Fatalf("err = %v", err)
	}
	if f.jobs[dlJobID].Status != "succeeded" || len(f.chunks[dlJobID]) != 2 {
		t.Fatalf("job touched: status=%s chunks=%d", f.jobs[dlJobID].Status, len(f.chunks[dlJobID]))
	}
}

func TestAbortDownload(t *testing.T) {
	f, _, svc := newDL(dlJob("downloading", time.Hour))
	if err := svc.AbortDownload(context.Background(), dlJobID); err != nil {
		t.Fatalf("Abort: %v", err)
	}
	if f.jobs[dlJobID].Status != "succeeded" || len(f.chunks[dlJobID]) != 2 {
		t.Fatalf("status=%s chunks=%d", f.jobs[dlJobID].Status, len(f.chunks[dlJobID]))
	}
	if err := svc.AbortDownload(context.Background(), dlJobID); !errors.Is(err, ErrNotDownloading) {
		t.Fatalf("second abort err = %v", err)
	}
}

func collect(it ChunkIterator) ([]string, error) {
	var out []string
	for d, err := range it {
		if err != nil {
			return out, err
		}
		out = append(out, string(d))
	}
	return out, nil
}

func TestOpenChunks_OrderAndErrors(t *testing.T) {
	ctx := context.Background()
	f, _, svc := newDL(dlJob("downloading", time.Hour))
	got, err := collect(svc.OpenChunks(ctx, dlJobID, 2))
	if err != nil || len(got) != 2 || got[0] != "a,b\n" || got[1] != "1,2\n" {
		t.Fatalf("got %q, %v", got, err)
	}
	if len(f.chunkQs) != 2 || f.chunkQs[0] != 0 || f.chunkQs[1] != 1 {
		t.Fatalf("queries %v, want one per seq in order", f.chunkQs)
	}

	got, err = collect(svc.OpenChunks(ctx, dlJobID, 3)) // seq 2 missing
	if !errors.Is(err, ErrInternal) || len(got) != 2 {
		t.Fatalf("missing chunk: got %q, err %v", got, err)
	}

	f.chunkQs = nil
	for range svc.OpenChunks(ctx, dlJobID, 2) {
		break // early stop: no further query
	}
	if len(f.chunkQs) != 1 {
		t.Fatalf("queries after break = %v", f.chunkQs)
	}

	f.failGet = errors.New("conn reset")
	if _, err := collect(svc.OpenChunks(ctx, dlJobID, 2)); !errors.Is(err, ErrInternal) {
		t.Fatalf("db error: %v", err)
	}
	if _, err := collect(svc.OpenChunks(ctx, dlJobID, -1)); !errors.Is(err, ErrInternal) {
		t.Fatalf("negative count: %v", err)
	}
}
