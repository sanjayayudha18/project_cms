import { render, screen, fireEvent } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import type { ReplenishmentSchedule } from '../types';
import { sortByStatusPriority, filterSchedules } from '../replenishment.utils';

// ─── Hoisted mock data (static, used by component at import time) ────────────

const { MOCK_DATA, mockToast } = vi.hoisted(() => ({
  MOCK_DATA: [
    {
      id: 'SCH-001',
      routeCode: 'JKT-S-001',
      region: 'South Jakarta',
      vendor: 'PT Gardanet',
      windowStart: '08:00',
      windowEnd: '12:00',
      machineCount: 18,
      completionCount: 15,
      status: 'in-transit',
      cashValue: 12800000000,
    },
    {
      id: 'SCH-002',
      routeCode: 'JKT-N-002',
      region: 'North Jakarta',
      vendor: 'PT SSI',
      windowStart: '07:30',
      windowEnd: '11:30',
      machineCount: 14,
      completionCount: 14,
      status: 'completed',
      cashValue: 9500000000,
    },
    {
      id: 'SCH-003',
      routeCode: 'BDG-C-003',
      region: 'Bandung',
      vendor: 'PT G4S',
      windowStart: '09:00',
      windowEnd: '13:00',
      machineCount: 12,
      completionCount: 4,
      status: 'delayed',
      cashValue: 7200000000,
    },
    {
      id: 'SCH-004',
      routeCode: 'SBY-E-004',
      region: 'Surabaya',
      vendor: 'PT Gardanet',
      windowStart: '08:30',
      windowEnd: '12:30',
      machineCount: 16,
      completionCount: 16,
      status: 'completed',
      cashValue: 11400000000,
    },
    {
      id: 'SCH-005',
      routeCode: 'JKT-S-005',
      region: 'South Jakarta',
      vendor: 'PT G4S',
      windowStart: '10:00',
      windowEnd: '14:00',
      machineCount: 10,
      completionCount: 0,
      status: 'scheduled',
      cashValue: 6800000000,
    },
    {
      id: 'SCH-006',
      routeCode: 'SBY-W-006',
      region: 'Surabaya',
      vendor: 'PT SSI',
      windowStart: '09:30',
      windowEnd: '13:30',
      machineCount: 8,
      completionCount: 0,
      status: 'pending-vendor',
      cashValue: 5100000000,
    },
  ] as ReplenishmentSchedule[],
  mockToast: vi.fn(),
}));

// ─── Mock modules ────────────────────────────────────────────────────────────

vi.mock('@/data/replenishment-schedules.json', () => ({ default: MOCK_DATA }));
vi.mock('@/lib/hooks/useToast', () => ({
  useToast: () => ({ toast: mockToast }),
}));

// ─── Import component after mocks ───────────────────────────────────────────

import { ReplenishmentScreen } from '../ReplenishmentScreen';

// ─── sortByStatusPriority Tests ──────────────────────────────────────────────

describe('sortByStatusPriority', () => {
  it('sorts records by priority: delayed → in-transit → pending-vendor → scheduled → completed', () => {
    const input: ReplenishmentSchedule[] = [
      { ...MOCK_DATA[1] }, // completed
      { ...MOCK_DATA[0] }, // in-transit
      { ...MOCK_DATA[2] }, // delayed
      { ...MOCK_DATA[4] }, // scheduled
      { ...MOCK_DATA[5] }, // pending-vendor
    ];

    const result = sortByStatusPriority(input);

    expect(result[0].status).toBe('delayed');
    expect(result[1].status).toBe('in-transit');
    expect(result[2].status).toBe('pending-vendor');
    expect(result[3].status).toBe('scheduled');
    expect(result[4].status).toBe('completed');
  });

  it('preserves original order for records with same status', () => {
    const input: ReplenishmentSchedule[] = [
      { ...MOCK_DATA[1], id: 'FIRST-COMPLETED' }, // completed
      { ...MOCK_DATA[3], id: 'SECOND-COMPLETED' }, // completed
    ];

    const result = sortByStatusPriority(input);

    expect(result[0].id).toBe('FIRST-COMPLETED');
    expect(result[1].id).toBe('SECOND-COMPLETED');
  });

  it('does not mutate the original array', () => {
    const input = [...MOCK_DATA];
    const originalFirst = input[0];

    sortByStatusPriority(input);

    expect(input[0]).toBe(originalFirst);
  });

  it('returns empty array for empty input', () => {
    const result = sortByStatusPriority([]);
    expect(result).toEqual([]);
  });
});

