//go:build integration

package exportjob

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
	"github.com/cimb-niaga/cms/pkg/csvexport"
	"github.com/cimb-niaga/cms/pkg/reports"
)

// vendor-export-pks 3.4 (Property 1 DB part, Property 2) on real Postgres. The vendor master-data producer
// (vendorsProducer, query ExportVendorsWithPksBatch) is run against a fixture whose vendor codes start
// with T94. Property 2 compares against the whole vendors table (read-only, real dev rows included);
// Property 1 compares the fixture rows with the vendor_pks report query. Everything inserted is removed in
// t.Cleanup and a last step fails the test on leftovers. Skips when DATABASE_URL is empty (p11Pool).

const (
	t94Months     = 9
	t94VendorCols = 7  // 7 vendor profile columns
	t94PksCols    = 11 // vendor_code, vendor_name + 9 director/PKS columns
)

// 2026-07-01 08:00 WIB.
var t94AsOf = time.Date(2026, 7, 1, 8, 0, 0, 0, reports.WIB)

type t94Fix struct {
	t      *testing.T
	pool   *pgxpool.Pool
	q      *repository.Queries
	userID int64
	ids    map[string]int64 // code -> id
}

func (f *t94Fix) id(sql string, args ...any) int64 {
	f.t.Helper()
	var id int64
	if err := f.pool.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		f.t.Fatalf("t94 insert: %v\n%s", err, sql)
	}
	return id
}

func (f *t94Fix) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.pool.Exec(context.Background(), sql, args...); err != nil {
		f.t.Fatalf("t94 exec: %v\n%s", err, sql)
	}
}

func (f *t94Fix) cleanup() {
	for _, s := range []string{
		`DELETE FROM vendor_pks WHERE vendor_id IN (SELECT id FROM vendors WHERE code LIKE 'T94%')`,
		`DELETE FROM vendor_pics WHERE vendor_id IN (SELECT id FROM vendors WHERE code LIKE 'T94%')`,
		`DELETE FROM vendors WHERE code LIKE 'T94%'`,
		`DELETE FROM users WHERE username LIKE 'T94%'`, // last: vendor_pks.created_by
	} {
		if _, err := f.pool.Exec(context.Background(), s); err != nil {
			f.t.Errorf("t94 cleanup: %v\n%s", err, s)
		}
	}
}

func (f *t94Fix) assertNoLeftovers() {
	for _, q := range []string{
		`SELECT count(*) FROM vendors WHERE code LIKE 'T94%'`,
		`SELECT count(*) FROM vendor_pks WHERE pks_name LIKE 'T94%'`,
		`SELECT count(*) FROM vendor_pics WHERE name LIKE 'T94%'`,
		`SELECT count(*) FROM users WHERE username LIKE 'T94%'`,
	} {
		var n int
		if err := f.pool.QueryRow(context.Background(), q).Scan(&n); err != nil {
			f.t.Errorf("t94 leftover query: %v", err)
		} else if n != 0 {
			f.t.Errorf("t94 leftover rows: %d from %s", n, q)
		}
	}
}

func newT94Fix(t *testing.T) *t94Fix {
	t.Helper()
	f := &t94Fix{t: t, pool: p11Pool(t), ids: map[string]int64{}}
	f.q = repository.New(f.pool)
	t.Cleanup(f.assertNoLeftovers)
	t.Cleanup(f.cleanup)
	f.cleanup() // stale rows of a crashed earlier run
	f.userID = f.id(`INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source)
		SELECT id, 'T94_admin', 'T94_admin', 'T94_admin@test.local', true, 'ldap' FROM roles WHERE role = 'ADMIN' RETURNING id`)
	f.seed()
	return f
}

