//go:build integration

package exportjob

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/backend-exq/internal/audit"
	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
	"github.com/cimb-niaga/cms/pkg/csvexport"
	"github.com/cimb-niaga/cms/pkg/reports"
)

// vendor-pks-list 6.3 on real Postgres: the Daftar PKS export (one line per vendor_pks row, history included,
// filter {status, pks_type, vendor, q}). The producer runs through the real worker path (Service.Create ->
// worker.produce: REPEATABLE READ READ ONLY snapshot through pgReader, ChunkWriter, csvexport.Writer).
// Every vendor the test inserts has a code starting with T93 and every request filters `q` on "T93", so real
// dev rows never reach the output. All rows are removed in t.Cleanup; a last step fails on leftovers.
// Skips when DATABASE_URL is empty (p11Pool).

// t93Header is the literal 13-column header (written out, not read from pkg/reports).
var t93Header = []string{"Kode Vendor", "Nama Perusahaan", "Nama Direktur Utama", "Telepon Direktur Utama",
	"Email Direktur Utama", "Type PKS", "No. WOD", "Nama PKS", "Tanggal Efektif PKS", "Tanggal Expired PKS",
	"Masa Berlaku (hari)", "Reminder (hari)", "Status PKS"}

var t93Types = []string{"CIT", "FLM"}

var t93Labels = map[string]string{"aktif": "Aktif", "need_renewal": "Need Renewal", "belum_berlaku": "Belum Berlaku",
	"expired": "Expired", "terminated": "Terminated", "diganti": "Diganti"}

var t93StatusKeys = []string{"terkini", "semua", "aktif", "need_renewal", "belum_berlaku", "expired", "terminated", "diganti"}

const t93Day = "2006-01-02"

// ---- reference model (vendor_pks_record_status, Req 3.1-3.4) ----

type t93Pks struct {
	typ, name            string
	eff, exp             time.Time
	renewal              int
	replaced, terminated bool
}

func (p t93Pks) status(today time.Time) string {
	switch {
	case p.replaced:
		return "diganti"
	case p.terminated:
		return "terminated"
	case p.eff.After(today):
		return "belum_berlaku"
	case p.exp.Before(today):
		return "expired"
	case p.exp.Before(today.AddDate(0, 0, p.renewal)):
		return "need_renewal"
	}
	return "aktif"
}

type t93Dir struct {
	name         string
	phone, email *string
	deletedExtra bool // a soft-deleted second director row exists: it must never show up
}

type t93Vendor struct {
	idx      int
	kind     string
	disabled bool
	pks      []t93Pks
	dir      *t93Dir
}

func (v t93Vendor) code() string { return fmt.Sprintf("T93V%02d", v.idx) }
func (v t93Vendor) name() string { return fmt.Sprintf("T93 Vendor %02d Ü", v.idx) }
func (v t93Vendor) wod(k int) string {
	return fmt.Sprintf("T93-WOD-%02d-%d", v.idx, k)
}

type t93Filter struct{ status, pksType, vendor, q string }

func (f t93Filter) values() url.Values {
	v := url.Values{"status": {f.status}, "q": {f.q}}
	if f.pksType != "" {
		v.Set("pks_type", f.pksType)
	}
	if f.vendor != "" {
		v.Set("vendor", f.vendor)
	}
	return v
}

