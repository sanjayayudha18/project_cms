package auth

import "testing"

// role-management-revamp 4.4 (Req 2.5, 2.6): vendorFields over role x auth_source x vendor_id x vendor_branch_id.
// A vendor role is recognised by the "VENDOR-" prefix alone, so a role created later behaves like the seeded ones.
func TestVendorFields_Matrix(t *testing.T) {
	v, b := ptrInt64(5), ptrInt64(7)
	tests := []struct {
		name      string
		role      string
		source    string
		vendorID  *int64
		branchID  *int64
		wantField string // "" = accepted
		wantRef   bool
	}{
		// vendor roles (seeded and new, any case) need local + vendor_id
		{"seeded vendor local vendor", "VENDOR-USER", "local", v, nil, "", false},
		{"seeded vendor local vendor branch", "VENDOR-VAULT-USER", "local", v, b, "", false},
		{"new vendor role local vendor", "VENDOR-AUDIT", "local", v, nil, "", false},
		{"lowercase vendor role local vendor", "vendor-audit", "local", v, nil, "", false},
		{"new vendor role ldap", "VENDOR-AUDIT", "ldap", v, nil, "auth_source", false},
		{"new vendor role local no vendor", "VENDOR-AUDIT", "local", nil, nil, "vendor_id", false},
		// non-vendor roles must not carry vendor data (Req 2.5)
		{"main role local no vendor", "OPERATOR", "local", nil, nil, "", false},
		{"main role ldap no vendor", "OPERATOR", "ldap", nil, nil, "", false},
		{"main role local with vendor", "OPERATOR", "local", v, nil, "vendor_id", true},
		{"main role local with vendor+branch", "OPERATOR", "local", v, b, "vendor_branch_id", true},
		{"main role ldap with branch", "OPERATOR", "ldap", nil, b, "vendor_branch_id", true},
		{"ldap vendor_id left to localCredentials", "OPERATOR", "ldap", v, nil, "", false},
		{"prefix without dash is not vendor", "VENDOR", "local", v, nil, "vendor_id", true},
		{"EXT-CLIENT with vendor", "EXT-CLIENT", "local", v, nil, "vendor_id", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := vendorFields(tt.role, tt.source, tt.vendorID, tt.branchID)
			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			assertFieldError(t, err, tt.wantField, tt.wantRef)
		})
	}
}
