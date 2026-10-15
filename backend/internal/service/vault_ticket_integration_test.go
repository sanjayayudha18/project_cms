//go:build integration

package service

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// cit-vault-select FR6: applyVaultTickets against real Postgres -- numbering
// per (vault, date), keep / deactivate / reactivate with the same number,
// numbers never reused. One outer tx rolled back on cleanup.
func TestIntegration_ApplyVaultTickets(t *testing.T) {
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
		t.Fatalf("begin outer tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	q := db.New(tx)

	var req1, req2 int64
	// Requests without vault tickets: a real dev request with tickets would see them deactivated by sync.
	if err := tx.QueryRow(ctx, `SELECT min(id), max(id) FROM vendor_requests vr
		WHERE NOT EXISTS (SELECT 1 FROM vendor_request_vault_tickets t WHERE t.vendor_request_id = vr.id)`).Scan(&req1, &req2); err != nil || req1 == req2 {
		t.Skipf("need two vendor_requests in dev (err=%v)", err)
	}
	var v1, v2 int64
	var code1, code2 string
	if err := tx.QueryRow(ctx, `SELECT a.id, a.vault_code, b.id, b.vault_code FROM
		(SELECT id, vault_code FROM vendor_vaults ORDER BY id LIMIT 1) a,
		(SELECT id, vault_code FROM vendor_vaults ORDER BY id OFFSET 1 LIMIT 1) b`).Scan(&v1, &code1, &v2, &code2); err != nil {
		t.Skipf("need two vendor_vaults in dev: %v", err)
	}
	date := pgtype.Date{Time: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}

	sync := func(req int64, wanted ...int64) vaultTicketAudit {
		t.Helper()
		existing, err := q.ListVaultTicketsForRequest(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		a, err := applyVaultTickets(ctx, q, req, date, planVaultTickets(existing, wanted))
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	check := func(name string, got, want vaultTicketAudit) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: audit = %+v, want %+v", name, got, want)
		}
	}
	n := func(code string, seq string) string { return code + "_20990101_" + seq }
	none := []string{}

	check("first submit", sync(req1, v1, v2),
		vaultTicketAudit{Issued: []string{n(code1, "001"), n(code2, "001")}, Kept: none, Reactivated: none, Deactivated: none})
	check("resubmit same vaults", sync(req1, v1, v2),
		vaultTicketAudit{Issued: none, Kept: []string{n(code1, "001"), n(code2, "001")}, Reactivated: none, Deactivated: none})
	check("drop v2", sync(req1, v1),
		vaultTicketAudit{Issued: none, Kept: []string{n(code1, "001")}, Reactivated: none, Deactivated: []string{n(code2, "001")}})
	check("other request same vault+date gets next number", sync(req2, v2),
		vaultTicketAudit{Issued: []string{n(code2, "002")}, Kept: none, Reactivated: none, Deactivated: none})
	check("v2 back in req1 keeps its old number", sync(req1, v1, v2),
		vaultTicketAudit{Issued: none, Kept: []string{n(code1, "001")}, Reactivated: []string{n(code2, "001")}, Deactivated: none})
}
