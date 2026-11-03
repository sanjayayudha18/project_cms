//go:build integration

package exportjob

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend-exq/internal/audit"
	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
)

// report-export-migration 6.1 (Properties 1, 7, 10) on real Postgres. The four report producers run
// through the real worker path (Service.Create -> worker.produce: REPEATABLE READ READ ONLY snapshot,
// ChunkWriter, csvexport.Writer) over a fixture whose codes all start with T61_. Every call is filtered to
// a fixture vendor; real dev rows are never read into the output, modified, or deleted. Everything the
// test inserts (report tables, export_jobs, audit_logs, users) is deleted in t.Cleanup, and a last
// cleanup step fails the test if anything with the T61_ marker is left.

const (
	t61Like = `T61\_%`
	bom     = "\xef\xbb\xbf"

	t61VendorName = "=T61 Vendor Ü" // leading '=' -> CSVSafe quote; non-ASCII
	t61VendorCell = "'=T61 Vendor Ü"

	t61DailyHeader   = "report_date,vendor_code,vendor,branch_code,branch_name,region_code,deadline_wib,first_received_at_wib,status,late_minutes,receipt_checksum,matched_locations\n"
	t61SummaryHeader = "vendor_code,vendor,branch_code,branch_name,region_code,obliged_days,ok_days,late_days,not_sent_days,pending_days,late_minutes_total\n"
	t61MismatchHdr   = "report_date,ticket_number,request_number,terminal_id,vendor,branch_code,region_code,scheduled_date,realized_date,classification,workday_offset,denom,order_amount,itm_amount,vendor_amount,diff,currency\n"
	t61TripsHeader   = "vendor_id,vendor,terminal_id,trips,first_date,last_date\n"
)

var (
	t61Wib   = time.FixedZone("WIB", 7*3600)
	t61AsOf  = time.Date(2026, 6, 4, 8, 0, 0, 0, t61Wib) // day 4 is still before its 09:00 deadline: pending
	t61Start = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
)

type t61Fix struct {
	t      *testing.T
	pool   *pgxpool.Pool
	svc    *Service
	userID int64
	v1, v2 int64
}

func (f *t61Fix) id(sql string, args ...any) int64 {
	f.t.Helper()
	var id int64
	if err := f.pool.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		f.t.Fatalf("t61 fixture insert: %v\n%s", err, sql)
	}
	return id
}

func (f *t61Fix) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.pool.Exec(context.Background(), sql, args...); err != nil {
		f.t.Fatalf("t61 fixture exec: %v\n%s", err, sql)
	}
}

func (f *t61Fix) cleanup() {
	ctx := context.Background()
	steps := []string{
		`DELETE FROM export_job_chunks WHERE export_job_id IN (SELECT id FROM export_jobs WHERE requested_by IN {USERS})`,
		`DELETE FROM export_jobs WHERE requested_by IN {USERS}`,
		`DELETE FROM audit_logs WHERE actor_id IN {USERS}`,
		`DELETE FROM replenish_classifications WHERE terminal_id LIKE {LIKE}`, // amounts cascade
		`DELETE FROM vendor_request_tickets WHERE request_number LIKE {LIKE}`,
		`DELETE FROM vendor_requests WHERE request_number LIKE {LIKE}`,
		`DELETE FROM dsr_receipts WHERE vendor_id IN (SELECT id FROM vendors WHERE code LIKE {LIKE})`,
		`DELETE FROM dsr_location_vault_maps WHERE vendor_id IN (SELECT id FROM vendors WHERE code LIKE {LIKE})`,
		`DELETE FROM vendor_vaults WHERE vault_code LIKE {LIKE}`,
		`DELETE FROM vendor_branches WHERE branch_code LIKE {LIKE}`,
		`DELETE FROM vendors WHERE code LIKE {LIKE}`,
		`DELETE FROM users WHERE username LIKE {LIKE}`, // last: vendor_requests.created_by restricts it
	}
	for _, s := range steps {
		s = f.bind(s)
		if _, err := f.pool.Exec(ctx, s); err != nil {
			f.t.Errorf("t61 cleanup: %v\n%s", err, s)
		}
	}
}