// t93Expected is the model of the file: FLM vendors only, one line per vendor_pks row that passes the filter,
// ordered by vendor code, type code, effective date desc, id asc (the id follows the insert order k).
func t93Expected(vs []t93Vendor, f t93Filter, today time.Time) [][]string {
	var out [][]string
	for _, v := range vs {
		if v.kind != "FLM_VENDOR" || (f.vendor != "" && f.vendor != v.code()) {
			continue
		}
		type ent struct {
			k int
			p t93Pks
		}
		var ents []ent
		for k, p := range v.pks {
			st := p.status(today)
			switch {
			case f.status == "semua":
			case f.status == "terkini" && !p.replaced:
			case f.status == st:
			default:
				continue
			}
			if f.pksType != "" && f.pksType != p.typ {
				continue
			}
			hay := strings.ToLower(p.name + "\x00" + v.wod(k))
			if !strings.Contains(hay, strings.ToLower(f.q)) {
				continue
			}
			ents = append(ents, ent{k, p})
		}
		slices.SortFunc(ents, func(a, b ent) int {
			if c := strings.Compare(a.p.typ, b.p.typ); c != 0 {
				return c
			}
			if c := b.p.eff.Compare(a.p.eff); c != 0 {
				return c
			}
			return a.k - b.k // keyset ties break on id ASC (insert order)
		})
		for _, e := range ents {
			out = append(out, t93Cells(v, e.k, e.p, today))
		}
	}
	return out
}

func t93Cells(v t93Vendor, k int, p t93Pks, today time.Time) []string {
	row := make([]string, len(t93Header))
	row[0], row[1] = csvexport.CSVSafe(v.code()), csvexport.CSVSafe(v.name())
	if d := v.dir; d != nil {
		row[2] = csvexport.CSVSafe(d.name)
		if d.phone != nil {
			row[3] = csvexport.CSVSafe(*d.phone)
		}
		if d.email != nil {
			row[4] = csvexport.CSVSafe(*d.email)
		}
	}
	row[5], row[6], row[7] = p.typ, csvexport.CSVSafe(v.wod(k)), csvexport.CSVSafe(p.name)
	row[8], row[9] = p.eff.Format(t93Day), p.exp.Format(t93Day)
	row[10] = strconv.Itoa(int(p.exp.Sub(p.eff).Hours() / 24))
	row[11] = strconv.Itoa(p.renewal)
	row[12] = t93Labels[p.status(today)]
	return row
}

// ---- fixture ----

type t93Fix struct {
	t        *testing.T
	pool     *pgxpool.Pool
	svc      *Service
	userID   int64
	otherUID int64
}

func (f *t93Fix) id(sql string, args ...any) int64 {
	f.t.Helper()
	var id int64
	if err := f.pool.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		f.t.Fatalf("t93 insert: %v\n%s", err, sql)
	}
	return id
}

func (f *t93Fix) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.pool.Exec(context.Background(), sql, args...); err != nil {
		f.t.Fatalf("t93 exec: %v\n%s", err, sql)
	}
}

func (f *t93Fix) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		f.t.Fatalf("t93 count: %v\n%s", err, sql)
	}
	return n
}

const t93Users = `(SELECT id FROM users WHERE username LIKE 'T93%')`

func (f *t93Fix) clearVendors() {
	ctx := context.Background()
	for _, s := range []string{
		`DELETE FROM vendor_pks WHERE vendor_id IN (SELECT id FROM vendors WHERE code LIKE 'T93%')`,
		`DELETE FROM vendor_pics WHERE vendor_id IN (SELECT id FROM vendors WHERE code LIKE 'T93%')`,
		`DELETE FROM vendors WHERE code LIKE 'T93%'`,
	} {
		if _, err := f.pool.Exec(ctx, s); err != nil {
			f.t.Errorf("t93 clear vendors: %v\n%s", err, s)
		}
	}
}

func (f *t93Fix) cleanup() {
	ctx := context.Background()
	for _, s := range []string{
		`DELETE FROM export_job_chunks WHERE export_job_id IN (SELECT id FROM export_jobs WHERE requested_by IN ` + t93Users + `)`,
		`DELETE FROM export_jobs WHERE requested_by IN ` + t93Users,
		`DELETE FROM audit_logs WHERE actor_id IN ` + t93Users,
	} {
		if _, err := f.pool.Exec(ctx, s); err != nil {
			f.t.Errorf("t93 cleanup: %v\n%s", err, s)
		}
	}
	f.clearVendors()
	if _, err := f.pool.Exec(ctx, `DELETE FROM users WHERE username LIKE 'T93%'`); err != nil { // last: vendor_pks.created_by
		f.t.Errorf("t93 cleanup users: %v", err)
	}
}

