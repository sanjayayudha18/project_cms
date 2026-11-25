package rolemgmt

import "strings"

// Always-on and locked screen keys (spec permission-catalog-coverage D7/D8).
// The same lists are mirrored in the CompanyPortal (lib/auth/permissionKeys.ts).

// AlwaysOnKeys are open to every internal role and can never be revoked.
var AlwaysOnKeys = []string{"dashboard", "approvals"}

// LockedKeys stay guarded by the static APPACCESS/ADMIN roles; the catalog
// cannot grant or revoke them.
var LockedKeys = []string{"settings.roles", "settings.roles.create", "settings.roles.edit", "settings.users"}

// VendorAlwaysOnKeys are VendorPortal keys every vendor role always holds
// (spec role-management-revamp Req 3.2). Not in the catalog and not guarded:
// MyPermissions adds them for vendor roles.
var VendorAlwaysOnKeys = []string{"vendor-notifications"}

// IsInternalRole reports whether role is a CompanyPortal (internal) role,
// i.e. anything except VENDOR-* and EXT-CLIENT.
func IsInternalRole(role string) bool {
	r := strings.ToUpper(strings.TrimSpace(role))
	return !strings.HasPrefix(r, "VENDOR-") && r != "EXT-CLIENT"
}

// IsAlwaysOn reports whether key is an always-on key.
func IsAlwaysOn(key string) bool { return contains(AlwaysOnKeys, key) }

// IsLocked reports whether key is a locked key.
func IsLocked(key string) bool { return contains(LockedKeys, key) }

func contains(list []string, key string) bool {
	for _, k := range list {
		if k == key {
			return true
		}
	}
	return false
}