// bind inlines the fixture marker and user id (constants of this test, never external input).
func (f *t61Fix) bind(sql string) string {
	return strings.NewReplacer("{LIKE}", `'`+t61Like+`'`, "{USERS}", `(SELECT id FROM users WHERE username LIKE '`+t61Like+`')`).Replace(sql)
}

// assertNoLeftovers runs after cleanup (registered before it, so it runs last).
func (f *t61Fix) assertNoLeftovers() {
	ctx := context.Background()
	queries := []string{
		`SELECT count(*) FROM vendors WHERE code LIKE {LIKE}`,
		`SELECT count(*) FROM vendor_branches WHERE branch_code LIKE {LIKE}`,
		`SELECT count(*) FROM vendor_vaults WHERE vault_code LIKE {LIKE}`,
		`SELECT count(*) FROM dsr_receipts WHERE vendor_label LIKE '%T61 %'`,
		`SELECT count(*) FROM replenish_classifications WHERE terminal_id LIKE {LIKE}`,
		`SELECT count(*) FROM vendor_request_tickets WHERE request_number LIKE {LIKE}`,
		`SELECT count(*) FROM vendor_requests WHERE request_number LIKE {LIKE}`,
		`SELECT count(*) FROM users WHERE username LIKE {LIKE}`,
		`SELECT count(*) FROM audit_logs WHERE actor_id = ` + fmt.Sprint(f.userID),
		`SELECT count(*) FROM export_jobs WHERE requested_by = ` + fmt.Sprint(f.userID),
	}
	total := 0
	for _, q := range queries {
		q = f.bind(q)
		var n int
		if err := f.pool.QueryRow(ctx, q).Scan(&n); err != nil {
			f.t.Errorf("t61 leftover query: %v\n%s", err, q)
		}
		if n != 0 {
			f.t.Errorf("t61 leftover rows: %d from %s", n, q)
		}
		total += n
	}
	f.t.Logf("t61 leftover rows after cleanup: %d", total)
}

// at returns a view of the fixture that reports failures on t (a subtest); cleanup keeps the parent.
func (f *t61Fix) at(t *testing.T) *t61Fix { g := *f; g.t = t; return &g }

func cs(c string) string { return strings.Repeat(c, 64) }

func newT61Fix(t *testing.T) *t61Fix {
	t.Helper()
	f := &t61Fix{t: t, pool: p11Pool(t)} // pool.Close is registered first, so it runs last
	f.svc = NewService(f.pool, audit.NewWriter(), nil)
	// Leftover check registered before the fixture cleanup: runs after it.
	t.Cleanup(f.assertNoLeftovers)
	t.Cleanup(f.cleanup)
	f.cleanup() // stale rows of a crashed earlier run; userID 0 matches nothing
	f.seed()
	return f
}

func (f *t61Fix) seed() {
	f.userID = f.id(`INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source)
		SELECT id, 'T61_u', 'T61_u', 'T61_u@test.local', true, 'ldap' FROM roles WHERE role = 'ADMIN' RETURNING id`)
	f.seedDSR()
	f.seedReplenish()
	f.seedSnapshotVendor()
}

