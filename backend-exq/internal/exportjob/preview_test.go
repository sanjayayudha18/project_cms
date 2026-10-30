package exportjob

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
)

// Task 3.2. Fakes shared with preview_order_test.go: countingReader, stubPreview.

var masterKinds = []Kind{KindVendor, KindCabang, KindHargaPaket, KindMasterDataGlobal}

// fakeScope stands in for the primary scope lookup and counts how often it is touched.
type fakeScope struct {
	createQueries
	calls int
}

func (s *fakeScope) GetVendorCodeByID(context.Context, int64) (string, error) {
	s.calls++
	return "V", nil
}

func (s *fakeScope) GetVendorBranchForVendor(context.Context, repository.GetVendorBranchForVendorParams) (string, error) {
	s.calls++
	return "B", nil
}

// scopedParams returns params with the scope ids ParseParams would require for kind.
func scopedParams(kind Kind) Params {
	v, b := int64(1), int64(2)
	p := Params{Kind: kind, Entity: "e", StatusFilter: "all"}
	switch kind {
	case KindVendor, KindHargaPaket:
		p.VendorID = &v
	case KindCabang:
		p.VendorID, p.BranchID = &v, &b
	}
	return p
}

func previewerFor(kind Kind, pp Producer, now func() time.Time) (*Previewer, *countingReader, *fakeScope) {
	r, s := &countingReader{}, &fakeScope{}
	if now == nil {
		now = time.Now
	}
	return &Previewer{scope: s, reader: r, producers: map[Kind]map[string]Producer{kind: {"e": pp}}, now: now}, r, s
}

// Property 5: preview right == export right, for every kind x role.
func TestPreviewRBACMatchesExport(t *testing.T) {
	roles := map[string]bool{
		"": true, "VENDOR-USER": true, "ACM-USER": true, "ACM-SPV": true, "APPACCESS": true,
		"VENDOR-ATM-USER": true, "admin": true, "ADMIN ": true,
	}
	var kinds []Kind
	for k, rs := range KindRoles {
		kinds = append(kinds, k)
		for _, r := range rs {
			roles[r] = true
		}
	}
	combos := 0
	for _, kind := range kinds {
		isMaster := slices.Contains(masterKinds, kind)
		for role := range roles {
			combos++
			t.Run(fmt.Sprintf("%s/%q", kind, role), func(t *testing.T) {
				var pp Producer = stubPreview{n: 3}
				if !isMaster {
					pp = producer{} // report kinds have no preview producer
				}
				v, r, s := previewerFor(kind, pp, nil)
				_, err := v.Preview(context.Background(), Actor{UserID: 1, Role: role}, scopedParams(kind), 0, 10)

				want := allowed(kind, role)
				if isMaster { // independent oracle: master data is ADMIN / ADMIN_PARAM only
					if want != (role == "ADMIN" || role == "ADMIN_PARAM") {
						t.Fatalf("allowed() = %v disagrees with the master-data role rule", want)
					}
				}
				switch {
				case !want:
					if !errors.Is(err, ErrForbidden) || r.begins != 0 || s.calls != 0 {
						t.Fatalf("denied: err=%v begins=%d scopeCalls=%d", err, r.begins, s.calls)
					}
				case !isMaster:
					if !errors.Is(err, ErrUnknownEntity) || r.begins != 0 {
						t.Fatalf("report kind: err=%v begins=%d", err, r.begins)
					}
				default:
					if err != nil || r.begins != 1 {
						t.Fatalf("allowed: err=%v begins=%d", err, r.begins)
					}
					if kind != KindMasterDataGlobal && s.calls == 0 {
						t.Fatal("scope was not resolved on the primary")
					}
				}
			})
		}
	}
	t.Logf("Property 5: %d kind x role combinations", combos)
}

