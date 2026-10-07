//go:build integration

package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// TestRegionAdminRepository exercises RegionAdminRepository against a real
// DB transaction rolled back on cleanup -- same harness convention as
// audit_log_repository_test.go. code is randomized-by-time so this test
// never collides with the seeded regions or a concurrent test run.
func TestRegionAdminRepository(t *testing.T) {
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

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { tx.Rollback(ctx) })

	tag := time.Now().Format("150405.000000000")
	code := "RARTEST" + tag

	// primary and replica both point at the same test tx -- this only
	// exercises correctness of each query, not primary-vs-replica routing
	// (design property 10 is an I/O-routing invariant, not testable inside a
	// single shared tx; DATABASE_REPLICA_URL wiring is exercised manually).
	repo := NewRegionAdminRepository(tx, tx)

	t.Run("create then get roundtrip", func(t *testing.T) {
		name := "Region Repo Test"
		created, err := repo.CreateTx(ctx, tx, db.CreateRegionAdminParams{Code: code, Region: &name})
		if err != nil {
			t.Fatalf("CreateTx: %v", err)
		}
		if created.Code != code {
			t.Errorf("Code = %q, want %q", created.Code, code)
		}
		if !created.IsActive {
			t.Error("IsActive = false, want true on create")
		}

		got, err := repo.GetByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got == nil {
			t.Fatal("GetByID returned nil, want the created row")
		}
		if got.Code != code {
			t.Errorf("GetByID Code = %q, want %q", got.Code, code)
		}
		if got.LocationCount != 0 {
			t.Errorf("LocationCount = %d, want 0 (no locations seeded yet)", got.LocationCount)
		}
	})

	t.Run("uq_regions_code surfaces as a unique violation", func(t *testing.T) {
		name := "Duplicate"
		// Runs inside its own savepoint (pgx.Tx.Begin on a Tx is a pseudo
		// nested transaction) and rolls it back afterward -- a real
		// constraint violation aborts the whole surrounding transaction in
		// Postgres, which would otherwise poison every later subtest sharing
		// the outer `tx`.
		sp, err := tx.Begin(ctx)
		if err != nil {
			t.Fatalf("begin savepoint: %v", err)
		}
		_, err = repo.CreateTx(ctx, sp, db.CreateRegionAdminParams{Code: code, Region: &name})
		if err == nil {
			t.Fatal("CreateTx with a duplicate code succeeded, want a unique-violation error")
		}
		var pgErr *pgconn.PgError
		if ok := errors.As(err, &pgErr); !ok || pgErr.Code != "23505" {
			t.Errorf("error = %v, want a pgconn.PgError with code 23505", err)
		}
		_ = sp.Rollback(ctx)
	})

	t.Run("FindByCode returns the id for an existing code", func(t *testing.T) {
		id, err := repo.FindByCode(ctx, code)
		if err != nil {
			t.Fatalf("FindByCode: %v", err)
		}
		if id == nil {
			t.Fatal("FindByCode returned nil, want an id")
		}
	})

	t.Run("FindByCode returns nil for an unused code", func(t *testing.T) {
		id, err := repo.FindByCode(ctx, code+"_UNUSED")
		if err != nil {
			t.Fatalf("FindByCode: %v", err)
		}
		if id != nil {
			t.Errorf("FindByCode(unused) = %v, want nil", *id)
		}
	})

	t.Run("list filter matrix: q on code, q on region, status, pagination count", func(t *testing.T) {
		nameA := "Alpha " + tag
		nameB := "Beta " + tag
		regA, err := repo.CreateTx(ctx, tx, db.CreateRegionAdminParams{Code: code + "A", Region: &nameA})
		if err != nil {
			t.Fatalf("CreateTx regA: %v", err)
		}
		regB, err := repo.CreateTx(ctx, tx, db.CreateRegionAdminParams{Code: code + "B", Region: &nameB})
		if err != nil {
			t.Fatalf("CreateTx regB: %v", err)
		}
		if _, err := repo.SetActiveTx(ctx, tx, regB.ID, false); err != nil {
			t.Fatalf("SetActiveTx disable regB: %v", err)
		}

		qCode := code + "A"
		gotByCode, err := repo.List(ctx, db.ListRegionsAdminParams{Q: &qCode, Status: "all", PageLimit: 100})
		if err != nil {
			t.Fatalf("List by code: %v", err)
		}
		if len(gotByCode) != 1 || gotByCode[0].ID != regA.ID {
			t.Errorf("List(q=%q) = %+v, want exactly regA", qCode, gotByCode)
		}

		qName := "Beta " + tag
		gotByName, err := repo.List(ctx, db.ListRegionsAdminParams{Q: &qName, Status: "all", PageLimit: 100})
		if err != nil {
			t.Fatalf("List by region name: %v", err)
		}
		if len(gotByName) != 1 || gotByName[0].ID != regB.ID {
			t.Errorf("List(q=%q) = %+v, want exactly regB", qName, gotByName)
		}

		qPrefix := code
		gotActive, err := repo.List(ctx, db.ListRegionsAdminParams{Q: &qPrefix, Status: "active", PageLimit: 100})
		if err != nil {
			t.Fatalf("List status=active: %v", err)
		}
		for _, r := range gotActive {
			if r.ID == regB.ID {
				t.Error("List status=active includes the disabled region")
			}
			if !r.IsActive {
				t.Errorf("List status=active returned an inactive row: %+v", r)
			}
		}

		gotInactive, err := repo.List(ctx, db.ListRegionsAdminParams{Q: &qPrefix, Status: "inactive", PageLimit: 100})
		if err != nil {
			t.Fatalf("List status=inactive: %v", err)
		}
		foundB := false
		for _, r := range gotInactive {
			if r.ID == regB.ID {
				foundB = true
			}
			if r.IsActive {
				t.Errorf("List status=inactive returned an active row: %+v", r)
			}
		}
		if !foundB {
			t.Error("List status=inactive did not include the disabled region")
		}

		count, err := repo.Count(ctx, db.CountRegionsAdminParams{Q: &qPrefix, Status: "all"})
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		page1, err := repo.List(ctx, db.ListRegionsAdminParams{Q: &qPrefix, Status: "all", PageLimit: 1, PageOffset: 0})
		if err != nil {
			t.Fatalf("List page1: %v", err)
		}
		page2, err := repo.List(ctx, db.ListRegionsAdminParams{Q: &qPrefix, Status: "all", PageLimit: 1, PageOffset: 1})
		if err != nil {
			t.Fatalf("List page2: %v", err)
		}
		page3, err := repo.List(ctx, db.ListRegionsAdminParams{Q: &qPrefix, Status: "all", PageLimit: 1, PageOffset: 2})
		if err != nil {
			t.Fatalf("List page3: %v", err)
		}
		total := len(page1) + len(page2) + len(page3)
		if int64(total) != count {
			t.Errorf("pages summed to %d rows, Count = %d, want equal", total, count)
		}
	})

	t.Run("SetActiveTx toggles is_active + deleted_at (disable then enable)", func(t *testing.T) {
		name := "Toggle " + tag
		created, err := repo.CreateTx(ctx, tx, db.CreateRegionAdminParams{Code: code + "TOGGLE", Region: &name})
		if err != nil {
			t.Fatalf("CreateTx: %v", err)
		}

		disabled, err := repo.SetActiveTx(ctx, tx, created.ID, false)
		if err != nil {
			t.Fatalf("SetActiveTx disable: %v", err)
		}
		if disabled.IsActive {
			t.Error("after disable: IsActive = true, want false")
		}
		if !disabled.DeletedAt.Valid {
			t.Error("after disable: DeletedAt not set")
		}

		enabled, err := repo.SetActiveTx(ctx, tx, created.ID, true)
		if err != nil {
			t.Fatalf("SetActiveTx enable: %v", err)
		}
		if !enabled.IsActive {
			t.Error("after enable: IsActive = false, want true")
		}
		if enabled.DeletedAt.Valid {
			t.Error("after enable: DeletedAt still set, want cleared")
		}
	})

	t.Run("CountActiveLocations counts only locations referencing the region", func(t *testing.T) {
		name := "WithLocations " + tag
		created, err := repo.CreateTx(ctx, tx, db.CreateRegionAdminParams{Code: code + "LOC", Region: &name})
		if err != nil {
			t.Fatalf("CreateTx: %v", err)
		}

		zero, err := repo.CountActiveLocations(ctx, created.ID)
		if err != nil {
			t.Fatalf("CountActiveLocations (before seeding): %v", err)
		}
		if zero != 0 {
			t.Fatalf("CountActiveLocations = %d, want 0 before seeding any location", zero)
		}

		for i := 0; i < 2; i++ {
			_, err := tx.Exec(ctx,
				`INSERT INTO locations (region_id, type, name, address_line1, city_or_regency, province, country_code)
				 VALUES ($1, 'branch', 'Loc Repo Test', 'Jl. Test No. 1', 'Jakarta', 'DKI Jakarta', 'ID')`,
				created.ID,
			)
			if err != nil {
				t.Fatalf("seed location %d: %v", i, err)
			}
		}

		got, err := repo.CountActiveLocations(ctx, created.ID)
		if err != nil {
			t.Fatalf("CountActiveLocations: %v", err)
		}
		if got != 2 {
			t.Errorf("CountActiveLocations = %d, want 2", got)
		}

		gotByID, err := repo.GetByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if gotByID.LocationCount != 2 {
			t.Errorf("GetByID LocationCount = %d, want 2 (matches CountActiveLocations)", gotByID.LocationCount)
		}
	})

	t.Run("UpdateNameTx updates region without touching code", func(t *testing.T) {
		name := "Original " + tag
		created, err := repo.CreateTx(ctx, tx, db.CreateRegionAdminParams{Code: code + "RENAME", Region: &name})
		if err != nil {
			t.Fatalf("CreateTx: %v", err)
		}

		updated, err := repo.UpdateNameTx(ctx, tx, created.ID, "Renamed "+tag)
		if err != nil {
			t.Fatalf("UpdateNameTx: %v", err)
		}
		if updated.Code != created.Code {
			t.Errorf("Code changed to %q, want unchanged %q", updated.Code, created.Code)
		}
		if updated.Region == nil || *updated.Region != "Renamed "+tag {
			t.Errorf("Region = %v, want %q", updated.Region, "Renamed "+tag)
		}
	})
}
