//go:build integration

package exportjob

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend-exq/internal/audit"
	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
	"github.com/cimb-niaga/cms/pkg/csvexport"
)

// task 11.1 (Properties 1, 2, 6, 7, 8) on real Postgres, producers read through the real
// repository. Every inserted row carries the unique tag P11X<hex> (vendor/branch/vault/
// package codes, DSR labels, terminal ids, usernames) and is deleted by tag in
// t.Cleanup (reverse FK order), even when the test fails. Existing dev rows are only read.
//
// Fixture: 2 vendors (the 2nd soft-deleted), 3 branches, random vaults / PICs /
// DSR maps / package prices (+ ATMs when a location exists). Expected results come from
// the in-memory model built while inserting, never from the producers' own SQL.

// p11Row is one fixture row as the model sees it.
type p11Row struct {
	id           int64
	vendor       int // index into p11Fix.vcodes
	branch       int // index into p11Fix.bcodes, -1 = none
	vcode, bcode string
	disabled     bool
	price        bool
	hasEnd       bool
	end          time.Time // UTC midnight of effective_end_date
	level        string    // price rows: vendor | branch | atm
	minCap       string    // vault rows
	maxCap       string
	base         string // price rows
	name         string // pic rows
}

type p11Fix struct {
	t       *testing.T
	pool    *pgxpool.Pool
	q       *repository.Queries
	rng     *rand.Rand
	tag     string
	vcodes  [2]string
	bcodes  [3]string
	bvendor [3]int
	vIDs    [2]int64
	bIDs    [3]int64
	userID  int64
	atmID   int64
	rows    map[string][]p11Row
	seq     int
}

func p11Seed(t *testing.T) *rand.Rand {
	t.Helper()
	seed := uint64(time.Now().UnixNano())
	if s := os.Getenv("P11_SEED"); s != "" {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			t.Fatalf("P11_SEED: %v", err)
		}
		seed = v
	}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("reproduce with P11_SEED=%d", seed)
		}
	})
	t.Logf("p11 seed=%d", seed)
	return rand.New(rand.NewPCG(seed, 0x11))
}

func p11Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set -- skipping integration test")
	}
	url = strings.Replace(url, "host.docker.internal", "localhost", 1)
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}
	cfg.MaxConns = 6
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping db: %v", err)
	}
	t.Cleanup(pool.Close) // registered first = runs last, after the fixture cleanup
	return pool
}

// p11Num returns a random numeric(20,2) as (SQL arg, expected Postgres text).
// i cycles through the edge cases: 0, max, NULL, small, random.
func p11Num(rng *rand.Rand, i int) (*string, string) {
	var s string
	switch i % 5 {
	case 0:
		s = "0"
	case 1:
		s = "999999999999999999.99"
	case 2:
		return nil, ""
	case 3:
		s = "0.01"
	default:
		n := 1 + rng.IntN(18)
		var b strings.Builder
		b.WriteByte(byte('1' + rng.IntN(9)))
		for range n - 1 {
			b.WriteByte(byte('0' + rng.IntN(10)))
		}
		s = b.String()
		if rng.IntN(2) == 0 {
			s += "." + strconv.Itoa(rng.IntN(100))
		}
	}
	return &s, p11Want(s)
}

// p11Want formats a plain decimal string the way numeric(20,2)::text prints it.
func p11Want(s string) string {
	whole, frac, _ := strings.Cut(s, ".")
	switch {
	case frac == "":
		frac = "00"
	case len(frac) == 1 && strings.Contains(s, "."):
		// "7.5" arrives as digits after Itoa: 5 means 0.05 only if the generator
		// wrote it so; treat the text literally: ".5" is 0.50.
		frac += "0"
	}
	return whole + "." + frac
}

func newP11Fix(t *testing.T) *p11Fix {
	t.Helper()
	f := &p11Fix{t: t, rng: p11Seed(t), rows: map[string][]p11Row{}}
	f.pool = p11Pool(t)
	f.q = repository.New(f.pool)
	f.tag = fmt.Sprintf("P11X%X", time.Now().UnixNano()%0xffffffffff)
	t.Cleanup(f.cleanup) // runs before pool.Close
	f.seed()
	return f
}

