import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { Banknote, Landmark, Cpu, Truck } from 'lucide-react';

import { VENDOR_COLORS } from '../constants';

vi.mock('../useCashFlowData', () => ({
  useCashFlowData: vi.fn(),
}));

vi.mock('recharts', async () => {
  const actual = await vi.importActual<typeof import('recharts')>('recharts');
  return {
    ...actual,
    ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
      <div style={{ width: 800, height: 280 }}>{children}</div>
    ),
  };
});

class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
(globalThis as Record<string, unknown>).ResizeObserver = ResizeObserver;

import { CashFlowScreen } from '../CashFlowScreen';
import { useCashFlowData } from '../useCashFlowData';

const mockUseCashFlowData = vi.mocked(useCashFlowData);

const mockData = {
  stats: [
    { label: 'Total Kas Beredar', icon: Banknote, value: 'Rp 48,2 M', trend: { direction: 'up' as const, percentage: 2.4 } },
    { label: 'Saldo Vault Vendor', icon: Landmark, value: 'Rp 21,7 M' },
    { label: 'Kas di Mesin ATM', icon: Cpu, value: 'Rp 26,5 M', trend: { direction: 'down' as const, percentage: 1.1 } },
    { label: 'Drop CIT Hari Ini', icon: Truck, value: 'Rp 3,9 M', subtitle: '6 order' },
  ],
  vendorChart: {
    data: [{ date: '2026-07-21', Abacus: 9.2, 'Bijak Jakarta': 7.0, Advantage: 6.0, SSI: 4.2 }],
    vendors: VENDOR_COLORS,
  },
  atmLevels: [
    { id: 'ATM-00417', label: 'ATM-00417', percentage: 82 },
  ],
};

describe('CashFlowScreen', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Loading state', () => {
    it('renders skeleton with animate-pulse when loading', () => {
      mockUseCashFlowData.mockReturnValue({
        data: undefined,
        isLoading: true,
        isError: false,
        error: null,
        refetch: vi.fn(),
      });

      const { container } = render(<CashFlowScreen />);
      const skeleton = container.querySelector('.animate-pulse');
      expect(skeleton).toBeInTheDocument();
    });
  });

  describe('Error state', () => {
    it('renders error message "Gagal memuat data cash flow"', () => {
      mockUseCashFlowData.mockReturnValue({
        data: undefined,
        isLoading: false,
        isError: true,
        error: new Error('Network error'),
        refetch: vi.fn(),
      });

      render(<CashFlowScreen />);
      expect(screen.getByText(/Gagal memuat data cash flow/)).toBeInTheDocument();
    });

    it('renders a Retry button', () => {
      mockUseCashFlowData.mockReturnValue({
        data: undefined,
        isLoading: false,
        isError: true,
        error: new Error('Network error'),
        refetch: vi.fn(),
      });

      render(<CashFlowScreen />);
      expect(screen.getByText('Retry')).toBeInTheDocument();
    });

    it('calls refetch when Retry button is clicked', () => {
      const mockRefetch = vi.fn();
      mockUseCashFlowData.mockReturnValue({
        data: undefined,
        isLoading: false,
        isError: true,
        error: new Error('Network error'),
        refetch: mockRefetch,
      });

      render(<CashFlowScreen />);
      fireEvent.click(screen.getByText('Retry'));
      expect(mockRefetch).toHaveBeenCalledTimes(1);
    });
  });

  describe('Success state', () => {
    beforeEach(() => {
      mockUseCashFlowData.mockReturnValue({
        data: mockData,
        isLoading: false,
        isError: false,
        error: null,
        refetch: vi.fn(),
      });
    });

    it('renders h1 with "Cash Flow Monitoring"', () => {
      render(<CashFlowScreen />);
      const h1 = screen.getByRole('heading', { level: 1 });
      expect(h1).toHaveTextContent('Cash Flow Monitoring');
    });

    it('renders h2 elements for panel titles', () => {
      render(<CashFlowScreen />);
      const h2s = screen.getAllByRole('heading', { level: 2 });
      const h2Texts = h2s.map((el) => el.textContent);
      expect(h2Texts).toContain('Cash Flow Harian per Vendor');
      expect(h2Texts).toContain('Level Kas per ATM');
    });

    it('renders the data source badge "Sumber: EOD H-1"', () => {
      render(<CashFlowScreen />);
      expect(screen.getByText('Sumber: EOD H-1')).toBeInTheDocument();
    });

    it('renders all 4 stat card labels', () => {
      render(<CashFlowScreen />);
      expect(screen.getByText('Total Kas Beredar')).toBeInTheDocument();
      expect(screen.getByText('Saldo Vault Vendor')).toBeInTheDocument();
      expect(screen.getByText('Kas di Mesin ATM')).toBeInTheDocument();
      expect(screen.getByText('Drop CIT Hari Ini')).toBeInTheDocument();
    });
  });
});
