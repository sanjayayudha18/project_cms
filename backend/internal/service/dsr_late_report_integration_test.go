//go:build integration

package service

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// dsr-late-report FR4.4/FR7 on real Postgres: one outer tx rolled back on cleanup serves as
// primary and replica (same convention as acmAreaHarness).
func TestIntegration_DsrLateReport_ExportAudit(t *testing.T) {
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

	var adminID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE username = 'Yudha'`).Scan(&adminID); err != nil {
		t.Fatalf("loading seeded admin user: %v", err)
	}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	svc := NewDsrLateReportService(tx, tx, &start)
	actor := Actor{UserID: adminID, Role: "ATM-SPV", IP: "10.0.0.7"}

	count := func() (n int) {
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action = 'report_exported' AND entity_type = 'dsr_late_report'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := count()

	if err := svc.RecordExport(ctx, Actor{UserID: adminID, Role: "VENDOR-USER"}, "daily", nil); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("vendor export err = %v, want ErrNotAuthorized", err)
	}
	if count() != before {
		t.Fatal("denied export must not write audit_logs")
	}

	filter := map[string]any{"month": "2026-10", "vendor_id": nil}
	if err := svc.RecordExport(ctx, actor, "daily", filter); err != nil {
		t.Fatalf("RecordExport: %v", err)
	}
	if count() != before+1 {
		t.Fatalf("audit rows = %d, want %d", count(), before+1)
	}
	var report, month, ip string
	var actorID int64
	if err := tx.QueryRow(ctx, `
		SELECT actor_id, after->>'report', after->'filter'->>'month', ip::text FROM audit_logs
		WHERE action = 'report_exported' AND entity_type = 'dsr_late_report' ORDER BY id DESC LIMIT 1`).Scan(&actorID, &report, &month, &ip); err != nil {
		t.Fatal(err)
	}
	if actorID != adminID || report != "daily" || month != "2026-10" || ip == "" {
		t.Errorf("audit row = actor %d report %q month %q ip %q", actorID, report, month, ip)
	}

	// The report queries run against the real schema (content is covered by the repository integration test).
	if _, err := svc.Daily(ctx, actor, DsrLateFilter{Month: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Errorf("Daily: %v", err)
	}
	if _, err := svc.Summary(ctx, actor, DsrLateFilter{Month: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Errorf("Summary: %v", err)
	}
	if _, err := svc.Warnings(ctx, actor, DsrLateFilter{Month: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Errorf("Warnings: %v", err)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := svc.RecordExport(cancelled, actor, "daily", filter); err == nil {
		t.Error("audit failure must surface as an error (handler then sends no CSV)")
	}
}
