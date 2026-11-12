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
	for _, exportOnlyName := range []string{PackagePrices, DSRLocationMaps, VendorPks} {
		if _, ok := Lookup(exportOnlyName); ok {
			t.Errorf("export-only entity %q must not be importable", exportOnlyName)
		}
		eo, ok := LookupExportOnly(exportOnlyName)
		if !ok {
			t.Fatalf("%s missing from LookupExportOnly", exportOnlyName)
		}
		first := eo.Header[0]
		eo.Header[0] = "mutated"
		if again, _ := LookupExportOnly(exportOnlyName); again.Header[0] != first {
			t.Errorf("LookupExportOnly(%s) leaked internal slice", exportOnlyName)
		}
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

func TestVendorsContractRevamp(t *testing.T) {
	e, ok := Lookup(Vendors)
	if !ok {
		t.Fatal("vendors must be importable")
	}
	wantHeader := []string{"name", "legal_name", "npwp", "contact_email", "contact_phone", "hq_address", "is_active"}
	if !slices.Equal(e.Header, wantHeader) {
		t.Fatalf("vendors header = %v, want %v", e.Header, wantHeader)
	}
	for _, c := range e.Header {
		if c == "id" || c == "code" || strings.Contains(c, "director") || strings.Contains(c, "pks") {
			t.Errorf("vendors header must not carry id/code/PKS/director column %q", c)
		}
	}
	if e.ReadOnlyColumns != nil {
		t.Errorf("vendors ReadOnlyColumns = %v, want nil", e.ReadOnlyColumns)
	}
	wantLegacy0 := []string{"id", "code", "name", "legal_name", "npwp", "contact_email", "contact_phone", "hq_address", "is_active"}
	wantLegacy1 := slices.Concat(wantLegacy0, VendorReadOnlyColumns())
	if len(e.LegacyHeaders) != 2 || !slices.Equal(e.LegacyHeaders[0], wantLegacy0) || !slices.Equal(e.LegacyHeaders[1], wantLegacy1) {
		t.Errorf("vendors legacy headers = %v", e.LegacyHeaders)
	}
	cells := VendorCells(VendorRow{
		Name: "PT Abacus", LegalName: "PT Abacus Cash Solution", Npwp: "012345678901234",
		ContactEmail: "ops@abacus.co.id", ContactPhone: "+62 812-3456", HqAddress: "Jl. Sudirman", IsActive: true,
	})
	wantCells := []string{"PT Abacus", "PT Abacus Cash Solution", "012345678901234", "ops@abacus.co.id", "+62 812-3456", "Jl. Sudirman", "true"}
	if !slices.Equal(cells, wantCells) {
		t.Errorf("VendorCells = %v, want %v", cells, wantCells)
	}
	want, err := os.ReadFile("testdata/headers.golden")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(want), "vendors header: "+strings.Join(e.Header, ",")+"\n") {
		t.Error("vendors header differs from golden")
	}
}

func TestVendorBranchesContractAndCells(t *testing.T) {
	e, ok := Lookup(VendorBranches)
	if !ok {
		t.Fatal("vendor-branches must be importable")
	}
	wantHeader := []string{"id", "vendor_code", "branch_code", "branch_name", "region", "region_code", "category", "location_id", "is_active"}
	if !slices.Equal(e.Header, wantHeader) {
		t.Fatalf("vendor-branches header = %v, want %v", e.Header, wantHeader)
	}
	if slices.Contains(e.Header, "cis_limit_amount") {
		t.Error("vendor-branches header must not contain cis_limit_amount")
	}
	wantLegacyWithCis := []string{"id", "vendor_code", "branch_code", "branch_name", "region", "region_code", "category", "location_id", "cis_limit_amount", "is_active"}
	if len(e.LegacyHeaders) != 3 || !slices.Equal(e.LegacyHeaders[0], wantLegacyWithCis) {
		t.Errorf("vendor-branches LegacyHeaders = %v", e.LegacyHeaders)
	}
	if len(e.LegacyFill) != 2 || e.LegacyFill["region_code"] != KeepCurrentCell || e.LegacyFill["category"] != KeepCurrentCell {
		t.Errorf("vendor-branches LegacyFill = %v", e.LegacyFill)
	}
	cells := VendorBranchCells(VendorBranchRow{
		ID: 12, VendorCode: "ABA", BranchCode: "ABA_DPS", BranchName: "Abacus Bali",
		Region: "Bali", RegionCode: "DPS", Category: "ATM_CASH", LocationID: "9", IsActive: true,
	})
	wantCells := []string{"12", "ABA", "ABA_DPS", "Abacus Bali", "Bali", "DPS", "ATM_CASH", "9", "true"}
	if !slices.Equal(cells, wantCells) {
		t.Errorf("VendorBranchCells = %v, want %v", cells, wantCells)
	}
}

