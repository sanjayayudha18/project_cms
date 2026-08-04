import { useMemo, useState } from 'react';
import { createColumnHelper } from '@tanstack/react-table';
import { TriangleAlert, AlertCircle, Search } from 'lucide-react';

import { PageHeader } from '@/components/ui/PageHeader';
import { NoticeBanner } from '@/components/ui/NoticeBanner';
import { DataTable } from '@/components/ui/DataTable';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { FilterSelect } from '@/components/ui/FilterSelect';
import { formatIDRFull, formatDifference } from '@/lib/formatters';
import { filterExceptions } from './reconciliation.utils';
import type { ReconciliationException } from './reconciliation.utils';
import { useToast } from '@/context/ToastContext';
import exceptions from '@/data/reconciliation-exceptions.json';

const data = exceptions as ReconciliationException[];

const exceptionTypeOptions = [
  { value: 'Open exceptions', label: 'Open exceptions' },
  { value: 'All records', label: 'All records' },
  { value: 'Resolved', label: 'Resolved' },
];

const severityOptions = [
  { value: 'All severity', label: 'All severity' },
  { value: 'High', label: 'High' },
  { value: 'Medium', label: 'Medium' },
];

const columnHelper = createColumnHelper<ReconciliationException>();

function formatTime(isoString: string): string {
  const date = new Date(isoString);
  return date.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', hour12: false });
}

const columns = [
  columnHelper.accessor('atmId', {
    header: 'Machine',
    cell: (info) => (
      <div>
        <span className="font-semibold text-n-900">{info.getValue()}</span>
        <span className="block text-xs text-n-500">{formatTime(info.row.original.lastCountTime)}</span>
      </div>
    ),
  }),
  columnHelper.accessor('location', {
    header: 'Location',
    cell: (info) => <span className="text-n-800">{info.getValue()}</span>,
  }),
  columnHelper.accessor('countedAmount', {
    header: 'Counted',
    meta: { align: 'right' },
    cell: (info) => (
      <span className="tabular-nums">{formatIDRFull(info.getValue())}</span>
    ),
  }),
  columnHelper.accessor('escrowAmount', {
    header: 'Escrow',
    meta: { align: 'right' },
    cell: (info) => (
      <span className="tabular-nums">{formatIDRFull(info.getValue())}</span>
    ),
  }),
  columnHelper.accessor('difference', {
    header: 'Difference',
    meta: { align: 'right' },
    cell: (info) => {
      const { text, colorClass } = formatDifference(info.getValue());
      return <span className={`tabular-nums font-medium ${colorClass}`}>{text}</span>;
    },
  }),
  columnHelper.accessor('severity', {
    header: 'Severity',
    cell: (info) => {
      const severity = info.getValue();
      return (
        <Badge
          variant={severity === 'high' ? 'danger' : 'warning'}
          icon={severity === 'high' ? AlertCircle : TriangleAlert}
          label={severity.charAt(0).toUpperCase() + severity.slice(1)}
        />
      );
    },
  }),
  columnHelper.accessor('owner', {
    header: 'Owner',
    cell: (info) => {
      const owner = info.getValue();
      return owner ? (
        <span className="text-n-800">{owner}</span>
      ) : (
        <span className="text-n-400 italic">Unassigned</span>
      );
    },
  }),
];

/**
 * Reconciliation screen — financial control exception management.
 * Displays a filterable table of reconciliation exceptions with severity badges,
 * a warning banner for unresolved high-severity items, and action buttons.
 *
 * @validates Requirements 7.1, 7.2, 7.3, 7.4, 7.5, 7.6, 7.7, 7.8, 7.9, 7.10, 7.11, 7.12, 8.7, 9.5, 12.1, 12.3, 12.4, 12.5, 12.6
 */
export function ReconciliationScreen() {
  const { showToast } = useToast();
  const [exceptionType, setExceptionType] = useState<string | null>('Open exceptions');
  const [severity, setSeverity] = useState<string | null>('All severity');

  const filtered = useMemo(
    () => filterExceptions(data, severity ?? 'All severity', exceptionType ?? 'All records'),
    [severity, exceptionType],
  );

  const unresolvedHighCount = useMemo(
    () => data.filter((r) => r.severity === 'high' && r.owner === null).length,
    [],
  );

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Financial control"
        title="Reconciliation"
        description="Compare physical cash counts against escrow records and resolve discrepancies before end-of-day cutoff."
        actions={
          <>
            <Button variant="secondary">Audit trail</Button>
            <Button variant="primary" onClick={() => showToast('Reconciliation initiated', 'success')}>
              Run reconciliation
            </Button>
          </>
        }
      />

      <NoticeBanner
        icon={TriangleAlert}
        title="Cutoff at 14:00 WIB"
        description={`${unresolvedHighCount} high-severity exception${unresolvedHighCount !== 1 ? 's' : ''} remain${unresolvedHighCount === 1 ? 's' : ''} unresolved and require${unresolvedHighCount === 1 ? 's' : ''} operator review.`}
        variant="warning"
      />

      <div className="flex flex-wrap items-end gap-4">
        <FilterSelect
          label="Exception type"
          options={exceptionTypeOptions}
          value={exceptionType}
          onChange={setExceptionType}
          placeholder="All records"
        />
        <FilterSelect
          label="Severity"
          options={severityOptions}
          value={severity}
          onChange={setSeverity}
          placeholder="All severity"
        />
        <div className="flex items-center gap-2 min-h-[44px] text-sm text-n-500">
          <Search className="h-4 w-4" aria-hidden="true" />
          <span>{filtered.length} exception{filtered.length !== 1 ? 's' : ''}</span>
        </div>
      </div>

      <div className="overflow-x-auto -mx-4 px-4 md:mx-0 md:px-0">
        <DataTable
          data={filtered}
          columns={columns}
          emptyMessage="No exceptions match the current filters."
        />
      </div>
    </div>
  );
}
