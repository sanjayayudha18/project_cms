package rolemgmt

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// errProbe is returned by every dbtxProbe call -- these tests only care
// which probe (primary vs replica) a Repository method touches, not the
// query result, so every call fails fast without needing a real Rows/Row
// implementation (Query returns the error before its caller ever reads
// rows; QueryRow's error surfaces through probeRow.Scan below).
var errProbe = errors.New("dbtxProbe: no real database")

// dbtxProbe is a minimal db.DBTX that records which pool (by name) each
// call landed on, for Property 9 (write/read topology): writes and
// read-after-write must target the primary pool; list/reporting reads must
// target the replica pool (Req 8.1-8.3).
type dbtxProbe struct {
	name  string
	calls *[]string
}

func (p *dbtxProbe) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	*p.calls = append(*p.calls, p.name+":Exec")
	return pgconn.CommandTag{}, errProbe
}

func (p *dbtxProbe) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	*p.calls = append(*p.calls, p.name+":Query")
	return nil, errProbe
}

func (p *dbtxProbe) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	*p.calls = append(*p.calls, p.name+":QueryRow")
	return probeRow{}
}

type probeRow struct{}

func (probeRow) Scan(_ ...any) error { return errProbe }

// TestRepository_WriteReadTopology is Property 9: for any write, the
// operation targets the primary pool; for any read-after-write, the read
// targets the primary pool; for any list/reporting read, the operation
// targets the replica pool.
func TestRepository_WriteReadTopology(t *testing.T) {
	var calls []string
	primary := &dbtxProbe{name: "primary", calls: &calls}
	replica := &dbtxProbe{name: "replica", calls: &calls}
	repo := NewRepository(primary, replica)
	ctx := context.Background()

	cases := []struct {
		name     string
		wantPool string
		call     func()
	}{
		{
			name:     "ListCatalog is a list read -> replica",
			wantPool: "replica",
			call:     func() { _, _ = repo.ListCatalog(ctx) },
		},
		{
			name:     "CatalogEntriesExist is a reporting/validation read -> replica",
			wantPool: "replica",
			call:     func() { _, _ = repo.CatalogEntriesExist(ctx, []int64{1}) },
		},
		{
			name:     "ListRolesWithPermissions is a list read -> replica",
			wantPool: "replica",
			call:     func() { _, _ = repo.ListRolesWithPermissions(ctx) },
		},
		{
			name:     "HasPermission (runtime evaluator) is a read -> replica",
			wantPool: "replica",
			call:     func() { _, _ = repo.HasPermission(ctx, "ADMIN", "dashboard") },
		},
		{
			name:     "FindRoleByName is a uniqueness pre-check ahead of a write -> primary (avoids replica lag masking a duplicate)",
			wantPool: "primary",
			call:     func() { _, _ = repo.FindRoleByName(ctx, "X") },
		},
		{
			name:     "CreateRole is a write -> primary",
			wantPool: "primary",
			call:     func() { _, _ = repo.CreateRole(ctx, db.CreateRoleParams{Role: "X"}) },
		},
		{
			name:     "GetRole is read-after-write -> primary",
			wantPool: "primary",
			call:     func() { _, _ = repo.GetRole(ctx, 1) },
		},
		{
			name:     "ListRolePermissionIDs is a before-snapshot read ahead of a write -> primary",
			wantPool: "primary",
			call:     func() { _, _ = repo.ListRolePermissionIDs(ctx, 1) },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls = nil
			tc.call()
			if len(calls) == 0 {
				t.Fatalf("no db call recorded")
			}
			for _, c := range calls {
				if c[:len(tc.wantPool)] != tc.wantPool {
					t.Fatalf("call %q, want it to target the %s pool", c, tc.wantPool)
				}
			}
		})
	}
}
