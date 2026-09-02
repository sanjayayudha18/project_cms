import {
  type ColumnDef,
  type RowData,
  type SortingState,
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  useReactTable,
} from "@tanstack/react-table";
import { ChevronDown, ChevronUp } from "lucide-react";
import { useState } from "react";

/**
 * Extend TanStack Table's column meta to support numeric column styling.
 * Columns with `meta: { numeric: true }` get right-aligned text and tabular-nums.
 */
declare module "@tanstack/react-table" {
  interface ColumnMeta<TData extends RowData, TValue> {
    numeric?: boolean;
  }
}

interface DataTableProps<T> {
  readonly data: readonly T[];
  readonly columns: ColumnDef<T, unknown>[];
  readonly sorting?: SortingState;
  readonly onSortingChange?: (sorting: SortingState) => void;
  readonly emptyMessage?: string;
}

/**
 * Generic data table component wrapping TanStack Table v8.
 * Supports controlled or uncontrolled sorting, numeric column alignment,
 * and responsive horizontal scrolling.
 */
export function DataTable<T>({
  data,
  columns,
  sorting: controlledSorting,
  onSortingChange,
  emptyMessage,
}: DataTableProps<T>) {
  const [internalSorting, setInternalSorting] = useState<SortingState>([]);

  const isControlled = controlledSorting !== undefined && onSortingChange !== undefined;
  const sorting = isControlled ? controlledSorting : internalSorting;

  const table = useReactTable({
    data: data as T[],
    columns,
    state: { sorting },
    onSortingChange: (updater) => {
      const next = typeof updater === "function" ? updater(sorting) : updater;
      if (isControlled) {
        onSortingChange(next);
      } else {
        setInternalSorting(next);
      }
    },
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  });

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          {table.getHeaderGroups().map((headerGroup) => (
            <tr
              key={headerGroup.id}
              className="bg-neutral-50 text-neutral-500 uppercase text-xs tracking-wider"
            >
              {headerGroup.headers.map((header) => {
                const isNumeric = header.column.columnDef.meta?.numeric;
                const canSort = header.column.getCanSort();

                return (
                  <th
                    key={header.id}
                    className={`px-4 py-3 font-medium select-none ${
                      isNumeric ? "text-right" : "text-left"
                    } ${canSort ? "cursor-pointer" : ""}`}
                    onClick={canSort ? header.column.getToggleSortingHandler() : undefined}
                    aria-sort={
                      header.column.getIsSorted() === "asc"
                        ? "ascending"
                        : header.column.getIsSorted() === "desc"
                          ? "descending"
                          : "none"
                    }
                  >
                    <span className="inline-flex items-center gap-1">
                      {header.isPlaceholder
                        ? null
                        : flexRender(header.column.columnDef.header, header.getContext())}
                      {canSort && header.column.getIsSorted() === "asc" && (
                        <ChevronUp className="size-3.5" aria-hidden="true" />
                      )}
                      {canSort && header.column.getIsSorted() === "desc" && (
                        <ChevronDown className="size-3.5" aria-hidden="true" />
                      )}
                    </span>
                  </th>
                );
              })}
            </tr>
          ))}
        </thead>
        <tbody>
          {table.getRowModel().rows.length === 0 && emptyMessage ? (
            <tr>
              <td colSpan={columns.length} className="px-4 py-8 text-center text-neutral-400">
                {emptyMessage}
              </td>
            </tr>
          ) : (
            table.getRowModel().rows.map((row) => (
              <tr key={row.id} className="border-b border-neutral-100 hover:bg-red-50/30">
                {row.getVisibleCells().map((cell) => {
                  const isNumeric = cell.column.columnDef.meta?.numeric;

                  return (
                    <td
                      key={cell.id}
                      className={`px-4 py-3 ${isNumeric ? "text-right tabular-nums" : ""}`}
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
