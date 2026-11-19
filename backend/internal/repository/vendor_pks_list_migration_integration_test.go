//go:build integration

package repository

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"pgregory.net/rapid"
)

// Migration 044 (.kiro/specs/vendor-pks-list/, task 1.4): renewal_days CHECKs + default,
// menu/permission seed, and Property 1 (vendor_pks_record_status == Go reference).
// Real dev DB (044 applied); every write is inside integrationTx and rolled back.

// refPksStatus is the reference for vendor_pks_record_status (design D4–D6).
func refPksStatus(replaced, terminated bool, effective, expiry time.Time, renewalDays int, today time.Time) string {
	switch {
	case replaced:
		return "diganti"
	case terminated:
		return "terminated"
	case effective.After(today):
		return "belum_berlaku"
	case expiry.Before(today):
		return "expired"
	case expiry.Before(today.AddDate(0, 0, renewalDays)):
		return "need_renewal"
	default:
		return "aktif"
	}
}

func TestVendorPksListMigration_RenewalDaysChecks(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()
	tag := time.Now().Format("150405.000000")

	var vendorID, typeID, userID int64
	if err := tx.QueryRow(ctx, `INSERT INTO vendors (code, name) VALUES ($1, $1) RETURNING id`, "P44-"+tag).Scan(&vendorID); err != nil {
		t.Fatalf("seed vendor: %v", err)
	}
	if err := tx.QueryRow(ctx, `SELECT id FROM vendor_pks_types ORDER BY id LIMIT 1`).Scan(&typeID); err != nil {
		t.Fatalf("pks type: %v", err)
	}
	if err := tx.QueryRow(ctx, `SELECT id FROM users ORDER BY id LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("user: %v", err)
	}

	start := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

	// tryInsert runs in a savepoint so a CHECK violation does not abort the outer tx.
	// replaced_at is set so rows stay out of the active-unique indexes.
	tryInsert := func(name string, termDays int, renewal *int) error {
		sp, err := tx.Begin(ctx)
		if err != nil {
			t.Fatalf("savepoint: %v", err)
		}
		defer func() { _ = sp.Rollback(ctx) }()
		wod := "W44-" + tag + "-" + name
		end := start.AddDate(0, 0, termDays)
		if renewal == nil {
			_, err = sp.Exec(ctx, `INSERT INTO vendor_pks (vendor_id, pks_name, effective_date, expiry_date, created_by, pks_type_id, wod_number, replaced_at)
				VALUES ($1, 'n', $2, $3, $4, $5, $6, now())`, vendorID, start, end, userID, typeID, wod)
			return err
		}
		_, err = sp.Exec(ctx, `INSERT INTO vendor_pks (vendor_id, pks_name, effective_date, expiry_date, created_by, pks_type_id, wod_number, replaced_at, renewal_days)
			VALUES ($1, 'n', $2, $3, $4, $5, $6, now(), $7)`, vendorID, start, end, userID, typeID, wod, *renewal)
		return err
	}
	ptr := func(i int) *int { return &i }

	for _, tc := range []struct {
		name     string
		termDays int
		renewal  *int
		wantErr  bool
	}{
		{"r29", 4000, ptr(29), true},
		{"r30", 4000, ptr(30), false},
		{"r3666", 4000, ptr(3666), false},
		{"r3667", 5000, ptr(3667), true},
		{"gtterm", 100, ptr(101), true},  // > masa PKS
		{"eqterm", 100, ptr(100), false}, // = masa PKS
		{"default270", 400, nil, false},  // default 270 <= masa
		{"defaultgt", 100, nil, true},    // default 270 > masa 100 → ditolak
	} {
		err := tryInsert(tc.name, tc.termDays, tc.renewal)
		if (err != nil) != tc.wantErr {
			t.Errorf("%s: err=%v, wantErr=%v", tc.name, err, tc.wantErr)
		}
	}

	var def string
	if err := tx.QueryRow(ctx, `SELECT column_default FROM information_schema.columns
		WHERE table_name='vendor_pks' AND column_name='renewal_days'`).Scan(&def); err != nil || def != "270" {
		t.Errorf("renewal_days default = %q, %v; want 270", def, err)
	}
}

func TestVendorPksListMigration_MenuPermissions(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()

	rows, err := tx.Query(ctx, `SELECT r.role || '|' || mf.key FROM role_permissions rp
		JOIN roles r ON r.id = rp.role_id JOIN menu_features mf ON mf.id = rp.menu_feature_id
		WHERE mf.key IN ('settings', 'settings.pks') AND r.role IN ('ADMIN','ADMIN_PARAM','ATM-SPV')`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	got, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	have := map[string]bool{}
	for _, g := range got {
		have[g] = true
	}
	for _, w := range []string{"ADMIN|settings.pks", "ADMIN_PARAM|settings.pks", "ATM-SPV|settings", "ATM-SPV|settings.pks"} {
		if !have[w] {
			t.Errorf("missing role_permission %s", w)
		}
	}

	// settings.pks must not be granted outside the seeded roles.
	orows, err := tx.Query(ctx, `SELECT DISTINCT r.role FROM role_permissions rp
		JOIN roles r ON r.id = rp.role_id JOIN menu_features mf ON mf.id = rp.menu_feature_id
		WHERE mf.key = 'settings.pks'`)
	if err != nil {
		t.Fatalf("query roles: %v", err)
	}
	roles, err := pgx.CollectRows(orows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("collect roles: %v", err)
	}
	sort.Strings(roles)
	want := []string{"ADMIN", "ADMIN_PARAM", "ATM-SPV"}
	if len(roles) != len(want) {
		t.Fatalf("settings.pks roles = %v, want %v", roles, want)
	}
	for i := range want {
		if roles[i] != want[i] {
			t.Fatalf("settings.pks roles = %v, want %v", roles, want)
		}
	}
}

// Property 1: DB function == Go reference on random input, incl. expiry = today.
func TestVendorPksRecordStatus_Property(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()
	today := time.Date(2030, 6, 15, 0, 0, 0, 0, time.UTC)

	rapid.Check(t, func(rt *rapid.T) {
		effOff := rapid.IntRange(-400, 400).Draw(rt, "effOff")
		expOff := rapid.OneOf(rapid.Just(0), rapid.IntRange(-400, 800)).Draw(rt, "expOff")
		renewal := rapid.OneOf(rapid.Just(30), rapid.Just(270), rapid.IntRange(30, 3666)).Draw(rt, "renewal")
		if rapid.Bool().Draw(rt, "nearBoundary") {
			expOff = renewal - 1 + rapid.IntRange(0, 2).Draw(rt, "jit") // around today+renewal
		}
		replaced := rapid.Bool().Draw(rt, "replaced")
		terminated := rapid.Bool().Draw(rt, "terminated")

		eff, exp := today.AddDate(0, 0, effOff), today.AddDate(0, 0, expOff)
		var repAt, termAt *time.Time
		if replaced {
			repAt = &today
		}
		if terminated {
			termAt = &today
		}

		var got string
		if err := tx.QueryRow(ctx, `SELECT vendor_pks_record_status($1::timestamptz,$2::timestamptz,$3::date,$4::date,$5::int,$6::date)`,
			repAt, termAt, eff, exp, renewal, today).Scan(&got); err != nil {
			rt.Fatalf("db: %v", err)
		}
		if want := refPksStatus(replaced, terminated, eff, exp, renewal, today); got != want {
			rt.Fatalf("status(replaced=%v term=%v eff=%+d exp=%+d renewal=%d) = %q, want %q",
				replaced, terminated, effOff, expOff, renewal, got, want)
		}
	})
}

// expiry = today is still valid (not expired); the day before is expired.
func TestVendorPksRecordStatus_ExpiryToday(t *testing.T) {
	tx := integrationTx(t)
	today := time.Date(2030, 6, 15, 0, 0, 0, 0, time.UTC)
	for off, want := range map[int]string{0: "need_renewal", -1: "expired"} {
		var got string
		if err := tx.QueryRow(context.Background(),
			`SELECT vendor_pks_record_status(NULL, NULL, $1::date, $2::date, 30, $3::date)`,
			today.AddDate(-1, 0, 0), today.AddDate(0, 0, off), today).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("expiry today%+d = %q, want %q", off, got, want)
		}
	}
}
