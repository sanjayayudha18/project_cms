package main

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/cimb-niaga/cms/pkg/config"
)

// routePolicy is one row of the design mount table (spec
// permission-catalog-coverage, design.md "Pemetaan mount → Screen_Key").
// minMW = minimum inline middlewares chi sees on every route under the prefix:
// RequireAuth + RequirePermission (+ the fixed RequireRoles guard). 0 = the
// key is enforced inside the handler (WithPermissions) or the prefix is
// excluded, which chi.Walk cannot see.
type routePolicy struct {
	prefix string
	keys   string // screen key(s), or "-" for an excluded prefix
	minMW  int
}

var routePolicies = []routePolicy{
	// Mount-level RequirePermission.
	{"/api/v1/dmaa-forecast", "forecasting.dmaa-forecast", 2},
	{"/api/v1/atm-portal", "monitoring.atm-portal", 2},
	{"/api/v1/audit-logs", "monitoring.audit-logs", 2},
	{"/api/v1/vault-plans", "cit.vault-plan", 2},
	{"/api/v1/admin/pks", "settings.pks", 2},
	{"/api/v1/admin/vendors", "settings.vendors|settings.pks", 2}, // vendorRead
	{"/api/v1/admin/vendors/{vendorID}/vaults", "settings.vendors", 3},
	{"/api/v1/admin/vendors/{vendorID}/dsr-location-maps", "settings.vendors", 3},
	{"/api/v1/admin/vendors/{vendorID}/pics", "settings.vendors", 3},
	{"/api/v1/admin/vendors/{vendorID}/pks", "settings.vendors", 3},
	{"/api/v1/admin/vendors/{vendorID}/packages", "settings.vendors", 3},
	{"/api/v1/admin/vendors/{vendorID}/package-prices", "settings.vendors", 3},
	{"/api/v1/admin/vendors/{vendorID}/branches", "settings.vendors", 3},
	{"/api/v1/admin/atms", "settings.atms", 3},
	{"/api/v1/admin/regions", "settings.regions", 3},
	{"/api/v1/admin/pks-types", "settings.pks-types", 3},
	{"/api/v1/admin/master-data/changes", "settings.master-data-io", 3},
	{"/api/v1/admin/master-data/import", "settings.master-data-io", 3},
	{"/api/v1/admin/acm-areas", "settings.acm-areas", 3},
	{"/api/v1/cash-count/category-params", "cash-count.parameters", 3},
	{"/api/v1/cash-count/anomaly-categories", "cash-count.parameters", 3},
	{"/api/v1/cash-count/checklist-items", "cash-count.parameters", 3},
	{"/api/v1/cash-count/areas", "cash-count.pic-mapping", 3},
	{"/api/v1/cash-count/pics", "cash-count.pic-mapping", 3},
	{"/api/v1/cash-count/import", "cash-count.pic-mapping", 3},
	{"/api/v1/admin/approval", "settings.rbac-*", 3},

	// Handler-level guards (WithPermissions); not visible to chi.Walk.
	{"/api/v1/vendor-requests", "replenish.vendor-requests|replenish.forecast-browser", 0},
	{"/api/v1/atm-visit-quotas", "replenish.vendor-requests", 0},
	{"/api/v1/dsr-late-reports", "monitoring.dsr-late-report", 0},
	{"/api/v1/replenishment", "replenish.schedule", 0},
	{"/api/v1/admin/holidays", "forecasting.holidays", 0},

	// Explicitly excluded (design table): no RequirePermission by design.
	{"/api/v1/admin/users", "-", 0},   // static APPACCESS/ADMIN (D8)
	{"/api/v1/admin/roles", "-", 0},   // static APPACCESS/ADMIN (D8)
	{"/api/v1/approvals", "-", 0},     // always-on / infrastructure
	{"/api/v1/notifications", "-", 0}, // always-on
	{"/api/v1/auth", "-", 0},          // auth
	{"/api/v1/dsr", "-", 0},           // used by VendorPortal
	{"/api/v1/vendor", "-", 0},        // vendorrole.All (D6)
	{"/health", "-", 0},               // public
	{"/ready", "-", 0},                // public
}

// policyFor returns the longest-prefix policy for a route pattern. A prefix
// matches only at a path-segment boundary so /api/v1/dsr does not swallow
// /api/v1/dsr-late-reports.
func policyFor(route string) (routePolicy, bool) {
	var best routePolicy
	found := false
	for _, p := range routePolicies {
		if route == p.prefix || strings.HasPrefix(route, p.prefix+"/") {
			if !found || len(p.prefix) > len(best.prefix) {
				best, found = p, true
			}
		}
	}
	return best, found
}

// newWalkRouter builds the real router over lazy pools: pgxpool.New and
// redis.NewClient do not connect until first use, and chi.Walk never serves a
// request, so no DB/Redis is needed.
func newWalkRouter(t *testing.T) chi.Router {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), "postgres://u:p@127.0.0.1:1/none?sslmode=disable")
	if err != nil {
		t.Fatalf("lazy pool: %v", err)
	}
	t.Cleanup(pool.Close)
	rc := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { _ = rc.Close() })

	cfg := &config.Config{
		JWTSecret:          []byte(strings.Repeat("k", 32)),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
		RateLimitUsername:  5,
		RateLimitIP:        20,
		RateLimitWindow:    time.Minute,
		Port:               "0",
	}
	r, err := buildRouter(routerDeps{Cfg: cfg, DB: pool, DBRead: pool, Redis: rc})
	if err != nil {
		t.Fatalf("buildRouter: %v", err)
	}
	return r
}

// Req 9.2: every route belongs to a prefix in the design mount table (moved or
// explicitly excluded), and mount-level prefixes keep their guard middlewares.
func TestRouteWalkPermissionMapping(t *testing.T) {
	r := newWalkRouter(t)

	// chi.Walk reports the router-wide r.Use middlewares on every route too;
	// a route's guard count is what it has beyond /health's.
	type hit struct {
		desc string
		p    routePolicy
		mws  int
	}
	var unmapped, weak []string
	var hits []hit
	globalMW := -1
	seen := map[string]bool{}
	err := chi.Walk(r, func(method, route string, _ http.Handler, mws ...func(http.Handler) http.Handler) error {
		route = strings.TrimSuffix(route, "/*")
		if route == "/health" {
			globalMW = len(mws)
		}
		p, ok := policyFor(route)
		if !ok {
			unmapped = append(unmapped, method+" "+route)
			return nil
		}
		seen[p.prefix] = true
		hits = append(hits, hit{method + " " + route + " (keys " + p.keys + ")", p, len(mws)})
		return nil
	})
	if err != nil {
		t.Fatalf("chi.Walk: %v", err)
	}
	if globalMW < 0 {
		t.Fatal("GET /health not found; cannot derive the router-wide middleware count")
	}
	for _, h := range hits {
		if h.mws-globalMW < h.p.minMW {
			weak = append(weak, h.desc)
		}
	}

	sort.Strings(unmapped)
	sort.Strings(weak)
	if len(unmapped) > 0 {
		t.Errorf("routes with no entry in the mount table (add a key or an explicit exclusion):\n%s", strings.Join(unmapped, "\n"))
	}
	if len(weak) > 0 {
		t.Errorf("routes with fewer guard middlewares than the mount table requires:\n%s", strings.Join(weak, "\n"))
	}

	// A table row nobody hits is stale (or its mount was dropped).
	for _, p := range routePolicies {
		if !seen[p.prefix] {
			t.Errorf("mount table prefix %s matches no route", p.prefix)
		}
	}
}
