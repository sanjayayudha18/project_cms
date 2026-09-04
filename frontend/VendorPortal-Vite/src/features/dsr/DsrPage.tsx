import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import { EmptyState } from "@/components/ui/EmptyState";
import { DsrDetailDialog } from "@/features/dsr/DsrDetailDialog";
import { DsrUploadDialog } from "@/features/dsr/DsrUploadDialog";
import type { DsrUploadListItem, DsrUploadSheetSummary } from "@/features/dsr/dsrUploadApi";
import { useDsrUploads } from "@/features/dsr/useDsrUploads";
import { useQueryClient } from "@tanstack/react-query";
import { type ColumnDef, type SortingState, createColumnHelper } from "@tanstack/react-table";
import { DatabaseZap, Plus } from "lucide-react";
import { useState } from "react";

const statusBadgeMap: Record<string, "danger" | "warning" | "success" | "neutral"> = {
  completed: "success",
  processing: "warning",
  pending: "warning",
  failed: "danger",
  skipped: "neutral",
};

function StatusBadge({ sheet }: { readonly sheet: DsrUploadSheetSummary | null }) {
  if (!sheet) return <span className="text-neutral-400">-</span>;
  return <Badge variant={statusBadgeMap[sheet.status] ?? "neutral"}>{sheet.status}</Badge>;
}

const columnHelper = createColumnHelper<DsrUploadListItem>();

const columns: ColumnDef<DsrUploadListItem, unknown>[] = [
  columnHelper.accessor("report_date", {
    header: "Tanggal",
    cell: (info) => info.getValue(),
  }),
  columnHelper.display({
    id: "daily",
    header: "Saldo Harian (Daily)",
    cell: ({ row }) => <StatusBadge sheet={row.original.daily} />,
    enableSorting: false,
  }),
  columnHelper.display({
    id: "rencana_isi",
    header: "Rencana Isi",
    cell: ({ row }) => <StatusBadge sheet={row.original.rencana_isi} />,
    enableSorting: false,
  }),
] as ColumnDef<DsrUploadListItem, unknown>[];

export function DsrPage() {
  const [selectedDate, setSelectedDate] = useState<string | undefined>("2024-01-15");
  const [sorting, setSorting] = useState<SortingState>([{ id: "report_date", desc: true }]);
  const [isUploadOpen, setIsUploadOpen] = useState(false);
  const [detailItem, setDetailItem] = useState<DsrUploadListItem | null>(null);

  const queryClient = useQueryClient();
  const { data, isLoading } = useDsrUploads(selectedDate);
  const records = data?.data ?? [];

  if (isLoading) {
    return (
      <div className="flex flex-col gap-6">
        <div className="flex items-center justify-between">
          <h1 className="text-xl font-semibold text-surface-text">DSR Monitor</h1>
          <Button size="sm" onClick={() => setIsUploadOpen(true)}>
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
        <Button size="sm" onClick={() => setIsUploadOpen(true)}>
          <Plus className="size-4" aria-hidden="true" />
          Add DSR
        </Button>
      </div>

      {/* Date selector */}
      <div className="flex flex-col gap-2">
        <label
          htmlFor="dsr-date"
          className="text-xs font-medium uppercase tracking-wide text-neutral-500"
        >
          Tanggal
        </label>
        <div className="flex gap-2">
          <input
            id="dsr-date"
            type="date"
            value={selectedDate ?? ""}
            onChange={(e) => setSelectedDate(e.target.value || undefined)}
            className="min-h-[44px] min-w-[44px] w-fit rounded-md border border-neutral-300 bg-white px-3 py-2 text-sm
                       text-surface-text
                       focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-sidebar-active
                       focus-visible:border-sidebar-active"
          />
          <Button
            size="sm"
            variant="secondary"
            onClick={() => setSelectedDate(undefined)}
            className="min-w-fit"
          >
            Tampilkan Semua
          </Button>
        </div>
      </div>

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
          onRowClick={setDetailItem}
        />
      )}

      {isUploadOpen && (
        <DsrUploadDialog
          onClose={() => setIsUploadOpen(false)}
          onConfirmed={() => {
            setIsUploadOpen(false);
            void queryClient.invalidateQueries({ queryKey: ["dsr-uploads"] });
          }}
        />
      )}

      {detailItem && <DsrDetailDialog item={detailItem} onClose={() => setDetailItem(null)} />}
    </div>
  );
}