func (f *p11Fix) cleanup() {
	ctx := context.Background()
	like := f.tag + "%"
	inVendors := `(SELECT id FROM vendors WHERE code LIKE $1)`
	steps := []struct{ name, sql string }{
		{"export_job_chunks", `DELETE FROM export_job_chunks WHERE export_job_id IN (SELECT id FROM export_jobs WHERE requested_by IN (SELECT id FROM users WHERE username LIKE $1))`},
		{"export_jobs", `DELETE FROM export_jobs WHERE requested_by IN (SELECT id FROM users WHERE username LIKE $1)`},
		{"users", `DELETE FROM users WHERE username LIKE $1`},
		{"dsr_location_vault_maps", `DELETE FROM dsr_location_vault_maps WHERE vendor_id IN ` + inVendors},
		{"vendor_package_prices", `DELETE FROM vendor_package_prices WHERE vendor_id IN ` + inVendors},
		{"atms", `DELETE FROM atms WHERE terminal_id LIKE $1`},
		{"vendor_pics", `DELETE FROM vendor_pics WHERE vendor_id IN ` + inVendors},
		{"vendor_vaults", `DELETE FROM vendor_vaults WHERE vendor_branch_id IN (SELECT id FROM vendor_branches WHERE vendor_id IN ` + inVendors + `)`},
		{"vendor_branches", `DELETE FROM vendor_branches WHERE vendor_id IN ` + inVendors},
		{"vendors", `DELETE FROM vendors WHERE code LIKE $1`},
	}
	for _, s := range steps {
		if _, err := f.pool.Exec(ctx, s.sql, like); err != nil {
			f.t.Errorf("p11 cleanup %s: %v", s.name, err)
		}
	}
}

func (f *p11Fix) id(sql string, args ...any) int64 {
	f.t.Helper()
	var id int64
	if err := f.pool.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		f.t.Fatalf("p11 fixture insert: %v\n%s", err, sql)
	}
	return id
}

func (f *p11Fix) chance(pct int) bool { return f.rng.IntN(100) < pct }

func (f *p11Fix) next() int { f.seq++; return f.seq }

var p11PicNames = []string{"Budi", "=SUM(1+1)", `A "quoted", name`, "Line\nbreak", "+62 plus", "-minus", "@at", "Siti, S.Kom"}

