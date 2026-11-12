package csvexport

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

var wantVendorHeader = []string{"name", "legal_name", "npwp", "contact_email", "contact_phone", "hq_address", "is_active"}

var wantVendorReadOnly = []string{
	"director_name", "director_phone", "director_email",
	"pks_name", "pks_effective_date", "pks_expiry_date",
	"pks_validity_months", "pks_remaining_days", "pks_status",
}

func TestVendorExportHeaderHasSevenProfileColumns(t *testing.T) {
	e, ok := Lookup(Vendors)
	if !ok {
		t.Fatal("vendors must be importable")
	}
	if e.ReadOnlyColumns != nil {
		t.Fatalf("vendors ReadOnlyColumns = %v, want nil", e.ReadOnlyColumns)
	}
	got := e.ExportHeader()
	if len(got) != 7 {
		t.Fatalf("export header has %d columns, want 7", len(got))
	}
	if !slices.Equal(got, wantVendorHeader) {
		t.Errorf("export header %v, want %v", got, wantVendorHeader)
	}
	if !slices.Equal(VendorReadOnlyColumns(), wantVendorReadOnly) {
		t.Errorf("VendorReadOnlyColumns %v", VendorReadOnlyColumns())
	}
}

func TestVendorExportContractMutationDoesNotLeak(t *testing.T) {
	e, _ := Lookup(Vendors)
	h := e.ExportHeader()
	h[0], h[len(h)-1] = "x", "x"
	e.Header[0] = "x"
	e.LegacyHeaders[0][0] = "x"
	VendorReadOnlyColumns()[1] = "x"

	f, _ := Lookup(Vendors)
	if !slices.Equal(f.Header, wantVendorHeader) || f.ReadOnlyColumns != nil {
		t.Error("mutating a returned copy changed the contract")
	}
	if !slices.Equal(f.ExportHeader(), wantVendorHeader) {
		t.Error("ExportHeader changed after mutation")
	}
	if f.LegacyHeaders[0][0] != "id" {
		t.Error("LegacyHeaders leaked its backing slice")
	}
	if !slices.Equal(VendorReadOnlyColumns(), wantVendorReadOnly) {
		t.Error("VendorReadOnlyColumns leaked its backing slice")
	}
}

func TestAllEntitiesHaveNoReadOnlyColumns(t *testing.T) {
	check := func(e Entity) {
		t.Helper()
		if e.ReadOnlyColumns != nil {
			t.Errorf("%s: ReadOnlyColumns = %v, want nil", e.Name, e.ReadOnlyColumns)
		}
		if got := e.ExportHeader(); !slices.Equal(got, e.Header) {
			t.Errorf("%s: ExportHeader %v != Header %v", e.Name, got, e.Header)
		}
	}
	for _, n := range ImportableNames() {
		e, _ := Lookup(n)
		check(e)
	}
	for _, n := range ExportOnlyNames() {
		e, _ := LookupExportOnly(n)
		check(e)
	}
}

func TestVendorImportTemplateHasSevenColumns(t *testing.T) {
	e, _ := Lookup(Vendors)
	if !slices.Equal(e.Header, wantVendorHeader) {
		t.Errorf("vendors Header %v, want %v", e.Header, wantVendorHeader)
	}
	var buf bytes.Buffer
	if err := WriteTemplate(&buf, Vendors); err != nil {
		t.Fatal(err)
	}
	first, _, _ := strings.Cut(strings.TrimPrefix(buf.String(), "\xef\xbb\xbf"), "\n")
	if got := strings.Split(strings.TrimSpace(first), ","); !slices.Equal(got, wantVendorHeader) {
		t.Errorf("template header %v, want %v", got, wantVendorHeader)
	}
}
