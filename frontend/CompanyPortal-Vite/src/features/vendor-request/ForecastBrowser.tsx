/**
 * Forecast Browser page (Req 11): pick a forecast date, browse
 * dmaa_atm_forecast rows, select some, and jump into vendor-request
 * creation with the selection carried via selectionStore (route state has
 * no generic-object option in TanStack Router v1).
 *
 * forecast-browser-summary (.claude/sdlc/forecast-browser-summary): the page
 * opens on a vendor × region recap plus the unfiltered detail table — FLM
 * Vendor/Region are optional ("Semua"), so no ATM hides behind an unpicked
 * filter. A recap row sets the detail filters. Create still needs every
 * selected row to share one vendor (CIT-2 Req 4).
 */

import { Button } from "@/components/ui/Button";
import { FilterSelect } from "@/components/ui/FilterSelect";
import { PageHeader } from "@/components/ui/PageHeader";
import { useToast } from "@/lib/hooks/useToast";
import { formatIDR } from "@/lib/utils/formatCurrency";
import { useNavigate } from "@tanstack/react-router";
import type { OnChangeFn, RowSelectionState, SortingState } from "@tanstack/react-table";
import { X } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { ForecastSummary, summaryGroupKey } from "./ForecastSummary";
import { ForecastTable, forecastRowId, isForecastRowSelectable } from "./ForecastTable";
import {
  getVendorRequestErrorMessage,
  useForecastBrowse,
  useForecastSelectAll,
  useForecastSummary,
  useVendorOptions,
} from "./hooks";
import { nextBusinessDayISO } from "./lib/nextBusinessDay";
import { usePendingVendorRequestSelection } from "./selectionStore";
import type { ForecastRow, ForecastSummaryGroup } from "./types";

const DEFAULT_PAGE_SIZE = 10;
const ATM_FILTER_DEBOUNCE_MS = 300;
// Same static ATM brand list as atm-portal/components/FilterBar.tsx — no
// dedicated "distinct brands" endpoint exists, and 3 known values don't
// warrant one.
const BRAND_OPTIONS = ["Hyosung", "Wincor", "Diebold"].map((b) => ({ value: b, label: b }));
// Vendor Request payloads are capped at 1000 items server-side
// (request-replenish-to-vendor spec) — select-all must respect the same cap.
const SELECT_ALL_ITEM_CAP = 1000;
const INPUT_CLASS =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]";
const LABEL_CLASS = "text-xs font-medium uppercase tracking-wider text-[var(--n-600)]";

/**
 * "" = Semua for the three selects. unassigned (no active vendor) and
 * noRegion (vendor branch without region, review R1) are set only from a recap
 * row and dropped as soon as the vendor or region select changes.
 */
interface DetailFilters {
  brand: string;
  flmVendor: string;
  flmVendorRegion: string;
  unassigned: boolean;
  noRegion: boolean;
}

const EMPTY_FILTERS: DetailFilters = {
  brand: "",
  flmVendor: "",
  flmVendorRegion: "",
  unassigned: false,
  noRegion: false,
};

const toOption = (value: string) => ({ value, label: value });

