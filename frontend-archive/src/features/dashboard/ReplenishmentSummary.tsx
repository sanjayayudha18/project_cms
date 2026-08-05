/**
 * ReplenishmentSummary — Today's replenishment schedules table for the dashboard.
 * Displays a sorted DataTable with route info, vendor, progress, status, and value.
 *
 * @validates Requirements 4.1, 4.2, 4.3, 4.5, 4.6, 4.7, 12.3, 12.4, 12.5, 12.6
 */
import { Link } from 'react-router-dom';
import { createColumnHelper } from '@tanstack/react-table';

import { DataTable } from '@/components/ui/DataTable';
import { ProgressBar } from '@/components/ui/ProgressBar';
import { Badge } from '@/components/ui/Badge';
import type { BadgeVariant } from '@/components/ui/Badge';
import {
  sortByStatusPriority,
  type ReplenishmentSchedule,
} from '@/features/replenishment/replenishment.utils';
import { formatIDRFull } from '@/lib/formatters';
import schedules from '@/data/replenishment-schedules.json';
import type { ProgressBarProps } from '@/components/ui/ProgressBar';

/** Map schedule status to Badge variant. */
const statusBadgeVariant: Record<ReplenishmentSchedule['status'], BadgeVariant> = {
  'in-transit': 'info',
  completed: 'success',
  delayed: 'warning',
  scheduled: 'neutral',
  'pending-vendor': 'neutral',
};

/** Map schedule status to display label. */
const statusLabel: Record<ReplenishmentSchedule['status'], string> = {
  'in-transit': 'In transit',
  completed: 'Completed',
  delayed: 'Delayed',
  scheduled: 'Scheduled',
  'pending-vendor': 'Pending vendor',
};

/** Map schedule status to ProgressBar-compatible status. */
function toProgressBarStatus(
  status: ReplenishmentSchedule['status'],
): ProgressBarProps['status'] {
  if (status === 'completed') return 'completed';
  if (status === 'delayed') return 'delayed';
  return 'in-transit';
}

const columnHelper = createColumnHelper<ReplenishmentSchedule>();

const columns = [
  columnHelper.accessor('routeCode', {
    header: 'Route',
    cell: (info) => (
      <div>
        <span className="font-bold text-[var(--n-900)]">{info.getValue()}</span>
        <span className="block text-xs text-[var(--n-500)]">
          {info.row.original.region}
        </span>
      </div>
    ),
  }),
  columnHelper.accessor('vendor', {
    header: 'Vendor',
    cell: (info) => (
      <span className="text-[var(--n-700)]">{info.getValue()}</span>
    ),
  }),
  columnHelper.accessor('machineCount', {
    header: 'Machines',
    cell: (info) => (
      <span className="tabular-nums text-[var(--n-800)]">{info.getValue()}</span>
    ),
  }),
  columnHelper.display({
    id: 'progress',
    header: 'Progress',
    cell: (info) => (
      <ProgressBar
        completed={info.row.original.completionCount}
        total={info.row.original.machineCount}
        status={toProgressBarStatus(info.row.original.status)}
      />
    ),
  }),
  columnHelper.accessor('status', {
    header: 'Status',
    cell: (info) => {
      const status = info.getValue();
      return <Badge variant={statusBadgeVariant[status]} label={statusLabel[status]} />;
    },
  }),
  columnHelper.accessor('cashValue', {
    header: 'Value',
    cell: (info) => (
      <span className="tabular-nums font-medium text-[var(--n-900)]">
        {formatIDRFull(info.getValue())}
      </span>
    ),
    meta: { align: 'right' },
  }),
];

/** Sorted schedule data for the dashboard table. */
const sortedSchedules = sortByStatusPriority(
  schedules as ReplenishmentSchedule[],
);

export function ReplenishmentSummary() {
  return (
    <section aria-labelledby="replenishment-summary-heading">
      <div className="mb-4 flex items-center justify-between">
        <h2
          id="replenishment-summary-heading"
          className="text-lg font-semibold text-[var(--n-900)]"
        >
          Today&apos;s replenishment
        </h2>
        <Link
          to="/replenishment"
          className="text-sm font-medium text-[var(--red-600)] hover:text-[var(--red-700)] transition-colors duration-100"
        >
          View all &rarr;
        </Link>
      </div>
      <DataTable data={sortedSchedules} columns={columns} />
    </section>
  );
}