func (f *p11Fix) seed() {
	ctx := context.Background()
	f.f_users(ctx)
	f.bvendor = [3]int{0, 0, 1}
	for i := range 2 {
		f.vcodes[i] = fmt.Sprintf("%sV%d", f.tag, i)
		dis := i == 1 // vendor B is soft-deleted
		f.vIDs[i] = f.id(`INSERT INTO vendors (code, name, is_active, deleted_at)
			VALUES ($1, $1, $2, CASE WHEN $3::bool THEN now() END) RETURNING id`, f.vcodes[i], !dis, dis)
		f.rows["vendor"] = append(f.rows["vendor"], p11Row{id: f.vIDs[i], vendor: i, branch: -1, vcode: f.vcodes[i], disabled: dis})
	}
	for b := range 3 {
		f.bcodes[b] = fmt.Sprintf("%sB%d", f.tag, b)
		dis := f.chance(25)
		v := f.bvendor[b]
		f.bIDs[b] = f.id(`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, is_active, deleted_at)
			VALUES ($1, $2, $2, $3, CASE WHEN $4::bool THEN now() END) RETURNING id`, f.vIDs[v], f.bcodes[b], !dis, dis)
		f.rows["branches"] = append(f.rows["branches"], p11Row{id: f.bIDs[b], vendor: v, branch: b, vcode: f.vcodes[v], bcode: f.bcodes[b], disabled: dis})
	}
	var vaultOf []p11Row // vaults, for the DSR maps
	n := 0
	for b := range 3 {
		for range f.rng.IntN(5) {
			cats := []string{"ATM", "CASH", "ATM_CASH"}
			cat := cats[f.rng.IntN(3)]
			dis := f.chance(30)
			minA, minW := p11Num(f.rng, n)
			maxA, maxW := p11Num(f.rng, n+f.rng.IntN(5))
			n++
			code := fmt.Sprintf("%sVL%d", f.tag, f.next())
			id := f.id(`INSERT INTO vendor_vaults (vendor_branch_id, vault_code, type, category, currency_code, is_active,
				min_capacity_amount, max_capacity_amount, deleted_at)
				VALUES ($1, $2, $3, $3, 'IDR', $4, $5::text::numeric, $6::text::numeric, CASE WHEN $7::bool THEN now() END) RETURNING id`,
				f.bIDs[b], code, cat, !dis, minA, maxA, dis)
			r := p11Row{id: id, vendor: f.bvendor[b], branch: b, vcode: f.vcodes[f.bvendor[b]], bcode: f.bcodes[b], disabled: dis, minCap: minW, maxCap: maxW}
			f.rows["vaults"] = append(f.rows["vaults"], r)
			vaultOf = append(vaultOf, r)
			_ = code
		}
	}
	for _, v := range vaultOf {
		if !f.chance(70) {
			continue
		}
		dis := f.chance(30)
		id := f.id(`INSERT INTO dsr_location_vault_maps (vendor_id, dsr_location, vendor_vault_id, is_active, deleted_at)
			VALUES ($1, $2, $3, $4, CASE WHEN $5::bool THEN now() END) RETURNING id`,
			f.vIDs[v.vendor], fmt.Sprintf("%sloc%d", f.tag, f.next()), v.id, !dis, dis)
		f.rows["dsr-location-maps"] = append(f.rows["dsr-location-maps"], p11Row{id: id, vendor: v.vendor, branch: v.branch, vcode: v.vcode, bcode: v.bcode, disabled: dis})
	}
	for range 8 + f.rng.IntN(8) {
		v := f.rng.IntN(2)
		b := -1
		var bp *int64
		if f.chance(60) { // branch PIC; branch must belong to the vendor
			for _, c := range f.rng.Perm(3) {
				if f.bvendor[c] == v {
					b = c
					bp = &f.bIDs[c]
					break
				}
			}
		}
		dis := f.chance(30)
		name := p11PicNames[f.rng.IntN(len(p11PicNames))]
		id := f.id(`INSERT INTO vendor_pics (vendor_id, vendor_branch_id, name, email, is_active, deleted_at)
			VALUES ($1, $2, $3, 'p11@test.local', $4, CASE WHEN $5::bool THEN now() END) RETURNING id`,
			f.vIDs[v], bp, name, !dis, dis)
		r := p11Row{id: id, vendor: v, branch: b, vcode: f.vcodes[v], disabled: dis, name: name}
		if b >= 0 {
			r.bcode = f.bcodes[b]
		}
		f.rows["pics"] = append(f.rows["pics"], r)
	}
	f.seedATM(ctx)
	f.seedPrices()
}

func (f *p11Fix) f_users(ctx context.Context) {
	f.userID = f.id(`INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source)
		SELECT id, $1, $1, $1 || '@test.local', true, 'ldap' FROM roles WHERE role = 'ADMIN' RETURNING id`, f.tag+"-u")
}

// seedATM inserts one ATM for atm-level prices when a location exists; failure only
// drops atm-level rows (the ATM table has constraints this test does not own).
func (f *p11Fix) seedATM(ctx context.Context) {
	var loc int64
	if err := f.pool.QueryRow(ctx, `SELECT id FROM locations ORDER BY id LIMIT 1`).Scan(&loc); err != nil {
		f.t.Logf("p11: no location, skipping atm-level prices (%v)", err)
		return
	}
	err := f.pool.QueryRow(ctx, `INSERT INTO atms (terminal_id, location_id, machine_type, brand, model, operation_hours, deployment_type)
		VALUES ($1, $2, 'ATM', 'P11', 'P11', '24H', 'ONSITE') RETURNING id`, f.tag+"T1", loc).Scan(&f.atmID)
	if err != nil {
		f.t.Logf("p11: atm insert failed, skipping atm-level prices: %v", err)
		f.atmID = 0
	}
}

