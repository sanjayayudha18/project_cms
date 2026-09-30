/**
 * Forecast Browser recap (forecast-browser-summary FR4.2-4.3): KPI strip +
 * one row per FLM vendor × region, sorted by the server with the most
 * not-yet-requested ATMs first, so no vendor/region has to be picked blindly
 * and nothing is missed. The no-active-vendor group (flm_vendor === "") is
 * listed too — those ATMs can't appear under any vendor filter.
 */

import { Skeleton } from "@/components/feedback/Skeleton";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { formatIDR } from "@/lib/utils/formatCurrency";
import { AlertCircle, AlertTriangle, CheckCircle2, Clock } from "lucide-react";
import type { ForecastSummaryGroup, ForecastSummaryResponse } from "./types";

/** Stable key of a group; "" identifies the no-active-vendor group. */
export function summaryGroupKey(
  group: Pick<ForecastSummaryGroup, "flm_vendor" | "flm_vendor_region">,
): string {
  return group.flm_vendor === "" ? "" : `${group.flm_vendor}|${group.flm_vendor_region}`;
}

interface ForecastSummaryProps {
  data: ForecastSummaryResponse | undefined;
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  /** summaryGroupKey of the group the detail table is filtered to; null = none. */
  activeKey: string | null;
  onSelectGroup: (group: ForecastSummaryGroup) => void;
}

export function ForecastSummary({
  data,
  isLoading,
  isError,
  onRetry,
  activeKey,
  onSelectGroup,
}: ForecastSummaryProps) {
  if (isError) {
    return (
      <div className="flex flex-col items-center gap-3 rounded-[var(--radius-lg)] border border-[var(--n-200)] px-4 py-8">
        <AlertCircle className="h-8 w-8 text-[var(--danger-fg)]" aria-hidden="true" />
        <p className="text-sm text-[var(--n-700)]">Gagal memuat ringkasan forecast</p>
        <Button variant="secondary" onClick={onRetry}>
          Coba Lagi
        </Button>
      </div>
    );
  }
  if (isLoading || !data) {
    return <Skeleton height={160} className="w-full" />;
  }

  const { totals, groups } = data;
  return (
    <section aria-label="Ringkasan forecast" className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <KpiCard label="Perlu isi" value={totals.atm_count} amount={totals.amount_replenish} />
        <KpiCard label="Sudah di-request" value={totals.requested_atm_count} />
        <KpiCard
          label="Belum di-request"
          value={totals.unrequested_atm_count}
          amount={totals.unrequested_amount_replenish}
        />
        <KpiCard
          label="Tanpa vendor aktif"
          value={totals.unassigned_atm_count}
          warn={totals.unassigned_atm_count > 0}
        />
      </div>

      <div className="rounded-[var(--radius-lg)] border border-[var(--n-200)]">
        <div className="overflow-x-auto">
          <table
            aria-label="Ringkasan per Vendor dan Region"
            className="w-full min-w-[720px] border-collapse text-sm"
          >
            <thead>
              <tr className="border-b border-[var(--n-200)] bg-[var(--n-50)] text-[var(--n-600)]">
                <th scope="col" className="px-3 py-2 text-left font-medium">
                  Vendor
                </th>
                <th scope="col" className="px-3 py-2 text-left font-medium">
                  Region
                </th>
                <th scope="col" className="px-3 py-2 text-right font-medium">
                  ATM
                </th>
                <th scope="col" className="px-3 py-2 text-right font-medium">
                  Total (IDR)
                </th>
                <th scope="col" className="px-3 py-2 text-right font-medium">
                  Sudah
                </th>
                <th scope="col" className="px-3 py-2 text-right font-medium">
                  Belum
                </th>
                <th scope="col" className="px-3 py-2 text-left font-medium">
                  Status
                </th>
                <th scope="col" className="px-3 py-2">
                  <span className="sr-only">Aksi</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {groups.length === 0 && (
                <tr>
                  <td colSpan={8} className="px-3 py-8 text-center text-[var(--n-700)]">
                    Tidak ada rekomendasi forecast untuk tanggal ini
                  </td>
                </tr>
              )}
              {groups.map((group) => (
                <GroupRow
                  key={summaryGroupKey(group)}
                  group={group}
                  isActive={summaryGroupKey(group) === activeKey}
                  onSelect={() => onSelectGroup(group)}
                />
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}

function GroupRow({
  group,
  isActive,
  onSelect,
}: {
  group: ForecastSummaryGroup;
  isActive: boolean;
  onSelect: () => void;
}) {
  const isUnassigned = group.flm_vendor === "";
  const vendorLabel = isUnassigned ? "Tanpa vendor aktif" : group.flm_vendor;
  // review R1: a vendor branch without a region is its own row (V, "").
  const regionLabel = isUnassigned ? "-" : group.flm_vendor_region || "(tanpa region)";
  const regionSuffix = isUnassigned ? "" : ` ${group.flm_vendor_region || "tanpa region"}`;
  return (
    <tr
      aria-current={isActive ? "true" : undefined}
      className={`border-b border-[var(--n-100)] last:border-0 ${isActive ? "bg-[var(--red-50)]" : ""}`}
    >
      <td className="px-3 py-2 font-medium text-[var(--n-900)]">{vendorLabel}</td>
      <td className="px-3 py-2">{regionLabel}</td>
      <td className="px-3 py-2 text-right tabular-nums">{group.atm_count}</td>
      <td className="px-3 py-2 text-right tabular-nums">Rp {formatIDR(group.amount_replenish)}</td>
      <td className="px-3 py-2 text-right tabular-nums">
        {isUnassigned ? "-" : group.requested_atm_count}
      </td>
      <td className="px-3 py-2 text-right font-semibold tabular-nums">
        {group.unrequested_atm_count}
      </td>
      <td className="px-3 py-2">
        <GroupStatus group={group} />
      </td>
      <td className="px-3 py-1 text-right">
        <Button variant="secondary" onClick={onSelect}>
          Lihat
          <span className="sr-only">
            {" "}
            ATM {vendorLabel}
            {regionSuffix}
          </span>
        </Button>
      </td>
    </tr>
  );
}

function GroupStatus({ group }: { group: ForecastSummaryGroup }) {
  if (group.flm_vendor === "") {
    return <Badge variant="warning" icon={AlertTriangle} label="Perlu master data" />;
  }
  if (group.unrequested_atm_count === 0) {
    return <Badge variant="success" icon={CheckCircle2} label="Selesai" />;
  }
  return <Badge variant="info" icon={Clock} label="Belum lengkap" />;
}

function KpiCard({
  label,
  value,
  amount,
  warn = false,
}: {
  label: string;
  value: number;
  amount?: number;
  warn?: boolean;
}) {
  return (
    <div className="rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-0)] p-4">
      <p className="flex items-center gap-1 text-sm font-medium text-[var(--n-600)]">
        {warn && <AlertTriangle className="h-4 w-4 text-[var(--warning-fg)]" aria-hidden="true" />}
        {label}
      </p>
      <p className="mt-1 text-xl font-semibold tabular-nums text-[var(--n-900)]">
        {value.toLocaleString("id-ID")}{" "}
        <span className="text-sm font-normal text-[var(--n-600)]">ATM</span>
      </p>
      {amount !== undefined && (
        <p className="mt-0.5 text-sm tabular-nums text-[var(--n-600)]">Rp {formatIDR(amount)}</p>
      )}
    </div>
  );
}