func (f *t93Fix) assertNoLeftovers() {
	for _, q := range []string{
		`SELECT count(*) FROM vendors WHERE code LIKE 'T93%'`,
		`SELECT count(*) FROM vendor_pks WHERE wod_number LIKE 'T93%'`,
		`SELECT count(*) FROM vendor_pics WHERE name LIKE 'T93%'`,
		`SELECT count(*) FROM users WHERE username LIKE 'T93%'`,
		`SELECT count(*) FROM export_jobs WHERE requested_by = ` + fmt.Sprint(f.userID),
		`SELECT count(*) FROM audit_logs WHERE actor_id = ` + fmt.Sprint(f.userID),
	} {
		if n := f.count(q); n != 0 {
			f.t.Errorf("t93 leftover rows: %d from %s", n, q)
		}
	}
}

func newT93Fix(t *testing.T) *t93Fix {
	t.Helper()
	f := &t93Fix{t: t, pool: p11Pool(t)} // pool.Close is registered first, so it runs last
	f.svc = NewService(f.pool, audit.NewWriter(), nil)
	t.Cleanup(f.assertNoLeftovers)
	t.Cleanup(f.cleanup)
	f.cleanup() // stale rows of a crashed earlier run
	user := func(name, role string) int64 {
		return f.id(`INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source)
			SELECT id, $1, $1, $1 || '@test.local', true, 'ldap' FROM roles WHERE role = $2 RETURNING id`, name, role)
	}
	f.userID = user("T93_admin", "ADMIN")
	f.otherUID = user("T93_spv", "ATM-SPV")
	return f
}

func (f *t93Fix) insertVendor(v t93Vendor) {
	f.t.Helper()
	var deleted any
	if v.disabled {
		deleted = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	vid := f.id(`INSERT INTO vendors (code, name, kind, is_active, deleted_at) VALUES ($1, $2, $3, $4, $5::timestamptz) RETURNING id`,
		v.code(), v.name(), v.kind, !v.disabled, deleted)
	for k, p := range v.pks { // insert order = id order
		var repl, term, by, reason any
		if p.replaced {
			repl = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		if p.terminated {
			term, by, reason = time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), f.userID, "T93 reason"
		}
		f.exec(`INSERT INTO vendor_pks (vendor_id, pks_type_id, wod_number, pks_name, effective_date, expiry_date, created_by,
				renewal_days, replaced_at, terminated_at, terminated_by, termination_reason)
			VALUES ($1, (SELECT id FROM vendor_pks_types WHERE code = $2), $3, $4, $5::date, $6::date, $7,
				$8, $9::timestamptz, $10::timestamptz, $11, $12)`,
			vid, p.typ, v.wod(k), p.name, p.eff, p.exp, f.userID, p.renewal, repl, term, by, reason)
	}
	if d := v.dir; d != nil {
		pic := func(name string, phone, email *string, deleted bool) {
			var del any
			if deleted {
				del = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			}
			f.exec(`INSERT INTO vendor_pics (vendor_id, name, phone, email, is_director, is_notification_recipient, roles, deleted_at)
				VALUES ($1, $2, $3, $4, true, false, '{}', $5::timestamptz)`, vid, name, phone, email, del)
		}
		if d.deletedExtra {
			pic("T93 deleted director", sptr("0000"), sptr("x@x"), true)
		}
		pic(d.name, d.phone, d.email, false)
	}
	// A regular PIC never leaks into the director columns.
	f.exec(`INSERT INTO vendor_pics (vendor_id, name, phone, email, is_director, is_notification_recipient, roles)
		VALUES ($1, 'T93 contact', '999', 'c@c', false, true, '{replenish}')`, vid)
}

func sptr(s string) *string { return &s }

// run creates the job through the service (as the ADMIN user), produces it with the real worker whose
// clock is asOf, and returns the parsed CSV (header + data records) and the raw file.
func (f *t93Fix) run(filter t93Filter, asOf time.Time) (records [][]string, raw string) {
	f.t.Helper()
	ctx := context.Background()
	p, err := ParseReportParams(KindVendorPks, filter.values(), time.Now())
	if err != nil {
		f.t.Fatalf("ParseReportParams(%v): %v", filter, err)
	}
	job, _, err := f.svc.Create(ctx, Actor{UserID: f.userID, Username: "T93_admin", Role: "ADMIN"}, p)
	if err != nil {
		f.t.Fatalf("Create: %v", err)
	}
	defer func() {
		f.exec(`DELETE FROM export_job_chunks WHERE export_job_id = $1`, job.ID)
		f.exec(`DELETE FROM export_jobs WHERE id = $1`, job.ID)
	}()
	f.exec(`UPDATE export_jobs SET status = 'running', started_at = now() WHERE id = $1`, job.ID)
	w := newWorker(pgStore{pool: f.pool, q: repository.New(f.pool)}, pgReader{pool: f.pool}, NewProducers(nil), WorkerConfig{})
	w.now = func() time.Time { return asOf }
	st, err := w.produce(ctx, job)
	if err != nil {
		f.t.Fatalf("produce: %v", err)
	}
	data, _ := p11ReadAll(f.t, f.svc, job.ID, int64(st.chunks))
	if !bytes.HasPrefix(data, []byte(bom)) {
		f.t.Fatalf("file has no UTF-8 BOM: %q", data[:min(len(data), 8)])
	}
	records, err = csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte(bom)))).ReadAll()
	if err != nil {
		f.t.Fatalf("parse CSV: %v\n%s", err, data)
	}
	if len(records) == 0 {
		f.t.Fatalf("empty file")
	}
	return records, string(data)
}

