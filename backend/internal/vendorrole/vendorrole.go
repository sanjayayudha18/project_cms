// Package vendorrole is the single source for mapping VendorPortal roles to party roles
// (vendor-party-scope, design "Pemetaan role → peran pihak"). No dependencies.
package vendorrole

const (
	User      = "VENDOR-USER"       // dua peran: replenish + vault
	ATMUser   = "VENDOR-ATM-USER"   // replenish saja
	VaultUser = "VENDOR-VAULT-USER" // vault saja
)

// Party role names as stored in vendor_request_vendor_parties.role / vendor_pics.roles.
const (
	partyReplenish = "replenish"
	partyVault     = "vault"
)

// All is every VendorPortal role (route guards, admin users validation).
var All = []string{User, ATMUser, VaultUser}

// IsVendor reports whether role is a VendorPortal role.
func IsVendor(role string) bool {
	switch role {
	case User, ATMUser, VaultUser:
		return true
	}
	return false
}

// PartyRoles: party roles a vendor role may see and decide. nil = not a vendor role.
// Returns a fresh slice on every call, so callers cannot change the mapping.
func PartyRoles(role string) []string {
	switch role {
	case User:
		return []string{partyReplenish, partyVault}
	case ATMUser:
		return []string{partyReplenish}
	case VaultUser:
		return []string{partyVault}
	}
	return nil
}

// ForParty: vendor roles that receive notices for a party role (D6a).
// nil for cashcount or unknown input: cashcount is never a PicRole for a party notice (Req 7.3).
// Returns a fresh slice on every call.
func ForParty(partyRole string) []string {
	switch partyRole {
	case partyReplenish:
		return []string{User, ATMUser}
	case partyVault:
		return []string{User, VaultUser}
	}
	return nil
}
