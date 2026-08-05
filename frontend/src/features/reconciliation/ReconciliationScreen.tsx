import { useMemo, useState } from 'react';
import { createColumnHelper, type ColumnDef } from '@tanstack/react-table';
import { TriangleAlert, AlertCircle, Search } from 'lucide-react';

import { PageHeader } from '@/components/ui/PageHeader';
import { NoticeBanner } from '@/components/ui/NoticeBanner';
import { DataTable } from '@/components/ui/DataTable';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { FilterSelect } from '@/components/ui/FilterSelect';
import { EmptyState } from '@/components/ui/EmptyState';
import { formatIDRFull, formatDifference } from '@/lib/utils/formatters';
import { useToast } from '@/lib/hooks/useToast';
import { filterExceptions } from './reconciliation.utils';
import type { ReconciliationException } from './types';
import exceptions from '@/data/reconciliation-exceptions.json';

const data = exceptions as ReconciliationException[];

const exceptionTypeOptions = [
  { value: 'Open exceptions', label: 'Exception terbuka' },
  { value: 'All records', label: 'Semua catatan' },
  { value: 'Resolved', label: 'Terselesaikan' },
];

const severityOptions = [
  { value: 'All severity', label: 'Semua tingkat' },
  { value: 'High', label: 'Tinggi' },
  { value: 'Medium', label: 'Sedang' },
];

const columnHelper = createColumnHelper<ReconciliationException>();

function formatTime(isoString: string): string {
  const date = new Date(isoString);
  return date.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit', hour12: false });
}

const columns = [
  columnHelper.accessor('atmId', {
    header: 'Mesin',
    cell: (info) => (
      <div>
        <span className="font-semibold text-[var(--n-900)]">{info.getValue()}</span>
        <span className="block text-xs text-[var(--n-500)]">{formatTime(info.row.original.lastCountTime)}</span>
      </div>
    ),
  }),
  columnHelper.accessor('location', {
    header: 'Lokasi',
    cell: (info) => <span className="text-[var(--n-800)]">{info.getValue()}</span>,
  }),
  columnHelper.accessor('countedAmount', {
    header: 'Terhitung',
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
    header: 'Selisih',
    meta: { align: 'right' },
    cell: (info) => {
      const { text, colorClass } = formatDifference(info.getValue());
      return <span className={`tabular-nums font-medium ${colorClass}`}>{text}</span>;
    },
  }),
  columnHelper.accessor('severity', {
    header: 'Tingkat',
    cell: (info) => {
      const severity = info.getValue();
      return (
        <Badge
          variant={severity === 'high' ? 'danger' : 'warning'}
          icon={severity === 'high' ? AlertCircle : TriangleAlert}
          label={severity === 'high' ? 'Tinggi' : 'Sedang'}
        />
      );
    },
  }),
  columnHelper.accessor('owner', {
    header: 'Penanggung Jawab',
    cell: (info) => {
      const owner = info.getValue();
      return owner ? (
        <span className="text-[var(--n-800)]">{owner}</span>
      ) : (
        <span className="text-[var(--n-400)] italic">Belum ditugaskan</span>
      );
    },
  }),
] as ColumnDef<ReconciliationException, unknown>[];

/**
 * Reconciliation screen — financial control exception management.
 * Displays a filterable table of reconciliation exceptions with severity badges,
 * a warning banner for unresolved high-severity items, and action buttons.
 */
export function ReconciliationScreen() {
  const { toast } = useToast();
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

  if (data.length === 0) {
    return (
      <div className="flex flex-col gap-[var(--space-6)]">
        <PageHeader
          eyebrow="Kontrol keuangan"
          title="Rekonsiliasi"
          description="Bandingkan perhitungan kas fisik dengan catatan escrow dan selesaikan perbedaan sebelum batas waktu akhir hari."
        />
        <EmptyState message="Tidak ada exception rekonsiliasi yang tersedia." />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-[var(--space-6)]">
      <PageHeader
        eyebrow="Kontrol keuangan"
        title="Rekonsiliasi"
        description="Bandingkan perhitungan kas fisik dengan catatan escrow dan selesaikan perbedaan sebelum batas waktu akhir hari."
        actions={
          <>
            <Button variant="secondary">Jejak audit</Button>
            <Button variant="primary" onClick={() => toast({ type: 'success', message: 'Rekonsiliasi dimulai' })}>
              Jalankan rekonsiliasi
            </Button>
          </>
        }
      />

      <NoticeBanner
        icon={TriangleAlert}
        title="Batas waktu 14:00 WIB"
        description={`${unresolvedHighCount} exception tingkat tinggi belum terselesaikan dan memerlukan peninjauan operator.`}
        variant="warning"
      />

      <div className="flex flex-wrap items-end gap-4">
        <FilterSelect
          label="Tipe exception"
          options={exceptionTypeOptions}
          value={exceptionType}
          onChange={setExceptionType}
          placeholder="Semua catatan"
        />
        <FilterSelect
          label="Tingkat"
          options={severityOptions}
          value={severity}
          onChange={setSeverity}
          placeholder="Semua tingkat"
        />
        <div className="flex items-center gap-2 min-h-[44px] text-sm text-[var(--n-500)]">
          <Search className="h-4 w-4" aria-hidden="true" />
          <span>{filtered.length} exception{filtered.length !== 1 ? '' : ''}</span>
        </div>
      </div>

      <DataTable
        data={filtered}
        columns={columns}
        emptyMessage="Tidak ada exception yang sesuai dengan filter saat ini."
      />
    </div>
  );
}
