import { useState, useMemo } from 'react';

import { FilterSelect } from '@/components/ui/FilterSelect';
import { EmptyState } from '@/components/ui/EmptyState';
import { compoundFilter } from '@/lib/filters';
import vendorsData from '@/data/vendors.json';

import { useCitData } from './useCitData';
import { CitTable } from './CitTable';
import { CitSummary } from './CitSummary';
import type { CitStatus } from './cit.types';

const statusOptions: { value: string; label: string }[] = [
  { value: 'Scheduled', label: 'Scheduled' },
  { value: 'In Transit', label: 'In Transit' },
  { value: 'Completed', label: 'Completed' },
  { value: 'Failed', label: 'Failed' },
];

const vendorOptions = vendorsData.map((v) => ({
  value: v.id,
  label: v.name,
}));

export function CitTracker() {
  const { data: orders = [] } = useCitData();
  const [statusFilter, setStatusFilter] = useState<CitStatus | null>(null);
  const [vendorFilter, setVendorFilter] = useState<string | null>(null);

  const filteredOrders = useMemo(
    () =>
      compoundFilter(orders, {
        status: statusFilter,
        vendorId: vendorFilter,
      }),
    [orders, statusFilter, vendorFilter],
  );

  return (
    <div className="flex flex-col gap-6 p-6">
      <h1 className="text-xl font-semibold text-n-900">CIT Tracker</h1>

      <div className="flex flex-wrap items-end gap-4">
        <FilterSelect
          label="Status"
          options={statusOptions}
          value={statusFilter}
          onChange={(val) => setStatusFilter(val as CitStatus | null)}
        />
        <FilterSelect
          label="Vendor"
          options={vendorOptions}
          value={vendorFilter}
          onChange={setVendorFilter}
        />
      </div>

      <CitSummary data={filteredOrders} />

      {filteredOrders.length === 0 ? (
        <EmptyState message="No CIT orders match the current filters" />
      ) : (
        <CitTable data={filteredOrders} />
      )}
    </div>
  );
}
