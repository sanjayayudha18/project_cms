//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// Migration 046 (.kiro/specs/permission-catalog-coverage/, task 1.4): Property 2 (grant matrix D9),
// Property 3 (re-run is idempotent + additive), and the catalog-keys fixture for the frontend coverage test.
// Real dev DB (046 applied); every write is inside integrationTx and rolled back.

const migration046Path = "../../migrations/046_permission_catalog_coverage.sql"

// newKeys046 = the 21 keys 046 adds (2 menus + 19 features).
var newKeys046 = []string{
	"approvals", "exports",
	"replenish.forecast-browser", "replenish.vendor-requests", "replenish.schedule",
	"monitoring.atm-portal", "monitoring.dsr-late-report", "monitoring.eod-monitoring", "monitoring.audit-logs",
	"forecasting.dmaa-forecast", "forecasting.holidays",
	"settings.users", "settings.vendors", "settings.atms", "settings.regions", "settings.pks-types",
	"settings.master-data-io", "settings.rbac-users", "settings.rbac-policies", "settings.rbac-delegations",
	"settings.rbac-leaves",
}

// matrixD9 = design "Matriks grant awal (D9)": role -> screen keys it must hold. Written independently of the SQL.
var matrixD9 = map[string][]string{
	"ADMIN": {"cash-flow", "exports",
		"replenish.forecast-browser", "replenish.vendor-requests", "replenish.schedule",
		"monitoring.atm-portal", "monitoring.dsr-late-report", "monitoring.eod-monitoring", "monitoring.audit-logs",
		"forecasting.dmaa-forecast", "forecasting.holidays",
		"settings.vendors", "settings.atms", "settings.regions", "settings.pks-types", "settings.master-data-io",
		"settings.rbac-users", "settings.rbac-policies", "settings.rbac-delegations", "settings.rbac-leaves"},
	"ADMIN_PARAM": {"cash-flow", "monitoring.atm-portal", "monitoring.audit-logs", "forecasting.dmaa-forecast",
		"settings.vendors", "settings.atms", "settings.regions", "settings.pks-types", "settings.master-data-io",
		"settings.rbac-users", "settings.rbac-policies", "settings.rbac-delegations", "settings.rbac-leaves"},
	"APPACCESS": {"monitoring.eod-monitoring",
		"settings.rbac-users", "settings.rbac-policies", "settings.rbac-delegations", "settings.rbac-leaves"},
	"ATM-USER": {"cash-flow", "exports",
		"replenish.forecast-browser", "replenish.vendor-requests", "replenish.schedule",
		"monitoring.atm-portal", "monitoring.dsr-late-report", "forecasting.dmaa-forecast"},
	"ATM-SPV": {"cash-flow", "exports",
		"replenish.forecast-browser", "replenish.vendor-requests", "replenish.schedule",
		"monitoring.atm-portal", "monitoring.dsr-late-report", "forecasting.dmaa-forecast", "settings.vendors"},
	"BRANCH-ATM-USER": {"exports",
		"replenish.forecast-browser", "replenish.vendor-requests", "replenish.schedule", "forecasting.dmaa-forecast"},
	"BRANCH-ATM-SPV": {"exports",
		"replenish.forecast-browser", "replenish.vendor-requests", "replenish.schedule",
		"monitoring.dsr-late-report", "forecasting.dmaa-forecast"},
}