// seedDSR: vendor V1 with branch A (2 mapped vaults, region MDN) and B (1 vault, no region); 4 report days
// 2026-06-01..04 as of t61AsOf. Expected statuses:
//
//	day  A           B
//	 1   ok          not_sent
//	 2   late (75m)  ok (09:00:00.5: K1, whole seconds)
//	 3   not_sent    ok (label matched case-insensitively)
//	 4   pending     pending
func (f *t61Fix) seedDSR() {
	f.v1 = f.id(`INSERT INTO vendors (code, name, created_at) VALUES ('T61_V1', $1, '2026-01-01') RETURNING id`, t61VendorName)
	branch := func(code, name string, region *string) int64 {
		return f.id(`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, region_code, created_at)
			VALUES ($1, $2, $3, $4, '2026-01-01') RETURNING id`, f.v1, code, name, region)
	}
	mdn := "MDN"
	a := branch("T61_BA", "+Cabang Ñ", &mdn)
	b := branch("T61_BB", `Cab, "Koma"`, nil)
	vault := func(branchID int64, code, label string) {
		vid := f.id(`INSERT INTO vendor_vaults (vendor_branch_id, vault_code, type, category, currency_code, created_at)
			VALUES ($1, $2, 'CASH', 'CASH', 'IDR', '2026-01-01') RETURNING id`, branchID, code)
		f.exec(`INSERT INTO dsr_location_vault_maps (vendor_id, dsr_location, vendor_vault_id) VALUES ($1, $2, $3)`, f.v1, label, vid)
	}
	vault(a, "T61_VA1", "-Vault Minus")
	vault(a, "T61_VA2", "Vault 雅加达")
	vault(b, "T61_VB1", "Gudang B")
	receipt := func(day, checksum, receivedAt string, labels ...string) {
		f.exec(`INSERT INTO dsr_receipts (vendor_label, vendor_id, report_date, checksum, locations, received_at)
			VALUES ($1, $2, $3::date, $4, $5, $6::timestamptz)`, t61VendorName, f.v1, day, checksum, labels, receivedAt)
	}
	receipt("2026-06-01", cs("a"), "2026-06-01 08:41:00+07", "-Vault Minus", "Vault 雅加达")
	receipt("2026-06-02", cs("b"), "2026-06-02 10:15:00+07", "-Vault Minus")
	receipt("2026-06-02", cs("c"), "2026-06-02 09:00:00.500+07", "Gudang B")
	receipt("2026-06-03", cs("d"), "2026-06-03 08:00:00+07", "gudang b")
}

// seedReplenish: classifications of V1 (terminals T61_ATM1..3), June 2026. Amounts diff = itm - order
// (generated column). c1 late+diff, c2 no_order+diff, c3 void+diff (in mismatch, not in trips),
// c4 on_schedule without diff, c5 pending with diff (not final: left out).
func (f *t61Fix) seedReplenish() {
	var branchA int64
	if err := f.pool.QueryRow(context.Background(), `SELECT id FROM vendor_branches WHERE branch_code = 'T61_BA'`).Scan(&branchA); err != nil {
		f.t.Fatal(err)
	}
	// One active ticket per (request, ATM): the second ticket of ATM1 needs its own request.
	f.exec(`INSERT INTO vendor_requests (request_number, forecast_date, created_by) VALUES ('T61_REQ1', '2026-06-01', $1), ('T61_REQ2', '2026-06-01', $1)`, f.userID)
	ticket := func(req, terminal, denom, date string, seq int) int64 {
		return f.id(`INSERT INTO vendor_request_tickets (request_number, terminal_id, denom_code, replenish_date, seq, ticket_number)
			VALUES ($5, $1, $2, $3::date, $4::smallint, $1::text || '_' || $2::text || '_' || to_char($3::date, 'YYYYMMDD') || '_' || lpad($4::smallint::text, 3, '0'))
			RETURNING id`, terminal, denom, date, seq, req)
	}
	class := func(ticketID *int64, terminal string, scheduled, realized *string, offset *int, class string, final, diff bool, branch *int64) int64 {
		return f.id(`INSERT INTO replenish_classifications (ticket_id, terminal_id, vendor_id, vendor_branch_id, scheduled_date, realized_date,
				workday_offset, classification, is_final, has_amount_diff, finalized_at)
			VALUES ($1, $2, $3, $4, $5::date, $6::date, $7, $8, $9, $10, CASE WHEN $9::bool THEN now() END) RETURNING id`,
			ticketID, terminal, f.v1, branch, scheduled, realized, offset, class, final, diff)
	}
	amount := func(id int64, denom int, order, itm int64, vendor *int64) {
		f.exec(`INSERT INTO replenish_classification_amounts (classification_id, denom, order_amount, itm_amount, vendor_amount)
			VALUES ($1, $2, $3, $4, $5)`, id, denom, order, itm, vendor)
	}
	s := func(v string) *string { return &v }
	i64p := func(v int64) *int64 { return &v }
	one, zero := 1, 0
	t1 := ticket("T61_REQ1", "T61_ATM1", "100K", "2026-06-05", 1)
	t3 := ticket("T61_REQ1", "T61_ATM3", "MIX", "2026-06-08", 1)
	t4 := ticket("T61_REQ2", "T61_ATM1", "50K", "2026-06-09", 2)
	t5 := ticket("T61_REQ1", "T61_ATM2", "50K", "2026-06-10", 1)

	c1 := class(&t1, "T61_ATM1", s("2026-06-05"), s("2026-06-06"), &one, "late", true, true, &branchA)
	amount(c1, 50000, 200000000, 150000000, i64p(150000000))
	amount(c1, 100000, 300000000, 300000000, nil)
	c2 := class(nil, "T61_ATM2", nil, s("2026-06-07"), nil, "no_order", true, true, nil)
	amount(c2, 100000, 0, 100000000, nil)
	c3 := class(&t3, "T61_ATM3", s("2026-06-08"), s("2026-06-08"), nil, "void", true, true, &branchA)
	amount(c3, 50000, 50000000, 0, i64p(0))
	c4 := class(&t4, "T61_ATM1", s("2026-06-09"), s("2026-06-09"), &zero, "on_schedule", true, false, &branchA)
	amount(c4, 100000, 1000000, 1000000, nil)
	c5 := class(&t5, "T61_ATM2", s("2026-06-10"), nil, nil, "pending", false, true, &branchA)
	amount(c5, 50000, 5000000, 0, nil)
}

