import { FilterSelect } from "@/components/ui/FilterSelect";
import { useEffect, useMemo, useState } from "react";
import type { PksType } from "../pksApi";
import type { PksStatus, VendorKind } from "../types";
import type { AdminVendorsUrlParams } from "../useAdminVendorsUrlState";

const KIND_OPTIONS: { value: VendorKind; label: string }[] = [
  { value: "FLM_VENDOR", label: "Vendor FLM" },
  { value: "INTERNAL", label: "ROH" },
];

const PKS_STATUS_OPTIONS: { value: PksStatus; label: string }[] = [
  { value: "aktif", label: "Aktif" },
  { value: "need_renewal", label: "Need Renewal" },
  { value: "expired", label: "Expired" },
  { value: "terminated", label: "Terminated" },
  { value: "belum_diisi", label: "Tidak ada" },
];

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
  pksTypes?: PksType[];
}

/**
 * Search + Status + PKS Type & Status filter for the admin Vendors list (Req 5.3, 5.6).
 * pks_type + pks_status are required together; status select is disabled until type is chosen.
 */
export function VendorFilterBar({
  params,
  searchInput,
  onSearchInputChange,
  onParamsChange,
  pksTypes = [],
}: VendorFilterBarProps) {
  const [selectedType, setSelectedType] = useState<string>(params.pks_type ?? "");

  useEffect(() => {
    setSelectedType(params.pks_type ?? "");
  }, [params.pks_type]);

  const pksTypeOptions = useMemo(() => {
    return pksTypes
      .filter((t) => t.is_active)
      .sort((a, b) => a.code.localeCompare(b.code))
      .map((t) => ({
        value: t.code,
        label: t.name ? `${t.code} - ${t.name}` : t.code,
      }));
  }, [pksTypes]);

  function handleTypeChange(val: string | null) {
    const nextType = val ?? "";
    setSelectedType(nextType);
    if (!nextType) {
      // Clearing type clears both type and status filter
      onParamsChange({ pks_type: undefined, pks_status: undefined, page: 1 });
      return;
    }
    // If status was already chosen, update the pair with new type
    if (params.pks_status) {
      onParamsChange({ pks_type: nextType, pks_status: params.pks_status, page: 1 });
    }
  }

  function handleStatusChange(val: string | null) {
    if (!val || !selectedType) {
      onParamsChange({ pks_type: undefined, pks_status: undefined, page: 1 });
      return;
    }
    onParamsChange({
      pks_type: selectedType,
      pks_status: val as PksStatus,
      page: 1,
    });
  }

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

      <FilterSelect
        label="Type PKS"
        options={pksTypeOptions}
        value={selectedType || null}
        onChange={handleTypeChange}
        placeholder="Semua"
      />

      <FilterSelect
        label="Status PKS"
        options={PKS_STATUS_OPTIONS}
        value={params.pks_status ?? null}
        disabled={!selectedType}
        onChange={handleStatusChange}
        placeholder="Semua"
      />

      <FilterSelect
        label="Jenis vendor"
        options={KIND_OPTIONS}
        value={params.kind ?? null}
        onChange={(value) =>
          onParamsChange({ kind: (value as VendorKind | null) ?? undefined, page: 1 })
        }
        placeholder="Semua"
      />
    </div>
  );
}
