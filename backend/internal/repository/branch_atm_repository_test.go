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

// TestBranchATMRepository exercises ListBranchATMs/CountBranchATMs/
// GetVendorBranchVendorID against a real DB transaction rolled back on
// cleanup -- same harness convention as vendor_branch_admin_repository_test.go.
// This is where the SQL-level correctness the fake repo (branch_atm_test.go)
// cannot prove actually gets exercised: the real DISTINCT ON dedup,
// COUNT(DISTINCT ...), and the active-assignment window filter.
func TestBranchATMRepository(t *testing.T) {
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

	var vendorA, vendorB int64
	rows, err := tx.Query(ctx, `SELECT id FROM vendors ORDER BY id LIMIT 2`)
	if err != nil {
		t.Fatalf("loading seeded vendors: %v", err)
	}
	var vendorIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scanning vendor id: %v", err)
		}
		vendorIDs = append(vendorIDs, id)
	}
	rows.Close()
	if len(vendorIDs) < 2 {
		t.Skip("fewer than 2 seeded vendors — skipping integration test")
	}
	vendorA, vendorB = vendorIDs[0], vendorIDs[1]

	var locationID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM locations LIMIT 1`).Scan(&locationID); err != nil {
		t.Fatalf("loading a seeded location: %v", err)
	}

	tag := time.Now().Format("150405.000000000")
	today := time.Now().Format("2006-01-02")
	longAgoStart := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	longAgoEnd := time.Now().AddDate(0, 0, -10).Format("2006-01-02")

	var branchA, branchB int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name) VALUES ($1, $2, $3) RETURNING id`,
		vendorA, "BATM_A_"+tag, "Branch A "+tag,
	).Scan(&branchA); err != nil {
		t.Fatalf("seed branch A: %v", err)
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name) VALUES ($1, $2, $3) RETURNING id`,
		vendorB, "BATM_B_"+tag, "Branch B "+tag,
	).Scan(&branchB); err != nil {
		t.Fatalf("seed branch B: %v", err)
	}

	insertPackage := func(branchID int64, code string) int64 {
		var id int64
		if err := tx.QueryRow(ctx,
			`INSERT INTO vendor_packages_branch (vendor_branch_id, package_code, machine_group, price_class)
			 VALUES ($1, $2, 'ATM', 'REGULAR') RETURNING id`,
			branchID, code,
		).Scan(&id); err != nil {
			t.Fatalf("seed package %s: %v", code, err)
		}
		return id
	}
	pkg1 := insertPackage(branchA, "PKG1_"+tag)
	pkg2 := insertPackage(branchA, "PKG2_"+tag)
	pkgB := insertPackage(branchB, "PKGB_"+tag)

	insertATM := func(terminalID string) int64 {
		var id int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO atms (terminal_id, location_id, machine_type, brand, model, operation_hours, deployment_type, is_active)
			VALUES ($1, $2, 'ATM', 'Hyosung', 'TestModel', '24H', 'ONSITE', true)
			RETURNING id
		`, terminalID, locationID).Scan(&id); err != nil {
			t.Fatalf("seed atm %s: %v", terminalID, err)
		}
		return id
	}
	// atmFirst/atmSecond: one active assignment each, to branchA's two
	// different packages -- terminal_id chosen to prove terminal_id ASC
	// ordering (Req 4.1), not insertion order.
	//
	// NOTE on dedup (Req 1.2): the DISTINCT ON (a.id) dedup in
	// ListBranchATMs cannot be exercised here with two SIMULTANEOUSLY
	// active rows for the same ATM -- the pre-existing exclusion constraint
	// atm_vendor_packages_no_overlap (migration 003) forbids two active
	// periods for the same atm_id from overlapping in time, regardless of
	// package, so at most one row can ever satisfy the "covers today"
	// filter per ATM in real data. The dedup branch is still correct
	// (proven by branch_atm_test.go's Property 1/2 against a fake repo
	// that isn't bound by this DB constraint) but is unreachable via
	// legitimate writes -- confirmed by this test's own attempt, which
	// hit the exclusion constraint when seeded that way.
	atmSecond := insertATM("BATM-ZZZ-" + tag)
	atmFirst := insertATM("BATM-AAA-" + tag)
	// atmEnded: one assignment whose window already closed -- must be excluded.
	atmEnded := insertATM("BATM-ENDED-" + tag)
	// atmOther: assigned only to branch B's package -- must never appear under branchA.
	atmOther := insertATM("BATM-OTHER-" + tag)

	insertAssignment := func(atmID, packageID int64, start string, end *string) {
		if _, err := tx.Exec(ctx, `
			INSERT INTO atm_vendor_packages (atm_id, vendor_package_id, effective_start_date, effective_end_date, is_active)
			VALUES ($1, $2, $3::date, $4::date, true)
		`, atmID, packageID, start, end); err != nil {
			t.Fatalf("seed assignment atm=%d package=%d: %v", atmID, packageID, err)
		}
	}
	insertAssignment(atmFirst, pkg1, today, nil)
	insertAssignment(atmSecond, pkg2, today, nil)
	insertAssignment(atmEnded, pkg1, longAgoStart, &longAgoEnd)
	insertAssignment(atmOther, pkgB, today, nil)

	repo := NewBranchATMRepository(tx)

	t.Run("BranchVendorID resolves the owning vendor, nil for a non-existent branch", func(t *testing.T) {
		got, err := repo.BranchVendorID(ctx, branchA)
		if err != nil {
			t.Fatalf("BranchVendorID: %v", err)
		}
		if got == nil || *got != vendorA {
			t.Fatalf("BranchVendorID(branchA) = %v, want %d", got, vendorA)
		}
		got, err = repo.BranchVendorID(ctx, -1)
		if err != nil {
			t.Fatalf("BranchVendorID(-1): %v", err)
		}
		if got != nil {
			t.Errorf("BranchVendorID(-1) = %v, want nil", got)
		}
	})

	t.Run("List returns terminal_id-ordered rows, excludes ended assignments and other branches", func(t *testing.T) {
		got, err := repo.List(ctx, db.ListBranchATMsParams{VendorBranchID: &branchA, PageLimit: 100, PageOffset: 0})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("len(List(branchA)) = %d, want 2 (got %+v)", len(got), got)
		}
		// atmFirst = "BATM-AAA-*", atmSecond = "BATM-ZZZ-*" -- ASC order
		// proves the outer ORDER BY terminal_id, not insertion order
		// (atmSecond/pkg2 was seeded and assigned before atmFirst/pkg1).
		if got[0].AtmID != atmFirst || got[1].AtmID != atmSecond {
			t.Fatalf("List(branchA) order = [%d, %d], want [%d, %d] (terminal_id ASC)",
				got[0].AtmID, got[1].AtmID, atmFirst, atmSecond)
		}
		if got[0].PackageCode != "PKG1_"+tag || got[1].PackageCode != "PKG2_"+tag {
			t.Errorf("List(branchA) package codes = [%q, %q], want [%q, %q]",
				got[0].PackageCode, got[1].PackageCode, "PKG1_"+tag, "PKG2_"+tag)
		}
		for _, r := range got {
			if r.AtmID == atmEnded {
				t.Errorf("List(branchA) unexpectedly included atmEnded (ended assignment)")
			}
			if r.AtmID == atmOther {
				t.Errorf("List(branchA) unexpectedly included atmOther (belongs to branch B)")
			}
		}
	})

	t.Run("Count matches the List length", func(t *testing.T) {
		count, err := repo.Count(ctx, branchA)
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if count != 2 {
			t.Errorf("Count(branchA) = %d, want 2", count)
		}
	})
}
