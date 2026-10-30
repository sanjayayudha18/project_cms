import { Link } from "@tanstack/react-router";
import { Download, RefreshCw, Search } from "lucide-react";
import { useState } from "react";

import { Badge } from "@/components/ui/Badge";
import type { BadgeVariant } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { FilterSelect } from "@/components/ui/FilterSelect";
import { PageHeader } from "@/components/ui/PageHeader";
import { PageNumbers } from "@/components/ui/PageNumbers";
import { formatAtmDate } from "@/features/atm-portal/lib/formatters";
import { getTodayWib } from "@/features/eod-monitoring/utils";
import {
  ExportProgressDialog,
  ReportExportNotice,
  type ReportParams,
  useExportFlow,
} from "@/features/exports";
import { useVendorOptions } from "@/features/vendor-request/hooks";
import { formatIDRFull } from "@/lib/utils/formatters";
import {
  CLASSIFICATION_LABELS,
  type Classification,
  type ClassificationView,
  rangeError,
  useClassifications,
} from "./api";

export const CLASS_VARIANT: Record<Classification, BadgeVariant> = {
  on_schedule: "success",
  early: "info",
  late: "warning",
  not_done: "danger",
  no_order: "danger",
  pending: "neutral",
  void: "neutral",
};

const inputClass =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]";
const labelClass = "text-xs font-medium uppercase tracking-wider text-[var(--n-600)]";

