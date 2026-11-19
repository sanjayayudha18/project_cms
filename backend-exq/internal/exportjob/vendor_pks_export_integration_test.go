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

// vendor-pks 9.3 (Property 4 and Property 10, export part) on real Postgres. The vendor_pks producer runs
// through the real worker path (Service.Create -> worker.produce: REPEATABLE READ READ ONLY snapshot,
// ChunkWriter, csvexport.Writer). Every vendor the test inserts has a code starting with T93 and every
// request filters `q` on a T93 fragment, so real dev rows never reach the output. All rows (vendors, PKS,
// PICs, users, export_jobs, audit_logs) are removed in t.Cleanup; a last step fails the test on leftovers.
// Skips when DATABASE_URL is empty (p11Pool).

// t93Header is the literal 13-column header (vendor-pks-type Req 6.1; written out, not read from pkg/reports).
var t93Header = []string{"Kode Vendor", "Nama Perusahaan", "Nama Direktur Utama", "Telepon Direktur Utama",
	"Email Direktur Utama", "Type PKS", "No. WOD", "Nama PKS", "Tanggal Efektif PKS", "Tanggal Expired PKS",
	"Masa Berlaku (bulan)", "Monitoring PKS (hari)", "Status PKS"}

// t93Types are the seeded PKS types, in the export's type-code order (CIT < FLM).
var t93Types = []string{"CIT", "FLM"}

var t93Labels = map[string]string{"aktif": "Aktif", "need_renewal": "Need Renewal", "expired": "Expired",
	"terminated": "Terminated", "belum_diisi": "Belum diisi"}

// ---- reference model (same rules as vendor_pks_status / vendor_pks_validity_months, Req 6.2-6.6, 7) ----

func t93AddMonths(d time.Time, n int) time.Time {
	total := d.Year()*12 + int(d.Month()) - 1 + n
	y, m := total/12, time.Month(total%12+1)
	last := time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
	return time.Date(y, m, min(d.Day(), last), 0, 0, 0, 0, time.UTC)
}

// t93Status of one active PKS: "" = no status (INTERNAL vendor).
func t93Status(kind string, p *t93Pks, today time.Time, months int) string {
	switch {
	case kind == "INTERNAL":
		return ""
	case p.terminated:
		return "terminated"
	case p.expiry.Before(today):
		return "expired"
	case p.expiry.Before(t93AddMonths(today, months)):
		return "need_renewal"
	}
	return "aktif"
}

func t93ValidityMonths(eff, exp time.Time) int {
	end, n := exp.AddDate(0, 0, 1), 0
	for !t93AddMonths(eff, n+1).After(end) {
		n++
	}
	return n
}

type t93Pks struct {
	name        string
	effective   time.Time
	expiry      time.Time
	terminated  bool
	hasReplaced bool // an older, replaced PKS row exists: it must never show up
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
	pks      map[string]*t93Pks // active PKS per type code; absent type = no active PKS of that type
	dir      *t93Dir
}

func (v t93Vendor) code() string { return fmt.Sprintf("T93V%02d", v.idx) }
func (v t93Vendor) name() string { return fmt.Sprintf("T93 Vendor %02d Ü", v.idx) }

type t93Filter struct {
	q, status          string // q is never empty (it scopes the export to the fixture)
	pksType, pksStatus string // both set or both empty (Req 6.2)
}

func (f t93Filter) values() url.Values {
	v := url.Values{"q": {f.q}, "status": {f.status}}
	if f.pksStatus != "" {
		v.Set("pks_type", f.pksType)
		v.Set("pks_status", f.pksStatus)
	}
	return v
}

func (v t93Vendor) wod(typ string) string { return fmt.Sprintf("T93-WOD-%02d-%s", v.idx, typ) }

