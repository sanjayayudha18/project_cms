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

// TestUserAdminRepository exercises List/Count/GetByID/Create/Update plus
// the uniqueness/reference-check helpers against a real DB transaction
// rolled back on cleanup — same harness convention as
// audit_log_repository_test.go / vendor_admin_repository_test.go.
func TestUserAdminRepository(t *testing.T) {
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

	roleRows, err := tx.Query(ctx, "SELECT id, role FROM roles ORDER BY id LIMIT 2")
	if err != nil {
		t.Fatalf("fetch seed roles: %v", err)
	}
	var roleIDs []int64
	var roleNames []string
	for roleRows.Next() {
		var id int64
		var role string
		if err := roleRows.Scan(&id, &role); err != nil {
			roleRows.Close()
			t.Fatalf("scan seed role: %v", err)
		}
		roleIDs = append(roleIDs, id)
		roleNames = append(roleNames, role)
	}
	roleRows.Close()
	if len(roleIDs) < 2 {
		t.Skip("fewer than 2 seeded roles available — skipping role-filter assertions")
	}
	roleAID, roleBID := roleIDs[0], roleIDs[1]
	roleAName := roleNames[0]

	var vendorID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO vendors (code, name, is_active) VALUES ($1, $2, true) RETURNING id`,
		"TUAT_"+tag, "Test Vendor "+tag,
	).Scan(&vendorID); err != nil {
		t.Fatalf("seed vendor: %v", err)
	}

	insertUser := func(username string, roleID int64, vendorIDArg *int64, disabled bool) int64 {
		var deletedAtArg any
		if disabled {
			deletedAtArg = time.Now()
		}
		var id int64
		if err := tx.QueryRow(ctx,
			`INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source, vendor_id, is_active, deleted_at)
			 VALUES ($1, $2, $2, $3, false, 'ldap', $4, $5, $6) RETURNING id`,
			roleID, username, username+"@example.com", vendorIDArg, !disabled, deletedAtArg,
		).Scan(&id); err != nil {
			t.Fatalf("seed user %s: %v", username, err)
		}
		return id
	}

	userA := insertUser("tuat_a_"+tag, roleAID, nil, false)
	userB := insertUser("tuat_b_"+tag, roleBID, &vendorID, false)
	userC := insertUser("tuat_c_"+tag, roleAID, nil, true)

	repo := NewUserAdminRepository(tx)

	t.Run("List q filter, default active status excludes soft-disabled", func(t *testing.T) {
		q := tag
		got, err := repo.List(ctx, db.ListUsersAdminParams{Q: &q, Status: "active", PageLimit: 100, PageOffset: 0})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("len(List(active)) = %d, want 2 (userA, userB)", len(got))
		}
		for _, r := range got {
			if r.ID == userC {
				t.Errorf("List(active) included soft-disabled userC")
			}
		}
	})

	t.Run("List role filter", func(t *testing.T) {
		q := tag
		got, err := repo.List(ctx, db.ListUsersAdminParams{Q: &q, Role: &roleAName, Status: "all", PageLimit: 100, PageOffset: 0})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 2 { // userA (active) + userC (disabled), both roleA
			t.Fatalf("len(List(role=%s)) = %d, want 2", roleAName, len(got))
		}
		for _, r := range got {
			if r.Role != roleAName {
				t.Errorf("List(role filter) returned role %q, want %q", r.Role, roleAName)
			}
		}
	})

	t.Run("List vendor_id filter", func(t *testing.T) {
		q := tag
		got, err := repo.List(ctx, db.ListUsersAdminParams{Q: &q, VendorID: &vendorID, Status: "all", PageLimit: 100, PageOffset: 0})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 1 || got[0].ID != userB {
			t.Fatalf("List(vendor_id) = %+v, want exactly userB", got)
		}
	})

	t.Run("List status=all returns all three, Count matches", func(t *testing.T) {
		q := tag
		got, err := repo.List(ctx, db.ListUsersAdminParams{Q: &q, Status: "all", PageLimit: 100, PageOffset: 0})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("len(List(all)) = %d, want 3", len(got))
		}
		count, err := repo.Count(ctx, db.CountUsersAdminParams{Q: &q, Status: "all"})
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if count != 3 {
			t.Errorf("Count(all) = %d, want 3", count)
		}
	})

	t.Run("Count vs summed pages", func(t *testing.T) {
		q := tag
		f := db.CountUsersAdminParams{Q: &q, Status: "all"}
		count, err := repo.Count(ctx, f)
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		page1, err := repo.List(ctx, db.ListUsersAdminParams{Q: &q, Status: "all", PageLimit: 2, PageOffset: 0})
		if err != nil {
			t.Fatalf("List page1: %v", err)
		}
		page2, err := repo.List(ctx, db.ListUsersAdminParams{Q: &q, Status: "all", PageLimit: 2, PageOffset: 2})
		if err != nil {
			t.Fatalf("List page2: %v", err)
		}
		if int64(len(page1)+len(page2)) != count {
			t.Errorf("pages summed to %d rows, Count = %d, want equal", len(page1)+len(page2), count)
		}
	})

	t.Run("GetByID includes soft-deleted rows, nil for non-existent id", func(t *testing.T) {
		got, err := repo.GetByID(ctx, userC)
		if err != nil {
			t.Fatalf("GetByID(disabled): %v", err)
		}
		if got == nil || got.IsActive {
			t.Fatalf("GetByID(userC) = %+v, want a row with IsActive=false", got)
		}
		got, err = repo.GetByID(ctx, -1)
		if err != nil {
			t.Fatalf("GetByID(-1): %v", err)
		}
		if got != nil {
			t.Errorf("GetByID(-1) = %+v, want nil", got)
		}
	})

	t.Run("Create -> FindByUsername/FindByEmail -> GetByID roundtrip", func(t *testing.T) {
		username := "tuat_new_" + tag
		email := username + "@example.com"
		created, err := repo.Create(ctx, db.CreateUserAdminParams{
			RoleID: roleAID, Username: username, FullName: "New User " + tag, Email: email,
			IsKaryawan: false, AuthSource: "ldap", MustChangePassword: false,
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		foundID, err := repo.FindByUsername(ctx, username)
		if err != nil {
			t.Fatalf("FindByUsername: %v", err)
		}
		if foundID == nil || *foundID != created.ID {
			t.Fatalf("FindByUsername = %v, want %d", foundID, created.ID)
		}
		foundID, err = repo.FindByEmail(ctx, email)
		if err != nil {
			t.Fatalf("FindByEmail: %v", err)
		}
		if foundID == nil || *foundID != created.ID {
			t.Fatalf("FindByEmail = %v, want %d", foundID, created.ID)
		}
		got, err := repo.GetByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got == nil || got.Username != username {
			t.Fatalf("GetByID(created) = %+v, want username %s", got, username)
		}
	})

	t.Run("Create with a duplicate username surfaces a unique violation", func(t *testing.T) {
		// Savepoint: the failed insert aborts its nested tx, not the shared tx.
		sp, err := tx.Begin(ctx)
		if err != nil {
			t.Fatalf("begin savepoint: %v", err)
		}
		defer sp.Rollback(ctx)
		_, err = NewUserAdminRepository(sp).Create(ctx, db.CreateUserAdminParams{
			RoleID: roleAID, Username: "tuat_a_" + tag, FullName: "Duplicate", Email: "dup." + tag + "@example.com",
			AuthSource: "ldap",
		})
		if err == nil {
			t.Fatal("Create with duplicate username: want a unique-violation error, got nil")
		}
	})

	t.Run("Update overwrites editable fields, not-found on soft-disabled target", func(t *testing.T) {
		updated, err := repo.Update(ctx, db.UpdateUserAdminParams{
			ID: userA, FullName: "Renamed A " + tag, Email: "renamed.a." + tag + "@example.com",
			RoleID: roleAID, IsKaryawan: true,
		})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if updated.FullName != "Renamed A "+tag {
			t.Errorf("Update FullName = %q, want %q", updated.FullName, "Renamed A "+tag)
		}

		_, err = repo.Update(ctx, db.UpdateUserAdminParams{
			ID: userC, FullName: "Should Not Apply", Email: "noop@example.com", RoleID: roleAID,
		})
		if err != pgx.ErrNoRows {
			t.Errorf("Update(disabled target) err = %v, want pgx.ErrNoRows", err)
		}
	})

	t.Run("FindByEmailExcludingID scopes out the target id", func(t *testing.T) {
		var userBEmail string
		if err := tx.QueryRow(ctx, "SELECT email FROM users WHERE id = $1", userB).Scan(&userBEmail); err != nil {
			t.Fatalf("fetch userB email: %v", err)
		}

		found, err := repo.FindByEmailExcludingID(ctx, userBEmail, userB)
		if err != nil {
			t.Fatalf("FindByEmailExcludingID(self excluded): %v", err)
		}
		if found != nil {
			t.Errorf("FindByEmailExcludingID(self excluded) = %v, want nil", found)
		}

		found, err = repo.FindByEmailExcludingID(ctx, userBEmail, userA)
		if err != nil {
			t.Fatalf("FindByEmailExcludingID(other id): %v", err)
		}
		if found == nil || *found != userB {
			t.Errorf("FindByEmailExcludingID(other id) = %v, want %d", found, userB)
		}
	})

	t.Run("GetRoleByName and ListRoles", func(t *testing.T) {
		role, err := repo.GetRoleByName(ctx, roleAName)
		if err != nil {
			t.Fatalf("GetRoleByName: %v", err)
		}
		if role == nil || role.ID != roleAID {
			t.Fatalf("GetRoleByName(%s) = %v, want id %d", roleAName, role, roleAID)
		}
		role, err = repo.GetRoleByName(ctx, "NOT_A_REAL_ROLE_"+tag)
		if err != nil {
			t.Fatalf("GetRoleByName(missing): %v", err)
		}
		if role != nil {
			t.Errorf("GetRoleByName(missing) = %v, want nil", role)
		}

		roles, err := repo.ListRoles(ctx)
		if err != nil {
			t.Fatalf("ListRoles: %v", err)
		}
		if len(roles) < 2 {
			t.Errorf("ListRoles returned %d roles, want at least 2", len(roles))
		}
	})

	t.Run("VendorExists and SupervisorExists reference checks", func(t *testing.T) {
		exists, err := repo.VendorExists(ctx, vendorID)
		if err != nil {
			t.Fatalf("VendorExists: %v", err)
		}
		if !exists {
			t.Errorf("VendorExists(%d) = false, want true", vendorID)
		}
		exists, err = repo.VendorExists(ctx, -1)
		if err != nil {
			t.Fatalf("VendorExists(-1): %v", err)
		}
		if exists {
			t.Errorf("VendorExists(-1) = true, want false")
		}

		exists, err = repo.SupervisorExists(ctx, userA)
		if err != nil {
			t.Fatalf("SupervisorExists(active): %v", err)
		}
		if !exists {
			t.Errorf("SupervisorExists(userA) = false, want true")
		}
		exists, err = repo.SupervisorExists(ctx, userC)
		if err != nil {
			t.Fatalf("SupervisorExists(disabled): %v", err)
		}
		if exists {
			t.Errorf("SupervisorExists(userC, soft-disabled) = true, want false")
		}
	})
}
