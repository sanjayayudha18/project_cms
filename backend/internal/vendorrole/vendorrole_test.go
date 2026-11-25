package vendorrole

import "testing"

func TestIsVendor(t *testing.T) {
	tests := []struct {
		name string
		role string
		want bool
	}{
		{"vendor-user", User, true},
		{"vendor-atm-user", ATMUser, true},
		{"vendor-vault-user", VaultUser, true},
		{"internal admin", "ADMIN", false},
		{"internal branch atm", "BRANCH-ATM-SPV", false},
		{"empty", "", false},
		{"new vendor role by prefix", "VENDOR-AUDIT", true},
		{"prefix is case-insensitive", "vendor-user", true},
		{"surrounding spaces trimmed", "  VENDOR-USER ", true},
		{"prefix alone without dash", "VENDOR", false},
		{"prefix not at start", "X-VENDOR-USER", false},
		{"ext client", "EXT-CLIENT", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsVendor(tt.role); got != tt.want {
				t.Errorf("IsVendor(%q) = %v, want %v", tt.role, got, tt.want)
			}
		})
	}
}
