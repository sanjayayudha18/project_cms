import type { LucideIcon } from 'lucide-react';

/** Direction of a trend indicator */
export type TrendDirection = 'up' | 'down';

/** Semantic color tier for ATM cash levels */
export type CashLevelTier = 'success' | 'warning' | 'danger';

/** A single KPI stats card data */
export interface StatsCardData {
  readonly label: string;
  readonly icon: LucideIcon;
  readonly value: string;
  readonly subtitle?: string;
  readonly trend?: {
    readonly direction: TrendDirection;
    readonly percentage: number;
  };
}

/** A single day's cash flow per vendor (chart data point) */
export interface VendorDayFlow {
  readonly date: string; // ISO date string (YYYY-MM-DD)
  readonly [vendorName: string]: number | string; // vendor amounts + date key
}

/** Vendor metadata for chart legend and color mapping */
export interface VendorConfig {
  readonly name: string;
  readonly color: string; // OKLCH color value
}

/** A single ATM cash level row */
export interface AtmLevel {
  readonly id: string; // e.g. "ATM-00417"
  readonly label: string; // Display identifier
  readonly percentage: number; // 0–100
}

/** Complete data structure returned by useCashFlowData */
export interface CashFlowSummary {
  readonly stats: readonly StatsCardData[];
  readonly vendorChart: {
    readonly data: readonly VendorDayFlow[];
    readonly vendors: readonly VendorConfig[];
  };
  readonly atmLevels: readonly AtmLevel[];
}

/** Hook return type wrapping TanStack Query state */
export interface UseCashFlowDataReturn {
  readonly data: CashFlowSummary | undefined;
  readonly isLoading: boolean;
  readonly isError: boolean;
  readonly error: Error | null;
  readonly refetch: () => void;
}
