import type { VendorConfig } from "./types";

/**
 * Palet warna vendor — hue OKLCH berbeda dipilih untuk kontras AA (≥3:1)
 * terhadap latar chart (--n-0: oklch(0.992 0.003 29)).
 *
 * Menggunakan nilai OKLCH langsung dari tokens.css karena recharts SVG
 * tidak dapat me-resolve CSS custom properties di atribut fill.
 *
 * --info-solid:    oklch(0.58 0.12 245)
 * --success-solid: oklch(0.56 0.13 155)
 * --red-500:       oklch(0.552 0.205 29)
 * --warning-solid: oklch(0.76 0.15 78)
 */
export const VENDOR_COLORS: readonly VendorConfig[] = [
  { name: "Abacus", color: "oklch(0.58 0.12 245)" }, // --info-solid (blue)
  { name: "Bijak Jakarta", color: "oklch(0.56 0.13 155)" }, // --success-solid (green)
  { name: "Advantage", color: "oklch(0.552 0.205 29)" }, // --red-500 (brand)
  { name: "SSI", color: "oklch(0.76 0.15 78)" }, // --warning-solid (amber)
] as const;

/** Kunci query TanStack Query untuk data ringkasan cash flow */
export const CASH_FLOW_QUERY_KEY = ["cash-flow", "summary"] as const;

/** Stale time untuk query cash flow — data EOD tidak sering berubah */
export const CASH_FLOW_STALE_TIME = 5 * 60 * 1000; // 5 menit

/** Tinggi minimum container chart untuk mencegah kompresi */
export const CHART_MIN_HEIGHT = 240;

/** Tinggi internal bar chart recharts */
export const CHART_HEIGHT = 280;

/** Lebar bar maksimum dalam grouped bar chart */
export const CHART_MAX_BAR_SIZE = 32;

/** Radius atas bar untuk sudut membulat */
export const CHART_BAR_RADIUS: [number, number, number, number] = [3, 3, 0, 0];
