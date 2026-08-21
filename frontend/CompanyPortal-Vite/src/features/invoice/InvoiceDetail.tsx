import { CheckCircle, Clock, XCircle } from "lucide-react";

import { Badge } from "@/components/ui/Badge";
import type { BadgeVariant } from "@/components/ui/Badge";
import { formatIDR } from "@/lib/utils/formatCurrency";
import type { InvoiceLineItem, MatchStatus } from "./types";

interface InvoiceDetailProps {
  lineItems: InvoiceLineItem[];
}

const matchStatusConfig: Record<
  MatchStatus,
  { variant: BadgeVariant; icon: typeof CheckCircle; label: string }
> = {
  Matched: { variant: "success", icon: CheckCircle, label: "Cocok" },
  Mismatch: { variant: "danger", icon: XCircle, label: "Tidak Cocok" },
  "Pending Review": { variant: "warning", icon: Clock, label: "Menunggu Review" },
};

/**
 * Expandable row detail showing invoice line items.
 * Displays: description, invoiced amount, matched order reference,
 * expected amount, variance, match status.
 */
export function InvoiceDetail({ lineItems }: InvoiceDetailProps) {
  return (
    <div className="overflow-x-auto bg-[var(--n-50)] px-6 py-4">
      <table className="w-full border-collapse text-sm">
        <thead>
          <tr>
            <th className="px-3 py-2 text-left text-xs font-medium uppercase tracking-wider text-[var(--n-500)]">
              Deskripsi
            </th>
            <th className="px-3 py-2 text-right text-xs font-medium uppercase tracking-wider text-[var(--n-500)]">
              Jumlah Ditagih (Rp)
            </th>
            <th className="px-3 py-2 text-left text-xs font-medium uppercase tracking-wider text-[var(--n-500)]">
              Ref. Order
            </th>
            <th className="px-3 py-2 text-right text-xs font-medium uppercase tracking-wider text-[var(--n-500)]">
              Jumlah Ekspektasi (Rp)
            </th>
            <th className="px-3 py-2 text-right text-xs font-medium uppercase tracking-wider text-[var(--n-500)]">
              Selisih (Rp)
            </th>
            <th className="px-3 py-2 text-left text-xs font-medium uppercase tracking-wider text-[var(--n-500)]">
              Status Pencocokan
            </th>
          </tr>
        </thead>
        <tbody>
          {lineItems.map((item) => {
            const config = matchStatusConfig[item.matchStatus];
            return (
              <tr key={item.id} className="border-b border-[var(--n-100)]">
                <td className="px-3 py-2 text-[var(--n-800)]">{item.description}</td>
                <td className="px-3 py-2 text-right tabular-nums text-[var(--n-800)]">
                  {formatIDR(item.invoicedAmount)}
                </td>
                <td className="px-3 py-2 font-mono text-xs text-[var(--n-700)]">
                  {item.matchedOrderRef ?? "—"}
                </td>
                <td className="px-3 py-2 text-right tabular-nums text-[var(--n-800)]">
                  {formatIDR(item.expectedAmount)}
                </td>
                <td className="px-3 py-2 text-right tabular-nums text-[var(--n-800)]">
                  {formatIDR(item.variance)}
                </td>
                <td className="px-3 py-2">
                  <Badge variant={config.variant} icon={config.icon} label={config.label} />
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
