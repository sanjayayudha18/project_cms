//go:build integration

package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// TestATMAdminRepository exercises List/Count/GetByID/FindByTerminalID/
// ListLocations/LocationExists against a real DB transaction
// rolled back on cleanup — same harness convention as
// vendor_admin_repository_test.go / user_admin_repository_test.go.
func TestATMAdminRepository(t *testing.T) {
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

	var regionID int64
	if err := tx.QueryRow(ctx, "SELECT id FROM regions ORDER BY id LIMIT 1").Scan(&regionID); err != nil {
		t.Fatalf("fetch seed region id: %v", err)
	}

	var locationID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO locations (region_id, type, name, address_line1, city_or_regency, province, country_code)
		 VALUES ($1, 'branch', $2, 'Jl. Test 1', 'Jakarta', 'DKI Jakarta', 'ID') RETURNING id`,
		regionID, "Test Location "+tag,
	).Scan(&locationID); err != nil {
		t.Fatalf("seed location: %v", err)
	}

	termActive := "TATM_ACT_" + tag
	termDisabled := "TATM_DIS_" + tag

	var activeID, disabledID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO atms (terminal_id, location_id, machine_type, brand, model, operation_hours, deployment_type, is_active)
		 VALUES ($1, $2, 'ATM', 'NCR', 'SelfServ', '24 Hours', 'Onsite', true) RETURNING id`,
		termActive, locationID,
	).Scan(&activeID); err != nil {
		t.Fatalf("seed active atm: %v", err)
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO atms (terminal_id, location_id, machine_type, brand, model, operation_hours, deployment_type, is_active, deleted_at)
		 VALUES ($1, $2, 'ATM', 'NCR', 'SelfServ', '24 Hours', 'Onsite', false, now()) RETURNING id`,
		termDisabled, locationID,
	).Scan(&disabledID); err != nil {
		t.Fatalf("seed disabled atm: %v", err)
	}

	repo := NewATMAdminRepository(tx)

	t.Run("List q filter matches terminal_id, default active status excludes disabled", func(t *testing.T) {
		q := tag
		got, err := repo.List(ctx, db.ListATMsAdminParams{
			Q: &q, Status: "active", PageLimit: 100, PageOffset: 0,
		})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 1 || got[0].ID != activeID {
			t.Fatalf("List(active) = %+v, want exactly the active seed atm", got)
		}
		if got[0].LocationName == nil || *got[0].LocationName != "Test Location "+tag {
			t.Errorf("List location_name = %v, want joined location name", got[0].LocationName)
		}
	})

	t.Run("List status=disabled returns only the disabled seed atm", func(t *testing.T) {
		q := tag
		got, err := repo.List(ctx, db.ListATMsAdminParams{
			Q: &q, Status: "disabled", PageLimit: 100, PageOffset: 0,
		})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 1 || got[0].ID != disabledID {
			t.Fatalf("List(disabled) = %+v, want exactly the disabled seed atm", got)
		}
	})

	t.Run("List status=all returns both, Count matches, brand/machine_type/deployment_type/location_id filters narrow", func(t *testing.T) {
		q := tag
		got, err := repo.List(ctx, db.ListATMsAdminParams{Q: &q, Status: "all", PageLimit: 100, PageOffset: 0})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("len(List(all)) = %d, want 2", len(got))
		}
		count, err := repo.Count(ctx, db.CountATMsAdminParams{Q: &q, Status: "all"})
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if count != 2 {
			t.Errorf("Count(all) = %d, want 2", count)
		}

		brand := "NCR"
		machineType := "ATM"
		deploymentType := "Onsite"
		got, err = repo.List(ctx, db.ListATMsAdminParams{
			Q: &q, Brand: &brand, MachineType: &machineType, DeploymentType: &deploymentType,
			LocationID: &locationID, Status: "all", PageLimit: 100, PageOffset: 0,
		})
		if err != nil {
			t.Fatalf("List with filters: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("List with matching filters = %d rows, want 2", len(got))
		}

		otherBrand := "Diebold"
		got, err = repo.List(ctx, db.ListATMsAdminParams{Q: &q, Brand: &otherBrand, Status: "all", PageLimit: 100, PageOffset: 0})
		if err != nil {
			t.Fatalf("List with non-matching brand: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("List with non-matching brand = %d rows, want 0", len(got))
		}
	})

	t.Run("Count vs summed pages", func(t *testing.T) {
		q := tag
		f := db.CountATMsAdminParams{Q: &q, Status: "all"}
		count, err := repo.Count(ctx, f)
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		page1, err := repo.List(ctx, db.ListATMsAdminParams{Q: &q, Status: "all", PageLimit: 1, PageOffset: 0})
		if err != nil {
			t.Fatalf("List page1: %v", err)
		}
		page2, err := repo.List(ctx, db.ListATMsAdminParams{Q: &q, Status: "all", PageLimit: 1, PageOffset: 1})
		if err != nil {
			t.Fatalf("List page2: %v", err)
		}
		if int64(len(page1)+len(page2)) != count {
			t.Errorf("pages summed to %d rows, Count = %d, want equal", len(page1)+len(page2), count)
		}
	})

	t.Run("GetByID includes soft-deleted rows, nil for non-existent id", func(t *testing.T) {
		got, err := repo.GetByID(ctx, disabledID)
		if err != nil {
			t.Fatalf("GetByID(disabled): %v", err)
		}
		if got == nil || got.IsActive {
			t.Fatalf("GetByID(disabled) = %+v, want a row with IsActive=false", got)
		}
		got, err = repo.GetByID(ctx, -1)
		if err != nil {
			t.Fatalf("GetByID(-1): %v", err)
		}
		if got != nil {
			t.Errorf("GetByID(-1) = %+v, want nil", got)
		}
	})

	// The repository has no write methods since T4.2 (writes happen in
	// service.ATMApplier, covered by the masterdata approval integration
	// tests); FindByTerminalID is the submit-time uniqueness lookup that remains.
	t.Run("FindByTerminalID finds active and soft-disabled terminals, nil for an unknown one", func(t *testing.T) {
		for term, want := range map[string]int64{termActive: activeID, termDisabled: disabledID} {
			foundID, err := repo.FindByTerminalID(ctx, term)
			if err != nil {
				t.Fatalf("FindByTerminalID(%s): %v", term, err)
			}
			if foundID == nil || *foundID != want {
				t.Errorf("FindByTerminalID(%s) = %v, want %d (soft-disabled terminal_ids stay reserved)", term, foundID, want)
			}
		}
		foundID, err := repo.FindByTerminalID(ctx, "TATM_NOPE_"+tag)
		if err != nil {
			t.Fatalf("FindByTerminalID(unknown): %v", err)
		}
		if foundID != nil {
			t.Errorf("FindByTerminalID(unknown) = %v, want nil", foundID)
		}
	})

	t.Run("LocationExists true/false", func(t *testing.T) {
		exists, err := repo.LocationExists(ctx, locationID)
		if err != nil {
			t.Fatalf("LocationExists(real): %v", err)
		}
		if !exists {
			t.Error("LocationExists(real) = false, want true")
		}
		exists, err = repo.LocationExists(ctx, -1)
		if err != nil {
			t.Fatalf("LocationExists(-1): %v", err)
		}
		if exists {
			t.Error("LocationExists(-1) = true, want false")
		}
	})

	t.Run("ListLocationsForSelect includes the seed location", func(t *testing.T) {
		// Ordering itself is asserted by the query's ORDER BY name ASC, not
		// re-checked here in Go: Postgres orders by the DB's locale
		// collation, which does not agree byte-for-byte with Go's simple
		// string "<" comparison across the ~800 real mixed-case location
		// names already in this table.
		got, err := repo.ListLocations(ctx)
		if err != nil {
			t.Fatalf("ListLocations: %v", err)
		}
		found := false
		for _, l := range got {
			if l.ID == locationID {
				found = true
			}
		}
		if !found {
			t.Error("ListLocations did not include the seed location")
		}
	})
}
