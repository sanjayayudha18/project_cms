import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { Banknote, Landmark, Cpu, Truck } from 'lucide-react';

import { StatsCard } from '../StatsCard';
import { StatsCardGrid } from '../StatsCardGrid';
import type { StatsCardData } from '../types';

const mockStats: StatsCardData[] = [
  {
    label: 'Total Kas Beredar',
    icon: Banknote,
    value: 'Rp 48,2 M',
    trend: { direction: 'up', percentage: 2.4 },
  },
  { label: 'Saldo Vault Vendor', icon: Landmark, value: 'Rp 21,7 M' },
  {
    label: 'Kas di Mesin ATM',
    icon: Cpu,
    value: 'Rp 26,5 M',
    trend: { direction: 'down', percentage: 1.1 },
  },
  {
    label: 'Drop CIT Hari Ini',
    icon: Truck,
    value: 'Rp 3,9 M',
    subtitle: '6 order',
  },
];

describe('StatsCard', () => {
  it('renders the label text correctly', () => {
    render(<StatsCard data={mockStats[0]!} />);
    expect(screen.getByText('Total Kas Beredar')).toBeInTheDocument();
  });

  it('renders the value with correct formatting', () => {
    render(<StatsCard data={mockStats[0]!} />);
    expect(screen.getByText('Rp 48,2 M')).toBeInTheDocument();
  });

  it('renders icon with aria-hidden="true"', () => {
    const { container } = render(<StatsCard data={mockStats[0]!} />);
    const svg = container.querySelector('svg');
    expect(svg).toHaveAttribute('aria-hidden', 'true');
  });

  it('renders subtitle when provided', () => {
    render(<StatsCard data={mockStats[3]!} />);
    expect(screen.getByText('6 order')).toBeInTheDocument();
  });

  it('does not render subtitle when not provided', () => {
    render(<StatsCard data={mockStats[0]!} />);
    expect(screen.queryByText('6 order')).not.toBeInTheDocument();
  });

  it('renders trend indicator with up arrow and percentage when direction is up', () => {
    render(<StatsCard data={mockStats[0]!} />);
    expect(screen.getByText('↑')).toBeInTheDocument();
    expect(screen.getByText('2.4%')).toBeInTheDocument();
  });

  it('renders trend indicator with down arrow and percentage when direction is down', () => {
    render(<StatsCard data={mockStats[2]!} />);
    expect(screen.getByText('↓')).toBeInTheDocument();
    expect(screen.getByText('1.1%')).toBeInTheDocument();
  });

  it('does not render trend indicator when trend is undefined', () => {
    render(<StatsCard data={mockStats[1]!} />);
    expect(screen.queryByText('↑')).not.toBeInTheDocument();
    expect(screen.queryByText('↓')).not.toBeInTheDocument();
  });

  it('trend up uses text-success-fg class', () => {
    const { container } = render(<StatsCard data={mockStats[0]!} />);
    const trendSpan = container.querySelector('.text-success-fg');
    expect(trendSpan).toBeInTheDocument();
  });

  it('trend down uses text-danger-fg class', () => {
    const { container } = render(<StatsCard data={mockStats[2]!} />);
    const trendSpan = container.querySelector('.text-danger-fg');
    expect(trendSpan).toBeInTheDocument();
  });
});

describe('StatsCardGrid', () => {
  it('renders 4 cards from stats data', () => {
    render(<StatsCardGrid stats={mockStats} />);
    expect(screen.getByText('Total Kas Beredar')).toBeInTheDocument();
    expect(screen.getByText('Saldo Vault Vendor')).toBeInTheDocument();
    expect(screen.getByText('Kas di Mesin ATM')).toBeInTheDocument();
    expect(screen.getByText('Drop CIT Hari Ini')).toBeInTheDocument();
  });

  it('has responsive grid classes', () => {
    const { container } = render(<StatsCardGrid stats={mockStats} />);
    const grid = container.firstElementChild as HTMLElement;
    expect(grid.className).toContain('grid-cols-1');
    expect(grid.className).toContain('grid-cols-2');
    expect(grid.className).toContain('grid-cols-4');
  });
});
