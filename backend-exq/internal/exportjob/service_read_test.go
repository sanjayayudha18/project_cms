package exportjob

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
)

// fakeReads mimics the SQL of ListExportJobsForUser / GetExportJobForUser over
// rows kept in query order (newest first). leak, when set, is returned as-is to
// simulate a broken query that ignores requested_by.
type fakeReads struct {
	rows      []repository.ExportJob
	calls     int
	lastLimit int32
	err       error
	leak      []repository.ExportJob
}

func (f *fakeReads) ListExportJobsForUser(_ context.Context, a repository.ListExportJobsForUserParams) ([]repository.ExportJob, error) {
	f.calls++
	f.lastLimit = a.RowLimit
	if f.err != nil {
		return nil, f.err
	}
	if f.leak != nil {
		return f.leak, nil
	}
	var out []repository.ExportJob
	for _, r := range f.rows {
		if r.RequestedBy == a.RequestedBy && int32(len(out)) < a.RowLimit {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeReads) GetExportJobForUser(_ context.Context, a repository.GetExportJobForUserParams) (repository.ExportJob, error) {
	f.calls++
	if f.err != nil {
		return repository.ExportJob{}, f.err
	}
	if f.leak != nil {
		return f.leak[0], nil
	}
	for _, r := range f.rows {
		if r.ID == a.ID && r.RequestedBy == a.RequestedBy {
			return r, nil
		}
	}
	return repository.ExportJob{}, pgx.ErrNoRows
}

const (
	userA = int64(1)
	userB = int64(2)
)

func readFixture() *fakeReads {
	return &fakeReads{rows: []repository.ExportJob{
		{ID: 30, RequestedBy: userA, Kind: "vendor", Status: "succeeded"},
		{ID: 20, RequestedBy: userB, Kind: "vendor", Status: "queued"},
		{ID: 10, RequestedBy: userA, Kind: "cabang", Status: "failed"},
	}}
}

func TestListGet_Unauthenticated_NoDBCall(t *testing.T) {
	f := readFixture()
	s := &Service{r: f}
	if _, err := s.List(context.Background(), Actor{}, 10); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("List err = %v, want ErrUnauthenticated", err)
	}
	if _, err := s.Get(context.Background(), Actor{}, 30); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Get err = %v, want ErrUnauthenticated", err)
	}
	if f.calls != 0 {
		t.Fatalf("db calls = %d, want 0", f.calls)
	}
}

func TestList_OwnJobsOnly_OrderPreserved(t *testing.T) {
	s := &Service{r: readFixture()}
	got, err := s.List(context.Background(), Actor{UserID: userA}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != 30 || got[1].ID != 10 {
		t.Fatalf("A jobs = %+v, want [30 10]", got)
	}
	got, err = s.List(context.Background(), Actor{UserID: userB}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 20 {
		t.Fatalf("B jobs = %+v, want [20]", got)
	}
}

func TestList_NoJobs_EmptyNotNil(t *testing.T) {
	s := &Service{r: readFixture()}
	got, err := s.List(context.Background(), Actor{UserID: 99}, 5)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %v, %v; want empty non-nil slice", got, err)
	}
}

func TestList_LimitClamp(t *testing.T) {
	cases := []struct {
		in   int
		want int32
	}{
		{-5, DefaultListLimit}, {0, DefaultListLimit}, {1, 1}, {37, 37},
		{MaxListLimit, MaxListLimit}, {MaxListLimit + 1, MaxListLimit}, {1 << 40, MaxListLimit},
	}
	for _, c := range cases {
		f := readFixture()
		s := &Service{r: f}
		if _, err := s.List(context.Background(), Actor{UserID: userA}, c.in); err != nil {
			t.Fatal(err)
		}
		if f.lastLimit != c.want {
			t.Errorf("limit %d -> %d, want %d", c.in, f.lastLimit, c.want)
		}
	}
}

func TestList_DropsLeakedRows(t *testing.T) {
	f := readFixture()
	f.leak = f.rows // broken query returning everyone's rows
	got, err := (&Service{r: f}).List(context.Background(), Actor{UserID: userB}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 20 {
		t.Fatalf("got %+v, want only job 20", got)
	}
}

func TestGet_Ownership(t *testing.T) {
	ctx := context.Background()
	s := &Service{r: readFixture()}

	j, err := s.Get(ctx, Actor{UserID: userA}, 30)
	if err != nil || j.ID != 30 || j.RequestedBy != userA {
		t.Fatalf("own job: %+v, %v", j, err)
	}

	_, errOther := s.Get(ctx, Actor{UserID: userB}, 30) // A's job
	_, errMissing := s.Get(ctx, Actor{UserID: userB}, 999)
	if errOther != ErrNotFound || errMissing != ErrNotFound {
		t.Fatalf("other=%v missing=%v, want the same ErrNotFound", errOther, errMissing)
	}
	if errOther.Error() != errMissing.Error() {
		t.Fatalf("messages differ: %q vs %q", errOther, errMissing)
	}
}

func TestGet_NonPositiveID_NotFound_NoDBCall(t *testing.T) {
	f := readFixture()
	s := &Service{r: f}
	for _, id := range []int64{0, -1} {
		if _, err := s.Get(context.Background(), Actor{UserID: userA}, id); err != ErrNotFound {
			t.Fatalf("id %d: err = %v, want ErrNotFound", id, err)
		}
	}
	if f.calls != 0 {
		t.Fatalf("db calls = %d, want 0", f.calls)
	}
}

func TestGet_LeakedRow_NotFound(t *testing.T) {
	f := readFixture()
	f.leak = f.rows[:1] // A's job returned to B by a broken query
	if _, err := (&Service{r: f}).Get(context.Background(), Actor{UserID: userB}, 30); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestListGet_DBError_Internal(t *testing.T) {
	boom := errors.New("connection reset")
	f := readFixture()
	f.err = boom
	s := &Service{r: f}

	_, err := s.List(context.Background(), Actor{UserID: userA}, 10)
	if !errors.Is(err, ErrInternal) || !errors.Is(err, boom) {
		t.Fatalf("List err = %v, want ErrInternal wrapping cause", err)
	}
	_, err = s.Get(context.Background(), Actor{UserID: userA}, 30)
	if !errors.Is(err, ErrInternal) || !errors.Is(err, boom) || errors.Is(err, ErrNotFound) {
		t.Fatalf("Get err = %v, want ErrInternal (not ErrNotFound)", err)
	}
}
