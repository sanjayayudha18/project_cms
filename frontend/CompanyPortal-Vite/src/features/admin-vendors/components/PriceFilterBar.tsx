import { FilterSelect } from "@/components/ui/FilterSelect";
import { machineLabel } from "../lib/packagePrice";
import type { PriceStatus } from "../lib/packagePrice";

export interface PriceFilters {
  /** null = all packages */
  package: string | null;
  /** null = all machine groups */
  machine_group: "ATM" | "CDM_CRM" | null;
  /** null = all statuses */
  status: PriceStatus | null;
}

const STATUS_OPTIONS: { value: PriceStatus; label: string }[] = [
  { value: "Berlaku", label: "Berlaku" },
  { value: "Dijadwalkan", label: "Dijadwalkan" },
  { value: "Berakhir", label: "Berakhir" },
];

interface PriceFilterBarProps {
  packageOptions: string[];
  machineOptions: ("ATM" | "CDM_CRM")[];
  value: PriceFilters;
  onChange: (value: PriceFilters) => void;
  matchCount: number;
}

/** Filter controls for the Harga Paket tab: paket / mesin / status, plus a live tier count (Req 7). */
export function PriceFilterBar({
  packageOptions,
  machineOptions,
  value,
  onChange,
  matchCount,
}: PriceFilterBarProps) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-4">
      <div className="flex flex-wrap items-end gap-4">
        <FilterSelect
          label="Paket"
          placeholder="Semua paket"
          options={packageOptions.map((p) => ({ value: p, label: p }))}
          value={value.package}
          onChange={(v) => onChange({ ...value, package: v })}
        />
        <FilterSelect
          label="Mesin"
          placeholder="Semua mesin"
          options={machineOptions.map((m) => ({ value: m, label: machineLabel(m) }))}
          value={value.machine_group}
          onChange={(v) =>
            onChange({ ...value, machine_group: v as PriceFilters["machine_group"] })
          }
        />
        <FilterSelect
          label="Status"
          placeholder="Semua status"
          options={STATUS_OPTIONS}
          value={value.status}
          onChange={(v) => onChange({ ...value, status: v as PriceStatus | null })}
        />
      </div>
      <span className="tabular-nums text-sm font-medium text-[var(--n-600)]">
        {matchCount} tingkat
      </span>
    </div>
  );
}
