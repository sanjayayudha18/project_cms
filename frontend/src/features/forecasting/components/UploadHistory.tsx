import {
  type ColumnDef,
  type SortingState,
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  useReactTable,
} from "@tanstack/react-table";
import { ChevronDown, ChevronUp, RefreshCw } from "lucide-react";
import { useMemo, useState } from "react";
import { useDSRHistory } from "../hooks/useDSRHistory";
import type { DSRUploadRecord } from "../types";

// ─── Status Badge ─────────────────────────────────────────────────────────────

const STATUS_CONFIG = {
  accepted: {
    label: "Diterima",
    bg: "var(--success-bg)",
    fg: "var(--success-fg)",
  },
  rejected: {
    label: "Ditolak",
    bg: "var(--danger-bg)",
    fg: "var(--danger-fg)",
  },
  processing: {
    label: "Diproses",
    bg: "var(--warning-bg)",
    fg: "var(--warning-fg)",
  },
} as const;

function StatusBadge({ status }: { status: DSRUploadRecord["status"] }) {
  const config = STATUS_CONFIG[status];
  return (
    <span
      className="inline-flex items-center rounded-full px-[var(--space-2)] py-0.5 text-xs font-medium"
      style={{ backgroundColor: config.bg, color: config.fg }}
    >
      {config.label}
    </span>
  );
}

// ─── Sort Indicator ───────────────────────────────────────────────────────────

function SortIndicator({ direction }: { direction: "asc" | "desc" | false }) {
  if (!direction) return null;
  const Icon = direction === "asc" ? ChevronUp : ChevronDown;
  return <Icon size={14} aria-hidden="true" className="inline-block ml-1" />;
}

// ─── Column Definitions ───────────────────────────────────────────────────────

function formatDate(dateStr: string): string {
  const date = new Date(dateStr);
  return date.toLocaleDateString("id-ID", {
    day: "numeric",
    month: "short",
    year: "numeric",
  });
}

const columns: ColumnDef<DSRUploadRecord>[] = [
  {
    accessorKey: "date",
    header: "Tanggal",
    cell: ({ getValue }) => formatDate(getValue<string>()),
  },
  {
    accessorKey: "filename",
    header: "Nama File",
  },
  {
    accessorKey: "rowCount",
    header: "Jumlah Baris",
    cell: ({ getValue }) => getValue<number>().toLocaleString("id-ID"),
    meta: { align: "right" },
  },
  {
    accessorKey: "status",
    header: "Status",
    cell: ({ getValue }) => <StatusBadge status={getValue<DSRUploadRecord["status"]>()} />,
    enableSorting: true,
  },
];

// ─── Loading Skeleton ─────────────────────────────────────────────────────────

function TableSkeleton() {
  return (
    <div className="flex flex-col gap-[var(--space-2)]">
      {Array.from({ length: 5 }).map((_, i) => (
        <div
          key={`skeleton-${i}`}
          className="h-10 w-full animate-pulse rounded-[var(--radius-sm)]"
          style={{ backgroundColor: "var(--n-100)" }}
        />
      ))}
    </div>
  );
}

// ─── Upload History Component ─────────────────────────────────────────────────

export function UploadHistory() {
  const { data, isLoading, isError, refetch } = useDSRHistory();

  const [sorting, setSorting] = useState<SortingState>([{ id: "date", desc: true }]);

  const tableData = useMemo(() => {
    if (!data) return [];
    return data.slice(0, 30);
  }, [data]);

  const table = useReactTable({
    data: tableData,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  });

  return (
    <div className="flex flex-col gap-[var(--space-4)]">
      <h2 className="text-base font-semibold" style={{ color: "var(--n-900)" }}>
        Riwayat Unggahan
      </h2>

      {/* Loading State */}
      {isLoading && <TableSkeleton />}

      {/* Error State */}
      {isError && (
        <div
          className="flex items-center justify-between rounded-[var(--radius-md)] p-[var(--space-4)]"
          style={{ backgroundColor: "var(--danger-bg)" }}
          role="alert"
        >
          <p className="text-sm" style={{ color: "var(--danger-fg)" }}>
            Gagal memuat riwayat unggahan.
          </p>
          <button
            type="button"
            onClick={() => refetch()}
            className="inline-flex items-center gap-[var(--space-2)] rounded-[var(--radius-md)] px-[var(--space-3)] py-[var(--space-2)] text-sm font-medium transition-colors duration-150"
            style={{
              backgroundColor: "var(--n-0)",
              color: "var(--danger-fg)",
              border: "1px solid var(--danger-fg)",
            }}
          >
            <RefreshCw size={14} aria-hidden="true" />
            Coba Lagi
          </button>
        </div>
      )}

      {/* Table */}
      {!isLoading && !isError && tableData.length > 0 && (
        <div
          className="overflow-x-auto rounded-[var(--radius-lg)] border"
          style={{ borderColor: "var(--n-200)" }}
        >
          <table className="w-full text-left text-sm">
            <thead>
              {table.getHeaderGroups().map((headerGroup) => (
                <tr key={headerGroup.id} style={{ backgroundColor: "var(--n-50)" }}>
                  {headerGroup.headers.map((header) => {
                    const align =
                      (header.column.columnDef.meta as { align?: string } | undefined)?.align ===
                      "right"
                        ? "text-right"
                        : "text-left";
                    return (
                      <th
                        key={header.id}
                        className={`px-[var(--space-3)] py-[var(--space-2)] text-xs font-medium uppercase tracking-wider select-none ${align}`}
                        style={{
                          color: "var(--n-500)",
                          cursor: header.column.getCanSort() ? "pointer" : "default",
                        }}
                        onClick={header.column.getToggleSortingHandler()}
                        aria-sort={
                          header.column.getIsSorted() === "asc"
                            ? "ascending"
                            : header.column.getIsSorted() === "desc"
                              ? "descending"
                              : "none"
                        }
                      >
                        {flexRender(header.column.columnDef.header, header.getContext())}
                        <SortIndicator direction={header.column.getIsSorted()} />
                      </th>
                    );
                  })}
                </tr>
              ))}
            </thead>
            <tbody>
              {table.getRowModel().rows.map((row) => (
                <tr
                  key={row.id}
                  className="border-t transition-colors duration-100"
                  style={{ borderColor: "var(--n-100)" }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.backgroundColor = "var(--red-50)";
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.backgroundColor = "";
                  }}
                >
                  {row.getVisibleCells().map((cell) => {
                    const align =
                      (cell.column.columnDef.meta as { align?: string } | undefined)?.align ===
                      "right"
                        ? "text-right"
                        : "text-left";
                    const isNumeric =
                      (cell.column.columnDef.meta as { align?: string } | undefined)?.align ===
                      "right";
                    return (
                      <td
                        key={cell.id}
                        className={`px-[var(--space-3)] py-[var(--space-2)] ${align} ${isNumeric ? "tabular-nums" : ""}`}
                        style={{ color: "var(--n-800)" }}
                      >
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Empty State */}
      {!isLoading && !isError && tableData.length === 0 && (
        <p className="text-sm" style={{ color: "var(--n-500)" }}>
          Belum ada riwayat unggahan.
        </p>
      )}
    </div>
  );
}
