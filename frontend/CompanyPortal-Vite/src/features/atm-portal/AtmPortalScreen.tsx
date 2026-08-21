/**
 * ATM Portal page: composes the presentational Task-9 pieces around
 * useAtmPortalUrlState (URL-owned filter/sort/page) and useAtmPortalData
 * (TanStack Query fetch). Rendered inside the existing `AppShell` via the
 * `_protected` route layout (design.md), so this component does not wrap
 * itself in AppShell — matching ReplenishmentScreen.tsx's convention.
 *
 * Retry re-fetches through `queryClient.refetchQueries` against the exact
 * current query key (Req 5.6/5.7/8.3) rather than resetting any filter
 * state, so an error retry preserves the user's current filter selections.
 */

import { PageHeader } from "@/components/ui/PageHeader";
import { useQueryClient } from "@tanstack/react-query";
import { AriaLiveRegion } from "./components/AriaLiveRegion";
import { AtmTable } from "./components/AtmTable";
import { DataFreshnessIndicator } from "./components/DataFreshnessIndicator";
import { FilterBar } from "./components/FilterBar";
import { PaginationControls } from "./components/PaginationControls";
import { SummaryCardsGrid } from "./components/SummaryCardsGrid";
import { ATM_PORTAL_QUERY_KEY } from "./constants";
import { useAtmPortalData } from "./useAtmPortalData";
import { useAtmPortalUrlState } from "./useAtmPortalUrlState";

export function AtmPortalScreen() {
  const queryClient = useQueryClient();
  const { params, searchInput, setSearchInput, setParams } = useAtmPortalUrlState();
  const { data, isLoading, isError } = useAtmPortalData(params);

  function handleRetry(): void {
    queryClient.refetchQueries({ queryKey: [...ATM_PORTAL_QUERY_KEY, params] });
  }

  function handleSortChange(sortBy: string, sortOrder: "asc" | "desc"): void {
    setParams({ sort_by: sortBy, sort_order: sortOrder });
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
        resultCount={data?.data.length ?? 0}
      />

      <DataFreshnessIndicator lastUpdated={data?.last_updated ?? null} />

      <SummaryCardsGrid summary={data?.summary} isLoading={isLoading} />

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

      <AtmTable
        data={data?.data ?? []}
        isLoading={isLoading}
        isError={isError}
        onRetry={handleRetry}
        sortBy={params.sort_by}
        sortOrder={params.sort_order}
        onSortChange={handleSortChange}
      />

      <PaginationControls
        page={data?.page ?? params.page}
        pageSize={data?.page_size ?? params.page_size}
        total={data?.total ?? 0}
        onPageChange={(page) => setParams({ page })}
        onPageSizeChange={(pageSize) => setParams({ page_size: pageSize, page: 1 })}
      />
    </div>
  );
}
