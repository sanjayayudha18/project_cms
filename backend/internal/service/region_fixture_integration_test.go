//go:build integration

package service

import (
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Since migration 042 vendor_branches.region_code is an FK to regions.code and
// the branch's region name is regions.region (region-standardization task 5),
// so a fixture branch with a region needs a regions row first.

type regionExecer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// seededRegions holds the region codes this run inserted outside a test tx;
// TestMain removes them once every test (and its cleanup) is done.
var seededRegions sync.Map

// itRegionCode derives a stable ^[A-Z0-9]{2,10}$ region code from a fixture key.
func itRegionCode(key string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return fmt.Sprintf("IT%08X", h.Sum32())
}

// seedTestRegion makes sure regions has (code, name). Inside a test tx the row
// rolls back with it; on a pool it is swept by TestMain (t.Cleanup runs LIFO,
// so it could run before the fixture's own branch cleanup and leave the row).
func seedTestRegion(t testing.TB, q regionExecer, code, name string) {
	t.Helper()
	tag, err := q.Exec(context.Background(), `INSERT INTO regions (code, region) VALUES ($1, $2) ON CONFLICT (code) DO NOTHING`, code, name)
	if err != nil {
		t.Fatalf("seed region %s: %v", code, err)
	}
	if _, inTx := q.(pgx.Tx); !inTx && tag.RowsAffected() == 1 {
		seededRegions.Store(code, name)
	}
}

func TestMain(m *testing.M) {
	code := m.Run()
	sweepSeededRegions()
	os.Exit(code)
}

// sweepSeededRegions deletes the regions this run created, only when nothing
// references them any more; rows from earlier runs are never touched.
func sweepSeededRegions() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sweep test regions: %v\n", err)
		return
	}
	defer func() { _ = conn.Close(ctx) }()
	seededRegions.Range(func(k, v any) bool {
		if _, err := conn.Exec(ctx, `
			DELETE FROM regions r WHERE r.code = $1 AND r.region = $2
			  AND NOT EXISTS (SELECT 1 FROM vendor_branches b WHERE b.region_code = r.code)
			  AND NOT EXISTS (SELECT 1 FROM locations l WHERE l.region_id = r.id)
			  AND NOT EXISTS (SELECT 1 FROM acm_area_regions a WHERE a.region_code = r.code)
			  AND NOT EXISTS (SELECT 1 FROM cash_count_area_regions c WHERE c.region_code = r.code)
			  AND NOT EXISTS (SELECT 1 FROM cash_count_pics p WHERE p.region_code = r.code)`, k, v); err != nil {
			fmt.Fprintf(os.Stderr, "sweep test region %v: %v\n", k, err)
		}
		return true
	})
}
