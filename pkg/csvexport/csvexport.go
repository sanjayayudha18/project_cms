// Package csvexport is the CSV contract of master-data export/import: entity
// names, stable headers (+ legacy headers the importer still accepts), cell
// shaping per entity, formula-injection guard and the streaming writer. Pure
// stdlib, no DB. Moved as-is from backend/internal/service/masterdata_export.go.
package csvexport

// Importable master-data entities (the {entity} path segment).
const (
	Vendors        = "vendors"
	VendorBranches = "vendor-branches"
	VendorVaults   = "vendor-vaults"
	VendorPICs     = "vendor-pics"
	ATMs           = "atms"
	ATMAssignments = "atm-assignments"
)

// Export-only entities (never accepted by the importer).
const (
	DSRLocationMaps = "dsr-location-maps"
	PackagePrices   = "package-prices"
)

// BOM lets Excel open the file as UTF-8. The importer strips it.
const BOM = "\xEF\xBB\xBF"

// Importer-only cell values.
const (
	// KeepCurrentCell is the legacyFill for vendor-branch files without the
	// region_code/category columns: never a valid code or category.
	KeepCurrentCell        = "<keep>"
	AssignmentSourceBranch = "branch"
	// PICRolesNone is the roles cell of a PIC with an empty role set.
	PICRolesNone = "none"
)
