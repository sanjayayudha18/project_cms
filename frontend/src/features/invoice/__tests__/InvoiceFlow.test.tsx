/**
 * Unit tests for Invoice Flow feature.
 *
 * Validates: Requirements 6.6, 6.7, 6.9
 */
import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { useMemo, useState } from 'react';
import type { ReactNode } from 'react';

import { InvoiceFlow } from '../InvoiceFlow';
import { RoleContext } from '@/context/RoleContext';
import type { Role } from '@/lib/constants';
import { INTERNAL_ROLES } from '@/lib/constants';

// --- Test utilities ---

interface RoleContextValue {
  role: Role;
  setRole: (role: Role) => void;
  isInternal: boolean;
}

function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { staleTime: Infinity, retry: false },
    },
  });
}

function TestRoleProvider({
  children,
  initialRole,
}: {
  children: ReactNode;
  initialRole: Role;
}) {
  const [role, setRole] = useState<Role>(initialRole);
  const value = useMemo<RoleContextValue>(() => {
    const isInternal = (INTERNAL_ROLES as readonly string[]).includes(role);
    return { role, setRole, isInternal };
  }, [role]);

  return <RoleContext.Provider value={value}>{children}</RoleContext.Provider>;
}

function renderInvoiceFlow(role: Role = 'Admin') {
  const queryClient = createTestQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <TestRoleProvider initialRole={role}>
        <MemoryRouter>
          <InvoiceFlow />
        </MemoryRouter>
      </TestRoleProvider>
    </QueryClientProvider>,
  );
}

// --- Tests ---

