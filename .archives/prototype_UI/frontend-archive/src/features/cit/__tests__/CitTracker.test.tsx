import { render, screen, fireEvent, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect } from 'vitest';

import { RoleProvider } from '@/context/RoleContext';
import { CitTracker } from '../CitTracker';

function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { staleTime: Infinity, retry: false },
    },
  });
}

function renderCitTracker() {
  const queryClient = createTestQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <RoleProvider>
        <MemoryRouter>
          <CitTracker />
        </MemoryRouter>
      </RoleProvider>
    </QueryClientProvider>,
  );
}

/**
 * Helper to get filter selects by their position (Status is first, Vendor is second).
 */
function getFilterSelects() {
  const selects = screen.getAllByRole('combobox');
  const statusSelect = selects[0]!;
  const vendorSelect = selects[1]!;
  return { statusSelect, vendorSelect };
}

/**
 * Helper to find a summary card by its label text and return the count value.
 * Summary cards render: <p class="...text-n-500">{label}</p><p class="...tabular-nums">{count}</p>
 */
function getSummaryCount(label: string): string {
  const summaryGrid = document.querySelector('.grid.grid-cols-2');
  const container = within(summaryGrid as HTMLElement);
  const labelEl = container.getAllByText(label).find(
    (el) => el.classList.contains('text-n-500'),
  );
  if (!labelEl) throw new Error(`Summary label "${label}" not found`);
  const countEl = labelEl.nextElementSibling as HTMLElement;
  return countEl.textContent ?? '';
}

describe('CitTracker', () => {
  describe('Compound filter (status + vendor)', () => {
    it('shows only orders matching both status and vendor when both filters are active', async () => {
      renderCitTracker();

      // Wait for data to load
      await screen.findByText('CIT-20240115-001');

      const { statusSelect, vendorSelect } = getFilterSelects();

      // Apply status filter: "Completed"
      fireEvent.change(statusSelect, { target: { value: 'Completed' } });

      // Apply vendor filter: "V-001" (PT Gardanet)
      fireEvent.change(vendorSelect, { target: { value: 'V-001' } });

      // Only orders matching BOTH: status=Completed AND vendorId=V-001
      // V-001 Completed orders: CIT-20240115-001, CIT-20240116-003, CIT-20240118-001
      expect(screen.getByText('CIT-20240115-001')).toBeInTheDocument();
      expect(screen.getByText('CIT-20240116-003')).toBeInTheDocument();
      expect(screen.getByText('CIT-20240118-001')).toBeInTheDocument();

      // Orders that should NOT appear:
      // CIT-20240119-002: V-001 but In Transit (wrong status)
      // CIT-20240120-001: V-001 but Scheduled (wrong status)
      // CIT-20240115-002: V-002 Completed (wrong vendor)
      expect(screen.queryByText('CIT-20240119-002')).not.toBeInTheDocument();
      expect(screen.queryByText('CIT-20240120-001')).not.toBeInTheDocument();
      expect(screen.queryByText('CIT-20240115-002')).not.toBeInTheDocument();
    });

    it('shows all orders for a vendor when only vendor filter is applied', async () => {
      renderCitTracker();

      await screen.findByText('CIT-20240115-001');

      const { vendorSelect } = getFilterSelects();

      // Apply only vendor filter: V-003 (PT G4S)
      fireEvent.change(vendorSelect, { target: { value: 'V-003' } });

      // V-003 orders: CIT-20240115-003, CIT-20240116-001, CIT-20240116-002, CIT-20240119-001, CIT-20240120-002, CIT-20240120-004
      expect(screen.getByText('CIT-20240115-003')).toBeInTheDocument();
      expect(screen.getByText('CIT-20240116-001')).toBeInTheDocument();
      expect(screen.getByText('CIT-20240119-001')).toBeInTheDocument();
      expect(screen.getByText('CIT-20240120-002')).toBeInTheDocument();

      // Orders from other vendors should NOT appear
      expect(screen.queryByText('CIT-20240115-001')).not.toBeInTheDocument(); // V-001
      expect(screen.queryByText('CIT-20240115-002')).not.toBeInTheDocument(); // V-002
    });
  });

  describe('Summary counts match filtered results', () => {
    it('shows correct total counts when no filter is applied', async () => {
      renderCitTracker();

      await screen.findByText('CIT-20240115-001');

      // From mock data counts:
      // Scheduled: 4, In Transit: 3, Completed: 8, Failed: 2
      expect(getSummaryCount('Scheduled')).toBe('4');
      expect(getSummaryCount('In Transit')).toBe('3');
      expect(getSummaryCount('Completed')).toBe('8');
      expect(getSummaryCount('Failed')).toBe('2');
    });

    it('updates summary counts to reflect filtered data', async () => {
      renderCitTracker();

      await screen.findByText('CIT-20240115-001');

      const { statusSelect } = getFilterSelects();

      // Filter by status: "In Transit"
      fireEvent.change(statusSelect, { target: { value: 'In Transit' } });

      // After filtering: only 3 In Transit orders remain
      expect(getSummaryCount('Scheduled')).toBe('0');
      expect(getSummaryCount('In Transit')).toBe('3');
      expect(getSummaryCount('Completed')).toBe('0');
      expect(getSummaryCount('Failed')).toBe('0');
    });
  });

  describe('Empty state and zero counts on no-match', () => {
    it('shows empty state message when filter combination matches no records', async () => {
      renderCitTracker();

      await screen.findByText('CIT-20240115-001');

      const { statusSelect, vendorSelect } = getFilterSelects();

      // Apply status: "Failed" + vendor: "V-001"
      // V-001 has no Failed orders in mock data
      fireEvent.change(statusSelect, { target: { value: 'Failed' } });
      fireEvent.change(vendorSelect, { target: { value: 'V-001' } });

      // Should show empty state
      expect(
        screen.getByText('No CIT orders match the current filters'),
      ).toBeInTheDocument();
    });

    it('shows zero for all status categories when no records match', async () => {
      renderCitTracker();

      await screen.findByText('CIT-20240115-001');

      const { statusSelect, vendorSelect } = getFilterSelects();

      // Apply impossible combination: Failed + V-001 (no V-001 failed orders)
      fireEvent.change(statusSelect, { target: { value: 'Failed' } });
      fireEvent.change(vendorSelect, { target: { value: 'V-001' } });

      // All summary counts should be 0
      expect(getSummaryCount('Scheduled')).toBe('0');
      expect(getSummaryCount('In Transit')).toBe('0');
      expect(getSummaryCount('Completed')).toBe('0');
      expect(getSummaryCount('Failed')).toBe('0');
    });
  });
});
