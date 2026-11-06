package csvexport

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestHeadersMatchGolden(t *testing.T) {
	var b strings.Builder
	for _, name := range ImportableNames() {
		e, _ := Lookup(name)
		fmt.Fprintf(&b, "%s header: %s\n", name, strings.Join(e.Header, ","))
		for i, h := range e.LegacyHeaders {
			fmt.Fprintf(&b, "%s legacy[%d]: %s\n", name, i, strings.Join(h, ","))
		}
		if len(e.LegacyFill) > 0 {
			cols := make([]string, 0, len(e.LegacyFill))
			for c, v := range e.LegacyFill {
				cols = append(cols, c+"="+v)
			}
			slices.Sort(cols)
			fmt.Fprintf(&b, "%s legacy fill: %s\n", name, strings.Join(cols, ","))
		}
	}
	want, err := os.ReadFile("testdata/headers.golden")
	if err != nil {
		t.Fatal(err)
	}
	if b.String() != string(want) {
		t.Errorf("headers differ from golden:\n got: %q\nwant: %q", b.String(), want)
	}
}

func TestLookupReturnsCopyAndHidesExportOnly(t *testing.T) {
	e, _ := Lookup(VendorBranches)
	e.Header[0] = "x"
	e.LegacyHeaders[0][0] = "x"
	e.LegacyFill["category"] = "x"
	if f, _ := Lookup(VendorBranches); f.Header[0] != "id" || f.LegacyHeaders[0][0] != "id" || f.LegacyFill["category"] != KeepCurrentCell {
		t.Error("Lookup leaked the internal contract")
	}
	if _, ok := Lookup(PackagePrices); ok {
		t.Error("export-only entity must not be importable")
	}
	if _, ok := LookupExportOnly(PackagePrices); !ok {
		t.Error("package-prices missing")
	}
}

func TestVendorPICContract(t *testing.T) {
	base := []string{"id", "vendor_code", "branch_code", "name", "position", "phone", "email", "is_notification_recipient", "is_active"}
	e, ok := Lookup(VendorPICs)
	if !ok {
		t.Fatal("vendor-pics must be importable")
	}
	if want := append(slices.Clone(base), "roles", "is_director"); !slices.Equal(e.Header, want) {
		t.Errorf("header %v, want %v", e.Header, want)
	}
	if len(e.LegacyHeaders) != 2 ||
		!slices.Equal(e.LegacyHeaders[0], append(slices.Clone(base), "roles")) ||
		!slices.Equal(e.LegacyHeaders[1], base) {
		t.Errorf("legacy headers %v", e.LegacyHeaders)
	}
	if len(e.LegacyFill) != 2 || e.LegacyFill["roles"] != "" || e.LegacyFill["is_director"] != KeepCurrentCell {
		t.Errorf("legacy fill %v", e.LegacyFill)
	}

	// Mutating a returned copy must not leak into the next Lookup.
	e.Header[0], e.LegacyHeaders[1][0] = "x", "x"
	e.LegacyFill["is_director"] = "x"
	f, _ := Lookup(VendorPICs)
	if f.Header[0] != "id" || f.LegacyHeaders[1][0] != "id" || f.LegacyFill["is_director"] != KeepCurrentCell {
		t.Error("Lookup leaked the internal vendor-pics contract")
	}
}

func TestVendorPICCellsIsDirector(t *testing.T) {
	r := VendorPICRow{ID: 7, VendorCode: "V", BranchCode: "B", Name: "N", IsActive: true, Roles: []string{"replenish"}}
	for _, dir := range []bool{true, false} {
		r.IsDirector = dir
		cells := VendorPICCells(r)
		e, _ := Lookup(VendorPICs)
		if len(cells) != len(e.Header) {
			t.Fatalf("cells %d != header %d", len(cells), len(e.Header))
		}
		want := "false"
		if dir {
			want = "true"
		}
		if got := cells[len(cells)-1]; got != want {
			t.Errorf("is_director cell = %q, want %q", got, want)
		}
	}
}

func TestVendorsContractUnchanged(t *testing.T) {
	e, _ := Lookup(Vendors)
	for _, c := range e.Header {
		if strings.Contains(c, "director") || strings.Contains(c, "pks") {
			t.Errorf("vendors header must not carry PKS/director column %q", c)
		}
	}
	want, err := os.ReadFile("testdata/headers.golden")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(want), "vendors header: "+strings.Join(e.Header, ",")+"\n") {
		t.Error("vendors header differs from golden")
	}
}

func TestWriterAndTemplate(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewWriter(&buf, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write([]string{"=1+1", "ok"}); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), BOM+"a,b\n'=1+1,ok\n"; got != want {
		t.Errorf("got %q want %q", got, want)
	}

	buf.Reset()
	if err := WriteTemplate(&buf, Vendors); err != nil {
		t.Fatal(err)
	}
	e, _ := Lookup(Vendors)
	if got, want := buf.String(), BOM+strings.Join(e.Header, ",")+"\n"; got != want {
		t.Errorf("template got %q want %q", got, want)
	}
	if WriteTemplate(&buf, "nope") == nil || WriteTemplate(&buf, PackagePrices) == nil {
		t.Error("unknown/export-only entity must error")
	}
	for _, s := range []string{"=x", "@x", "\tx", "-abc", "+62 8", "-6.2", "a", ""} {
		if UnCSVSafe(CSVSafe(s)) != s {
			t.Errorf("round trip failed for %q", s)
		}
	}
}
