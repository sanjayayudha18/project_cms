import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts';
import type { VendorDayFlow, VendorConfig } from './types';
import {
  CHART_HEIGHT,
  CHART_MAX_BAR_SIZE,
  CHART_BAR_RADIUS,
} from './constants';

interface VendorBarChartProps {
  readonly data: readonly VendorDayFlow[];
  readonly vendors: readonly VendorConfig[];
}

/**
 * Formats an ISO date string (YYYY-MM-DD) to short format (e.g., "15 Jul").
 */
export function formatShortDate(dateStr: string): string {
  const date = new Date(dateStr);
  const day = date.getUTCDate();
  const month = date.toLocaleDateString('en-US', {
    month: 'short',
    timeZone: 'UTC',
  });
  return `${day} ${month}`;
}

/**
 * Formats a numeric chart value with "M" suffix for millions.
 * Values are already in millions from the mock data.
 */
export function formatChartValue(value: number): string {
  return `Rp ${value} M`;
}

export function VendorBarChart({ data, vendors }: VendorBarChartProps) {
  return (
    <div
      className="min-h-[240px] w-full"
      aria-label="Bar chart showing daily cash flow per vendor for the past 7 days"
    >
      <ResponsiveContainer width="100%" height={CHART_HEIGHT}>
        <BarChart
          data={[...data]}
          margin={{ top: 8, right: 8, left: 0, bottom: 0 }}
        >
          <CartesianGrid
            strokeDasharray="3 3"
            stroke="oklch(0.908 0.006 29)"
            vertical={false}
          />
          <XAxis
            dataKey="date"
            tick={{ fontSize: 12, fill: 'oklch(0.560 0.009 29)' }}
            tickFormatter={formatShortDate}
          />
          <YAxis
            tick={{ fontSize: 12, fill: 'oklch(0.560 0.009 29)' }}
            tickFormatter={formatChartValue}
          />
          <Tooltip />
          <Legend verticalAlign="bottom" wrapperStyle={{ paddingTop: 12 }} />
          {vendors.map((vendor) => (
            <Bar
              key={vendor.name}
              dataKey={vendor.name}
              fill={vendor.color}
              radius={CHART_BAR_RADIUS}
              maxBarSize={CHART_MAX_BAR_SIZE}
            />
          ))}
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
