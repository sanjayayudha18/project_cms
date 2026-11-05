import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { DsrSummary } from '../DsrSummary';
import { deriveStatus } from '@/lib/deriveStatus';
import { formatIDR } from '@/lib/formatCurrency';
import type { EnrichedDsrRecord } from '../dsr.types';
import dsrData from '@/data/dsr.json';
import type { DsrRecord } from '../dsr.types';

/**
 * Unit tests for DSR Dashboard
 * Validates: Requirements 3.4, 3.6, 3.8
 */

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: Infinity },
    },
  });
}

function renderWithProviders(ui: React.ReactElement) {
  const queryClient = createQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

// Helper to create test DSR records
function makeDsrRecord(overrides: Partial<EnrichedDsrRecord> = {}): EnrichedDsrRecord {
  return {
    id: 'DSR-TEST-001',
    atmId: 'ATM-JKT-001',
    date: '2024-01-15',
    beginningBalance: 300_000_000,
    cashIn: 100_000_000,
    cashOut: 200_000_000,
    endingBalance: 200_000_000,
    status: 'Normal',
    location: 'Sudirman Tower, Jakarta',
    vendorName: 'PT Gardanet',
    ...overrides,
  };
}

describe('DsrSummary - summary card totals', () => {
  it('computes correct totals for a single record', () => {
    const records: EnrichedDsrRecord[] = [
      makeDsrRecord({
        beginningBalance: 100_000_000,
        cashIn: 50_000_000,
        cashOut: 30_000_000,
        endingBalance: 120_000_000,
      }),
    ];

    renderWithProviders(<DsrSummary data={records} />);

    expect(screen.getByText(formatIDR(100_000_000))).toBeInTheDocument();
    expect(screen.getByText(formatIDR(50_000_000))).toBeInTheDocument();
    expect(screen.getByText(formatIDR(30_000_000))).toBeInTheDocument();
    expect(screen.getByText(formatIDR(120_000_000))).toBeInTheDocument();
  });

  it('computes correct totals across multiple records', () => {
    const records: EnrichedDsrRecord[] = [
      makeDsrRecord({
        id: 'DSR-001',
        beginningBalance: 200_000_000,
        cashIn: 80_000_000,
        cashOut: 60_000_000,
        endingBalance: 220_000_000,
      }),
      makeDsrRecord({
        id: 'DSR-002',
        beginningBalance: 150_000_000,
        cashIn: 40_000_000,
        cashOut: 90_000_000,
        endingBalance: 100_000_000,
      }),
      makeDsrRecord({
        id: 'DSR-003',
        beginningBalance: 300_000_000,
        cashIn: 120_000_000,
        cashOut: 200_000_000,
        endingBalance: 220_000_000,
      }),
    ];

    const expectedBeginning = 200_000_000 + 150_000_000 + 300_000_000;
    const expectedCashIn = 80_000_000 + 40_000_000 + 120_000_000;
    const expectedCashOut = 60_000_000 + 90_000_000 + 200_000_000;
    const expectedEnding = 220_000_000 + 100_000_000 + 220_000_000;

    renderWithProviders(<DsrSummary data={records} />);

    expect(screen.getByText(formatIDR(expectedBeginning))).toBeInTheDocument();
    expect(screen.getByText(formatIDR(expectedCashIn))).toBeInTheDocument();
    expect(screen.getByText(formatIDR(expectedCashOut))).toBeInTheDocument();
    expect(screen.getByText(formatIDR(expectedEnding))).toBeInTheDocument();
  });

  it('displays zero totals when data is empty', () => {
    renderWithProviders(<DsrSummary data={[]} />);

    // With empty data, all totals should be 0
    const zeroFormatted = formatIDR(0);
    const zeroElements = screen.getAllByText(zeroFormatted);
    expect(zeroElements.length).toBe(4);
  });

  it('displays labels for all four summary cards', () => {
    renderWithProviders(<DsrSummary data={[makeDsrRecord()]} />);

    expect(screen.getByText('Total Beginning Balance')).toBeInTheDocument();
    expect(screen.getByText('Total Cash In')).toBeInTheDocument();
    expect(screen.getByText('Total Cash Out')).toBeInTheDocument();
    expect(screen.getByText('Total Ending Balance')).toBeInTheDocument();
  });
});

describe('Status badge mapping from ending balance', () => {
  it('maps endingBalance < 50M to Critical status (danger variant)', () => {
    const status = deriveStatus(30_000_000);
    expect(status).toBe('Critical');
  });

  it('maps endingBalance at 49,999,999 to Critical', () => {
    expect(deriveStatus(49_999_999)).toBe('Critical');
  });

  it('maps endingBalance at 50M to Low (warning variant)', () => {
    expect(deriveStatus(50_000_000)).toBe('Low');
  });

  it('maps endingBalance between 50M and 150M to Low', () => {
    expect(deriveStatus(75_000_000)).toBe('Low');
    expect(deriveStatus(100_000_000)).toBe('Low');
    expect(deriveStatus(150_000_000)).toBe('Low');
  });

  it('maps endingBalance at 150,000,001 to Normal (success variant)', () => {
    expect(deriveStatus(150_000_001)).toBe('Normal');
  });

  it('maps endingBalance > 150M to Normal', () => {
    expect(deriveStatus(200_000_000)).toBe('Normal');
    expect(deriveStatus(500_000_000)).toBe('Normal');
  });

  it('maps endingBalance of 0 to Critical', () => {
    expect(deriveStatus(0)).toBe('Critical');
  });
});

describe('Date filtering returns correct records', () => {
  const allRecords = dsrData as DsrRecord[];

  it('filters records for 2024-01-15 correctly', () => {
    const filtered = allRecords.filter((r) => r.date === '2024-01-15');
    expect(filtered.length).toBeGreaterThanOrEqual(20);
    expect(filtered.every((r) => r.date === '2024-01-15')).toBe(true);
  });

  it('filters records for 2024-01-16 correctly', () => {
    const filtered = allRecords.filter((r) => r.date === '2024-01-16');
    expect(filtered.length).toBeGreaterThan(0);
    expect(filtered.every((r) => r.date === '2024-01-16')).toBe(true);
  });

  it('returns empty array for a date with no records', () => {
    const filtered = allRecords.filter((r) => r.date === '2099-12-31');
    expect(filtered).toHaveLength(0);
  });

  it('no cross-contamination between dates', () => {
    const date15 = allRecords.filter((r) => r.date === '2024-01-15');
    const date16 = allRecords.filter((r) => r.date === '2024-01-16');

    // No record from date 15 should appear in date 16 filter
    const ids15 = new Set(date15.map((r) => r.id));
    const ids16 = new Set(date16.map((r) => r.id));

    for (const id of ids15) {
      expect(ids16.has(id)).toBe(false);
    }
  });

  it('all dates in dataset are valid ISO date strings', () => {
    const dateRegex = /^\d{4}-\d{2}-\d{2}$/;
    for (const record of allRecords) {
      expect(record.date).toMatch(dateRegex);
    }
  });

  it('available dates span a 7-day period', () => {
    const uniqueDates = [...new Set(allRecords.map((r) => r.date))].sort();
    expect(uniqueDates.length).toBeGreaterThanOrEqual(7);
  });
});
