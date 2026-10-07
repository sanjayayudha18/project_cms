package notification

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var errProbe = errors.New("dbtxProbe: no real database")

// dbtxProbe records which pool (primary/replica) a Repository method hits
// (same pattern as rolemgmt/repository_topology_test.go).
type dbtxProbe struct {
	name  string
	calls *[]string
}

func (p *dbtxProbe) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	*p.calls = append(*p.calls, p.name)
	return pgconn.CommandTag{}, errProbe
}

func (p *dbtxProbe) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	*p.calls = append(*p.calls, p.name)
	return nil, errProbe
}

func (p *dbtxProbe) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	*p.calls = append(*p.calls, p.name)
	return probeRow{}
}

type probeRow struct{}

func (probeRow) Scan(_ ...any) error { return errProbe }

// Spec AC6: list + unread-count read the replica; mark read/read-all write
// the primary.
func TestRepository_WriteReadTopology(t *testing.T) {
	var calls []string
	repo := NewRepository(&dbtxProbe{name: "primary", calls: &calls}, &dbtxProbe{name: "replica", calls: &calls})
	ctx := context.Background()

	cases := []struct {
		name string
		want string
		call func()
	}{
		{"List", "replica", func() { _, _, _ = repo.List(ctx, 1, false, 1, 20) }},
		{"UnreadCount", "replica", func() { _, _ = repo.UnreadCount(ctx, 1) }},
		{"MarkRead", "primary", func() { _, _ = repo.MarkRead(ctx, 5, 1) }},
		{"MarkAllRead", "primary", func() { _, _ = repo.MarkAllRead(ctx, 1) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls = nil
			tc.call()
			if len(calls) == 0 {
				t.Fatal("no database call made")
			}
			for _, c := range calls {
				if c != tc.want {
					t.Fatalf("calls = %v, want all on %s", calls, tc.want)
				}
			}
		})
	}
}
