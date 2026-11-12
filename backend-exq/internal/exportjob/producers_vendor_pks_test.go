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
// - vendorPksMasterProducer (entity vendor-pks) emits 11 columns: vendor_code, vendor_name + the nine
//   director/PKS cells from reports.VendorPksDetailCells.
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

func pgDate(t *rapid.T, label string) pgtype.Date {
	d := time.Date(rapid.IntRange(2020, 2035).Draw(t, label+"Y"), time.Month(rapid.IntRange(1, 12).Draw(t, label+"M")),
		rapid.IntRange(1, 28).Draw(t, label+"D"), 0, 0, 0, 0, time.UTC)
	return pgtype.Date{Time: d, Valid: true}
}

func TestProperty1VendorTailEqualsPksDetailCells(t *testing.T) {
	statuses := []string{""} // "" = no active PKS (INTERNAL or FLM without PKS)
	for k := range reports.PksStatuses {
		statuses = append(statuses, k)
	}
	slices.Sort(statuses)
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
		if rapid.Bool().Draw(t, "hasPks") {
			r.PksName = optText("pn").Draw(t, "pn")
			r.EffectiveDate, r.ExpiryDate = pgDate(t, "eff"), pgDate(t, "exp")
			r.ValidityMonths = rapid.Int32Range(1, 120).Draw(t, "vm")
			r.RemainingDays = rapid.Int32Range(-400, 4000).Draw(t, "rd")
			r.PksStatus = rapid.SampledFrom(statuses).Draw(t, "status")
		}

		// Expected tail built from the row independently of pksDetailRow.
		want := reports.VendorPksRow{DirectorName: r.DirectorName, DirectorPhone: r.DirectorPhone, DirectorEmail: r.DirectorEmail, PksName: r.PksName}
		if r.ExpiryDate.Valid {
			eff, exp, vm, rd := r.EffectiveDate.Time, r.ExpiryDate.Time, r.ValidityMonths, r.RemainingDays
			want.EffectiveDate, want.ExpiryDate, want.ValidityMonths, want.RemainingDays = &eff, &exp, &vm, &rd
		}
		if r.PksStatus != "" {
			s := r.PksStatus
			want.PksStatus = &s
		}
		wantTail := reports.VendorPksDetailCells(want)
		wantHead := []string{r.Code, r.Name}

		q := &propQ{
			vendorPks: []propEntry[repository.ExportVendorsWithPksBatchRow]{{id: 1, row: r, active: r.IsActive}},
			vendors: []propEntry[repository.ExportVendorsBatchRow]{{id: 1, row: repository.ExportVendorsBatchRow{
				ID: r.ID, Code: r.Code, Name: r.Name, LegalName: r.LegalName,
				Npwp: r.Npwp, ContactEmail: r.ContactEmail, ContactPhone: r.ContactPhone,
				HqAddress: r.HqAddress, IsActive: r.IsActive,
			}, active: r.IsActive}},
		}
		prod := vendorPksMasterProducer(9)
		rows, _, err := prod.(PreviewProducer).Page(context.Background(), q, Params{StatusFilter: "all"}, asOf, 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || len(rows[0]) != len(prod.Header()) || len(prod.Header()) != 11 {
			t.Fatalf("got %d rows, want 1 row of 11 cells (header=%d)", len(rows), len(prod.Header()))
		}
		n := len(wantHead)
		if !slices.Equal(rows[0][:n], wantHead) {
			t.Fatalf("head = %q, want %q", rows[0][:n], wantHead)
		}
		if !slices.Equal(rows[0][n:], wantTail) || len(wantTail) != 9 {
			t.Fatalf("tail = %q, want %q", rows[0][n:], wantTail)
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

// "Today" for the PKS status is the WIB calendar day of asOf, not the UTC day.
func TestVendorsProducerTodayIsWIBDate(t *testing.T) {
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
			f := &fakeQ{n: 1}
			rows, _, err := vendorPksMasterProducer(7).(PreviewProducer).Page(context.Background(), f, Params{StatusFilter: "active"}, tt.asOf, 0, 10)
			if err != nil || len(rows) != 1 {
				t.Fatalf("rows=%d err=%v", len(rows), err)
			}
			got := f.calls[0].(repository.ExportVendorsWithPksBatchParams)
			if !got.Today.Valid || !got.Today.Time.Equal(tt.want) {
				t.Fatalf("Today = %v, want %v", got.Today, tt.want)
			}
			if got.RenewalMonths != 7 {
				t.Fatalf("RenewalMonths = %d, want 7", got.RenewalMonths)
			}
		})
	}
}
