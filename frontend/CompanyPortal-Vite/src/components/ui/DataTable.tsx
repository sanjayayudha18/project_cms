import {
  type ColumnDef,
  type SortingState,
  flexRender,
  getCoreRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
} from "@tanstack/react-table";
import { ArrowDown, ArrowUp } from "lucide-react";
import { useState } from "react";
import { Button } from "./Button";
import { PageNumbers } from "./PageNumbers";

interface DataTableProps<T> {
  data: T[];
  // biome-ignore lint/suspicious/noExplicitAny: TanStack Table requires any for column defs
  columns: ColumnDef<T, any>[];
  defaultSorting?: SortingState;
  emptyMessage?: string;
  /** Optional row click handler (mouse convenience; put a real button in a cell for keyboard access). */
  onRowClick?: (row: T) => void;
  /** Client-side pagination: rows per page. Omit to render every row. */
  pageSize?: number;
}

export function DataTable<T>({
  data,
  columns,
  defaultSorting = [],
  emptyMessage = "Tidak ada data tersedia",
  onRowClick,
  pageSize,
}: DataTableProps<T>) {
  const [sorting, setSorting] = useState<SortingState>(defaultSorting);

  const table = useReactTable({
    data,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    ...(pageSize
      ? {
          getPaginationRowModel: getPaginationRowModel(),
          initialState: { pagination: { pageIndex: 0, pageSize } },
        }
      : {}),
  });
  const pageCount = table.getPageCount();
  const showPager = pageSize !== undefined && data.length > pageSize;

  return (
    <div className="flex flex-col gap-3">
      <div className="overflow-x-auto rounded-lg border border-[var(--n-200)]">
        <table className="w-full border-collapse text-sm">
          <thead>
            {table.getHeaderGroups().map((headerGroup) => (
              <tr key={headerGroup.id}>
                {headerGroup.headers.map((header) => {
                  const align = (header.column.columnDef.meta as { align?: string } | undefined)
                    ?.align;
                  const canSort = header.column.getCanSort();

                  const sortHandler = header.column.getToggleSortingHandler();

                  return (
                    <th
                      key={header.id}
                      className={[
                        "px-4 py-3 text-xs font-medium uppercase tracking-wider",
                        "text-[var(--n-500)] bg-[var(--n-50)]",
                        "border-b border-[var(--n-100)]",
                        canSort ? "cursor-pointer select-none" : "",
                        align === "right" ? "text-right" : "text-left",
                      ].join(" ")}
                      onClick={sortHandler}
                      onKeyDown={
                        canSort
                          ? (e) => {
                              if (e.key === "Enter" || e.key === " ") {
                                e.preventDefault();
                                sortHandler?.(e);
                              }
                            }
                          : undefined
                      }
                      tabIndex={canSort ? 0 : undefined}
                      role={canSort ? "button" : undefined}
                    >
                      <span className="inline-flex items-center gap-1">
                        {header.isPlaceholder
                          ? null
                          : flexRender(header.column.columnDef.header, header.getContext())}
                        {header.column.getIsSorted() === "asc" && (
                          <ArrowUp className="h-3.5 w-3.5" aria-label="Urutan naik" />
                        )}
                        {header.column.getIsSorted() === "desc" && (
                          <ArrowDown className="h-3.5 w-3.5" aria-label="Urutan turun" />
                        )}
                      </span>
                    </th>
                  );
                })}
              </tr>
            ))}
          </thead>
          <tbody>
            {table.getRowModel().rows.length === 0 ? (
              <tr>
                <td colSpan={columns.length} className="px-4 py-12 text-center text-[var(--n-500)]">
                  {emptyMessage}
                </td>
              </tr>
            ) : (
              table.getRowModel().rows.map((row) => (
                // biome-ignore lint/a11y/useKeyWithClickEvents: onRowClick is mouse-only sugar; callers give keyboard users a real button in a cell (see DataTableProps.onRowClick).
                <tr
                  key={row.id}
                  onClick={onRowClick ? () => onRowClick(row.original) : undefined}
                  className={[
                    "border-b border-[var(--n-100)] transition-colors duration-100 hover:bg-[var(--red-50)]",
                    onRowClick ? "cursor-pointer" : "",
                  ].join(" ")}
                >
                  {row.getVisibleCells().map((cell) => {
                    const align = (cell.column.columnDef.meta as { align?: string } | undefined)
                      ?.align;

                    return (
                      <td
                        key={cell.id}
                        className={[
                          "px-4 py-3",
                          align === "right" ? "text-right tabular-nums" : "",
                        ].join(" ")}
                      >
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </td>
                    );
                  })}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
      {showPager && (
        <div className="flex flex-wrap items-center justify-between gap-4 text-sm text-[var(--n-600)]">
          <span>
            Halaman {table.getState().pagination.pageIndex + 1} dari {pageCount} ({data.length}{" "}
            baris)
          </span>
          <div className="flex items-center gap-2">
            <Button
              variant="secondary"
              disabled={!table.getCanPreviousPage()}
              onClick={() => table.previousPage()}
            >
              Sebelumnya
            </Button>
            <PageNumbers
              current={table.getState().pagination.pageIndex + 1}
              total={pageCount}
              onChange={(n) => table.setPageIndex(n - 1)}
            />
            <Button
              variant="secondary"
              disabled={!table.getCanNextPage()}
              onClick={() => table.nextPage()}
            >
              Berikutnya
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
