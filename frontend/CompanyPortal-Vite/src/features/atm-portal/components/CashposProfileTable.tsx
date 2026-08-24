/**
 * Cash position history table for the ATM Profile page: date range filter
 * (with client-side end-before-start validation, since the backend rejects
 * that combination with a 400 — Req 5.8), grouped denomination columns
 * (10K/20K/50K/100K x Starting/In/Out/Position), horizontal scroll.
 */

import { Skeleton } from "@/components/feedback/Skeleton";
import { useEffect, useState } from "react";
import { formatDateIndonesian, formatWholeNumber } from "../lib/formatters";
import type { AtmCashposRecord, AtmProfileHistoryParams } from "../types";
import { PaginationControls } from "./PaginationControls";

const SKELETON_ROW_COUNT = 5;

const DENOMINATIONS = ["10k", "20k", "50k", "100k"] as const;
const DENOM_FIELDS = ["Starting", "In", "Out", "Position"] as const;

function denomKey(
  field: (typeof DENOM_FIELDS)[number],
  denom: (typeof DENOMINATIONS)[number],
): keyof AtmCashposRecord {
  const prefix: Record<(typeof DENOM_FIELDS)[number], string> = {
    Starting: "starting_cash",
    In: "cash_in",
    Out: "cash_out",
    Position: "cash_position",
  };
  return `${prefix[field]}_${denom}` as keyof AtmCashposRecord;
}

const TOTAL_COLUMNS = 4 /* identity cols */ + DENOMINATIONS.length * DENOM_FIELDS.length;

interface CashposProfileTableProps {
  data: readonly AtmCashposRecord[];
  total: number;
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  params: AtmProfileHistoryParams;
  onParamsChange: (partial: Partial<AtmProfileHistoryParams>) => void;
}

