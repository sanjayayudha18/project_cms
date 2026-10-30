package exportjob

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
	"github.com/cimb-niaga/cms/pkg/csvexport"
)

// Golden files of the legacy report exports (written without BOM; the async export adds it).
const reportGoldenDir = "../../../backend/internal/handler/testdata/report_csv"

var testWIB = time.FixedZone("WIB", 7*3600)

func tsAt(d, h, m int) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: time.Date(2026, 10, d, h, m, 0, 0, testWIB), Valid: true}
}

func dayAt(d int) pgtype.Date {
	return pgtype.Date{Time: time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC), Valid: true}
}

// reportQ serves fixed rows; it applies the status / classification filter the way the SQL does.
type reportQ struct {
	*fakeQ
	daily     []repository.ListDsrLateDailyRow
	summary   []repository.ListDsrLateSummaryRow
	classes   []repository.ListReplenishClassificationsRow
	amounts   map[int64][]repository.ReplenishClassificationAmount
	trips     []repository.ReportReplenishTripsRow
	dailyArgs []repository.ListDsrLateDailyParams
	classArgs []repository.ListReplenishClassificationsParams
	amountIDs [][]int64
	queries   int
}

func window[R any](rows []R, offset, limit int64) []R {
	if offset >= int64(len(rows)) {
		return nil
	}
	return rows[offset:min(int64(len(rows)), offset+limit)]
}

func (q *reportQ) ListDsrLateDaily(_ context.Context, a repository.ListDsrLateDailyParams) ([]repository.ListDsrLateDailyRow, error) {
	q.queries++
	q.dailyArgs = append(q.dailyArgs, a)
	var rows []repository.ListDsrLateDailyRow
	for _, r := range q.daily {
		if a.Status == nil || *a.Status == r.Status {
			rows = append(rows, r)
		}
	}
	return window(rows, a.PageOffset, *a.PageLimit), nil
}

func (q *reportQ) ListDsrLateSummary(context.Context, repository.ListDsrLateSummaryParams) ([]repository.ListDsrLateSummaryRow, error) {
	q.queries++
	return q.summary, nil
}

func (q *reportQ) ListReplenishClassifications(_ context.Context, a repository.ListReplenishClassificationsParams) ([]repository.ListReplenishClassificationsRow, error) {
	q.queries++
	q.classArgs = append(q.classArgs, a)
	var rows []repository.ListReplenishClassificationsRow
	for _, r := range q.classes {
		if a.Classification == nil || *a.Classification == r.Classification {
			rows = append(rows, r)
		}
	}
	return window(rows, a.PageOffset, a.PageLimit), nil
}

func (q *reportQ) ListReplenishClassificationAmounts(_ context.Context, ids []int64) ([]repository.ReplenishClassificationAmount, error) {
	q.amountIDs = append(q.amountIDs, ids)
	var out []repository.ReplenishClassificationAmount
	for _, id := range ids {
		out = append(out, q.amounts[id]...)
	}
	return out, nil
}

func (q *reportQ) ReportReplenishTrips(context.Context, repository.ReportReplenishTripsParams) ([]repository.ReportReplenishTripsRow, error) {
	q.queries++
	return q.trips, nil
}

