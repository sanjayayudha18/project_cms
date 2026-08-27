/**
 * DMAA Forecast Viewer page: URL-owned state, filters, table, pagination.
 * Read-only — no row selection, export, or mutation (Req overview).
 */

import { PageHeader } from "@/components/ui/PageHeader";
import { PaginationControls } from "@/features/atm-portal/components/PaginationControls";
import { DmaaForecastFilters } from "./DmaaForecastFilters";
import { DmaaForecastTable } from "./DmaaForecastTable";
import { useDmaaForecastData } from "./useDmaaForecastData";
import { useDmaaForecastUrlState } from "./useDmaaForecastUrlState";

export function DmaaForecastView() {
  const { params, terminalInput, setTerminalInput, setParams } = useDmaaForecastUrlState();
  const { data, isLoading, isError, refetch } = useDmaaForecastData(params);

  function handleSortChange(sortBy: string, sortOrder: "asc" | "desc"): void {
    setParams({ sortBy, sortOrder, page: 1 });
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Peramalan"
        title="DMAA Forecast"
        description="Data prediksi replenishment dan refund per ATM dari mesin prediksi DMAA"
      />

      <DmaaForecastFilters
        terminalInput={terminalInput}
        onTerminalInputChange={setTerminalInput}
        dateFrom={params.dateFrom}
        dateTo={params.dateTo}
        onFilterChange={setParams}
      />

      <DmaaForecastTable
        data={data?.data ?? []}
        isLoading={isLoading}
        isError={isError}
        onRetry={() => refetch()}
        sortBy={params.sortBy}
        sortOrder={params.sortOrder}
        onSortChange={handleSortChange}
      />

      <PaginationControls
        page={data?.pagination.page ?? params.page}
        pageSize={data?.pagination.page_size ?? params.pageSize}
        total={data?.pagination.total_rows ?? 0}
        onPageChange={(nextPage) => setParams({ page: nextPage })}
        onPageSizeChange={(nextSize) => setParams({ pageSize: nextSize, page: 1 })}
      />
    </div>
  );
}
