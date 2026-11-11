import { useMemo, useState } from 'react';

import { EmptyState } from '@/components/ui/EmptyState';
import { FilterSelect } from '@/components/ui/FilterSelect';
import { SummaryCard } from '@/components/ui/SummaryCard';

import { ForecastTable } from './ForecastTable';
import { ScheduleList } from './ScheduleList';
import { useForecastData } from './useForecastData';

const priorityOptions = [
  { value: 'High', label: 'High' },
  { value: 'Medium', label: 'Medium' },
  { value: 'Low', label: 'Low' },
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
        <div className="bg-danger-bg text-danger-fg rounded-lg p-4 text-sm">
          Unable to load data. Please check mock data files.
        </div>
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      <header>
        <h1 className="text-xl font-semibold text-n-900">Forecast View</h1>
        <p className="text-sm text-n-600 mt-1">
          Cash demand forecasting and replenishment recommendations
        </p>
      </header>

      <div className="flex flex-wrap items-end gap-4">
        <FilterSelect
          label="Priority"
          options={priorityOptions}
          value={priorityFilter}
          onChange={setPriorityFilter}
          placeholder="All Priorities"
        />
        <SummaryCard
          label="Total Recommended Replenishment"
          value={totalReplenishment}
          format="currency"
        />
      </div>

      {filteredRecords.length === 0 ? (
        <EmptyState message="No ATMs match the selected priority" />
      ) : (
        <ForecastTable data={filteredRecords} />
      )}

      <ScheduleList />
    </div>
  );
}
