import { SummaryCard } from '@/components/ui/SummaryCard';
import type { EnrichedDsrRecord } from './dsr.types';

interface DsrSummaryProps {
  data: EnrichedDsrRecord[];
}

export function DsrSummary({ data }: DsrSummaryProps) {
  const totals = data.reduce(
    (acc, record) => ({
      beginningBalance: acc.beginningBalance + record.beginningBalance,
      cashIn: acc.cashIn + record.cashIn,
      cashOut: acc.cashOut + record.cashOut,
      endingBalance: acc.endingBalance + record.endingBalance,
    }),
    { beginningBalance: 0, cashIn: 0, cashOut: 0, endingBalance: 0 },
  );

  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <SummaryCard label="Total Beginning Balance" value={totals.beginningBalance} format="currency" />
      <SummaryCard label="Total Cash In" value={totals.cashIn} format="currency" />
      <SummaryCard label="Total Cash Out" value={totals.cashOut} format="currency" />
      <SummaryCard label="Total Ending Balance" value={totals.endingBalance} format="currency" />
    </div>
  );
}
