import { type ColumnDef } from '@tanstack/react-table';
import { AlertCircle, AlertTriangle, Minus } from 'lucide-react';

import { Badge } from '@/components/ui/Badge';
import { DataTable } from '@/components/ui/DataTable';
import { formatIDR } from '@/lib/formatCurrency';

import type { EnrichedForecastRecord } from './forecast.types';

const priorityOrder: Record<string, number> = { High: 0, Medium: 1, Low: 2 };

const columns: ColumnDef<EnrichedForecastRecord, unknown>[] = [
  {
    accessorKey: 'atmId',
    header: 'ATM ID',
  },
  {
    accessorKey: 'location',
    header: 'Location',
  },
  {
    accessorKey: 'vendorName',
    header: 'Vendor',
  },
  {
    accessorKey: 'currentBalance',
    header: 'Current Balance',
    meta: { align: 'right' },
    cell: ({ getValue }) => formatIDR(getValue<number>()),
  },
  {
    accessorKey: 'predictedUsageH1',
    header: 'Predicted H+1',
    meta: { align: 'right' },
    cell: ({ getValue }) => formatIDR(getValue<number>()),
  },
  {
    accessorKey: 'predictedUsageH2',
    header: 'Predicted H+2',
    meta: { align: 'right' },
    cell: ({ getValue }) => formatIDR(getValue<number>()),
  },
  {
    accessorKey: 'recommendedReplenishment',
    header: 'Recommended Replenishment',
    meta: { align: 'right' },
    cell: ({ getValue }) => formatIDR(getValue<number>()),
  },
  {
    accessorKey: 'priority',
    header: 'Priority',
    sortingFn: (rowA, rowB) => {
      const a = priorityOrder[rowA.getValue<string>('priority')] ?? 2;
      const b = priorityOrder[rowB.getValue<string>('priority')] ?? 2;
      return a - b;
    },
    cell: ({ getValue }) => {
      const priority = getValue<string>();
      if (priority === 'High') {
        return <Badge variant="danger" icon={AlertCircle} label="High" />;
      }
      if (priority === 'Medium') {
        return <Badge variant="warning" icon={AlertTriangle} label="Medium" />;
      }
      return <Badge variant="neutral" icon={Minus} label="Low" />;
    },
  },
];

interface ForecastTableProps {
  data: EnrichedForecastRecord[];
}

export function ForecastTable({ data }: ForecastTableProps) {
  return (
    <DataTable
      data={data}
      columns={columns}
      defaultSorting={[{ id: 'priority', desc: false }]}
      emptyMessage="No ATMs match the selected priority"
    />
  );
}
