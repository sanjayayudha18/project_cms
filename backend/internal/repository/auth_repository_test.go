//go:build integration

package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestAuthRepository walks one local user through the login-security
// lifecycle (lockout counters, initial vs self-service password, soft
// delete closing login) on a real DB transaction rolled back on cleanup.
func TestAuthRepository(t *testing.T) {
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
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	tag := time.Now().Format("150405.000000000")
	username, email := "itauth-"+tag, "itauth-"+tag+"@test.local"
	var id int64
	if err := tx.QueryRow(ctx, `INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source, password_hash)
		SELECT id, $1, 'IT Auth', $2, false, 'local', 'old-hash' FROM roles ORDER BY id LIMIT 1 RETURNING id`, username, email).Scan(&id); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	repo := NewAuthRepository(tx)

	for name, find := range map[string]func() (any, error){
		"email":    func() (any, error) { return repo.FindByEmail(ctx, email) },
		"username": func() (any, error) { return repo.FindByUsername(ctx, username) },
		"id":       func() (any, error) { return repo.FindByID(ctx, id) },
		"profile":  func() (any, error) { return repo.GetUserProfile(ctx, id) },
	} {
		if u, err := find(); err != nil || u == nil {
			t.Fatalf("find by %s = %v, %v", name, u, err)
		}
	}
	u, _ := repo.FindByEmail(ctx, email)
	if u.ID != id || u.AuthSource != "local" || u.PasswordHash == nil || *u.PasswordHash != "old-hash" || u.DeletedAt != nil || u.LockedUntil != nil {
		t.Fatalf("FindByEmail record = %+v", u)
	}

	// Missing user is (nil, nil), never an error (login must not leak which part was wrong).
	if u, err := repo.FindByEmail(ctx, "nobody-"+email); u != nil || err != nil {
		t.Fatalf("missing email = %v, %v", u, err)
	}
	if u, err := repo.FindByUsername(ctx, "nobody-"+username); u != nil || err != nil {
		t.Fatalf("missing username = %v, %v", u, err)
	}
	if u, err := repo.FindByID(ctx, -1); u != nil || err != nil {
		t.Fatalf("missing id = %v, %v", u, err)
	}
	if u, err := repo.GetUserProfile(ctx, -1); u != nil || err != nil {
		t.Fatalf("missing profile = %v, %v", u, err)
	}

	// Lockout: counter increments, lock is stored, reset clears both.
	for want := int32(1); want <= 2; want++ {
		if n, err := repo.IncrementFailedLogin(ctx, id); err != nil || n != want {
			t.Fatalf("IncrementFailedLogin = %d, %v, want %d", n, err, want)
		}
	}
	until := time.Now().Add(15 * time.Minute).UTC().Truncate(time.Microsecond)
	if err := repo.LockAccount(ctx, id, until); err != nil {
		t.Fatal(err)
	}
	if u, _ := repo.FindByID(ctx, id); u.LockedUntil == nil || !u.LockedUntil.Equal(until) || u.FailedLoginAttempts != 2 {
		t.Fatalf("after lock = %+v", u)
	}
	if err := repo.ResetLockout(ctx, id); err != nil {
		t.Fatal(err)
	}
	if u, _ := repo.FindByID(ctx, id); u.LockedUntil != nil || u.FailedLoginAttempts != 0 {
		t.Fatalf("after reset = %+v", u)
	}

	// Admin-set password forces a change; self-service change clears it and the lockout.
	if err := repo.SetInitialPassword(ctx, id, "temp-hash"); err != nil {
		t.Fatal(err)
	}
	if u, _ := repo.FindByID(ctx, id); !u.MustChangePassword || *u.PasswordHash != "temp-hash" || u.PasswordChangedAt == nil {
		t.Fatalf("after SetInitialPassword = %+v", u)
	}
	if _, err := repo.IncrementFailedLogin(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetPassword(ctx, id, "new-hash"); err != nil {
		t.Fatal(err)
	}
	if u, _ := repo.FindByID(ctx, id); u.MustChangePassword || *u.PasswordHash != "new-hash" || u.FailedLoginAttempts != 0 {
		t.Fatalf("after SetPassword = %+v", u)
	}
	if err := repo.MarkPasswordExpired(ctx, id); err != nil {
		t.Fatal(err)
	}
	if u, _ := repo.FindByID(ctx, id); !u.MustChangePassword {
		t.Fatal("MarkPasswordExpired did not force a change")
	}
	if err := repo.UpdateLastLogin(ctx, id); err != nil {
		t.Fatal(err)
	}
	var lastLogin *time.Time
	if err := tx.QueryRow(ctx, `SELECT last_login_at FROM users WHERE id = $1`, id).Scan(&lastLogin); err != nil || lastLogin == nil {
		t.Fatalf("last_login_at = %v, %v", lastLogin, err)
	}

	// Soft delete closes login; reactivate opens it again.
	if err := repo.Deactivate(ctx, id); err != nil {
		t.Fatal(err)
	}
	if u, err := repo.FindByEmail(ctx, email); u != nil || err != nil {
		t.Fatalf("deactivated user still found: %v, %v", u, err)
	}
	if u, _ := repo.GetUserProfile(ctx, id); u != nil {
		t.Fatal("deactivated user still has a profile")
	}
	if err := repo.Reactivate(ctx, id); err != nil {
		t.Fatal(err)
	}
	if u, _ := repo.FindByEmail(ctx, email); u == nil || !u.IsActive {
		t.Fatalf("reactivated user = %+v", u)
	}
}
