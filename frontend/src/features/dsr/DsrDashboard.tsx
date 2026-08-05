import { useState } from 'react';

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
        <h1 className="text-xl font-semibold text-[var(--n-900)]">DSR Dashboard</h1>
        <div className="flex items-center gap-2">
          <label
            htmlFor="dsr-date"
            className="text-xs font-medium uppercase tracking-wider text-[var(--n-500)]"
          >
            Tanggal
          </label>
          <input
            id="dsr-date"
            type="date"
            value={date}
            onChange={(e) => setDate(e.target.value)}
            className="rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 py-1.5 text-sm text-[var(--n-800)] focus:border-[var(--red-400)] focus:outline-none focus:ring-2 focus:ring-[var(--red-100)]"
          />
        </div>
      </div>

      {isError && (
        <div className="flex items-center gap-2 rounded-[var(--radius-md)] border border-[var(--danger-bg)] bg-[var(--danger-bg)] p-4 text-sm text-[var(--danger-fg)]">
          <span aria-hidden="true">⚠</span>
          <span>Gagal memuat data. Silakan periksa file data.</span>
        </div>
      )}

      {isLoading ? (
        <p className="text-sm text-[var(--n-500)]">Memuat...</p>
      ) : data.length === 0 ? (
        <EmptyState message="Tidak ada data DSR tersedia untuk tanggal ini." />
      ) : (
        <>
          <DsrSummary data={data} />
          <DsrTable data={data} />
        </>
      )}
    </div>
  );
}
