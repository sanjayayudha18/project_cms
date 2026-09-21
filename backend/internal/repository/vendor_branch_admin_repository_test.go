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

// TestVendorBranchAdminRepository exercises List/Count/GetByID/FindByCode
// against a real DB transaction rolled back on cleanup -- same harness
// convention as vendor_admin_repository_test.go. Create/Update/Disable/
// Enable are exercised through VendorBranchApplier's own integration test
// (internal/service), since this repository's mutation queries are only
// ever invoked there, tx-scoped, inside the apply-on-approve hook -- not
// directly by VendorBranchAdminService.
func TestVendorBranchAdminRepository(t *testing.T) {
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

	var vendorID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM vendors LIMIT 1`).Scan(&vendorID); err != nil {
		t.Fatalf("loading a seeded vendor: %v", err)
	}

	tag := time.Now().Format("150405.000000000")
	codeActive := "TVB_ACT_" + tag
	codeDisabled := "TVB_DIS_" + tag

	var activeID, disabledID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name) VALUES ($1, $2, $3) RETURNING id`,
		vendorID, codeActive, "Active Branch "+tag,
	).Scan(&activeID); err != nil {
		t.Fatalf("seed active branch: %v", err)
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, is_active, deleted_at) VALUES ($1, $2, $3, false, now()) RETURNING id`,
		vendorID, codeDisabled, "Disabled Branch "+tag,
	).Scan(&disabledID); err != nil {
		t.Fatalf("seed disabled branch: %v", err)
	}

	repo := NewVendorBranchAdminRepository(tx)

	t.Run("List scoped to vendor_id, default active status excludes disabled", func(t *testing.T) {
		got, err := repo.List(ctx, db.ListVendorBranchesAdminParams{
			VendorID: vendorID, Status: "active", PageLimit: 100, PageOffset: 0,
		})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		found := false
		for _, b := range got {
			if b.ID == disabledID {
				t.Fatalf("List(active) unexpectedly included the disabled branch")
			}
			if b.ID == activeID {
				found = true
			}
		}
		if !found {
			t.Fatalf("List(active) = %+v, want to include the active seed branch", got)
		}
	})

	t.Run("List status=all returns both, Count matches", func(t *testing.T) {
		q := tag
		got, err := repo.List(ctx, db.ListVendorBranchesAdminParams{
			VendorID: vendorID, Q: &q, Status: "all", PageLimit: 100, PageOffset: 0,
		})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("len(List(all)) = %d, want 2", len(got))
		}
		count, err := repo.Count(ctx, db.CountVendorBranchesAdminParams{VendorID: vendorID, Q: &q, Status: "all"})
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if count != 2 {
			t.Errorf("Count(all) = %d, want 2", count)
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

	t.Run("FindByCode roundtrip and nil for unknown code", func(t *testing.T) {
		foundID, err := repo.FindByCode(ctx, codeActive)
		if err != nil {
			t.Fatalf("FindByCode: %v", err)
		}
		if foundID == nil || *foundID != activeID {
			t.Fatalf("FindByCode(%s) = %v, want %d", codeActive, foundID, activeID)
		}
		foundID, err = repo.FindByCode(ctx, "no-such-code-"+tag)
		if err != nil {
			t.Fatalf("FindByCode(unknown): %v", err)
		}
		if foundID != nil {
			t.Errorf("FindByCode(unknown) = %v, want nil", foundID)
		}
	})
}
