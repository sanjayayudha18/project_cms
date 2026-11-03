package service

import (
	"errors"
	"strings"
	"testing"
)

func TestParseHolidayCSV(t *testing.T) {
	tests := []struct {
		name     string
		csv      string
		wantRows int
		wantErr  bool
	}{
		{"valid with BOM and CRLF", "\xef\xbb\xbfholiday_date,name\r\n2026-12-25,Hari Raya Natal\r\n2027-01-01,Tahun Baru\r\n", 2, false},
		{"name trimmed, blank lines ignored", "holiday_date,name\n2026-12-25,  Natal  \n\n", 1, false},
		{"missing header", "2026-12-25,Natal\n", 0, true},
		{"bad date", "holiday_date,name\n25-12-2026,Natal\n", 0, true},
		{"empty name", "holiday_date,name\n2026-12-25,  \n", 0, true},
		{"name too long", "holiday_date,name\n2026-12-25," + strings.Repeat("x", 101) + "\n", 0, true},
		{"duplicate date in file", "holiday_date,name\n2026-12-25,A\n2026-12-25,B\n", 0, true},
		{"no rows", "holiday_date,name\n", 0, true},
		{"wrong column count", "holiday_date,name\n2026-12-25\n", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := parseHolidayCSV(strings.NewReader(tt.csv))
			if tt.wantErr {
				var ve *ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("want ValidationError, got %v", err)
				}
				return
			}
			if err != nil || len(rows) != tt.wantRows {
				t.Fatalf("rows = %v, err = %v; want %d rows", rows, err, tt.wantRows)
			}
		})
	}
	rows, _ := parseHolidayCSV(strings.NewReader("holiday_date,name\n2026-12-25,  Natal  \n"))
	if rows[0].name != "Natal" || rows[0].date.Format("2006-01-02") != "2026-12-25" {
		t.Fatalf("row = %+v", rows[0])
	}
}

func TestValidateHolidayName(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
		ok   bool
	}{{"  Natal ", "Natal", true}, {"", "", false}, {"   ", "", false}, {strings.Repeat("é", 100), strings.Repeat("é", 100), true}, {strings.Repeat("é", 101), "", false}} {
		got, err := validateHolidayName(tc.in)
		if (err == nil) != tc.ok || (tc.ok && got != tc.want) {
			t.Errorf("validateHolidayName(%q) = %q, %v", tc.in, got, err)
		}
	}
}