// seedSnapshotVendor: V2 with 40 branches (1 mapped vault each, no receipts) so a June 2026 daily export
// has 30 x 40 = 1200 rows: two pages of PageSize.
func (f *t61Fix) seedSnapshotVendor() {
	f.v2 = f.id(`INSERT INTO vendors (code, name, created_at) VALUES ('T61_V2', 'T61 Snapshot', '2026-01-01') RETURNING id`)
	f.snapBranches(1, 40)
}

// snapBranches adds branches T61_S<lo>..T61_S<hi> with a vault and a location map each.
func (f *t61Fix) snapBranches(lo, hi int) {
	f.t.Helper()
	f.exec(`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, created_at)
		SELECT $1, 'T61_S' || lpad(i::text, 3, '0'), 'T61 Snap ' || i, '2026-01-01' FROM generate_series($2::int, $3::int) i`, f.v2, lo, hi)
	f.exec(`INSERT INTO vendor_vaults (vendor_branch_id, vault_code, type, category, currency_code, created_at)
		SELECT id, 'T61_SV' || substr(branch_code, 6), 'CASH', 'CASH', 'IDR', '2026-01-01'
		FROM vendor_branches WHERE vendor_id = $1 AND branch_code >= 'T61_S' || lpad($2::int::text, 3, '0') AND branch_code <= 'T61_S' || lpad($3::int::text, 3, '0')`, f.v2, lo, hi)
	f.exec(`INSERT INTO dsr_location_vault_maps (vendor_id, dsr_location, vendor_vault_id)
		SELECT $1, 'T61_sloc' || v.vault_code, v.id FROM vendor_vaults v JOIN vendor_branches b ON b.id = v.vendor_branch_id
		WHERE b.vendor_id = $1 AND b.branch_code >= 'T61_S' || lpad($2::int::text, 3, '0') AND b.branch_code <= 'T61_S' || lpad($3::int::text, 3, '0')`, f.v2, lo, hi)
}

