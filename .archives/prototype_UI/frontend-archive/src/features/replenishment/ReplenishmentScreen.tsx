import { useMemo, useState } from 'react';
import { createColumnHelper } from '@tanstack/react-table';
import { Search, Truck, CheckCircle2, AlertTriangle, Clock } from 'lucide-react';

import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable } from '@/components/ui/DataTable';
import { Badge } from '@/components/ui/Badge';
import type { BadgeVariant } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { FilterSelect } from '@/components/ui/FilterSelect';
import { formatIDRFull } from '@/lib/formatters';
import { useToast } from '@/context/ToastContext';
import {
  filterSchedules,
  sortByStatusPriority,
  type ReplenishmentSchedule,
} from './replenishment.utils';
import schedulesData from '@/data/replenishment-schedules.json';

const schedules = schedulesData as ReplenishmentSchedule[];

const STATUS_CONFIG: Record<
  ReplenishmentSchedule['status'],
  { label: string; variant: BadgeVariant; icon: typeof Truck }
> = {
  'in-transit': { label: 'In transit', variant: 'info', icon: Truck },
  completed: { label: 'Completed', variant: 'success', icon: CheckCircle2 },
  delayed: { label: 'Delayed', variant: 'warning', icon: AlertTriangle },
  'pending-vendor': { label: 'Pending vendor', variant: 'warning', icon: Clock },
  scheduled: { label: 'Scheduled', variant: 'neutral', icon: Clock },
};

const columnHelper = createColumnHelper<ReplenishmentSchedule>();

const columns = [
  columnHelper.accessor('id', {
    header: 'Schedule',
    cell: (info) => (
      <div>
        <span className="font-semibold text-n-900">{info.getValue()}</span>
        <span className="block text-xs text-n-500">{info.row.original.routeCode}</span>
      </div>
    ),
  }),
  columnHelper.accessor('region', {
    header: 'Region',
    cell: (info) => info.getValue(),
  }),
  columnHelper.accessor('vendor', {
    header: 'Vendor',
    cell: (info) => info.getValue(),
  }),
  columnHelper.display({
    id: 'window',
    header: 'Window',
    cell: ({ row }) => `${row.original.windowStart}\u2013${row.original.windowEnd}`,
  }),
  columnHelper.accessor('machineCount', {
    header: 'Machines',
    cell: (info) => info.getValue(),
  }),
  columnHelper.accessor('status', {
    header: 'Status',
    cell: (info) => {
      const cfg = STATUS_CONFIG[info.getValue()];
      return <Badge variant={cfg.variant} icon={cfg.icon} label={cfg.label} />;
    },
  }),
  columnHelper.accessor('cashValue', {
    header: 'Cash value',
    cell: (info) => formatIDRFull(info.getValue()),
    meta: { align: 'right' },
  }),
];

/**
 * Replenishment schedules screen.
 * Displays filterable table of cash replenishment schedules with region/vendor filters.
 *
 * @validates Requirements 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 6.7, 6.8, 6.9, 6.10, 6.11, 8.7, 9.4, 12.1, 12.3, 12.4, 12.5, 12.6
 */
export function ReplenishmentScreen() {
  const { showToast } = useToast();
  const [regionFilter, setRegionFilter] = useState<string | null>(null);
  const [vendorFilter, setVendorFilter] = useState<string | null>(null);

  const regions = useMemo(
    () =>
      Array.from(new Set(schedules.map((s) => s.region)))
        .sort()
        .map((r) => ({ value: r, label: r })),
    [],
  );

  const vendors = useMemo(
    () =>
      Array.from(new Set(schedules.map((s) => s.vendor)))
        .sort()
        .map((v) => ({ value: v, label: v })),
    [],
  );

  const todayLabel = useMemo(() => {
    const d = new Date();
    return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' });
  }, []);

  const filteredData = useMemo(() => {
    const filtered = filterSchedules(
      schedules,
      regionFilter ?? 'All regions',
      vendorFilter ?? 'All vendors',
    );
    return sortByStatusPriority(filtered);
  }, [regionFilter, vendorFilter]);

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Cash operations"
        title="Replenishment schedules"
        description="Monitor daily CIT routes, vendor assignments, and schedule adherence across all regions."
        actions={
          <>
            <Button variant="secondary">Import plan</Button>
            <Button
              variant="primary"
              onClick={() => showToast('Schedule created successfully', 'success')}
            >
              New schedule
            </Button>
          </>
        }
      />

      {/* Filter toolbar */}
      <div className="flex flex-wrap items-end gap-4">
        <FilterSelect
          label="Date"
          options={[{ value: todayLabel, label: todayLabel }]}
          value={todayLabel}
          onChange={() => {}}
          placeholder={todayLabel}
        />
        <FilterSelect
          label="Region"
          options={regions}
          value={regionFilter}
          onChange={setRegionFilter}
          placeholder="All regions"
        />
        <FilterSelect
          label="Vendor"
          options={vendors}
          value={vendorFilter}
          onChange={setVendorFilter}
          placeholder="All vendors"
        />
        <span className="text-sm text-n-500 self-end pb-3">
          {filteredData.length} schedules
        </span>
      </div>

      {/* Data table or empty state */}
      {filteredData.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-center">
          <Search className="h-12 w-12 text-n-300 mb-3" aria-hidden="true" />
          <h3 className="text-base font-semibold text-n-800 mb-1">No schedules found</h3>
          <p className="text-sm text-n-500">Try a route number, vendor, or region.</p>
        </div>
      ) : (
        <div className="overflow-x-auto -mx-4 px-4 md:mx-0 md:px-0">
          <DataTable data={filteredData} columns={columns} />
        </div>
      )}
    </div>
  );
}
