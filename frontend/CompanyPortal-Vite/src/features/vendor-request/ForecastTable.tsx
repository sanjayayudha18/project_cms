/**
 * Forecast Browser table (Req 11.2-11.4, 11.8-11.9, 11.11): checkbox
 * selection + "Select All" (current page), sortable on ATM ID,
 * Denomination, and Amount Replenish (default sort: ATM ID ascending).
 * Uses real TanStack Table (useReactTable) for sorting + row-selection
 * state, unlike the read-only DmaaForecastTable — the shared DataTable
 * component doesn't support row selection, so this page owns its own
 * table markup (same loading/error/empty-row shape as DmaaForecastTable).
 */

import { Skeleton } from "@/components/feedback/Skeleton";
import { Button } from "@/components/ui/Button";
import { formatIDR } from "@/lib/utils/formatCurrency";
import {
  type ColumnDef,
  type OnChangeFn,
  type RowSelectionState,
  type SortingState,
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  useReactTable,
} from "@tanstack/react-table";
import { AlertCircle, ArrowDown, ArrowUp } from "lucide-react";
import type { ForecastRow } from "./types";

const SKELETON_ROW_COUNT = 5;

export function forecastRowId(row: ForecastRow): string {
  return `${row.terminal_id}|${row.periode_pred}|${row.denom}`;
}

const columns: ColumnDef<ForecastRow>[] = [
  {
    id: "select",
    header: ({ table }) => (
      <input
        type="checkbox"
        aria-label="Pilih semua baris"
        checked={table.getIsAllRowsSelected()}
        ref={(el) => {
          if (el) el.indeterminate = table.getIsSomeRowsSelected() && !table.getIsAllRowsSelected();
        }}
        onChange={table.getToggleAllRowsSelectedHandler()}
        className="h-4 w-4 cursor-pointer accent-[var(--red-500)]"
      />
    ),
    cell: ({ row }) => (
      <input
        type="checkbox"
        aria-label={`Pilih baris ${row.original.terminal_id}`}
        checked={row.getIsSelected()}
        onChange={row.getToggleSelectedHandler()}
        className="h-4 w-4 cursor-pointer accent-[var(--red-500)]"
      />
    ),
    enableSorting: false,
  },
  {
    accessorKey: "terminal_id",
    header: "ATM ID",
  },
  {
    accessorKey: "denom",
    header: "Denominasi",
    cell: ({ getValue }) => formatIDR(getValue<number>()),
    meta: { align: "right" },
  },
  {
    accessorKey: "amount_replenish",
    header: "Amount Replenish",
    cell: ({ getValue }) => formatIDR(getValue<number>()),
    meta: { align: "right" },
  },
  {
    accessorKey: "amount_refund",
    header: "Amount Refund",
    cell: ({ getValue }) => formatIDR(getValue<number>()),
    meta: { align: "right" },
    enableSorting: false,
  },
];

interface ForecastTableProps {
  data: ForecastRow[];
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  sorting: SortingState;
  onSortingChange: OnChangeFn<SortingState>;
  rowSelection: RowSelectionState;
  onRowSelectionChange: OnChangeFn<RowSelectionState>;
}

