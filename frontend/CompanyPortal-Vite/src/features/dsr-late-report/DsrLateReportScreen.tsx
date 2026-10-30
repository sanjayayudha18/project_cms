import { Download, RefreshCw, Search } from "lucide-react";
import { useState } from "react";

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
import type { ApiError } from "@/lib/api/client";
import { useAuthStore } from "@/lib/auth/store";
import { StatusBadge } from "./StatusBadge";
import { WarningsPanel } from "./WarningsPanel";
import {
  type DsrLateStatus,
  STATUS_LABELS,
  useDsrLateDaily,
  useDsrLateSummary,
  useDsrLateWarnings,
} from "./api";

type Tab = "summary" | "daily";

const inputClass =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]";
const labelClass = "text-xs font-medium uppercase tracking-wider text-[var(--n-600)]";
const thClass = "px-3 py-2 text-left font-medium";
const thNumClass = "px-3 py-2 text-right font-medium";

const pad = (n: number) => String(n).padStart(2, "0");

/** dd/MM/yyyy HH:mm:ss in WIB (UTC+7, no DST). */
function formatWib(iso: string | null): string {
  if (!iso) return "—";
  const d = new Date(new Date(iso).getTime() + 7 * 3_600_000);
  return `${pad(d.getUTCDate())}/${pad(d.getUTCMonth() + 1)}/${d.getUTCFullYear()} ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}:${pad(d.getUTCSeconds())}`;
}

function isForbidden(e: unknown): boolean {
  return (e as ApiError | null)?.status === 403;
}

function StateRow({ cols, children }: { cols: number; children: React.ReactNode }) {
  return (
    <tr>
      <td colSpan={cols} className="px-3 py-10 text-center text-[var(--n-600)]">
        {children}
      </td>
    </tr>
  );
}

function ErrorCell({ error, retry }: { error: unknown; retry: () => void }) {
  if (isForbidden(error)) {
    return <span role="alert">Anda tidak memiliki akses ke laporan ini.</span>;
  }
  return (
    <span role="alert">
      <span className="text-[var(--danger-fg)]">Gagal memuat data. </span>
      <Button variant="secondary" onClick={retry}>
        <RefreshCw className="h-4 w-4" aria-hidden="true" />
        Coba lagi
      </Button>
    </span>
  );
}

