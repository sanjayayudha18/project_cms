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
}

func (e Entity) clone() Entity {
	e.Header = slices.Clone(e.Header)
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
		Header: []string{"id", "code", "name", "legal_name", "npwp", "contact_email", "contact_phone", "hq_address", "is_active"},
	},
	VendorBranches: {
		Name:   VendorBranches,
		Header: []string{"id", "vendor_code", "branch_code", "branch_name", "region", "region_code", "category", "location_id", "is_active"},
		// Older files have no category (and, before migration 026, no
		// region_code): keep the branch's current value instead of clearing it.
		LegacyHeaders: [][]string{
			{"id", "vendor_code", "branch_code", "branch_name", "region", "region_code", "location_id", "is_active"},
			{"id", "vendor_code", "branch_code", "branch_name", "region", "location_id", "is_active"},
		},
		LegacyFill: map[string]string{"region_code": KeepCurrentCell, "category": KeepCurrentCell},
	},
	VendorVaults: {
		Name:   VendorVaults,
		Header: []string{"id", "vendor_code", "branch_code", "vault_code", "category", "currency_code", "min_capacity_amount", "max_capacity_amount", "latitude", "longitude", "operating_hours", "location_id", "is_active"},
	},
	VendorPICs: {
		Name:   VendorPICs,
		Header: []string{"id", "vendor_code", "branch_code", "name", "position", "phone", "email", "is_notification_recipient", "is_active", "roles"},
		// Older files have no roles column: blank (create = all roles, update = unchanged).
		LegacyHeaders: [][]string{{"id", "vendor_code", "branch_code", "name", "position", "phone", "email", "is_notification_recipient", "is_active"}},
		LegacyFill:    map[string]string{"roles": ""},
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
