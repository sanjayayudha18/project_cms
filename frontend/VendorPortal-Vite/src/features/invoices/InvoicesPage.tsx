import { Badge } from "@/components/ui/Badge";
import { Card } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { InvoiceDetail } from "@/features/invoices/InvoiceDetail";
import { useInvoices } from "@/features/invoices/useInvoices";
import { formatIDR } from "@/lib/formatters";
import type { Invoice } from "@/lib/types";
import { CheckCircle, ChevronDown, ChevronRight, FileText } from "lucide-react";
import { useCallback, useMemo, useState } from "react";

type ValidationStatus = Invoice["validationStatus"];

const statusBadgeMap: Record<ValidationStatus, "info" | "warning" | "danger" | "success"> = {
  Uploaded: "info",
  Validated: "warning",
  "Mismatch Detected": "danger",
  Approved: "success",
};

export function InvoicesPage() {
  const { data: invoices = [], isLoading } = useInvoices();
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set());

  const toggleExpanded = useCallback((id: string) => {
    setExpandedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  const handleRowKeyDown = useCallback(
    (e: React.KeyboardEvent, id: string) => {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        toggleExpanded(id);
      }
    },
    [toggleExpanded],
  );

  // Summary computations
  const summary = useMemo(() => {
    const totalInvoiced = invoices.reduce((sum, inv) => sum + inv.totalAmount, 0);
    const countByStatus: Record<ValidationStatus, number> = {
      Uploaded: 0,
      Validated: 0,
      "Mismatch Detected": 0,
      Approved: 0,
    };
    let approvedSum = 0;

    for (const inv of invoices) {
      countByStatus[inv.validationStatus]++;
      if (inv.validationStatus === "Approved") {
        approvedSum += inv.totalAmount;
      }
    }

    return { totalInvoiced, countByStatus, approvedSum };
  }, [invoices]);

  if (isLoading) {
    return (
      <div className="flex flex-col gap-6">
        <h1 className="text-xl font-semibold text-surface-text">Invoices</h1>
        <p className="text-sm text-neutral-500">Memuat data...</p>
      </div>
    );
  }

  if (invoices.length === 0) {
    return (
      <div className="flex flex-col gap-6">
        <h1 className="text-xl font-semibold text-surface-text">Invoices</h1>
        <EmptyState
          icon={FileText}
          title="Belum ada invoice"
          description="Tidak ada invoice yang tersedia untuk vendor ini."
        />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-xl font-semibold text-surface-text">Invoices</h1>

      {/* Summary Section */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <Card>
          <p className="text-xs uppercase tracking-wider text-neutral-500 font-medium">
            Total Invoiced
          </p>
          <p className="mt-1 text-lg font-semibold tabular-nums text-surface-text">
            {formatIDR(summary.totalInvoiced)}
          </p>
        </Card>
        <Card>
          <p className="text-xs uppercase tracking-wider text-neutral-500 font-medium">
            Approved Amount
          </p>
          <p className="mt-1 text-lg font-semibold tabular-nums text-success-fg">
            {formatIDR(summary.approvedSum)}
          </p>
        </Card>
        <Card>
          <p className="text-xs uppercase tracking-wider text-neutral-500 font-medium">By Status</p>
          <div className="mt-1 flex flex-wrap gap-2">
            <Badge variant="info">{summary.countByStatus.Uploaded} Uploaded</Badge>
            <Badge variant="warning">{summary.countByStatus.Validated} Validated</Badge>
          </div>
        </Card>
        <Card>
          <p className="text-xs uppercase tracking-wider text-neutral-500 font-medium">By Status</p>
          <div className="mt-1 flex flex-wrap gap-2">
            <Badge variant="danger">{summary.countByStatus["Mismatch Detected"]} Mismatch</Badge>
            <Badge variant="success" icon={CheckCircle}>
              {summary.countByStatus.Approved} Approved
            </Badge>
          </div>
        </Card>
      </div>

      {/* Invoice Table */}
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-neutral-50 text-neutral-500 uppercase text-xs tracking-wider">
              <th className="px-4 py-3 text-left font-medium w-8" aria-label="Expand" />
              <th className="px-4 py-3 text-left font-medium">Invoice Number</th>
              <th className="px-4 py-3 text-left font-medium">Period</th>
              <th className="px-4 py-3 text-right font-medium">Total Amount</th>
              <th className="px-4 py-3 text-right font-medium">Line Items</th>
              <th className="px-4 py-3 text-left font-medium">Validation Status</th>
            </tr>
          </thead>
          <tbody>
            {invoices.map((invoice) => {
              const isExpanded = expandedIds.has(invoice.id);

              return (
                <InvoiceRow
                  key={invoice.id}
                  invoice={invoice}
                  isExpanded={isExpanded}
                  onToggle={() => toggleExpanded(invoice.id)}
                  onKeyDown={(e) => handleRowKeyDown(e, invoice.id)}
                />
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

interface InvoiceRowProps {
  readonly invoice: Invoice;
  readonly isExpanded: boolean;
  readonly onToggle: () => void;
  readonly onKeyDown: (e: React.KeyboardEvent) => void;
}

function InvoiceRow({ invoice, isExpanded, onToggle, onKeyDown }: InvoiceRowProps) {
  const status = invoice.validationStatus;

  return (
    <>
      <tr
        className="border-b border-neutral-100 hover:bg-red-50/30 cursor-pointer"
        onClick={onToggle}
        onKeyDown={onKeyDown}
        tabIndex={0}
        role="button"
        aria-expanded={isExpanded}
        aria-label={`Invoice ${invoice.invoiceNumber}, ${isExpanded ? "collapse" : "expand"} details`}
      >
        <td className="px-4 py-3">
          {isExpanded ? (
            <ChevronDown className="size-4 text-neutral-500" aria-hidden="true" />
          ) : (
            <ChevronRight className="size-4 text-neutral-500" aria-hidden="true" />
          )}
        </td>
        <td className="px-4 py-3 font-medium">{invoice.invoiceNumber}</td>
        <td className="px-4 py-3">{invoice.period}</td>
        <td className="px-4 py-3 text-right tabular-nums">{formatIDR(invoice.totalAmount)}</td>
        <td className="px-4 py-3 text-right tabular-nums">{invoice.lineItemsCount}</td>
        <td className="px-4 py-3">
          <Badge
            variant={statusBadgeMap[status]}
            icon={status === "Approved" ? CheckCircle : undefined}
          >
            {status}
          </Badge>
        </td>
      </tr>
      {isExpanded && (
        <tr>
          <td colSpan={6} className="px-4 py-3 bg-neutral-50/50">
            <InvoiceDetail lineItems={invoice.lineItems} />
          </td>
        </tr>
      )}
    </>
  );
}