func (f *p11Fix) seedPrices() {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	insert := func(v, b int, atm bool, level string) {
		i := f.next()
		var bp, ap *int64
		bcode := ""
		if b >= 0 {
			bp, bcode = &f.bIDs[b], f.bcodes[b]
		}
		if atm {
			ap = &f.atmID
		}
		bA, bW := p11Num(f.rng, i)
		var endArg *string
		r := p11Row{vendor: v, branch: b, vcode: f.vcodes[v], bcode: bcode, price: true, level: level, base: bW}
		if f.chance(65) {
			r.hasEnd = true
			r.end = base.AddDate(0, 0, 10+f.rng.IntN(300))
			s := r.end.Format("2006-01-02")
			endArg = &s
		}
		r.id = f.id(`INSERT INTO vendor_package_prices (vendor_id, package, package_code, machine_group, price_class, tier_min,
			base_price, vendor_branch_id, atm_id, currency, effective_start_date, effective_end_date)
			VALUES ($1, $2, $3, 'ATM', 'REGULAR', 1, $4::text::numeric, $5, $6, 'IDR', '2026-01-01', $7::text::date) RETURNING id`,
			f.vIDs[v], fmt.Sprintf("PAKET %s-%d", f.tag, i), fmt.Sprintf("%sPC%d", f.tag, i), bA, bp, ap, endArg)
		f.rows["package-prices"] = append(f.rows["package-prices"], r)
	}
	for v := range 2 {
		for range 2 + f.rng.IntN(3) {
			insert(v, -1, false, "vendor")
		}
	}
	for b := range 3 {
		for range 1 + f.rng.IntN(3) {
			insert(f.bvendor[b], b, false, "branch")
		}
	}
	if f.atmID != 0 {
		insert(0, -1, true, "atm")
		insert(1, -1, true, "atm")
	}
}

// ---- expected results (model) ----

type p11Q struct {
	entity string
	vendor *int // fixture vendor index
	branch *int
	status string // active | disabled | all
	level  string // price rows: vendor|branch|atm|all
	asOf   time.Time
}

