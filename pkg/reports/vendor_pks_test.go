package reports

import (
	"bytes"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cimb-niaga/cms/pkg/csvexport"
)

func TestParseVendorPksRenewalMonths(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    int
		wantErr bool
	}{
		{"empty defaults to 9", "", 9, false},
		{"blank defaults to 9", "  ", 9, false},
		{"explicit 9", "9", 9, false},
		{"min bound", "1", 1, false},
		{"max bound", "24", 24, false},
		{"zero rejected", "0", 0, true},
		{"above max rejected", "25", 0, true},
		{"negative rejected", "-3", 0, true},
		{"non numeric rejected", "abc", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVendorPksRenewalMonths(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestTodayWIBDayBoundary(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want string
	}{
		{"16:59 UTC is 23:59 WIB, same day", time.Date(2026, 10, 31, 16, 59, 0, 0, time.UTC), "2026-10-31"},
		{"17:00 UTC is 00:00 WIB, next day", time.Date(2026, 10, 31, 17, 0, 0, 0, time.UTC), "2026-11-01"},
		{"input already in WIB", time.Date(2026, 11, 1, 0, 30, 0, 0, WIB), "2026-11-01"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TodayWIB(tt.now)
			if got.Format("2006-01-02") != tt.want {
				t.Errorf("got %s, want %s", got.Format("2006-01-02"), tt.want)
			}
			if got.Location() != time.UTC || got.Hour() != 0 || got.Minute() != 0 {
				t.Errorf("want UTC midnight, got %v", got)
			}
		})
	}
}

func TestVendorPksFileName(t *testing.T) {
	tests := []struct {
		now  time.Time
		want string
	}{
		{time.Date(2026, 10, 31, 16, 59, 0, 0, time.UTC), "vendor-pks-20261031.csv"},
		{time.Date(2026, 10, 31, 17, 0, 0, 0, time.UTC), "vendor-pks-20261101.csv"},
	}
	for _, tt := range tests {
		if got := VendorPksFileName(tt.now); got != tt.want {
			t.Errorf("%v: got %q, want %q", tt.now, got, tt.want)
		}
	}
}

func TestPksStatusesKeysAndLabels(t *testing.T) {
	want := map[string]string{"aktif": "Aktif", "need_renewal": "Need Renewal", "expired": "Expired",
		"terminated": "Terminated", "belum_diisi": "Belum diisi"}
	if len(PksStatuses) != len(want) {
		t.Fatalf("got %v", PksStatuses)
	}
	for k, v := range want {
		if PksStatuses[k] != v {
			t.Errorf("PksStatuses[%q] = %q, want %q", k, PksStatuses[k], v)
		}
	}
}

// Golden: the exact CSV lines written through the shared csvexport.Writer (which applies CSVSafe
// again — must stay idempotent so an apostrophe is never doubled).
func TestVendorPksCsvGolden(t *testing.T) {
	d := func(y int, m time.Month, day int) *time.Time {
		return ptr(time.Date(y, m, day, 0, 0, 0, 0, time.UTC))
	}
	rows := []VendorPksRow{
		{VendorCode: "ABACUS", VendorName: "PT Abacus Ñandú 雅加达",
			DirectorName: ptr("Budi Santoso"), DirectorPhone: ptr("0812 3456"), DirectorEmail: ptr("budi@abacus.co.id"),
			PksName: ptr("PKS Replenish 2026"), EffectiveDate: d(2026, 1, 1), ExpiryDate: d(2027, 6, 30),
			ValidityMonths: ptr(int32(18)), RemainingDays: ptr(int32(237)), PksStatus: ptr("aktif")},
		{VendorCode: "KEJAR", VendorName: `=HYPERLINK("x")`,
			DirectorName: ptr("@admin"), DirectorPhone: ptr("+62 811"), DirectorEmail: ptr("-x@y.id"),
			PksName: ptr("PKS, \"Koma\""), EffectiveDate: d(2026, 2, 3), ExpiryDate: d(2026, 8, 3),
			ValidityMonths: ptr(int32(6)), RemainingDays: ptr(int32(-4)), PksStatus: ptr("expired")},
		{VendorCode: "NEWV", VendorName: "PT Baru", PksStatus: ptr("belum_diisi")},
		{VendorCode: "T", VendorName: "PT T", PksName: ptr("PKS T"),
			EffectiveDate: d(2025, 1, 1), ExpiryDate: d(2026, 1, 1), ValidityMonths: ptr(int32(12)), PksStatus: ptr("terminated")},
		{VendorCode: "R", VendorName: "PT R", PksStatus: ptr("need_renewal")},
	}
	var buf bytes.Buffer
	w, err := csvexport.NewWriter(&buf, VendorPksHeader)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		cells := VendorPksCells(r)
		if len(cells) != len(VendorPksHeader) {
			t.Fatalf("%s: cells %d != header %d", r.VendorCode, len(cells), len(VendorPksHeader))
		}
		if err := w.Write(cells); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	want := csvexport.BOM + strings.Join([]string{
		strings.Join(VendorPksHeader, ","),
		"ABACUS,PT Abacus Ñandú 雅加达,Budi Santoso,0812 3456,budi@abacus.co.id,PKS Replenish 2026,2026-01-01,2027-06-30,18,237,Aktif",
		`KEJAR,"'=HYPERLINK(""x"")",'@admin,+62 811,'-x@y.id,"PKS, ""Koma""",2026-02-03,2026-08-03,6,-4,Expired`,
		"NEWV,PT Baru,,,,,,,,,Belum diisi",
		"T,PT T,,,,PKS T,2025-01-01,2026-01-01,12,,Terminated",
		"R,PT R,,,,,,,,,Need Renewal",
	}, "\n") + "\n"
	if got := buf.String(); got != want {
		t.Errorf("csv differs\n got: %q\nwant: %q", got, want)
	}
}

func TestVendorPksHeaderStable(t *testing.T) {
	want := []string{"Kode Vendor", "Nama Perusahaan", "Nama Direktur Utama", "Telepon Direktur Utama",
		"Email Direktur Utama", "Nama PKS", "Tanggal Efektif PKS", "Tanggal Expired PKS", "Masa Berlaku (bulan)",
		"Monitoring PKS (hari)", "Status PKS"}
	if !slices.Equal(VendorPksHeader, want) {
		t.Errorf("header %v", VendorPksHeader)
	}
}
