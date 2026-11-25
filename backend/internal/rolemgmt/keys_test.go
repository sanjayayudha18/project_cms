package rolemgmt

import "testing"

func TestIsInternalRole(t *testing.T) {
	tests := []struct {
		role string
		want bool
	}{
		{"ADMIN", true},
		{"ATM-USER", true},
		{"ACM-SPV", true},
		{"VENDOR-USER", false},
		{"VENDOR-ATM-USER", false},
		{"vendor-vault-user", false},
		{"EXT-CLIENT", false},
		{"", true},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			if got := IsInternalRole(tt.role); got != tt.want {
				t.Errorf("IsInternalRole(%q) = %v; want %v", tt.role, got, tt.want)
			}
		})
	}
}

func TestIsAlwaysOnAndLocked(t *testing.T) {
	for _, k := range AlwaysOnKeys {
		if !IsAlwaysOn(k) || IsLocked(k) {
			t.Errorf("key %q should be always-on only", k)
		}
	}
	for _, k := range LockedKeys {
		if !IsLocked(k) || IsAlwaysOn(k) {
			t.Errorf("key %q should be locked only", k)
		}
	}
	if IsAlwaysOn("settings.vendors") || IsLocked("settings.vendors") {
		t.Error("settings.vendors must be neither")
	}
}
