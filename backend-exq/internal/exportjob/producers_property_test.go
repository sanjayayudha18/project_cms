package exportjob

import (
	"bytes"
	"context"
	"encoding/csv"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
	"github.com/cimb-niaga/cms/pkg/csvexport"
)

// Properties 1 and 3 (page part) of export-menu-revamp: the preview (Page) and
// the file (Produce) read the same rows, and keyset paging is bounded and complete.

// propEntry is one in-memory table row plus the hidden columns the SQL filters on.
type propEntry[R any] struct {
	id             int64
	row            R
	vendor, branch int64
	active         bool
}

func (e propEntry[R]) matches(vendor, branch *int64, status string) bool {
	if vendor != nil && *vendor != e.vendor || branch != nil && *branch != e.branch {
		return false
	}
	switch status {
	case "active":
		return e.active
	case "disabled":
		return !e.active
	}
	return true
}

// propQ answers the eight Export*Batch queries like `WHERE id > after AND <scope>
// ORDER BY id LIMIT n`. Unset Querier methods (Count*, reports) panic if called.
type propQ struct {
	Querier
	vendors     []propEntry[repository.ExportVendorsBatchRow]
	branches    []propEntry[repository.ExportVendorBranchesBatchRow]
	vaults      []propEntry[repository.ExportVendorVaultsBatchRow]
	pics        []propEntry[repository.ExportVendorPicsBatchRow]
	atms        []propEntry[repository.ExportATMsBatchRow]
	assignments []propEntry[repository.ExportATMAssignmentsBatchRow]
	dsrMaps     []propEntry[repository.ExportDsrLocationMapsBatchRow]
	prices      []propEntry[repository.ExportPackagePricesBatchRow]
	served      []int64 // ids returned by the last query
}

func propPage[R any](q *propQ, es []propEntry[R], after int64, limit int32, vendor, branch *int64, status string) []R {
	q.served = nil
	out := []R{}
	for _, e := range es {
		if len(out) >= int(limit) {
			break
		}
		if e.id > after && e.matches(vendor, branch, status) {
			out = append(out, e.row)
			q.served = append(q.served, e.id)
		}
	}
	return out
}

func (q *propQ) ExportVendorsBatch(_ context.Context, a repository.ExportVendorsBatchParams) ([]repository.ExportVendorsBatchRow, error) {
	return propPage(q, q.vendors, a.AfterID, a.BatchSize, a.VendorID, nil, a.Status), nil
}

func (q *propQ) ExportVendorBranchesBatch(_ context.Context, a repository.ExportVendorBranchesBatchParams) ([]repository.ExportVendorBranchesBatchRow, error) {
	return propPage(q, q.branches, a.AfterID, a.BatchSize, a.VendorID, nil, a.Status), nil
}

func (q *propQ) ExportVendorVaultsBatch(_ context.Context, a repository.ExportVendorVaultsBatchParams) ([]repository.ExportVendorVaultsBatchRow, error) {
	return propPage(q, q.vaults, a.AfterID, a.BatchSize, a.VendorID, a.BranchID, a.Status), nil
}

func (q *propQ) ExportVendorPicsBatch(_ context.Context, a repository.ExportVendorPicsBatchParams) ([]repository.ExportVendorPicsBatchRow, error) {
	return propPage(q, q.pics, a.AfterID, a.BatchSize, a.VendorID, a.BranchID, a.Status), nil
}

func (q *propQ) ExportATMsBatch(_ context.Context, a repository.ExportATMsBatchParams) ([]repository.ExportATMsBatchRow, error) {
	return propPage(q, q.atms, a.AfterID, a.BatchSize, nil, nil, a.Status), nil
}

func (q *propQ) ExportATMAssignmentsBatch(_ context.Context, a repository.ExportATMAssignmentsBatchParams) ([]repository.ExportATMAssignmentsBatchRow, error) {
	return propPage(q, q.assignments, a.AfterID, a.BatchSize, nil, nil, a.Status), nil
}

func (q *propQ) ExportDsrLocationMapsBatch(_ context.Context, a repository.ExportDsrLocationMapsBatchParams) ([]repository.ExportDsrLocationMapsBatchRow, error) {
	return propPage(q, q.dsrMaps, a.AfterID, a.BatchSize, a.VendorID, a.BranchID, a.Status), nil
}

func (q *propQ) ExportPackagePricesBatch(_ context.Context, a repository.ExportPackagePricesBatchParams) ([]repository.ExportPackagePricesBatchRow, error) {
	return propPage(q, q.prices, a.AfterID, a.BatchSize, &a.VendorID, a.BranchID, a.Status), nil
}

