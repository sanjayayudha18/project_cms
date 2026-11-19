package exportjob

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
	"github.com/cimb-niaga/cms/pkg/csvexport"
	"github.com/cimb-niaga/cms/pkg/reports"
)

// Feature: vendor-vaults-export-revamp, Property 1 (unit part):
// - vendorPksMasterProducer (entity vendor-pks) emits 5 columns: vendor_code, vendor_name + 3 director cells.
// - vendorsProducer (entity vendors) emits 7 vendor profile columns without id, code, director, or PKS.

func optText(label string) *rapid.Generator[*string] {
	return rapid.Custom(func(t *rapid.T) *string {
		if rapid.Bool().Draw(t, label+"Nil") {
			return nil
		}
		s := propText.Draw(t, label)
		return &s
	})
}

func TestProperty1VendorPksMasterIsCodeNameDirector(t *testing.T) {
	asOf := time.Date(2026, 10, 30, 8, 0, 0, 0, wib)
	rapid.Check(t, func(t *rapid.T) {
		r := repository.ExportVendorsWithPksBatchRow{
			ID: 1, Code: propText.Draw(t, "code"), Name: propText.Draw(t, "name"), LegalName: propText.Draw(t, "legal"),
			Npwp: propText.Draw(t, "npwp"), ContactEmail: propText.Draw(t, "email"), ContactPhone: propText.Draw(t, "phone"),
			HqAddress: propText.Draw(t, "addr"), IsActive: rapid.Bool().Draw(t, "active"),
		}
		if rapid.Bool().Draw(t, "hasDirector") {
			r.DirectorName, r.DirectorPhone, r.DirectorEmail = optText("dn").Draw(t, "dn"), optText("dp").Draw(t, "dp"), optText("de").Draw(t, "de")
		}
		// Expected: code, name + the three director cells, built independently of vendorPksMasterCells.
		want := []string{r.Code, r.Name, derefStr(r.DirectorName), derefStr(r.DirectorPhone), derefStr(r.DirectorEmail)}

		q := &propQ{
			vendorPks: []propEntry[repository.ExportVendorsWithPksBatchRow]{{id: 1, row: r, active: r.IsActive}},
			vendors: []propEntry[repository.ExportVendorsBatchRow]{{id: 1, row: repository.ExportVendorsBatchRow{
				ID: r.ID, Code: r.Code, Name: r.Name, LegalName: r.LegalName,
				Npwp: r.Npwp, ContactEmail: r.ContactEmail, ContactPhone: r.ContactPhone,
				HqAddress: r.HqAddress, IsActive: r.IsActive,
			}, active: r.IsActive}},
		}
		prod := vendorPksMasterProducer()
		rows, _, err := prod.(PreviewProducer).Page(context.Background(), q, Params{StatusFilter: "all"}, asOf, 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(prod.Header()) != 5 || len(rows) != 1 || !slices.Equal(rows[0], want) {
			t.Fatalf("rows = %q (header %d), want [%q]", rows, len(prod.Header()), want)
		}

		// Vendors producer emits 7 profile columns without id, code, director, or PKS.
		vProd := vendorsProducer(false)
		vRows, _, err := vProd.(PreviewProducer).Page(context.Background(), q, Params{StatusFilter: "all"}, asOf, 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		wantProfile := csvexport.VendorCells(csvexport.VendorRow{
			Name: r.Name, LegalName: r.LegalName, Npwp: r.Npwp,
			ContactEmail: r.ContactEmail, ContactPhone: r.ContactPhone, HqAddress: r.HqAddress, IsActive: r.IsActive,
		})
		if len(vRows) != 1 || len(vProd.Header()) != 7 || !slices.Equal(vRows[0], wantProfile) {
			t.Fatalf("vendor profile row = %q, want %q", vRows, wantProfile)
		}
	})
}

// pksQ captures the report producer's queries and serves canned pages.
type pksQ struct {
	*fakeQ
	batchArgs   []repository.ExportVendorPksBatchParams
	missingArgs []repository.ExportVendorPksMissingBatchParams
	batch       []repository.ExportVendorPksBatchRow
	missing     []repository.ExportVendorPksMissingBatchRow
}

func (q *pksQ) ExportVendorPksBatch(_ context.Context, a repository.ExportVendorPksBatchParams) ([]repository.ExportVendorPksBatchRow, error) {
	q.batchArgs = append(q.batchArgs, a)
	return q.batch, nil
}

func (q *pksQ) ExportVendorPksMissingBatch(_ context.Context, a repository.ExportVendorPksMissingBatchParams) ([]repository.ExportVendorPksMissingBatchRow, error) {
	q.missingArgs = append(q.missingArgs, a)
	return q.missing, nil
}

func runVendorPks(t *testing.T, q Querier, rp ReportParams, asOf time.Time) [][]string {
	t.Helper()
	var out [][]string
	err := vendorPksProducer(7).Produce(context.Background(), q, Params{Kind: KindVendorPks, Report: &rp}, asOf,
		func(c []string) error { out = append(out, c); return nil })
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// "Today" for the PKS status is the WIB calendar day of asOf, not the UTC day.
func TestVendorPksProducerTodayIsWIBDate(t *testing.T) {
	tests := []struct {
		name string
		asOf time.Time
		want time.Time
	}{
		{"23:30 WIB stays the same day", time.Date(2026, 10, 30, 23, 30, 0, 0, wib), time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)},
		{"00:30 WIB is the next day (still previous day in UTC)", time.Date(2026, 10, 31, 0, 30, 0, 0, wib), time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &pksQ{fakeQ: &fakeQ{}}
			runVendorPks(t, q, ReportParams{}, tt.asOf)
			got := q.batchArgs[0]
			if !got.Today.Valid || !got.Today.Time.Equal(tt.want) {
				t.Fatalf("Today = %v, want %v", got.Today, tt.want)
			}
			if got.RenewalMonths != 7 {
				t.Fatalf("RenewalMonths = %d, want 7", got.RenewalMonths)
			}
		})
	}
}

// Req 6.2/A3: belum_diisi + type switches to the missing query, one line per vendor with only
// code/name/director/type/status filled; any other filter stays on the active-PKS query.
func TestVendorPksProducerBelumDiisiAndFilterRouting(t *testing.T) {
	asOf := time.Date(2026, 11, 18, 8, 0, 0, 0, wib)
	cit, bd, aktif := "CIT", "belum_diisi", "aktif"
	dn := "=Budi"

	q := &pksQ{fakeQ: &fakeQ{}, missing: []repository.ExportVendorPksMissingBatchRow{
		{VendorCode: "NEWV", VendorName: "PT Baru", DirectorName: &dn, PksTypeCode: "CIT"},
	}}
	got := runVendorPks(t, q, ReportParams{PksType: &cit, PksStatus: &bd}, asOf)
	want := reports.VendorPksCells(reports.VendorPksRow{VendorCode: "NEWV", VendorName: "PT Baru", DirectorName: &dn,
		PksTypeCode: &cit, PksStatus: &bd})
	if len(got) != 1 || !slices.Equal(got[0], want) || len(got[0]) != len(reports.VendorPksHeader) {
		t.Fatalf("got %q, want [%q]", got, want)
	}
	if got[0][2] != "'=Budi" || got[0][5] != "CIT" || got[0][12] != "Belum diisi" || got[0][6] != "" {
		t.Errorf("cells = %q", got[0])
	}
	if len(q.batchArgs) != 0 || q.missingArgs[0].PksType != "CIT" {
		t.Errorf("batch calls %d, missing args %+v", len(q.batchArgs), q.missingArgs)
	}

	q = &pksQ{fakeQ: &fakeQ{}}
	runVendorPks(t, q, ReportParams{PksType: &cit, PksStatus: &aktif}, asOf)
	if len(q.missingArgs) != 0 || len(q.batchArgs) != 1 || *q.batchArgs[0].PksType != "CIT" || *q.batchArgs[0].PksStatus != "aktif" {
		t.Errorf("aktif routing: missing %d batch %+v", len(q.missingArgs), q.batchArgs)
	}
}

// One output line per active PKS, with CSVSafe on type/WOD and the 13-column width.
func TestVendorPksProducerRowPerPks(t *testing.T) {
	eff := pgtype.Date{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}
	exp := pgtype.Date{Time: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}
	q := &pksQ{fakeQ: &fakeQ{}, batch: []repository.ExportVendorPksBatchRow{
		{VendorCode: "A", VendorName: "PT A", PksTypeCode: "CIT", WodNumber: "=W1", PksName: "P1", EffectiveDate: eff, ExpiryDate: exp,
			ValidityMonths: 12, RemainingDays: 30, PksStatus: "aktif"},
		{VendorCode: "A", VendorName: "PT A", PksTypeCode: "FLM", WodNumber: "W2", PksName: "P2", EffectiveDate: eff, ExpiryDate: exp,
			ValidityMonths: 12, RemainingDays: -3, PksStatus: "expired"},
	}}
	got := runVendorPks(t, q, ReportParams{}, time.Date(2026, 11, 18, 8, 0, 0, 0, wib))
	if len(got) != 2 {
		t.Fatalf("rows = %d, want 2 (one per PKS)", len(got))
	}
	for _, c := range got {
		if len(c) != len(reports.VendorPksHeader) {
			t.Fatalf("cells %d != header %d", len(c), len(reports.VendorPksHeader))
		}
	}
	if got[0][5] != "CIT" || got[0][6] != "'=W1" || got[1][5] != "FLM" || got[1][12] != "Expired" || got[1][11] != "-3" {
		t.Errorf("rows = %q", got)
	}
}