// ---- random data ----

// drawT93Vendors draws 0-6 vendors with 0-4 PKS rows each. At most one non-replaced row per (vendor, type)
// (vendor_pks_active_uq); expiries cluster around today and the renewal boundary.
func drawT93Vendors(rt *rapid.T, today time.Time) []t93Vendor {
	n := rapid.IntRange(0, 6).Draw(rt, "vendors")
	vs := make([]t93Vendor, 0, n)
	for i := 1; i <= n; i++ {
		v := t93Vendor{idx: i,
			kind:     rapid.SampledFrom([]string{"FLM_VENDOR", "FLM_VENDOR", "INTERNAL"}).Draw(rt, "kind"),
			disabled: rapid.IntRange(0, 3).Draw(rt, "disabled") == 0,
		}
		activeUsed := map[string]bool{}
		for k := range rapid.IntRange(0, 4).Draw(rt, "pks_count") {
			typ := rapid.SampledFrom(t93Types).Draw(rt, "typ")
			replaced := rapid.Bool().Draw(rt, "replaced")
			if !replaced {
				if activeUsed[typ] {
					replaced = true
				}
				activeUsed[typ] = true
			}
			term := rapid.IntRange(30, 1200).Draw(rt, "term_days")
			renewal := rapid.IntRange(30, term).Draw(rt, "renewal")
			var exp time.Time
			switch rapid.IntRange(0, 2).Draw(rt, "expiry_mode") {
			case 0:
				exp = today.AddDate(0, 0, rapid.IntRange(-400, 700).Draw(rt, "exp_off"))
			case 1:
				exp = today.AddDate(0, 0, rapid.IntRange(-2, 2).Draw(rt, "exp_today"))
			default: // around the renewal boundary
				exp = today.AddDate(0, 0, renewal+rapid.IntRange(-2, 2).Draw(rt, "exp_renew"))
			}
			v.pks = append(v.pks, t93Pks{typ: typ, name: fmt.Sprintf("T93 PKS %d-%d %s =x", i, k, typ),
				eff: exp.AddDate(0, 0, -term), exp: exp, renewal: renewal, replaced: replaced,
				terminated: rapid.IntRange(0, 3).Draw(rt, "terminated") == 0})
		}
		if rapid.Bool().Draw(rt, "has_director") {
			d := &t93Dir{name: fmt.Sprintf("T93 Direktur %d", i), deletedExtra: rapid.Bool().Draw(rt, "deleted_dir")}
			if rapid.Bool().Draw(rt, "has_phone") {
				d.phone = sptr(fmt.Sprintf("+62%d", 800+i))
			}
			if rapid.Bool().Draw(rt, "has_email") {
				d.email = sptr(fmt.Sprintf("d%d@t93.test", i))
			}
			v.dir = d
		}
		vs = append(vs, v)
	}
	return vs
}