// propText mixes arbitrary text with the leading characters CSVSafe reacts to.
// "\n" is dropped so the CSV round trip in Property 1 stays exact.
var propText = rapid.Custom(func(t *rapid.T) string {
	prefix := rapid.SampledFrom([]string{"", "", "", "=", "+", "-", "@", "\t", "\r", "+62 812-345", "-6.2", "'="}).Draw(t, "prefix")
	body := rapid.StringN(0, 12, -1).Draw(t, "body")
	return prefix + strings.ReplaceAll(body, "\n", " ")
})

// propRow fills every string/bool/[]string field of a sqlc row struct at random.
func propRow[R any](t *rapid.T) R {
	var row R
	v := reflect.ValueOf(&row).Elem()
	for i := range v.NumField() {
		f := v.Field(i)
		switch f.Kind() {
		case reflect.String:
			f.SetString(propText.Draw(t, "str"))
		case reflect.Bool:
			f.SetBool(rapid.Bool().Draw(t, "bool"))
		case reflect.Int64:
			// ID is overwritten by propTable
		case reflect.Slice:
			f.Set(reflect.ValueOf(rapid.SliceOfN(propText, 0, 3).Draw(t, "strs")))
		default:
			t.Fatalf("propRow: unsupported field kind %s", f.Kind())
		}
	}
	return row
}

// propTable builds n rows with ascending ids (gaps included). Only the first 20 rows
// are drawn; the rest cycle through them, so n > PageSize stays cheap.
func propTable[R any](t *rapid.T, n int) []propEntry[R] {
	base := make([]propEntry[R], min(n, 20))
	for i := range base {
		base[i] = propEntry[R]{
			row:    propRow[R](t),
			vendor: rapid.Int64Range(1, 3).Draw(t, "vendor"),
			branch: rapid.Int64Range(1, 3).Draw(t, "branch"),
			active: rapid.Bool().Draw(t, "active"),
		}
	}
	out := make([]propEntry[R], n)
	var id int64
	for i := range out {
		e := base[i%len(base)]
		id += 1 + int64(i%3)
		e.id = id
		reflect.ValueOf(&e.row).Elem().FieldByName("ID").SetInt(id)
		out[i] = e
	}
	return out
}

type propCase struct {
	name string
	prod Producer
	fill func(t *rapid.T, q *propQ, n int)
}

func propCases() []propCase {
	return []propCase{
		{"vendors", vendorsProducer(false), func(t *rapid.T, q *propQ, n int) {
			q.vendors = propTable[repository.ExportVendorsBatchRow](t, n)
		}},
		{"vendors-forceAll", vendorsProducer(true), func(t *rapid.T, q *propQ, n int) {
			q.vendors = propTable[repository.ExportVendorsBatchRow](t, n)
		}},
		{"branches", branchesProducer(), func(t *rapid.T, q *propQ, n int) {
			q.branches = propTable[repository.ExportVendorBranchesBatchRow](t, n)
		}},
		{"vaults", vaultsProducer(), func(t *rapid.T, q *propQ, n int) {
			q.vaults = propTable[repository.ExportVendorVaultsBatchRow](t, n)
		}},
		{"pics", picsProducer(), func(t *rapid.T, q *propQ, n int) {
			q.pics = propTable[repository.ExportVendorPicsBatchRow](t, n)
		}},
		{"atms", atmsProducer(), func(t *rapid.T, q *propQ, n int) {
			q.atms = propTable[repository.ExportATMsBatchRow](t, n)
		}},
		{"atm-assignments", atmAssignmentsProducer(), func(t *rapid.T, q *propQ, n int) {
			q.assignments = propTable[repository.ExportATMAssignmentsBatchRow](t, n)
		}},
		{"dsr-location-maps", dsrMapsProducer(), func(t *rapid.T, q *propQ, n int) {
			q.dsrMaps = propTable[repository.ExportDsrLocationMapsBatchRow](t, n)
		}},
		{"package-prices", packagePricesProducer(), func(t *rapid.T, q *propQ, n int) {
			q.prices = propTable[repository.ExportPackagePricesBatchRow](t, n)
		}},
	}
}

var propRowCount = rapid.OneOf(
	rapid.IntRange(0, 60),
	rapid.IntRange(PageSize-1, 2*PageSize+3), // crosses Produce's page boundary
)

