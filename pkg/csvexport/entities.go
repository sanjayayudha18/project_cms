package csvexport

import (
	"maps"
	"slices"
)

// Entity is one entity's contract: the stable header, older headers the importer
// still accepts, and the value filled for each column missing in an old header.
type Entity struct {
	Name          string
	Header        []string
	LegacyHeaders [][]string
	LegacyFill    map[string]string
	// ReadOnlyColumns follow Header in export files only. The importer accepts
	// Header+ReadOnlyColumns and drops these cells before validation (vendor-export-pks).
	ReadOnlyColumns []string
}

// ExportHeader is Header followed by ReadOnlyColumns (a fresh slice).
func (e Entity) ExportHeader() []string {
	return slices.Concat(e.Header, e.ReadOnlyColumns)
}

// VendorReadOnlyColumns are the nine director + active-PKS columns (used by
// vendor-pks export and legacy vendor export headers). Returns a fresh slice
// so callers cannot mutate the contract.
func VendorReadOnlyColumns() []string { return slices.Clone(vendorReadOnlyColumns) }

var vendorReadOnlyColumns = []string{
	"director_name", "director_phone", "director_email",
	"pks_name", "pks_effective_date", "pks_expiry_date",
	"pks_validity_months", "pks_remaining_days", "pks_status",
}

func (e Entity) clone() Entity {
	e.Header = slices.Clone(e.Header)
	e.ReadOnlyColumns = slices.Clone(e.ReadOnlyColumns)
	if e.LegacyHeaders != nil {
		lh := make([][]string, len(e.LegacyHeaders))
		for i, h := range e.LegacyHeaders {
			lh[i] = slices.Clone(h)
		}
		e.LegacyHeaders = lh
	}
	e.LegacyFill = maps.Clone(e.LegacyFill)
	return e
}

var importable = map[string]Entity{
	Vendors: {
		Name:   Vendors,
		Header: []string{"name", "legal_name", "npwp", "contact_email", "contact_phone", "hq_address", "is_active"},
		// Older files still carry id and code (and, from vendor-export-pks, the nine read-only PKS columns).
		LegacyHeaders: [][]string{
			{"id", "code", "name", "legal_name", "npwp", "contact_email", "contact_phone", "hq_address", "is_active"},
			slices.Concat([]string{"id", "code", "name", "legal_name", "npwp", "contact_email", "contact_phone", "hq_address", "is_active"}, vendorReadOnlyColumns),
		},
	},
	VendorBranches: {
		Name:   VendorBranches,
		Header: []string{"id", "vendor_code", "branch_code", "branch_name", "region", "region_code", "category", "location_id", "is_active"},
		// Older files may still carry cis_limit_amount (migration 038..039, now moved to vendor_vaults),
		// or lack category / region_code (before migration 026): keep the branch's current value instead of clearing it.
		LegacyHeaders: [][]string{
			{"id", "vendor_code", "branch_code", "branch_name", "region", "region_code", "category", "location_id", "cis_limit_amount", "is_active"},
			{"id", "vendor_code", "branch_code", "branch_name", "region", "region_code", "location_id", "is_active"},
			{"id", "vendor_code", "branch_code", "branch_name", "region", "location_id", "is_active"},
		},
		LegacyFill: map[string]string{"region_code": KeepCurrentCell, "category": KeepCurrentCell},
	},
	VendorVaults: {
		Name: VendorVaults,
		Header: []string{
			"no", "branch_name", "vault_name", "category", "operating_hours", "currency_code",
			"region", "alamat", "min_balance", "max_balance", "cis_limit_amount",
			"escrow_cash", "escrow_atm", "is_active",
		},
		// Pre-040 files carry id/vendor_code/branch_code/vault_code/min_capacity_amount/max_capacity_amount
		// (with or without escrow_account from migration 038); missing 040 columns keep their current values.
		LegacyHeaders: [][]string{
			{"id", "vendor_code", "branch_code", "vault_code", "category", "currency_code", "min_capacity_amount", "max_capacity_amount", "latitude", "longitude", "operating_hours", "location_id", "escrow_account", "is_active"},
			{"id", "vendor_code", "branch_code", "vault_code", "category", "currency_code", "min_capacity_amount", "max_capacity_amount", "latitude", "longitude", "operating_hours", "location_id", "is_active"},
		},
		LegacyFill: map[string]string{
			"alamat":           KeepCurrentCell,
			"cis_limit_amount": KeepCurrentCell,
			"escrow_cash":      KeepCurrentCell,
			"escrow_atm":       KeepCurrentCell,
		},
	},
	VendorPICs: {
		Name:   VendorPICs,
		Header: []string{"id", "vendor_code", "branch_code", "name", "position", "phone", "email", "is_notification_recipient", "is_active", "roles", "is_director"},
		// Older files have no roles column: blank (create = all roles, update = unchanged).
		LegacyHeaders: [][]string{{"id", "vendor_code", "branch_code", "name", "position", "phone", "email", "is_notification_recipient", "is_active", "roles"},
			{"id", "vendor_code", "branch_code", "name", "position", "phone", "email", "is_notification_recipient", "is_active"},
		},
		LegacyFill: map[string]string{"roles": "", "is_director": KeepCurrentCell},
	},
	ATMs: {
		Name:   ATMs,
		Header: []string{"id", "terminal_id", "location_id", "machine_type", "brand", "model", "operation_hours", "deployment_type", "capacity_amount", "low_threshold_amount", "critical_threshold_amount", "blacklisted", "escrow_account", "priority_class", "is_active"},
	},
	ATMAssignments: {
		Name:          ATMAssignments,
		Header:        []string{"id", "terminal_id", "vendor_code", "branch_code", "package_source", "package_code", "effective_start_date", "effective_end_date", "is_active"},
		LegacyHeaders: [][]string{{"id", "terminal_id", "vendor_code", "branch_code", "package_code", "effective_start_date", "effective_end_date", "is_active"}},
		LegacyFill:    map[string]string{"package_source": AssignmentSourceBranch},
	},
}

var exportOnly = map[string]Entity{
	DSRLocationMaps: {
		Name:   DSRLocationMaps,
		Header: []string{"id", "vendor_code", "branch_code", "vault_code", "dsr_location", "is_active"},
	},
	PackagePrices: {
		Name: PackagePrices,
		Header: []string{"id", "vendor_code", "price_level", "branch_code", "terminal_id", "package", "package_code", "machine_group",
			"price_class", "tier_min", "tier_max", "base_price", "currency", "sla_note", "effective_start_date", "effective_end_date"},
	},
	VendorPks: {
		Name:   VendorPks,
		Header: slices.Concat([]string{"vendor_code", "vendor_name"}, vendorReadOnlyColumns),
	},
}

// Lookup returns a copy of an importable entity's contract (callers cannot
// mutate the original). Export-only entities are not found here, so the
// importer keeps rejecting them.
func Lookup(name string) (Entity, bool) {
	e, ok := importable[name]
	return e.clone(), ok
}

// LookupExportOnly is Lookup for the export-only entities.
func LookupExportOnly(name string) (Entity, bool) {
	e, ok := exportOnly[name]
	return e.clone(), ok
}

// ImportableNames lists the importable entities, sorted.
func ImportableNames() []string { return slices.Sorted(maps.Keys(importable)) }

// ExportOnlyNames lists the export-only entities, sorted.
func ExportOnlyNames() []string { return slices.Sorted(maps.Keys(exportOnly)) }
