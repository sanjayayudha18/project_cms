/**
 * The main ATM data table: semantic table markup, sortable headers with
 * aria-sort, monospace Terminal ID, right-aligned Rupiah cells, and four
 * render states — loading (5 skeleton rows), error (icon + message + Coba
 * Lagi), empty ("Tidak ada ATM yang sesuai filter"), and loaded (Req 4.1,
 * 4.2, 4.3, 4.4, 4.6, 4.7, 8.1, 8.2, 8.4, 10.2, 10.3, 11.1, 11.7).
 *
 * Purely presentational/controlled, matching FilterBar's convention:
 * sortBy/sortOrder/onSortChange are props, not internal state — the parent
 * screen (Task 10.1) owns the URL-synced sort state via
 * useAtmPortalUrlState and passes it down.
 */

import { Skeleton } from "@/components/feedback/Skeleton";
import { Button } from "@/components/ui/Button";
import { Link } from "@tanstack/react-router";
import { AlertCircle, ArrowDown, ArrowUp } from "lucide-react";
import { formatAtmDate, formatRupiah } from "../lib/formatters";
import type { AtmRecord } from "../types";
import { StatusBadge } from "./StatusBadge";

const SKELETON_ROW_COUNT = 5;

interface Column {
  readonly key: string;
  readonly label: string;
  readonly sortable: boolean;
  readonly align?: "right";
}

const COLUMNS: readonly Column[] = [
  { key: "terminal_id", label: "Terminal ID", sortable: true },
  { key: "location", label: "Location", sortable: true },
  { key: "machine_type", label: "Machine Type", sortable: false },
  { key: "brand", label: "Brand", sortable: false },
  { key: "deployment_type", label: "Deployment Type", sortable: false },
  { key: "last_replenish_date", label: "Last Replenish Date", sortable: true },
  { key: "refund_total", label: "Refund Total", sortable: true, align: "right" },
  { key: "replenish_total", label: "Total Replenish", sortable: true, align: "right" },
  { key: "threshold", label: "Threshold", sortable: false, align: "right" },
  { key: "status", label: "Status", sortable: true },
];

interface AtmTableProps {
  data: readonly AtmRecord[];
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  sortBy: string;
  sortOrder: "asc" | "desc";
  onSortChange: (sortBy: string, sortOrder: "asc" | "desc") => void;
}

export function AtmTable({
  data,
  isLoading,
  isError,
  onRetry,
  sortBy,
  sortOrder,
  onSortChange,
}: AtmTableProps) {
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
      <table aria-label="Daftar ATM" className="w-full min-w-[1040px] border-collapse text-sm">
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
          {!isLoading && !isError && data.map((atm) => <AtmRow key={atm.terminal_id} atm={atm} />)}
        </tbody>
      </table>
    </div>
  );
}

function AtmRow({ atm }: { atm: AtmRecord }) {
  return (
    <tr className="border-[var(--n-100)] border-b last:border-0">
      <td className="px-3 py-2 font-mono">
        <Link
          to="/atm-portal/$terminalId"
          params={{ terminalId: atm.terminal_id }}
          className="text-[var(--red-600)] underline-offset-2 hover:underline"
        >
          {atm.terminal_id}
        </Link>
      </td>
      <td className="px-3 py-2">{atm.location_name}</td>
      <td className="px-3 py-2">{atm.machine_type}</td>
      <td className="px-3 py-2">{atm.brand}</td>
      <td className="px-3 py-2">{atm.deployment_type}</td>
      <td className="px-3 py-2">
        {atm.last_replenish_date ? formatAtmDate(new Date(atm.last_replenish_date)) : "—"}
      </td>
      <td className="px-3 py-2 text-right tabular-nums">{formatRupiah(atm.refund_total)}</td>
      <td className="px-3 py-2 text-right tabular-nums">{formatRupiah(atm.replenish_total)}</td>
      <td className="px-3 py-2 text-right tabular-nums">{formatRupiah(atm.low_threshold)}</td>
      <td className="px-3 py-2">
        <StatusBadge status={atm.status} />
      </td>
    </tr>
  );
}

function SkeletonRows({ columnCount }: { columnCount: number }) {
  return (
    <>
      {Array.from({ length: SKELETON_ROW_COUNT }).map((_, rowIndex) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: generic loading placeholder rows have no stable identity to key by
        <tr key={`skeleton-row-${rowIndex}`}>
          {Array.from({ length: columnCount }).map((_, colIndex) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: generic loading placeholder cells have no stable identity to key by
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
          <p className="text-sm text-[var(--n-700)]">Gagal memuat data ATM</p>
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
        <p className="text-sm text-[var(--n-700)]">Tidak ada ATM yang sesuai filter</p>
        <p className="mt-1 text-xs text-[var(--n-500)]">
          Coba ubah atau hapus filter untuk melihat data
        </p>
      </td>
    </tr>
  );
}
