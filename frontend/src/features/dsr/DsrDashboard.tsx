import { useState } from 'react';
import { DatePicker } from '@/components/ui/DatePicker';
import { EmptyState } from '@/components/ui/EmptyState';
import { DsrSummary } from './DsrSummary';
import { DsrTable } from './DsrTable';
import { useDsrData } from './useDsrData';

const DEFAULT_DATE = '2024-01-15';

export function DsrDashboard() {
  const [date, setDate] = useState(DEFAULT_DATE);
  const { data = [], isLoading, isError } = useDsrData(date);

  return (
    <div className="flex flex-col gap-6 p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold text-n-900">DSR Dashboard</h1>
        <DatePicker value={date} onChange={setDate} label="Date" />
      </div>

      {isError && (
        <div className="rounded-lg border border-danger-bg bg-danger-bg p-4 text-sm text-danger-fg">
          Unable to load data. Please check mock data files.
        </div>
      )}

      {isLoading ? (
        <p className="text-sm text-n-500">Loading...</p>
      ) : data.length === 0 ? (
        <EmptyState message="No DSR data available for this date." />
      ) : (
        <>
          <DsrSummary data={data} />
          <DsrTable data={data} />
        </>
      )}
    </div>
  );
}