// t93Expected is the model of the file (Property 9): one row per ACTIVE PKS, ordered by vendor code then type
// code, filtered like the vendor list screen; pks_status = belum_diisi switches to one row per FLM vendor
// without an active PKS of the requested type.
func t93Expected(vs []t93Vendor, f t93Filter, today time.Time, months int) [][]string {
	var out [][]string
	for _, v := range vs {
		hay := strings.ToLower(v.code() + "\x00" + v.name())
		if !strings.Contains(hay, strings.ToLower(f.q)) {
			continue
		}
		if (f.status == "active" && v.disabled) || (f.status == "disabled" && !v.disabled) {
			continue
		}
		if f.pksStatus == "belum_diisi" {
			if v.kind == "FLM_VENDOR" && v.pks[f.pksType] == nil {
				out = append(out, t93Cells(v, f.pksType, nil, "belum_diisi", today))
			}
			continue
		}
		for _, typ := range t93Types {
			p := v.pks[typ]
			if p == nil || (f.pksType != "" && f.pksType != typ) {
				continue
			}
			st := t93Status(v.kind, p, today, months)
			if f.pksStatus != "" && st != f.pksStatus {
				continue
			}
			out = append(out, t93Cells(v, typ, p, st, today))
		}
	}
	return out
}

func t93Cells(v t93Vendor, typ string, p *t93Pks, st string, today time.Time) []string {
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
	row[5] = typ
	if p != nil {
		row[6] = csvexport.CSVSafe(v.wod(typ))
		row[7] = csvexport.CSVSafe(p.name)
		row[8], row[9] = p.effective.Format("2006-01-02"), p.expiry.Format("2006-01-02")
		row[10] = strconv.Itoa(t93ValidityMonths(p.effective, p.expiry))
		row[11] = strconv.Itoa(int(p.expiry.Sub(today).Hours() / 24)) // may be negative
	}
	row[12] = t93Labels[st] // "" for INTERNAL
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
	for _, typ := range t93Types {
		p := v.pks[typ]
		if p == nil {
			continue
		}
		insert := func(name, wod string, eff, exp time.Time, replaced, terminated bool) {
			var repl, term, by, reason any
			if replaced {
				repl = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			}
			if terminated {
				term, by, reason = time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), f.userID, "T93 reason"
			}
			f.exec(`INSERT INTO vendor_pks (vendor_id, pks_type_id, wod_number, pks_name, effective_date, expiry_date, created_by,
					replaced_at, terminated_at, terminated_by, termination_reason)
				VALUES ($1, (SELECT id FROM vendor_pks_types WHERE code = $2), $3, $4, $5::date, $6::date, $7,
					$8::timestamptz, $9::timestamptz, $10, $11)`,
				vid, typ, wod, name, eff, exp, f.userID, repl, term, by, reason)
		}
		if p.hasReplaced { // an older PKS of the same type, already replaced by the active one
			insert("T93 old PKS", v.wod(typ)+"-OLD", p.effective.AddDate(-3, 0, 0), p.effective.AddDate(-1, 0, 0), true, false)
		}
		insert(p.name, v.wod(typ), p.effective, p.expiry, false, p.terminated)
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
func (f *t93Fix) run(filter t93Filter, months int, asOf time.Time) (records [][]string, raw string) {
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
	w := newWorker(pgStore{pool: f.pool, q: repository.New(f.pool)}, pgReader{pool: f.pool}, NewProducers(nil, months), WorkerConfig{})
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

// ---- tests ----

// Property 9 / Property 4 (export part): for random vendors x types x PKS and random filters, the exported
// lines equal the reference model: one line per active PKS (type, WOD, status label, validity, remaining days,
// director), `belum_diisi` = FLM vendor without an active PKS of the type, order by vendor code then type code.
func TestVendorPksExportMatchesModel(t *testing.T) {
	f := newT93Fix(t)
	qs := []string{"T93", "t93v", "T93V01", "93 vendor", "T93V0"}

	rapid.Check(t, func(rt *rapid.T) {
		f.clearVendors()
		defer f.clearVendors()

		asOf := time.Date(2026, 1, 1, 0, 0, 0, 0, reports.WIB).AddDate(0, 0, rapid.IntRange(0, 1000).Draw(rt, "day")).
			Add(time.Duration(rapid.IntRange(0, 23*60+59).Draw(rt, "minute_of_day")) * time.Minute)
		today := reports.TodayWIB(asOf)
		months := rapid.IntRange(1, 24).Draw(rt, "renewal_months")

		n := rapid.IntRange(0, 6).Draw(rt, "vendors")
		vs := make([]t93Vendor, 0, n)
		for i := 1; i <= n; i++ {
			v := t93Vendor{idx: i, pks: map[string]*t93Pks{},
				kind:     rapid.SampledFrom([]string{"FLM_VENDOR", "FLM_VENDOR", "INTERNAL"}).Draw(rt, "kind"),
				disabled: rapid.IntRange(0, 3).Draw(rt, "disabled") == 0,
			}
			for _, typ := range t93Types {
				if !rapid.Bool().Draw(rt, "has_pks_"+typ) {
					continue
				}
				var exp time.Time
				switch rapid.IntRange(0, 2).Draw(rt, "expiry_mode_"+typ) {
				case 0:
					exp = today.AddDate(0, 0, rapid.IntRange(-400, 700).Draw(rt, "exp_off_"+typ))
				case 1: // around today
					exp = today.AddDate(0, 0, rapid.IntRange(-2, 2).Draw(rt, "exp_today_"+typ))
				default: // around the renewal boundary
					exp = t93AddMonths(today, months).AddDate(0, 0, rapid.IntRange(-2, 2).Draw(rt, "exp_renew_"+typ))
				}
				eff := exp.AddDate(0, 0, -rapid.IntRange(0, 1200).Draw(rt, "span_"+typ))
				v.pks[typ] = &t93Pks{name: fmt.Sprintf("T93 PKS %d %s =x", i, typ), effective: eff, expiry: exp,
					terminated:  rapid.IntRange(0, 3).Draw(rt, "terminated_"+typ) == 0,
					hasReplaced: rapid.Bool().Draw(rt, "replaced_"+typ)}
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
			f.insertVendor(v)
			vs = append(vs, v)
		}

		filter := t93Filter{
			q:      rapid.SampledFrom(qs).Draw(rt, "q"),
			status: rapid.SampledFrom([]string{"active", "disabled", "all"}).Draw(rt, "status"),
		}
		if rapid.Bool().Draw(rt, "filter_pks") {
			filter.pksType = rapid.SampledFrom(t93Types).Draw(rt, "pks_type")
			filter.pksStatus = rapid.SampledFrom([]string{"aktif", "need_renewal", "expired", "terminated", "belum_diisi"}).Draw(rt, "pks_status")
		}

		records, _ := f.run(filter, months, asOf)
		if !slices.Equal(records[0], t93Header) {
			rt.Fatalf("header = %q, want %q", records[0], t93Header)
		}
		want := t93Expected(vs, filter, today, months)
		got := records[1:]
		if len(got) != len(want) {
			rt.Fatalf("filter=%+v today=%s months=%d: %d rows, want %d\ngot:  %q\nwant: %q", filter, today.Format("2006-01-02"), months, len(got), len(want), got, want)
		}
		for i := range want {
			if !slices.Equal(got[i], want[i]) {
				rt.Fatalf("filter=%+v today=%s months=%d row %d\ngot:  %q\nwant: %q", filter, today.Format("2006-01-02"), months, i, got[i], want[i])
			}
		}
	})
}

// Property 9, fixed: a vendor with both types gives two lines, a vendor with one type gives one, and
// belum_diisi for CIT lists the FLM-only vendor but neither the both-type vendor nor INTERNAL.
func TestVendorPksExportOneRowPerActivePks(t *testing.T) {
	f := newT93Fix(t)
	asOf := time.Date(2026, 7, 1, 8, 0, 0, 0, reports.WIB)
	today := reports.TodayWIB(asOf)
	mk := func(name string) *t93Pks {
		return &t93Pks{name: name, effective: today.AddDate(-1, 0, 0), expiry: today.AddDate(2, 0, 0)}
	}
	vs := []t93Vendor{
		{idx: 1, kind: "FLM_VENDOR", pks: map[string]*t93Pks{"FLM": mk("T93 A-FLM"), "CIT": mk("T93 A-CIT")}},
		{idx: 2, kind: "FLM_VENDOR", pks: map[string]*t93Pks{"FLM": mk("T93 B-FLM")}},
		{idx: 3, kind: "FLM_VENDOR"},
		{idx: 4, kind: "INTERNAL"},
	}
	for _, v := range vs {
		f.insertVendor(v)
	}
	records, _ := f.run(t93Filter{q: "T93V", status: "active"}, 9, asOf)
	if got := len(records) - 1; got != 3 {
		t.Fatalf("unfiltered rows = %d, want 3 (2 + 1 active PKS, vendors without PKS absent)", got)
	}
	if records[1][0] != "T93V01" || records[1][5] != "CIT" || records[2][0] != "T93V01" || records[2][5] != "FLM" ||
		records[3][0] != "T93V02" || records[3][5] != "FLM" {
		t.Errorf("order/type cells: %q", records[1:])
	}
	if records[1][6] != "T93-WOD-01-CIT" {
		t.Errorf("WOD cell = %q", records[1][6])
	}

	records, _ = f.run(t93Filter{q: "T93V", status: "active", pksType: "CIT", pksStatus: "belum_diisi"}, 9, asOf)
	var codes []string
	for _, r := range records[1:] {
		codes = append(codes, r[0])
		if r[5] != "CIT" || r[12] != "Belum diisi" || r[6] != "" || r[7] != "" || r[8] != "" || r[11] != "" {
			t.Errorf("belum_diisi row has PKS cells: %q", r)
		}
	}
	if !slices.Equal(codes, []string{"T93V02", "T93V03"}) {
		t.Errorf("belum_diisi CIT vendors = %v, want [T93V02 T93V03] (FLM vendors lacking CIT; not INTERNAL)", codes)
	}
}

// Fixed boundary cases that pin the empty-cell contract (INTERNAL, belum_diisi) and the WIB day (Req 10.2).
func TestVendorPksExportEmptyCellsAndWIBDay(t *testing.T) {
	f := newT93Fix(t)
	// 2026-06-30 18:00 UTC is 2026-07-01 01:00 WIB: "today" is July 1, not June 30.
	asOf := time.Date(2026, 6, 30, 18, 0, 0, 0, time.UTC)
	today := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	flm := func(name string, exp time.Time) map[string]*t93Pks {
		return map[string]*t93Pks{"FLM": {name: name, effective: today.AddDate(-1, 0, 0), expiry: exp}}
	}
	vs := []t93Vendor{
		{idx: 1, kind: "FLM_VENDOR"}, // no PKS, no director -> only in the belum_diisi export
		{idx: 2, kind: "INTERNAL", pks: flm("T93 PKS 2", today.AddDate(1, 0, 0))},    // INTERNAL: empty status
		{idx: 3, kind: "FLM_VENDOR", pks: flm("T93 PKS 3", today)},                   // expires today: still valid
		{idx: 4, kind: "FLM_VENDOR", pks: flm("T93 PKS 4", today.AddDate(0, 0, -1))}, // expired yesterday
	}
	for _, v := range vs {
		f.insertVendor(v)
	}
	filter := t93Filter{q: "T93V", status: "active"}
	records, raw := f.run(filter, 9, asOf)
	want := t93Expected(vs, filter, today, 9)
	if len(records) != 1+len(want) || len(want) != 3 {
		t.Fatalf("rows = %d, want %d\n%s", len(records)-1, len(want), raw)
	}
	for i := range want {
		if !slices.Equal(records[1+i], want[i]) {
			t.Errorf("row %d\ngot:  %q\nwant: %q", i, records[1+i], want[i])
		}
	}
	// Literal spot checks, independent of the model.
	if r := records[1]; r[0] != "T93V02" || r[5] != "FLM" || r[12] != "" {
		t.Errorf("INTERNAL vendor with PKS must have an empty status: %q", r)
	}
	if r := records[2]; r[11] != "0" || r[12] == "Expired" {
		t.Errorf("PKS expiring today (WIB day): remaining %q status %q", r[11], r[12])
	}
	if r := records[3]; r[11] != "-1" || r[12] != "Expired" {
		t.Errorf("PKS expired yesterday: remaining %q status %q", r[11], r[12])
	}
	records, _ = f.run(t93Filter{q: "T93V", status: "active", pksType: "FLM", pksStatus: "belum_diisi"}, 9, asOf)
	if len(records) != 2 || records[1][0] != "T93V01" || records[1][12] != "Belum diisi" || records[1][2] != "" || records[1][10] != "" {
		t.Errorf("belum_diisi FLM export: %q", records)
	}
}

// Keyset paging of the belum_diisi branch over more than one page: no row lost or doubled.
func TestVendorPksExportKeysetPaging(t *testing.T) {
	f := newT93Fix(t)
	total := PageSize + 37
	f.exec(`INSERT INTO vendors (code, name, kind, is_active)
		SELECT 'T93P' || lpad(i::text, 5, '0'), 'T93 Page ' || lpad(((i + 1) / 2)::text, 5, '0'), 'FLM_VENDOR', true
		FROM generate_series(1, $1::int) i`, total)
	records, _ := f.run(t93Filter{q: "T93P", status: "active", pksType: "FLM", pksStatus: "belum_diisi"}, 9, time.Date(2026, 7, 1, 8, 0, 0, 0, reports.WIB))
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

// Role gate and audit (Req 6.8, 10.5, 10.6): a non master-data role is refused with no job row and no audit;
// the allowed role gets export_requested at create and report_exported at download.
func TestVendorPksExportRoleAndAudit(t *testing.T) {
	f := newT93Fix(t)
	ctx := context.Background()
	p, err := ParseReportParams(KindVendorPks, t93Filter{q: "T93", status: "active"}.values(), time.Now())
	if err != nil {
		t.Fatal(err)
	}

	for _, role := range []string{"ATM-SPV", "BRANCH-ATM-SPV", "VENDOR-USER", "ATM-USER", ""} {
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

	a := Actor{UserID: f.userID, Username: "T93_admin", Role: "ADMIN", IP: "127.0.0.1"}
	job, created, err := f.svc.Create(ctx, a, p)
	if err != nil || !created {
		t.Fatalf("Create: created=%v err=%v", created, err)
	}
	if !strings.HasPrefix(job.FileName, "vendor-pks-") || !strings.HasSuffix(job.FileName, ".csv") {
		t.Errorf("file name = %q", job.FileName)
	}
	auditN := func(action string) int {
		return f.count(`SELECT count(*) FROM audit_logs WHERE actor_id = $1 AND action = $2 AND entity_type = 'vendor_pks_report'`, f.userID, action)
	}
	if auditN(ActionExportRequested) != 1 || auditN(ActionReportExported) != 0 {
		t.Fatalf("after create: export_requested=%d report_exported=%d, want 1/0", auditN(ActionExportRequested), auditN(ActionReportExported))
	}
	var status string
	if err := f.pool.QueryRow(ctx, `SELECT after->'filter'->>'status' FROM audit_logs WHERE actor_id = $1 AND action = $2`, f.userID, ActionExportRequested).Scan(&status); err != nil || status != "active" {
		t.Errorf("audit filter.status = %q err=%v, want active", status, err)
	}

	// Mark it ready (same way as the other service tests) and download once.
	f.exec(`UPDATE export_jobs SET status = 'succeeded', row_count = 0, byte_size = 5, chunk_count = 1,
		finished_at = now(), expires_at = now() + interval '10 minutes' WHERE id = $1`, job.ID)
	f.exec(`INSERT INTO export_job_chunks (export_job_id, seq, data) VALUES ($1, 0, $2)`, job.ID, []byte("a,b\n1\n"))
	if _, err := f.svc.StartDownload(ctx, a, job.ID); err != nil {
		t.Fatalf("StartDownload: %v", err)
	}
	if auditN(ActionReportExported) != 1 {
		t.Errorf("report_exported after download = %d, want 1", auditN(ActionReportExported))
	}
	if _, err := f.svc.StartDownload(ctx, a, job.ID); !errors.Is(err, ErrNotReady) {
		t.Errorf("second StartDownload: err = %v, want ErrNotReady", err)
	}
	if auditN(ActionReportExported) != 1 {
		t.Errorf("a refused second download wrote audit: %d", auditN(ActionReportExported))
	}
}
