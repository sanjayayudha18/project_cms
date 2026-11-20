package reports

import (
	"testing"
	"time"
)

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
