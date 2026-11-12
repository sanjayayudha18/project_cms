package csvexport

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// Property 5: CSVSafe is safe and reversible.
func FuzzCSVSafe(f *testing.F) {
	for _, s := range []string{"", "=x", "@x", "\tx", "\rx", "+62 812-3456", "-6.2", "-cmd", "+cmd", "-", "'=x", "'", "'a",
		"=1+1", "+", "(021) 555", "-1.2.3", "héllo", "日本語", "=日本", "\u00a0=x"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got := CSVSafe(s)
		if got != "" {
			switch got[0] {
			case '=', '@', '\t', '\r':
				t.Fatalf("CSVSafe(%q) = %q starts with a formula trigger", s, got)
			case '+', '-':
				if got != s || !plainNumberRe.MatchString(s) {
					t.Fatalf("CSVSafe(%q) = %q: +/- start allowed only for an unchanged plain number", s, got)
				}
			}
		}
		// Inputs starting with "'" are ambiguous by design (tasks.md 2.1 note).
		if !strings.HasPrefix(s, "'") {
			if back := UnCSVSafe(got); back != s {
				t.Fatalf("UnCSVSafe(CSVSafe(%q)) = %q", s, back)
			}
		}
	})
}

// Writer: BOM + header + rows survive an encoding/csv re-read.
func FuzzWriterRoundTrip(f *testing.F) {
	f.Add("a", "b", "c")
	f.Add("=x", "@y", "+cmd")
	f.Add("", "", "")
	f.Add("with,comma", "quote\"d", "multi\nline")
	f.Add("-6.2", "\tx", "日本語")
	f.Fuzz(func(t *testing.T, a, b, c string) {
		cells := []string{a, b, c}
		for _, s := range cells {
			// ambiguous by design; and encoding/csv rewrites CRLF to LF inside
			// quoted fields, which is the reader's behaviour, not the writer's.
			if strings.HasPrefix(s, "'") || strings.Contains(s, "\r\n") {
				t.Skip()
			}
		}
		header := []string{"h1", "h2", "h3"}
		var buf bytes.Buffer
		w, err := NewWriter(&buf, header)
		if err != nil {
			t.Fatal(err)
		}
		if err := w.Write(cells); err != nil {
			t.Fatal(err)
		}
		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if !strings.HasPrefix(out, BOM) {
			t.Fatalf("missing BOM: %q", out)
		}
		recs, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(out, BOM))).ReadAll()
		if err != nil {
			t.Fatalf("re-read %q: %v", out, err)
		}
		if len(recs) != 2 || !slices.Equal(recs[0], header) {
			t.Fatalf("unexpected records %q", recs)
		}
		for i, got := range recs[1] {
			if UnCSVSafe(got) != cells[i] {
				t.Fatalf("cell %d: got %q want %q", i, UnCSVSafe(got), cells[i])
			}
		}
	})
}

func TestImportableAndExportOnlyNames(t *testing.T) {
	want := []string{ATMAssignments, ATMs, VendorBranches, VendorPICs, VendorVaults, Vendors}
	if got := ImportableNames(); !slices.Equal(got, want) {
		t.Errorf("ImportableNames = %v, want %v", got, want)
	}
	wantExportOnly := []string{DSRLocationMaps, PackagePrices, VendorPks}
	if got := ExportOnlyNames(); !slices.Equal(got, wantExportOnly) {
		t.Errorf("ExportOnlyNames = %v, want %v", got, wantExportOnly)
	}
	golden := mustRead(t, "testdata/headers.golden")
	for _, n := range want {
		if !strings.Contains(golden, n+" header: ") {
			t.Errorf("headers.golden lacks %s", n)
		}
	}
	// legacy headers are covered by golden for the entities that have them.
	for _, n := range []string{VendorBranches, VendorPICs, VendorVaults, Vendors, ATMAssignments} {
		if !strings.Contains(golden, n+" legacy[0]: ") {
			t.Errorf("headers.golden lacks legacy[0] for %s", n)
		}
	}
	for _, n := range []string{VendorBranches, VendorPICs, VendorVaults, ATMAssignments} {
		if !strings.Contains(golden, n+" legacy fill: ") {
			t.Errorf("headers.golden lacks legacy fill for %s", n)
		}
	}
}