func goldenReportQ() *reportQ {
	i16, i64p := int16(1), func(v int64) *int64 { return &v }
	return &reportQ{
		fakeQ: &fakeQ{},
		daily: []repository.ListDsrLateDailyRow{
			{ReportDate: dayAt(1), VendorCode: "ABACUS", VendorName: "PT Abacus Cash Solution", BranchCode: "ABACUS_004",
				BranchName: "Medan – Jl. Gatot Subroto Ñ", RegionCode: sp("MDN"), Deadline: tsAt(1, 9, 0), FirstReceivedAt: tsAt(1, 8, 41),
				Status: "ok", ReceiptChecksum: "a1b2c3", MatchedLocations: []string{"Gudang Medan"}},
			{ReportDate: dayAt(1), VendorCode: "KEJAR", VendorName: `=HYPERLINK("x")`, BranchCode: "KJR_01", BranchName: "+Cabang Plus",
				RegionCode: sp("JKT"), Deadline: tsAt(1, 9, 0), FirstReceivedAt: tsAt(1, 10, 15), Status: "late", LateMinutes: 75,
				ReceiptChecksum: "d4e5f6", MatchedLocations: []string{"-Vault Minus", "Vault 雅加达"}},
			{ReportDate: dayAt(2), VendorCode: "KEJAR", VendorName: `=HYPERLINK("x")`, BranchCode: "KJR_02", BranchName: `Cab, "Koma"`,
				Deadline: tsAt(2, 9, 0), Status: "not_sent"},
			{ReportDate: dayAt(3), VendorCode: "ABACUS", VendorName: "PT Abacus Cash Solution", BranchCode: "ABACUS_004",
				BranchName: "Medan – Jl. Gatot Subroto Ñ", RegionCode: sp("MDN"), Deadline: tsAt(3, 9, 0), Status: "pending"},
		},
		summary: []repository.ListDsrLateSummaryRow{
			{VendorCode: "ABACUS", VendorName: "PT Abacus Cash Solution", BranchCode: "ABACUS_004", BranchName: "Medan – Jl. Gatot Subroto Ñ",
				RegionCode: sp("MDN"), ObligedDays: 3, OkDays: 1, PendingDays: 1},
			{VendorCode: "KEJAR", VendorName: `=HYPERLINK("x")`, BranchCode: "KJR_01", BranchName: "+Cabang Plus",
				RegionCode: sp("JKT"), ObligedDays: 2, LateDays: 1, NotSentDays: 1, LateMinutesTotal: 75},
		},
		classes: []repository.ListReplenishClassificationsRow{
			{ID: 1, TicketNumber: sp("ATM001_100K_20261005_001"), RequestNumber: sp("REP-ABC-MDN-20261005-001"), TerminalID: "ATM001",
				VendorName: sp("PT Abacus Cash Solution"), BranchCode: sp("ABACUS_004"), RegionCode: sp("MDN"),
				ScheduledDate: dayAt(5), RealizedDate: dayAt(6), WorkdayOffset: &i16, Classification: "late"},
			{ID: 2, TerminalID: "-ATM002", VendorName: sp("Vendor Ü"), RealizedDate: dayAt(7), Classification: "no_order"},
			{ID: 3, TicketNumber: sp("ATM003_MIX_20261008_002"), RequestNumber: sp("REP-KJR-JKT-20261008-001"), TerminalID: "ATM003",
				VendorName: sp(`=HYPERLINK("x")`), BranchCode: sp("KJR_01"), RegionCode: sp("JKT"), ScheduledDate: dayAt(8), Classification: "void"},
		},
		amounts: map[int64][]repository.ReplenishClassificationAmount{
			1: {{ClassificationID: 1, Denom: 50000, OrderAmount: 200000000, ItmAmount: 150000000, VendorAmount: i64p(150000000), Diff: i64p(-50000000)},
				{ClassificationID: 1, Denom: 100000, OrderAmount: 300000000, ItmAmount: 300000000}},
			2: {{ClassificationID: 2, Denom: 100000, ItmAmount: 100000000, Diff: i64p(100000000)}},
			3: {{ClassificationID: 3, Denom: 50000, OrderAmount: 50000000, VendorAmount: i64p(0), Diff: i64p(-50000000)}},
		},
		trips: []repository.ReportReplenishTripsRow{
			{VendorID: i64p(12), VendorName: sp("PT Abacus Cash Solution"), TerminalID: "ATM001", Trips: 3, FirstDate: dayAt(1), LastDate: dayAt(20)},
			{VendorName: sp("+Vendor Ü"), TerminalID: "ATM002", Trips: 1, FirstDate: dayAt(7), LastDate: dayAt(7)},
		},
	}
}

var dsrStart = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

func reportJob(kind Kind, r ReportParams) Params {
	return Params{Kind: kind, Entity: reportEntities[kind], Report: &r}
}