func drawT93Filter(rt *rapid.T, vs []t93Vendor) t93Filter {
	f := t93Filter{
		status: rapid.SampledFrom(t93StatusKeys).Draw(rt, "status"),
		q:      rapid.SampledFrom([]string{"T93", "t93 pks", "T93 PKS 1-", "-WOD-02", "=x"}).Draw(rt, "q"),
	}
	if rapid.Bool().Draw(rt, "filter_type") {
		f.pksType = rapid.SampledFrom(t93Types).Draw(rt, "pks_type")
	}
	if len(vs) > 0 && rapid.Bool().Draw(rt, "filter_vendor") {
		f.vendor = rapid.SampledFrom(vs).Draw(rt, "vendor").code()
	}
	return f
}

func drawT93AsOf(rt *rapid.T) time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, reports.WIB).AddDate(0, 0, rapid.IntRange(0, 1000).Draw(rt, "day")).
		Add(time.Duration(rapid.IntRange(0, 23*60+59).Draw(rt, "minute_of_day")) * time.Minute)
}

// ---- tests ----

// Export columns: for random vendors x PKS rows and random filters, the exported lines equal the reference
// model (columns, labels, CSVSafe, order, history, FLM only, director cells).
func TestVendorPksExportMatchesModel(t *testing.T) {
	f := newT93Fix(t)
	rapid.Check(t, func(rt *rapid.T) {
		f.clearVendors()
		defer f.clearVendors()
		asOf := drawT93AsOf(rt)
		today := reports.TodayWIB(asOf)
		vs := drawT93Vendors(rt, today)
		for _, v := range vs {
			f.insertVendor(v)
		}
		filter := drawT93Filter(rt, vs)

		records, _ := f.run(filter, asOf)
		if !slices.Equal(records[0], t93Header) {
			rt.Fatalf("header = %q, want %q", records[0], t93Header)
		}
		want := t93Expected(vs, filter, today)
		got := records[1:]
		if len(got) != len(want) {
			rt.Fatalf("filter=%+v today=%s: %d rows, want %d\ngot:  %q\nwant: %q", filter, today.Format(t93Day), len(got), len(want), got, want)
		}
		for i := range want {
			if !slices.Equal(got[i], want[i]) {
				rt.Fatalf("filter=%+v today=%s row %d\ngot:  %q\nwant: %q", filter, today.Format(t93Day), i, got[i], want[i])
			}
		}
	})
}

