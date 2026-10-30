//go:build integration

package exportjob

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// report-export-migration task 4.2 (Property 9): migration 035 keeps every 034-era row valid
// and admits the four report kinds only in their own shape. Each case runs in its own
// transaction that is always rolled back, so nothing is left in export_jobs.

func TestMigration035KindConstraints(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set -- skipping integration test")
	}
	url = strings.Replace(url, "host.docker.internal", "localhost", 1)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	// Existing rows only referenced by FK; nothing here is modified.
	var userID, vendorID, branchID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM users ORDER BY id LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("pick user: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT vendor_id, id FROM vendor_branches ORDER BY id LIMIT 1`).Scan(&vendorID, &branchID); err != nil {
		t.Fatalf("pick vendor branch: %v", err)
	}

	str := func(s string) *string { return &s }
	id := func(v int64) *int64 { return &v }

	cases := []struct {
		name         string
		kind         string
		vendorID     *int64
		branchID     *int64
		statusFilter *string
		priceLevel   *string
		reportParams *string // JSON text; nil = SQL NULL
		wantConstr   string  // "" = insert must succeed
	}{
		// valid: 034-style master-data rows
		{name: "ok vendor", kind: "vendor", vendorID: id(vendorID), statusFilter: str("active")},
		{name: "ok cabang", kind: "cabang", vendorID: id(vendorID), branchID: id(branchID), statusFilter: str("all")},
		{name: "ok harga_paket", kind: "harga_paket", vendorID: id(vendorID), statusFilter: str("disabled"), priceLevel: str("branch")},
		{name: "ok master_data_global", kind: "master_data_global", statusFilter: str("active")},
		// valid: report kinds
		{name: "ok dsr_late_daily", kind: "dsr_late_daily", reportParams: str(`{"month":"2026-10"}`)},
		{name: "ok dsr_late_summary", kind: "dsr_late_summary", reportParams: str(`{"month":"2026-10"}`)},
		{name: "ok replenishment_amount_mismatch", kind: "replenishment_amount_mismatch", reportParams: str(`{"date_from":"2026-10-01","date_to":"2026-10-07"}`)},
		{name: "ok replenishment_trips", kind: "replenishment_trips", reportParams: str(`{}`)},
		// rejected
		{name: "report without report_params", kind: "dsr_late_daily", wantConstr: "export_jobs_params_kind_chk"},
		{name: "report with status_filter", kind: "dsr_late_daily", statusFilter: str("active"), reportParams: str(`{}`), wantConstr: "export_jobs_params_kind_chk"},
		{name: "report with vendor_id", kind: "replenishment_trips", vendorID: id(vendorID), reportParams: str(`{}`), wantConstr: "export_jobs_scope_chk"},
		{name: "report with vendor_branch_id", kind: "replenishment_trips", vendorID: id(vendorID), branchID: id(branchID), reportParams: str(`{}`), wantConstr: "export_jobs_scope_chk"},
		{name: "report_params JSON array", kind: "dsr_late_summary", reportParams: str(`[1,2]`), wantConstr: "export_jobs_params_kind_chk"},
		{name: "report_params JSON scalar", kind: "dsr_late_summary", reportParams: str(`"x"`), wantConstr: "export_jobs_params_kind_chk"},
		{name: "master kind with report_params", kind: "vendor", vendorID: id(vendorID), statusFilter: str("active"), reportParams: str(`{}`), wantConstr: "export_jobs_params_kind_chk"},
		{name: "master kind without status_filter", kind: "master_data_global", wantConstr: "export_jobs_params_kind_chk"},
		{name: "unknown kind", kind: "bogus", statusFilter: str("active"), wantConstr: "export_jobs_kind_chk"},
	}

	const q = `INSERT INTO export_jobs
		(requested_by, kind, entity, vendor_id, vendor_branch_id, status_filter, price_level, report_params, params_hash, file_name)
		VALUES ($1, $2, 'm035', $3, $4, $5, $6, $7::jsonb, 'm035-hash', 'm035.csv')`

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatalf("begin: %v", err)
			}
			defer func() { _ = tx.Rollback(ctx) }()

			_, err = tx.Exec(ctx, q, userID, tc.kind, tc.vendorID, tc.branchID, tc.statusFilter, tc.priceLevel, tc.reportParams)
			if tc.wantConstr == "" {
				if err != nil {
					t.Fatalf("insert rejected: %v", err)
				}
				return
			}
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) {
				t.Fatalf("want check violation %s, got %v", tc.wantConstr, err)
			}
			if pgErr.Code != "23514" || pgErr.ConstraintName != tc.wantConstr {
				t.Fatalf("got SQLSTATE %s constraint %q, want 23514 %q", pgErr.Code, pgErr.ConstraintName, tc.wantConstr)
			}
		})
	}
}
