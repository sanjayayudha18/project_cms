package reports

import (
	"slices"
	"testing"
	"time"

	"github.com/cimb-niaga/cms/pkg/csvexport"
	"pgregory.net/rapid"
)

// Text that hits the CSVSafe prefix rules as often as plain text.
func textGen() *rapid.Generator[string] {
	return rapid.OneOf(
		rapid.String(),
		rapid.SampledFrom([]string{"=SUM(A1)", "+1", "-1", "@x", "\tx", "\rx", "Ani"}),
	)
}

func optGen[T any](g *rapid.Generator[T]) *rapid.Generator[*T] {
	return rapid.OneOf(rapid.Just[*T](nil), rapid.Map(g, func(v T) *T { return &v }))
}

func vendorPksRowGen() *rapid.Generator[VendorPksRow] {
	date := rapid.Map(rapid.Int64Range(0, 4_000_000_000), func(s int64) time.Time { return time.Unix(s, 0).UTC() })
	statuses := make([]string, 0, len(PksStatuses))
	for k := range PksStatuses {
		statuses = append(statuses, k)
	}
	slices.Sort(statuses)
	return rapid.Custom(func(t *rapid.T) VendorPksRow {
		return VendorPksRow{
			VendorCode:     textGen().Draw(t, "code"),
			VendorName:     textGen().Draw(t, "name"),
			DirectorName:   optGen(textGen()).Draw(t, "dirName"),
			DirectorPhone:  optGen(textGen()).Draw(t, "dirPhone"),
			DirectorEmail:  optGen(textGen()).Draw(t, "dirEmail"),
			PksName:        optGen(textGen()).Draw(t, "pksName"),
			EffectiveDate:  optGen(date).Draw(t, "eff"),
			ExpiryDate:     optGen(date).Draw(t, "exp"),
			ValidityMonths: optGen(rapid.Int32Range(-1000, 1000)).Draw(t, "months"),
			RemainingDays:  optGen(rapid.Int32Range(-100000, 100000)).Draw(t, "days"),
			PksStatus:      optGen(rapid.SampledFrom(statuses)).Draw(t, "status"),
		}
	})
}

// Property 6: VendorPksCells = CSVSafe(code,name) + CSVSafe over VendorPksDetailCells.
func TestVendorPksCellsProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		r := vendorPksRowGen().Draw(t, "row")
		got := VendorPksCells(r)
		detail := VendorPksDetailCells(r)
		if len(got) != len(VendorPksHeader) || len(detail) != len(VendorPksHeader)-2 {
			t.Fatalf("lengths: cells %d detail %d header %d", len(got), len(detail), len(VendorPksHeader))
		}
		if got[0] != csvexport.CSVSafe(r.VendorCode) || got[1] != csvexport.CSVSafe(r.VendorName) {
			t.Fatalf("code/name cells %q %q", got[0], got[1])
		}
		for i, c := range detail {
			if got[i+2] != csvexport.CSVSafe(c) {
				t.Fatalf("cell %d: %q != CSVSafe(%q)", i+2, got[i+2], c)
			}
		}
	})
}

func TestVendorPksDetailCellsRaw(t *testing.T) {
	// Raw: no CSVSafe quote prefix.
	r := VendorPksRow{DirectorName: ptr("=1+1"), DirectorPhone: ptr("+62"), PksName: ptr("@x"),
		PksStatus: ptr("aktif")}
	got := VendorPksDetailCells(r)
	want := []string{"=1+1", "+62", "", "@x", "", "", "", "", "Aktif"}
	if !slices.Equal(got, want) {
		t.Errorf("raw cells = %q; want %q", got, want)
	}
	// All nil: nine empty cells.
	want = make([]string, 9)
	if got := VendorPksDetailCells(VendorPksRow{}); !slices.Equal(got, want) {
		t.Errorf("nil row cells = %q; want nine empty", got)
	}
}
