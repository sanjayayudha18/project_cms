//go:build integration

package rolemgmt

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Task 2.4, spec role-management-revamp. Properties 1-3 of migration 047.
// Every statement runs in one tx that is rolled back, so dev data is untouched.

// Mojibake patterns written as escapes so the source encoding cannot corrupt them.
const (
	mojibakeEmDash = "â€”"      // 'â€”' (em dash, encoded once)
	mojibakeEnDash = "Ã¢â‚¬â€œ" // 'Ã¢â‚¬â€œ' (en dash, encoded twice)
)

func portalTx(t *testing.T) (context.Context, pgx.Tx) {
	t.Helper()
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
	return ctx, tx
}

// Property 1: the text-repair step is idempotent and leaves no broken text on
// the rows it targets. The 13 other dev-only broken locations are out of scope.
func TestProperty1_TextRepairIdempotent(t *testing.T) {
	ctx, tx := portalTx(t)

	// Re-run the migration's UPDATEs: no row matches any more -> 0 changes.
	tag, err := tx.Exec(ctx, `UPDATE public.roles SET description = replace(description, $1, $2)
		WHERE description LIKE '%'||$1||'%'`, mojibakeEmDash, "—")
	if err != nil {
		t.Fatalf("re-run roles repair: %v", err)
	}
	if tag.RowsAffected() != 0 {
		t.Errorf("roles repair re-run changed %d rows, want 0", tag.RowsAffected())
	}
	tag, err = tx.Exec(ctx, `UPDATE public.locations
		SET name = replace(name, $1, $2), address_line1 = replace(address_line1, $1, $2)
		WHERE name LIKE '%'||$1||'%' OR address_line1 LIKE '%'||$1||'%'`, mojibakeEnDash, "–")
	if err != nil {
		t.Fatalf("re-run locations repair: %v", err)
	}
	if tag.RowsAffected() != 0 {
		t.Errorf("locations repair re-run changed %d rows, want 0", tag.RowsAffected())
	}

	// No leftover pattern in roles.description, and the 9 baseline roles now hold the real em dash.
	var left, fixed int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.roles WHERE description LIKE '%'||$1||'%'`, mojibakeEmDash).Scan(&left); err != nil {
		t.Fatalf("count leftover roles: %v", err)
	}
	if left != 0 {
		t.Errorf("%d roles.description still contain %q", left, mojibakeEmDash)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.roles
		WHERE role IN ('ADMIN','ADMIN_PARAM','ATM-USER','ATM-SPV','BRANCH-USER','BRANCH-SPV','BRANCH-ATM-USER','BRANCH-ATM-SPV','VENDOR-USER')
		AND description LIKE '%'||$1||'%'`, "—").Scan(&fixed); err != nil {
		t.Fatalf("count fixed roles: %v", err)
	}
	if fixed != 9 {
		t.Errorf("%d of 9 baseline roles hold the em dash, want 9", fixed)
	}

	// Seed rows (002) locations 911, 1450, 1692: no pattern left, real en dash present.
	rows, err := tx.Query(ctx, `SELECT id, name, address_line1 FROM public.locations WHERE id IN (911, 1450, 1692) ORDER BY id`)
	if err != nil {
		t.Fatalf("load locations: %v", err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var id int64
		var name, addr string
		if err := rows.Scan(&id, &name, &addr); err != nil {
			t.Fatalf("scan location: %v", err)
		}
		n++
		for col, v := range map[string]string{"name": name, "address_line1": addr} {
			if strings.Contains(v, mojibakeEnDash) {
				t.Errorf("location %d %s still holds the broken pattern", id, col)
			}
		}
		if !strings.Contains(name, "–") {
			t.Errorf("location %d name %q has no en dash", id, name)
		}
	}
	if n != 3 {
		t.Errorf("found %d of 3 target locations", n)
	}
}

