import { useState, useCallback } from 'react';

import { WorkflowSteps } from '@/components/ui/WorkflowSteps';
import { useRole } from '@/context/RoleContext';
import { useInvoiceData } from './useInvoiceData';
import type { InvoiceWithVendor } from './useInvoiceData';
import { InvoiceList } from './InvoiceList';
import { InvoiceDetail } from './InvoiceDetail';
import type { Invoice } from './invoice.types';

type ValidationStatus = Invoice['validationStatus'];

function deriveWorkflowSteps(status: ValidationStatus | null) {
  if (!status) {
    return [
      { label: 'Upload', status: 'upcoming' as const },
      { label: 'Validate', status: 'upcoming' as const },
      { label: 'Approve', status: 'upcoming' as const },
    ];
  }

  switch (status) {
    case 'Uploaded':
      return [
        { label: 'Upload', status: 'current' as const },
        { label: 'Validate', status: 'upcoming' as const },
        { label: 'Approve', status: 'upcoming' as const },
      ];
    case 'Validated':
      return [
        { label: 'Upload', status: 'completed' as const },
        { label: 'Validate', status: 'current' as const },
        { label: 'Approve', status: 'upcoming' as const },
      ];
    case 'Approved':
      return [
        { label: 'Upload', status: 'completed' as const },
        { label: 'Validate', status: 'completed' as const },
        { label: 'Approve', status: 'completed' as const },
      ];
    case 'Mismatch Detected':
      return [
        { label: 'Upload', status: 'completed' as const },
        { label: 'Validate', status: 'current' as const },
        { label: 'Approve', status: 'upcoming' as const },
      ];
  }
}

export function InvoiceFlow() {
  const { data: invoices, isLoading, isError } = useInvoiceData();
  const { role } = useRole();
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [localInvoices, setLocalInvoices] = useState<InvoiceWithVendor[] | null>(null);

  // Use local state if we've performed an approval, otherwise use query data
  const displayInvoices = localInvoices ?? invoices ?? [];

  const selectedInvoice = displayInvoices.find((inv) => inv.id === selectedId) ?? null;
  const workflowSteps = deriveWorkflowSteps(selectedInvoice?.validationStatus ?? null);

  const handleSelect = useCallback((invoiceId: string) => {
    setSelectedId(invoiceId);
  }, []);

  const handleApprove = useCallback(
    (invoiceId: string) => {
      const now = new Date().toISOString();
      const approverName = role;

      setLocalInvoices((prev) => {
        const source = prev ?? invoices ?? [];
        return source.map((inv) =>
          inv.id === invoiceId
            ? {
                ...inv,
                validationStatus: 'Approved' as const,
                approverName,
                approvedAt: now,
              }
            : inv,
        );
      });
    },
    [invoices, role],
  );

  if (isLoading) {
    return (
      <div className="p-6">
        <p className="text-sm text-[var(--n-500)]">Loading invoices…</p>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="p-6">
        <div className="rounded-md bg-danger-bg p-4 text-sm text-danger-fg">
          Unable to load invoice data. Please check mock data files.
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6 p-6">
      <h1 className="text-xl font-semibold text-[var(--n-900)]">Invoice Flow</h1>

      {/* Workflow steps indicator */}
      <WorkflowSteps steps={workflowSteps} />

      {/* Invoice list */}
      <InvoiceList data={displayInvoices} onSelect={handleSelect} />

      {/* Invoice detail panel */}
      {selectedInvoice && (
        <div className="rounded-lg border border-[var(--n-200)] bg-[var(--n-0)] p-6 shadow-sm">
          <InvoiceDetail invoice={selectedInvoice} onApprove={handleApprove} />
        </div>
      )}
    </div>
  );
}
