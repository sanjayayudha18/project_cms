//go:build integration

package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// TestVendorAdminRepository exercises List/Count/GetByID/Create/Update/
// Disable/Enable/CountActiveUsers against a real DB transaction rolled back
// on cleanup — same harness convention as audit_log_repository_test.go. Row
// values are uniquely tagged by run timestamp so this test never collides
// with existing data or a concurrent run.
func TestVendorAdminRepository(t *testing.T) {
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
	codeActive := "TVA_ACT_" + tag
	codeDisabled := "TVA_DIS_" + tag

	var activeID, disabledID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO vendors (code, name, contact_email, is_active) VALUES ($1, $2, $3, true) RETURNING id`,
		codeActive, "Active Vendor "+tag, "active."+tag+"@example.com",
	).Scan(&activeID); err != nil {
		t.Fatalf("seed active vendor: %v", err)
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO vendors (code, name, is_active, deleted_at) VALUES ($1, $2, false, now()) RETURNING id`,
		codeDisabled, "Disabled Vendor "+tag,
	).Scan(&disabledID); err != nil {
		t.Fatalf("seed disabled vendor: %v", err)
	}

	repo := NewVendorAdminRepository(tx)

	t.Run("List q filter matches code and name, default active status excludes disabled", func(t *testing.T) {
		q := tag
		got, err := repo.List(ctx, db.ListVendorsAdminParams{
			Q: &q, Status: "active", PageLimit: 100, PageOffset: 0,
		})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 1 || got[0].ID != activeID {
			t.Fatalf("List(active) = %+v, want exactly the active seed vendor", got)
		}
	})

	t.Run("List status=disabled returns only the disabled seed vendor", func(t *testing.T) {
		q := tag
		got, err := repo.List(ctx, db.ListVendorsAdminParams{
			Q: &q, Status: "disabled", PageLimit: 100, PageOffset: 0,
		})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 1 || got[0].ID != disabledID {
			t.Fatalf("List(disabled) = %+v, want exactly the disabled seed vendor", got)
		}
	})

	t.Run("List status=all returns both, Count matches", func(t *testing.T) {
		q := tag
		got, err := repo.List(ctx, db.ListVendorsAdminParams{
			Q: &q, Status: "all", PageLimit: 100, PageOffset: 0,
		})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("len(List(all)) = %d, want 2", len(got))
		}
		count, err := repo.Count(ctx, db.CountVendorsAdminParams{Q: &q, Status: "all"})
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if count != 2 {
			t.Errorf("Count(all) = %d, want 2", count)
		}
	})

	t.Run("Count vs summed pages", func(t *testing.T) {
		q := tag
		f := db.CountVendorsAdminParams{Q: &q, Status: "all"}
		count, err := repo.Count(ctx, f)
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		page1, err := repo.List(ctx, db.ListVendorsAdminParams{Q: &q, Status: "all", PageLimit: 1, PageOffset: 0})
		if err != nil {
			t.Fatalf("List page1: %v", err)
		}
		page2, err := repo.List(ctx, db.ListVendorsAdminParams{Q: &q, Status: "all", PageLimit: 1, PageOffset: 1})
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

	t.Run("Create -> FindByCode -> GetByID roundtrip", func(t *testing.T) {
		code := "TVA_NEW_" + tag
		created, err := repo.Create(ctx, db.CreateVendorAdminParams{
			Code: code, Name: "New Vendor " + tag,
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		foundID, err := repo.FindByCode(ctx, code)
		if err != nil {
			t.Fatalf("FindByCode: %v", err)
		}
		if foundID == nil || *foundID != created.ID {
			t.Fatalf("FindByCode = %v, want %d", foundID, created.ID)
		}
		got, err := repo.GetByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got == nil || got.Code != code || !got.IsActive {
			t.Fatalf("GetByID(created) = %+v, want active row with code %s", got, code)
		}
	})

	t.Run("Create with a duplicate code surfaces a unique violation", func(t *testing.T) {
		_, err := repo.Create(ctx, db.CreateVendorAdminParams{
			Code: codeActive, Name: "Duplicate Code Vendor",
		})
		if err == nil {
			t.Fatal("Create with duplicate code: want a unique-violation error, got nil")
		}
	})

	t.Run("Update overwrites editable fields, code untouched, not-found on soft-disabled target", func(t *testing.T) {
		newEmail := "updated." + tag + "@example.com"
		updated, err := repo.Update(ctx, db.UpdateVendorAdminParams{
			ID: activeID, Name: "Renamed Vendor " + tag, ContactEmail: &newEmail,
		})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if updated.Code != codeActive {
			t.Errorf("Update changed code to %q, want unchanged %q", updated.Code, codeActive)
		}
		if updated.ContactEmail == nil || *updated.ContactEmail != newEmail {
			t.Errorf("Update ContactEmail = %v, want %s", updated.ContactEmail, newEmail)
		}

		_, err = repo.Update(ctx, db.UpdateVendorAdminParams{ID: disabledID, Name: "Should Not Apply"})
		if err != pgx.ErrNoRows {
			t.Errorf("Update(disabled target) err = %v, want pgx.ErrNoRows", err)
		}
	})

	t.Run("Disable then Enable toggles is_active and deleted_at", func(t *testing.T) {
		if err := repo.Disable(ctx, activeID); err != nil {
			t.Fatalf("Disable: %v", err)
		}
		got, err := repo.GetByID(ctx, activeID)
		if err != nil {
			t.Fatalf("GetByID after Disable: %v", err)
		}
		if got.IsActive || !got.DeletedAt.Valid {
			t.Fatalf("after Disable, got %+v, want IsActive=false and DeletedAt set", got)
		}

		if err := repo.Enable(ctx, activeID); err != nil {
			t.Fatalf("Enable: %v", err)
		}
		got, err = repo.GetByID(ctx, activeID)
		if err != nil {
			t.Fatalf("GetByID after Enable: %v", err)
		}
		if !got.IsActive || got.DeletedAt.Valid {
			t.Fatalf("after Enable, got %+v, want IsActive=true and DeletedAt NULL", got)
		}
	})

	t.Run("CountActiveUsers counts only non-disabled users linked to the vendor", func(t *testing.T) {
		var roleID int64
		if err := tx.QueryRow(ctx, "SELECT id FROM roles ORDER BY id LIMIT 1").Scan(&roleID); err != nil {
			t.Fatalf("fetch seed role id: %v", err)
		}

		insertUser := func(username string, disabled bool) {
			var deletedAtArg any
			if disabled {
				deletedAtArg = time.Now()
			}
			_, err := tx.Exec(ctx,
				`INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source, vendor_id, is_active, deleted_at)
				 VALUES ($1, $2, $2, $3, false, 'ldap', $4, $5, $6)`,
				roleID, username, username+"@example.com", activeID, !disabled, deletedAtArg,
			)
			if err != nil {
				t.Fatalf("seed user %s: %v", username, err)
			}
		}
		insertUser("tvat_active1_"+tag, false)
		insertUser("tvat_active2_"+tag, false)
		insertUser("tvat_disabled1_"+tag, true)

		count, err := repo.CountActiveUsers(ctx, activeID)
		if err != nil {
			t.Fatalf("CountActiveUsers: %v", err)
		}
		if count != 2 {
			t.Errorf("CountActiveUsers = %d, want 2", count)
		}
	})
}
