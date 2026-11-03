import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import { DatePicker } from "@/components/ui/DatePicker";
import { EmptyState } from "@/components/ui/EmptyState";
import { FilterTabs } from "@/components/ui/FilterTabs";
import { useAuth } from "@/features/auth/useAuth";
import {
  type Classification,
  ORDER_PAGE_SIZE,
  type VendorClassification,
} from "@/features/orders/api";
import { formatDate } from "@/features/orders/labels";
import { useRealizations } from "@/features/orders/useOrders";
import { formatIDR } from "@/lib/formatters";
import { type ColumnDef, createColumnHelper } from "@tanstack/react-table";
import { ClipboardCheck, Lock } from "lucide-react";
import { useState } from "react";

// replenishment-realisasi FR5.2: this vendor's own fills as CIMB classified them from the ATM's
// ITM data (on schedule / early / late / not done / filled without an order) and the amount gap.
// Scope (vendor, branch) is enforced on the server.

type BadgeVariant = "info" | "warning" | "success" | "danger" | "neutral";
const MAX_RANGE_DAYS = 31;

export const CLASS_LABEL: Record<Classification, { label: string; variant: BadgeVariant }> = {
  on_schedule: { label: "Sesuai jadwal", variant: "success" },
  early: { label: "Maju", variant: "info" },
  late: { label: "Mundur", variant: "warning" },
  not_done: { label: "Tidak dilakukan", variant: "danger" },
  no_order: { label: "Tanpa order", variant: "danger" },
  pending: { label: "Menunggu", variant: "neutral" },
  void: { label: "Batal", variant: "neutral" },
};

type ClassFilter = "all" | Classification;
const FILTERS: readonly { value: ClassFilter; label: string }[] = [
  { value: "all", label: "Semua" },
  ...(["on_schedule", "early", "late", "not_done", "no_order", "pending"] as const).map((c) => ({
    value: c,
    label: CLASS_LABEL[c].label,
  })),
];

