package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// poolProbe is a db.DBTX that only records which pool a call landed on.
// Every call fails fast; these tests check routing, not query results.
type poolProbe struct {
	name string
	hits *[]string
}

var errPoolProbe = errors.New("poolProbe: no database")

func (p poolProbe) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	*p.hits = append(*p.hits, p.name)
	return pgconn.CommandTag{}, errPoolProbe
}

func (p poolProbe) Query(context.Context, string, ...any) (pgx.Rows, error) {
	*p.hits = append(*p.hits, p.name)
	return nil, errPoolProbe
}

func (p poolProbe) QueryRow(context.Context, string, ...any) pgx.Row {
	*p.hits = append(*p.hits, p.name)
	return poolProbeRow{}
}

type poolProbeRow struct{}

func (poolProbeRow) Scan(...any) error { return errPoolProbe }

// CLAUDE.md Sec 6: admin list screens read the replica; GetByID feeds the
// maker-checker "before" snapshot and must stay on the primary.
func TestAdminRepositories_ReadTopology(t *testing.T) {
	ctx := context.Background()
	type call func(primary, replica db.DBTX) error
	cases := []struct {
		name string
		call call
		want string
	}{
		{"vault List", func(p, r db.DBTX) error {
			_, err := NewVendorVaultAdminRepository(p, r).List(ctx, db.ListVendorVaultsAdminParams{})
			return err
		}, "replica"},
		{"vault Count", func(p, r db.DBTX) error {
			_, err := NewVendorVaultAdminRepository(p, r).Count(ctx, db.CountVendorVaultsAdminParams{})
			return err
		}, "replica"},
		{"vault GetByID", func(p, r db.DBTX) error { _, err := NewVendorVaultAdminRepository(p, r).GetByID(ctx, 1); return err }, "primary"},
		{"pic List", func(p, r db.DBTX) error {
			_, err := NewVendorPicAdminRepository(p, r).List(ctx, db.ListVendorPicsAdminParams{})
			return err
		}, "replica"},
		{"pic Count", func(p, r db.DBTX) error {
			_, err := NewVendorPicAdminRepository(p, r).Count(ctx, db.CountVendorPicsAdminParams{})
			return err
		}, "replica"},
		{"pic GetByID", func(p, r db.DBTX) error { _, err := NewVendorPicAdminRepository(p, r).GetByID(ctx, 1); return err }, "primary"},
		{"package List", func(p, r db.DBTX) error {
			_, err := NewVendorPackageAdminRepository(p, r).List(ctx, db.ListVendorPackagesAdminParams{})
			return err
		}, "replica"},
		{"package Count", func(p, r db.DBTX) error {
			_, err := NewVendorPackageAdminRepository(p, r).Count(ctx, db.CountVendorPackagesAdminParams{})
			return err
		}, "replica"},
		{"package GetByID", func(p, r db.DBTX) error { _, err := NewVendorPackageAdminRepository(p, r).GetByID(ctx, 1); return err }, "primary"},
		{"price List", func(p, r db.DBTX) error {
			_, err := NewVendorPackagePriceAdminRepository(p, r).List(ctx, db.ListVendorPackagePricesAdminParams{})
			return err
		}, "replica"},
		{"price Count", func(p, r db.DBTX) error {
			_, err := NewVendorPackagePriceAdminRepository(p, r).Count(ctx, db.CountVendorPackagePricesAdminParams{})
			return err
		}, "replica"},
		{"price GetByID", func(p, r db.DBTX) error {
			_, err := NewVendorPackagePriceAdminRepository(p, r).GetByID(ctx, 1)
			return err
		}, "primary"},
		{"branch List", func(p, r db.DBTX) error {
			_, err := NewVendorBranchAdminRepository(p, r).List(ctx, db.ListVendorBranchesAdminParams{})
			return err
		}, "replica"},
		{"branch Count", func(p, r db.DBTX) error {
			_, err := NewVendorBranchAdminRepository(p, r).Count(ctx, db.CountVendorBranchesAdminParams{})
			return err
		}, "replica"},
		{"branch GetByID", func(p, r db.DBTX) error { _, err := NewVendorBranchAdminRepository(p, r).GetByID(ctx, 1); return err }, "primary"},
		{"assignment List", func(p, r db.DBTX) error {
			_, err := NewATMAssignmentAdminRepository(p, r).List(ctx, db.ListATMAssignmentsAdminParams{})
			return err
		}, "replica"},
		{"assignment Count", func(p, r db.DBTX) error {
			_, err := NewATMAssignmentAdminRepository(p, r).Count(ctx, db.CountATMAssignmentsAdminParams{})
			return err
		}, "replica"},
		{"assignment GetByID", func(p, r db.DBTX) error { _, err := NewATMAssignmentAdminRepository(p, r).GetByID(ctx, 1); return err }, "primary"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var hits []string
			err := tc.call(poolProbe{"primary", &hits}, poolProbe{"replica", &hits})
			if !errors.Is(err, errPoolProbe) {
				t.Fatalf("want probe error, got %v", err)
			}
			if len(hits) != 1 || hits[0] != tc.want {
				t.Fatalf("want one call on %s, got %v", tc.want, hits)
			}
		})
	}
}
