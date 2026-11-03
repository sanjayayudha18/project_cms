package exportjob

import (
	"slices"

	"github.com/cimb-niaga/cms/pkg/reports"
)

// Audit actions of export jobs (Req 8.1, 8.2). report_exported is the action the legacy
// backend/ report CSV wrote, kept so old and new rows are found together (report-export-migration Req 5).
const (
	ActionExportRequested    = "export_requested"
	ActionMasterDataExported = "master_data_exported"
	ActionReportExported     = "report_exported"
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

// fixed is the audit entity of a report kind: the legacy entity_type, id 0 (report-export-migration Req 5.3).
func fixed(entityType string) func(Job) (string, int64) {
	return func(Job) (string, int64) { return entityType, 0 }
}

// KindSpecs is the per-kind registry. Report kinds take the roles of the report screens (pkg/reports);
// master-data kinds stay ADMIN/ADMIN_PARAM.
var KindSpecs = map[Kind]KindSpec{
	KindVendor:           {Roles: masterDataAdmin, DownloadAction: ActionMasterDataExported, EntityRef: vendorRef},
	KindCabang:           {Roles: masterDataAdmin, DownloadAction: ActionMasterDataExported, EntityRef: branchRef},
	KindHargaPaket:       {Roles: masterDataAdmin, DownloadAction: ActionMasterDataExported, EntityRef: priceRef},
	KindMasterDataGlobal: {Roles: masterDataAdmin, DownloadAction: ActionMasterDataExported, EntityRef: globalRef},

	KindDsrLateDaily:   {Roles: reports.DsrLateViewerRoles, DownloadAction: ActionReportExported, EntityRef: fixed("dsr_late_report")},
	KindDsrLateSummary: {Roles: reports.DsrLateViewerRoles, DownloadAction: ActionReportExported, EntityRef: fixed("dsr_late_report")},
	KindReplenishmentAmountMismatch: {Roles: reports.ReplenishmentViewerRoles, DownloadAction: ActionReportExported,
		EntityRef: fixed("replenishment_report")},
	KindReplenishmentTrips: {Roles: reports.ReplenishmentViewerRoles, DownloadAction: ActionReportExported,
		EntityRef: fixed("replenishment_report")},
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