// renderReport runs a producer the way the worker does: csvexport.Writer over the header, emit per row.
func renderReport(t *testing.T, start *time.Time, q Querier, p Params) (string, error) {
	t.Helper()
	pr := NewProducers(start)[p.Kind][p.Entity]
	if pr == nil {
		t.Fatalf("no producer for %s/%s", p.Kind, p.Entity)
	}
	var buf bytes.Buffer
	w, err := csvexport.NewWriter(&buf, pr.Header())
	if err != nil {
		t.Fatal(err)
	}
	if err := pr.Produce(context.Background(), q, p, time.Date(2026, 10, 10, 6, 0, 0, 0, testWIB), w.Write); err != nil {
		return "", err
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	return buf.String(), nil
}

func TestReportProducersMatchLegacyGolden(t *testing.T) {
	month, from, to := "2026-10", "2026-10-01", "2026-10-31"
	late := "late"
	cases := []struct {
		golden string
		p      Params
	}{
		{"dsr_late_daily", reportJob(KindDsrLateDaily, ReportParams{Month: &month})},
		{"dsr_late_daily_status_late", reportJob(KindDsrLateDaily, ReportParams{Month: &month, Status: &late})},
		{"dsr_late_summary", reportJob(KindDsrLateSummary, ReportParams{Month: &month})},
		{"amount_mismatch", reportJob(KindReplenishmentAmountMismatch, ReportParams{DateFrom: &from, DateTo: &to})},
		{"amount_mismatch_class_late", reportJob(KindReplenishmentAmountMismatch, ReportParams{DateFrom: &from, DateTo: &to, Classification: &late})},
		{"trips", reportJob(KindReplenishmentTrips, ReportParams{DateFrom: &from, DateTo: &to, RegionCode: sp("MDN")})},
	}
	for _, tc := range cases {
		t.Run(tc.golden, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join(reportGoldenDir, tc.golden+".csv"))
			if err != nil {
				t.Fatal(err)
			}
			got, err := renderReport(t, &dsrStart, goldenReportQ(), tc.p)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(got, "\xef\xbb\xbf") {
				t.Error("missing BOM")
			}
			if got = strings.TrimPrefix(got, "\xef\xbb\xbf"); got != string(want) {
				t.Errorf("differs from golden\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
		})
	}
}

func TestReportProducersPassFiltersAndWindow(t *testing.T) {
	month, from, to := "2026-10", "2026-10-01", "2026-10-31"
	q := goldenReportQ()
	if _, err := renderReport(t, &dsrStart, q, reportJob(KindDsrLateDaily, ReportParams{Month: &month, VendorID: i64(7), VendorBranchID: i64(9)})); err != nil {
		t.Fatal(err)
	}
	a := q.dailyArgs[0]
	if *a.VendorID != 7 || *a.VendorBranchID != 9 || a.MonthStart.Time.Format("2006-01-02") != "2026-10-01" ||
		a.StartDate.Time.Format("2006-01-02") != "2026-09-01" || !a.AsOf.Valid || !a.AsOf.Time.Equal(time.Date(2026, 10, 10, 6, 0, 0, 0, testWIB)) {
		t.Errorf("daily args = %+v", a)
	}
	q = goldenReportQ()
	if _, err := renderReport(t, nil, q, reportJob(KindReplenishmentAmountMismatch, ReportParams{DateFrom: &from, DateTo: &to, VendorID: i64(3), RegionCode: sp("MDN")})); err != nil {
		t.Fatal(err)
	}
	c := q.classArgs[0]
	if !c.OnlyDiff || !c.FinalOnly || *c.VendorID != 3 || *c.RegionCode != "MDN" || c.VendorBranchID != nil || c.PageLimit != PageSize {
		t.Errorf("classification args = %+v", c)
	}
}

func TestDsrReportWithoutStartDateIsHeaderOnly(t *testing.T) {
	month := "2026-10"
	for _, kind := range []Kind{KindDsrLateDaily, KindDsrLateSummary} {
		q := goldenReportQ()
		got, err := renderReport(t, nil, q, reportJob(kind, ReportParams{Month: &month}))
		if err != nil {
			t.Fatal(err)
		}
		if want := "\xef\xbb\xbf" + strings.Join(NewProducers(nil)[kind][reportEntities[kind]].Header(), ",") + "\n"; got != want {
			t.Errorf("%s = %q, want header only", kind, got)
		}
		if q.queries != 0 {
			t.Errorf("%s ran %d queries without a start date", kind, q.queries)
		}
	}
}

func TestReportProducerFailsWithoutParams(t *testing.T) {
	for _, kind := range []Kind{KindDsrLateDaily, KindDsrLateSummary, KindReplenishmentAmountMismatch, KindReplenishmentTrips} {
		q := goldenReportQ()
		_, err := renderReport(t, &dsrStart, q, Params{Kind: kind, Entity: reportEntities[kind]})
		if !errors.Is(err, errNoReportParams) || q.queries != 0 {
			t.Errorf("%s: err = %v, queries = %d", kind, err, q.queries)
		}
	}
}

func TestReportProducerRejectsBadDates(t *testing.T) {
	bad := "x"
	for _, p := range []Params{
		reportJob(KindDsrLateDaily, ReportParams{}),
		reportJob(KindDsrLateDaily, ReportParams{Month: &bad}),
		reportJob(KindReplenishmentTrips, ReportParams{DateFrom: &bad, DateTo: &bad}),
		reportJob(KindReplenishmentAmountMismatch, ReportParams{}),
	} {
		if _, err := renderReport(t, &dsrStart, goldenReportQ(), p); !errors.Is(err, errNoReportParams) {
			t.Errorf("%s: err = %v", p.Kind, err)
		}
	}
}

func TestDailyPagingSpansPagesInOrder(t *testing.T) {
	const n = 2500
	q := &reportQ{fakeQ: &fakeQ{}}
	for i := range n {
		q.daily = append(q.daily, repository.ListDsrLateDailyRow{ReportDate: dayAt(1), BranchCode: "B" + strconv.Itoa(i), Deadline: tsAt(1, 9, 0), Status: "ok"})
	}
	month := "2026-10"
	got, err := renderReport(t, &dsrStart, q, reportJob(KindDsrLateDaily, ReportParams{Month: &month}))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	if len(lines) != n+1 {
		t.Fatalf("lines = %d, want %d", len(lines), n+1)
	}
	for i := range n {
		if !strings.Contains(lines[i+1], ",B"+strconv.Itoa(i)+",") {
			t.Fatalf("row %d out of order: %s", i, lines[i+1])
		}
	}
	if len(q.dailyArgs) != 3 || q.dailyArgs[1].PageOffset != 1000 || q.dailyArgs[2].PageOffset != 2000 {
		t.Errorf("paging calls = %+v", q.dailyArgs)
	}
}

func TestAmountMismatchPagingAndLimit(t *testing.T) {
	from, to := "2026-10-01", "2026-10-31"
	q := &reportQ{fakeQ: &fakeQ{}, amounts: map[int64][]repository.ReplenishClassificationAmount{}}
	const total = 100500 // more than ReplenishmentExportLimit
	for i := int64(1); i <= total; i++ {
		q.classes = append(q.classes, repository.ListReplenishClassificationsRow{ID: i, TerminalID: "T", Classification: "late"})
	}
	for i := int64(1); i <= total; i++ {
		q.amounts[i] = []repository.ReplenishClassificationAmount{{ClassificationID: i, Denom: 50000}}
	}
	rows := 0
	pr := NewProducers(nil)[KindReplenishmentAmountMismatch]["amount_mismatch"]
	err := pr.Produce(context.Background(), q, reportJob(KindReplenishmentAmountMismatch, ReportParams{DateFrom: &from, DateTo: &to}), time.Now(),
		func([]string) error { rows++; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if rows != 100000 || len(q.classArgs) != 100 || len(q.amountIDs) != 100 {
		t.Errorf("rows = %d, class pages = %d, amount lookups = %d; want 100000, 100, 100", rows, len(q.classArgs), len(q.amountIDs))
	}
}

func TestReportProducerStopsOnCancelAndEmitError(t *testing.T) {
	month, from, to := "2026-10", "2026-10-01", "2026-10-31"
	boom := errors.New("boom")
	for _, p := range []Params{
		reportJob(KindDsrLateDaily, ReportParams{Month: &month}),
		reportJob(KindDsrLateSummary, ReportParams{Month: &month}),
		reportJob(KindReplenishmentAmountMismatch, ReportParams{DateFrom: &from, DateTo: &to}),
		reportJob(KindReplenishmentTrips, ReportParams{DateFrom: &from, DateTo: &to}),
	} {
		pr := NewProducers(&dsrStart)[p.Kind][p.Entity]
		if err := pr.Produce(context.Background(), goldenReportQ(), p, time.Now(), func([]string) error { return boom }); !errors.Is(err, boom) {
			t.Errorf("%s emit error = %v", p.Kind, err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if p.Kind == KindDsrLateDaily || p.Kind == KindReplenishmentAmountMismatch {
			if err := pr.Produce(ctx, goldenReportQ(), p, time.Now(), func([]string) error { return nil }); !errors.Is(err, context.Canceled) {
				t.Errorf("%s cancel = %v", p.Kind, err)
			}
		}
	}
}
