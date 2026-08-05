import { useState } from 'react';
import { CheckCircle, XCircle, Clock } from 'lucide-react';

import { Badge } from '@/components/ui/Badge';
import type { BadgeVariant } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { formatIDR } from '@/lib/formatCurrency';
import { useRole } from '@/context/RoleContext';
import type { InvoiceWithVendor } from './useInvoiceData';
import type { InvoiceLineItem } from './invoice.types';

interface InvoiceDetailProps {
  invoice: InvoiceWithVendor;
  onApprove: (invoiceId: string) => void;
}

type MatchStatus = InvoiceLineItem['matchStatus'];

const matchStatusConfig: Record<
  MatchStatus,
  { variant: BadgeVariant; icon: typeof CheckCircle; label: string }
> = {
  Matched: { variant: 'success', icon: CheckCircle, label: 'Matched' },
  Mismatch: { variant: 'danger', icon: XCircle, label: 'Mismatch' },
  'Pending Review': { variant: 'warning', icon: Clock, label: 'Pending Review' },
};

export function InvoiceDetail({ invoice, onApprove }: InvoiceDetailProps) {
  const { role } = useRole();
  const [showConfirmation, setShowConfirmation] = useState(false);

  const isManager = role === 'Manager';
  const canApprove = isManager && invoice.validationStatus === 'Validated';

  function handleApprove() {
    onApprove(invoice.id);
    setShowConfirmation(true);
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-[var(--n-900)]">
          {invoice.id} — {invoice.vendorName}
        </h2>
      </div>

      {/* Maker-checker fields */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <span className="text-xs font-medium uppercase tracking-wider text-[var(--n-500)]">
            Validator
          </span>
          <p className="mt-1 text-sm text-[var(--n-800)]">
            {invoice.validatorName ?? '—'}
          </p>
        </div>
        <div>
          <span className="text-xs font-medium uppercase tracking-wider text-[var(--n-500)]">
            Approver
          </span>
          <p className="mt-1 text-sm text-[var(--n-800)]">
            {invoice.approverName ?? '—'}
          </p>
        </div>
      </div>

      {/* Approve button — visible only for Manager + Validated status */}
      {canApprove && (
        <div>
          <Button variant="primary" onClick={handleApprove}>
            Approve Invoice
          </Button>
        </div>
      )}

      {/* Confirmation message */}
      {showConfirmation && (
        <div className="rounded-md bg-success-bg p-4 text-sm text-success-fg">
          <div className="flex items-center gap-2">
            <CheckCircle className="h-4 w-4 shrink-0" aria-hidden="true" />
            <span>
              Invoice <strong>{invoice.id}</strong> approved by{' '}
              <strong>{invoice.approverName ?? role}</strong> at{' '}
              {invoice.approvedAt
                ? new Date(invoice.approvedAt).toLocaleString('id-ID')
                : new Date().toLocaleString('id-ID')}
            </span>
          </div>
        </div>
      )}

      {/* Line items table */}
      <div className="overflow-x-auto rounded-lg border border-[var(--n-200)]">
        <table className="w-full border-collapse text-sm">
          <thead>
            <tr>
              <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-[var(--n-500)] bg-[var(--n-50)] border-b border-[var(--n-100)]">
                Description
              </th>
              <th className="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-[var(--n-500)] bg-[var(--n-50)] border-b border-[var(--n-100)]">
                Invoiced Amount (IDR)
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-[var(--n-500)] bg-[var(--n-50)] border-b border-[var(--n-100)]">
                Matched Order Ref
              </th>
              <th className="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-[var(--n-500)] bg-[var(--n-50)] border-b border-[var(--n-100)]">
                Expected Amount (IDR)
              </th>
              <th className="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-[var(--n-500)] bg-[var(--n-50)] border-b border-[var(--n-100)]">
                Variance (IDR)
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-[var(--n-500)] bg-[var(--n-50)] border-b border-[var(--n-100)]">
                Match Status
              </th>
            </tr>
          </thead>
          <tbody>
            {invoice.lineItems.map((item) => {
              const config = matchStatusConfig[item.matchStatus];
              return (
                <tr
                  key={item.id}
                  className="border-b border-[var(--n-100)]"
                >
                  <td className="px-4 py-3">{item.description}</td>
                  <td className="px-4 py-3 text-right tabular-nums">
                    {formatIDR(item.invoicedAmount)}
                  </td>
                  <td className="px-4 py-3 font-mono text-xs">
                    {item.matchedOrderRef ?? '—'}
                  </td>
                  <td className="px-4 py-3 text-right tabular-nums">
                    {formatIDR(item.expectedAmount)}
                  </td>
                  <td className="px-4 py-3 text-right tabular-nums">
                    {formatIDR(item.variance)}
                  </td>
                  <td className="px-4 py-3">
                    <Badge
                      variant={config.variant}
                      icon={config.icon}
                      label={config.label}
                    />
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
