/**
 * Forecast Browser page (Req 11): pick a forecast date, browse
 * dmaa_atm_forecast rows, select some, and jump into vendor-request
 * creation with the selection carried via selectionStore (route state has
 * no generic-object option in TanStack Router v1).
 */

import { Button } from "@/components/ui/Button";
import { PageHeader } from "@/components/ui/PageHeader";
import { useToast } from "@/lib/hooks/useToast";
import { formatIDR } from "@/lib/utils/formatCurrency";
import { useNavigate } from "@tanstack/react-router";
import type { OnChangeFn, RowSelectionState, SortingState } from "@tanstack/react-table";
import { useEffect, useState } from "react";
import { ForecastTable, forecastRowId } from "./ForecastTable";
import { getVendorRequestErrorMessage, useForecastBrowse, useForecastSelectAll } from "./hooks";
import { nextBusinessDayISO } from "./lib/nextBusinessDay";
import { usePendingVendorRequestSelection } from "./selectionStore";
import type { ForecastRow } from "./types";

const DEFAULT_PAGE_SIZE = 25;
const ATM_FILTER_DEBOUNCE_MS = 300;
// Vendor Request payloads are capped at 1000 items server-side
// (request-replenish-to-vendor spec) — select-all must respect the same cap.
const SELECT_ALL_ITEM_CAP = 1000;