// run creates the job through the service (real validation, file name, audit), produces it with a worker
// whose clock is asOf and whose DSR start date is start, and returns the stored file and the job's file name.
// The job rows are removed right away (MaxActivePerUser); audit rows go with the final cleanup.
func (f *t61Fix) run(kind Kind, q url.Values, start *time.Time, asOf time.Time, reader snapshotReader) (name, file string) {
	f.t.Helper()
	ctx := context.Background()
	p, err := ParseReportParams(kind, q, time.Now())
	if err != nil {
		f.t.Fatalf("ParseReportParams(%s, %v): %v", kind, q, err)
	}
	job, _, err := f.svc.Create(ctx, Actor{UserID: f.userID, Username: "T61_u", Role: "ATM-SPV"}, p)
	if err != nil {
		f.t.Fatalf("Create(%s): %v", kind, err)
	}
	defer func() {
		f.exec(`DELETE FROM export_job_chunks WHERE export_job_id = $1`, job.ID)
		f.exec(`DELETE FROM export_jobs WHERE id = $1`, job.ID)
	}()
	// The worker's progress update only matches a running row (what ClaimNext would have made it).
	f.exec(`UPDATE export_jobs SET status = 'running', started_at = now() WHERE id = $1`, job.ID)
	if reader == nil {
		reader = pgReader{pool: f.pool}
	}
	w := newWorker(pgStore{pool: f.pool, q: repository.New(f.pool)}, reader, NewProducers(start), WorkerConfig{})
	w.now = func() time.Time { return asOf }
	st, err := w.produce(ctx, job)
	if err != nil {
		f.t.Fatalf("produce(%s): %v", kind, err)
	}
	data, _ := p11ReadAll(f.t, f.svc, job.ID, int64(st.chunks))
	return job.FileName, string(data)
}

func vals(kv ...string) url.Values {
	v := url.Values{}
	for i := 0; i < len(kv); i += 2 {
		v.Set(kv[i], kv[i+1])
	}
	return v
}

// Expected CSV lines of the DSR fixture (see seedDSR). V = vendor cell, A/B = branch cells.
const (
	t61A = "T61_V1," + t61VendorCell + ",T61_BA,'+Cabang Ñ,MDN,"
	t61B = "T61_V1," + t61VendorCell + `,T61_BB,"Cab, ""Koma""",,`
)

func dsrDailyLines(days ...int) string {
	rows := map[int]string{
		1: "2026-06-01," + t61A + "2026-06-01 09:00:00,2026-06-01 08:41:00,ok,0," + cs("a") + ",'-Vault Minus; Vault 雅加达\n" +
			"2026-06-01," + t61B + "2026-06-01 09:00:00,,not_sent,0,,\n",
		2: "2026-06-02," + t61A + "2026-06-02 09:00:00,2026-06-02 10:15:00,late,75," + cs("b") + ",'-Vault Minus\n" +
			"2026-06-02," + t61B + "2026-06-02 09:00:00,2026-06-02 09:00:00,ok,0," + cs("c") + ",Gudang B\n",
		3: "2026-06-03," + t61A + "2026-06-03 09:00:00,,not_sent,0,,\n" +
			"2026-06-03," + t61B + "2026-06-03 09:00:00,2026-06-03 08:00:00,ok,0," + cs("d") + ",gudang b\n",
		4: "2026-06-04," + t61A + "2026-06-04 09:00:00,,pending,0,,\n" +
			"2026-06-04," + t61B + "2026-06-04 09:00:00,,pending,0,,\n",
	}
	var b strings.Builder
	for _, d := range days {
		b.WriteString(rows[d])
	}
	return b.String()
}