// t93ListRef is the reference for Property 6: the SELECT/WHERE of backend ListPksAdmin
// (backend/queries/vendor_pks_list.sql) copied minus paging and ordering. backend-exq cannot import backend,
// so the same SQL is run directly on the pool.
const t93ListRef = `
SELECT p.wod_number,
       vendor_pks_record_status(p.replaced_at, p.terminated_at, p.effective_date, p.expiry_date,
                                p.renewal_days, $1::date)::text AS status,
       (p.expiry_date - $1::date)::int AS remaining_days
FROM vendor_pks p
JOIN vendors v ON v.id = p.vendor_id
JOIN vendor_pks_types t ON t.id = p.pks_type_id
WHERE v.kind = 'FLM_VENDOR'
  AND ($2::text = 'semua'
        OR ($2::text = 'terkini' AND p.replaced_at IS NULL)
        OR vendor_pks_record_status(p.replaced_at, p.terminated_at, p.effective_date, p.expiry_date,
                                    p.renewal_days, $1::date) = $2::text)
  AND ($3::text IS NULL OR t.code = $3::text)
  AND ($4::text IS NULL OR v.code = $4::text)
  AND ($5::text IS NULL
        OR p.pks_name ILIKE '%' || $5::text || '%'
        OR p.wod_number ILIKE '%' || $5::text || '%')`

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Property 6: for random data and filter, the set of (PKS, status, remaining days) in the export equals what
// the Daftar PKS list query returns for the same filter. The export has no remaining-days column, so it is
// derived from the Tanggal Expired cell and the job's WIB day; the PKS identity is the unique WOD number.
func TestVendorPksExportEqualsListQuery(t *testing.T) {
	f := newT93Fix(t)
	rapid.Check(t, func(rt *rapid.T) {
		f.clearVendors()
		defer f.clearVendors()
		asOf := drawT93AsOf(rt)
		today := reports.TodayWIB(asOf)
		vs := drawT93Vendors(rt, today)
		for _, v := range vs {
			f.insertVendor(v)
		}
		filter := drawT93Filter(rt, vs)

		rows, err := f.pool.Query(context.Background(), t93ListRef, today, filter.status,
			nilIfEmpty(filter.pksType), nilIfEmpty(filter.vendor), nilIfEmpty(filter.q))
		if err != nil {
			rt.Fatalf("reference query: %v", err)
		}
		want := map[string]string{}
		for rows.Next() {
			var wod, st string
			var rem int
			if err := rows.Scan(&wod, &st, &rem); err != nil {
				rt.Fatalf("scan: %v", err)
			}
			want[wod] = st + "|" + strconv.Itoa(rem)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			rt.Fatal(err)
		}

		records, _ := f.run(filter, asOf)
		got := map[string]string{}
		for _, r := range records[1:] {
			exp, err := time.Parse(t93Day, r[9])
			if err != nil {
				rt.Fatalf("expiry cell %q: %v", r[9], err)
			}
			key := ""
			for k, l := range t93Labels {
				if l == r[12] {
					key = k
				}
			}
			if _, dup := got[r[6]]; dup {
				rt.Fatalf("WOD %q exported twice", r[6])
			}
			got[r[6]] = key + "|" + strconv.Itoa(int(exp.Sub(today).Hours()/24))
		}
		if len(got) != len(want) {
			rt.Fatalf("filter=%+v today=%s: export %d rows, list %d\nexport: %v\nlist:   %v", filter, today.Format(t93Day), len(got), len(want), got, want)
		}
		for wod, w := range want {
			if got[wod] != w {
				rt.Fatalf("filter=%+v WOD %s: export %q, list %q", filter, wod, got[wod], w)
			}
		}
	})
}