export function ForecastBrowser() {
  const navigate = useNavigate();
  const setPending = usePendingVendorRequestSelection((s) => s.setPending);
  const { toast } = useToast();

  const [forecastDate, setForecastDate] = useState(() => nextBusinessDayISO());
  const [atmIdInput, setAtmIdInput] = useState("");
  const [debouncedAtmId, setDebouncedAtmId] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);
  const [sorting, setSorting] = useState<SortingState>([{ id: "terminal_id", desc: false }]);
  const [rowSelection, setRowSelectionState] = useState<RowSelectionState>({});
  const [selectedRowsMap, setSelectedRowsMap] = useState<Record<string, ForecastRow>>({});

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedAtmId(atmIdInput), ATM_FILTER_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [atmIdInput]);

  const { data, isLoading, isError, refetch } = useForecastBrowse({
    forecastDate,
    atmId: debouncedAtmId,
    page,
    pageSize,
  });

  const selectAllQuery = useForecastSelectAll(
    { forecastDate, atmId: debouncedAtmId },
    SELECT_ALL_ITEM_CAP,
  );

  // Sorting applies to the currently fetched page only — the backend
  // guarantees atm_id ascending as the base order (Req 3.2); ForecastTable's
  // TanStack sort model re-orders within that page for the other two
  // sortable columns (denom, amount_replenish).
  const rows = data?.data ?? [];

  const rowSelectionEntries = Object.keys(selectedRowsMap).length;
  const selectedTotal = Object.values(selectedRowsMap).reduce(
    (sum, row) => sum + row.amount_replenish,
    0,
  );

  const handleRowSelectionChange: OnChangeFn<RowSelectionState> = (updater) => {
    setRowSelectionState((old) => {
      const next = typeof updater === "function" ? updater(old) : updater;
      setSelectedRowsMap((prevMap) => {
        const nextMap = { ...prevMap };
        for (const row of rows) {
          const id = forecastRowId(row);
          if (next[id]) {
            nextMap[id] = row;
          } else {
            delete nextMap[id];
          }
        }
        return nextMap;
      });
      return next;
    });
  };

  function handleDateChange(nextDate: string): void {
    setForecastDate(nextDate);
    setPage(1);
    setRowSelectionState({});
    setSelectedRowsMap({});
  }

  function handleAtmIdChange(value: string): void {
    setAtmIdInput(value);
    setPage(1);
    // Req 2.6: the selection may include rows outside the new filter —
    // clearing is simpler than reconciling and never misrepresents it.
    setRowSelectionState({});
    setSelectedRowsMap({});
  }

  function handlePageSizeChange(nextPageSize: number): void {
    setPageSize(nextPageSize);
    setPage(1);
  }

  function handleCreateClick(): void {
    setPending({ forecastDate, items: Object.values(selectedRowsMap) });
    navigate({ to: "/replenishment/vendor-requests/new" });
  }

  async function handleSelectAll(): Promise<void> {
    const result = await selectAllQuery.refetch();
    if (result.error) {
      toast({ type: "error", message: getVendorRequestErrorMessage(result.error) });
      return;
    }
    const payload = result.data;
    if (!payload) return;

    if (payload.exceededCap) {
      toast({
        type: "warning",
        message: `Ditemukan ${payload.totalCount} rekomendasi, melebihi batas ${SELECT_ALL_ITEM_CAP} item per Vendor Request. Persempit tanggal atau filter ATM ID, lalu coba lagi.`,
      });
      return;
    }

    // Merge onto the existing map/selection (Req 2.8: preserve any manual
    // ticks already made) rather than replacing it.
    setSelectedRowsMap((prev) => {
      const next = { ...prev };
      for (const row of payload.rows) {
        next[forecastRowId(row)] = row;
      }
      return next;
    });
    setRowSelectionState((prev) => {
      const next = { ...prev };
      for (const row of payload.rows) {
        next[forecastRowId(row)] = true;
      }
      return next;
    });
  }

  function handleClearSelection(): void {
    setRowSelectionState({});
    setSelectedRowsMap({});
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Replenishment"
        title="Forecast Browser"
        description="Pilih data forecast DMAA untuk disusun menjadi Vendor Request ke vendor CIT"
      />

      <div className="flex flex-wrap items-end gap-4">
        <div className="flex flex-col gap-1">
          <label
            htmlFor="forecast-date"
            className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
          >
            Tanggal Forecast
          </label>
          <input
            id="forecast-date"
            type="date"
            value={forecastDate}
            onChange={(e) => handleDateChange(e.target.value)}
            className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
          />
        </div>

        <div className="flex flex-col gap-1">
          <label
            htmlFor="forecast-atm-filter"
            className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
          >
            Cari ATM ID
          </label>
          <input
            id="forecast-atm-filter"
            type="text"
            value={atmIdInput}
            onChange={(e) => handleAtmIdChange(e.target.value)}
            placeholder="Cari ATM ID..."
            className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
          />
        </div>
      </div>

      <ForecastTable
        data={rows}
        isLoading={isLoading}
        isError={isError}
        onRetry={() => refetch()}
        sorting={sorting}
        onSortingChange={setSorting}
        rowSelection={rowSelection}
        onRowSelectionChange={handleRowSelectionChange}
        pagination={data?.pagination}
        page={page}
        pageSize={pageSize}
        onPageChange={setPage}
        onPageSizeChange={handlePageSizeChange}
      />

      <div className="flex flex-wrap items-center justify-between gap-4 rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-50)] px-4 py-3">
        <div className="flex items-center gap-6 text-sm text-[var(--n-700)]">
          <span>
            <span className="font-semibold text-[var(--n-900)]">{rowSelectionEntries}</span> item
            terpilih
          </span>
          <span>
            Total:{" "}
            <span className="font-semibold tabular-nums text-[var(--n-900)]">
              Rp {formatIDR(selectedTotal)}
            </span>
          </span>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="secondary"
            onClick={handleSelectAll}
            disabled={!data || data.pagination.total_count === 0 || selectAllQuery.isFetching}
          >
            {selectAllQuery.isFetching ? "Memilih…" : "Pilih Semua Rekomendasi"}
          </Button>
          <Button
            variant="secondary"
            onClick={handleClearSelection}
            disabled={rowSelectionEntries === 0}
          >
            Bersihkan Pilihan
          </Button>
          <Button onClick={handleCreateClick} disabled={rowSelectionEntries === 0}>
            Buat Vendor Request
          </Button>
        </div>
      </div>
    </div>
  );
}
