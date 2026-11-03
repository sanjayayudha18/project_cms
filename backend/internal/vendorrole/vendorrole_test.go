package vendorrole

import (
	"reflect"
	"testing"
)

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
		{"case sensitive", "vendor-user", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsVendor(tt.role); got != tt.want {
				t.Errorf("IsVendor(%q) = %v, want %v", tt.role, got, tt.want)
			}
		})
	}
}

func TestPartyRoles(t *testing.T) {
	tests := []struct {
		name string
		role string
		want []string
	}{
		{"vendor-user gets both parties", User, []string{"replenish", "vault"}},
		{"vendor-atm-user replenish only", ATMUser, []string{"replenish"}},
		{"vendor-vault-user vault only", VaultUser, []string{"vault"}},
		{"internal role is nil", "ADMIN", nil},
		{"empty role is nil", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PartyRoles(tt.role); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PartyRoles(%q) = %v, want %v", tt.role, got, tt.want)
			}
		})
	}
}

func TestForParty(t *testing.T) {
	tests := []struct {
		name      string
		partyRole string
		want      []string
	}{
		{"replenish notifies vendor-user and atm-user", "replenish", []string{User, ATMUser}},
		{"vault notifies vendor-user and vault-user", "vault", []string{User, VaultUser}},
		{"cashcount is never a party notice", "cashcount", nil},
		{"empty party role is nil", "", nil},
		{"unknown party role is nil", "unknown", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ForParty(tt.partyRole); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ForParty(%q) = %v, want %v", tt.partyRole, got, tt.want)
			}
		})
	}
}

func TestReturnedSlicesDoNotAliasMapping(t *testing.T) {
	// Mutating a returned slice must not change what the next call returns.
	pr := PartyRoles(User)
	pr[0] = "tampered"
	if got := PartyRoles(User); got[0] != "replenish" {
		t.Errorf("PartyRoles after mutation = %v, want replenish first", got)
	}

	fp := ForParty("vault")
	fp[0] = "tampered"
	if got := ForParty("vault"); got[0] != User {
		t.Errorf("ForParty after mutation = %v, want %s first", got, User)
	}
}
