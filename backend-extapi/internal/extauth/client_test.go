package extauth

import (
	"net/netip"
	"reflect"
	"testing"
)

func TestValidScope(t *testing.T) {
	for s, want := range map[string]bool{
		ScopeUsersRead: true, ScopeUsersWrite: true,
		"": false, "users:admin": false, "USERS:READ": false, " users:read": false,
	} {
		if got := ValidScope(s); got != want {
			t.Errorf("ValidScope(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestParseScopes(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []string
		wantErr bool
	}{
		{"empty", "", nil, false},
		{"blank", "  , ", nil, false},
		{"space", "users:read users:write", []string{"users:read", "users:write"}, false},
		{"comma", "users:write,users:read", []string{"users:write", "users:read"}, false},
		{"mixed + dedup", " users:read, users:read  users:write ", []string{"users:read", "users:write"}, false},
		{"unknown", "users:read users:admin", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseScopes(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClientAllowsIP(t *testing.T) {
	c := Client{AllowedCIDRs: []netip.Prefix{
		netip.MustParsePrefix("10.20.0.0/16"),
		netip.MustParsePrefix("2001:db8::/32"),
	}}
	tests := []struct {
		ip   string
		want bool
	}{
		{"10.20.3.4", true},
		{"10.21.0.1", false},
		{"2001:db8::1", true},
		{"::ffff:10.20.3.4", true}, // IPv4-mapped form from RemoteAddr
		{"192.0.2.1", false},
	}
	for _, tt := range tests {
		if got := c.AllowsIP(netip.MustParseAddr(tt.ip)); got != tt.want {
			t.Errorf("AllowsIP(%s) = %v, want %v", tt.ip, got, tt.want)
		}
	}
	if (Client{}).AllowsIP(netip.MustParseAddr("10.20.3.4")) {
		t.Error("empty CIDR list must reject every IP (Req 1.7)")
	}
	if c.AllowsIP(netip.Addr{}) {
		t.Error("invalid addr must be rejected")
	}
}

func TestClientCanAssign(t *testing.T) {
	// ADMIN in the allowlist only happens if the DB CHECK is bypassed; still refused.
	c := Client{AssignableRoles: []string{"ATM-USER", "ACM-SPV", "ADMIN"}}
	tests := map[string]bool{
		"ATM-USER": true, "ACM-SPV": true,
		"ADMIN": false, "APPACCESS": false, "ADMIN_PARAM": false, "EXT-CLIENT": false, " admin ": false,
		"ATM-SPV": false, "atm-user": false, "": false,
	}
	for role, want := range tests {
		if got := c.CanAssign(role); got != want {
			t.Errorf("CanAssign(%q) = %v, want %v", role, got, want)
		}
	}
}

func TestClientHasScope(t *testing.T) {
	c := Client{Scopes: []string{ScopeUsersRead}}
	if !c.HasScope(ScopeUsersRead) || c.HasScope(ScopeUsersWrite) || c.HasScope("") {
		t.Error("HasScope mismatch")
	}
}