// ─── filterSchedules Tests ───────────────────────────────────────────────────

describe('filterSchedules', () => {
  it('returns all records when region is "All" and vendor is "All"', () => {
    const result = filterSchedules(MOCK_DATA, 'All', 'All');
    expect(result).toHaveLength(MOCK_DATA.length);
  });

  it('returns all records when region is "All regions" and vendor is "All vendors"', () => {
    const result = filterSchedules(MOCK_DATA, 'All regions', 'All vendors');
    expect(result).toHaveLength(MOCK_DATA.length);
  });

  it('filters by region only when vendor is "All"', () => {
    const result = filterSchedules(MOCK_DATA, 'South Jakarta', 'All');
    expect(result).toHaveLength(2);
    expect(result.every((r) => r.region === 'South Jakarta')).toBe(true);
  });

  it('filters by vendor only when region is "All regions"', () => {
    const result = filterSchedules(MOCK_DATA, 'All regions', 'PT Gardanet');
    expect(result).toHaveLength(2);
    expect(result.every((r) => r.vendor === 'PT Gardanet')).toBe(true);
  });

  it('applies combined AND filter for both region and vendor', () => {
    const result = filterSchedules(MOCK_DATA, 'South Jakarta', 'PT G4S');
    expect(result).toHaveLength(1);
    expect(result[0].id).toBe('SCH-005');
  });

  it('returns empty array when no records match the filter', () => {
    const result = filterSchedules(MOCK_DATA, 'Bandung', 'PT Gardanet');
    expect(result).toHaveLength(0);
  });

  it('returns empty array for empty input', () => {
    const result = filterSchedules([], 'South Jakarta', 'PT Gardanet');
    expect(result).toHaveLength(0);
  });
});

// ─── ReplenishmentScreen Column Rendering Tests ──────────────────────────────

