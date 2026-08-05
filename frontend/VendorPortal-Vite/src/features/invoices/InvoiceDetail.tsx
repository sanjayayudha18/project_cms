import { Badge } from '@/components/ui/Badge';
import { formatIDR } from '@/lib/formatters';
import type { InvoiceLineItem } from '@/lib/types';

interface InvoiceDetailProps {
  readonly lineItems: readonly InvoiceLineItem[];
}

const matchStatusBadgeMap: Record<InvoiceLineItem['matchStatus'], 'success' | 'danger' | 'warning'> = {
  Match: 'success',
  Mismatch: 'danger',
  Pending: 'warning',
};

/**
 * Expandable detail section showing invoice line items in a sub-table.
 * Mismatch rows are highlighted with a danger background tint and show variance.
 */
export function InvoiceDetail({ lineItems }: InvoiceDetailProps) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="bg-neutral-50 text-neutral-500 uppercase text-xs tracking-wider">
            <th className="px-4 py-2 text-left font-medium">Description</th>
            <th className="px-4 py-2 text-right font-medium">Invoiced Amount</th>
            <th className="px-4 py-2 text-left font-medium">Matched Order Ref</th>
            <th className="px-4 py-2 text-right font-medium">Expected Amount</th>
            <th className="px-4 py-2 text-right font-medium">Variance</th>
            <th className="px-4 py-2 text-left font-medium">Match Status</th>
          </tr>
        </thead>
        <tbody>
          {lineItems.map((item, index) => {
            const isMismatch = item.matchStatus === 'Mismatch';

            return (
              <tr
                key={`${item.matchedOrderRef}-${index}`}
                className={`border-b border-neutral-100 ${
                  isMismatch ? 'bg-danger-bg/50' : ''
                }`}
              >
                <td className="px-4 py-2">{item.description}</td>
                <td className="px-4 py-2 text-right tabular-nums">
                  {formatIDR(item.invoicedAmount)}
                </td>
                <td className="px-4 py-2 font-mono text-xs">
                  {item.matchedOrderRef}
                </td>
                <td className="px-4 py-2 text-right tabular-nums">
                  {formatIDR(item.expectedAmount)}
                </td>
                <td className="px-4 py-2 text-right tabular-nums">
                  {item.variance !== 0 ? formatIDR(item.variance) : '—'}
                </td>
                <td className="px-4 py-2">
                  <Badge variant={matchStatusBadgeMap[item.matchStatus]}>
                    {item.matchStatus}
                  </Badge>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