// Fixed cases: "terkini" skips replaced rows and "semua" keeps them (newest effective first); INTERNAL vendors
// are absent; CSVSafe neutralises a formula-looking text; director cells are empty without a director; the WIB
// day (Req 10.2) decides the status; the reminder cell is renewal_days.
func TestVendorPksExportTerkiniSemuaAndCells(t *testing.T) {
	f := newT93Fix(t)
	// 2026-06-30 18:00 UTC is 2026-07-01 01:00 WIB: "today" is July 1, not June 30.
	asOf := time.Date(2026, 6, 30, 18, 0, 0, 0, time.UTC)
	today := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	pk := func(typ, name string, effYears, expDays int, replaced bool) t93Pks {
		return t93Pks{typ: typ, name: name, eff: today.AddDate(effYears, 0, 0), exp: today.AddDate(0, 0, expDays),
			renewal: 30, replaced: replaced}
	}
	formula := "=T93 Direktur"
	vs := []t93Vendor{
		{idx: 1, kind: "FLM_VENDOR", dir: &t93Dir{name: formula, phone: sptr("0812")}, pks: []t93Pks{
			pk("FLM", "T93 lama", -3, -400, true), pk("FLM", "T93 baru", -1, 400, false), pk("CIT", "T93 cit", -1, 0, false)}},
		{idx: 2, kind: "FLM_VENDOR", pks: []t93Pks{pk("FLM", "T93 expired", -2, -1, false)}},
		{idx: 3, kind: "INTERNAL", pks: []t93Pks{pk("FLM", "T93 internal", -1, 400, false)}},
	}
	for _, v := range vs {
		f.insertVendor(v)
	}

	records, raw := f.run(t93Filter{status: "terkini", q: "T93"}, asOf)
	if len(records)-1 != 3 {
		t.Fatalf("terkini rows = %d, want 3 (replaced and INTERNAL absent)\n%s", len(records)-1, raw)
	}
	// T93V01 CIT: expires today (WIB) -> still valid, inside the 30-day reminder window.
	r := records[1]
	if r[0] != "T93V01" || r[5] != "CIT" || r[10] != "365" || r[11] != "30" || r[12] != "Need Renewal" || r[3] != "0812" || r[4] != "" {
		t.Errorf("CIT row: %q", r)
	}
	if r[2] == formula || r[2] != csvexport.CSVSafe(formula) {
		t.Errorf("director cell %q must be CSVSafe(%q) = %q", r[2], formula, csvexport.CSVSafe(formula))
	}
	if r := records[2]; r[0] != "T93V01" || r[5] != "FLM" || r[7] != "T93 baru" || r[12] != "Aktif" {
		t.Errorf("FLM current row: %q", r)
	}
	if r := records[3]; r[0] != "T93V02" || r[12] != "Expired" || r[2] != "" || r[3] != "" || r[4] != "" {
		t.Errorf("expired yesterday (WIB day), no director: %q", r)
	}

	records, _ = f.run(t93Filter{status: "semua", q: "T93"}, asOf)
	if len(records)-1 != 4 {
		t.Fatalf("semua rows = %d, want 4", len(records)-1)
	}
	if records[2][7] != "T93 baru" || records[3][7] != "T93 lama" || records[3][12] != "Diganti" {
		t.Errorf("semua keeps history, newest effective first: %q", records[2:4])
	}

	records, _ = f.run(t93Filter{status: "diganti", q: "T93"}, asOf)
	if len(records)-1 != 1 || records[1][12] != "Diganti" {
		t.Errorf("status=diganti: %q", records)
	}
}

// Keyset paging over more than one page: no row lost or doubled (vendor code order).
func TestVendorPksExportKeysetPaging(t *testing.T) {
	f := newT93Fix(t)
	total := PageSize + 37
	f.exec(`INSERT INTO vendors (code, name, kind, is_active)
		SELECT 'T93P' || lpad(i::text, 5, '0'), 'T93 Page ' || lpad(i::text, 5, '0'), 'FLM_VENDOR', true
		FROM generate_series(1, $1::int) i`, total)
	f.exec(`INSERT INTO vendor_pks (vendor_id, pks_type_id, wod_number, pks_name, effective_date, expiry_date, created_by)
		SELECT v.id, (SELECT id FROM vendor_pks_types WHERE code = 'FLM'), 'T93-' || v.code, 'T93 pks ' || v.code,
		       DATE '2025-01-01', DATE '2030-01-01', $1
		FROM vendors v WHERE v.code LIKE 'T93P%'`, f.userID)
	records, _ := f.run(t93Filter{status: "terkini", q: "T93"}, time.Date(2026, 7, 1, 8, 0, 0, 0, reports.WIB))
	got := records[1:]
	if len(got) != total {
		t.Fatalf("rows = %d, want %d", len(got), total)
	}
	for i, r := range got {
		if want := fmt.Sprintf("T93P%05d", i+1); r[0] != want {
			t.Fatalf("row %d code = %s, want %s (lost, doubled or misordered at the page boundary)", i, r[0], want)
		}
	}
}

