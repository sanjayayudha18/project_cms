import { useMemo } from 'react';
import { Card } from '@/components/ui/Card';
import { Badge } from '@/components/ui/Badge';
import { formatIDR, getBalanceStatus } from '@/lib/formatters';
import type { DsrRecord } from '@/lib/types';
import { Monitor } from 'lucide-react';

interface DsrSummaryCardProps {
  readonly records: readonly DsrRecord[];
}

/**
 * Summary card for DSR Monitor showing:
 * - Total ATMs monitored (distinct ATM IDs)
 * - Critical count (ending balance < 50M)
 * - Low count (ending balance 50M–150M)
 * - Total Ending Balance (sum)
 */
export function DsrSummaryCard({ records }: DsrSummaryCardProps) {
  const summary = useMemo(() => {
    const atmIds = new Set(records.map((r) => r.atmId));
    let criticalCount = 0;
    let lowCount = 0;
    let totalEndingBalance = 0;

    for (const record of records) {
      const status = getBalanceStatus(record.endingBalance);
      if (status === 'Critical') criticalCount++;
      if (status === 'Low') lowCount++;
      totalEndingBalance += record.endingBalance;
    }

    return {
      totalAtms: atmIds.size,
      criticalCount,
      lowCount,
      totalEndingBalance,
    };
  }, [records]);

  return (
    <Card className="flex flex-wrap items-center gap-6">
      <div className="flex items-center gap-2">
        <Monitor className="size-5 text-neutral-500" aria-hidden="true" />
        <div>
          <p className="text-xs font-medium uppercase tracking-wide text-neutral-500">
            Total ATMs
          </p>
          <p className="text-lg font-semibold text-surface-text tabular-nums">
            {summary.totalAtms}
          </p>
        </div>
      </div>

      <div className="flex items-center gap-2">
        <Badge variant="danger">Critical</Badge>
        <span className="text-lg font-semibold text-surface-text tabular-nums">
          {summary.criticalCount}
        </span>
      </div>

      <div className="flex items-center gap-2">
        <Badge variant="warning">Low</Badge>
        <span className="text-lg font-semibold text-surface-text tabular-nums">
          {summary.lowCount}
        </span>
      </div>

      <div>
        <p className="text-xs font-medium uppercase tracking-wide text-neutral-500">
          Total Ending Balance
        </p>
        <p className="text-lg font-semibold text-surface-text tabular-nums">
          {formatIDR(summary.totalEndingBalance)}
        </p>
      </div>
    </Card>
  );
}
