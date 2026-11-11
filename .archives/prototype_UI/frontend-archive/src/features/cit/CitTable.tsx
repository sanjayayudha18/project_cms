import { type ColumnDef } from '@tanstack/react-table';
import {
  Calendar,
  Truck,
  CheckCircle,
  XCircle,
  ExternalLink,
} from 'lucide-react';

import { DataTable } from '@/components/ui/DataTable';
import { Badge, type BadgeVariant } from '@/components/ui/Badge';
import { formatIDR } from '@/lib/formatCurrency';

import type { EnrichedCitOrder, CitStatus } from './cit.types';

const statusConfig: Record<
  CitStatus,
  { variant: BadgeVariant; icon: typeof Calendar; label: string }
> = {
  Scheduled: { variant: 'info', icon: Calendar, label: 'Scheduled' },
  'In Transit': { variant: 'warning', icon: Truck, label: 'In Transit' },
  Completed: { variant: 'success', icon: CheckCircle, label: 'Completed' },
  Failed: { variant: 'danger', icon: XCircle, label: 'Failed' },
};

const columns: ColumnDef<EnrichedCitOrder, unknown>[] = [
  {
    accessorKey: 'id',
    header: 'Order ID',
  },
  {
    accessorKey: 'atmId',
    header: 'ATM ID',
  },
  {
    accessorKey: 'vendorName',
    header: 'Vendor',
  },
  {
    accessorKey: 'orderDate',
    header: 'Order Date',
  },
  {
    accessorKey: 'scheduledDate',
    header: 'Scheduled Date',
  },
  {
    accessorKey: 'amount',
    header: 'Amount (IDR)',
    meta: { align: 'right' },
    cell: ({ getValue }) => formatIDR(getValue() as number),
  },
  {
    accessorKey: 'status',
    header: 'Status',
    cell: ({ getValue }) => {
      const status = getValue() as CitStatus;
      const config = statusConfig[status];
      return <Badge variant={config.variant} icon={config.icon} label={config.label} />;
    },
  },
  {
    accessorKey: 'evidenceUrl',
    header: 'Evidence',
    enableSorting: false,
    cell: ({ getValue }) => {
      const url = getValue() as string | null;
      if (!url) {
        return <span className="text-n-400">&mdash;</span>;
      }
      return (
        <a
          href={url}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1 min-h-[44px] min-w-[44px] px-2 text-sm text-red-600 hover:text-red-700 underline"
          aria-label="View evidence (opens in new tab)"
        >
          View
          <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
        </a>
      );
    },
  },
];

interface CitTableProps {
  data: EnrichedCitOrder[];
}

export function CitTable({ data }: CitTableProps) {
  return (
    <DataTable
      data={data}
      columns={columns}
      defaultSorting={[{ id: 'scheduledDate', desc: true }]}
      emptyMessage="No CIT orders match the current filters"
    />
  );
}