// Property 3 (validation part).
func TestParsePaging(t *testing.T) {
	tests := []struct {
		name, after, limit string
		wantAfter          int64
		wantLimit          int
		wantErr            bool
	}{
		{"defaults", "", "", 0, 50, false},
		{"explicit", "7", "100", 7, 100, false},
		{"min limit", "0", "1", 0, 1, false},
		{"after only", "12", "", 12, 50, false},
		{"limit only", "", "20", 0, 20, false},
		{"after negative", "-1", "", 0, 0, true},
		{"after not a number", "abc", "", 0, 0, true},
		{"after decimal", "1.5", "", 0, 0, true},
		{"after padded", " 1", "", 0, 0, true},
		{"after overflow", "99999999999999999999", "", 0, 0, true},
		{"limit zero", "", "0", 0, 0, true},
		{"limit over max", "", "101", 0, 0, true},
		{"limit negative", "", "-5", 0, 0, true},
		{"limit not a number", "", "x", 0, 0, true},
		{"limit decimal", "", "1.5", 0, 0, true},
		{"both bad", "-1", "0", 0, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			after, limit, err := ParsePaging(tc.after, tc.limit)
			if tc.wantErr {
				if !errors.Is(err, ErrValidation) {
					t.Fatalf("err = %v, want ErrValidation", err)
				}
				return
			}
			if err != nil || after != tc.wantAfter || limit != tc.wantLimit {
				t.Fatalf("got (%d, %d, %v), want (%d, %d)", after, limit, err, tc.wantAfter, tc.wantLimit)
			}
		})
	}
	for l := -10; l <= 110; l++ { // limit is valid exactly on 1..100
		_, got, err := ParsePaging("", strconv.Itoa(l))
		if ok := l >= 1 && l <= 100; ok != (err == nil) || (ok && got != l) {
			t.Fatalf("limit %d: got %d, err %v", l, got, err)
		}
	}
}

// failingPreview serves ids 1..n but fails Count and/or the failPage-th Page call (1-based).
type failingPreview struct {
	stubPreview
	countErr, pageErr error
	failPage, pages   int
}

func (f *failingPreview) Count(ctx context.Context, q Querier, p Params, t time.Time) (int64, error) {
	if f.countErr != nil {
		return 0, f.countErr
	}
	return f.stubPreview.Count(ctx, q, p, t)
}

func (f *failingPreview) Page(ctx context.Context, q Querier, p Params, t time.Time, after int64, limit int) ([][]string, int64, error) {
	f.pages++
	if f.pageErr != nil && f.pages == f.failPage {
		return nil, after, f.pageErr
	}
	return f.stubPreview.Page(ctx, q, p, t, after, limit)
}

type errReader struct{ err error }

func (r errReader) Begin(context.Context) (Querier, func(), error) { return nil, func() {}, r.err }

func TestPreviewReplicaErrors(t *testing.T) {
	conflict := &pgconn.PgError{Code: "40001", Message: "could not serialize access"}
	recovery := errors.New("ERROR: canceling statement due to conflict with recovery (SQLSTATE 40001)")
	boom := errors.New("connection reset")

	tests := []struct {
		name        string
		begin       error
		countErr    error
		pageErr     error
		failPage    int
		wantUnavail bool
	}{
		{"begin 40001", conflict, nil, nil, 0, true},
		{"count 40001", nil, conflict, nil, 0, true},
		{"count recovery text", nil, recovery, nil, 0, true},
		{"page 40001", nil, nil, conflict, 1, true},
		{"wrapped 40001", nil, nil, fmt.Errorf("page: %w", conflict), 1, true},
		{"probe page 40001", nil, nil, conflict, 2, true}, // second Page call = next_after probe
		{"begin other error", boom, nil, nil, 0, false},
		{"count other error", nil, boom, nil, 0, false},
		{"page other error", nil, nil, boom, 1, false},
		{"probe other error", nil, nil, boom, 2, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pp := &failingPreview{stubPreview: stubPreview{n: 5}, countErr: tc.countErr, pageErr: tc.pageErr, failPage: tc.failPage}
			v, _, _ := previewerFor(KindMasterDataGlobal, pp, nil)
			if tc.begin != nil {
				v.reader = errReader{tc.begin}
			}
			_, err := v.Preview(context.Background(), Actor{UserID: 1, Role: "ADMIN"}, scopedParams(KindMasterDataGlobal), 0, 2)
			if err == nil {
				t.Fatal("want error")
			}
			if errors.Is(err, ErrUnavailable) != tc.wantUnavail || errors.Is(err, ErrInternal) == tc.wantUnavail {
				t.Fatalf("err = %v, unavailable=%v internal=%v, want unavailable=%v", err, errors.Is(err, ErrUnavailable), errors.Is(err, ErrInternal), tc.wantUnavail)
			}
		})
	}
}

// asOfQuerier records the as_of date the Harga Paket producer sends (the only as_of consumer).
type asOfQuerier struct {
	Querier
	asOf []pgtype.Date
}