function addDays(iso: string, days: number): string {
  const d = new Date(`${iso}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + days);
  return d.toISOString().slice(0, 10);
}

function date(iso: string | null): string {
  return iso ? formatAtmDate(new Date(iso)) : "—";
}

function total(row: ClassificationView, key: "order_amount" | "itm_amount" | "diff"): number {
  return row.amounts.reduce((s, a) => s + a[key], 0);
}

function offsetLabel(row: ClassificationView): string {
  const n = row.workday_offset;
  if (n === null || n === 0) return "";
  return n < 0 ? ` (${-n} hari kerja lebih awal)` : ` (${n} hari kerja terlambat)`;
}

/**
 * Realisasi pengisian vs order (replenishment-realisasi FR4.1, FR6): one row per tiket replenish
 * (or per ITM fill without an order), classified daily by the EOD job against itm_replenish.
 */
export function ReplenishmentScreen() {
  const exportFlow = useExportFlow();
  const today = getTodayWib();
  const [dateFrom, setDateFrom] = useState(addDays(today, -6));
  const [dateTo, setDateTo] = useState(today);
  const [vendorId, setVendorId] = useState<string | null>(null);
  const [regionCode, setRegionCode] = useState<string | null>(null);
  const [classification, setClassification] = useState<string | null>(null);
  const [onlyDiff, setOnlyDiff] = useState(false);
  const [page, setPage] = useState(1);

  const options = useVendorOptions();
  const filter = {
    dateFrom,
    dateTo,
    vendorId: vendorId === null ? null : Number(vendorId),
    regionCode,
    classification: classification as Classification | null,
    onlyDiff,
  };
  const rangeMsg = rangeError(dateFrom, dateTo);
  const list = useClassifications({ ...filter, page });
  const rows = list.data?.items ?? [];
  const pages = Math.max(1, Math.ceil((list.data?.total ?? 0) / (list.data?.page_size ?? 20)));

  function changed<T>(set: (v: T) => void) {
    return (v: T) => {
      set(v);
      setPage(1);
    };
  }

  function requestExport(report: "replenishment-amount-mismatch" | "replenishment-trips") {
    const params: ReportParams = { date_from: dateFrom, date_to: dateTo };
    if (filter.vendorId !== null) params.vendor_id = filter.vendorId;
    if (regionCode) params.region_code = regionCode;
    if (report === "replenishment-amount-mismatch" && classification) {
      params.classification = classification;
    }
    void exportFlow.request({ kind: "report", report, params });
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Operasi kas"
        title="Realisasi pengisian"
        description="Pengisian ATM menurut ITM dibandingkan dengan jadwal dan nominal order, dalam hari kerja."
        actions={
          <>
            <Button
              variant="secondary"
              disabled={rangeMsg !== null || exportFlow.isRequesting}
              onClick={() => requestExport("replenishment-amount-mismatch")}
            >
              <Download className="h-4 w-4" aria-hidden="true" />
              Selisih nominal (CSV)
            </Button>
            <Button
              variant="secondary"
              disabled={rangeMsg !== null || exportFlow.isRequesting}
              onClick={() => requestExport("replenishment-trips")}
            >
              <Download className="h-4 w-4" aria-hidden="true" />
              Trip per ATM (CSV)
            </Button>
          </>
        }
      />
      <ReportExportNotice error={exportFlow.requestError} />
      <ExportProgressDialog {...exportFlow.dialogProps} />

      <div className="flex flex-wrap items-end gap-4">
        <div className="flex flex-col gap-1">
          <label htmlFor="rp-from" className={labelClass}>
            Dari tanggal
          </label>
          <input
            id="rp-from"
            type="date"
            value={dateFrom}
            onChange={(e) => changed(setDateFrom)(e.target.value)}
            className={inputClass}
          />
        </div>
        <div className="flex flex-col gap-1">
          <label htmlFor="rp-to" className={labelClass}>
            Sampai tanggal
          </label>
          <input
            id="rp-to"
            type="date"
            value={dateTo}
            onChange={(e) => changed(setDateTo)(e.target.value)}
            className={inputClass}
          />
        </div>
        <FilterSelect
          label="Vendor"
          options={(options.data?.vendors ?? []).map((v) => ({
            value: String(v.id),
            label: v.name,
          }))}
          value={vendorId}
          onChange={changed(setVendorId)}
          placeholder="Semua vendor"
        />
        <FilterSelect
          label="Region"
          options={(options.data?.regions ?? []).map((r) => ({ value: r, label: r }))}
          value={regionCode}
          onChange={changed(setRegionCode)}
          placeholder="Semua region"
        />
        <FilterSelect
          label="Klasifikasi"
          options={Object.entries(CLASSIFICATION_LABELS).map(([value, label]) => ({
            value,
            label,
          }))}
          value={classification}
          onChange={changed(setClassification)}
          placeholder="Semua klasifikasi"
        />
        <label className="flex min-h-[44px] items-center gap-2 text-sm text-[var(--n-700)]">
          <input
            type="checkbox"
            checked={onlyDiff}
            onChange={(e) => changed(setOnlyDiff)(e.target.checked)}
            className="h-4 w-4 accent-[var(--red-500)]"
          />
          Hanya yang nominalnya selisih
        </label>
      </div>
      {rangeMsg && (
        <p role="alert" className="text-sm text-[var(--danger-fg)]">
          {rangeMsg}
        </p>
      )}

      <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
        <table className="w-full min-w-[1080px] border-collapse text-sm">
          <thead>
            <tr className="border-b border-[var(--n-200)] bg-[var(--n-50)] text-[var(--n-600)]">
              <th className="px-3 py-2 text-left font-medium">Jadwal</th>
              <th className="px-3 py-2 text-left font-medium">Tiket / Request</th>
              <th className="px-3 py-2 text-left font-medium">ATM</th>
              <th className="px-3 py-2 text-left font-medium">Vendor / Cabang</th>
              <th className="px-3 py-2 text-left font-medium">Klasifikasi</th>
              <th className="px-3 py-2 text-left font-medium">Realisasi ITM</th>
              <th className="px-3 py-2 text-right font-medium">Order</th>
              <th className="px-3 py-2 text-right font-medium">ITM</th>
              <th className="px-3 py-2 text-right font-medium">Selisih</th>
              <th className="px-3 py-2 text-left font-medium">Catatan</th>
            </tr>
          </thead>
          <tbody>
            {list.isLoading && (
              <tr>
                <td colSpan={10} className="px-3 py-8 text-center text-[var(--n-500)]">
                  Memuat data...
                </td>
              </tr>
            )}
            {list.isError && (
              <tr>
                <td colSpan={10} className="px-3 py-8 text-center">
                  <span className="text-[var(--danger-fg)]">Gagal memuat data. </span>
                  <Button variant="secondary" onClick={() => list.refetch()}>
                    <RefreshCw className="h-4 w-4" aria-hidden="true" />
                    Coba lagi
                  </Button>
                </td>
              </tr>
            )}
            {!list.isLoading && !list.isError && rows.length === 0 && (
              <tr>
                <td colSpan={10} className="px-3 py-12 text-center">
                  <Search
                    className="mx-auto mb-2 h-10 w-10 text-[var(--n-300)]"
                    aria-hidden="true"
                  />
                  <span className="text-[var(--n-600)]">
                    Belum ada data realisasi untuk filter ini.
                  </span>
                </td>
              </tr>
            )}
            {rows.map((row) => {
              const diff = total(row, "diff");
              return (
                <tr key={row.id} className="border-b border-[var(--n-100)] last:border-0">
                  <td className="px-3 py-2">{date(row.scheduled_date)}</td>
                  <td className="px-3 py-2">
                    <span className="block font-mono text-xs">{row.ticket_number ?? "—"}</span>
                    {row.request_id !== null && (
                      <Link
                        to="/replenishment/vendor-requests/$id"
                        params={{ id: String(row.request_id) }}
                        className="text-xs font-medium text-[var(--red-600)] hover:underline"
                      >
                        {row.request_number}
                      </Link>
                    )}
                  </td>
                  <td className="px-3 py-2 font-mono">{row.terminal_id}</td>
                  <td className="px-3 py-2">
                    <span className="block">{row.vendor_name ?? "—"}</span>
                    <span className="text-xs text-[var(--n-500)]">
                      {row.branch_code ?? ""} {row.region_code ? `· ${row.region_code}` : ""}
                    </span>
                  </td>
                  <td className="px-3 py-2">
                    <Badge
                      variant={CLASS_VARIANT[row.classification]}
                      label={CLASSIFICATION_LABELS[row.classification] + offsetLabel(row)}
                    />
                  </td>
                  <td className="px-3 py-2">
                    {date(row.realized_date)}
                    {row.realized_time && (
                      <span className="block text-xs text-[var(--n-500)]">
                        {row.realized_time.slice(0, 5)} WIB
                      </span>
                    )}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {formatIDRFull(total(row, "order_amount"))}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {formatIDRFull(total(row, "itm_amount"))}
                  </td>
                  <td
                    className={`px-3 py-2 text-right tabular-nums ${diff !== 0 ? "font-semibold text-[var(--danger-fg)]" : ""}`}
                  >
                    {diff > 0 ? "+" : ""}
                    {formatIDRFull(diff)}
                  </td>
                  <td className="px-3 py-2">
                    <div className="flex flex-wrap gap-1">
                      {row.vendor_mismatch && (
                        <Badge variant="warning" label="Beda laporan vendor" />
                      )}
                      {row.vendor_report_late && (
                        <Badge variant="danger" label="Laporan vendor telat" />
                      )}
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {pages > 1 && (
        <nav aria-label="Halaman" className="flex flex-wrap items-center justify-end gap-2">
          <span className="mr-auto text-sm text-[var(--n-500)]">{list.data?.total ?? 0} baris</span>
          <Button variant="secondary" disabled={page <= 1} onClick={() => setPage(page - 1)}>
            Sebelumnya
          </Button>
          <PageNumbers current={page} total={pages} onChange={setPage} />
          <Button variant="secondary" disabled={page >= pages} onClick={() => setPage(page + 1)}>
            Berikutnya
          </Button>
        </nav>
      )}
    </div>
  );
}