func TestWriteTemplateEveryImportable(t *testing.T) {
	for _, n := range ImportableNames() {
		e, _ := Lookup(n)
		var buf bytes.Buffer
		if err := WriteTemplate(&buf, n); err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		if got, want := buf.String(), BOM+strings.Join(e.Header, ",")+"\n"; got != want {
			t.Errorf("%s: got %q want %q", n, got, want)
		}
	}
	for _, n := range append(ExportOnlyNames(), "nope", "") {
		if err := WriteTemplate(&bytes.Buffer{}, n); err == nil {
			t.Errorf("WriteTemplate(%q) must error", n)
		}
	}
}

func TestCellsMatchHeaderAndFirstColumn(t *testing.T) {
	cases := map[string]struct {
		cells     []string
		firstCol  string
		firstCell string
	}{
		Vendors:        {VendorCells(VendorRow{Name: "Vendor Tujuh"}), "name", "Vendor Tujuh"},
		VendorBranches: {VendorBranchCells(VendorBranchRow{ID: 7}), "id", "7"},
		VendorVaults:   {VendorVaultCells(VendorVaultRow{No: 7}), "no", "7"},
		VendorPICs:     {VendorPICCells(VendorPICRow{ID: 7}), "id", "7"},
		ATMs:           {ATMCells(ATMRow{ID: 7}), "id", "7"},
		ATMAssignments: {ATMAssignmentCells(ATMAssignmentRow{ID: 7}), "id", "7"},
	}
	if len(cases) != len(ImportableNames()) {
		t.Fatalf("cells cases (%d) do not cover every importable entity", len(cases))
	}
	for n, tc := range cases {
		e, ok := Lookup(n)
		if !ok {
			t.Fatalf("no entity %s", n)
		}
		if len(tc.cells) != len(e.Header) {
			t.Errorf("%s: %d cells, header has %d", n, len(tc.cells), len(e.Header))
		}
		if e.Header[0] != tc.firstCol || tc.cells[0] != tc.firstCell {
			t.Errorf("%s: unexpected first col (header[0]=%q cells[0]=%q, want %q/%q)", n, e.Header[0], tc.cells[0], tc.firstCol, tc.firstCell)
		}
	}
	if got := VendorPICCells(VendorPICRow{})[9]; got != PICRolesNone {
		t.Errorf("empty roles cell = %q", got)
	}
	if got := VendorPICCells(VendorPICRow{Roles: []string{"replenish", "vault"}})[9]; got != "replenish;vault" {
		t.Errorf("roles cell = %q", got)
	}
}

