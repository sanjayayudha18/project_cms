//go:build integration

package repository

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// Migration 043 (vendor_pks_types, vendor_pks.pks_type_id/wod_number, per-type active index) —
// vendor-pks-type task 1.4. Reuses pksExec/pksRejected/pksCount from migration022_integration_test.go.
// Everything runs in the integrationTx transaction (rolled back on cleanup).

func TestMigration043Constraints(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()

	var userID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM users ORDER BY id LIMIT 1`).Scan(&userID); err != nil {
		t.Skipf("need a seeded user: %v", err)
	}
	var flmID, citID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM vendor_pks_types WHERE code='FLM' AND is_active`).Scan(&flmID); err != nil {
		t.Fatalf("seed type FLM missing: %v", err)
	}
	if err := tx.QueryRow(ctx, `SELECT id FROM vendor_pks_types WHERE code='CIT' AND is_active`).Scan(&citID); err != nil {
		t.Fatalf("seed type CIT missing: %v", err)
	}

	sfx := time.Now().UnixNano() % 1e12
	var vendorID int64
	if err := tx.QueryRow(ctx, `INSERT INTO vendors (code, name) VALUES ($1, 'ITEST PKS TYPE') RETURNING id`,
		fmt.Sprintf("PT%d", sfx)).Scan(&vendorID); err != nil {
		t.Fatalf("insert vendor: %v", err)
	}

	const insPKS = `INSERT INTO vendor_pks (vendor_id, pks_type_id, wod_number, pks_name, effective_date, expiry_date, created_by, replaced_at)
		VALUES ($1, $2, $3, 'PKS', '2026-01-01', '2027-12-31', $4, $5)`
	wod := func(s string) string { return fmt.Sprintf("WOD-%d-%s", sfx, s) }

	t.Run("one active PKS per (vendor, type)", func(t *testing.T) {
		pksExec(t, tx, insPKS, vendorID, flmID, wod("a"), userID, nil)
		pksRejected(t, tx, "second active, same type", insPKS, vendorID, flmID, wod("b"), userID, nil)
		pksExec(t, tx, insPKS, vendorID, citID, wod("c"), userID, nil) // other type: ok
		// History row (replaced) of the same type is allowed.
		pksExec(t, tx, insPKS, vendorID, flmID, wod("h"), userID, time.Now())
	})

	t.Run("WOD unique case-insensitive", func(t *testing.T) {
		pksRejected(t, tx, "same WOD, different case", insPKS, vendorID, flmID, strings.ToLower(wod("A")), userID, time.Now())
	})

	t.Run("WOD edge spaces and length", func(t *testing.T) {
		pksRejected(t, tx, "leading space", insPKS, vendorID, flmID, " "+wod("s1"), userID, time.Now())
		pksRejected(t, tx, "trailing space", insPKS, vendorID, flmID, wod("s2")+" ", userID, time.Now())
		pksRejected(t, tx, "empty", insPKS, vendorID, flmID, "", userID, time.Now())
		pksRejected(t, tx, ">100 chars", insPKS, vendorID, flmID, strings.Repeat("x", 101), userID, time.Now())
	})

	t.Run("type FK restrict and code rules", func(t *testing.T) {
		pksRejected(t, tx, "delete type in use", `DELETE FROM vendor_pks_types WHERE id=$1`, flmID)
		pksRejected(t, tx, "duplicate code, other case", `INSERT INTO vendor_pks_types (code, name) VALUES ('FLM', 'x')`)
		pksRejected(t, tx, "bad code", `INSERT INTO vendor_pks_types (code, name) VALUES ('flm1', 'x')`)
		pksRejected(t, tx, "blank name", `INSERT INTO vendor_pks_types (code, name) VALUES ('ZZ1', '   ')`)
	})

	t.Run("pending change per (vendor, type) and per WOD", func(t *testing.T) {
		const ins = `INSERT INTO master_data_change_requests (entity_type, entity_id, op, payload, maker_id, status)
			VALUES ('vendor_pks', NULL, 'create',
			        jsonb_build_object('vendor_id', $1::bigint, 'pks_type_id', $2::bigint, 'wod_number', $3::text), $4, 'pending')`
		pksExec(t, tx, ins, vendorID, flmID, wod("p1"), userID)
		pksRejected(t, tx, "same vendor+type pending", ins, vendorID, flmID, wod("p2"), userID)
		pksExec(t, tx, ins, vendorID, citID, wod("p3"), userID) // other type: ok
		pksRejected(t, tx, "same WOD pending (case-insens.)", ins, vendorID+1, flmID, strings.ToLower(wod("P3")), userID)

		const insType = `INSERT INTO master_data_change_requests (entity_type, entity_id, op, payload, maker_id, status)
			VALUES ('vendor_pks_type', NULL, 'create', jsonb_build_object('code', $1::text), $2, 'pending')`
		pksExec(t, tx, insType, "NEWT", userID)
		pksRejected(t, tx, "same type code pending", insType, "newt", userID)
	})
}