// Role gate and audit (Req 10.4-10.6): ATM-SPV and ADMIN_PARAM may create the job; other roles are refused
// with no job row and no audit; the allowed actor gets export_requested at create and report_exported at the
// one download. The producer reads through pgReader (REPEATABLE READ READ ONLY snapshot on the replica pool),
// the same path every run() above uses.
func TestVendorPksExportRoleAndAudit(t *testing.T) {
	f := newT93Fix(t)
	ctx := context.Background()
	filter := t93Filter{status: "terkini", q: "T93"}
	p, err := ParseReportParams(KindVendorPks, filter.values(), time.Now())
	if err != nil {
		t.Fatal(err)
	}

	for _, role := range []string{"BRANCH-ATM-SPV", "VENDOR-USER", "ATM-USER", "ACM-SPV", ""} {
		_, _, err := f.svc.Create(ctx, Actor{UserID: f.otherUID, Username: "T93_spv", Role: role}, p)
		if !errors.Is(err, ErrForbidden) {
			t.Errorf("Create as %q: err = %v, want ErrForbidden", role, err)
		}
	}
	if n := f.count(`SELECT count(*) FROM export_jobs WHERE requested_by = $1`, f.otherUID); n != 0 {
		t.Errorf("export_jobs rows after refused creates = %d, want 0", n)
	}
	if n := f.count(`SELECT count(*) FROM audit_logs WHERE actor_id = $1`, f.otherUID); n != 0 {
		t.Errorf("audit rows after refused creates = %d, want 0", n)
	}

	spv := Actor{UserID: f.otherUID, Username: "T93_spv", Role: "ATM-SPV", IP: "127.0.0.1"}
	job, created, err := f.svc.Create(ctx, spv, p)
	if err != nil || !created {
		t.Fatalf("Create as ATM-SPV: created=%v err=%v", created, err)
	}
	if !strings.HasPrefix(job.FileName, "vendor-pks-") || !strings.HasSuffix(job.FileName, ".csv") {
		t.Errorf("file name = %q", job.FileName)
	}
	auditN := func(action string) int {
		return f.count(`SELECT count(*) FROM audit_logs WHERE actor_id = $1 AND action = $2 AND entity_type = 'vendor_pks_report'`, f.otherUID, action)
	}
	if auditN(ActionExportRequested) != 1 || auditN(ActionReportExported) != 0 {
		t.Fatalf("after create: export_requested=%d report_exported=%d, want 1/0", auditN(ActionExportRequested), auditN(ActionReportExported))
	}
	var status string
	if err := f.pool.QueryRow(ctx, `SELECT after->'filter'->>'status' FROM audit_logs WHERE actor_id = $1 AND action = $2`, f.otherUID, ActionExportRequested).Scan(&status); err != nil || status != "terkini" {
		t.Errorf("audit filter.status = %q err=%v, want terkini", status, err)
	}

	// Mark it ready (same way as the other service tests) and download once.
	f.exec(`UPDATE export_jobs SET status = 'succeeded', row_count = 0, byte_size = 5, chunk_count = 1,
		finished_at = now(), expires_at = now() + interval '10 minutes' WHERE id = $1`, job.ID)
	f.exec(`INSERT INTO export_job_chunks (export_job_id, seq, data) VALUES ($1, 0, $2)`, job.ID, []byte("a,b\n1\n"))
	if _, err := f.svc.StartDownload(ctx, spv, job.ID); err != nil {
		t.Fatalf("StartDownload: %v", err)
	}
	if auditN(ActionReportExported) != 1 {
		t.Errorf("report_exported after download = %d, want 1", auditN(ActionReportExported))
	}
	if _, err := f.svc.StartDownload(ctx, spv, job.ID); !errors.Is(err, ErrNotReady) {
		t.Errorf("second StartDownload: err = %v, want ErrNotReady", err)
	}
	if auditN(ActionReportExported) != 1 {
		t.Errorf("a refused second download wrote audit: %d", auditN(ActionReportExported))
	}

	// ADMIN_PARAM is allowed too (PksViewerRoles).
	if _, _, err := f.svc.Create(ctx, Actor{UserID: f.userID, Username: "T93_admin", Role: "ADMIN_PARAM"}, p); err != nil {
		t.Errorf("Create as ADMIN_PARAM: %v", err)
	}
}
