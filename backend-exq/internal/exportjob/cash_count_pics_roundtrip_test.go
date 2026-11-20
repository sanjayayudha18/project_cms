package exportjob

import (
	"slices"
	"testing"

	"github.com/cimb-niaga/cms/pkg/csvexport"
)

// uploadHeaderCashCountPICs is the documented 5.1 upload header (AC-5.1-12).
var uploadHeaderCashCountPICs = []string{"pic_role", "username", "region_code", "area_code"}

// AC-5.1-12: the downloaded CSV has the same columns as the upload file, so a
// download can be re-uploaded as is.
func TestCashCountPicsExportHeaderEqualsUploadHeader(t *testing.T) {
	p, ok := DefaultProducers()[KindCashCountPics][csvexport.CashCountPICs]
	if !ok {
		t.Fatal("cash_count_pics producer not registered")
	}
	if got := p.Header(); !slices.Equal(got, uploadHeaderCashCountPICs) {
		t.Errorf("export header %v, want upload header %v", got, uploadHeaderCashCountPICs)
	}

	// Once task 9.1 makes the entity importable, the importer's header must match too.
	if e, ok := csvexport.Lookup(csvexport.CashCountPICs); ok {
		if got := e.ExportHeader(); !slices.Equal(got, p.Header()) {
			t.Errorf("importer header %v != export header %v", got, p.Header())
		}
	}
}
