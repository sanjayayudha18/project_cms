import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from "recharts";
import type { VendorDayFlow, VendorConfig } from "./types";
import { CHART_HEIGHT, CHART_MAX_BAR_SIZE, CHART_BAR_RADIUS } from "./constants";

interface VendorBarChartProps {
  readonly data: readonly VendorDayFlow[];
  readonly vendors: readonly VendorConfig[];
}

/**
 * Memformat string tanggal ISO (YYYY-MM-DD) ke format pendek (misal "15 Jul").
 */
export function formatShortDate(dateStr: string): string {
  const date = new Date(dateStr);
  const day = date.getUTCDate();
  const month = date.toLocaleDateString("id-ID", {
    month: "short",
    timeZone: "UTC",
  });
  return `${day} ${month}`;
}

/**
 * Memformat nilai numerik chart dengan suffix "M" untuk jutaan.
 * Nilai sudah dalam satuan juta dari data mock.
 */
export function formatChartValue(value: number): string {
  return `Rp ${value} M`;
}

export function VendorBarChart({ data, vendors }: VendorBarChartProps) {
  return (
    <div
      className="min-h-[240px] w-full"
      aria-label="Grafik batang menampilkan arus kas harian per vendor selama 7 hari terakhir"
    >
      <ResponsiveContainer width="100%" height={CHART_HEIGHT}>
        <BarChart data={[...data]} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
          <CartesianGrid
            strokeDasharray="3 3"
            stroke="oklch(0.908 0.006 29)"
            vertical={false}
          />
          <XAxis
            dataKey="date"
            tick={{ fontSize: 12, fill: "oklch(0.56 0.009 29)" }}
            tickFormatter={formatShortDate}
          />
          <YAxis
            tick={{ fontSize: 12, fill: "oklch(0.56 0.009 29)" }}
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
