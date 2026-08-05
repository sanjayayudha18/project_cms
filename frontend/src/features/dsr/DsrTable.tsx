import { createColumnHelper } from '@tanstack/react-table';
import { AlertCircle, AlertTriangle, CheckCircle } from 'lucide-react';

import { DataTable } from '@/components/ui/DataTable';
import { Badge, type BadgeVariant } from '@/components/ui/Badge';
import { formatIDR } from '@/lib/utils/formatCurrency';

import type { EnrichedDsrRecord } from './types';

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
    header: 'Lokasi',
    cell: (info) => info.getValue(),
  }),
  columnHelper.accessor('vendorName', {
    header: 'Vendor',
    cell: (info) => info.getValue(),
  }),
  columnHelper.accessor('beginningBalance', {
    header: 'Saldo Awal',
    cell: (info) => formatIDR(info.getValue()),
    meta: { align: 'right' },
  }),
  columnHelper.accessor('cashIn', {
    header: 'Kas Masuk',
    cell: (info) => formatIDR(info.getValue()),
    meta: { align: 'right' },
  }),
  columnHelper.accessor('cashOut', {
    header: 'Kas Keluar',
    cell: (info) => formatIDR(info.getValue()),
    meta: { align: 'right' },
  }),
  columnHelper.accessor('endingBalance', {
    header: 'Saldo Akhir',
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
  readonly data: readonly EnrichedDsrRecord[];
}

export function DsrTable({ data }: DsrTableProps) {
  return (
    <DataTable
      data={data as EnrichedDsrRecord[]}
      columns={columns}
      emptyMessage="Tidak ada data DSR tersedia untuk tanggal ini."
    />
  );
}
