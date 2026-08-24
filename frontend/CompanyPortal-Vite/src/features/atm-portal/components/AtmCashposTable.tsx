/**
 * ATM Cashpos table: all 23 itm_cashpos fields, semantic markup, sortable
 * headers for backend-supported columns, decimal-string money formatting.
 */

import { Skeleton } from "@/components/feedback/Skeleton";
import { Button } from "@/components/ui/Button";
import { AlertCircle, ArrowDown, ArrowUp } from "lucide-react";
import { formatAtmDate, formatAtmDateTime, formatRupiahDecimal } from "../lib/formatters";
import type { AtmCashposRecord } from "../types";

const SKELETON_ROW_COUNT = 5;

interface Column {
  readonly key: string;
  readonly label: string;
  readonly sortable: boolean;
  readonly align?: "right";
  readonly mono?: boolean;
}

const COLUMNS: readonly Column[] = [
  { key: "id", label: "ID", sortable: true },
  { key: "file_id", label: "File ID", sortable: false },
  { key: "created_at", label: "Created At", sortable: true },
  { key: "cashpos_date", label: "Cashpos Date", sortable: true },
  { key: "terminal_id", label: "Terminal ID", sortable: true, mono: true },
  { key: "machine_type", label: "Machine Type", sortable: true },
  { key: "teller_id", label: "Teller ID", sortable: false },
  { key: "branch_code", label: "Branch Code", sortable: true },
  { key: "position_source", label: "Position Source", sortable: false },
  { key: "starting_cash_10k", label: "Starting 10K", sortable: false, align: "right" },
  { key: "cash_in_10k", label: "In 10K", sortable: false, align: "right" },
  { key: "cash_out_10k", label: "Out 10K", sortable: false, align: "right" },
  { key: "cash_position_10k", label: "Pos 10K", sortable: false, align: "right" },
  { key: "starting_cash_20k", label: "Starting 20K", sortable: false, align: "right" },
  { key: "cash_in_20k", label: "In 20K", sortable: false, align: "right" },
  { key: "cash_out_20k", label: "Out 20K", sortable: false, align: "right" },
  { key: "cash_position_20k", label: "Pos 20K", sortable: false, align: "right" },
  { key: "starting_cash_50k", label: "Starting 50K", sortable: false, align: "right" },
  { key: "cash_in_50k", label: "In 50K", sortable: false, align: "right" },
  { key: "cash_out_50k", label: "Out 50K", sortable: false, align: "right" },
  { key: "cash_position_50k", label: "Pos 50K", sortable: false, align: "right" },
  { key: "starting_cash_100k", label: "Starting 100K", sortable: false, align: "right" },
  { key: "cash_in_100k", label: "In 100K", sortable: false, align: "right" },
  { key: "cash_out_100k", label: "Out 100K", sortable: false, align: "right" },
  { key: "cash_position_100k", label: "Pos 100K", sortable: false, align: "right" },
];

const MONEY_KEYS = new Set([
  "starting_cash_10k",
  "cash_in_10k",
  "cash_out_10k",
  "cash_position_10k",
  "starting_cash_20k",
  "cash_in_20k",
  "cash_out_20k",
  "cash_position_20k",
  "starting_cash_50k",
  "cash_in_50k",
  "cash_out_50k",
  "cash_position_50k",
  "starting_cash_100k",
  "cash_in_100k",
  "cash_out_100k",
  "cash_position_100k",
]);

interface AtmCashposTableProps {
  data: readonly AtmCashposRecord[];
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  sortBy: string;
  sortOrder: "asc" | "desc";
  onSortChange: (sortBy: string, sortOrder: "asc" | "desc") => void;
}

export function AtmCashposTable({
  data,
  isLoading,
  isError,
  onRetry,
  sortBy,
  sortOrder,
  onSortChange,
}: AtmCashposTableProps) {
  function handleHeaderClick(column: Column) {
    if (!column.sortable) {
      return;
    }
    if (sortBy === column.key) {
      onSortChange(column.key, sortOrder === "asc" ? "desc" : "asc");
    } else {
      onSortChange(column.key, "asc");
    }
  }

  return (
    <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
      <table
        aria-label="Daftar ATM Cashpos"
        className="w-full min-w-[2200px] border-collapse text-sm"
      >
        <thead>
          <tr className="border-[var(--n-200)] border-b bg-[var(--n-50)]">
            {COLUMNS.map((column) => {
              const isActive = sortBy === column.key;
              return (
                <th
                  key={column.key}
                  scope="col"
                  aria-sort={
                    column.sortable
                      ? isActive
                        ? sortOrder === "asc"
                          ? "ascending"
                          : "descending"
                        : "none"
                      : undefined
                  }
                  className={`px-3 py-2 font-medium text-[var(--n-600)] ${column.align === "right" ? "text-right" : "text-left"}`}
                >
                  {column.sortable ? (
                    <button
                      type="button"
                      onClick={() => handleHeaderClick(column)}
                      className={`inline-flex min-h-[44px] items-center gap-1 font-medium outline-none focus-visible:ring-2 focus-visible:ring-[var(--red-100)] ${
                        column.align === "right" ? "flex-row-reverse" : ""
                      }`}
                    >
                      {column.label}
                      {isActive &&
                        (sortOrder === "asc" ? (
                          <ArrowUp className="h-3.5 w-3.5" aria-hidden="true" />
                        ) : (
                          <ArrowDown className="h-3.5 w-3.5" aria-hidden="true" />
                        ))}
                    </button>
                  ) : (
                    column.label
                  )}
                </th>
              );
            })}
          </tr>
        </thead>
        <tbody>
          {isLoading && <SkeletonRows columnCount={COLUMNS.length} />}
          {!isLoading && isError && <ErrorRow columnCount={COLUMNS.length} onRetry={onRetry} />}
          {!isLoading && !isError && data.length === 0 && <EmptyRow columnCount={COLUMNS.length} />}
          {!isLoading && !isError && data.map((row) => <CashposRow key={row.id} row={row} />)}
        </tbody>
      </table>
    </div>
  );
}

function cellValue(row: AtmCashposRecord, key: string): string {
  if (key === "cashpos_date") {
    return formatAtmDate(new Date(row.cashpos_date));
  }
  if (key === "created_at") {
    return formatAtmDateTime(new Date(row.created_at));
  }
  if (MONEY_KEYS.has(key)) {
    return formatRupiahDecimal(row[key as keyof AtmCashposRecord] as string);
  }
  const v = row[key as keyof AtmCashposRecord];
  return v === null || v === undefined ? "—" : String(v);
}

function CashposRow({ row }: { row: AtmCashposRecord }) {
  return (
    <tr className="border-[var(--n-100)] border-b last:border-0">
      {COLUMNS.map((column) => (
        <td
          key={column.key}
          className={`px-3 py-2 ${column.align === "right" ? "text-right tabular-nums" : ""} ${column.mono ? "font-mono" : ""} ${MONEY_KEYS.has(column.key) ? "font-mono" : ""}`}
        >
          {cellValue(row, column.key)}
        </td>
      ))}
    </tr>
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
          <p className="text-sm text-[var(--n-700)]">Gagal memuat data ATM Cashpos</p>
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
        <p className="text-sm text-[var(--n-700)]">Tidak ada data cashpos yang sesuai filter</p>
        <p className="mt-1 text-xs text-[var(--n-500)]">
          Coba ubah atau hapus filter untuk melihat data
        </p>
      </td>
    </tr>
  );
}
