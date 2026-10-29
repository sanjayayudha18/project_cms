package csvexport

import (
	"bytes"
	"encoding/csv"
	"os"
	"slices"
	"strings"
	"testing"
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

func TestImportableNamesCoverSixEntities(t *testing.T) {
	want := []string{ATMAssignments, ATMs, VendorBranches, VendorPICs, VendorVaults, Vendors}
	if got := ImportableNames(); !slices.Equal(got, want) {
		t.Errorf("ImportableNames = %v, want %v", got, want)
	}
	golden := mustRead(t, "testdata/headers.golden")
	for _, n := range want {
		if !strings.Contains(golden, n+" header: ") {
			t.Errorf("headers.golden lacks %s", n)
		}
	}
	// legacy headers are covered by golden for the entities that have them.
	for _, n := range []string{VendorBranches, VendorPICs, ATMAssignments} {
		if !strings.Contains(golden, n+" legacy[0]: ") || !strings.Contains(golden, n+" legacy fill: ") {
			t.Errorf("headers.golden lacks legacy data for %s", n)
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

func TestCellsMatchHeaderAndIDFirst(t *testing.T) {
	cases := map[string][]string{
		Vendors:        VendorCells(VendorRow{ID: 7}),
		VendorBranches: VendorBranchCells(VendorBranchRow{ID: 7}),
		VendorVaults:   VendorVaultCells(VendorVaultRow{ID: 7}),
		VendorPICs:     VendorPICCells(VendorPICRow{ID: 7}),
		ATMs:           ATMCells(ATMRow{ID: 7}),
		ATMAssignments: ATMAssignmentCells(ATMAssignmentRow{ID: 7}),
	}
	if len(cases) != len(ImportableNames()) {
		t.Fatalf("cells cases (%d) do not cover every importable entity", len(cases))
	}
	for n, cells := range cases {
		e, ok := Lookup(n)
		if !ok {
			t.Fatalf("no entity %s", n)
		}
		if len(cells) != len(e.Header) {
			t.Errorf("%s: %d cells, header has %d", n, len(cells), len(e.Header))
		}
		if e.Header[0] != "id" || cells[0] != "7" {
			t.Errorf("%s: id not first (header[0]=%q cells[0]=%q)", n, e.Header[0], cells[0])
		}
	}
	if got := VendorPICCells(VendorPICRow{})[9]; got != PICRolesNone {
		t.Errorf("empty roles cell = %q", got)
	}
	if got := VendorPICCells(VendorPICRow{Roles: []string{"replenish", "vault"}})[9]; got != "replenish;vault" {
		t.Errorf("roles cell = %q", got)
	}
}

func mustRead(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