func TestPermissionCatalogMigration_GrantMatrix(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()

	rows, err := tx.Query(ctx, `SELECT r.role, mf.key FROM role_permissions rp
		JOIN roles r ON r.id = rp.role_id JOIN menu_features mf ON mf.id = rp.menu_feature_id`)
	if err != nil {
		t.Fatalf("query grants: %v", err)
	}
	have := map[string]map[string]bool{}
	for rows.Next() {
		var role, key string
		if err := rows.Scan(&role, &key); err != nil {
			t.Fatal(err)
		}
		if have[role] == nil {
			have[role] = map[string]bool{}
		}
		have[role][key] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	rrows, err := tx.Query(ctx, `SELECT role FROM roles`)
	if err != nil {
		t.Fatal(err)
	}
	allRoles, err := pgx.CollectRows(rrows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}

	parents := map[string]string{}
	prow, err := tx.Query(ctx, `SELECT c.key, p.key FROM menu_features c JOIN menu_features p ON p.id = c.parent_id`)
	if err != nil {
		t.Fatal(err)
	}
	for prow.Next() {
		var c, p string
		if err := prow.Scan(&c, &p); err != nil {
			t.Fatal(err)
		}
		parents[c] = p
	}
	prow.Close()

	for _, role := range allRoles {
		want := map[string]bool{}
		for _, k := range matrixD9[role] {
			want[k] = true
			if p, ok := parents[k]; ok && !have[role][p] {
				t.Errorf("%s: parent %s of %s not granted", role, p, k)
			}
		}
		// iff on the keys 046 introduced: grant exists <=> matrix cell is a tick.
		for _, k := range newKeys046 {
			if have[role][k] != want[k] {
				t.Errorf("%s|%s: granted=%v, matrix=%v", role, k, have[role][k], want[k])
			}
		}
		// existing key in the matrix (cash-flow): must be held, 046 does not remove it.
		if want["cash-flow"] && !have[role]["cash-flow"] {
			t.Errorf("%s|cash-flow missing", role)
		}
	}
}

// 046 body without its own BEGIN/COMMIT; ADD COLUMN made IF NOT EXISTS (cmd/migrate runs the file once,
// this test re-runs it on an already migrated DB).
func migration046Body(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(migration046Path)
	if err != nil {
		t.Fatalf("read 046: %v", err)
	}
	s := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if strings.Count(s, "\nBEGIN;\n") != 1 || strings.Count(s, "\nCOMMIT;") != 1 {
		t.Fatal("046: expected exactly one BEGIN; and one COMMIT;")
	}
	s = strings.Replace(s, "\nBEGIN;\n", "\n", 1)
	s = strings.Replace(s, "\nCOMMIT;", "\n", 1)
	const add = "ADD COLUMN assignable_roles text[]"
	if !strings.Contains(s, add) {
		t.Fatal("046: ADD COLUMN statement not found")
	}
	return strings.Replace(s, add, "ADD COLUMN IF NOT EXISTS assignable_roles text[]", 1)
}

// Property 3: re-running 046 changes no row and adds none (rolled back, dev untouched).
func TestPermissionCatalogMigration_RerunIdempotent(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()

	snap := func() (n [2]int, h [2]string) {
		for i, tbl := range []string{"menu_features", "role_permissions"} {
			if err := tx.QueryRow(ctx, `SELECT count(*), coalesce(md5(string_agg(x::text, '|' ORDER BY x::text)), '') FROM public.`+tbl+` x`).
				Scan(&n[i], &h[i]); err != nil {
				t.Fatalf("snapshot %s: %v", tbl, err)
			}
		}
		return
	}
	n0, h0 := snap()

	if _, err := tx.Exec(ctx, migration046Body(t)); err != nil {
		t.Fatalf("re-run 046: %v", err)
	}

	n1, h1 := snap()
	if n0 != n1 {
		t.Errorf("row counts changed: %v -> %v", n0, n1)
	}
	if h0 != h1 {
		t.Errorf("row content changed: %v -> %v", h0, h1)
	}
}

// Writes the catalog key list consumed by the CompanyPortal coverage test (Property 1) when UPDATE_CATALOG_FIXTURE=1.
func TestPermissionCatalogMigration_WriteFixture(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()

	rows, err := tx.Query(ctx, `SELECT key FROM menu_features ORDER BY key`)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, k := range keys {
		have[k] = true
	}
	for _, k := range newKeys046 {
		if !have[k] {
			t.Errorf("catalog missing 046 key %s", k)
		}
	}

	if os.Getenv("UPDATE_CATALOG_FIXTURE") != "1" {
		return
	}
	out, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.FromSlash("../../../frontend/CompanyPortal-Vite/src/lib/auth/__fixtures__/catalog-keys.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
