/**
 * Catalog of exportable data (export-menu-revamp). One place maps a dataset + applied filter to
 * the `ExportRequestParams` used by BOTH the preview and the export request (Req 3.5).
 * `roles` mirror backend-exq `KindRoles` / pkg/reports and deliberately have NO ADMIN bypass:
 * ADMIN_PARAM must not see report datasets. The backend still answers 403 if this goes stale.
 */

import type { DbRole } from "@/lib/auth/store";
import type { MasterExportParams, PriceLevel, StatusFilter } from "./types";

export type DatasetId =
  | "vendors"
  | "vendor-branches"
  | "vendor-vaults"
  | "vendor-pics"
  | "dsr-location-maps"
  | "package-prices"
  | "atms"
  | "atm-assignments";

export interface PreviewFilter {
  vendorId?: number;
  branchId?: number;
  status: StatusFilter;
  priceLevel?: PriceLevel;
}

export interface MasterDataset {
  group: "master";
  id: DatasetId;
  label: string;
  description: string;
  filters: { vendor: "none" | "optional" | "required"; branch: boolean; priceLevel: boolean };
  /** Right-aligned + tabular-nums (Req 2.9). Includes `moneyColumns`. */
  numericColumns: string[];
  /** Numeric columns that are also monospace. */
  moneyColumns: string[];
  roles: DbRole[];
}

export interface ReportDataset {
  group: "report";
  id: string;
  label: string;
  description: string;
  href: string;
  roles: DbRole[];
}

export type ExportDataset = MasterDataset | ReportDataset;

const MASTER_ROLES: DbRole[] = ["ADMIN", "ADMIN_PARAM"];
// pkg/reports DsrLateViewerRoles / ReplenishmentViewerRoles, copied explicitly.
const DSR_LATE_ROLES: DbRole[] = ["ATM-USER", "ATM-SPV", "BRANCH-ATM-SPV", "ADMIN"];
const REPLENISHMENT_ROLES: DbRole[] = [
  "ADMIN",
  "ATM-USER",
  "ATM-SPV",
  "BRANCH-ATM-USER",
  "BRANCH-ATM-SPV",
];

function master(
  d: Omit<MasterDataset, "group" | "roles" | "moneyColumns"> & { moneyColumns?: string[] },
): MasterDataset {
  return { group: "master", roles: MASTER_ROLES, moneyColumns: [], ...d };
}

// Column names are the CSV_Contract headers of pkg/csvexport/entities.go.
export const EXPORT_DATASETS: ExportDataset[] = [
  master({
    id: "vendors",
    label: "Vendor",
    description: "Daftar vendor beserta kontak dan alamat kantor pusat.",
    filters: { vendor: "none", branch: false, priceLevel: false },
    numericColumns: ["id"],
  }),
  master({
    id: "vendor-branches",
    label: "Cabang vendor",
    description: "Cabang vendor beserta wilayah dan kategori.",
    filters: { vendor: "optional", branch: false, priceLevel: false },
    numericColumns: ["id", "location_id"],
  }),
  master({
    id: "vendor-vaults",
    label: "Vault",
    description: "Vault vendor beserta kapasitas minimum dan maksimum.",
    filters: { vendor: "optional", branch: true, priceLevel: false },
    numericColumns: [
      "id",
      "min_capacity_amount",
      "max_capacity_amount",
      "latitude",
      "longitude",
      "location_id",
    ],
    moneyColumns: ["min_capacity_amount", "max_capacity_amount"],
  }),
  master({
    id: "vendor-pics",
    label: "PIC vendor",
    description: "PIC vendor beserta kontak dan peran notifikasi.",
    filters: { vendor: "optional", branch: true, priceLevel: false },
    numericColumns: ["id"],
  }),
  master({
    id: "dsr-location-maps",
    label: "Pemetaan lokasi DSR",
    description: "Pemetaan label lokasi DSR ke vault vendor. Vendor wajib dipilih.",
    filters: { vendor: "required", branch: true, priceLevel: false },
    numericColumns: ["id"],
  }),
  master({
    id: "package-prices",
    label: "Harga paket",
    description: "Harga paket per tingkat (vendor, cabang, ATM). Vendor wajib dipilih.",
    filters: { vendor: "required", branch: true, priceLevel: true },
    numericColumns: ["id", "tier_min", "tier_max", "base_price"],
    moneyColumns: ["base_price"],
  }),
  master({
    id: "atms",
    label: "ATM",
    description: "Data mesin ATM beserta kapasitas dan ambang saldo.",
    filters: { vendor: "none", branch: false, priceLevel: false },
    numericColumns: [
      "id",
      "location_id",
      "capacity_amount",
      "low_threshold_amount",
      "critical_threshold_amount",
    ],
    moneyColumns: ["capacity_amount", "low_threshold_amount", "critical_threshold_amount"],
  }),
  master({
    id: "atm-assignments",
    label: "Kelolaan ATM",
    description: "Penugasan ATM ke vendor dan cabang beserta paketnya.",
    filters: { vendor: "none", branch: false, priceLevel: false },
    numericColumns: ["id"],
  }),
  {
    group: "report",
    id: "dsr-late-report",
    label: "Laporan DSR telat",
    description: "Laporan harian dan ringkasan bulanan DSR yang terlambat.",
    href: "/dsr-late-report",
    roles: DSR_LATE_ROLES,
  },
  {
    group: "report",
    id: "replenishment",
    label: "Laporan replenishment",
    description: "Laporan selisih nominal dan jumlah kunjungan replenishment.",
    href: "/replenishment",
    roles: REPLENISHMENT_ROLES,
  },
];