// Guard blocks: run the real DO block from the migration file against current state.
func TestMigration043Guard(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()

	raw, err := os.ReadFile("../../migrations/043_vendor_pks_type.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	s := string(raw)
	start := strings.Index(s, "DO $$")
	end := strings.Index(s[start:], "$$;")
	if start < 0 || end < 0 {
		t.Fatal("guard DO block not found in 043")
	}
	guard := s[start : start+end+3]

	runGuard := func() error {
		sp, err := tx.Begin(ctx)
		if err != nil {
			t.Fatalf("savepoint: %v", err)
		}
		defer func() { _ = sp.Rollback(ctx) }()
		_, err = sp.Exec(ctx, guard)
		return err
	}

	var userID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM users ORDER BY id LIMIT 1`).Scan(&userID); err != nil {
		t.Skipf("need a seeded user: %v", err)
	}

	if pksCount(t, tx, `SELECT count(*) FROM vendor_pks`) == 0 &&
		pksCount(t, tx, `SELECT count(*) FROM master_data_change_requests WHERE entity_type='vendor_pks' AND status='pending'`) == 0 {
		if err := runGuard(); err != nil {
			t.Errorf("guard must pass on empty state, got %v", err)
		}
	}

	t.Run("pending vendor_pks change", func(t *testing.T) {
		pksExec(t, tx, `INSERT INTO master_data_change_requests (entity_type, entity_id, op, payload, maker_id, status)
			VALUES ('vendor_pks', NULL, 'create', jsonb_build_object('vendor_id', -1::bigint, 'pks_type_id', -1::bigint, 'wod_number', 'GUARD-W'), $1, 'pending')`, userID)
		if err := runGuard(); err == nil || !strings.Contains(err.Error(), "043") {
			t.Errorf("guard should raise on pending change, got %v", err)
		}
	})

	t.Run("vendor_pks has rows", func(t *testing.T) {
		var vendorID, typeID int64
		if err := tx.QueryRow(ctx, `INSERT INTO vendors (code, name) VALUES ($1, 'ITEST GUARD') RETURNING id`,
			fmt.Sprintf("GD%d", time.Now().UnixNano()%1e12)).Scan(&vendorID); err != nil {
			t.Fatalf("insert vendor: %v", err)
		}
		if err := tx.QueryRow(ctx, `SELECT id FROM vendor_pks_types WHERE code='FLM'`).Scan(&typeID); err != nil {
			t.Fatalf("type: %v", err)
		}
		pksExec(t, tx, `INSERT INTO vendor_pks (vendor_id, pks_type_id, wod_number, pks_name, effective_date, expiry_date, created_by)
			VALUES ($1, $2, $3, 'PKS', '2026-01-01', '2027-12-31', $4)`, vendorID, typeID, fmt.Sprintf("GUARD-%d", vendorID), userID)
		if err := runGuard(); err == nil || !strings.Contains(err.Error(), "043") {
			t.Errorf("guard should raise on non-empty vendor_pks, got %v", err)
		}
	})
}
