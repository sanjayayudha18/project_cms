//go:build integration

package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// dsr-late-report FR4 on real Postgres: the report queries run against the real schema (content is
// covered by the repository integration test). Read-only; the CSV export audit moved to backend-exq.
func TestIntegration_DsrLateReport_Queries(t *testing.T) {
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

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	svc := NewDsrLateReportService(pool, &start)
	actor := Actor{UserID: 1, Role: "ATM-SPV", IP: "10.0.0.7"}
	f := DsrLateFilter{Month: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}

	if _, err := svc.Daily(ctx, actor, f); err != nil {
		t.Errorf("Daily: %v", err)
	}
	if _, err := svc.Summary(ctx, actor, f); err != nil {
		t.Errorf("Summary: %v", err)
	}
	if _, err := svc.Warnings(ctx, actor, f); err != nil {
		t.Errorf("Warnings: %v", err)
	}
}
