import { useMemo } from 'react';

import { formatIDR } from '@/lib/utils/formatCurrency';

import type { DsrRecord, DsrSummaryTotals } from './types';

interface DsrSummaryProps {
  readonly data: readonly DsrRecord[];
}

/** Pure aggregation logic — exported for unit/property testing. */
export function computeDsrTotals(records: readonly DsrRecord[]): DsrSummaryTotals {
  return records.reduce(
    (acc, record) => ({
      beginningBalance: acc.beginningBalance + record.beginningBalance,
      cashIn: acc.cashIn + record.cashIn,
      cashOut: acc.cashOut + record.cashOut,
      endingBalance: acc.endingBalance + record.endingBalance,
    }),
    { beginningBalance: 0, cashIn: 0, cashOut: 0, endingBalance: 0 },
  );
}

export function DsrSummary({ data }: DsrSummaryProps) {
  const totals: DsrSummaryTotals = useMemo(() => computeDsrTotals(data), [data]);

  const cards: { label: string; value: number }[] = [
    { label: 'Total Saldo Awal', value: totals.beginningBalance },
    { label: 'Total Kas Masuk', value: totals.cashIn },
    { label: 'Total Kas Keluar', value: totals.cashOut },
    { label: 'Total Saldo Akhir', value: totals.endingBalance },
  ];

  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
      {cards.map((card) => (
        <div
          key={card.label}
          className="rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-0)] p-4"
        >
          <p className="text-xs font-medium uppercase tracking-wider text-[var(--n-500)]">
            {card.label}
          </p>
          <p className="mt-1 text-lg font-semibold tabular-nums text-[var(--n-900)]">
            {formatIDR(card.value)}
          </p>
        </div>
      ))}
    </div>
  );
}