export function ForecastTable({
  data,
  isLoading,
  isError,
  onRetry,
  sorting,
  onSortingChange,
  rowSelection,
  onRowSelectionChange,
}: ForecastTableProps) {
  const table = useReactTable({
    data,
    columns,
    getRowId: forecastRowId,
    state: { sorting, rowSelection },
    onSortingChange,
    onRowSelectionChange,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    enableRowSelection: true,
  });

  return (
    <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
      <table
        aria-label="Data Forecast DMAA"
        className="w-full min-w-[720px] border-collapse text-sm"
      >
        <thead>
          {table.getHeaderGroups().map((headerGroup) => (
            <tr key={headerGroup.id} className="border-[var(--n-200)] border-b bg-[var(--n-50)]">
              {headerGroup.headers.map((header) => {
                const align = (header.column.columnDef.meta as { align?: string } | undefined)
                  ?.align;
                const canSort = header.column.getCanSort();
                const isSorted = header.column.getIsSorted();
                return (
                  <th
                    key={header.id}
                    scope="col"
                    aria-sort={
                      isSorted ? (isSorted === "asc" ? "ascending" : "descending") : "none"
                    }
                    className={`px-3 py-2 font-medium text-[var(--n-600)] ${align === "right" ? "text-right" : "text-left"}`}
                  >
                    {header.isPlaceholder ? null : canSort ? (
                      <button
                        type="button"
                        onClick={header.column.getToggleSortingHandler()}
                        className={`inline-flex min-h-[44px] items-center gap-1 font-medium outline-none focus-visible:ring-2 focus-visible:ring-[var(--red-100)] ${align === "right" ? "flex-row-reverse" : ""}`}
                      >
                        {flexRender(header.column.columnDef.header, header.getContext())}
                        {isSorted === "asc" && (
                          <ArrowUp className="h-3.5 w-3.5" aria-hidden="true" />
                        )}
                        {isSorted === "desc" && (
                          <ArrowDown className="h-3.5 w-3.5" aria-hidden="true" />
                        )}
                      </button>
                    ) : (
                      flexRender(header.column.columnDef.header, header.getContext())
                    )}
                  </th>
                );
              })}
            </tr>
          ))}
        </thead>
        <tbody>
          {isLoading && <SkeletonRows columnCount={columns.length} />}
          {!isLoading && isError && <ErrorRow columnCount={columns.length} onRetry={onRetry} />}
          {!isLoading && !isError && data.length === 0 && <EmptyRow columnCount={columns.length} />}
          {!isLoading &&
            !isError &&
            table.getRowModel().rows.map((row) => (
              <tr
                key={row.id}
                className={`border-[var(--n-100)] border-b last:border-0 ${row.getIsSelected() ? "bg-[var(--red-50)]" : ""}`}
              >
                {row.getVisibleCells().map((cell) => {
                  const align = (cell.column.columnDef.meta as { align?: string } | undefined)
                    ?.align;
                  return (
                    <td
                      key={cell.id}
                      className={`px-3 py-2 ${align === "right" ? "text-right tabular-nums" : ""}`}
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
  );
}

function SkeletonRows({ columnCount }: { columnCount: number }) {
  return (
    <>
      {Array.from({ length: SKELETON_ROW_COUNT }).map((_, rowIndex) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: generic loading placeholder rows have no stable identity
        <tr key={`skeleton-row-${rowIndex}`}>
          {Array.from({ length: columnCount }).map((_, colIndex) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: generic loading placeholder cells have no stable identity
            <td key={`skeleton-cell-${rowIndex}-${colIndex}`} className="px-3 py-3">
              <Skeleton height={14} className="w-full" />
            </td>
          ))}
        </tr>
      ))}
    </>
  );
}

function ErrorRow({ columnCount, onRetry }: { columnCount: number; onRetry: () => void }) {
  return (
    <tr>
      <td colSpan={columnCount} className="px-3 py-12 text-center">
        <div className="flex flex-col items-center gap-3">
          <AlertCircle className="h-10 w-10 text-[var(--danger-fg)]" aria-hidden="true" />
          <p className="text-sm text-[var(--n-700)]">Gagal memuat data forecast</p>
          <Button variant="secondary" onClick={onRetry}>
            Coba Lagi
          </Button>
        </div>
      </td>
    </tr>
  );
}

function EmptyRow({ columnCount }: { columnCount: number }) {
  return (
    <tr>
      <td colSpan={columnCount} className="px-3 py-12 text-center">
        <p className="text-sm text-[var(--n-700)]">
          Tidak ada data forecast untuk tanggal yang dipilih
        </p>
      </td>
    </tr>
  );
}