// propSetup draws a table and a random filter/scope for one case.
func propSetup(t *rapid.T, c propCase) (*propQ, PreviewProducer, Params, int) {
	n := propRowCount.Draw(t, "rows")
	q := &propQ{}
	c.fill(t, q, n)
	vendor := rapid.Int64Range(1, 3).Draw(t, "vendorScope")
	p := Params{StatusFilter: rapid.SampledFrom([]string{"active", "disabled", "all"}).Draw(t, "status")}
	if rapid.Bool().Draw(t, "scopeVendor") || c.name == "package-prices" { // prices need vendor_id
		p.VendorID = &vendor
	}
	if rapid.Bool().Draw(t, "scopeBranch") {
		branch := rapid.Int64Range(1, 3).Draw(t, "branchScope")
		p.BranchID = &branch
	}
	if c.name == "package-prices" {
		level := priceLevelAll
		p.PriceLevel = &level
	}
	return q, c.prod.(PreviewProducer), p, n
}

// Feature: export-menu-revamp, Property 1: pratinjau berhalaman sama dengan isi file.
func TestProperty1PreviewPagesEqualFile(t *testing.T) {
	asOf := time.Date(2026, 10, 30, 8, 0, 0, 0, wib)
	for _, c := range propCases() {
		t.Run(c.name, rapid.MakeCheck(func(t *rapid.T) {
			q, prod, p, n := propSetup(t, c)
			limit := rapid.IntRange(1, 100).Draw(t, "limit")
			ctx := context.Background()

			var preview [][]string
			var after int64
			for i := 0; ; i++ {
				if i > n+1 {
					t.Fatalf("paging did not terminate after %d pages", i)
				}
				rows, last, err := prod.Page(ctx, q, p, asOf, after, limit)
				if err != nil {
					t.Fatal(err)
				}
				preview = append(preview, rows...)
				after = last
				if len(rows) < limit {
					break
				}
			}

			var file [][]string
			if err := prod.Produce(ctx, q, p, asOf, func(r []string) error {
				file = append(file, r)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if !slices.EqualFunc(preview, file, slices.Equal[[]string]) {
				t.Fatalf("pages (%d rows) differ from Produce (%d rows)", len(preview), len(file))
			}

			var buf bytes.Buffer
			w, err := csvexport.NewWriter(&buf, prod.Header())
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range file {
				if err := w.Write(r); err != nil {
					t.Fatal(err)
				}
			}
			if err := w.Flush(); err != nil {
				t.Fatal(err)
			}
			recs, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(buf.Bytes(), []byte(csvexport.BOM)))).ReadAll()
			if err != nil {
				t.Fatal(err)
			}
			if len(recs) != len(preview)+1 {
				t.Fatalf("csv has %d records, want header + %d", len(recs), len(preview))
			}
			for i, cells := range preview {
				want := make([]string, len(cells))
				for j, s := range cells {
					want[j] = csvexport.CSVSafe(s)
				}
				if !slices.Equal(recs[i+1], want) {
					t.Fatalf("file row %d = %q, want CSVSafe(preview) %q", i, recs[i+1], want)
				}
			}
		}))
	}
}

// Feature: export-menu-revamp, Property 3 (page part): keyset paging is bounded and complete.
func TestProperty3KeysetPageBounded(t *testing.T) {
	asOf := time.Date(2026, 10, 30, 8, 0, 0, 0, wib)
	for _, c := range propCases() {
		t.Run(c.name, rapid.MakeCheck(func(t *rapid.T) {
			q, prod, p, n := propSetup(t, c)
			limit := rapid.IntRange(1, 100).Draw(t, "limit")
			after := rapid.Int64Range(0, int64(3*n+3)).Draw(t, "after")
			ctx := context.Background()

			// Everything after `after`, unbounded: the expected sequence.
			_, _, err := prod.Page(ctx, q, p, asOf, after, 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			all := slices.Clone(q.served)

			rows, last, err := prod.Page(ctx, q, p, asOf, after, limit)
			if err != nil {
				t.Fatal(err)
			}
			got := q.served
			if len(rows) > limit || len(got) != len(rows) {
				t.Fatalf("got %d rows (%d ids), limit %d", len(rows), len(got), limit)
			}
			for i, id := range got {
				if id <= after || i > 0 && id <= got[i-1] {
					t.Fatalf("ids %v not strictly ascending above after=%d", got, after)
				}
			}
			if want := all[:min(limit, len(all))]; !slices.Equal(got, want) {
				t.Fatalf("ids %v, want first %d of %v", got, limit, all)
			}
			wantLast := after
			if len(got) > 0 {
				wantLast = got[len(got)-1]
			}
			if last != wantLast {
				t.Fatalf("lastID = %d, want %d", last, wantLast)
			}
			// A short page means nothing is left; a full page may or may not have more.
			if len(rows) < limit && len(all) > len(rows) {
				t.Fatalf("short page (%d < %d) but %d rows remain", len(rows), limit, len(all)-len(rows))
			}
		}))
	}
}