/** Laporan DSR telat / tidak kirim per cabang vendor (dsr-late-report FR6): dasar penalti FLM. */
export function DsrLateReportScreen() {
  const exportFlow = useExportFlow();
  const isAdmin = useAuthStore((s) => s.user?.role === "ADMIN");
  const [month, setMonth] = useState(getTodayWib().slice(0, 7));
  const [vendorId, setVendorId] = useState<string | null>(null);
  const [branchId, setBranchId] = useState<string | null>(null);
  const [status, setStatus] = useState<string | null>(null);
  const [tab, setTab] = useState<Tab>("summary");
  const [page, setPage] = useState(1);

  const vendorNum = vendorId === null ? null : Number(vendorId);
  const branchNum = branchId === null ? null : Number(branchId);
  const filter = { month, vendorId: vendorNum, vendorBranchId: branchNum };

  const options = useVendorOptions();
  const warnings = useDsrLateWarnings(month, vendorNum);
  // Fetched without the branch filter so it also feeds the (vendor-dependent) cabang options.
  const summary = useDsrLateSummary({ ...filter, vendorBranchId: null });
  const daily = useDsrLateDaily({ ...filter, status: status as DsrLateStatus | null }, page);

  const summaryRows = (summary.data?.items ?? []).filter(
    (r) => branchNum === null || r.vendor_branch_id === branchNum,
  );
  const branchOptions =
    vendorNum === null
      ? []
      : (summary.data?.items ?? []).map((r) => ({
          value: String(r.vendor_branch_id),
          label: `${r.branch_code} – ${r.branch_name}`,
        }));
  const dailyRows = daily.data?.items ?? [];
  const pages = Math.max(1, Math.ceil((daily.data?.total ?? 0) / (daily.data?.page_size || 20)));
  const startDate = daily.data?.start_date ?? summary.data?.start_date ?? null;
  const featureOff =
    (daily.data && daily.data.start_date === null) ||
    (summary.data && summary.data.start_date === null);

  function changed<T>(set: (v: T) => void) {
    return (v: T) => {
      set(v);
      setPage(1);
    };
  }

  function requestExport() {
    const params: ReportParams = { month };
    if (vendorNum !== null) params.vendor_id = vendorNum;
    if (branchNum !== null) params.vendor_branch_id = branchNum;
    if (tab === "daily" && status) params.status = status;
    void exportFlow.request({
      kind: "report",
      report: tab === "daily" ? "dsr-late-daily" : "dsr-late-summary",
      params,
    });
  }

  const emptyText = featureOff
    ? "Laporan belum aktif."
    : startDate
      ? `Belum ada data untuk filter ini. Laporan dihitung mulai ${formatAtmDate(new Date(`${startDate}T00:00:00`))}.`
      : "Belum ada data untuk filter ini.";

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Monitoring"
        title="Laporan DSR Telat"
        description="DSR vendor yang terlambat atau tidak dikirim per cabang. Batas kirim 09:00 WIB pada tanggal laporan."
        actions={
          <Button
            variant="secondary"
            disabled={exportFlow.isRequesting || !month}
            onClick={requestExport}
          >
            <Download className="h-4 w-4" aria-hidden="true" />
            Ekspor CSV
          </Button>
        }
      />
      <ReportExportNotice error={exportFlow.requestError} />
      <ExportProgressDialog {...exportFlow.dialogProps} />

      {warnings.data && <WarningsPanel data={warnings.data} isAdmin={isAdmin} />}

      <div className="flex flex-wrap items-end gap-4">
        <div className="flex flex-col gap-1">
          <label htmlFor="dl-month" className={labelClass}>
            Bulan
          </label>
          <input
            id="dl-month"
            type="month"
            value={month}
            max={getTodayWib().slice(0, 7)}
            onChange={(e) => e.target.value && changed(setMonth)(e.target.value)}
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
          onChange={(v) => {
            setBranchId(null);
            changed(setVendorId)(v);
          }}
          placeholder="Semua vendor"
        />
        <FilterSelect
          label="Cabang"
          options={branchOptions}
          value={branchId}
          onChange={changed(setBranchId)}
          placeholder={vendorId === null ? "Pilih vendor dulu" : "Semua cabang"}
        />
        {tab === "daily" && (
          <FilterSelect
            label="Status"
            options={Object.entries(STATUS_LABELS).map(([value, label]) => ({ value, label }))}
            value={status}
            onChange={changed(setStatus)}
            placeholder="Semua status"
          />
        )}
      </div>

      <div role="tablist" aria-label="Tampilan laporan" className="flex gap-2">
        {(
          [
            ["summary", "Rekap per cabang"],
            ["daily", "Detail harian"],
          ] as const
        ).map(([id, label]) => (
          <button
            key={id}
            type="button"
            role="tab"
            aria-selected={tab === id}
            onClick={() => setTab(id)}
            className={`min-h-[44px] rounded-[var(--radius-md)] border px-4 text-sm font-medium ${
              tab === id
                ? "border-[var(--red-500)] bg-[var(--red-50)] text-[var(--red-700)]"
                : "border-[var(--n-300)] text-[var(--n-700)]"
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {tab === "summary" ? (
        <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
          <table className="w-full min-w-[900px] border-collapse text-sm">
            <thead>
              <tr className="border-b border-[var(--n-200)] bg-[var(--n-50)] text-[var(--n-600)]">
                <th className={thClass}>Vendor</th>
                <th className={thClass}>Cabang</th>
                <th className={thClass}>Region</th>
                <th className={thNumClass}>Hari wajib</th>
                <th className={thNumClass}>OK</th>
                <th className={thNumClass}>Telat</th>
                <th className={thNumClass}>Tidak kirim</th>
                <th className={thNumClass}>Belum jatuh tempo</th>
                <th className={thNumClass}>Total telat (menit)</th>
              </tr>
            </thead>
            <tbody>
              {summary.isLoading && <StateRow cols={9}>Memuat data...</StateRow>}
              {summary.isError && (
                <StateRow cols={9}>
                  <ErrorCell error={summary.error} retry={() => summary.refetch()} />
                </StateRow>
              )}
              {!summary.isLoading && !summary.isError && summaryRows.length === 0 && (
                <StateRow cols={9}>
                  <Search
                    className="mx-auto mb-2 h-10 w-10 text-[var(--n-300)]"
                    aria-hidden="true"
                  />
                  {emptyText}
                </StateRow>
              )}
              {summaryRows.map((r) => (
                <tr
                  key={r.vendor_branch_id}
                  className="border-b border-[var(--n-100)] last:border-0"
                >
                  <td className="px-3 py-2">{r.vendor_name}</td>
                  <td className="px-3 py-2">
                    {r.branch_code} – {r.branch_name}
                  </td>
                  <td className="px-3 py-2">{r.region_code ?? "—"}</td>
                  <td className="px-3 py-2 text-right tabular-nums">{r.obliged_days}</td>
                  <td className="px-3 py-2 text-right tabular-nums">{r.ok_days}</td>
                  <td className="px-3 py-2 text-right tabular-nums">{r.late_days}</td>
                  <td className="px-3 py-2 text-right tabular-nums">{r.not_sent_days}</td>
                  <td className="px-3 py-2 text-right tabular-nums">{r.pending_days}</td>
                  <td className="px-3 py-2 text-right tabular-nums">{r.late_minutes_total}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <>
          <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
            <table className="w-full min-w-[960px] border-collapse text-sm">
              <thead>
                <tr className="border-b border-[var(--n-200)] bg-[var(--n-50)] text-[var(--n-600)]">
                  <th className={thClass}>Tanggal laporan</th>
                  <th className={thClass}>Vendor</th>
                  <th className={thClass}>Cabang</th>
                  <th className={thClass}>Region</th>
                  <th className={thClass}>Diterima pertama (WIB)</th>
                  <th className={thClass}>Status</th>
                  <th className={thNumClass}>Telat (menit)</th>
                </tr>
              </thead>
              <tbody>
                {daily.isLoading && <StateRow cols={7}>Memuat data...</StateRow>}
                {daily.isError && (
                  <StateRow cols={7}>
                    <ErrorCell error={daily.error} retry={() => daily.refetch()} />
                  </StateRow>
                )}
                {!daily.isLoading && !daily.isError && dailyRows.length === 0 && (
                  <StateRow cols={7}>
                    <Search
                      className="mx-auto mb-2 h-10 w-10 text-[var(--n-300)]"
                      aria-hidden="true"
                    />
                    {emptyText}
                  </StateRow>
                )}
                {dailyRows.map((r) => (
                  <tr
                    key={`${r.report_date}-${r.vendor_branch_id}`}
                    className="border-b border-[var(--n-100)] last:border-0"
                  >
                    <td className="px-3 py-2 tabular-nums">
                      {formatAtmDate(new Date(`${r.report_date}T00:00:00`))}
                    </td>
                    <td className="px-3 py-2">{r.vendor_name}</td>
                    <td className="px-3 py-2">
                      {r.branch_code} – {r.branch_name}
                    </td>
                    <td className="px-3 py-2">{r.region_code ?? "—"}</td>
                    <td className="px-3 py-2 tabular-nums">{formatWib(r.first_received_at)}</td>
                    <td className="px-3 py-2">
                      <StatusBadge status={r.status} deadline={formatWib(r.deadline)} />
                    </td>
                    <td className="px-3 py-2 text-right tabular-nums">
                      {r.status === "late" || r.status === "not_sent" ? r.late_minutes : "—"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {pages > 1 && (
            <nav aria-label="Halaman" className="flex flex-wrap items-center justify-end gap-2">
              <span className="mr-auto text-sm text-[var(--n-500)]">
                {daily.data?.total ?? 0} baris
              </span>
              <Button variant="secondary" disabled={page <= 1} onClick={() => setPage(page - 1)}>
                Sebelumnya
              </Button>
              <PageNumbers current={page} total={pages} onChange={setPage} />
              <Button
                variant="secondary"
                disabled={page >= pages}
                onClick={() => setPage(page + 1)}
              >
                Berikutnya
              </Button>
            </nav>
          )}
        </>
      )}
    </div>
  );
}
