//go:build integration

package repository

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// idsOf extracts the ids of one typed page (or passes the error through).
func idsOf[R any](rows []R, err error, id func(R) int64) ([]int64, error) {
	if err != nil {
		return nil, err
	}
	out := make([]int64, len(rows))
	for i, r := range rows {
		out[i] = id(r)
	}
	return out, nil
}

// TestMasterDataExportRepository_KeysetPagingIsCompleteAndExact runs the real
// keyset queries against the real tables (read-only, so no cleanup): for every
// entity and status, paging with a small batch must return exactly as many rows
// as the table holds for that status -- no row dropped or duplicated, ids
// strictly increasing -- which also proves the natural-key JOINs lose nothing.
func TestMasterDataExportRepository_KeysetPagingIsCompleteAndExact(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}
	t.Cleanup(pool.Close)
	repo := NewMasterDataExportRepository(pool)

	// pageAll walks the keyset cursor with `batch` rows per page and returns every id seen.
	const batch = 7
	pageAll := func(t *testing.T, fetch func(after int64, limit int32) ([]int64, error)) []int64 {
		t.Helper()
		var all []int64
		var after int64
		for pages := 0; ; pages++ {
			if pages > 100000 {
				t.Fatal("paging did not terminate")
			}
			ids, err := fetch(after, batch)
			if err != nil {
				t.Fatalf("fetch after %d: %v", after, err)
			}
			for _, id := range ids {
				if id <= after {
					t.Fatalf("id %d is not > cursor %d: keyset ordering broken", id, after)
				}
				after = id
				all = append(all, id)
			}
			if len(ids) < batch {
				return all
			}
		}
	}

	type fetchFn func(after int64, limit int32) ([]int64, error)
	type entity struct {
		name  string
		table string
		where map[string]string // status -> SQL predicate on the base table
		fetch func(status string) fetchFn
	}
	softDeleted := map[string]string{"active": "deleted_at IS NULL", "disabled": "deleted_at IS NOT NULL", "all": "TRUE"}
	entities := []entity{
		{"vendors", "vendors", softDeleted, func(s string) fetchFn {
			return func(a int64, l int32) ([]int64, error) {
				rows, err := repo.Vendors(ctx, a, s, l)
				return idsOf(rows, err, func(r db.ExportVendorsBatchRow) int64 { return r.ID })
			}
		}},
		{"vendor_branches", "vendor_branches", softDeleted, func(s string) fetchFn {
			return func(a int64, l int32) ([]int64, error) {
				rows, err := repo.VendorBranches(ctx, a, s, l)
				return idsOf(rows, err, func(r db.ExportVendorBranchesBatchRow) int64 { return r.ID })
			}
		}},
		{"vendor_vaults", "vendor_vaults", softDeleted, func(s string) fetchFn {
			return func(a int64, l int32) ([]int64, error) {
				rows, err := repo.VendorVaults(ctx, a, s, l)
				return idsOf(rows, err, func(r db.ExportVendorVaultsBatchRow) int64 { return r.ID })
			}
		}},
		{"vendor_pics", "vendor_pics", softDeleted, func(s string) fetchFn {
			return func(a int64, l int32) ([]int64, error) {
				rows, err := repo.VendorPICs(ctx, a, s, l)
				return idsOf(rows, err, func(r db.ExportVendorPicsBatchRow) int64 { return r.ID })
			}
		}},
		{"atms", "atms", softDeleted, func(s string) fetchFn {
			return func(a int64, l int32) ([]int64, error) {
				rows, err := repo.ATMs(ctx, a, s, l)
				return idsOf(rows, err, func(r db.ExportATMsBatchRow) int64 { return r.ID })
			}
		}},
		{"atm_assignments", "atm_vendor_packages", map[string]string{"active": "is_active", "disabled": "NOT is_active", "all": "TRUE"}, func(s string) fetchFn {
			return func(a int64, l int32) ([]int64, error) {
				rows, err := repo.ATMAssignments(ctx, a, s, l)
				return idsOf(rows, err, func(r db.ExportATMAssignmentsBatchRow) int64 { return r.ID })
			}
		}},
	}

	for _, e := range entities {
		for _, status := range []string{"active", "disabled", "all"} {
			t.Run(e.name+"/"+status, func(t *testing.T) {
				var want int
				if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+e.table+" WHERE "+e.where[status]).Scan(&want); err != nil {
					t.Fatalf("counting %s: %v", e.table, err)
				}
				ids := pageAll(t, e.fetch(status))
				if len(ids) != want {
					t.Errorf("exported %d rows, table has %d (%s): a row was dropped or a JOIN lost it", len(ids), want, status)
				}
			})
		}
	}

	t.Run("atms: money is the exact stored text, empty for NULL", func(t *testing.T) {
		type stored struct{ capacity, low string }
		want := map[int64]stored{}
		rows, err := pool.Query(ctx, `SELECT id, COALESCE(capacity_amount::text,''), COALESCE(low_threshold_amount::text,'') FROM atms ORDER BY id LIMIT 300`)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var id int64
			var s stored
			if err := rows.Scan(&id, &s.capacity, &s.low); err != nil {
				t.Fatal(err)
			}
			want[id] = s
		}
		rows.Close()

		exported, err := repo.ATMs(ctx, 0, "all", 300)
		if err != nil {
			t.Fatal(err)
		}
		if len(exported) == 0 {
			t.Skip("no ATMs in the database")
		}
		for _, a := range exported {
			if w := want[a.ID]; a.CapacityAmount != w.capacity || a.LowThresholdAmount != w.low {
				t.Fatalf("atm %d: capacity=%q low=%q, want %q / %q", a.ID, a.CapacityAmount, a.LowThresholdAmount, w.capacity, w.low)
			}
		}
	})
}