export function CashposProfileTable({
  data,
  total,
  isLoading,
  isError,
  onRetry,
  params,
  onParamsChange,
}: CashposProfileTableProps) {
  const [localFrom, setLocalFrom] = useState(params.date_from);
  const [localTo, setLocalTo] = useState(params.date_to);

  useEffect(() => {
    setLocalFrom(params.date_from);
    setLocalTo(params.date_to);
  }, [params.date_from, params.date_to]);

  const isInvalidRange = localFrom !== "" && localTo !== "" && localTo < localFrom;

  function commit(nextFrom: string, nextTo: string): void {
    if (nextFrom !== "" && nextTo !== "" && nextTo < nextFrom) {
      return;
    }
    onParamsChange({ date_from: nextFrom, date_to: nextTo, page: 1 });
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-start gap-4">
        <div className="flex flex-col gap-1">
          <label
            htmlFor="cashpos-date-from"
            className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
          >
            Dari Tanggal
          </label>
          <input
            id="cashpos-date-from"
            type="date"
            value={localFrom}
            onChange={(e) => {
              setLocalFrom(e.target.value);
              commit(e.target.value, localTo);
            }}
            className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
          />
        </div>
        <div className="flex flex-col gap-1">
          <label
            htmlFor="cashpos-date-to"
            className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
          >
            Sampai Tanggal
          </label>
          <input
            id="cashpos-date-to"
            type="date"
            value={localTo}
            onChange={(e) => {
              setLocalTo(e.target.value);
              commit(localFrom, e.target.value);
            }}
            className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
          />
        </div>
        {isInvalidRange && (
          <p role="alert" className="self-end pb-2.5 text-xs text-[var(--danger-fg)]">
            Tanggal akhir tidak boleh sebelum tanggal awal
          </p>
        )}
      </div>

      <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
        <table
          aria-label="Riwayat Cash Position"
          className="w-full min-w-[2000px] border-collapse text-sm"
        >
          <thead>
            <tr className="border-[var(--n-200)] border-b bg-[var(--n-50)]">
              <th
                scope="col"
                rowSpan={2}
                className="px-3 py-2 text-left font-medium text-[var(--n-600)]"
              >
                Tanggal
              </th>
              <th
                scope="col"
                rowSpan={2}
                className="px-3 py-2 text-left font-medium text-[var(--n-600)]"
              >
                Teller ID
              </th>
              <th
                scope="col"
                rowSpan={2}
                className="px-3 py-2 text-left font-medium text-[var(--n-600)]"
              >
                Branch Code
              </th>
              <th
                scope="col"
                rowSpan={2}
                className="px-3 py-2 text-left font-medium text-[var(--n-600)]"
              >
                Sumber
              </th>
              {DENOMINATIONS.map((denom) => (
                <th
                  key={denom}
                  scope="colgroup"
                  colSpan={DENOM_FIELDS.length}
                  className="border-[var(--n-200)] border-l px-3 py-2 text-center font-semibold text-[var(--n-700)] uppercase"
                >
                  {denom}
                </th>
              ))}
            </tr>
            <tr className="border-[var(--n-200)] border-b bg-[var(--n-50)]">
              {DENOMINATIONS.map((denom) =>
                DENOM_FIELDS.map((field, i) => (
                  <th
                    key={`${denom}-${field}`}
                    scope="col"
                    className={`px-3 py-1.5 text-right font-medium text-[var(--n-600)] text-xs ${i === 0 ? "border-[var(--n-200)] border-l" : ""}`}
                  >
                    {field}
                  </th>
                )),
              )}
            </tr>
          </thead>
          <tbody>
            {isLoading && <SkeletonRows />}
            {!isLoading && isError && <ErrorRow onRetry={onRetry} />}
            {!isLoading && !isError && data.length === 0 && <EmptyRow />}
            {!isLoading &&
              !isError &&
              data.map((row) => (
                <tr key={row.id} className="border-[var(--n-100)] border-b last:border-0">
                  <td className="px-3 py-2">{formatDateIndonesian(row.cashpos_date)}</td>
                  <td className="px-3 py-2">{row.teller_id}</td>
                  <td className="px-3 py-2">{row.branch_code}</td>
                  <td className="px-3 py-2">{row.position_source}</td>
                  {DENOMINATIONS.map((denom) =>
                    DENOM_FIELDS.map((field, i) => (
                      <td
                        key={`${denom}-${field}`}
                        className={`px-3 py-2 text-right font-mono tabular-nums ${i === 0 ? "border-[var(--n-200)] border-l" : ""}`}
                      >
                        {formatWholeNumber(row[denomKey(field, denom)] as string)}
                      </td>
                    )),
                  )}
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

function SkeletonRows() {
  return (
    <>
      {Array.from({ length: SKELETON_ROW_COUNT }).map((_, rowIndex) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: generic loading placeholder rows have no stable identity
        <tr key={`cashpos-profile-skeleton-row-${rowIndex}`}>
          {Array.from({ length: TOTAL_COLUMNS }).map((_, colIndex) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: generic loading placeholder cells have no stable identity
            <td key={`cashpos-profile-skeleton-cell-${rowIndex}-${colIndex}`} className="px-3 py-3">
              <Skeleton height={14} className="w-full" />
            </td>
          ))}
        </tr>
      ))}
    </>
  );
}

function ErrorRow({ onRetry }: { onRetry: () => void }) {
  return (
    <tr>
      <td colSpan={TOTAL_COLUMNS} className="px-3 py-12 text-center">
        <div className="flex flex-col items-center gap-3">
          <p className="text-sm text-[var(--n-700)]">Gagal memuat data cashpos</p>
          <button
            type="button"
            onClick={onRetry}
            className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-4 text-sm font-medium text-[var(--n-800)] outline-none focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
          >
            Coba Lagi
          </button>
        </div>
      </td>
    </tr>
  );
}

function EmptyRow() {
  return (
    <tr>
      <td colSpan={TOTAL_COLUMNS} className="px-3 py-12 text-center">
        <p className="text-sm text-[var(--n-700)]">Belum ada data cashpos untuk ATM ini</p>
      </td>
    </tr>
  );
}
