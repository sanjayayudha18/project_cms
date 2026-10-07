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
import { VISIT_QUOTA_CHECKER_ROLES, useResetVendorQuota } from "@/features/atm-portal/visitQuota";
import { useAuthStore } from "@/lib/auth/store";
import { useToast } from "@/lib/hooks/useToast";
import { formatIDR } from "@/lib/utils/formatCurrency";
import { AlertCircle, AlertTriangle, CheckCircle2, Clock, RotateCcw } from "lucide-react";
import { useState } from "react";
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

  return <SummaryContent data={data} activeKey={activeKey} onSelectGroup={onSelectGroup} />;
}

const PAGE_SIZE_OPTIONS = [10, 20, 50] as const;
const SELECT_CLASS =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]";
const LABEL_CLASS = "text-xs font-medium uppercase tracking-wider text-[var(--n-600)]";

function SummaryContent({
  data,
  activeKey,
  onSelectGroup,
}: {
  data: ForecastSummaryResponse;
  activeKey: string | null;
  onSelectGroup: (group: ForecastSummaryGroup) => void;
}) {
  const { totals, groups } = data;
  const [vendorPick, setVendorPick] = useState("");
  const [regionPick, setRegionPick] = useState("");
  const [pageSize, setPageSize] = useState<number>(PAGE_SIZE_OPTIONS[0]);
  const [pageRaw, setPageRaw] = useState(1);

  const vendors = [...new Set(groups.map((g) => g.flm_vendor).filter(Boolean))].sort();
  const regions = [...new Set(groups.map((g) => g.flm_vendor_region).filter(Boolean))].sort();
  // A pick that no longer exists (e.g. after a date change) falls back to "Semua".
  const vendor = vendors.includes(vendorPick) ? vendorPick : "";
  const region = regions.includes(regionPick) ? regionPick : "";

  // atm-visit-quota (FR6.6): reset kuota is per vendor (all its regions), so it
  // is offered once a single vendor is picked, not per vendor × region row.
  const vendorId = vendor ? (groups.find((g) => g.flm_vendor === vendor)?.vendor_id ?? 0) : 0;

  const filtered = groups.filter(
    (g) => (!vendor || g.flm_vendor === vendor) && (!region || g.flm_vendor_region === region),
  );
  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const page = Math.min(pageRaw, totalPages);
  const pageGroups = filtered.slice((page - 1) * pageSize, page * pageSize);

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

      <h2 className="text-base font-semibold text-[var(--n-900)]">Summary Rekomendasi ATM</h2>
      <div className="flex flex-wrap items-end gap-4">
        <div className="flex flex-col gap-1">
          <label htmlFor="summary-vendor-filter" className={LABEL_CLASS}>
            Vendor
          </label>
          <select
            id="summary-vendor-filter"
            value={vendor}
            onChange={(e) => {
              setVendorPick(e.target.value);
              setPageRaw(1);
            }}
            className={SELECT_CLASS}
          >
            <option value="">Semua</option>
            {vendors.map((v) => (
              <option key={v} value={v}>
                {v}
              </option>
            ))}
          </select>
        </div>
        <div className="flex flex-col gap-1">
          <label htmlFor="summary-region-filter" className={LABEL_CLASS}>
            Region
          </label>
          <select
            id="summary-region-filter"
            value={region}
            onChange={(e) => {
              setRegionPick(e.target.value);
              setPageRaw(1);
            }}
            className={SELECT_CLASS}
          >
            <option value="">Semua</option>
            {regions.map((r) => (
              <option key={r} value={r}>
                {r}
              </option>
            ))}
          </select>
        </div>
        {vendorId > 0 && <VendorQuotaReset vendorId={vendorId} vendorName={vendor} />}
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
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={8} className="px-3 py-8 text-center text-[var(--n-700)]">
                    {groups.length === 0
                      ? "Tidak ada rekomendasi forecast untuk tanggal ini"
                      : "Tidak ada data yang cocok dengan filter"}
                  </td>
                </tr>
              )}
              {pageGroups.map((group) => (
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

        <div className="flex flex-wrap items-center justify-between gap-4 border-t border-[var(--n-200)] px-4 py-3 text-sm text-[var(--n-600)]">
          <div className="flex items-center gap-2">
            <label htmlFor="summary-page-size">Baris per halaman ringkasan</label>
            <select
              id="summary-page-size"
              value={pageSize}
              onChange={(e) => {
                setPageSize(Number(e.target.value));
                setPageRaw(1);
              }}
              className={SELECT_CLASS}
            >
              {PAGE_SIZE_OPTIONS.map((size) => (
                <option key={size} value={size}>
                  {size}
                </option>
              ))}
            </select>
          </div>
          <span>
            Menampilkan {pageGroups.length} dari {filtered.length}
          </span>
          <div className="flex items-center gap-2">
            <span>
              Halaman {page} dari {totalPages}
            </span>
            <Button variant="secondary" disabled={page <= 1} onClick={() => setPageRaw(page - 1)}>
              Sebelumnya<span className="sr-only"> ringkasan</span>
            </Button>
            <Button
              variant="secondary"
              disabled={page >= totalPages}
              onClick={() => setPageRaw(page + 1)}
            >
              Berikutnya<span className="sr-only"> ringkasan</span>
            </Button>
          </div>
        </div>
      </div>
    </section>
  );
}

const RESET_TOAST_MS = 6000;

/** Checker-only, two-step "Reset kuota vendor" (resets every ATM of the vendor). */
function VendorQuotaReset({ vendorId, vendorName }: { vendorId: number; vendorName: string }) {
  const role = useAuthStore((s) => s.user?.role);
  const { toast, dismiss } = useToast();
  const resetMutation = useResetVendorQuota();
  const [confirming, setConfirming] = useState(false);

  if (!role || !VISIT_QUOTA_CHECKER_ROLES.includes(role)) return null;

  async function handleReset(): Promise<void> {
    try {
      const res = await resetMutation.mutateAsync(vendorId);
      const skipped =
        res.skipped_count > 0 ? `, ${res.skipped_count} dilewati (paket tidak dikenali)` : "";
      const id = toast({
        type: "success",
        message: `Kuota ${vendorName} di-reset: ${res.reset_count} ATM${skipped}`,
      });
      setTimeout(() => dismiss(id), RESET_TOAST_MS);
    } catch (err) {
      const id = toast({
        type: "error",
        message: err instanceof Error ? err.message : "Gagal me-reset kuota vendor",
      });
      setTimeout(() => dismiss(id), RESET_TOAST_MS);
    } finally {
      setConfirming(false);
    }
  }

  if (!confirming) {
    return (
      <Button variant="secondary" onClick={() => setConfirming(true)}>
        <RotateCcw className="mr-1.5 h-4 w-4" aria-hidden="true" />
        Reset Kuota Vendor
      </Button>
    );
  }
  return (
    <div className="flex flex-wrap items-center gap-2 text-sm">
      <span>Reset sisa kunjungan semua ATM {vendorName} ke kuota paketnya?</span>
      <Button variant="secondary" onClick={() => setConfirming(false)}>
        Batal
      </Button>
      <Button disabled={resetMutation.isPending} onClick={handleReset}>
        {resetMutation.isPending ? "Me-reset..." : "Ya, Reset"}
      </Button>
    </div>
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