function shiftDays(iso: string, days: number): string {
  const d = new Date(`${iso}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + days);
  return d.toISOString().slice(0, 10);
}

export function rangeProblem(from: string, to: string): string | null {
  if (to < from) return "Tanggal akhir sebelum tanggal awal";
  if (shiftDays(from, MAX_RANGE_DAYS - 1) < to) return `Rentang maksimal ${MAX_RANGE_DAYS} hari`;
  return null;
}

const total = (r: VendorClassification, key: "order_amount" | "itm_amount" | "diff") =>
  r.amounts.reduce((s, a) => s + a[key], 0);

const columnHelper = createColumnHelper<VendorClassification>();
const columns = [
  columnHelper.accessor("scheduled_date", {
    header: "Jadwal",
    cell: (info) => formatDate(info.getValue()),
    enableSorting: false,
  }),
  columnHelper.accessor("terminal_id", {
    header: "ATM",
    cell: (info) => (
      <span>
        <span className="block font-medium">{info.getValue()}</span>
        <span className="text-xs text-neutral-500">{info.row.original.ticket_number ?? "-"}</span>
      </span>
    ),
    enableSorting: false,
  }),
  columnHelper.display({
    id: "classification",
    header: "Klasifikasi",
    cell: (info) => {
      const r = info.row.original;
      const c = CLASS_LABEL[r.classification];
      const off = r.workday_offset ? ` (${Math.abs(r.workday_offset)} hari kerja)` : "";
      return <Badge variant={c.variant}>{c.label + off}</Badge>;
    },
  }),
  columnHelper.accessor("realized_date", {
    header: "Realisasi ITM",
    cell: (info) => formatDate(info.getValue()),
    enableSorting: false,
  }),
  columnHelper.display({
    id: "order",
    header: "Order",
    cell: (info) => formatIDR(total(info.row.original, "order_amount")),
    meta: { numeric: true },
  }),
  columnHelper.display({
    id: "itm",
    header: "ITM",
    cell: (info) => formatIDR(total(info.row.original, "itm_amount")),
    meta: { numeric: true },
  }),
  columnHelper.display({
    id: "diff",
    header: "Selisih",
    cell: (info) => {
      const d = total(info.row.original, "diff");
      return <span className={d !== 0 ? "font-semibold text-danger-fg" : ""}>{formatIDR(d)}</span>;
    },
    meta: { numeric: true },
  }),
  columnHelper.display({
    id: "flags",
    header: "Catatan",
    cell: (info) => {
      const r = info.row.original;
      return (
        <span className="flex flex-wrap gap-1">
          {r.vendor_mismatch && <Badge variant="warning">Beda dengan laporan Anda</Badge>}
          {r.vendor_report_late && <Badge variant="danger">Laporan telat</Badge>}
        </span>
      );
    },
  }),
] as ColumnDef<VendorClassification, unknown>[];

// Req 8.5: a vault-only account has no replenish parties, so the list is closed to it
// (the API answers 403). Gate before RealizationsPage so its query never fires.
export function RealizationsGate() {
  const { state } = useAuth();
  if (state.user?.role === "VENDOR-VAULT-USER") {
    return (
      <div className="flex items-center gap-3 rounded-md border border-neutral-300 p-4 text-surface-text">
        <Lock className="size-5 shrink-0" aria-hidden="true" />
        <p className="text-sm font-medium">Tidak tersedia untuk peran Anda</p>
      </div>
    );
  }
  return <RealizationsPage />;
}

export function RealizationsPage() {
  const today = new Date().toLocaleDateString("sv-SE", { timeZone: "Asia/Jakarta" });
  const [from, setFrom] = useState(shiftDays(today, -6));
  const [to, setTo] = useState(today);
  const [classification, setClassification] = useState<ClassFilter>("all");
  const [page, setPage] = useState(1);
  const problem = rangeProblem(from, to);
  const { data, isLoading, isError } = useRealizations(
    { from, to, classification: classification === "all" ? "" : classification, page },
    problem === null,
  );
  const items = data?.items ?? [];
  const lastPage = Math.max(1, Math.ceil((data?.total ?? 0) / ORDER_PAGE_SIZE));

  function reset<T>(set: (v: T) => void) {
    return (v: T) => {
      set(v);
      setPage(1);
    };
  }

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-xl font-semibold text-surface-text">Realisasi</h1>
        <p className="text-sm text-neutral-600">
          Hasil pengisian ATM Anda menurut data mesin (ITM), dibandingkan dengan jadwal dan nominal
          order.
        </p>
      </div>
      <div className="flex flex-col gap-4">
        <FilterTabs
          options={FILTERS}
          selected={classification}
          onChange={reset(setClassification)}
        />
        <DatePicker
          startDate={from}
          endDate={to}
          onStartDateChange={reset((v: string | null) => setFrom(v ?? shiftDays(today, -6)))}
          onEndDateChange={reset((v: string | null) => setTo(v ?? today))}
        />
        {problem && (
          <p role="alert" className="text-sm font-semibold text-danger-fg">
            {problem}
          </p>
        )}
      </div>
      {problem ? null : isLoading ? (
        <p className="text-sm text-neutral-500">Memuat data...</p>
      ) : isError ? (
        <p role="alert" className="text-sm font-semibold text-danger-fg">
          Gagal memuat realisasi. Coba muat ulang halaman.
        </p>
      ) : items.length === 0 ? (
        <EmptyState
          icon={ClipboardCheck}
          title="Belum ada realisasi"
          description="Klasifikasi muncul setelah data ITM pengisian ATM diproses."
        />
      ) : (
        <>
          <DataTable data={items} columns={columns} />
          <nav aria-label="Halaman" className="flex items-center justify-end gap-3 text-sm">
            <Button
              variant="secondary"
              size="sm"
              disabled={page <= 1}
              onClick={() => setPage(page - 1)}
            >
              Sebelumnya
            </Button>
            <span className="tabular-nums">
              Halaman {page} dari {lastPage}
            </span>
            <Button
              variant="secondary"
              size="sm"
              disabled={page >= lastPage}
              onClick={() => setPage(page + 1)}
            >
              Berikutnya
            </Button>
          </nav>
        </>
      )}
    </div>
  );
}