func TestVendorVaultsContractAndCells(t *testing.T) {
	e, ok := Lookup(VendorVaults)
	if !ok {
		t.Fatal("vendor-vaults must be importable")
	}
	wantHeader := []string{
		"no", "branch_name", "vault_name", "category", "operating_hours", "currency_code",
		"region", "alamat", "min_balance", "max_balance", "cis_limit_amount",
		"escrow_cash", "escrow_atm", "is_active",
	}
	if len(e.Header) != 14 || !slices.Equal(e.Header, wantHeader) {
		t.Fatalf("vendor-vaults header (%d) = %v, want %v", len(e.Header), e.Header, wantHeader)
	}
	for _, hidden := range []string{"id", "vendor_code", "branch_code", "vault_code", "location_id", "latitude", "longitude", "min_capacity_amount", "max_capacity_amount", "escrow_account"} {
		if slices.Contains(e.Header, hidden) {
			t.Errorf("vendor-vaults header must not contain hidden/legacy column %q", hidden)
		}
	}
	if len(e.LegacyHeaders) != 2 {
		t.Fatalf("vendor-vaults LegacyHeaders count = %d, want 2", len(e.LegacyHeaders))
	}
	for _, k := range []string{"alamat", "cis_limit_amount", "escrow_cash", "escrow_atm"} {
		if e.LegacyFill[k] != KeepCurrentCell {
			t.Errorf("vendor-vaults LegacyFill[%q] = %q, want %q", k, e.LegacyFill[k], KeepCurrentCell)
		}
	}
	cells := VendorVaultCells(VendorVaultRow{
		No: 1, BranchName: "Abacus Bali", VaultName: "ABA_Bali_V001", Category: "ATM_CASH",
		OperatingHours: "08:00-17:00", CurrencyCode: "IDR", Region: "Bali", Alamat: "Jl. Sunset Road No. 8",
		MinBalance: "500000000.00", MaxBalance: "10000000000.00", CisLimitAmount: "2500000000.00",
		EscrowCash: "001234567890", EscrowAtm: "009876543210", IsActive: true,
	})
	wantCells := []string{
		"1", "Abacus Bali", "ABA_Bali_V001", "ATM_CASH",
		"08:00-17:00", "IDR", "Bali", "Jl. Sunset Road No. 8",
		"500000000.00", "10000000000.00", "2500000000.00",
		"001234567890", "009876543210", "true",
	}
	if !slices.Equal(cells, wantCells) {
		t.Errorf("VendorVaultCells = %v, want %v", cells, wantCells)
	}
}

func TestVendorPksExportOnlyContract(t *testing.T) {
	if VendorPks != "vendor-pks" {
		t.Fatalf("VendorPks = %q, want %q", VendorPks, "vendor-pks")
	}
	if _, ok := Lookup(VendorPks); ok {
		t.Error("vendor-pks must not be importable")
	}
	e, ok := LookupExportOnly(VendorPks)
	if !ok {
		t.Fatal("vendor-pks must be registered in exportOnly")
	}
	wantHeader := []string{
		"vendor_code", "vendor_name",
		"director_name", "director_phone", "director_email",
		"pks_name", "pks_effective_date", "pks_expiry_date",
		"pks_validity_months", "pks_remaining_days", "pks_status",
	}
	if len(e.Header) != 11 || !slices.Equal(e.Header, wantHeader) {
		t.Errorf("vendor-pks header (%d) = %v, want %v", len(e.Header), e.Header, wantHeader)
	}
	if !slices.Contains(ExportOnlyNames(), VendorPks) {
		t.Errorf("ExportOnlyNames() = %v, missing %q", ExportOnlyNames(), VendorPks)
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
