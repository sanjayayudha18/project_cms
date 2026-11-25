// Package vendorrole identifies VendorPortal roles. Stdlib only.
//
// role-management-revamp: a vendor role is any role named "VENDOR-*" (roles.portal='vendor'
// is locked to that prefix by roles_portal_prefix_chk); its party scope lives in
// roles.party_scope and is read through ScopeReader.
package vendorrole

import (
	"context"
	"strings"
)

// Prefix marks a VendorPortal (Portal_Cabang) role.
const Prefix = "VENDOR-"

// ScopeReader reads a role's party scope (roles.party_scope). Unknown role or NULL scope
// → nil, nil; callers must treat an empty result as "no access" (fail closed).
type ScopeReader interface {
	PartyScope(ctx context.Context, role string) ([]string, error)
}

// Seeded vendor roles (migration 047 party_scope). Tests and fixtures only:
// access decisions use IsVendor and ScopeReader, never these names.
const (
	User      = "VENDOR-USER"       // dua peran: replenish + vault
	ATMUser   = "VENDOR-ATM-USER"   // replenish saja
	VaultUser = "VENDOR-VAULT-USER" // vault saja
)

// IsVendor reports whether role is a VendorPortal role: prefix "VENDOR-", case-insensitive,
// surrounding spaces ignored. No DB lookup.
func IsVendor(role string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(role)), Prefix)
}