// seed: A = FLM with active PKS + replaced PKS + director; B = FLM without PKS and director;
// C = INTERNAL with director; D = disabled FLM with active PKS; E = FLM with terminated PKS, director with
// a soft-deleted twin and a regular PIC next to it.
func (f *t94Fix) seed() {
	vendor := func(code, kind string, disabled bool) int64 {
		var deleted any
		if disabled {
			deleted = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		id := f.id(`INSERT INTO vendors (code, name, kind, legal_name, npwp, contact_email, contact_phone, hq_address, is_active, deleted_at)
			VALUES ($1, $2, $3, 'PT ' || $1, '012345678901234', $1 || '@v.test', '021', 'Jl. =Alamat, "1"', $4, $5::timestamptz) RETURNING id`,
			code, "T94 Vendor "+code+" Ü", kind, !disabled, deleted)
		f.ids[code] = id
		return id
	}
	pks := func(vid int64, name, eff, exp string, replaced, terminated bool) {
		var repl, term, by, reason any
		if replaced {
			repl = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		if terminated {
			term, by, reason = time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), f.userID, "T94 reason"
		}
		f.exec(`INSERT INTO vendor_pks (vendor_id, pks_name, effective_date, expiry_date, created_by, replaced_at, terminated_at, terminated_by, termination_reason)
			VALUES ($1, $2, $3::date, $4::date, $5, $6::timestamptz, $7::timestamptz, $8, $9)`,
			vid, name, eff, exp, f.userID, repl, term, by, reason)
	}
	pic := func(vid int64, name string, director, deleted bool) {
		var del any
		if deleted {
			del = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		f.exec(`INSERT INTO vendor_pics (vendor_id, name, phone, email, is_director, is_notification_recipient, roles, deleted_at)
			VALUES ($1, $2, '0812', $2 || '@p.test', $3, $4, '{}', $5::timestamptz)`, vid, name, director, !director, del)
	}

	a := vendor("T94A", "FLM_VENDOR", false)
	pks(a, "T94 PKS lama", "2022-01-01", "2023-12-31", true, false)
	pks(a, "T94 PKS =aktif", "2025-01-01", "2027-12-31", false, false)
	pic(a, "T94 Direktur A", true, false)

	vendor("T94B", "FLM_VENDOR", false)

	c := vendor("T94C", "INTERNAL", false)
	pic(c, "T94 Direktur C", true, false)

	d := vendor("T94D", "FLM_VENDOR", true)
	pks(d, "T94 PKS D", "2025-07-01", "2026-09-30", false, false) // need renewal

	e := vendor("T94E", "FLM_VENDOR", false)
	pks(e, "T94 PKS E", "2025-01-01", "2028-12-31", false, true)
	pic(e, "T94 deleted director", true, true)
	pic(e, "T94 Direktur E", true, false)
	pic(e, "T94 contact E", false, false)
}

func (f *t94Fix) produce(p Params) [][]string {
	f.t.Helper()
	pr := NewProducers(nil, t94Months)[KindMasterDataGlobal][csvexport.Vendors]
	if p.VendorID != nil {
		pr = NewProducers(nil, t94Months)[KindVendor]["vendor"]
	}
	var rows [][]string
	err := pr.Produce(context.Background(), f.q, p, t94AsOf, func(c []string) error {
		rows = append(rows, slices.Clone(c))
		return nil
	})
	if err != nil {
		f.t.Fatalf("produce: %v", err)
	}
	return rows
}

func (f *t94Fix) producePks(p Params) [][]string {
	f.t.Helper()
	pr := NewProducers(nil, t94Months)[KindMasterDataGlobal][csvexport.VendorPks]
	if p.VendorID != nil {
		pr = NewProducers(nil, t94Months)[KindVendor][csvexport.VendorPks]
	}
	var rows [][]string
	err := pr.Produce(context.Background(), f.q, p, t94AsOf, func(c []string) error {
		rows = append(rows, slices.Clone(c))
		return nil
	})
	if err != nil {
		f.t.Fatalf("producePks: %v", err)
	}
	return rows
}

// oldVendors reads ExportVendorsBatch (the importer's comparison query) in one page.
func (f *t94Fix) oldVendors(vendorID *int64, status string) []repository.ExportVendorsBatchRow {
	f.t.Helper()
	rows, err := f.q.ExportVendorsBatch(context.Background(), repository.ExportVendorsBatchParams{
		VendorID: vendorID, Status: status, BatchSize: 1 << 30})
	if err != nil {
		f.t.Fatalf("ExportVendorsBatch: %v", err)
	}
	return rows
}

func t94OldCells(r repository.ExportVendorsBatchRow) []string {
	return csvexport.VendorCells(csvexport.VendorRow{Name: r.Name, LegalName: r.LegalName,
		Npwp: r.Npwp, ContactEmail: r.ContactEmail, ContactPhone: r.ContactPhone, HqAddress: r.HqAddress, IsActive: r.IsActive})
}

// Property 2: the seven vendor profile cells equal csvexport.VendorCells of ExportVendorsBatch, same order,
// one line per vendor.
func TestVendorExportPks_ImportColumnsMatchOldQuery(t *testing.T) {
	f := newT94Fix(t)
	one := f.ids["T94A"]
	cases := []struct {
		name   string
		vendor *int64
		status string
	}{
		{"active", nil, "active"}, {"disabled", nil, "disabled"}, {"all", nil, "all"},
		{"scope A", &one, "all"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := *f
			f.t = t
			old := f.oldVendors(c.vendor, c.status)
			got := f.produce(Params{VendorID: c.vendor, StatusFilter: c.status})
			if len(got) != len(old) {
				t.Fatalf("rows = %d, want %d (one line per vendor)", len(got), len(old))
			}
			for i, r := range old {
				if len(got[i]) != t94VendorCols {
					t.Fatalf("row %d has %d cells, want %d", i, len(got[i]), t94VendorCols)
				}
				if want := t94OldCells(r); !slices.Equal(got[i], want) {
					t.Fatalf("row %d (id %d) import cells\ngot:  %q\nwant: %q", i, r.ID, got[i], want)
				}
			}
		})
	}
}

// Property 1 (DB part) + the fixture rules: vendor-pks cells (after CSVSafe) equal cells 3-11 of the
// vendor_pks report for the same vendor, As_Of_Date and renewal months.
func TestVendorExportPks_ReadOnlyColumnsMatchPksReport(t *testing.T) {
	f := newT94Fix(t)
	ctx := context.Background()
	q := "T94"
	rep, err := f.q.ExportVendorPksBatch(ctx, repository.ExportVendorPksBatchParams{
		Today: wibDate(reports.TodayWIB(t94AsOf)), RenewalMonths: t94Months, Q: &q, Status: "all", BatchSize: 1000})
	if err != nil {
		t.Fatalf("ExportVendorPksBatch: %v", err)
	}
	want := map[string][]string{} // vendor code -> report cells 3..11
	for _, r := range rep {
		want[r.VendorCode] = reports.VendorPksCells(vendorPksRow(r))[2:]
	}
	if len(want) != len(f.ids) {
		t.Fatalf("report rows = %d, want %d", len(want), len(f.ids))
	}

	byCode := map[string][]string{}
	for _, row := range f.producePks(Params{StatusFilter: "all"}) {
		if len(row) != t94PksCols {
			t.Fatalf("row has %d cells, want %d", len(row), t94PksCols)
		}
		code := row[0] // cell 0 is vendor_code
		if _, ok := f.ids[code]; !ok {
			continue // real dev vendor
		}
		if _, dup := byCode[code]; dup {
			t.Fatalf("vendor %s exported more than once (replaced PKS must not show)", code)
		}
		ro := make([]string, 0, 9)
		for _, c := range row[2:] {
			ro = append(ro, csvexport.CSVSafe(c))
		}
		byCode[code] = ro
	}
	for code := range f.ids {
		got, ok := byCode[code]
		if !ok {
			t.Fatalf("vendor %s missing from export", code)
		}
		if !slices.Equal(got, want[code]) {
			t.Errorf("%s read-only cells\ngot:  %q\nwant: %q", code, got, want[code])
		}
	}

	// Literal checks, independent of the report query.
	if r := byCode["T94A"]; r[0] != "T94 Direktur A" || r[3] != "T94 PKS =aktif" || r[5] != "2027-12-31" || r[8] != "Aktif" {
		t.Errorf("T94A (old replaced PKS leaked?): %q", r)
	}
	if r := byCode["T94B"]; r[8] != "Belum diisi" || r[0] != "" || r[3] != "" {
		t.Errorf("FLM without PKS/director: %q", r)
	}
	if r := byCode["T94C"]; r[8] != "" || r[3] != "" || r[0] != "T94 Direktur C" {
		t.Errorf("INTERNAL with director: %q", r)
	}
	if r := byCode["T94D"]; r[8] != "Need Renewal" {
		t.Errorf("disabled vendor with PKS: status %q", r[8])
	}
	if r := byCode["T94E"]; r[8] != "Terminated" || r[0] != "T94 Direktur E" {
		t.Errorf("T94E: %q", r)
	}
}
