import type { VendorConfig } from './types';

/**
 * Vendor color palette — distinct OKLCH hues chosen for AA contrast (≥3:1)
 * against chart background (--n-0: oklch(0.992 0.003 29)).
 */
export const VENDOR_COLORS: readonly VendorConfig[] = [
  { name: 'Abacus', color: 'oklch(0.55 0.18 245)' }, // blue
  { name: 'Bijak Jakarta', color: 'oklch(0.60 0.16 155)' }, // green
  { name: 'Advantage', color: 'oklch(0.58 0.17 29)' }, // red (brand)
  { name: 'SSI', color: 'oklch(0.60 0.15 78)' }, // amber
] as const;

/** TanStack Query key for cash flow summary data */
export const CASH_FLOW_QUERY_KEY = ['cash-flow', 'summary'] as const;

/** Stale time for cash flow queries — EOD data doesn't change frequently */
export const CASH_FLOW_STALE_TIME = 5 * 60 * 1000; // 5 minutes

/** Minimum chart container height to prevent compression */
export const CHART_MIN_HEIGHT = 240;

/** Recharts bar chart internal height */
export const CHART_HEIGHT = 280;

/** Maximum bar width in the grouped bar chart */
export const CHART_MAX_BAR_SIZE = 32;

/** Bar top radius for rounded corners */
export const CHART_BAR_RADIUS: [number, number, number, number] = [3, 3, 0, 0];
