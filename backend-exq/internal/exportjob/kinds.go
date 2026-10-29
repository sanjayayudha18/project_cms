package exportjob

import "slices"

// Audit actions of export jobs (Req 8.1, 8.2).
const (
	ActionExportRequested    = "export_requested"
	ActionMasterDataExported = "master_data_exported"
)

// KindSpec collects every rule that differs per Export_Kind (design "Registry per Kind", D23).
type KindSpec struct {
	Roles          []string                  // roles allowed to request and download
	DownloadAction string                    // audit action on download
	EntityRef      func(Job) (string, int64) // audit entity_type, entity_id
}

var masterDataAdmin = []string{"ADMIN", "ADMIN_PARAM"}

func idOrZero(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func vendorRef(j Job) (string, int64) { return "vendor", idOrZero(j.Params.VendorID) }
func branchRef(j Job) (string, int64) { return "vendor_branch", idOrZero(j.Params.BranchID) }
func globalRef(Job) (string, int64)   { return "master_data", 0 }

// priceRef: harga_paket is scoped to a cabang when one was given, else to the vendor (Req 8.3).
func priceRef(j Job) (string, int64) {
	if j.Params.BranchID != nil {
		return branchRef(j)
	}
	return vendorRef(j)
}

// KindSpecs is the per-kind registry. report-export-migration adds entries here only.
var KindSpecs = map[Kind]KindSpec{
	KindVendor:           {Roles: masterDataAdmin, DownloadAction: ActionMasterDataExported, EntityRef: vendorRef},
	KindCabang:           {Roles: masterDataAdmin, DownloadAction: ActionMasterDataExported, EntityRef: branchRef},
	KindHargaPaket:       {Roles: masterDataAdmin, DownloadAction: ActionMasterDataExported, EntityRef: priceRef},
	KindMasterDataGlobal: {Roles: masterDataAdmin, DownloadAction: ActionMasterDataExported, EntityRef: globalRef},
}

// KindRoles maps each kind to its allowed roles; used by the route guard and the service.
var KindRoles = rolesOf(KindSpecs)

func rolesOf(specs map[Kind]KindSpec) map[Kind][]string {
	out := make(map[Kind][]string, len(specs))
	for k, s := range specs {
		out[k] = slices.Clone(s.Roles)
	}
	return out
}

// allowed reports whether role may request/download kind. Unknown kind = false.
func allowed(kind Kind, role string) bool {
	return role != "" && slices.Contains(KindRoles[kind], role)
}
