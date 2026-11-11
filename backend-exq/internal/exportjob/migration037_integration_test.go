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

// vendor-pks task 2.2: migration 037 admits report kind 'vendor_pks' only in the report shape
// (report_params object, no status_filter, no vendor/branch scope) and keeps the 035 cases valid.
// Each case runs in its own transaction that is always rolled back.

func TestMigration037VendorPksKind(t *testing.T) {
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
		reportParams *string // JSON text; nil = SQL NULL
		wantConstr   string  // "" = insert must succeed
	}{
		// valid: vendor_pks
		{name: "ok vendor_pks empty params", kind: "vendor_pks", reportParams: str(`{}`)},
		{name: "ok vendor_pks with filters", kind: "vendor_pks", reportParams: str(`{"q":"abc","status":"active","pks_status":"expired"}`)},
		// valid: 035 cases still pass
		{name: "ok vendor", kind: "vendor", vendorID: id(vendorID), statusFilter: str("active")},
		{name: "ok cabang", kind: "cabang", vendorID: id(vendorID), branchID: id(branchID), statusFilter: str("all")},
		{name: "ok master_data_global", kind: "master_data_global", statusFilter: str("active")},
		{name: "ok dsr_late_daily", kind: "dsr_late_daily", reportParams: str(`{"month":"2026-10"}`)},
		{name: "ok replenishment_trips", kind: "replenishment_trips", reportParams: str(`{}`)},
		// rejected
		{name: "vendor_pks without report_params", kind: "vendor_pks", wantConstr: "export_jobs_params_kind_chk"},
		{name: "vendor_pks with status_filter", kind: "vendor_pks", statusFilter: str("active"), reportParams: str(`{}`), wantConstr: "export_jobs_params_kind_chk"},
		{name: "vendor_pks report_params JSON array", kind: "vendor_pks", reportParams: str(`[1]`), wantConstr: "export_jobs_params_kind_chk"},
		{name: "vendor_pks with vendor_id", kind: "vendor_pks", vendorID: id(vendorID), reportParams: str(`{}`), wantConstr: "export_jobs_scope_chk"},
		{name: "vendor_pks with vendor_branch_id", kind: "vendor_pks", vendorID: id(vendorID), branchID: id(branchID), reportParams: str(`{}`), wantConstr: "export_jobs_scope_chk"},
		{name: "unknown kind", kind: "bogus", statusFilter: str("active"), wantConstr: "export_jobs_kind_chk"},
	}

	const q = `INSERT INTO export_jobs
		(requested_by, kind, entity, vendor_id, vendor_branch_id, status_filter, report_params, params_hash, file_name)
		VALUES ($1, $2, 'm037', $3, $4, $5, $6::jsonb, 'm037-hash', 'm037.csv')`

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatalf("begin: %v", err)
			}
			defer func() { _ = tx.Rollback(ctx) }()

			_, err = tx.Exec(ctx, q, userID, tc.kind, tc.vendorID, tc.branchID, tc.statusFilter, tc.reportParams)
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
