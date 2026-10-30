//go:build integration

package handler

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/service"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// report-export-migration 8.3, Property 11 against the real handlers + services + dev DB (read-only):
// format=csv on the four report endpoints is a 400 and writes no audit_logs row.
func TestCutoverIntegration_FormatCSVWritesNoAudit(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}
	dbURL = strings.Replace(dbURL, "host.docker.internal", "localhost", 1)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	var actorID int64
	if err := pool.QueryRow(ctx, "SELECT min(id) FROM users").Scan(&actorID); err != nil {
		t.Fatalf("existing user id: %v", err)
	}
	count := func() (actor, total int64) {
		t.Helper()
		if err := pool.QueryRow(ctx, "SELECT count(*) FILTER (WHERE actor_id = $1), count(*) FROM audit_logs", actorID).Scan(&actor, &total); err != nil {
			t.Fatalf("count audit_logs: %v", err)
		}
		return
	}
	beforeActor, beforeTotal := count()

	ts := testTokenService()
	r := chi.NewRouter()
	auth := r.With(custommw.RequireAuth(ts))
	auth.Mount("/dsr", NewDsrLateHandler(service.NewDsrLateReportService(pool, nil)).Routes())
	auth.Mount("/repl", NewReplenishmentHandler(service.NewReplenishmentReadService(pool)).Routes())

	const rng = "date_from=2026-10-01&date_to=2026-10-14"
	cases := []struct{ name, path, role string }{
		{"dsr daily", "/dsr/daily?month=2026-10&format=csv", "ADMIN"},
		{"dsr summary", "/dsr/summary?month=2026-10&format=csv", "ATM-SPV"},
		{"amount-mismatch", "/repl/reports/amount-mismatch?" + rng + "&format=csv", "ADMIN"},
		{"trips", "/repl/reports/trips?" + rng + "&format=csv", "ATM-SPV"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(r, http.MethodGet, tc.path, tokenForRole(t, ts, actorID, tc.role), "")
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"field":"format"`) {
				t.Fatalf("code = %d, body %s", rec.Code, rec.Body.String())
			}
			if ct := rec.Header().Get("Content-Type"); strings.HasPrefix(ct, "text/csv") {
				t.Errorf("Content-Type = %q", ct)
			}
			if a, tot := count(); a != beforeActor || tot != beforeTotal {
				t.Errorf("audit_logs changed: actor %d->%d, total %d->%d", beforeActor, a, beforeTotal, tot)
			}
		})
	}
}
