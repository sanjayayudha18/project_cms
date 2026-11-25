//go:build integration

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/cimb-niaga/cms/backend/internal/rolemgmt"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	"github.com/cimb-niaga/cms/pkg/config"
)

// rbacRow is one probe of the design mount table (spec
// permission-catalog-coverage task 4.5, Property 4 + 7). keys come from the 4.4
// table (routePolicies) unless set here; roles = the fixed RequireRoles guard
// that stays behind the screen key (nil = no fixed guard).
type rbacRow struct {
	path  string
	keys  string // override; "" = look up in routePolicies
	roles []string
}

var (
	rolesMD   = []string{"ADMIN", "ADMIN_PARAM"}
	rolesRBAC = []string{"ADMIN", "ADMIN_PARAM", "APPACCESS"}
	rolesCWM  = []string{"CC-CWM", "ADMIN"}
	rolesCCR  = []string{"CC-CCR", "ADMIN"}
)

// GET only: no probe changes data. Mounts get a non-existent child so the
// request stops at 404 behind the guards; per-route guards use a real route.
var rbacRows = []rbacRow{
	{path: "/api/v1/dmaa-forecast/__probe"},
	{path: "/api/v1/atm-portal/atms"},
	{path: "/api/v1/audit-logs/__probe"},
	{path: "/api/v1/vault-plans/__probe"},
	{path: "/api/v1/admin/pks/__probe"},
	{path: "/api/v1/admin/vendors/__probe"},
	{path: "/api/v1/admin/vendors/1/vaults/__probe", roles: rolesMD},
	{path: "/api/v1/admin/vendors/1/dsr-location-maps/__probe", roles: rolesMD},
	{path: "/api/v1/admin/vendors/1/pics/__probe", roles: rolesMD},
	{path: "/api/v1/admin/vendors/1/pks/__probe", roles: rolesMD},
	{path: "/api/v1/admin/vendors/1/packages/__probe", roles: rolesMD},
	{path: "/api/v1/admin/vendors/1/package-prices/__probe", roles: rolesMD},
	{path: "/api/v1/admin/vendors/1/branches/__probe", roles: rolesMD},
	{path: "/api/v1/admin/atms/__probe", roles: rolesMD},
	{path: "/api/v1/admin/regions/__probe", roles: rolesMD},
	{path: "/api/v1/admin/pks-types/__probe", roles: rolesMD},
	{path: "/api/v1/admin/master-data/changes/__probe", roles: rolesMD},
	{path: "/api/v1/admin/master-data/import/__probe", roles: rolesMD},
	{path: "/api/v1/admin/acm-areas/__probe", roles: []string{"ADMIN"}},
	{path: "/api/v1/cash-count/category-params/__probe", roles: rolesCWM},
	{path: "/api/v1/cash-count/anomaly-categories/__probe", roles: rolesCCR},
	{path: "/api/v1/cash-count/checklist-items/__probe", roles: rolesCCR},
	{path: "/api/v1/cash-count/areas/__probe", roles: rolesCWM},
	{path: "/api/v1/cash-count/pics/__probe", roles: rolesCWM},
	{path: "/api/v1/cash-count/import/__probe", roles: []string{"CC-CWM"}},
	{path: "/api/v1/admin/approval/users/hierarchy", keys: "settings.rbac-users", roles: rolesRBAC},
	{path: "/api/v1/admin/approval/delegations", keys: "settings.rbac-delegations", roles: rolesRBAC},
	{path: "/api/v1/admin/approval/leaves", keys: "settings.rbac-leaves", roles: rolesRBAC},
	{path: "/api/v1/admin/approval/policies", keys: "settings.rbac-policies", roles: rolesRBAC},
	// Handler-level guards (viewers: key only; holidays: key + ADMIN).
	{path: "/api/v1/vendor-requests/forecast"},
	{path: "/api/v1/atm-visit-quotas/atms/__probe", keys: "replenish.vendor-requests"},
	{path: "/api/v1/dsr-late-reports/__probe"},
	{path: "/api/v1/replenishment/classifications"},
	{path: "/api/v1/admin/holidays", roles: []string{"ADMIN"}},
}

func (row rbacRow) keyList(t *testing.T) []string {
	t.Helper()
	k := row.keys
	if k == "" {
		p, ok := policyFor(strings.Split(row.path, "/__probe")[0])
		if !ok || p.keys == "-" {
			t.Fatalf("no key mapping for %s in routePolicies", row.path)
		}
		k = p.keys
	}
	return strings.Split(k, "|")
}

func dialDevDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	url = strings.Replace(url, "host.docker.internal", "localhost", 1)
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// rbacHarness builds the real router over the dev DB (and a miniredis) and a
// token minter signed with the same secret. Read-only: nothing is written.
func rbacHarness(t *testing.T) (chi.Router, *pkgauth.TokenService, *pgxpool.Pool) {
	t.Helper()
	pool := dialDevDB(t)
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	secret := []byte(strings.Repeat("k", 32))
	cfg := &config.Config{
		JWTSecret: secret, AccessTokenExpiry: 15 * time.Minute, SessionMaxLifetime: time.Hour,
		RateLimitUsername: 5, RateLimitIP: 20, RateLimitWindow: time.Minute, Port: "0",
	}
	router, err := buildRouter(routerDeps{Cfg: cfg, DB: pool, DBRead: pool, Redis: rc})
	if err != nil {
		t.Fatalf("buildRouter: %v", err)
	}
	ts := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey: secret, AccessTokenExpiry: 15 * time.Minute, SessionMaxLifetime: time.Hour,
	}, nil)
	return router, ts, pool
}

func probe(t *testing.T, router http.Handler, ts *pkgauth.TokenService, role, path string) int {
	t.Helper()
	tok, _, err := ts.GenerateTokenPair(&pkgauth.AuthIdentity{UserID: 1, Username: "rbac-probe", Role: role})
	if err != nil {
		t.Fatalf("token for %s: %v", role, err)
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Code
}

// grants returns role -> set of held catalog keys, read straight from
// role_permissions (independent of the evaluator under test), plus all roles.
func grants(t *testing.T, pool *pgxpool.Pool) (map[string]map[string]bool, []string) {
	t.Helper()
	ctx := context.Background()
	rows, err := pool.Query(ctx, `SELECT role FROM roles ORDER BY role`)
	if err != nil {
		t.Fatalf("roles: %v", err)
	}
	var roles []string
	held := map[string]map[string]bool{}
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			t.Fatal(err)
		}
		roles = append(roles, r)
		held[r] = map[string]bool{}
	}
	rows.Close()
	grows, err := pool.Query(ctx, `SELECT r.role, mf.key FROM role_permissions rp
		JOIN roles r ON r.id = rp.role_id JOIN menu_features mf ON mf.id = rp.menu_feature_id`)
	if err != nil {
		t.Fatalf("grants: %v", err)
	}
	defer grows.Close()
	for grows.Next() {
		var r, k string
		if err := grows.Scan(&r, &k); err != nil {
			t.Fatal(err)
		}
		held[r][k] = true
	}
	return held, roles
}

func inList(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// Property 4 + 7: for every probed mount and every role in the dev DB, the
// route answers 403 exactly when the role is non-internal, lacks every key of
// the row, or sits outside the row's fixed role guard.
func TestRBACDenialPerMountRow(t *testing.T) {
	router, ts, pool := rbacHarness(t)
	held, roles := grants(t, pool)
	if len(roles) == 0 {
		t.Fatal("no roles in dev DB")
	}

	var sawAllow, sawDeny, sawGuardOnly, sawVendor bool
	for _, row := range rbacRows {
		keys := row.keyList(t)
		for _, role := range roles {
			hasKey := false
			for _, k := range keys {
				if held[role][k] || rolemgmt.IsAlwaysOn(k) {
					hasKey = true
				}
			}
			internal := rolemgmt.IsInternalRole(role)
			wantAllowed := internal && hasKey && (row.roles == nil || inList(row.roles, role))

			got := probe(t, router, ts, role, row.path)
			if got == http.StatusUnauthorized {
				t.Errorf("%s as %s: 401 (token rejected)", row.path, role)
				continue
			}
			if (got != http.StatusForbidden) != wantAllowed {
				t.Errorf("%s as %s: status %d, want allowed=%v (keys %v, holds=%v, guard=%v)",
					row.path, role, got, wantAllowed, keys, hasKey, row.roles)
			}
			switch {
			case wantAllowed:
				sawAllow = true
			case !internal && strings.Contains(row.path, "atm-portal"):
				sawVendor = true
			case internal && hasKey && row.roles != nil:
				sawGuardOnly = true
			default:
				sawDeny = true
			}
		}
	}
	// Guard against a vacuous pass (e.g. an empty grant table).
	if !sawAllow || !sawDeny || !sawGuardOnly || !sawVendor {
		t.Errorf("matrix too thin: allow=%v deny=%v key-but-outside-guard=%v vendor-atm-portal=%v",
			sawAllow, sawDeny, sawGuardOnly, sawVendor)
	}
}

// Req 7.3 / D8: Manajemen Peran stays APPACCESS/ADMIN whatever the catalog says.
func TestRBACRoleManagementStaysStatic(t *testing.T) {
	router, ts, pool := rbacHarness(t)
	_, roles := grants(t, pool)

	for _, path := range []string{"/api/v1/admin/roles", "/api/v1/admin/users"} {
		for _, role := range roles {
			got := probe(t, router, ts, role, path)
			static := role == "APPACCESS" || role == "ADMIN"
			if (got != http.StatusForbidden) != static {
				t.Errorf("%s as %s: status %d, want forbidden=%v", path, role, got, !static)
			}
		}
	}
}
