/**
 * Replenish history table for the ATM Profile page: date range filter,
 * semantic table (matches AtmCashposTable.tsx's structure), pagination.
 */

import { Skeleton } from "@/components/feedback/Skeleton";
import { Button } from "@/components/ui/Button";
import { AlertCircle } from "lucide-react";
import { formatDateIndonesian, formatRupiahDecimal } from "../lib/formatters";
import type { AtmProfileHistoryParams, AtmReplenishRecord } from "../types";
import { PaginationControls } from "./PaginationControls";

const SKELETON_ROW_COUNT = 5;

interface Column {
  readonly key: keyof AtmReplenishRecord;
  readonly label: string;
  readonly align?: "right";
  readonly money?: boolean;
}

const COLUMNS: readonly Column[] = [
  { key: "replenish_date", label: "Tanggal" },
  { key: "replenish_time", label: "Waktu" },
  { key: "refund_denom_10k", label: "Refund 10K", align: "right", money: true },
  { key: "refund_denom_20k", label: "Refund 20K", align: "right", money: true },
  { key: "refund_denom_50k", label: "Refund 50K", align: "right", money: true },
  { key: "refund_denom_100k", label: "Refund 100K", align: "right", money: true },
  { key: "refund_total", label: "Total Refund", align: "right", money: true },
  { key: "replenish_denom_10k", label: "Replenish 10K", align: "right", money: true },
  { key: "replenish_denom_20k", label: "Replenish 20K", align: "right", money: true },
  { key: "replenish_denom_50k", label: "Replenish 50K", align: "right", money: true },
  { key: "replenish_denom_100k", label: "Replenish 100K", align: "right", money: true },
  { key: "replenish_total", label: "Total Replenish", align: "right", money: true },
  { key: "escrow", label: "Escrow", align: "right", money: true },
];

function cellValue(row: AtmReplenishRecord, column: Column): string {
  if (column.key === "replenish_date") {
    return formatDateIndonesian(row.replenish_date);
  }
  if (column.money) {
    return formatRupiahDecimal(row[column.key]);
  }
  return row[column.key];
}

interface ReplenishTableProps {
  data: readonly AtmReplenishRecord[];
  total: number;
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  params: AtmProfileHistoryParams;
  onParamsChange: (partial: Partial<AtmProfileHistoryParams>) => void;
}

export function ReplenishTable({
  data,
  total,
  isLoading,
  isError,
  onRetry,
  params,
  onParamsChange,
}: ReplenishTableProps) {
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end gap-4">
        <div className="flex flex-col gap-1">
          <label
            htmlFor="replenish-date-from"
            className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
          >
            Dari Tanggal
          </label>
          <input
            id="replenish-date-from"
            type="date"
            value={params.date_from}
            onChange={(e) => onParamsChange({ date_from: e.target.value, page: 1 })}
            className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
          />
        </div>
        <div className="flex flex-col gap-1">
          <label
            htmlFor="replenish-date-to"
            className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
          >
            Sampai Tanggal
          </label>
          <input
            id="replenish-date-to"
            type="date"
            value={params.date_to}
            onChange={(e) => onParamsChange({ date_to: e.target.value, page: 1 })}
            className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
          />
        </div>
      </div>

      <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
        <table
          aria-label="Riwayat Replenish"
          className="w-full min-w-[1400px] border-collapse text-sm"
        >
          <thead>
            <tr className="border-[var(--n-200)] border-b bg-[var(--n-50)]">
              {COLUMNS.map((column) => (
                <th
                  key={column.key}
                  scope="col"
                  className={`px-3 py-2 font-medium text-[var(--n-600)] ${column.align === "right" ? "text-right" : "text-left"}`}
                >
                  {column.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {isLoading && <SkeletonRows columnCount={COLUMNS.length} />}
            {!isLoading && isError && <ErrorRow columnCount={COLUMNS.length} onRetry={onRetry} />}
            {!isLoading && !isError && data.length === 0 && (
              <EmptyRow columnCount={COLUMNS.length} />
            )}
            {!isLoading &&
              !isError &&
              data.map((row, i) => (
                <tr
                  key={`${row.replenish_date}-${row.replenish_time}-${i}`}
                  className="border-[var(--n-100)] border-b last:border-0"
                >
                  {COLUMNS.map((column) => (
                    <td
                      key={column.key}
                      className={`px-3 py-2 ${column.align === "right" ? "text-right tabular-nums" : ""} ${column.money ? "font-mono" : ""}`}
                    >
                      {cellValue(row, column)}
                    </td>
                  ))}
                </tr>
              ))}
          </tbody>
        </table>
      </div>

      {!isLoading && !isError && data.length > 0 && (
        <PaginationControls
          page={params.page}
          pageSize={params.page_size}
          total={total}
          onPageChange={(page) => onParamsChange({ page })}
          onPageSizeChange={(pageSize) => onParamsChange({ page_size: pageSize, page: 1 })}
        />
      )}
    </div>
  );
}

function SkeletonRows({ columnCount }: { columnCount: number }) {
  return (
    <>
      {Array.from({ length: SKELETON_ROW_COUNT }).map((_, rowIndex) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: generic loading placeholder rows have no stable identity
        <tr key={`replenish-skeleton-row-${rowIndex}`}>
          {Array.from({ length: columnCount }).map((_, colIndex) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: generic loading placeholder cells have no stable identity
            <td key={`replenish-skeleton-cell-${rowIndex}-${colIndex}`} className="px-3 py-3">
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
          <p className="text-sm text-[var(--n-700)]">Gagal memuat data replenish</p>
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
        <p className="text-sm text-[var(--n-700)]">Belum ada data replenish untuk ATM ini</p>
      </td>
    </tr>
  );
}
