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

// ext-user-api task 3.5 (Property 4, Req 9.5/9.6): service accounts
// (auth_source='service') never appear in the admin list/count, are not a
// valid supervisor, and the EXT-CLIENT role is not offered to the admin form.
// Everything runs in a tx rolled back on cleanup.
func TestUserAdminRepository_ServiceAccountHidden(t *testing.T) {
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

	// 039 may be absent from a DB built with 001+002 only: seed the role.
	if _, err := tx.Exec(ctx, `INSERT INTO roles (role) VALUES ('EXT-CLIENT') ON CONFLICT DO NOTHING`); err != nil {
		t.Fatalf("seed EXT-CLIENT role: %v", err)
	}
	var extRoleID, otherRoleID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM roles WHERE role = 'EXT-CLIENT'`).Scan(&extRoleID); err != nil {
		t.Fatalf("EXT-CLIENT role id: %v", err)
	}
	if err := tx.QueryRow(ctx, `SELECT id FROM roles WHERE role <> 'EXT-CLIENT' ORDER BY id LIMIT 1`).Scan(&otherRoleID); err != nil {
		t.Fatalf("other role id: %v", err)
	}

	insert := func(username string, roleID int64, source string) int64 {
		var id int64
		if err := tx.QueryRow(ctx,
			`INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source, is_active)
			 VALUES ($1, $2, $2, $3, false, $4, true) RETURNING id`,
			roleID, username, username+"@example.com", source,
		).Scan(&id); err != nil {
			t.Fatalf("seed user %s: %v", username, err)
		}
		return id
	}
	svcID := insert("tuat_svc_"+tag, extRoleID, "service")
	humanID := insert("tuat_human_"+tag, otherRoleID, "ldap")

	repo := NewUserAdminRepository(tx)
	needle := "tuat_svc_" + tag

	t.Run("List and Count exclude the service account", func(t *testing.T) {
		for _, status := range []string{"active", "all"} {
			rows, err := repo.List(ctx, db.ListUsersAdminParams{Q: &needle, Status: status, PageLimit: 100})
			if err != nil {
				t.Fatalf("List(%s): %v", status, err)
			}
			if len(rows) != 0 {
				t.Errorf("List(%s) returned %d rows for the service account, want 0", status, len(rows))
			}
			n, err := repo.Count(ctx, db.CountUsersAdminParams{Q: &needle, Status: status})
			if err != nil {
				t.Fatalf("Count(%s): %v", status, err)
			}
			if n != 0 {
				t.Errorf("Count(%s) = %d, want 0", status, n)
			}
		}
		human := "tuat_human_" + tag
		rows, err := repo.List(ctx, db.ListUsersAdminParams{Q: &human, Status: "all", PageLimit: 100})
		if err != nil || len(rows) != 1 || rows[0].ID != humanID {
			t.Fatalf("control: List(human) = %+v, err %v, want exactly the human user", rows, err)
		}
	})

	t.Run("SupervisorExists false for service account, true for human", func(t *testing.T) {
		ok, err := repo.SupervisorExists(ctx, svcID)
		if err != nil || ok {
			t.Errorf("SupervisorExists(service) = %v, %v, want false, nil", ok, err)
		}
		ok, err = repo.SupervisorExists(ctx, humanID)
		if err != nil || !ok {
			t.Errorf("SupervisorExists(human) = %v, %v, want true, nil", ok, err)
		}
	})

	t.Run("EXT-CLIENT absent from the role options", func(t *testing.T) {
		roles, err := db.New(tx).ListRoles(ctx)
		if err != nil {
			t.Fatalf("ListRoles: %v", err)
		}
		if len(roles) == 0 {
			t.Fatal("ListRoles returned no roles")
		}
		for _, r := range roles {
			if r.Role == "EXT-CLIENT" {
				t.Errorf("ListRoles offered EXT-CLIENT")
			}
		}
	})
}
