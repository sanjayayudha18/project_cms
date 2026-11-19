package reports

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestParsePksListFilter(t *testing.T) {
	tests := []struct {
		name                       string
		status, pksType, vendor, q string
		want                       PksListFilter
		wantErr                    bool
	}{
		{"empty defaults to terkini", "", "", "", "", PksListFilter{Status: "terkini"}, false},
		{"semua view", "semua", "", "", "", PksListFilter{Status: "semua"}, false},
		{"per-row status", "need_renewal", "", "", "", PksListFilter{Status: "need_renewal"}, false},
		{"diganti allowed", "diganti", "", "", "", PksListFilter{Status: "diganti"}, false},
		{"codes upper-cased and trimmed", " aktif ", " cit ", " v001 ", "  abc ",
			PksListFilter{Status: "aktif", PksType: "CIT", Vendor: "V001", Q: "abc"}, false},
		{"belum_diisi rejected", "belum_diisi", "", "", "", PksListFilter{}, true},
		{"unknown status rejected", "bogus", "", "", "", PksListFilter{}, true},
		{"q at 100 chars ok", "", "", "", strings.Repeat("a", 100),
			PksListFilter{Status: "terkini", Q: strings.Repeat("a", 100)}, false},
		{"q over 100 chars rejected", "", "", "", strings.Repeat("a", 101), PksListFilter{}, true},
		{"q counts runes not bytes", "", "", "", strings.Repeat("é", 100),
			PksListFilter{Status: "terkini", Q: strings.Repeat("é", 100)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePksListFilter(tt.status, tt.pksType, tt.vendor, tt.q)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestPksListStatusFilters(t *testing.T) {
	for _, k := range []string{"terkini", "semua", "aktif", "need_renewal", "belum_berlaku", "expired", "terminated", "diganti"} {
		if _, ok := PksListStatusFilters[k]; !ok {
			t.Errorf("filter %q missing", k)
		}
	}
	if len(PksListStatusFilters) != 8 {
		t.Errorf("len = %d, want 8", len(PksListStatusFilters))
	}
}

func TestPksStatusLabels(t *testing.T) {
	want := map[string]string{
		"aktif": "Aktif", "need_renewal": "Need Renewal", "belum_berlaku": "Belum Berlaku",
		"expired": "Expired", "terminated": "Terminated", "diganti": "Diganti",
	}
	if len(PksStatusLabels) != len(want) {
		t.Fatalf("len = %d, want %d", len(PksStatusLabels), len(want))
	}
	for k, v := range want {
		if PksStatusLabels[k] != v {
			t.Errorf("label[%q] = %q, want %q", k, PksStatusLabels[k], v)
		}
	}
}

func TestRenewalDaysBounds(t *testing.T) {
	if RenewalDaysMin != 30 || RenewalDaysMax != 3666 || RenewalDaysDefault != 270 {
		t.Errorf("bounds = %d/%d/%d", RenewalDaysMin, RenewalDaysMax, RenewalDaysDefault)
	}
}

func TestPksViewerRoles(t *testing.T) {
	want := []string{"ADMIN", "ADMIN_PARAM", "ATM-SPV"}
	if !slices.Equal(PksViewerRoles, want) {
		t.Errorf("PksViewerRoles = %v, want %v", PksViewerRoles, want)
	}
}

func TestPksExportHeader(t *testing.T) {
	want := []string{"Kode Vendor", "Nama Perusahaan", "Nama Direktur Utama", "Telepon Direktur Utama",
		"Email Direktur Utama", "Type PKS", "No. WOD", "Nama PKS", "Tanggal Efektif PKS", "Tanggal Expired PKS",
		"Masa Berlaku (hari)", "Reminder (hari)", "Status PKS"}
	if !slices.Equal(PksExportHeader, want) {
		t.Errorf("header = %v", PksExportHeader)
	}
	for _, h := range PksExportHeader {
		if strings.Contains(h, "bulan") {
			t.Errorf("header %q still mentions bulan", h)
		}
	}
}

func TestPksExportCells(t *testing.T) {
	s := func(v string) *string { return &v }
	i := func(v int32) *int32 { return &v }
	d := func(y int, m time.Month, day int) *time.Time {
		v := time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
		return &v
	}

	t.Run("full row", func(t *testing.T) {
		got := PksExportCells(PksExportRow{
			VendorCode: "V001", VendorName: "PT Satu", DirectorName: s("Budi"), DirectorPhone: s("0812"),
			DirectorEmail: s("b@x.id"), PksTypeCode: s("CIT"), WodNumber: s("WOD-1"), PksName: s("PKS CIT"),
			EffectiveDate: d(2026, 1, 1), ExpiryDate: d(2026, 12, 31), ValidityDays: i(364), RenewalDays: i(270),
			Status: s("need_renewal"),
		})
		if len(got) != len(PksExportHeader) {
			t.Fatalf("len = %d, want %d", len(got), len(PksExportHeader))
		}
		for idx, want := range []string{"V001", "PT Satu", "Budi", "0812", "b@x.id", "CIT", "WOD-1", "PKS CIT"} {
			if got[idx] != want {
				t.Errorf("cell %d = %q, want %q", idx, got[idx], want)
			}
		}
		if got[10] != "364" || got[11] != "270" || got[12] != "Need Renewal" {
			t.Errorf("tail = %v", got[10:])
		}
		if got[8] == "" || got[9] == "" {
			t.Errorf("date cells empty: %q %q", got[8], got[9])
		}
	})

	t.Run("nil pointers give empty cells", func(t *testing.T) {
		got := PksExportCells(PksExportRow{VendorCode: "V002", VendorName: "PT Dua"})
		if len(got) != len(PksExportHeader) {
			t.Fatalf("len = %d", len(got))
		}
		for idx := 2; idx < len(got); idx++ {
			if got[idx] != "" {
				t.Errorf("cell %d = %q, want empty", idx, got[idx])
			}
		}
	})

	t.Run("formula injection neutralised in text cells", func(t *testing.T) {
		got := PksExportCells(PksExportRow{VendorCode: "=1+1", VendorName: "+cmd", PksName: s("@x")})
		for _, idx := range []int{0, 1, 7} {
			if strings.HasPrefix(got[idx], "=") || strings.HasPrefix(got[idx], "+") || strings.HasPrefix(got[idx], "@") {
				t.Errorf("cell %d = %q not CSV-safe", idx, got[idx])
			}
		}
	})
}
