package exportjob

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
	"github.com/cimb-niaga/cms/pkg/csvexport"
)

// fakeQ serves n rows with ids 1..n from every query and records the params.
type fakeQ struct {
	n     int
	err   error
	calls []any
}

func ids(after int64, n int) []int64 {
	var out []int64
	for i := after + 1; i <= int64(n) && len(out) < PageSize; i++ {
		out = append(out, i)
	}
	return out
}

func (f *fakeQ) ExportVendorsBatch(_ context.Context, a repository.ExportVendorsBatchParams) ([]repository.ExportVendorsBatchRow, error) {
	f.calls = append(f.calls, a)
	var out []repository.ExportVendorsBatchRow
	for _, id := range ids(a.AfterID, f.n) {
		out = append(out, repository.ExportVendorsBatchRow{ID: id, Code: "V", Name: "n"})
	}
	return out, f.err
}
func (f *fakeQ) ExportVendorBranchesBatch(_ context.Context, a repository.ExportVendorBranchesBatchParams) ([]repository.ExportVendorBranchesBatchRow, error) {
	f.calls = append(f.calls, a)
	var out []repository.ExportVendorBranchesBatchRow
	for _, id := range ids(a.AfterID, f.n) {
		out = append(out, repository.ExportVendorBranchesBatchRow{ID: id, BranchCode: "B"})
	}
	return out, f.err
}
func (f *fakeQ) ExportVendorVaultsBatch(_ context.Context, a repository.ExportVendorVaultsBatchParams) ([]repository.ExportVendorVaultsBatchRow, error) {
	f.calls = append(f.calls, a)
	var out []repository.ExportVendorVaultsBatchRow
	for _, id := range ids(a.AfterID, f.n) {
		out = append(out, repository.ExportVendorVaultsBatchRow{ID: id, VaultCode: "VL"})
	}
	return out, f.err
}
func (f *fakeQ) ExportVendorPicsBatch(_ context.Context, a repository.ExportVendorPicsBatchParams) ([]repository.ExportVendorPicsBatchRow, error) {
	f.calls = append(f.calls, a)
	var out []repository.ExportVendorPicsBatchRow
	for _, id := range ids(a.AfterID, f.n) {
		out = append(out, repository.ExportVendorPicsBatchRow{ID: id, Name: "p", Roles: []string{"vault"}})
	}
	return out, f.err
}
func (f *fakeQ) ExportATMsBatch(_ context.Context, a repository.ExportATMsBatchParams) ([]repository.ExportATMsBatchRow, error) {
	f.calls = append(f.calls, a)
	var out []repository.ExportATMsBatchRow
	for _, id := range ids(a.AfterID, f.n) {
		out = append(out, repository.ExportATMsBatchRow{ID: id, TerminalID: "T"})
	}
	return out, f.err
}
func (f *fakeQ) ExportATMAssignmentsBatch(_ context.Context, a repository.ExportATMAssignmentsBatchParams) ([]repository.ExportATMAssignmentsBatchRow, error) {
	f.calls = append(f.calls, a)
	var out []repository.ExportATMAssignmentsBatchRow
	for _, id := range ids(a.AfterID, f.n) {
		out = append(out, repository.ExportATMAssignmentsBatchRow{ID: id, TerminalID: "T"})
	}
	return out, f.err
}
func (f *fakeQ) ExportDsrLocationMapsBatch(_ context.Context, a repository.ExportDsrLocationMapsBatchParams) ([]repository.ExportDsrLocationMapsBatchRow, error) {
	f.calls = append(f.calls, a)
	var out []repository.ExportDsrLocationMapsBatchRow
	for _, id := range ids(a.AfterID, f.n) {
		out = append(out, repository.ExportDsrLocationMapsBatchRow{ID: id, VendorCode: "V", BranchCode: "B", VaultCode: "VL", DsrLocation: "loc", IsActive: true})
	}
	return out, f.err
}
func (f *fakeQ) ExportPackagePricesBatch(_ context.Context, a repository.ExportPackagePricesBatchParams) ([]repository.ExportPackagePricesBatchRow, error) {
	f.calls = append(f.calls, a)
	var out []repository.ExportPackagePricesBatchRow
	for _, id := range ids(a.AfterID, f.n) {
		out = append(out, repository.ExportPackagePricesBatchRow{ID: id, VendorCode: "V", PriceLevel: "vendor", Package: "PAKET 3", TierMin: "1", BasePrice: "1000.00", Currency: "IDR", EffectiveStartDate: "2026-01-01"})
	}
	return out, f.err
}

