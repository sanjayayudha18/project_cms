// Mock ResizeObserver for Recharts
class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
(globalThis as Record<string, unknown>).ResizeObserver = ResizeObserver;

import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';

import { VendorBarChart, formatShortDate, formatChartValue } from '../VendorBarChart';
import { VENDOR_COLORS } from '../constants';

// Mock ResponsiveContainer to render children at a fixed size (JSDOM has no layout engine)
vi.mock('recharts', async () => {
  const actual = await vi.importActual<typeof import('recharts')>('recharts');
  return {
    ...actual,
    ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
      <div style={{ width: 800, height: 280 }}>{children}</div>
    ),
  };
});

const mockData = [
  { date: '2026-07-15', Abacus: 8.2, 'Bijak Jakarta': 6.1, Advantage: 5.4, SSI: 3.9 },
  { date: '2026-07-16', Abacus: 7.5, 'Bijak Jakarta': 6.8, Advantage: 5.1, SSI: 4.1 },
];

describe('formatShortDate', () => {
  it('formats "2026-07-15" to "15 Jul"', () => {
    expect(formatShortDate('2026-07-15')).toBe('15 Jul');
  });
});

describe('formatChartValue', () => {
  it('formats 8.2 to "Rp 8.2 M"', () => {
    expect(formatChartValue(8.2)).toBe('Rp 8.2 M');
  });
});

describe('VendorBarChart', () => {
  it('renders the chart container with correct aria-label', () => {
    render(<VendorBarChart data={mockData} vendors={VENDOR_COLORS} />);
    expect(
      screen.getByLabelText('Bar chart showing daily cash flow per vendor for the past 7 days'),
    ).toBeInTheDocument();
  });

  it('has min-h-[240px] class on the container', () => {
    render(<VendorBarChart data={mockData} vendors={VENDOR_COLORS} />);
    const container = screen.getByLabelText(
      'Bar chart showing daily cash flow per vendor for the past 7 days',
    );
    expect(container.className).toContain('min-h-[240px]');
  });

  it('renders the chart container', () => {
    const { container } = render(<VendorBarChart data={mockData} vendors={VENDOR_COLORS} />);
    expect(container.firstElementChild).toBeTruthy();
  });
});
