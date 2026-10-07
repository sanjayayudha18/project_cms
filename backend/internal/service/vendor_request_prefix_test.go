package service

import "testing"

// TestFallbackVendorPrefix covers Task 3.3: the deterministic fallback used
// when vendors.request_prefix is NULL (Req 4.2, 4.3, design.md Q1) must
// always produce exactly 3 uppercase characters, never empty or truncated.
// Note: this fallback only ever runs for a vendor with no seeded prefix.
// ABACUS's actual request_prefix is seeded to 'ABA' by migration 036
// (confirmed with the user 2026-09-14), which happens to equal the
// fallback's own first-3-chars rule for that code -- this test exercises
// the fallback function directly, independent of the seed.
func TestFallbackVendorPrefix(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{"seeded 3-char code unchanged", "TAG", "TAG"},
		{"seeded code longer than 3 truncates to first 3", "ADVANTAGE", "ADV"},
		{"seeded code longer than 3, different vendor", "BIJAK", "BIJ"},
		{"unseeded code truncates to first 3 (matches seeded 'ABA')", "ABACUS", "ABA"},
		{"already 3 chars", "ROH", "ROH"},
		{"2-char code right-pads with X", "AB", "ABX"},
		{"1-char code right-pads with X", "A", "AXX"},
		{"lowercase input is uppercased", "abc", "ABC"},
		{"non-alnum characters are stripped before padding", "A-1", "A1X"},
		{"empty code pads to all X", "", "XXX"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fallbackVendorPrefix(tt.code)
			if got != tt.want {
				t.Errorf("fallbackVendorPrefix(%q) = %q, want %q", tt.code, got, tt.want)
			}
			if len(got) != 3 {
				t.Errorf("fallbackVendorPrefix(%q) = %q, want exactly 3 chars", tt.code, got)
			}
		})
	}
}