describe('InvoiceFlow', () => {
  describe('Approve button visibility (Requirement 6.6)', () => {
    it('shows Approve button when role is Manager and invoice is Validated', async () => {
      renderInvoiceFlow('Manager');

      // Wait for data to load
      await waitFor(() => {
        expect(screen.getByText('INV-2024-002')).toBeInTheDocument();
      });

      // Click on a Validated invoice (INV-2024-002 is Validated)
      fireEvent.click(screen.getByText('INV-2024-002'));

      // Approve button should be visible (actual <button> with "Approve Invoice")
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /approve invoice/i })).toBeInTheDocument();
      });
    });

    it('hides Approve button when Manager selects non-Validated invoice', async () => {
      renderInvoiceFlow('Manager');

      await waitFor(() => {
        expect(screen.getByText('INV-2024-001')).toBeInTheDocument();
      });

      // Click on an Approved invoice (INV-2024-001 is Approved)
      fireEvent.click(screen.getByText('INV-2024-001'));

      // Wait for detail to appear
      await waitFor(() => {
        expect(screen.getByText(/Validator/)).toBeInTheDocument();
      });

      // Approve button should NOT be visible
      expect(
        screen.queryByRole('button', { name: /approve invoice/i }),
      ).not.toBeInTheDocument();
    });

    it('hides Approve button for Uploaded invoice even with Manager role', async () => {
      renderInvoiceFlow('Manager');

      await waitFor(() => {
        expect(screen.getByText('INV-2024-005')).toBeInTheDocument();
      });

      // Click on an Uploaded invoice (INV-2024-005 is Uploaded)
      fireEvent.click(screen.getByText('INV-2024-005'));

      // Wait for detail to appear
      await waitFor(() => {
        expect(screen.getByText(/Validator/)).toBeInTheDocument();
      });

      expect(
        screen.queryByRole('button', { name: /approve invoice/i }),
      ).not.toBeInTheDocument();
    });
  });

  describe('Approval flow (Requirement 6.7)', () => {
    it('clicking Approve updates status and shows confirmation message', async () => {
      renderInvoiceFlow('Manager');

      await waitFor(() => {
        expect(screen.getByText('INV-2024-002')).toBeInTheDocument();
      });

      // Select the Validated invoice
      fireEvent.click(screen.getByText('INV-2024-002'));

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /approve invoice/i })).toBeInTheDocument();
      });

      // Click Approve
      fireEvent.click(screen.getByRole('button', { name: /approve invoice/i }));

      // Confirmation message should appear with "approved by" text
      await waitFor(() => {
        expect(screen.getByText(/approved by/i)).toBeInTheDocument();
      });

      // The confirmation contains the invoice number and approver (Manager)
      const confirmation = screen.getByText(/approved by/i).closest('span');
      expect(confirmation?.textContent).toMatch(/INV-2024-002/);
      expect(confirmation?.textContent).toMatch(/Manager/);

      // Approve button should no longer be visible (status changed to Approved)
      expect(
        screen.queryByRole('button', { name: /approve invoice/i }),
      ).not.toBeInTheDocument();
    });
  });

  describe('Read-only mode for non-Manager roles (Requirement 6.9)', () => {
    it('hides Approve button for Operator role even on Validated invoice', async () => {
      renderInvoiceFlow('Operator');

      await waitFor(() => {
        expect(screen.getByText('INV-2024-002')).toBeInTheDocument();
      });

      // Click on the Validated invoice
      fireEvent.click(screen.getByText('INV-2024-002'));

      // Detail should show but without Approve button
      await waitFor(() => {
        expect(screen.getByText(/Validator/)).toBeInTheDocument();
      });
      expect(
        screen.queryByRole('button', { name: /approve invoice/i }),
      ).not.toBeInTheDocument();
    });

    it('hides Approve button for Vendor role even on Validated invoice', async () => {
      renderInvoiceFlow('Vendor');

      await waitFor(() => {
        expect(screen.getByText('INV-2024-002')).toBeInTheDocument();
      });

      // Click on the Validated invoice
      fireEvent.click(screen.getByText('INV-2024-002'));

      await waitFor(() => {
        expect(screen.getByText(/Validator/)).toBeInTheDocument();
      });
      expect(
        screen.queryByRole('button', { name: /approve invoice/i }),
      ).not.toBeInTheDocument();
    });

    it('hides Approve button for Admin role even on Validated invoice', async () => {
      renderInvoiceFlow('Admin');

      await waitFor(() => {
        expect(screen.getByText('INV-2024-002')).toBeInTheDocument();
      });

      // Click on the Validated invoice
      fireEvent.click(screen.getByText('INV-2024-002'));

      await waitFor(() => {
        expect(screen.getByText(/Validator/)).toBeInTheDocument();
      });
      expect(
        screen.queryByRole('button', { name: /approve invoice/i }),
      ).not.toBeInTheDocument();
    });
  });

  describe('Workflow step derivation from invoice status (Requirement 6.1)', () => {
    it('shows Upload step as current for Uploaded invoice', async () => {
      renderInvoiceFlow('Manager');

      await waitFor(() => {
        expect(screen.getByText('INV-2024-005')).toBeInTheDocument();
      });

      // Click on Uploaded invoice
      fireEvent.click(screen.getByText('INV-2024-005'));

      await waitFor(() => {
        const steps = screen.getByRole('list', { name: /workflow/i });
        expect(steps).toBeInTheDocument();
      });

      // Upload step should be current
      const uploadStep = screen.getByText('Upload').closest('[role="listitem"]');
      expect(uploadStep?.querySelector('[aria-current="step"]')).toBeInTheDocument();
    });

    it('shows Validate step as current for Validated invoice', async () => {
      renderInvoiceFlow('Manager');

      await waitFor(() => {
        expect(screen.getByText('INV-2024-002')).toBeInTheDocument();
      });

      // Click on Validated invoice
      fireEvent.click(screen.getByText('INV-2024-002'));

      await waitFor(() => {
        const steps = screen.getByRole('list', { name: /workflow/i });
        expect(steps).toBeInTheDocument();
      });

      // Validate step should be current
      const validateStep = screen.getByText('Validate').closest('[role="listitem"]');
      expect(validateStep?.querySelector('[aria-current="step"]')).toBeInTheDocument();
    });

    it('shows all steps as completed for Approved invoice', async () => {
      renderInvoiceFlow('Manager');

      await waitFor(() => {
        expect(screen.getByText('INV-2024-001')).toBeInTheDocument();
      });

      // Click on Approved invoice
      fireEvent.click(screen.getByText('INV-2024-001'));

      await waitFor(() => {
        const steps = screen.getByRole('list', { name: /workflow/i });
        expect(steps).toBeInTheDocument();
      });

      // No step should have aria-current="step" (all completed)
      const stepsList = screen.getByRole('list', { name: /workflow/i });
      expect(stepsList.querySelector('[aria-current="step"]')).not.toBeInTheDocument();
    });

    it('shows Validate step as current for Mismatch Detected invoice', async () => {
      renderInvoiceFlow('Manager');

      await waitFor(() => {
        expect(screen.getByText('INV-2024-003')).toBeInTheDocument();
      });

      // Click on Mismatch Detected invoice
      fireEvent.click(screen.getByText('INV-2024-003'));

      await waitFor(() => {
        const steps = screen.getByRole('list', { name: /workflow/i });
        expect(steps).toBeInTheDocument();
      });

      // Validate step should be current (Mismatch is still in validate phase)
      const validateStep = screen.getByText('Validate').closest('[role="listitem"]');
      expect(validateStep?.querySelector('[aria-current="step"]')).toBeInTheDocument();
    });
  });
});