describe('ReplenishmentScreen - Column rendering', () => {
  it('renders all expected column headers', () => {
    render(<ReplenishmentScreen />);

    expect(screen.getByText('Jadwal')).toBeInTheDocument();
    // "Wilayah" appears as both filter label and column header
    expect(screen.getAllByText('Wilayah').length).toBeGreaterThanOrEqual(2);
    // "Vendor" appears as both filter label and column header
    expect(screen.getAllByText('Vendor').length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText('Jendela waktu')).toBeInTheDocument();
    expect(screen.getByText('Mesin')).toBeInTheDocument();
    expect(screen.getByText('Status')).toBeInTheDocument();
    expect(screen.getByText('Nilai kas')).toBeInTheDocument();
  });

  it('renders schedule IDs and route codes', () => {
    render(<ReplenishmentScreen />);

    expect(screen.getByText('SCH-003')).toBeInTheDocument();
    expect(screen.getByText('BDG-C-003')).toBeInTheDocument();
  });

  it('renders region values', () => {
    render(<ReplenishmentScreen />);

    expect(screen.getAllByText('South Jakarta').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Bandung').length).toBeGreaterThanOrEqual(1);
  });

  it('renders vendor values', () => {
    render(<ReplenishmentScreen />);

    expect(screen.getAllByText('PT Gardanet').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('PT SSI').length).toBeGreaterThanOrEqual(1);
  });

  it('renders time window formatted as start–end', () => {
    render(<ReplenishmentScreen />);

    expect(screen.getByText('08:00\u201312:00')).toBeInTheDocument();
    expect(screen.getByText('09:00\u201313:00')).toBeInTheDocument();
  });

  it('renders status badges with correct labels', () => {
    render(<ReplenishmentScreen />);

    expect(screen.getByText('Dalam perjalanan')).toBeInTheDocument();
    expect(screen.getByText('Terlambat')).toBeInTheDocument();
    expect(screen.getByText('Menunggu vendor')).toBeInTheDocument();
    expect(screen.getByText('Terjadwal')).toBeInTheDocument();
    expect(screen.getAllByText('Selesai').length).toBeGreaterThanOrEqual(1);
  });

  it('renders page header with correct title', () => {
    render(<ReplenishmentScreen />);

    expect(screen.getByText('Jadwal pengisian ulang')).toBeInTheDocument();
    expect(screen.getByText(/Pantau rute CIT harian/)).toBeInTheDocument();
  });

  it('renders data sorted by status priority (delayed first)', () => {
    render(<ReplenishmentScreen />);

    // Get all rows (skipping header)
    const rows = screen.getAllByRole('row').slice(1);
    // First row should be "delayed" status → SCH-003
    expect(rows[0]).toHaveTextContent('SCH-003');
  });
});

// ─── ReplenishmentScreen Filter Behavior Tests ───────────────────────────────

describe('ReplenishmentScreen - Filter behavior', () => {
  it('displays all records count initially', () => {
    render(<ReplenishmentScreen />);

    expect(screen.getByText('6 jadwal')).toBeInTheDocument();
  });

  it('filters by region when region filter is changed', () => {
    render(<ReplenishmentScreen />);

    const selects = screen.getAllByRole('combobox');
    // Region filter is the second combobox (first is date)
    fireEvent.change(selects[1], { target: { value: 'South Jakarta' } });

    expect(screen.getByText('2 jadwal')).toBeInTheDocument();
  });

  it('filters by vendor when vendor filter is changed', () => {
    render(<ReplenishmentScreen />);

    const selects = screen.getAllByRole('combobox');
    // Vendor filter is the third combobox
    fireEvent.change(selects[2], { target: { value: 'PT SSI' } });

    expect(screen.getByText('2 jadwal')).toBeInTheDocument();
  });

  it('applies combined region + vendor filter', () => {
    render(<ReplenishmentScreen />);

    const selects = screen.getAllByRole('combobox');
    fireEvent.change(selects[1], { target: { value: 'South Jakarta' } });
    fireEvent.change(selects[2], { target: { value: 'PT Gardanet' } });

    expect(screen.getByText('1 jadwal')).toBeInTheDocument();
  });

  it('resets filter when set back to empty (null)', () => {
    render(<ReplenishmentScreen />);

    const selects = screen.getAllByRole('combobox');
    // Filter to South Jakarta
    fireEvent.change(selects[1], { target: { value: 'South Jakarta' } });
    expect(screen.getByText('2 jadwal')).toBeInTheDocument();

    // Reset back to "all"
    fireEvent.change(selects[1], { target: { value: '' } });
    expect(screen.getByText('6 jadwal')).toBeInTheDocument();
  });
});

// ─── ReplenishmentScreen Empty State Tests ───────────────────────────────────

describe('ReplenishmentScreen - Empty state', () => {
  it('displays empty state when filters match no records', () => {
    render(<ReplenishmentScreen />);

    const selects = screen.getAllByRole('combobox');
    // Filter to Bandung + PT Gardanet (no match in mock data)
    fireEvent.change(selects[1], { target: { value: 'Bandung' } });
    fireEvent.change(selects[2], { target: { value: 'PT Gardanet' } });

    expect(screen.getByText('Tidak ada jadwal ditemukan')).toBeInTheDocument();
    expect(
      screen.getByText('Coba nomor rute, vendor, atau wilayah lain.'),
    ).toBeInTheDocument();
  });

  it('shows "0 jadwal" count when empty state is displayed', () => {
    render(<ReplenishmentScreen />);

    const selects = screen.getAllByRole('combobox');
    fireEvent.change(selects[1], { target: { value: 'Bandung' } });
    fireEvent.change(selects[2], { target: { value: 'PT Gardanet' } });

    expect(screen.getByText('0 jadwal')).toBeInTheDocument();
  });

  it('displays empty state when data JSON is empty array', async () => {
    vi.resetModules();
    vi.doMock('@/data/replenishment-schedules.json', () => ({ default: [] }));
    vi.doMock('@/lib/hooks/useToast', () => ({
      useToast: () => ({ toast: vi.fn() }),
    }));

    const { ReplenishmentScreen: EmptyScreen } = await import(
      '../ReplenishmentScreen'
    );
    render(<EmptyScreen />);

    expect(screen.getByText('Tidak ada jadwal ditemukan')).toBeInTheDocument();
  });

  it('still renders page header in empty data state', async () => {
    vi.resetModules();
    vi.doMock('@/data/replenishment-schedules.json', () => ({ default: [] }));
    vi.doMock('@/lib/hooks/useToast', () => ({
      useToast: () => ({ toast: vi.fn() }),
    }));

    const { ReplenishmentScreen: EmptyScreen } = await import(
      '../ReplenishmentScreen'
    );
    render(<EmptyScreen />);

    expect(screen.getByText('Jadwal pengisian ulang')).toBeInTheDocument();
  });
});
