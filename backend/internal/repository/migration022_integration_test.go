//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// Migrations 022 (vendor_pks, vendor_pics.is_director, status functions) and 037
// (director roles constraint) — vendor-pks task 1.3. Everything runs inside the
// integrationTx transaction, rolled back on cleanup. Statements expected to fail
// run in a savepoint so the outer tx stays usable.

func pksExec(t *testing.T, tx pgx.Tx, sql string, args ...any) {
	t.Helper()
	if _, err := tx.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec failed: %v\n%s", err, sql)
	}
}

// pksRejected asserts the statement violates a constraint (SQLSTATE class 23).
func pksRejected(t *testing.T, tx pgx.Tx, name, sql string, args ...any) {
	t.Helper()
	ctx := context.Background()
	sp, err := tx.Begin(ctx)
	if err != nil {
		t.Fatalf("%s: savepoint: %v", name, err)
	}
	defer func() { _ = sp.Rollback(ctx) }()
	if _, err := sp.Exec(ctx, sql, args...); err == nil {
		t.Errorf("%s: expected constraint violation, got success", name)
	}
}

func pksCount(t *testing.T, tx pgx.Tx, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := tx.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v\n%s", err, sql)
	}
	return n
}

func TestMigration022Schema(t *testing.T) {
	tx := integrationTx(t)

	for _, c := range []struct{ table, col string }{
		{"vendor_pics", "is_director"},
		{"vendor_pks", "vendor_id"}, {"vendor_pks", "pks_name"}, {"vendor_pks", "effective_date"},
		{"vendor_pks", "expiry_date"}, {"vendor_pks", "created_by"}, {"vendor_pks", "replaced_at"},
		{"vendor_pks", "replaced_by_pks_id"}, {"vendor_pks", "terminated_at"},
		{"vendor_pks", "terminated_by"}, {"vendor_pks", "termination_reason"},
	} {
		if pksCount(t, tx, `SELECT count(*) FROM information_schema.columns
			WHERE table_schema='public' AND table_name=$1 AND column_name=$2`, c.table, c.col) != 1 {
			t.Errorf("column %s.%s missing", c.table, c.col)
		}
	}

	for _, name := range []string{
		"vendor_pics_director_chk", "vendor_pics_director_roles_chk", "vendor_pks_dates_chk",
		"vendor_pks_terminate_chk", "vendor_pks_replaced_chk", "vendor_pks_name_chk",
	} {
		if pksCount(t, tx, `SELECT count(*) FROM pg_constraint WHERE conname=$1 AND contype='c'`, name) != 1 {
			t.Errorf("CHECK %s missing", name)
		}
	}
	for _, name := range []string{
		"vendor_pics_director_uq", "vendor_pks_active_uq", "vendor_pks_vendor_idx",
		"master_data_change_requests_vendor_pks_pending_uq",
	} {
		if pksCount(t, tx, `SELECT count(*) FROM pg_indexes WHERE schemaname='public' AND indexname=$1`, name) != 1 {
			t.Errorf("index %s missing", name)
		}
	}
	for _, fn := range []string{"vendor_pks_status", "vendor_pks_validity_months"} {
		if pksCount(t, tx, `SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
			WHERE n.nspname='public' AND p.proname=$1`, fn) != 1 {
			t.Errorf("function %s missing", fn)
		}
	}

	// D32: vendors is untouched by 022/037.
	for _, col := range []string{"is_director", "pks_name", "pks_effective_date", "pks_expiry_date", "pks_terminated_at"} {
		if pksCount(t, tx, `SELECT count(*) FROM information_schema.columns
			WHERE table_schema='public' AND table_name='vendors' AND column_name=$1`, col) != 0 {
			t.Errorf("vendors.%s must not exist", col)
		}
	}

	// Rows that predate the migration default to is_director = false.
	if n := pksCount(t, tx, `SELECT count(*) FROM vendor_pics WHERE is_director`); n != 0 {
		t.Errorf("existing vendor_pics with is_director=true: %d, want 0", n)
	}
}

func TestMigration022Constraints(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()

	var userID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM users ORDER BY id LIMIT 1`).Scan(&userID); err != nil {
		t.Skipf("need a seeded user: %v", err)
	}
	code := fmt.Sprintf("PK%d", time.Now().UnixNano()%1e12)
	var vendorID, branchID int64
	if err := tx.QueryRow(ctx, `INSERT INTO vendors (code, name) VALUES ($1, 'ITEST PKS') RETURNING id`, code).Scan(&vendorID); err != nil {
		t.Fatalf("insert vendor: %v", err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO vendor_branches (vendor_id, branch_code, branch_name) VALUES ($1, 'B1', 'Cabang 1') RETURNING id`, vendorID).Scan(&branchID); err != nil {
		t.Fatalf("insert vendor_branch: %v", err)
	}

	const insPIC = `INSERT INTO vendor_pics (vendor_id, vendor_branch_id, name, is_notification_recipient, roles, is_director, deleted_at)
		VALUES ($1, $2, 'Dir', $3, $4, true, $5)`
	none := []string{}

	t.Run("director rules", func(t *testing.T) {
		pksRejected(t, tx, "director with branch", insPIC, vendorID, branchID, false, none, nil)
		pksRejected(t, tx, "director notification recipient", insPIC, vendorID, nil, true, none, nil)
		pksRejected(t, tx, "director with roles", insPIC, vendorID, nil, false, []string{"replenish"}, nil)

		pksExec(t, tx, insPIC, vendorID, nil, false, none, nil)
		pksRejected(t, tx, "second active director", insPIC, vendorID, nil, false, none, nil)
		// A soft-deleted director next to the active one is allowed.
		pksExec(t, tx, insPIC, vendorID, nil, false, none, time.Now())
	})

	const insPKS = `INSERT INTO vendor_pks (vendor_id, pks_name, effective_date, expiry_date, created_by, replaced_at)
		VALUES ($1, 'PKS', $2::date, $3::date, $4, $5)`

	t.Run("pks rules", func(t *testing.T) {
		pksRejected(t, tx, "expiry before effective", insPKS, vendorID, "2026-02-01", "2026-01-31", userID, nil)

		pksExec(t, tx, insPKS, vendorID, "2026-01-01", "2027-12-31", userID, nil)
		pksRejected(t, tx, "second active PKS", insPKS, vendorID, "2026-01-01", "2027-12-31", userID, nil)
		// A replaced (history) row next to the active one is allowed.
		pksExec(t, tx, insPKS, vendorID, "2024-01-01", "2025-12-31", userID, time.Now())
	})

	t.Run("pending change uniqueness", func(t *testing.T) {
		const insChange = `INSERT INTO master_data_change_requests (entity_type, entity_id, op, payload, maker_id, status)
			VALUES ('vendor_pks', NULL, 'create', jsonb_build_object('vendor_id', $1::bigint), $2, $3)`
		pksExec(t, tx, insChange, vendorID, userID, "pending")
		pksRejected(t, tx, "second pending vendor_pks change", insChange, vendorID, userID, "pending")
		// Once the first is no longer pending, a new pending one is fine.
		pksExec(t, tx, `UPDATE master_data_change_requests SET status='rejected'
			WHERE entity_type='vendor_pks' AND (payload->>'vendor_id')::bigint=$1`, vendorID)
		pksExec(t, tx, insChange, vendorID, userID, "pending")
	})
}