// Count* serve n, the number of rows every Export*Batch query pages through.
func (f *fakeQ) CountVendors(context.Context, repository.CountVendorsParams) (int64, error) {
	return int64(f.n), f.err
}
func (f *fakeQ) CountVendorBranches(context.Context, repository.CountVendorBranchesParams) (int64, error) {
	return int64(f.n), f.err
}
func (f *fakeQ) CountVendorVaults(context.Context, repository.CountVendorVaultsParams) (int64, error) {
	return int64(f.n), f.err
}
func (f *fakeQ) CountVendorPics(context.Context, repository.CountVendorPicsParams) (int64, error) {
	return int64(f.n), f.err
}
func (f *fakeQ) CountATMs(context.Context, string) (int64, error)           { return int64(f.n), f.err }
func (f *fakeQ) CountATMAssignments(context.Context, string) (int64, error) { return int64(f.n), f.err }
func (f *fakeQ) CountDsrLocationMaps(context.Context, repository.CountDsrLocationMapsParams) (int64, error) {
	return int64(f.n), f.err
}
func (f *fakeQ) CountPackagePrices(context.Context, repository.CountPackagePricesParams) (int64, error) {
	return int64(f.n), f.err
}

func (f *fakeQ) ListDsrLateDaily(context.Context, repository.ListDsrLateDailyParams) ([]repository.ListDsrLateDailyRow, error) {
	return nil, f.err
}
func (f *fakeQ) ListDsrLateSummary(context.Context, repository.ListDsrLateSummaryParams) ([]repository.ListDsrLateSummaryRow, error) {
	return nil, f.err
}
func (f *fakeQ) ListReplenishClassifications(context.Context, repository.ListReplenishClassificationsParams) ([]repository.ListReplenishClassificationsRow, error) {
	return nil, f.err
}
func (f *fakeQ) ListReplenishClassificationAmounts(context.Context, []int64) ([]repository.ReplenishClassificationAmount, error) {
	return nil, f.err
}
func (f *fakeQ) ReportReplenishTrips(context.Context, repository.ReportReplenishTripsParams) ([]repository.ReportReplenishTripsRow, error) {
	return nil, f.err
}

func i64(v int64) *int64 { return &v }