export function ForecastBrowser() {
  const navigate = useNavigate();
  const setPending = usePendingVendorRequestSelection((s) => s.setPending);
  const { toast } = useToast();
  const detailRef = useRef<HTMLDivElement>(null);

  const [forecastDate, setForecastDate] = useState(() => nextBusinessDayISO());
  const [atmIdInput, setAtmIdInput] = useState("");
  const [debouncedAtmId, setDebouncedAtmId] = useState("");
  const [filters, setFilters] = useState<DetailFilters>(EMPTY_FILTERS);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);
  const [sorting, setSorting] = useState<SortingState>([{ id: "terminal_id", desc: false }]);
  const [rowSelection, setRowSelectionState] = useState<RowSelectionState>({});
  const [selectedRowsMap, setSelectedRowsMap] = useState<Record<string, ForecastRow>>({});

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedAtmId(atmIdInput), ATM_FILTER_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [atmIdInput]);

  const vendorOptionsQuery = useVendorOptions();
  const vendorOptions = vendorOptionsQuery.data?.vendors ?? [];
  const regionOptions = vendorOptionsQuery.data?.regions ?? [];

  const summaryQuery = useForecastSummary(forecastDate);

  const filterParams = { forecastDate, atmId: debouncedAtmId, ...filters };
  const { data, isLoading, isError, refetch } = useForecastBrowse({
    ...filterParams,
    page,
    pageSize,
  });
  const selectAllQuery = useForecastSelectAll(filterParams, SELECT_ALL_ITEM_CAP);

  // Sorting applies to the currently fetched page only — the backend
  // guarantees atm_id ascending as the base order (Req 3.2); ForecastTable's
  // TanStack sort model re-orders within that page for the other two
  // sortable columns (denom, amount_replenish).
  const rows = data?.data ?? [];

  const selectedRows = Object.values(selectedRowsMap);
  const selectedTotal = selectedRows.reduce((sum, row) => sum + row.amount_replenish, 0);
  // Every Vendor Request is for exactly one vendor (CIT-2 Req 4): with the
  // vendor filter now optional, the vendor comes from the selected rows.
  const selectedVendors = [...new Set(selectedRows.map((row) => row.flm_vendor))];
  const isMixedVendor = selectedVendors.length > 1;

  // Which recap row the detail table currently mirrors (highlighted there).
  let activeGroupKey: string | null = null;
  if (filters.unassigned) {
    activeGroupKey = "";
  } else if (filters.flmVendor && (filters.flmVendorRegion || filters.noRegion)) {
    activeGroupKey = summaryGroupKey({
      flm_vendor: filters.flmVendor,
      flm_vendor_region: filters.flmVendorRegion,
    });
  }

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

  // Req 2.6: any date/filter change may leave selected rows outside the new
  // result — clearing is simpler than reconciling and never misrepresents it.
  function resetPageAndSelection(): void {
    setPage(1);
    setRowSelectionState({});
    setSelectedRowsMap({});
  }

  function handleDateChange(nextDate: string): void {
    setForecastDate(nextDate);
    resetPageAndSelection();
  }

  function handleAtmIdChange(value: string): void {
    setAtmIdInput(value);
    resetPageAndSelection();
  }

  // Discrete selects refetch immediately on change (Req 1.11) — no debounce
  // needed, unlike the free-text ATM ID filter above. A vendor/region pick
  // drops the recap-only flags, which can't be combined with it.
  function handleFilterChange(patch: Partial<DetailFilters>): void {
    const touchesVendor = "flmVendor" in patch || "flmVendorRegion" in patch;
    setFilters((prev) => ({
      ...prev,
      ...(touchesVendor ? { unassigned: false, noRegion: false } : {}),
      ...patch,
    }));
    resetPageAndSelection();
  }

  function handleSelectGroup(group: ForecastSummaryGroup): void {
    const base = { flmVendor: group.flm_vendor, flmVendorRegion: group.flm_vendor_region };
    if (group.flm_vendor === "") {
      handleFilterChange({ ...base, unassigned: true });
    } else if (group.flm_vendor_region === "") {
      handleFilterChange({ ...base, noRegion: true });
    } else {
      handleFilterChange(base);
    }
    detailRef.current?.scrollIntoView?.({ behavior: "smooth", block: "start" });
  }

  function handlePageSizeChange(nextPageSize: number): void {
    setPageSize(nextPageSize);
    setPage(1);
  }

  function handleCreateClick(): void {
    // Enabled only for a single-vendor selection, and rows without a vendor
    // can't be selected, so selectedVendors[0] is a real vendor name (Req 4,
    // Q2). Create re-checks every terminal's active vendor server-side.
    const vendorId = vendorOptions.find((v) => v.name === selectedVendors[0])?.id ?? 0;
    setPending({ forecastDate, items: selectedRows, vendorId });
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
    // ticks already made) rather than replacing it. Already-requested rows
    // are skipped (forecast-browser-summary FR4.5) — they'd be requested twice.
    const selectable = payload.rows.filter(isForecastRowSelectable);
    setSelectedRowsMap((prev) => {
      const next = { ...prev };
      for (const row of selectable) {
        next[forecastRowId(row)] = row;
      }
      return next;
    });
    setRowSelectionState((prev) => {
      const next = { ...prev };
      for (const row of selectable) {
        next[forecastRowId(row)] = true;
      }
      return next;
    });
  }

  function handleClearSelection(): void {
    setRowSelectionState({});
    setSelectedRowsMap({});
  }

  const isSelectAllDisabled =
    filters.flmVendor === "" ||
    !data ||
    data.pagination.total_count === 0 ||
    selectAllQuery.isFetching;

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Replenishment"
        title="Forecast Browser"
        description="Pilih data forecast DMAA untuk disusun menjadi Vendor Request ke vendor CIT"
      />

      <div className="flex flex-col gap-1">
        <label htmlFor="forecast-date" className={LABEL_CLASS}>
          Tanggal Forecast
        </label>
        <input
          id="forecast-date"
          type="date"
          value={forecastDate}
          onChange={(e) => handleDateChange(e.target.value)}
          className={`${INPUT_CLASS} w-fit`}
        />
      </div>

      <ForecastSummary
        data={summaryQuery.data}
        isLoading={summaryQuery.isLoading}
        isError={summaryQuery.isError}
        onRetry={() => summaryQuery.refetch()}
        activeKey={activeGroupKey}
        onSelectGroup={handleSelectGroup}
      />

      <div ref={detailRef} className="flex scroll-mt-4 flex-col gap-4">
        <h2 className="text-base font-semibold text-[var(--n-900)]">Detail Rekomendasi ATM</h2>
        <div className="flex flex-wrap items-end gap-4">
          <div className="flex flex-col gap-1">
            <label htmlFor="forecast-atm-filter" className={LABEL_CLASS}>
              Cari ATM ID
            </label>
            <input
              id="forecast-atm-filter"
              type="text"
              value={atmIdInput}
              onChange={(e) => handleAtmIdChange(e.target.value)}
              placeholder="Cari ATM ID..."
              className={INPUT_CLASS}
            />
          </div>
          <FilterSelect
            label="FLM Vendor"
            value={filters.flmVendor}
            options={vendorOptions.map((v) => toOption(v.name))}
            onChange={(v) => handleFilterChange({ flmVendor: v ?? "" })}
          />
          <FilterSelect
            label="FLM Vendor Region"
            value={filters.flmVendorRegion}
            options={regionOptions.map(toOption)}
            onChange={(v) => handleFilterChange({ flmVendorRegion: v ?? "" })}
          />
          <FilterSelect
            label="Brand"
            value={filters.brand}
            options={BRAND_OPTIONS}
            onChange={(v) => handleFilterChange({ brand: v ?? "" })}
          />
          {filters.unassigned && (
            <FilterChip
              label="Hanya ATM tanpa vendor aktif"
              onRemove={() => handleFilterChange({ unassigned: false })}
            />
          )}
          {filters.noRegion && (
            <FilterChip
              label="Tanpa region"
              onRemove={() => handleFilterChange({ noRegion: false })}
            />
          )}
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
          <div className="flex flex-col gap-1 text-sm text-[var(--n-700)]">
            <div className="flex items-center gap-6">
              <span>
                <span className="font-semibold text-[var(--n-900)]">{selectedRows.length}</span>{" "}
                item terpilih
              </span>
              <span>
                Total:{" "}
                <span className="font-semibold tabular-nums text-[var(--n-900)]">
                  Rp {formatIDR(selectedTotal)}
                </span>
              </span>
            </div>
            {isMixedVendor && (
              <output className="block font-medium text-[var(--warning-fg)]">
                Pilihan berisi {selectedVendors.length} vendor. Satu Vendor Request hanya untuk satu
                vendor.
              </output>
            )}
            {filters.flmVendor === "" && (
              <p className="text-xs text-[var(--n-600)]">
                Pilih FLM Vendor untuk memakai "Pilih Semua Rekomendasi".
              </p>
            )}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="secondary" onClick={handleSelectAll} disabled={isSelectAllDisabled}>
              {selectAllQuery.isFetching ? "Memilih…" : "Pilih Semua Rekomendasi"}
            </Button>
            <Button
              variant="secondary"
              onClick={handleClearSelection}
              disabled={selectedRows.length === 0}
            >
              Bersihkan Pilihan
            </Button>
            <Button
              onClick={handleCreateClick}
              disabled={selectedRows.length === 0 || isMixedVendor}
            >
              Buat Vendor Request
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

/** Removable pill for a recap-only filter (no input control of its own). */
function FilterChip({ label, onRemove }: { label: string; onRemove: () => void }) {
  return (
    <button
      type="button"
      onClick={onRemove}
      className="inline-flex min-h-[44px] items-center gap-2 rounded-full border border-[var(--n-300)] bg-[var(--n-50)] px-4 text-sm text-[var(--n-800)] outline-none hover:bg-[var(--n-100)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
    >
      {label}
      <X className="h-4 w-4" aria-hidden="true" />
      <span className="sr-only">(hapus filter)</span>
    </button>
  );
}
