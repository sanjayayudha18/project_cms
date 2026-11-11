import { Upload, CheckCircle, AlertTriangle, XCircle } from 'lucide-react';

import { Badge } from '@/components/ui/Badge';
import type { BadgeVariant } from '@/components/ui/Badge';
import { formatIDR } from '@/lib/formatCurrency';
import type { InvoiceWithVendor } from './useInvoiceData';
import type { Invoice } from './invoice.types';

interface InvoiceListProps {
  data: InvoiceWithVendor[];
  onSelect: (invoiceId: string) => void;
}

type ValidationStatus = Invoice['validationStatus'];

const statusConfig: Record<
  ValidationStatus,
  { variant: BadgeVariant; icon: typeof Upload; label: string }
> = {
  Uploaded: { variant: 'info', icon: Upload, label: 'Uploaded' },
  Validated: { variant: 'warning', icon: AlertTriangle, label: 'Validated' },
  Approved: { variant: 'success', icon: CheckCircle, label: 'Approved' },
  'Mismatch Detected': { variant: 'danger', icon: XCircle, label: 'Mismatch Detected' },
};

export function InvoiceList({ data, onSelect }: InvoiceListProps) {
  return (
    <div className="overflow-x-auto rounded-lg border border-[var(--n-200)]">
      <table className="w-full border-collapse text-sm">
        <thead>
          <tr>
            <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-[var(--n-500)] bg-[var(--n-50)] border-b border-[var(--n-100)]">
              Invoice Number
            </th>
            <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-[var(--n-500)] bg-[var(--n-50)] border-b border-[var(--n-100)]">
              Vendor
            </th>
            <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-[var(--n-500)] bg-[var(--n-50)] border-b border-[var(--n-100)]">
              Period
            </th>
            <th className="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-[var(--n-500)] bg-[var(--n-50)] border-b border-[var(--n-100)]">
              Total Amount (IDR)
            </th>
            <th className="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-[var(--n-500)] bg-[var(--n-50)] border-b border-[var(--n-100)]">
              Line Items
            </th>
            <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-[var(--n-500)] bg-[var(--n-50)] border-b border-[var(--n-100)]">
              Validation Status
            </th>
          </tr>
        </thead>
        <tbody>
          {data.length === 0 ? (
            <tr>
              <td colSpan={6} className="px-4 py-12 text-center text-[var(--n-500)]">
                No invoices available
              </td>
            </tr>
          ) : (
            data.map((invoice) => {
              const config = statusConfig[invoice.validationStatus];
              return (
                <tr
                  key={invoice.id}
                  className="border-b border-[var(--n-100)] cursor-pointer transition-colors duration-100 hover:bg-[var(--red-50)]"
                  onClick={() => onSelect(invoice.id)}
                  role="button"
                  tabIndex={0}
                  aria-label={`View invoice ${invoice.id}`}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      onSelect(invoice.id);
                    }
                  }}
                >
                  <td className="px-4 py-3 font-medium text-[var(--n-900)]">
                    {invoice.id}
                  </td>
                  <td className="px-4 py-3">{invoice.vendorName}</td>
                  <td className="px-4 py-3">{invoice.period}</td>
                  <td className="px-4 py-3 text-right tabular-nums">
                    {formatIDR(invoice.totalAmount)}
                  </td>
                  <td className="px-4 py-3 text-right tabular-nums">
                    {invoice.lineItemsCount}
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
            })
          )}
        </tbody>
      </table>
    </div>
  );
}
