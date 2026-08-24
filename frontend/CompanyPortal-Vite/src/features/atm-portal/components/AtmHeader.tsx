/**
 * ATM Profile header: master data fields in a responsive grid (3-col
 * >=1024px, 2-col 768-1023px, 1-col <768px) + replenishment status badge.
 * Monetary fields formatted via formatRupiahDecimal (em dash for null).
 */

import { Skeleton } from "@/components/feedback/Skeleton";
import { formatRupiahDecimal } from "../lib/formatters";
import type { AtmProfileMasterData } from "../types";
import { StatusBadge } from "./StatusBadge";

const SKELETON_ROW_COUNT = 3;

interface Field {
  readonly label: string;
  readonly value: string;
  readonly mono?: boolean;
  readonly ariaLabel?: string;
}

function fields(data: AtmProfileMasterData): readonly Field[] {
  return [
    { label: "Terminal ID", value: data.terminal_id, mono: true },
    { label: "Lokasi", value: data.location_name },
    { label: "Alamat", value: data.address },
    { label: "Tipe Mesin", value: data.machine_type },
    { label: "Brand", value: data.brand },
    { label: "Model", value: data.model },
    { label: "Tipe Penempatan", value: data.deployment_type },
    { label: "Jam Operasional", value: data.operation_hours },
    {
      label: "Kapasitas",
      value: formatRupiahDecimal(data.capacity_amount),
      ariaLabel: ariaLabelForAmount(data.capacity_amount),
    },
    {
      label: "Batas Rendah",
      value: formatRupiahDecimal(data.low_threshold_amount),
      ariaLabel: ariaLabelForAmount(data.low_threshold_amount),
    },
    {
      label: "Batas Kritis",
      value: formatRupiahDecimal(data.critical_threshold_amount),
      ariaLabel: ariaLabelForAmount(data.critical_threshold_amount),
    },
    { label: "Status Aktif", value: data.is_active ? "Aktif" : "Tidak Aktif" },
  ];
}

function ariaLabelForAmount(value: string | null): string {
  return value === null ? "tidak tersedia" : formatRupiahDecimal(value);
}

interface AtmHeaderProps {
  data: AtmProfileMasterData | undefined;
  isLoading: boolean;
}

export function AtmHeader({ data, isLoading }: AtmHeaderProps) {
  if (isLoading || !data) {
    return (
      <section
        aria-label="Data ATM"
        className="grid grid-cols-1 gap-4 rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-0)] p-5 md:grid-cols-2 lg:grid-cols-3"
      >
        {Array.from({ length: SKELETON_ROW_COUNT }).map((_, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: generic loading placeholder rows have no stable identity
          <div key={`atm-header-skeleton-${i}`} className="flex flex-col gap-1.5">
            <Skeleton height={12} width="40%" />
            <Skeleton height={16} width="70%" />
          </div>
        ))}
      </section>
    );
  }

  return (
    <section
      aria-label="Data ATM"
      className="rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-0)] p-5"
    >
      <div className="mb-4 flex items-center justify-between gap-3">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-[var(--n-600)]">
          Informasi ATM
        </h2>
        <StatusBadge status={data.replenishment_status} />
      </div>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
        {fields(data).map((field) => (
          <div key={field.label} className="flex flex-col gap-1">
            <span className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]">
              {field.label}
            </span>
            <span
              className={`text-sm text-[var(--n-900)] ${field.mono ? "font-mono" : ""}`}
              aria-label={field.ariaLabel}
            >
              {field.value}
            </span>
          </div>
        ))}
      </div>
    </section>
  );
}
