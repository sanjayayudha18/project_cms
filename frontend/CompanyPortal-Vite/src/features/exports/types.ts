import type { components, paths } from "@/lib/api/exq-schema";

export type ExportJob = components["schemas"]["ExportJob"];
export type ExportStatus = components["schemas"]["ExportStatus"];
export type ExportKind = components["schemas"]["ExportKind"];
export type StatusFilter = components["schemas"]["StatusFilter"];
export type PriceLevel = components["schemas"]["PriceLevel"];

export type ReportParams = components["schemas"]["ReportParams"];
/** URL segment of `POST /exports/reports/{report}`. */
export type ReportKey =
  | "dsr-late-daily"
  | "dsr-late-summary"
  | "replenishment-amount-mismatch"
  | "replenishment-trips";

export type MasterDataEntity = components["parameters"]["MasterDataEntity"];
export type VendorEntity = "vendor" | "branches" | "vaults" | "pics" | "dsr-location-maps";
export type BranchEntity = "vaults" | "pics" | "dsr-location-maps";

export type ListExportJobsResponse =
  paths["/exports/jobs"]["get"]["responses"][200]["content"]["application/json"];

/** One export request; `kind` mirrors the job kind the backend will create. */
export type ExportRequestParams =
  | { kind: "vendor"; vendorId: number; entity: VendorEntity; status?: StatusFilter }
  | {
      kind: "cabang";
      vendorId: number;
      branchId: number;
      entity: BranchEntity;
      status?: StatusFilter;
    }
  | {
      kind: "harga_paket";
      vendorId: number;
      branchId?: number;
      priceLevel?: PriceLevel;
      status?: StatusFilter;
    }
  | { kind: "master_data_global"; entity: MasterDataEntity; status?: StatusFilter }
  | { kind: "report"; report: ReportKey; params: ReportParams };

export interface RequestExportResult {
  job: ExportJob;
  /** true = 202 (new job); false = 200 (identical active job reused). */
  created: boolean;
}
