import { createColumnHelper } from '@tanstack/react-table';
import { AlertCircle, AlertTriangle, CheckCircle } from 'lucide-react';
import { DataTable } from '@/components/ui/DataTable';
import { Badge, type BadgeVariant } from '@/components/ui/Badge';
import { formatIDR } from '@/lib/formatCurrency';
import type { EnrichedDsrRecord } from './dsr.types';

const columnHelper = createColumnHelper<EnrichedDsrRecord>();

const statusConfig: Record<
  EnrichedDsrRecord['status'],
  { variant: BadgeVariant; icon: typeof AlertCircle }
> = {
  Critical: { variant: 'danger', icon: AlertCircle },
  Low: { variant: 'warning', icon: AlertTriangle },
  Normal: { variant: 'success', icon: CheckCircle },
};

const columns = [
  columnHelper.accessor('atmId', {
    header: 'ATM ID',
    cell: (info) => info.getValue(),
  }),
  columnHelper.accessor('location', {
    header: 'Location',
    cell: (info) => info.getValue(),
  }),
  columnHelper.accessor('vendorName', {
    header: 'Vendor',
    cell: (info) => info.getValue(),
  }),
  columnHelper.accessor('beginningBalance', {
    header: 'Beginning Balance',
    cell: (info) => formatIDR(info.getValue()),
    meta: { align: 'right' },
  }),
  columnHelper.accessor('cashIn', {
    header: 'Cash In',
    cell: (info) => formatIDR(info.getValue()),
    meta: { align: 'right' },
  }),
  columnHelper.accessor('cashOut', {
    header: 'Cash Out',
    cell: (info) => formatIDR(info.getValue()),
    meta: { align: 'right' },
  }),
  columnHelper.accessor('endingBalance', {
    header: 'Ending Balance',
    cell: (info) => formatIDR(info.getValue()),
    meta: { align: 'right' },
  }),
  columnHelper.accessor('status', {
    header: 'Status',
    cell: (info) => {
      const status = info.getValue();
      const config = statusConfig[status];
      return <Badge variant={config.variant} icon={config.icon} label={status} />;
    },
  }),
];

interface DsrTableProps {
  data: EnrichedDsrRecord[];
}

export function DsrTable({ data }: DsrTableProps) {
  return (
    <DataTable
      data={data}
      columns={columns}
      emptyMessage="No DSR data available for this date."
    />
  );
}
