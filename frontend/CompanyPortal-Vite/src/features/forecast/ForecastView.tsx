import { useMemo, useState } from "react";

import { EmptyState } from "@/components/ui/EmptyState";
import { FilterSelect } from "@/components/ui/FilterSelect";
import { SummaryCard } from "@/components/ui/SummaryCard";

import { ForecastTable } from "./ForecastTable";
import { ScheduleList } from "./ScheduleList";
import { useForecastData } from "./useForecastData";

const priorityOptions = [
  { value: "High", label: "High" },
  { value: "Medium", label: "Medium" },
  { value: "Low", label: "Low" },
];

export function ForecastView() {
  const { data: records = [], isError } = useForecastData();
  const [priorityFilter, setPriorityFilter] = useState<string | null>(null);

  const filteredRecords = useMemo(() => {
    if (!priorityFilter) return records;
    return records.filter((r) => r.priority === priorityFilter);
  }, [records, priorityFilter]);

  const totalReplenishment = useMemo(
    () => filteredRecords.reduce((sum, r) => sum + r.recommendedReplenishment, 0),
    [filteredRecords],
  );

  if (isError) {
    return (
      <div className="p-6">
        <div className="flex items-center gap-2 rounded-[var(--radius-md)] border border-[var(--danger-bg)] bg-[var(--danger-bg)] p-4 text-sm text-[var(--danger-fg)]">
          <span aria-hidden="true">⚠</span>
          <span>Gagal memuat data forecast. Silakan periksa file data.</span>
        </div>
      </div>
    );
  }

  if (records.length === 0) {
    return (
      <div className="p-6 space-y-6">
        <header>
          <h1 className="text-xl font-semibold text-[var(--n-900)]">Forecast</h1>
          <p className="text-sm text-[var(--n-600)] mt-1">
            Proyeksi kebutuhan kas dan rekomendasi pengisian
          </p>
        </header>
        <EmptyState message="Tidak ada data forecast tersedia." />
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      <header>
        <h1 className="text-xl font-semibold text-[var(--n-900)]">Forecast</h1>
        <p className="text-sm text-[var(--n-600)] mt-1">
          Proyeksi kebutuhan kas dan rekomendasi pengisian
        </p>
      </header>

      <div className="flex flex-wrap items-end gap-4">
        <FilterSelect
          label="Prioritas"
          options={priorityOptions}
          value={priorityFilter}
          onChange={setPriorityFilter}
          placeholder="Semua Prioritas"
        />
        <SummaryCard
          label="Total Rekomendasi Pengisian"
          value={totalReplenishment}
          format="currency"
        />
      </div>

      {filteredRecords.length === 0 ? (
        <EmptyState message="Tidak ada ATM yang sesuai dengan prioritas yang dipilih" />
      ) : (
        <ForecastTable data={filteredRecords} />
      )}

      <ScheduleList />
    </div>
  );
}
