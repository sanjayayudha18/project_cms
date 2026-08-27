/**
 * DMAA Forecast table: 7 columns, all sortable server-side. Monetary
 * amounts right-aligned tabular-nums monospace with IDR formatting;
 * periode_pred as "dd MMM yyyy"; created_at as "dd MMM yyyy, HH:mm";
 * denom dot-separated (Req 7.1-7.6, 6.6-6.7, 9.1-9.3). Formatting reuses
 * the atm-portal formatters (single source of truth for these shapes).
 */

import { Skeleton } from "@/components/feedback/Skeleton";
import { Button } from "@/components/ui/Button";
import {
  formatAtmDate,
  formatAtmDateTime,
  formatRupiah,
  formatWholeNumber,
} from "@/features/atm-portal/lib/formatters";
import { AlertCircle, ArrowDown, ArrowUp } from "lucide-react";
import type { DmaaForecastRecord } from "./types";

const SKELETON_ROW_COUNT = 5;

interface Column {
  readonly key: string;
  readonly label: string;
  readonly align?: "right";
}

const COLUMNS: readonly Column[] = [
  { key: "terminal_id", label: "Terminal ID" },
  { key: "dmaa_file_id", label: "DMAA File ID" },
  { key: "periode_pred", label: "Periode Prediksi" },
  { key: "denom", label: "Denominasi", align: "right" },
  { key: "amount_replenish", label: "Jumlah Replenish", align: "right" },
  { key: "amount_refund", label: "Jumlah Refund", align: "right" },
  { key: "created_at", label: "Dibuat Pada" },
];

const MONEY_KEYS = new Set(["amount_replenish", "amount_refund"]);

interface DmaaForecastTableProps {
  data: readonly DmaaForecastRecord[];
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  sortBy: string;
  sortOrder: "asc" | "desc";
  onSortChange: (sortBy: string, sortOrder: "asc" | "desc") => void;
}

export function DmaaForecastTable({
  data,
  isLoading,
  isError,
  onRetry,
  sortBy,
  sortOrder,
  onSortChange,
}: DmaaForecastTableProps) {
  function handleHeaderClick(column: Column) {
    if (sortBy === column.key) {
      onSortChange(column.key, sortOrder === "asc" ? "desc" : "asc");
    } else {
      onSortChange(column.key, "asc");
    }
  }

  return (
    <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
      <table
        aria-label="Data DMAA Forecast"
        className="w-full min-w-[900px] border-collapse text-sm"
      >
        <thead>
          <tr className="border-[var(--n-200)] border-b bg-[var(--n-50)]">
            {COLUMNS.map((column) => {
              const isActive = sortBy === column.key;
              return (
                <th
                  key={column.key}
                  scope="col"
                  aria-sort={isActive ? (sortOrder === "asc" ? "ascending" : "descending") : "none"}
                  className={`px-3 py-2 font-medium text-[var(--n-600)] ${column.align === "right" ? "text-right" : "text-left"}`}
                >
                  <button
                    type="button"
                    onClick={() => handleHeaderClick(column)}
                    className={`inline-flex min-h-[44px] items-center gap-1 font-medium outline-none focus-visible:ring-2 focus-visible:ring-[var(--red-100)] ${column.align === "right" ? "flex-row-reverse" : ""}`}
                  >
                    {column.label}
                    {isActive &&
                      (sortOrder === "asc" ? (
                        <ArrowUp className="h-3.5 w-3.5" aria-hidden="true" />
                      ) : (
                        <ArrowDown className="h-3.5 w-3.5" aria-hidden="true" />
                      ))}
                  </button>
                </th>
              );
            })}
          </tr>
        </thead>
        <tbody>
          {isLoading && <SkeletonRows columnCount={COLUMNS.length} />}
          {!isLoading && isError && <ErrorRow columnCount={COLUMNS.length} onRetry={onRetry} />}
          {!isLoading && !isError && data.length === 0 && <EmptyRow columnCount={COLUMNS.length} />}
          {!isLoading &&
            !isError &&
            data.map((row) => (
              <DmaaRow
                key={`${row.dmaa_file_id}-${row.terminal_id}-${row.periode_pred}-${row.denom}`}
                row={row}
              />
            ))}
        </tbody>
      </table>
    </div>
  );
}

function cellValue(row: DmaaForecastRecord, key: string): string {
  if (key === "periode_pred") {
    return formatAtmDate(new Date(row.periode_pred));
  }
  if (key === "created_at") {
    return formatAtmDateTime(new Date(row.created_at));
  }
  if (key === "denom") {
    return formatWholeNumber(String(row.denom));
  }
  if (MONEY_KEYS.has(key)) {
    return formatRupiah(row[key as keyof DmaaForecastRecord] as number);
  }
  return String(row[key as keyof DmaaForecastRecord]);
}

function DmaaRow({ row }: { row: DmaaForecastRecord }) {
  return (
    <tr className="border-[var(--n-100)] border-b last:border-0">
      {COLUMNS.map((column) => (
        <td
          key={column.key}
          className={`px-3 py-2 ${column.align === "right" ? "text-right tabular-nums" : ""} ${MONEY_KEYS.has(column.key) || column.key === "terminal_id" ? "font-mono" : ""}`}
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
          <p className="text-sm text-[var(--n-700)]">Gagal memuat data DMAA Forecast</p>
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
          Tidak ada data forecast yang sesuai dengan kriteria
        </p>
        <p className="mt-1 text-xs text-[var(--n-500)]">
          Coba ubah atau hapus filter untuk melihat data
        </p>
      </td>
    </tr>
  );
}