/** Datasets the role may request. No ADMIN/ADMIN_PARAM bypass (design T6). */
export const datasetsForRole = (role: DbRole): ExportDataset[] =>
  EXPORT_DATASETS.filter((d) => d.roles.includes(role));

export const findMasterDataset = (id: string): MasterDataset | undefined =>
  EXPORT_DATASETS.find((d): d is MasterDataset => d.group === "master" && d.id === id);

/** Filter → export params per Req 3.1; `null` when a required filter is missing or branch has no vendor. */
export function toParams(id: DatasetId, f: PreviewFilter): MasterExportParams | null {
  const { vendorId, branchId, status } = f;
  if (branchId !== undefined && vendorId === undefined) return null;
  switch (id) {
    case "vendors":
    case "atms":
    case "atm-assignments":
      return { kind: "master_data_global", entity: id, status };
    case "vendor-branches":
      return vendorId === undefined
        ? { kind: "master_data_global", entity: "vendor-branches", status }
        : { kind: "vendor", vendorId, entity: "branches", status };
    case "vendor-vaults":
    case "vendor-pics": {
      const entity = id === "vendor-vaults" ? "vaults" : "pics";
      if (vendorId === undefined) return { kind: "master_data_global", entity: id, status };
      return branchId === undefined
        ? { kind: "vendor", vendorId, entity, status }
        : { kind: "cabang", vendorId, branchId, entity, status };
    }
    case "dsr-location-maps":
      if (vendorId === undefined) return null;
      return branchId === undefined
        ? { kind: "vendor", vendorId, entity: "dsr-location-maps", status }
        : { kind: "cabang", vendorId, branchId, entity: "dsr-location-maps", status };
    case "package-prices":
      if (vendorId === undefined) return null;
      return { kind: "harga_paket", vendorId, branchId, priceLevel: f.priceLevel, status };
  }
}

const STATUSES: readonly StatusFilter[] = ["active", "disabled", "all"];
const PRICE_LEVELS: readonly PriceLevel[] = ["all", "vendor", "branch", "atm"];

const positiveId = (v: unknown): number | undefined => {
  const n = typeof v === "string" && v.trim() !== "" ? Number(v) : v;
  return typeof n === "number" && Number.isInteger(n) && n > 0 ? n : undefined;
};

/** URL search → valid filter; invalid values fall back to defaults, never throw. */
export function parseFilterSearch(id: DatasetId, search: Record<string, unknown>): PreviewFilter {
  const ds = findMasterDataset(id);
  const status = STATUSES.find((s) => s === search.status) ?? "active";
  const vendorId = ds?.filters.vendor === "none" ? undefined : positiveId(search.vendorId);
  const branchId =
    ds?.filters.branch && vendorId !== undefined ? positiveId(search.branchId) : undefined;
  const filter: PreviewFilter = { status };
  if (vendorId !== undefined) filter.vendorId = vendorId;
  if (branchId !== undefined) filter.branchId = branchId;
  if (ds?.filters.priceLevel) {
    filter.priceLevel = PRICE_LEVELS.find((p) => p === search.priceLevel) ?? "all";
  }
  return filter;
}

/** Filter → URL search; empty fields are omitted. Inverse of `parseFilterSearch`. */
export function toSearch(f: PreviewFilter): Record<string, string | number> {
  const s: Record<string, string | number> = { status: f.status };
  if (f.vendorId !== undefined) s.vendorId = f.vendorId;
  if (f.branchId !== undefined) s.branchId = f.branchId;
  if (f.priceLevel !== undefined) s.priceLevel = f.priceLevel;
  return s;
}