func t61Mismatch() string {
	const c1 = "2026-06-05,T61_ATM1_100K_20260605_001,T61_REQ1,T61_ATM1," + t61VendorCell + ",T61_BA,MDN,2026-06-05,2026-06-06,late,1,"
	return c1 + "100000,300000000,300000000,,0,IDR\n" +
		c1 + "50000,200000000,150000000,150000000,-50000000,IDR\n" +
		"2026-06-07,,,T61_ATM2," + t61VendorCell + ",,,,2026-06-07,no_order,,100000,0,100000000,,100000000,IDR\n" +
		"2026-06-08,T61_ATM3_MIX_20260608_001,T61_REQ1,T61_ATM3," + t61VendorCell + ",T61_BA,MDN,2026-06-08,2026-06-08,void,,50000,50000000,0,0,-50000000,IDR\n"
}

func TestReportProducersIntegration(t *testing.T) {
	f := newT61Fix(t)
	start := t61Start
	month := vals("month", "2026-06", "vendor_id", fmt.Sprint(f.v1))
	withStatus := vals("month", "2026-06", "vendor_id", fmt.Sprint(f.v1), "status", "late")
	repl := func(kv ...string) url.Values {
		return vals(append([]string{"date_from", "2026-06-01", "date_to", "2026-06-30", "vendor_id", fmt.Sprint(f.v1)}, kv...)...)
	}
	const mismatchName = "replenishment-amount-mismatch-2026-06-01-2026-06-30.csv"

	t.Run("Property1_file_equals_legacy_export", func(t *testing.T) {
		cases := []struct {
			name     string
			kind     Kind
			q        url.Values
			wantName string
			want     string
		}{
			{"daily", KindDsrLateDaily, month, "dsr-late-daily-2026-06.csv", t61DailyHeader + dsrDailyLines(1, 2, 3, 4)},
			{"daily_status_late", KindDsrLateDaily, withStatus, "dsr-late-daily-2026-06.csv",
				t61DailyHeader + "2026-06-02," + t61A + "2026-06-02 09:00:00,2026-06-02 10:15:00,late,75," + cs("b") + ",'-Vault Minus\n"},
			{"summary", KindDsrLateSummary, month, "dsr-late-summary-2026-06.csv", t61SummaryHeader +
				"T61_V1," + t61VendorCell + ",T61_BA,'+Cabang Ñ,MDN,4,1,1,1,1,75\n" +
				"T61_V1," + t61VendorCell + `,T61_BB,"Cab, ""Koma""",,4,2,0,1,1,0` + "\n"},
			{"amount_mismatch", KindReplenishmentAmountMismatch, repl(), mismatchName, t61MismatchHdr + t61Mismatch()},
			{"amount_mismatch_class_late", KindReplenishmentAmountMismatch, repl("classification", "late"), mismatchName,
				t61MismatchHdr + strings.SplitAfterN(t61Mismatch(), "\n", 3)[0] + strings.SplitAfterN(t61Mismatch(), "\n", 3)[1]},
			{"trips", KindReplenishmentTrips, repl(), "replenishment-trips-2026-06-01-2026-06-30.csv", t61TripsHeader +
				fmt.Sprintf("%d,%s,T61_ATM1,2,2026-06-06,2026-06-09\n", f.v1, t61VendorCell) +
				fmt.Sprintf("%d,%s,T61_ATM2,1,2026-06-07,2026-06-07\n", f.v1, t61VendorCell)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				f := f.at(t)
				name, got := f.run(tc.kind, tc.q, &start, t61AsOf, nil)
				if name != tc.wantName {
					t.Errorf("job file name = %q, want %q", name, tc.wantName)
				}
				if want := bom + tc.want; got != want {
					t.Errorf("file differs\n--- got ---\n%s\n--- want ---\n%s", got, want)
				}
			})
		}
	})

	t.Run("Property7_dsr_start_date_gate", func(t *testing.T) {
		later := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
		afterAll := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
		cases := []struct {
			name  string
			kind  Kind
			start *time.Time
			want  string
		}{
			{"daily_nil", KindDsrLateDaily, nil, t61DailyHeader},
			{"summary_nil", KindDsrLateSummary, nil, t61SummaryHeader},
			{"daily_after_all_days", KindDsrLateDaily, &afterAll, t61DailyHeader},
			{"summary_after_all_days", KindDsrLateSummary, &afterAll, t61SummaryHeader},
			{"daily_from_day3", KindDsrLateDaily, &later, t61DailyHeader + dsrDailyLines(3, 4)},
			{"summary_from_day3", KindDsrLateSummary, &later, t61SummaryHeader +
				"T61_V1," + t61VendorCell + ",T61_BA,'+Cabang Ñ,MDN,2,0,0,1,1,0\n" +
				"T61_V1," + t61VendorCell + `,T61_BB,"Cab, ""Koma""",,2,1,0,0,1,0` + "\n"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				f := f.at(t)
				_, got := f.run(tc.kind, month, tc.start, t61AsOf, nil)
				if want := bom + tc.want; got != want {
					t.Errorf("file differs\n--- got ---\n%q\n--- want ---\n%q", got, want)
				}
			})
		}
	})

	t.Run("Property10_snapshot_survives_writes_between_pages", func(t *testing.T) {
		f := f.at(t)
		snap := vals("month", "2026-06", "vendor_id", fmt.Sprint(f.v2))
		asOf := time.Date(2026, 6, 30, 12, 0, 0, 0, t61Wib)
		_, base := f.run(KindDsrLateDaily, snap, &start, asOf, nil)
		if n := strings.Count(base, "\n") - 1; n != 1200 || n <= PageSize {
			t.Fatalf("baseline has %d data rows, want 1200 (more than one page of %d)", n, PageSize)
		}

		hook := func() {
			// Committed from another connection after page 1 was read: a new branch (shifts every OFFSET),
			// a renamed branch on page 2, and a receipt that turns a not_sent day into ok.
			f.snapBranches(999, 999)
			f.exec(`UPDATE vendor_branches SET branch_name = 'T61 renamed' WHERE branch_code = 'T61_S040'`)
			f.exec(`INSERT INTO dsr_receipts (vendor_label, vendor_id, report_date, checksum, locations, received_at)
				SELECT 'T61 Snapshot', $1, '2026-06-30', $2, ARRAY[m.dsr_location], '2026-06-30 08:00:00+07'::timestamptz
				FROM dsr_location_vault_maps m JOIN vendor_vaults v ON v.id = m.vendor_vault_id WHERE v.vault_code = 'T61_SV040'`, f.v2, cs("e"))
		}
		hooked := &hookReader{inner: pgReader{pool: f.pool}, hook: hook}
		_, got := f.run(KindDsrLateDaily, snap, &start, asOf, hooked)
		if !hooked.fired {
			t.Fatal("hook never ran: the export did not span a second page")
		}
		if got != base {
			t.Errorf("export changed by writes between pages: %d bytes, baseline %d", len(got), len(base))
		}
		// The writes are real and visible outside the snapshot.
		_, after := f.run(KindDsrLateDaily, snap, &start, asOf, nil)
		if after == base || !strings.Contains(after, "T61 renamed") || !strings.Contains(after, "T61_S999") {
			t.Error("a fresh export does not show the concurrent writes: the hook did not change visible data")
		}
	})
}

// hookReader runs hook once, right after the first daily page was read inside the snapshot.
type hookReader struct {
	inner snapshotReader
	hook  func()
	fired bool
}

func (h *hookReader) Begin(ctx context.Context) (Querier, func(), error) {
	q, rollback, err := h.inner.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	return &hookQuerier{Querier: q, h: h}, rollback, nil
}

type hookQuerier struct {
	Querier
	h *hookReader
}

func (q *hookQuerier) ListDsrLateDaily(ctx context.Context, a repository.ListDsrLateDailyParams) ([]repository.ListDsrLateDailyRow, error) {
	rows, err := q.Querier.ListDsrLateDaily(ctx, a)
	if err == nil && a.PageOffset == 0 && !q.h.fired {
		q.h.fired = true
		q.h.hook()
	}
	return rows, err
}
