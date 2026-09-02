import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import { EmptyState } from "@/components/ui/EmptyState";
import { DsrSummaryCard } from "@/features/dsr/DsrSummaryCard";
import { useDsr } from "@/features/dsr/useDsr";
import { formatIDR, getBalanceStatus } from "@/lib/formatters";
import type { BalanceStatus, DsrRecord } from "@/lib/types";
import { type ColumnDef, type SortingState, createColumnHelper } from "@tanstack/react-table";
import { DatabaseZap, Plus } from "lucide-react";
import { useState } from "react";

const balanceStatusBadgeMap: Record<BalanceStatus, "danger" | "warning" | "success"> = {
  Critical: "danger",
  Low: "warning",
  Normal: "success",
};

const columnHelper = createColumnHelper<DsrRecord>();

const columns: ColumnDef<DsrRecord, unknown>[] = [
  columnHelper.accessor("atmId", {
    header: "ATM ID",
    cell: (info) => info.getValue(),
  }),
  columnHelper.accessor("location", {
    header: "Location",
    cell: (info) => info.getValue(),
  }),
  columnHelper.accessor("date", {
    header: "Date",
    cell: (info) => info.getValue(),
  }),
  columnHelper.accessor("beginningBalance", {
    header: "Beginning Balance",
    cell: (info) => formatIDR(info.getValue()),
    meta: { numeric: true },
  }),
  columnHelper.accessor("cashIn", {
    header: "Cash In",
    cell: (info) => formatIDR(info.getValue()),
    meta: { numeric: true },
  }),
  columnHelper.accessor("cashOut", {
    header: "Cash Out",
    cell: (info) => formatIDR(info.getValue()),
    meta: { numeric: true },
  }),
  columnHelper.accessor("endingBalance", {
    header: "Ending Balance",
    cell: (info) => formatIDR(info.getValue()),
    meta: { numeric: true },
  }),
  columnHelper.display({
    id: "balanceStatus",
    header: "Balance Status",
    cell: ({ row }) => {
      const status = getBalanceStatus(row.original.endingBalance);
      return <Badge variant={balanceStatusBadgeMap[status]}>{status}</Badge>;
    },
    enableSorting: false,
  }),
] as ColumnDef<DsrRecord, unknown>[];

export function DsrPage() {
  const [selectedDate, setSelectedDate] = useState("2024-01-15");
  const [sorting, setSorting] = useState<SortingState>([{ id: "atmId", desc: false }]);

  const { data: records = [], isLoading } = useDsr(selectedDate);

  if (isLoading) {
    return (
      <div className="flex flex-col gap-6">
        <div className="flex items-center justify-between">
          <h1 className="text-xl font-semibold text-surface-text">DSR Monitor</h1>
          <Button size="sm">
            <Plus className="size-4" aria-hidden="true" />
            Add DSR
          </Button>
        </div>
        <p className="text-sm text-neutral-500">Memuat data...</p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold text-surface-text">DSR Monitor</h1>
        <Button size="sm">
          <Plus className="size-4" aria-hidden="true" />
          Add DSR
        </Button>
      </div>

      {/* Date selector */}
      <div className="flex flex-col gap-1">
        <label
          htmlFor="dsr-date"
          className="text-xs font-medium uppercase tracking-wide text-neutral-500"
        >
          Tanggal
        </label>
        <input
          id="dsr-date"
          type="date"
          value={selectedDate}
          onChange={(e) => setSelectedDate(e.target.value)}
          className="min-h-[44px] min-w-[44px] w-fit rounded-md border border-neutral-300 bg-white px-3 py-2 text-sm
                     text-surface-text
                     focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-sidebar-active
                     focus-visible:border-sidebar-active"
        />
      </div>

      {/* Summary card */}
      <DsrSummaryCard records={records} />

      {/* Data table or empty state */}
      {records.length === 0 ? (
        <EmptyState
          icon={DatabaseZap}
          title="Tidak ada data DSR"
          description="Tidak ditemukan data DSR untuk tanggal yang dipilih."
        />
      ) : (
        <DataTable
          data={records}
          columns={columns}
          sorting={sorting}
          onSortingChange={setSorting}
        />
      )}
    </div>
  );
}