func p11Date(t time.Time) time.Time {
	y, m, d := t.In(time.FixedZone("WIB", 7*3600)).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func (r p11Row) matches(status string, asOfD time.Time) bool {
	if status == "all" {
		return true
	}
	active := !r.disabled
	if r.price {
		active = !r.hasEnd || !r.end.Before(asOfD)
	}
	return active == (status == "active")
}

func (f *p11Fix) expect(q p11Q, forceAll bool) []p11Row {
	var out []p11Row
	for _, r := range f.rows[q.entity] {
		if q.vendor != nil && r.vendor != *q.vendor {
			continue
		}
		if q.branch != nil && r.branch != *q.branch {
			continue
		}
		if q.level != "" && q.level != "all" && r.level != q.level {
			continue
		}
		if !forceAll && !r.matches(q.status, p11Date(q.asOf)) {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}

func (f *p11Fix) ids(rows []p11Row) []int64 {
	out := make([]int64, len(rows))
	for i, r := range rows {
		out[i] = r.id
	}
	return out
}

// ---- running the producer ----

func p11Produce(ctx context.Context, q Querier, p Params, asOf time.Time) ([][]string, error) {
	pr := DefaultProducers()[p.Kind][p.Entity]
	if pr == nil {
		return nil, fmt.Errorf("no producer %s/%s", p.Kind, p.Entity)
	}
	var rows [][]string
	err := pr.Produce(ctx, q, p, asOf, func(c []string) error {
		rows = append(rows, slices.Clone(c))
		return nil
	})
	return rows, err
}

func p11CellIDs(t *testing.T, rows [][]string) []int64 {
	t.Helper()
	out := make([]int64, len(rows))
	for i, r := range rows {
		id, err := strconv.ParseInt(r[0], 10, 64)
		if err != nil {
			t.Fatalf("row %d: bad id cell %q", i, r[0])
		}
		out[i] = id
	}
	return out
}

func p11Params(t *testing.T, kind Kind, q p11Q, f *p11Fix) Params {
	t.Helper()
	var v, b *int64
	if q.vendor != nil {
		v = &f.vIDs[*q.vendor]
	}
	if q.branch != nil {
		b = &f.bIDs[*q.branch]
	}
	level := ""
	if kind == KindHargaPaket {
		level = q.level
	}
	p, err := ParseParams(kind, q.entity, v, b, q.status, level)
	if err != nil {
		t.Fatalf("ParseParams(%s %+v): %v", kind, q, err)
	}
	return p
}

// p11CheckCells compares the cells of every produced row with the model row.
func (f *p11Fix) checkCells(t *testing.T, q p11Q, rows [][]string, want []p11Row) {
	t.Helper()
	for i, w := range want {
		c := rows[i]
		if c[1] != w.vcode {
			t.Errorf("%s id %d: vendor cell %q, want %q", q.entity, w.id, c[1], w.vcode)
		}
		switch q.entity {
		case "vendor":
		case "package-prices":
			if c[3] != w.bcode || c[2] != w.level || c[11] != w.base {
				t.Errorf("price id %d: level/branch/base %q/%q/%q, want %q/%q/%q", w.id, c[2], c[3], c[11], w.level, w.bcode, w.base)
			}
		default:
			if c[2] != w.bcode {
				t.Errorf("%s id %d: branch cell %q, want %q", q.entity, w.id, c[2], w.bcode)
			}
		}
		if q.entity == "vaults" && (c[6] != w.minCap || c[7] != w.maxCap) {
			t.Errorf("vault id %d: capacity %q/%q, want %q/%q", w.id, c[6], c[7], w.minCap, w.maxCap)
		}
		if q.entity == "pics" {
			if got := csvexport.UnCSVSafe(c[3]); got != w.name {
				t.Errorf("pic id %d: name %q, want %q", w.id, got, w.name)
			}
			if q.branch != nil && c[2] == "" {
				t.Errorf("pic id %d: vendor-wide PIC in a cabang export", w.id)
			}
		}
	}
}

func p11Set(ids []int64) map[int64]bool {
	m := make(map[int64]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// ---- Properties 1, 2, 6, 8 (fixture-scoped) ----

func TestP11ProducersProperties(t *testing.T) {
	f := newP11Fix(t)
	ctx := context.Background()
	t.Logf("fixture rows: vendor=%d branches=%d vaults=%d pics=%d maps=%d prices=%d atm=%v",
		len(f.rows["vendor"]), len(f.rows["branches"]), len(f.rows["vaults"]), len(f.rows["pics"]),
		len(f.rows["dsr-location-maps"]), len(f.rows["package-prices"]), f.atmID != 0)

	statuses := []string{"active", "disabled", "all"}
	levels := []string{"vendor", "branch", "atm", "all"}
	ip := func(i int) *int { return &i }

	const iterations = 150
	for it := range iterations {
		var kind Kind
		var q p11Q
		switch it % 4 {
		case 0:
			kind = KindVendor
			q = p11Q{entity: []string{"vendor", "branches", "vaults", "pics", "dsr-location-maps"}[f.rng.IntN(5)], vendor: ip(f.rng.IntN(2))}
		case 1:
			kind = KindCabang // vendor and cabang may mismatch: must give nothing
			q = p11Q{entity: []string{"vaults", "pics", "dsr-location-maps"}[f.rng.IntN(3)], vendor: ip(f.rng.IntN(2)), branch: ip(f.rng.IntN(3))}
		case 2:
			kind = KindHargaPaket
			q = p11Q{entity: "package-prices", vendor: ip(f.rng.IntN(2)), level: levels[f.rng.IntN(4)]}
			if f.chance(50) {
				q.branch = ip(f.rng.IntN(3))
			}
		default:
			kind = KindVendor
			q = p11Q{entity: "package-prices"} // placeholder, replaced below
			q = p11Q{entity: "branches", vendor: ip(f.rng.IntN(2))}
		}
		// as-of: around a random end date, at an hour that straddles midnight WIB
		q.asOf = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 8+f.rng.IntN(320)).Add(time.Duration(f.rng.IntN(24)) * time.Hour)

		var perStatus [3][]int64
		for si, st := range statuses {
			qq := q
			qq.status = st
			p := p11Params(t, kind, qq, f)
			forceAll := kind == KindVendor && q.entity == "vendor"
			want := f.expect(qq, forceAll)

			rows, err := p11Produce(ctx, f.q, p, qq.asOf)
			if err != nil {
				t.Fatalf("it %d %s/%s %+v: %v", it, kind, q.entity, qq, err)
			}
			got := p11CellIDs(t, rows)
			// Property 1 + 6: exactly the in-scope rows, once each, ascending ids.
			if !slices.Equal(got, f.ids(want)) {
				t.Fatalf("it %d %s/%s status=%s level=%s v=%v b=%v asOf=%s:\n got %v\nwant %v",
					it, kind, q.entity, st, q.level, q.vendor, q.branch, qq.asOf.Format(time.RFC3339), got, f.ids(want))
			}
			f.checkCells(t, qq, rows, want)
			// Property 6: determinism.
			again, err := p11Produce(ctx, f.q, p, qq.asOf)
			if err != nil || !reflect.DeepEqual(rows, again) {
				t.Fatalf("it %d %s/%s status=%s: second run differs (err=%v)", it, kind, q.entity, st, err)
			}
			perStatus[si] = got
		}
		// Property 2: active and disabled partition all.
		act, dis, all := p11Set(perStatus[0]), p11Set(perStatus[1]), p11Set(perStatus[2])
		if kind == KindVendor && q.entity == "vendor" {
			for _, s := range perStatus {
				if len(s) != 1 {
					t.Fatalf("it %d: vendor profile rows = %v, want exactly 1 for every status", it, s)
				}
			}
			continue
		}
		for id := range act {
			if dis[id] {
				t.Fatalf("it %d %s/%s: id %d is both active and disabled", it, kind, q.entity, id)
			}
		}
		if len(act)+len(dis) != len(all) {
			t.Fatalf("it %d %s/%s: |active|+|disabled|=%d != |all|=%d", it, kind, q.entity, len(act)+len(dis), len(all))
		}
		for id := range all {
			if !act[id] && !dis[id] {
				t.Fatalf("it %d %s/%s: id %d in all but in neither partition", it, kind, q.entity, id)
			}
		}
	}

	t.Run("Property8_money_text_matches_postgres", func(t *testing.T) {
		check := func(table, col string, rows []p11Row, pick func(p11Row) string) {
			for _, r := range rows {
				var txt *string
				if err := f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT %s::text FROM %s WHERE id = $1`, col, table), r.id).Scan(&txt); err != nil {
					t.Fatalf("%s.%s id %d: %v", table, col, r.id, err)
				}
				got := ""
				if txt != nil {
					got = *txt
				}
				if got != pick(r) {
					t.Errorf("%s.%s id %d: postgres text %q, model %q", table, col, r.id, got, pick(r))
				}
			}
		}
		check("vendor_vaults", "min_capacity_amount", f.rows["vaults"], func(r p11Row) string { return r.minCap })
		check("vendor_vaults", "max_capacity_amount", f.rows["vaults"], func(r p11Row) string { return r.maxCap })
		check("vendor_package_prices", "base_price", f.rows["package-prices"], func(r p11Row) string { return r.base })
		// producers print exactly that text, for every status
		for _, ent := range []string{"vaults", "package-prices"} {
			kind, qq := KindVendor, p11Q{entity: ent, status: "all"}
			if ent == "package-prices" {
				kind, qq.level = KindHargaPaket, "all"
			}
			for v := range 2 {
				qq.vendor = &v
				rows, err := p11Produce(ctx, f.q, p11Params(t, kind, qq, f), time.Now())
				if err != nil {
					t.Fatal(err)
				}
				f.checkCells(t, qq, rows, f.expect(qq, false))
			}
		}
	})
}

// ---- Property 1/2/6 on the global kind: only fixture rows are asserted ----

func TestP11GlobalProducersFixtureOnce(t *testing.T) {
	f := newP11Fix(t)
	ctx := context.Background()
	entities := map[string]string{
		csvexport.Vendors: "vendor", csvexport.VendorBranches: "branches",
		csvexport.VendorVaults: "vaults", csvexport.VendorPICs: "pics",
	}
	for ent, model := range entities {
		for _, st := range []string{"active", "disabled", "all"} {
			p, err := ParseParams(KindMasterDataGlobal, ent, nil, nil, st, "")
			if err != nil {
				t.Fatal(err)
			}
			rows, err := p11Produce(ctx, f.q, p, time.Now())
			if err != nil {
				t.Fatalf("%s/%s: %v", ent, st, err)
			}
			ids := p11CellIDs(t, rows)
			if !slices.IsSorted(ids) {
				t.Fatalf("%s/%s: ids not ascending", ent, st)
			}
			for i := 1; i < len(ids); i++ {
				if ids[i] == ids[i-1] {
					t.Fatalf("%s/%s: duplicate id %d", ent, st, ids[i])
				}
			}
			want := f.expect(p11Q{entity: model, status: st}, false)
			fix := p11Set(f.ids(f.rows[model]))
			var got []int64
			for _, id := range ids {
				if fix[id] {
					got = append(got, id)
				}
			}
			if !slices.Equal(got, f.ids(want)) {
				t.Fatalf("%s/%s: fixture ids in global export %v, want %v", ent, st, got, f.ids(want))
			}
			again, _ := p11Produce(ctx, f.q, p, time.Now())
			if len(again) < len(rows) { // other tests may add rows concurrently; fewer would be a bug
				t.Fatalf("%s/%s: second run has fewer rows (%d < %d)", ent, st, len(again), len(rows))
			}
		}
	}
}

// ---- Property 7: bytes stored == bytes written, across chunk boundaries ----

func (f *p11Fix) newJob(hash string) int64 {
	return f.id(`INSERT INTO export_jobs (requested_by, kind, entity, vendor_id, vendor_code, status_filter, params_hash, file_name)
		VALUES ($1, 'vendor', 'vaults', $2, $3, 'all', $4, 'p11.csv') RETURNING id`,
		f.userID, f.vIDs[0], f.vcodes[0], f.tag+hash)
}

func p11Render(t *testing.T, pr Producer, rows [][]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := csvexport.NewWriter(&buf, pr.Header())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if err := w.Write(r); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func p11ReadAll(t *testing.T, svc *Service, jobID, chunks int64) ([]byte, [][]byte) {
	t.Helper()
	var all []byte
	var parts [][]byte
	for data, err := range svc.OpenChunks(context.Background(), jobID, chunks) {
		if err != nil {
			t.Fatalf("OpenChunks: %v", err)
		}
		parts = append(parts, data)
		all = append(all, data...)
	}
	return all, parts
}

func TestP11ChunksEndToEnd(t *testing.T) {
	f := newP11Fix(t)
	ctx := context.Background()
	svc := NewService(f.pool, audit.NewWriter(), nil)
	pr := DefaultProducers()[KindVendor]["pics"]
	// PICs carry quotes, commas, newlines and formula prefixes: worst case for CSV escaping.
	v := 0
	p := p11Params(t, KindVendor, p11Q{entity: "pics", vendor: &v, status: "all"}, f)
	rows, err := p11Produce(ctx, f.q, p, time.Now())
	if err != nil || len(rows) == 0 {
		t.Fatalf("produce pics: %d rows, err=%v", len(rows), err)
	}
	want := p11Render(t, pr, rows)

	// small random chunk sizes force many boundaries inside rows and inside quoted cells
	for i := range 25 {
		size := 5 + f.rng.IntN(300)
		jobID := f.newJob(fmt.Sprintf("c%d", i))
		cw := NewChunkWriter(ctx, QueriesSink{Q: f.q}, jobID)
		cw.chunkSize = size
		w, err := csvexport.NewWriter(cw, pr.Header())
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if err := w.Write(r); err != nil {
				t.Fatal(err)
			}
		}
		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}
		if err := cw.Close(); err != nil {
			t.Fatal(err)
		}
		got, parts := p11ReadAll(t, svc, jobID, int64(cw.Chunks()))
		if !bytes.Equal(got, want) {
			t.Fatalf("chunk size %d: stored file differs from written file (%d vs %d bytes)", size, len(got), len(want))
		}
		if cw.Bytes() != int64(len(want)) {
			t.Fatalf("chunk size %d: Bytes()=%d, want %d", size, cw.Bytes(), len(want))
		}
		for k, part := range parts {
			if len(part) > size || (k < len(parts)-1 && len(part) != size) {
				t.Fatalf("chunk size %d: chunk %d has %d bytes", size, k, len(part))
			}
		}
		if len(want) > size && len(parts) < 2 {
			t.Fatalf("chunk size %d: expected a multi-chunk file", size)
		}
		recs, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(got, []byte(csvexport.BOM)))).ReadAll()
		if err != nil || len(recs) != len(rows)+1 {
			t.Fatalf("chunk size %d: parsed %d records (err=%v), want %d", size, len(recs), err, len(rows)+1)
		}
		if err := f.q.DeleteExportJobChunks(ctx, jobID); err != nil {
			t.Fatal(err)
		}
	}

	// The real worker path (snapshot reader + ChunkWriter with production limits).
	for _, ent := range []string{"vaults", "pics", "branches", "dsr-location-maps"} {
		jobID := f.newJob("w" + ent)
		p := p11Params(t, KindVendor, p11Q{entity: ent, vendor: &v, status: "all"}, f)
		w := newWorker(pgStore{pool: f.pool, q: f.q}, pgReader{pool: f.pool}, DefaultProducers(), WorkerConfig{})
		st, err := w.produce(ctx, Job{ID: jobID, Params: p})
		if err != nil {
			t.Fatalf("worker.produce %s: %v", ent, err)
		}
		erows, err := p11Produce(ctx, f.q, p, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		ewant := p11Render(t, DefaultProducers()[KindVendor][ent], erows)
		got, _ := p11ReadAll(t, svc, jobID, int64(st.chunks))
		if !bytes.Equal(got, ewant) || st.rows != int64(len(erows)) || st.bytes != int64(len(ewant)) {
			t.Fatalf("worker.produce %s: stored %d bytes/%d rows (stats %d/%d), want %d/%d",
				ent, len(got), len(erows), st.bytes, st.rows, len(ewant), len(erows))
		}
	}

	// No rows: header-only file still has one chunk with BOM + header.
	jobID := f.newJob("empty")
	vv, bb := 1, 0 // vendor B x branch of vendor A -> nothing
	pe := p11Params(t, KindCabang, p11Q{entity: "vaults", vendor: &vv, branch: &bb, status: "all"}, f)
	w := newWorker(pgStore{pool: f.pool, q: f.q}, pgReader{pool: f.pool}, DefaultProducers(), WorkerConfig{})
	st, err := w.produce(ctx, Job{ID: jobID, Params: pe})
	if err != nil || st.rows != 0 || st.chunks != 1 {
		t.Fatalf("empty export: stats=%+v err=%v", st, err)
	}
	got, _ := p11ReadAll(t, svc, jobID, int64(st.chunks))
	if !bytes.Equal(got, p11Render(t, DefaultProducers()[KindCabang]["vaults"], nil)) {
		t.Fatalf("empty export is not BOM+header only: %q", got)
	}
}

// ---- Replica snapshot: REPEATABLE READ READ ONLY ----

// p11TxDB digs the pgx.Tx out of the Queries returned by pgReader.Begin: the field is
// unexported (sqlc) and the production code exposes no other handle on the transaction.
func p11TxDB(t *testing.T, q Querier) repository.DBTX {
	t.Helper()
	qs, ok := q.(*repository.Queries)
	if !ok {
		t.Fatalf("pgReader.Begin returned %T", q)
	}
	fld := reflect.ValueOf(qs).Elem().FieldByName("db")
	return reflect.NewAt(fld.Type(), unsafe.Pointer(fld.UnsafeAddr())).Elem().Interface().(repository.DBTX)
}

func TestP11SnapshotIsRepeatableReadReadOnly(t *testing.T) {
	f := newP11Fix(t)
	ctx := context.Background()
	q, rollback, err := pgReader{pool: f.pool}.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	released := false
	defer func() {
		if !released {
			rollback()
		}
	}()
	db := p11TxDB(t, q)

	var iso, ro string
	if err := db.QueryRow(ctx, `SHOW transaction_isolation`).Scan(&iso); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `SHOW transaction_read_only`).Scan(&ro); err != nil {
		t.Fatal(err)
	}
	if iso != "repeatable read" || ro != "on" {
		t.Fatalf("snapshot tx isolation=%q read_only=%q, want repeatable read / on", iso, ro)
	}

	// Writes are refused (SQLSTATE 25006 read_only_sql_transaction).
	_, err = db.Exec(ctx, `UPDATE vendors SET name = name WHERE code = $1`, f.vcodes[0])
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "25006" {
		t.Fatalf("write in snapshot tx: err=%v, want SQLSTATE 25006", err)
	}
	// The failed statement aborted the tx; reopen to test the snapshot itself.
	rollback()
	released = true

	q, rollback, err = pgReader{pool: f.pool}.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer rollback()
	db = p11TxDB(t, q)
	count := func(d repository.DBTX) int {
		var n int
		if err := d.QueryRow(ctx, `SELECT count(*) FROM vendor_branches WHERE branch_code LIKE $1`, f.tag+"%").Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := count(db)
	f.id(`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name) VALUES ($1, $2, $2) RETURNING id`, f.vIDs[0], f.tag+"BLATE")
	if inTx, outside := count(db), count(f.pool); inTx != before || outside != before+1 {
		t.Fatalf("snapshot not stable: in tx %d (want %d), outside %d (want %d)", inTx, before, outside, before+1)
	}
	// producers read through the same snapshot: the late branch is invisible to them.
	rows, err := p11Produce(ctx, q, p11Params(t, KindVendor, p11Q{entity: "branches", vendor: new(int), status: "all"}, f), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r[2] == f.tag+"BLATE" {
			t.Fatal("producer in snapshot tx saw a row committed after the snapshot")
		}
	}
}