func run(t *testing.T, f *fakeQ, p Params, asOf time.Time) [][]string {
	t.Helper()
	pr := DefaultProducers()[p.Kind][p.Entity]
	if pr == nil {
		t.Fatalf("no producer for %s/%s", p.Kind, p.Entity)
	}
	var rows [][]string
	if err := pr.Produce(context.Background(), f, p, asOf, func(r []string) error { rows = append(rows, r); return nil }); err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestDefaultProducersCoverEntities(t *testing.T) {
	m := DefaultProducers()
	for _, k := range []Kind{KindVendor, KindCabang, KindHargaPaket, KindMasterDataGlobal} {
		for _, e := range entitiesFor(k) {
			if m[k][e] == nil {
				t.Errorf("missing producer %s/%s", k, e)
			}
		}
		if len(m[k]) != len(entitiesFor(k)) {
			t.Errorf("%s: %d producers, %d entities", k, len(m[k]), len(entitiesFor(k)))
		}
	}
}

func TestHeadersMatchCSVExport(t *testing.T) {
	m := DefaultProducers()
	for name, kind := range map[string]Kind{"vendor": KindVendor} {
		_ = name
		want, _ := csvexport.Lookup(csvexport.Vendors)
		if !slices.Equal(m[kind]["vendor"].Header(), want.Header) {
			t.Error("vendor header differs")
		}
	}
	for _, e := range csvexport.ImportableNames() {
		want, _ := csvexport.Lookup(e)
		if !slices.Equal(m[KindMasterDataGlobal][e].Header(), want.Header) {
			t.Errorf("%s header differs", e)
		}
	}
	for _, e := range csvexport.ExportOnlyNames() {
		want, _ := csvexport.LookupExportOnly(e)
		kind := KindVendor
		if e == csvexport.PackagePrices {
			kind = KindHargaPaket
		}
		if !slices.Equal(m[kind][e].Header(), want.Header) {
			t.Errorf("%s header differs", e)
		}
	}
}

func TestScopeAndStatusPerKind(t *testing.T) {
	asOf := time.Date(2026, 10, 28, 20, 0, 0, 0, time.UTC) // 03:00 WIB next day
	tests := []struct {
		name string
		p    Params
		want any
	}{
		{"vendor profile forces all", Params{Kind: KindVendor, Entity: "vendor", VendorID: i64(7), StatusFilter: "active"},
			repository.ExportVendorsBatchParams{VendorID: i64(7), Status: "all", BatchSize: PageSize}},
		{"vendor branches", Params{Kind: KindVendor, Entity: "branches", VendorID: i64(7), StatusFilter: "disabled"},
			repository.ExportVendorBranchesBatchParams{VendorID: i64(7), Status: "disabled", BatchSize: PageSize}},
		{"vendor pics no branch", Params{Kind: KindVendor, Entity: "pics", VendorID: i64(7), StatusFilter: "active"},
			repository.ExportVendorPicsBatchParams{VendorID: i64(7), Status: "active", BatchSize: PageSize}},
		{"cabang vaults", Params{Kind: KindCabang, Entity: "vaults", VendorID: i64(7), BranchID: i64(3), StatusFilter: "all"},
			repository.ExportVendorVaultsBatchParams{VendorID: i64(7), BranchID: i64(3), Status: "all", BatchSize: PageSize}},
		{"cabang pics", Params{Kind: KindCabang, Entity: "pics", VendorID: i64(7), BranchID: i64(3), StatusFilter: "active"},
			repository.ExportVendorPicsBatchParams{VendorID: i64(7), BranchID: i64(3), Status: "active", BatchSize: PageSize}},
		{"cabang dsr maps", Params{Kind: KindCabang, Entity: "dsr-location-maps", VendorID: i64(7), BranchID: i64(3), StatusFilter: "active"},
			repository.ExportDsrLocationMapsBatchParams{VendorID: i64(7), BranchID: i64(3), Status: "active", BatchSize: PageSize}},
		{"global vendors nil scope", Params{Kind: KindMasterDataGlobal, Entity: csvexport.Vendors, StatusFilter: "active"},
			repository.ExportVendorsBatchParams{Status: "active", BatchSize: PageSize}},
		{"global atms", Params{Kind: KindMasterDataGlobal, Entity: csvexport.ATMs, StatusFilter: "all"},
			repository.ExportATMsBatchParams{Status: "all", BatchSize: PageSize}},
		{"global assignments", Params{Kind: KindMasterDataGlobal, Entity: csvexport.ATMAssignments, StatusFilter: "disabled"},
			repository.ExportATMAssignmentsBatchParams{Status: "disabled", BatchSize: PageSize}},
		{"prices", Params{Kind: KindHargaPaket, Entity: "package-prices", VendorID: i64(7), BranchID: i64(3), StatusFilter: "active", PriceLevel: strp("branch")},
			repository.ExportPackagePricesBatchParams{VendorID: 7, BranchID: i64(3), PriceLevel: "branch", Status: "active", BatchSize: PageSize}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeQ{n: 1}
			run(t, f, tc.p, asOf)
			if len(f.calls) == 0 {
				t.Fatal("no query")
			}
			got := f.calls[0]
			if pp, ok := got.(repository.ExportPackagePricesBatchParams); ok {
				wantDate := wibDate(asOf)
				if pp.AsOf != wantDate || wantDate.Time.Day() != 29 {
					t.Errorf("as_of %v, want WIB date 2026-10-29", pp.AsOf)
				}
				pp.AsOf = tc.want.(repository.ExportPackagePricesBatchParams).AsOf
				got = pp
			}
			if !samePtrs(got, tc.want) {
				t.Errorf("params = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func strp(s string) *string { return &s }

// samePtrs compares params by value, dereferencing the id pointers.
func samePtrs(a, b any) bool {
	return deref(a) == deref(b)
}

func deref(v any) string {
	d := func(p *int64) int64 {
		if p == nil {
			return -1
		}
		return *p
	}
	switch x := v.(type) {
	case repository.ExportVendorsBatchParams:
		return str(x.AfterID, d(x.VendorID), -2, x.Status, x.BatchSize)
	case repository.ExportVendorBranchesBatchParams:
		return str(x.AfterID, d(x.VendorID), -2, x.Status, x.BatchSize)
	case repository.ExportVendorPicsBatchParams:
		return str(x.AfterID, d(x.VendorID), d(x.BranchID), x.Status, x.BatchSize)
	case repository.ExportVendorVaultsBatchParams:
		return str(x.AfterID, d(x.VendorID), d(x.BranchID), x.Status, x.BatchSize)
	case repository.ExportDsrLocationMapsBatchParams:
		return str(x.AfterID, d(x.VendorID), d(x.BranchID), x.Status, x.BatchSize)
	case repository.ExportATMsBatchParams:
		return str(x.AfterID, -2, -2, x.Status, x.BatchSize)
	case repository.ExportATMAssignmentsBatchParams:
		return str(x.AfterID, -2, -2, x.Status, x.BatchSize)
	case repository.ExportPackagePricesBatchParams:
		return str(x.AfterID, x.VendorID, d(x.BranchID), x.Status+"/"+x.PriceLevel, x.BatchSize)
	}
	return "?"
}

func str(after, v, b int64, status string, size int32) string {
	return string(rune(after+'0')) + "|" + itoa(v) + "|" + itoa(b) + "|" + status + "|" + itoa(int64(size))
}

func itoa(n int64) string {
	if n < 0 {
		return "n" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

func TestPagingAcrossPages(t *testing.T) {
	f := &fakeQ{n: 2*PageSize + 5}
	rows := run(t, f, Params{Kind: KindMasterDataGlobal, Entity: csvexport.Vendors, StatusFilter: "all"}, time.Now())
	if len(rows) != 2*PageSize+5 {
		t.Fatalf("rows = %d", len(rows))
	}
	for i, r := range rows {
		if r[0] != itoa(int64(i+1)) {
			t.Fatalf("row %d id %s", i, r[0])
		}
	}
	if len(f.calls) != 3 {
		t.Errorf("queries = %d, want 3", len(f.calls))
	}
	after := []int64{0, PageSize, 2 * PageSize}
	for i, c := range f.calls {
		if got := c.(repository.ExportVendorsBatchParams).AfterID; got != after[i] {
			t.Errorf("call %d after_id %d, want %d", i, got, after[i])
		}
	}
}

func TestExactPageBoundaryFetchesOneMore(t *testing.T) {
	f := &fakeQ{n: PageSize}
	rows := run(t, f, Params{Kind: KindMasterDataGlobal, Entity: csvexport.ATMs, StatusFilter: "all"}, time.Now())
	if len(rows) != PageSize || len(f.calls) != 2 {
		t.Errorf("rows=%d queries=%d, want %d and 2", len(rows), len(f.calls), PageSize)
	}
}

func TestEmptyEmitsNothing(t *testing.T) {
	f := &fakeQ{}
	if rows := run(t, f, Params{Kind: KindVendor, Entity: "vaults", VendorID: i64(1), StatusFilter: "all"}, time.Now()); len(rows) != 0 {
		t.Errorf("rows = %v", rows)
	}
}

func TestCellsMatchCSVExport(t *testing.T) {
	f := &fakeQ{n: 1}
	got := run(t, f, Params{Kind: KindVendor, Entity: "pics", VendorID: i64(1), StatusFilter: "all"}, time.Now())[0]
	want := csvexport.VendorPICCells(csvexport.VendorPICRow{ID: 1, Name: "p", Roles: []string{"vault"}})
	if !slices.Equal(got, want) {
		t.Errorf("cells = %v, want %v", got, want)
	}
}

func TestExportOnlyCellOrder(t *testing.T) {
	f := &fakeQ{n: 1}
	got := run(t, f, Params{Kind: KindVendor, Entity: "dsr-location-maps", VendorID: i64(1), StatusFilter: "all"}, time.Now())[0]
	if want := []string{"1", "V", "B", "VL", "loc", "true"}; !slices.Equal(got, want) {
		t.Errorf("dsr cells = %v", got)
	}
	got = run(t, f, Params{Kind: KindHargaPaket, Entity: "package-prices", VendorID: i64(1), StatusFilter: "all"}, time.Now())[0]
	want := []string{"1", "V", "vendor", "", "", "PAKET 3", "", "", "", "1", "", "1000.00", "IDR", "", "2026-01-01", ""}
	if !slices.Equal(got, want) {
		t.Errorf("price cells = %v", got)
	}
	h := DefaultProducers()[KindHargaPaket]["package-prices"].Header()
	if len(got) != len(h) {
		t.Errorf("cells %d vs header %d", len(got), len(h))
	}
}

func TestPriceDefaultsLevelAllAndRequiresVendor(t *testing.T) {
	f := &fakeQ{n: 1}
	run(t, f, Params{Kind: KindHargaPaket, Entity: "package-prices", VendorID: i64(1), StatusFilter: "all"}, time.Now())
	if f.calls[0].(repository.ExportPackagePricesBatchParams).PriceLevel != "all" {
		t.Error("price level default")
	}
	pr := DefaultProducers()[KindHargaPaket]["package-prices"]
	err := pr.Produce(context.Background(), f, Params{Kind: KindHargaPaket, Entity: "package-prices"}, time.Now(), func([]string) error { return nil })
	if !errors.Is(err, ErrValidation) {
		t.Errorf("err = %v", err)
	}
}

func TestErrorsPropagate(t *testing.T) {
	boom := errors.New("boom")
	pr := DefaultProducers()[KindMasterDataGlobal][csvexport.Vendors]
	p := Params{Kind: KindMasterDataGlobal, Entity: csvexport.Vendors, StatusFilter: "all"}
	if err := pr.Produce(context.Background(), &fakeQ{err: boom}, p, time.Now(), func([]string) error { return nil }); !errors.Is(err, boom) {
		t.Errorf("query err = %v", err)
	}
	if err := pr.Produce(context.Background(), &fakeQ{n: 3}, p, time.Now(), func([]string) error { return boom }); !errors.Is(err, boom) {
		t.Errorf("emit err = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := pr.Produce(ctx, &fakeQ{n: 3}, p, time.Now(), func([]string) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Errorf("ctx err = %v", err)
	}
}