// Property 2: vendor-* keys per role = old NAV_ITEMS (dsr, orders, realizations)
// minus realizations for VENDOR-VAULT-USER.
func TestProperty2_VendorGrantsEqualOldNav(t *testing.T) {
	ctx, tx := portalTx(t)
	all := []string{"vendor-dsr", "vendor-orders", "vendor-realizations"}
	want := map[string][]string{
		"VENDOR-USER":       all,
		"VENDOR-ATM-USER":   all,
		"VENDOR-VAULT-USER": {"vendor-dsr", "vendor-orders"},
	}
	for role, keys := range want {
		rows, err := tx.Query(ctx, `SELECT mf.key FROM public.role_permissions rp
			JOIN public.roles r ON r.id = rp.role_id
			JOIN public.menu_features mf ON mf.id = rp.menu_feature_id
			WHERE r.role = $1 AND mf.key LIKE 'vendor-%' ORDER BY mf.key`, role)
		if err != nil {
			t.Fatalf("%s: query: %v", role, err)
		}
		var got []string
		for rows.Next() {
			var k string
			if err := rows.Scan(&k); err != nil {
				t.Fatalf("%s: scan: %v", role, err)
			}
			got = append(got, k)
		}
		rows.Close()
		if !slices.Equal(got, keys) {
			t.Errorf("%s: vendor keys = %v, want %v", role, got, keys)
		}
	}
	var wrongPortal int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.menu_features WHERE key = ANY($1) AND portal <> 'vendor'`, all).Scan(&wrongPortal); err != nil {
		t.Fatalf("portal check: %v", err)
	}
	if wrongPortal != 0 {
		t.Errorf("%d vendor-* catalog keys not in portal vendor", wrongPortal)
	}
}

// Property 3: inserts that break the portal/prefix/scope rules are rejected by CHECK.
func TestProperty3_ChecksRejectInvalidRoles(t *testing.T) {
	ctx, tx := portalTx(t)
	cases := []struct {
		name, role, portal string
		scope              []string
		constraint         string
	}{
		{"VENDOR- prefix, portal main", "VENDOR-TPORTAL1", "main", nil, "roles_portal_prefix_chk"},
		{"portal vendor without VENDOR- prefix", "TPORTAL-B1", "vendor", []string{"vault"}, "roles_portal_prefix_chk"},
		{"unknown portal", "TPORTAL-C1", "bogus", nil, "roles_portal_chk"},
		{"vendor without scope", "VENDOR-TPORTAL2", "vendor", nil, "roles_party_scope_portal_chk"},
		{"vendor with empty scope", "VENDOR-TPORTAL3", "vendor", []string{}, "roles_party_scope_portal_chk"},
		{"main with scope", "TPORTAL-D1", "main", []string{"vault"}, "roles_party_scope_portal_chk"},
		{"scope value outside set", "VENDOR-TPORTAL4", "vendor", []string{"cash"}, "roles_party_scope_values_chk"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sp, err := tx.Begin(ctx) // savepoint: a failed insert must not poison the outer tx
			if err != nil {
				t.Fatalf("savepoint: %v", err)
			}
			defer func() { _ = sp.Rollback(ctx) }()
			_, err = sp.Exec(ctx, `INSERT INTO public.roles (role, portal, party_scope) VALUES ($1, $2, $3)`, c.role, c.portal, c.scope)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
				t.Fatalf("insert error = %v, want check_violation 23514", err)
			}
			if pgErr.ConstraintName != c.constraint {
				t.Errorf("violated %q, want %q", pgErr.ConstraintName, c.constraint)
			}
		})
	}

	t.Run("valid vendor role is accepted", func(t *testing.T) {
		sp, err := tx.Begin(ctx)
		if err != nil {
			t.Fatalf("savepoint: %v", err)
		}
		defer func() { _ = sp.Rollback(ctx) }()
		if _, err := sp.Exec(ctx, `INSERT INTO public.roles (role, portal, party_scope) VALUES ('VENDOR-TPORTAL9', 'vendor', ARRAY['replenish'])`); err != nil {
			t.Fatalf("valid insert rejected: %v", err)
		}
	})
}
