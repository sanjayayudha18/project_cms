/**
 * ATM Portal page: mode selector (Replenish | Cashpos), URL-owned state,
 * and mode-specific table/query composition.
 *
 * Layout order matches replenish chrome: header → freshness/summary →
 * filters (incl. dates) → table mode dropdown → table → pagination.
 */

import { PageHeader } from "@/components/ui/PageHeader";
import { useQueryClient } from "@tanstack/react-query";
import { AriaLiveRegion } from "./components/AriaLiveRegion";
import { AtmCashposTable } from "./components/AtmCashposTable";
import { AtmTable } from "./components/AtmTable";
import { DataFreshnessIndicator } from "./components/DataFreshnessIndicator";
import { FilterBar } from "./components/FilterBar";
import { PaginationControls } from "./components/PaginationControls";
import { SummaryCardsGrid } from "./components/SummaryCardsGrid";
import { TableModeSelect } from "./components/TableModeSelect";
import { ATM_CASHPOS_QUERY_KEY, ATM_PORTAL_MODE_CASHPOS, ATM_PORTAL_QUERY_KEY } from "./constants";
import { useAtmCashposData, useAtmPortalData } from "./useAtmPortalData";
import { useAtmPortalUrlState } from "./useAtmPortalUrlState";

export function AtmPortalScreen() {
  const queryClient = useQueryClient();
  const { params, searchInput, setSearchInput, setParams, setMode } = useAtmPortalUrlState();
  const isCashpos = params.mode === ATM_PORTAL_MODE_CASHPOS;

  // Replenish query stays enabled for shared chrome (freshness + summary)
  // even in cashpos mode so the page chrome matches ATM Replenish.
  const replenishQuery = useAtmPortalData({ ...params, mode: "replenish" });
  const cashposQuery = useAtmCashposData(params);

  const isLoading = isCashpos ? cashposQuery.isLoading : replenishQuery.isLoading;
  const isError = isCashpos ? cashposQuery.isError : replenishQuery.isError;
  const resultCount = isCashpos
    ? (cashposQuery.data?.data.length ?? 0)
    : (replenishQuery.data?.data.length ?? 0);
  const page = isCashpos
    ? (cashposQuery.data?.page ?? params.page)
    : (replenishQuery.data?.page ?? params.page);
  const pageSize = isCashpos
    ? (cashposQuery.data?.page_size ?? params.page_size)
    : (replenishQuery.data?.page_size ?? params.page_size);
  const total = isCashpos ? (cashposQuery.data?.total ?? 0) : (replenishQuery.data?.total ?? 0);

  function handleRetry(): void {
    if (isCashpos) {
      queryClient.refetchQueries({
        queryKey: [
          ...ATM_CASHPOS_QUERY_KEY,
          {
            page: params.page,
            page_size: params.page_size,
            search: params.search,
            date_from: params.date_from,
            date_to: params.date_to,
            sort_by: params.sort_by,
            sort_order: params.sort_order,
          },
        ],
      });
      return;
    }
    queryClient.refetchQueries({ queryKey: [...ATM_PORTAL_QUERY_KEY, params] });
  }

  function handleSortChange(sortBy: string, sortOrder: "asc" | "desc"): void {
    setParams({ sort_by: sortBy, sort_order: sortOrder, page: 1 });
  }

  function handleClearAll(): void {
    setParams({
      status: "all",
      machine_type: "",
      brand: "",
      deployment_type: "",
      date_from: "",
      date_to: "",
      page: 1,
    });
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Monitoring"
        title="ATM Portal"
        description="Monitor posisi kas dan status replenishment seluruh ATM"
      />

      <AriaLiveRegion
        isLoading={isLoading}
        isError={isError}
        resultCount={resultCount}
        mode={params.mode}
      />

      <DataFreshnessIndicator lastUpdated={replenishQuery.data?.last_updated ?? null} />

      <SummaryCardsGrid
        summary={replenishQuery.data?.summary}
        isLoading={replenishQuery.isLoading && !replenishQuery.data}
      />

      <FilterBar
        searchInput={searchInput}
        onSearchInputChange={setSearchInput}
        status={params.status}
        machineType={params.machine_type}
        brand={params.brand}
        deploymentType={params.deployment_type}
        dateFrom={params.date_from}
        dateTo={params.date_to}
        onFilterChange={setParams}
        onClearAll={handleClearAll}
      />

      <TableModeSelect mode={params.mode} onModeChange={setMode} />

      {isCashpos ? (
        <AtmCashposTable
          data={cashposQuery.data?.data ?? []}
          isLoading={isLoading}
          isError={isError}
          onRetry={handleRetry}
          sortBy={params.sort_by}
          sortOrder={params.sort_order}
          onSortChange={handleSortChange}
        />
      ) : (
        <AtmTable
          data={replenishQuery.data?.data ?? []}
          isLoading={isLoading}
          isError={isError}
          onRetry={handleRetry}
          sortBy={params.sort_by}
          sortOrder={params.sort_order}
          onSortChange={handleSortChange}
        />
      )}

      <PaginationControls
        page={page}
        pageSize={pageSize}
        total={total}
        onPageChange={(nextPage) => setParams({ page: nextPage })}
        onPageSizeChange={(nextSize) => setParams({ page_size: nextSize, page: 1 })}
      />
    </div>
  );
}
