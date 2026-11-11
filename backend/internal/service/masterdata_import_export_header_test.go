package service

import (
	"bytes"
	"encoding/csv"
	"reflect"
	"slices"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/pkg/csvexport"
)

// Spec vendor-export-pks, task 2.2: the vendor importer accepts the 18-column
// Export_Header file and drops the nine read-only cells (Properties 4, 5).

var (
	vendorImportCols = strings.Split(strings.TrimSpace(vendorHeader), ",")
	vendorExportCols = slices.Concat(vendorImportCols, csvexport.VendorReadOnlyColumns())
)

// v1Row is vendor V1 exactly as fakeImportRepo exports it (hq_address carries
// the CSVSafe apostrophe), so it imports as "no change".
const v1Row = "1,V1,Vendor Satu,PT Satu,012345678901234,a@v1.id,+62 812-345,'= Jl. Mawar,true"

func roCells(v string) string { return strings.TrimSuffix(strings.Repeat(v+",", 9), ",") }

func csvBytes(rows ...[]string) string {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	_ = w.WriteAll(rows)
	return b.String()
}

func dryRunRapid(t *rapid.T, csvText string) *ImportResult {
	res, err := NewMasterDataImporter(fakeImportRepo{}).DryRun(adminCtx(7), ExportVendors, strings.NewReader(csvText))
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	return res
}

// Property 4: an Export_Header file imports exactly like the Import_Header file
// with the same body, whatever the nine read-only cells hold.
func TestProperty_ExportHeaderImportsLikeImportHeader(t *testing.T) {
	cell := rapid.StringMatching(`[A-Za-z0-9 .@=+'-]{0,12}`)
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 5).Draw(t, "rows")
		body := make([][]string, 0, n+1)
		if rapid.Bool().Draw(t, "withV1") {
			name := rapid.SampledFrom([]string{"Vendor Satu", "Renamed", ""}).Draw(t, "v1name")
			row := strings.Split(v1Row, ",")
			row[2] = name
			body = append(body, row)
		}
		for i := 0; i < n; i++ {
			body = append(body, []string{"", "N" + string(rune('A'+i)), cell.Draw(t, "name"), "", "", "", "", "", "true"})
		}
		withRO := make([][]string, len(body))
		for i, r := range body {
			ro := make([]string, 9)
			for j := range ro {
				ro[j] = cell.Draw(t, "ro")
			}
			withRO[i] = slices.Concat(r, ro)
		}

		imp := dryRunRapid(t, csvBytes(slices.Concat([][]string{vendorImportCols}, body)...))
		exp := dryRunRapid(t, csvBytes(slices.Concat([][]string{vendorExportCols}, withRO)...))

		if !reflect.DeepEqual(imp.Preview, exp.Preview) {
			t.Fatalf("import-header preview %+v\nexport-header preview %+v", imp.Preview, exp.Preview)
		}
	})
}

// Property 5: any header that is neither Import_Header, Export_Header nor a
// legacy header is rejected at the header, with no row processed.
func TestProperty_UnknownVendorHeaderRejected(t *testing.T) {
	legacy := exportSpecs[ExportVendors].legacyHeaders
	rapid.Check(t, func(t *rapid.T) {
		base := rapid.SampledFrom([][]string{vendorImportCols, vendorExportCols}).Draw(t, "base")
		h := slices.Clone(base)
		switch rapid.IntRange(0, 4).Draw(t, "mutation") {
		case 0: // column missing
			i := rapid.IntRange(0, len(h)-1).Draw(t, "i")
			h = slices.Delete(h, i, i+1)
		case 1: // extra column
			h = append(h, "extra")
		case 2: // two columns swapped
			i := rapid.IntRange(0, len(h)-1).Draw(t, "i")
			j := rapid.IntRange(0, len(h)-1).Filter(func(j int) bool { return j != i }).Draw(t, "j")
			h[i], h[j] = h[j], h[i]
		case 3: // column renamed
			i := rapid.IntRange(0, len(h)-1).Draw(t, "i")
			h[i] += "_x"
		case 4: // read-only columns only
			h = csvexport.VendorReadOnlyColumns()
		}
		if slices.Equal(h, vendorImportCols) || slices.Equal(h, vendorExportCols) {
			t.Skip()
		}
		for _, l := range legacy {
			if slices.Equal(h, l) {
				t.Skip()
			}
		}

		res := dryRunRapid(t, csvBytes(h, slices.Concat(strings.Split(v1Row, ","), strings.Split(roCells("x"), ","))))

		p := res.Preview
		if len(p.Errors) != 1 || p.Errors[0].Row != 1 || p.Errors[0].Field != "header" || p.TotalRows != 0 || len(res.Rows) != 0 {
			t.Fatalf("header %v: want one header error and no rows, got %+v", h, p)
		}
		if !strings.Contains(p.Errors[0].Message, strings.Join(vendorImportCols, ",")) {
			t.Fatalf("message must name the Import_Header, got %q", p.Errors[0].Message)
		}
	})
}

func TestImport_VendorExportHeaderExamples(t *testing.T) {
	exportHdr := strings.Join(vendorExportCols, ",") + "\n"

	t.Run("export file untouched is zero changes", func(t *testing.T) {
		p := dryRun(t, ExportVendors, exportHdr+v1Row+","+roCells("")+"\n").Preview
		if len(p.Errors) != 0 || p.TotalRows != 1 || p.Unchanged != 1 || p.Creates != 0 || p.Updates != 0 || len(p.Changes) != 0 {
			t.Fatalf("want 1 unchanged, got %+v", p)
		}
	})

	t.Run("editing only read-only cells is zero changes", func(t *testing.T) {
		p := dryRun(t, ExportVendors, exportHdr+v1Row+",Budi,0812,b@x.id,PKS 2026,2026-01-01,2027-01-01,12,300,Aktif\n").Preview
		if len(p.Errors) != 0 || p.Unchanged != 1 || p.Updates != 0 || len(p.Changes) != 0 {
			t.Fatalf("want 1 unchanged, got %+v", p)
		}
	})

	t.Run("jumlah sel salah", func(t *testing.T) {
		cells := func(n int) string { return strings.TrimSuffix(strings.Repeat("a,", n), ",") }
		res := dryRun(t, ExportVendors, exportHdr+cells(9)+"\n"+cells(17)+"\n"+cells(19)+"\n")
		for line := 2; line <= 4; line++ {
			if !hasErr(res, line, "row") {
				t.Errorf("line %d: want a row error, got %+v", line, res.Preview.Errors)
			}
		}
		for _, e := range res.Preview.Errors {
			if !strings.Contains(e.Message, "jumlah kolom") || !strings.Contains(e.Message, "seharusnya 18") {
				t.Errorf("message %q must say 'jumlah kolom N, seharusnya 18'", e.Message)
			}
		}
		if res.Preview.ValidRows != 0 {
			t.Errorf("no row may pass, got %d valid", res.Preview.ValidRows)
		}
	})

	t.Run("import header file still accepted", func(t *testing.T) {
		p := dryRun(t, ExportVendors, vendorHeader+v1Row+"\n").Preview
		if len(p.Errors) != 0 || p.Unchanged != 1 {
			t.Fatalf("want 1 unchanged, got %+v", p)
		}
	})
}