func (q *asOfQuerier) CountPackagePrices(_ context.Context, a repository.CountPackagePricesParams) (int64, error) {
	q.asOf = append(q.asOf, a.AsOf)
	return 0, nil
}

func (q *asOfQuerier) ExportPackagePricesBatch(_ context.Context, a repository.ExportPackagePricesBatchParams) ([]repository.ExportPackagePricesBatchRow, error) {
	q.asOf = append(q.asOf, a.AsOf)
	return nil, nil
}

type fixedReader struct{ q Querier }

func (r fixedReader) Begin(context.Context) (Querier, func(), error) { return r.q, func() {}, nil }

func TestPreviewAsOfIsWIBDateOfInjectedClock(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want string
	}{
		{"UTC evening is next WIB day", time.Date(2026, 10, 30, 18, 0, 0, 0, time.UTC), "2026-10-31"},
		{"just before WIB midnight", time.Date(2026, 10, 30, 16, 59, 0, 0, time.UTC), "2026-10-30"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := &asOfQuerier{}
			v, _, _ := previewerFor(KindHargaPaket, packagePricesProducer(), func() time.Time { return tc.now })
			v.reader = fixedReader{rec}
			p := scopedParams(KindHargaPaket)
			p.Entity = "package-prices"
			v.producers[KindHargaPaket]["package-prices"] = packagePricesProducer()
			if _, err := v.Preview(context.Background(), Actor{UserID: 1, Role: "ADMIN"}, p, 0, 10); err != nil {
				t.Fatal(err)
			}
			if len(rec.asOf) < 2 { // Count + Page
				t.Fatalf("recorded %d as_of values, want Count and Page", len(rec.asOf))
			}
			for _, d := range rec.asOf {
				if !d.Valid || d.Time.Format("2006-01-02") != tc.want {
					t.Fatalf("as_of = %+v, want %s", d, tc.want)
				}
			}
		})
	}
}

// next_after is right for every (n, limit, after), including n an exact multiple of limit,
// and following it visits every remaining row once.
func TestPreviewNextAfterSweep(t *testing.T) {
	combos := 0
	for n := int64(0); n <= 8; n++ {
		for limit := 1; limit <= 4; limit++ {
			for after := int64(0); after <= n; after++ {
				combos++
				v, _ := newStubPreviewer(n)
				p := Params{Kind: KindMasterDataGlobal, Entity: "vendors"}
				got, err := v.Preview(context.Background(), Actor{UserID: 1, Role: "ADMIN"}, p, after, limit)
				if err != nil {
					t.Fatal(err)
				}
				wantRows := min(int64(limit), n-after)
				last := after + wantRows
				hasNext := wantRows == int64(limit) && n > last
				if int64(len(got.Rows)) != wantRows || got.Total != n || (got.NextAfter != nil) != hasNext ||
					(hasNext && *got.NextAfter != last) {
					t.Fatalf("n=%d limit=%d after=%d: rows=%d total=%d next=%v; want rows=%d next=%v@%d",
						n, limit, after, len(got.Rows), got.Total, got.NextAfter, wantRows, hasNext, last)
				}
				if after != 0 {
					continue
				}
				var ids []string // walk all pages
				for cur, guard := int64(0), 0; ; guard++ {
					pg, err := v.Preview(context.Background(), Actor{UserID: 1, Role: "ADMIN"}, p, cur, limit)
					if err != nil || guard > 20 {
						t.Fatalf("walk: err=%v guard=%d", err, guard)
					}
					for _, r := range pg.Rows {
						ids = append(ids, r[0])
					}
					if pg.NextAfter == nil {
						break
					}
					cur = *pg.NextAfter
				}
				if int64(len(ids)) != n {
					t.Fatalf("n=%d limit=%d: walk visited %d rows %v", n, limit, len(ids), ids)
				}
			}
		}
	}
	t.Logf("next_after: %d (n, limit, after) combinations", combos)
}

// The previewer must not hold an audit writer: it cannot audit (D5, Req 4.11).
func TestPreviewerHasNoAuditWriter(t *testing.T) {
	iface := reflect.TypeOf((*auditWriter)(nil)).Elem()
	typ := reflect.TypeOf(Previewer{})
	for i := range typ.NumField() {
		if f := typ.Field(i); f.Type.Implements(iface) {
			t.Fatalf("Previewer.%s can write audit entries", f.Name)
		}
	}
}
