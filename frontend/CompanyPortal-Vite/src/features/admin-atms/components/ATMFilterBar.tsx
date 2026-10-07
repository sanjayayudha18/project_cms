import { FilterSelect } from "@/components/ui/FilterSelect";
import type { AdminATMsUrlParams } from "../useAdminATMsUrlState";

const STATUS_OPTIONS = [
  { value: "active", label: "Aktif" },
  { value: "disabled", label: "Nonaktif" },
  { value: "all", label: "Semua" },
];

// Distinct values observed on the live `atms` table (2026-09-16). Small,
// stable set -- a fixed dropdown is fine at current volumes, same call as
// the deferred search-index decision (Task 0). Revisit if brand/machine
// type/deployment options grow or start coming from data entry.
const BRAND_OPTIONS = ["Diebold", "Hitachi", "Hyosung", "NCR", "OKI", "Wincor"].map((v) => ({
  value: v,
  label: v,
}));
const MACHINE_TYPE_OPTIONS = ["ATM", "CDM", "CRM"].map((v) => ({ value: v, label: v }));
const DEPLOYMENT_TYPE_OPTIONS = [
  { value: "ONSITE", label: "Onsite" },
  { value: "OFFSITE", label: "Offsite" },
];
const PRIORITY_CLASS_OPTIONS = [
  { value: "VIP", label: "VIP" },
  { value: "Non VIP", label: "Non VIP" },
  { value: "Industri", label: "Industri" },
];

interface ATMFilterBarProps {
  params: AdminATMsUrlParams;
  searchInput: string;
  onSearchInputChange: (value: string) => void;
  onParamsChange: (partial: Partial<AdminATMsUrlParams>) => void;
}

/** Search + Brand/Tipe Mesin/Deployment/Prioritas/Status filters for the admin ATMs list (Req 8). */
export function ATMFilterBar({
  params,
  searchInput,
  onSearchInputChange,
  onParamsChange,
}: ATMFilterBarProps) {
  return (
    <div className="flex flex-wrap items-end gap-4">
      <div className="flex flex-col gap-1">
        <label
          htmlFor="admin-atms-search"
          className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
        >
          Cari
        </label>
        <input
          id="admin-atms-search"
          type="text"
          value={searchInput}
          onChange={(e) => onSearchInputChange(e.target.value.slice(0, 100))}
          maxLength={100}
          placeholder="Terminal ID"
          className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
        />
      </div>

      <FilterSelect
        label="Brand"
        options={BRAND_OPTIONS}
        value={params.brand || null}
        onChange={(value) => onParamsChange({ brand: value ?? "", page: 1 })}
        placeholder="Semua"
      />

      <FilterSelect
        label="Tipe Mesin"
        options={MACHINE_TYPE_OPTIONS}
        value={params.machine_type || null}
        onChange={(value) => onParamsChange({ machine_type: value ?? "", page: 1 })}
        placeholder="Semua"
      />

      <FilterSelect
        label="Deployment"
        options={DEPLOYMENT_TYPE_OPTIONS}
        value={params.deployment_type || null}
        onChange={(value) => onParamsChange({ deployment_type: value ?? "", page: 1 })}
        placeholder="Semua"
      />

      <FilterSelect
        label="Prioritas"
        options={PRIORITY_CLASS_OPTIONS}
        value={params.priority_class || null}
        onChange={(value) =>
          onParamsChange({
            priority_class: (value as AdminATMsUrlParams["priority_class"]) ?? "",
            page: 1,
          })
        }
        placeholder="Semua"
      />

      <FilterSelect
        label="Status"
        options={STATUS_OPTIONS}
        value={params.status}
        onChange={(value) =>
          onParamsChange({
            status: (value as AdminATMsUrlParams["status"] | null) ?? "active",
            page: 1,
          })
        }
        placeholder="Aktif"
      />
    </div>
  );
}
