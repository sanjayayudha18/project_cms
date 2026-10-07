import { FilterSelect } from "@/components/ui/FilterSelect";
import type { AdminVendorsUrlParams } from "../useAdminVendorsUrlState";

const STATUS_OPTIONS = [
  { value: "active", label: "Aktif" },
  { value: "disabled", label: "Nonaktif" },
  { value: "all", label: "Semua" },
];

interface VendorFilterBarProps {
  params: AdminVendorsUrlParams;
  searchInput: string;
  onSearchInputChange: (value: string) => void;
  onParamsChange: (partial: Partial<AdminVendorsUrlParams>) => void;
}

/** Search + Status filter for the admin Vendors list (Req 6, 13.3-13.4). */
export function VendorFilterBar({
  params,
  searchInput,
  onSearchInputChange,
  onParamsChange,
}: VendorFilterBarProps) {
  return (
    <div className="flex flex-wrap items-end gap-4">
      <div className="flex flex-col gap-1">
        <label
          htmlFor="admin-vendors-search"
          className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
        >
          Cari
        </label>
        <input
          id="admin-vendors-search"
          type="text"
          value={searchInput}
          onChange={(e) => onSearchInputChange(e.target.value.slice(0, 100))}
          maxLength={100}
          placeholder="Kode atau nama vendor"
          className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
        />
      </div>

      <FilterSelect
        label="Status"
        options={STATUS_OPTIONS}
        value={params.status}
        onChange={(value) =>
          onParamsChange({
            status: (value as AdminVendorsUrlParams["status"] | null) ?? "active",
            page: 1,
          })
        }
        placeholder="Aktif"
      />
    </div>
  );
}
