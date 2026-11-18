//go:build integration

package repository

import (
	"context"
	"testing"
)

// Migration 042 (region-standardization task 1.3): state after the migration —
// standard regions, region_cities, vendor_branches.region_code FK, dropped
// vendor_branches.region, backup table. Runs inside integrationTx (rolled back);
// the before/after comparisons (Property 1, 7, 9) were checked when 042 was
// applied to dev (tasks.md task 1.4). Reuses pksExec/pksRejected/pksCount from
// migration022_integration_test.go.

func TestMigration042Schema(t *testing.T) {
	tx := integrationTx(t)

	for _, c := range []struct{ table, col string }{
		{"region_cities", "code"}, {"region_cities", "name"}, {"region_cities", "region_id"},
		{"region_cities", "is_active"}, {"region_cities", "deleted_at"},
		{"migration_042_backup", "entity"}, {"migration_042_backup", "entity_key"}, {"migration_042_backup", "payload"},
	} {
		if pksCount(t, tx, `SELECT count(*) FROM information_schema.columns
			WHERE table_schema='public' AND table_name=$1 AND column_name=$2`, c.table, c.col) != 1 {
			t.Errorf("column %s.%s missing", c.table, c.col)
		}
	}
	if n := pksCount(t, tx, `SELECT count(*) FROM information_schema.columns
		WHERE table_schema='public' AND table_name='vendor_branches' AND column_name='region'`); n != 0 {
		t.Errorf("vendor_branches.region still exists (Req 4.1)")
	}
	if n := pksCount(t, tx, `SELECT count(*) FROM pg_constraint
		WHERE conname='vendor_branches_region_code_fk' AND contype='f'
		  AND confrelid='public.regions'::regclass`); n != 1 {
		t.Errorf("vendor_branches_region_code_fk missing (Req 3.1)")
	}
}

func TestMigration042Data(t *testing.T) {
	tx := integrationTx(t)

	// Standard regions (region-mapping.md A): renamed in place, old codes gone.
	for _, r := range []struct{ code, name string }{
		{"BALNUS", "Bali Nusra"}, {"INDTIM", "Indonesia Timur"}, {"JABAR", "Jawa Barat"},
		{"JATENG", "Jawa Tengah"}, {"JATIM", "Jawa Timur"}, {"JKTBAR", "Jakarta Barat"},
		{"JKTPST", "Jakarta Pusat"}, {"JKTSEL", "Jakarta Selatan"}, {"JKTTIM", "Jakarta Timur"},
		{"JKTUTR", "Jakarta Utara"}, {"KAL", "Kalimantan"}, {"SUMATERA", "Sumatera"}, {"TGR", "Tangerang"},
	} {
		if pksCount(t, tx, `SELECT count(*) FROM regions WHERE code=$1 AND region=$2 AND is_active`, r.code, r.name) != 1 {
			t.Errorf("standard region %s %q missing", r.code, r.name)
		}
	}
	if n := pksCount(t, tx, `SELECT count(*) FROM regions WHERE code = ANY($1)`,
		[]string{"BALI_NUSRA", "JKT_CENTRAL", "JKT_EAST", "JKT_NORTH", "JKT_SOUTH", "JKT_WEST", "KALIMANTAN", "TANGERANG"}); n != 0 {
		t.Errorf("%d old region codes still present (Q6 rename)", n)
	}
	// Req 1.5: every active non-test region matches the shared code format.
	if n := pksCount(t, tx, `SELECT count(*) FROM regions
		WHERE is_active AND region IS DISTINCT FROM 'Integration Test Region' AND code !~ '^[A-Z0-9]{2,10}$'`); n != 0 {
		t.Errorf("%d active regions violate ^[A-Z0-9]{2,10}$", n)
	}

	// Cities (region-mapping.md B): 22 rows, each under the mapped region.
	if n := pksCount(t, tx, `SELECT count(*) FROM region_cities`); n != 22 {
		t.Errorf("region_cities = %d rows, want 22", n)
	}
	for _, c := range []struct{ city, region string }{
		{"MDN", "SUMATERA"}, {"PLG", "SUMATERA"}, {"MKS", "INDTIM"}, {"BJM", "KAL"},
		{"SMD", "KAL"}, {"MTR", "BALNUS"}, {"KUPANG", "BALNUS"}, {"JEMBER", "JATIM"},
	} {
		if pksCount(t, tx, `SELECT count(*) FROM region_cities c JOIN regions r ON r.id = c.region_id
			WHERE c.code=$1 AND r.code=$2`, c.city, c.region) != 1 {
			t.Errorf("city %s not under region %s", c.city, c.region)
		}
	}

	// Q8: no active branch left without a region.
	if n := pksCount(t, tx, `SELECT count(*) FROM vendor_branches WHERE is_active AND region_code IS NULL`); n != 0 {
		t.Errorf("%d active vendor branches without region_code", n)
	}
	// Backup rows still point at real regions (rollback script input, Req 8.3).
	if n := pksCount(t, tx, `SELECT count(*) FROM migration_042_backup k
		WHERE k.entity='regions' AND NOT EXISTS (SELECT 1 FROM regions r WHERE r.id::text = k.entity_key)`); n != 0 {
		t.Errorf("%d backup region rows without a matching region", n)
	}
}

// Property 2: a vendor branch can only reference an existing region code.
func TestMigration042RegionCodeFK(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()

	var vendorID int64
	if err := tx.QueryRow(ctx, `INSERT INTO vendors (code, name) VALUES ('IT042V', 'IT 042 Vendor') RETURNING id`).Scan(&vendorID); err != nil {
		t.Fatalf("seed vendor: %v", err)
	}
	pksExec(t, tx, `INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, region_code)
		VALUES ($1, 'IT042_OK', 'IT 042 ok', 'JKTBAR')`, vendorID)
	pksRejected(t, tx, "unknown region_code", `INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, region_code)
		VALUES ($1, 'IT042_BAD', 'IT 042 bad', 'NOPE99')`, vendorID)
	pksRejected(t, tx, "region delete while referenced",
		`DELETE FROM regions WHERE code = 'JKTBAR'`)
}
