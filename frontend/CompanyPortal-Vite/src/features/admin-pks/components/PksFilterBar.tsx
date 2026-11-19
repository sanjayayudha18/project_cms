import { FilterSelect } from "@/components/ui/FilterSelect";
import {
  type AdminPksUrlParams,
  PKS_LIST_STATUSES,
  type PksListStatusValue,
} from "../useAdminPksUrlState";

const STATUS_LABEL: Record<PksListStatusValue, string> = {
  terkini: "Terkini",
  semua: "Semua",
  aktif: "Aktif",
  need_renewal: "Need Renewal",
  belum_berlaku: "Belum Berlaku",
  expired: "Expired",
  terminated: "Terminated",
  diganti: "Diganti",
};

const STATUS_OPTIONS = PKS_LIST_STATUSES.map((value) => ({ value, label: STATUS_LABEL[value] }));

export interface PksFilterBarProps {
  params: AdminPksUrlParams;
  searchInput: string;
  onSearchInputChange: (value: string) => void;
  onParamsChange: (partial: Partial<AdminPksUrlParams>) => void;
  typeOptions: { value: string; label: string }[];
  vendorOptions: { value: string; label: string }[];
}

/** Status (default Terkini), Type, Vendor and a judul/WOD search; all state lives in the URL. */
export function PksFilterBar({
  params,
  searchInput,
  onSearchInputChange,
  onParamsChange,
  typeOptions,
  vendorOptions,
}: PksFilterBarProps) {
  return (
    <div className="flex flex-wrap items-end gap-4">
      <div className="flex flex-col gap-1">
        <label
          htmlFor="admin-pks-search"
          className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
        >
          Cari
        </label>
        <input
          id="admin-pks-search"
          type="text"
          value={searchInput}
          onChange={(e) => onSearchInputChange(e.target.value.slice(0, 100))}
          maxLength={100}
          placeholder="Judul PKS atau No. WOD"
          className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
        />
      </div>

      <FilterSelect
        label="Status"
        options={STATUS_OPTIONS}
        value={params.status}
        onChange={(v) => onParamsChange({ status: (v as PksListStatusValue | null) ?? "terkini" })}
        placeholder="Terkini"
      />
      <FilterSelect
        label="Type PKS"
        options={typeOptions}
        value={params.pks_type ?? null}
        onChange={(v) => onParamsChange({ pks_type: v ?? undefined })}
      />
      <FilterSelect
        label="Vendor"
        options={vendorOptions}
        value={params.vendor ?? null}
        onChange={(v) => onParamsChange({ vendor: v ?? undefined })}
      />
    </div>
  );
}
