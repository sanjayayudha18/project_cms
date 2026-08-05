import { useState } from "react";
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  flexRender,
  type ColumnDef,
  type SortingState,
} from "@tanstack/react-table";
import { ArrowUp, ArrowDown } from "lucide-react";

interface DataTableProps<T> {
  data: T[];
  // biome-ignore lint/suspicious/noExplicitAny: TanStack Table requires any for column defs
  columns: ColumnDef<T, any>[];
  defaultSorting?: SortingState;
  emptyMessage?: string;
}

export function DataTable<T>({
  data,
  columns,
  defaultSorting = [],
  emptyMessage = "Tidak ada data tersedia",
}: DataTableProps<T>) {
  const [sorting, setSorting] = useState<SortingState>(defaultSorting);

  const table = useReactTable({
    data,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  });

  return (
    <div className="overflow-x-auto rounded-lg border border-[var(--n-200)]">
      <table className="w-full border-collapse text-sm">
        <thead>
          {table.getHeaderGroups().map((headerGroup) => (
            <tr key={headerGroup.id}>
              {headerGroup.headers.map((header) => {
                const align =
                  (header.column.columnDef.meta as { align?: string } | undefined)?.align;
                const canSort = header.column.getCanSort();

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
                    onClick={header.column.getToggleSortingHandler()}
                  >
                    <span className="inline-flex items-center gap-1">
                      {header.isPlaceholder
                        ? null
                        : flexRender(
                            header.column.columnDef.header,
                            header.getContext(),
                          )}
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
              <td
                colSpan={columns.length}
                className="px-4 py-12 text-center text-[var(--n-500)]"
              >
                {emptyMessage}
              </td>
            </tr>
          ) : (
            table.getRowModel().rows.map((row) => (
              <tr
                key={row.id}
                className="border-b border-[var(--n-100)] transition-colors duration-100 hover:bg-[var(--red-50)]"
              >
                {row.getVisibleCells().map((cell) => {
                  const align =
                    (cell.column.columnDef.meta as { align?: string } | undefined)?.align;

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
  );
}