// Property 1 & 2 (vendor-vaults-export-revamp): cell builders align with entity headers
// and preserve exact decimal money strings (numeric(20,2)) and 12-digit escrow identifiers
// without float conversion or truncation.
func TestProperty_RevampedCellsAlignAndPreserveMoneyStrings(t *testing.T) {
	decimalGen := rapid.Custom(func(t *rapid.T) string {
		if rapid.Bool().Draw(t, "empty") {
			return ""
		}
		whole := rapid.Int64Range(1, 999_999_999_999_999_999).Draw(t, "whole")
		frac := rapid.IntRange(0, 99).Draw(t, "frac")
		return fmt.Sprintf("%d.%02d", whole, frac)
	})
	escrowGen := rapid.Custom(func(t *rapid.T) string {
		if rapid.Bool().Draw(t, "empty") {
			return ""
		}
		return rapid.StringMatching(`^[0-9]{12}$`).Draw(t, "escrow12")
	})
	cleanTextGen := rapid.Custom(func(t *rapid.T) string {
		s := rapid.StringN(0, 24, -1).Draw(t, "text")
		s = strings.TrimPrefix(s, "'")
		return strings.ReplaceAll(s, "\r\n", "\n")
	})

	rapid.Check(t, func(t *rapid.T) {
		vaultRow := VendorVaultRow{
			No:             rapid.Int64Range(1, 1_000_000).Draw(t, "no"),
			BranchName:     cleanTextGen.Draw(t, "branchName"),
			VaultName:      rapid.StringMatching(`^[A-Z]{3}_[A-Za-z0-9_]+_V[0-9]{3}$`).Draw(t, "vaultName"),
			Category:       rapid.SampledFrom([]string{"ATM", "CASH", "ATM_CASH"}).Draw(t, "cat"),
			OperatingHours: cleanTextGen.Draw(t, "hours"),
			CurrencyCode:   "IDR",
			Region:         cleanTextGen.Draw(t, "region"),
			Alamat:         cleanTextGen.Draw(t, "alamat"),
			MinBalance:     decimalGen.Draw(t, "minBal"),
			MaxBalance:     decimalGen.Draw(t, "maxBal"),
			CisLimitAmount: decimalGen.Draw(t, "cisLimit"),
			EscrowCash:     escrowGen.Draw(t, "escrowCash"),
			EscrowAtm:      escrowGen.Draw(t, "escrowAtm"),
			IsActive:       rapid.Bool().Draw(t, "active"),
		}
		vaultEntity, _ := Lookup(VendorVaults)
		vaultCells := VendorVaultCells(vaultRow)
		if len(vaultCells) != len(vaultEntity.Header) {
			t.Fatalf("vault cells len %d != header len %d", len(vaultCells), len(vaultEntity.Header))
		}
		if vaultCells[8] != vaultRow.MinBalance || vaultCells[9] != vaultRow.MaxBalance || vaultCells[10] != vaultRow.CisLimitAmount {
			t.Fatalf("money string mutated: got (%q, %q, %q)", vaultCells[8], vaultCells[9], vaultCells[10])
		}
		if vaultCells[11] != vaultRow.EscrowCash || vaultCells[12] != vaultRow.EscrowAtm {
			t.Fatalf("escrow string mutated: got (%q, %q)", vaultCells[11], vaultCells[12])
		}

		// Round-trip through CSV Writer preserves exact decimals and 12-digit leading zeros.
		var buf bytes.Buffer
		w, err := NewWriter(&buf, vaultEntity.Header)
		if err != nil {
			t.Fatal(err)
		}
		if err := w.Write(vaultCells); err != nil {
			t.Fatal(err)
		}
		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}
		recs, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(buf.String(), BOM))).ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		if len(recs) != 2 || !slices.Equal(recs[0], vaultEntity.Header) {
			t.Fatalf("unexpected CSV header/rows: %v", recs)
		}
		for i, got := range recs[1] {
			if UnCSVSafe(got) != vaultCells[i] {
				t.Fatalf("col %d (%s): got %q, want %q", i, vaultEntity.Header[i], UnCSVSafe(got), vaultCells[i])
			}
		}

		branchEntity, _ := Lookup(VendorBranches)
		branchCells := VendorBranchCells(VendorBranchRow{
			ID:         rapid.Int64Range(1, 100_000).Draw(t, "branchID"),
			VendorCode: "ABA",
			BranchCode: "ABA_DPS",
			BranchName: cleanTextGen.Draw(t, "bName"),
			Region:     cleanTextGen.Draw(t, "bRegion"),
			RegionCode: "DPS",
			Category:   "ATM_CASH",
			LocationID: "5",
			IsActive:   rapid.Bool().Draw(t, "bActive"),
		})
		if len(branchCells) != len(branchEntity.Header) {
			t.Fatalf("branch cells len %d != header len %d", len(branchCells), len(branchEntity.Header))
		}

		vendorEntity, _ := Lookup(Vendors)
		vendorCells := VendorCells(VendorRow{
			Name:         cleanTextGen.Draw(t, "vName"),
			LegalName:    cleanTextGen.Draw(t, "vLegal"),
			Npwp:         "012345678901234",
			ContactEmail: "ops@vendor.co.id",
			ContactPhone: "+62 812-3456",
			HqAddress:    cleanTextGen.Draw(t, "vAddr"),
			IsActive:     rapid.Bool().Draw(t, "vActive"),
		})
		if len(vendorCells) != len(vendorEntity.Header) {
			t.Fatalf("vendor cells len %d != header len %d", len(vendorCells), len(vendorEntity.Header))
		}
	})
}

func mustRead(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
