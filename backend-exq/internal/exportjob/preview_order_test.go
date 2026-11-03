package exportjob

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"
)

// countingReader records snapshot opens; Querier is never used by stubPreview.
type countingReader struct{ begins int }

func (r *countingReader) Begin(context.Context) (Querier, func(), error) {
	r.begins++
	return nil, func() {}, nil
}

// stubPreview serves rows with ids 1..n.
type stubPreview struct{ n int64 }

func (stubPreview) Header() []string { return []string{"id"} }

func (stubPreview) Produce(context.Context, Querier, Params, time.Time, func([]string) error) error {
	return nil
}

func (s stubPreview) Count(context.Context, Querier, Params, time.Time) (int64, error) {
	return s.n, nil
}

func (s stubPreview) Page(_ context.Context, _ Querier, _ Params, _ time.Time, after int64, limit int) ([][]string, int64, error) {
	var rows [][]string
	last := after
	for id := after + 1; id <= s.n && len(rows) < limit; id++ {
		rows = append(rows, []string{strconv.FormatInt(id, 10)})
		last = id
	}
	return rows, last, nil
}

func newStubPreviewer(n int64) (*Previewer, *countingReader) {
	r := &countingReader{}
	return &Previewer{
		reader:    r,
		producers: map[Kind]map[string]Producer{KindMasterDataGlobal: {"vendors": stubPreview{n: n}}},
		now:       time.Now,
	}, r
}

// Global scope needs no scope lookup, so a nil scope (panics if touched) is safe here.
func TestPreviewForbiddenRoleOpensNoSnapshot(t *testing.T) {
	v, r := newStubPreviewer(3)
	p := Params{Kind: KindMasterDataGlobal, Entity: "vendors"}

	for _, role := range []string{"", "ATM-USER", "VENDOR-USER"} {
		_, err := v.Preview(context.Background(), Actor{UserID: 1, Role: role}, p, 0, 10)
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("role %q: err = %v, want ErrForbidden", role, err)
		}
	}
	if _, err := v.Preview(context.Background(), Actor{Role: "ADMIN"}, p, 0, 10); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("user 0: err = %v, want ErrUnauthenticated", err)
	}
	if r.begins != 0 {
		t.Fatalf("snapshot opened %d times for rejected callers", r.begins)
	}
}

func TestPreviewNextAfter(t *testing.T) {
	p := Params{Kind: KindMasterDataGlobal, Entity: "vendors"}
	tests := []struct {
		name         string
		n, after     int64
		limit, rows  int
		wantNextNull bool
		wantNext     int64
	}{
		{"more pages", 5, 0, 2, 2, false, 2},
		{"exact last page", 4, 2, 2, 2, true, 0},
		{"short last page", 3, 2, 2, 1, true, 0},
		{"empty", 0, 0, 2, 0, true, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v, r := newStubPreviewer(tc.n)
			got, err := v.Preview(context.Background(), Actor{UserID: 1, Role: "ADMIN"}, p, tc.after, tc.limit)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Rows) != tc.rows || got.Total != tc.n || got.Limit != tc.limit || r.begins != 1 {
				t.Fatalf("got %+v, begins %d", got, r.begins)
			}
			if got.Rows == nil {
				t.Fatal("rows must be [] not null")
			}
			if (got.NextAfter == nil) != tc.wantNextNull || (got.NextAfter != nil && *got.NextAfter != tc.wantNext) {
				t.Fatalf("next_after = %v, want null=%v / %d", got.NextAfter, tc.wantNextNull, tc.wantNext)
			}
		})
	}
}

func TestPreviewReportKindIsUnknownEntity(t *testing.T) {
	v, r := newStubPreviewer(1)
	v.producers[KindDsrLateDaily] = map[string]Producer{"x": producer{}}
	role := KindRoles[KindDsrLateDaily][0]
	_, err := v.Preview(context.Background(), Actor{UserID: 1, Role: role}, Params{Kind: KindDsrLateDaily, Entity: "x"}, 0, 10)
	if !errors.Is(err, ErrUnknownEntity) || r.begins != 0 {
		t.Fatalf("err = %v, begins %d", err, r.begins)
	}
}
